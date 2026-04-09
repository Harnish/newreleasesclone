# Pagination Design

**Date:** 2026-04-09  
**Status:** Approved

## Overview

Add client-side pagination to the project cards on the main releases page. Users can set a page size of 5, 10, or 20 projects per page. The preference is persisted per user in the database and cached in `localStorage` for instant load.

## Approach

Client-side pagination. The existing `/api/projects` and `/api/releases` endpoints return all data unchanged. Slicing happens in JavaScript. This avoids API complexity and is appropriate for the expected number of projects per user.

## Data & State

### Database

- Add `page_size INTEGER NOT NULL DEFAULT 10` column to the `users` table.
- Schema version bumped from 8 to 9 in `migrate()`.
- `GetUserByID` updated to scan `page_size`.
- `User` struct gets a `PageSize int` field (JSON: `page_size`).

### API

- `GET /api/me` — already returns the full `User`; `page_size` is included automatically.
- `POST /api/account-settings` — accepts `{"page_size": 5|10|20}`, validates the value, writes to DB.

### Client state

- `allProjects` and `allReleases` — module-level JS variables holding the full API response, set once on `loadReleases()` and updated in-place on add/delete.
- `currentPage` (integer, starts at 1) — resets to 1 on page size change, project add, or project delete.
- `pageSize` (integer) — initialized from `localStorage.getItem('page_size')` (fast path), then confirmed/overridden by `/api/me` response.

## UI Controls

### Page size selector

- A `<select>` with options 5, 10, 20 added to the releases header bar, left of the "+ Add Project" button.
- On change: save to `localStorage`, POST to `/api/account-settings`, reset `currentPage` to 1, call `renderPage()`.

### Pagination controls

- Rendered below the project cards.
- Format: `<- Prev  |  Page N of M  |  Next ->`
- Prev/Next buttons disabled at page boundaries.
- Hidden entirely when total projects <= page size (everything fits on one page).

### Project card releases

- No change. Each project card continues to show 5 releases with "+N more" expand.

## Implementation

### New function: `renderPage()`

Slices `allProjects` to the current page window, filters `allReleases` to only releases belonging to the visible projects, passes both to `buildReleasesHTML`, then appends pagination controls. All output is escaped via the existing `esc()` helper before insertion into the DOM.

### Updated `loadReleases()`

- Fetches `/api/releases` and `/api/projects` as before.
- Stores results in `allReleases` and `allProjects`.
- Calls `renderPage()` instead of directly building the full HTML.

### Updated add/delete flows

- On project add: push new project into `allProjects`, re-call `renderPage()` (no re-fetch; releases populate on next refresh-check).
- On project delete: filter project out of `allProjects` and its releases out of `allReleases`, re-call `renderPage()`. If `currentPage` is now beyond the last page, decrement it first.

### `buildPaginationHTML()`

Returns an HTML string for prev/next controls and page indicator. Only emits markup when `allProjects.length > pageSize`.

### Files changed

| File | Change |
|------|--------|
| `store.go` | `migrate()` v8 to v9, add `page_size` column; update `GetUserByID` scan; add `SetPageSize(userID, size)` store method |
| `models.go` | Add `PageSize int` (json: `page_size`) to `User` struct |
| `handlers.go` | `handleAccountSettings` reads/writes `page_size` |
| `ui.go` | Add `pageSize` selector to header, add `renderPage()`, `buildPaginationHTML()`, update `loadReleases()`, add/delete flows |

## Error Handling

- Invalid `page_size` values (not 5, 10, or 20) rejected with 400 from the API.
- If `localStorage` has a stale/invalid value, fall back to 10.
- If `currentPage` goes out of range after a delete, clamp to last valid page before rendering.

## Testing

- Unit test `SetPageSize` store method (valid and invalid values).
- Handler test for `POST /api/account-settings` with `page_size`.
- JS logic is exercised manually (no test harness for frontend JS in this project).
