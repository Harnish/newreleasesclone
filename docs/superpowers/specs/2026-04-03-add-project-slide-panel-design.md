# Design: Add Project Slide-over Panel

**Date:** 2026-04-03
**Status:** Approved

## Summary

Move the "Add Project" form from its own tab into a slide-over panel on the right side of the Releases page. The panel pushes the releases list to the left when open. The `+ Add Project` and `Projects` nav tabs are removed.

## Nav Changes

- Remove the `+ Add Project` tab from the nav bar.
- Remove the `Projects` tab from the nav bar.
- Remove the entire `<nav class="tabs">` bar — with only one destination remaining it serves no purpose.

> **Trade-off:** Removing the Projects tab temporarily hides webhook management. This is intentional — the next TODO item ("Inline project controls") will add Refresh, Delete, and webhook access directly onto the project cards in the Releases view.

## Layout

`#panel-releases` becomes a flex row with two children:
1. `#releases-root` — flex: 1, min-width: 0, holds the existing project cards
2. `#add-project-panel` — a fixed-width panel (280px) on the right

The panel uses `max-width` + `overflow: hidden` + CSS transition to animate open/close without JS layout calculation:
- Closed: `max-width: 0`, content invisible
- Open: `max-width: 280px`

## Trigger

A `+ Add Project` button is placed in a header row above the releases list (right-aligned). Clicking it toggles the panel open/closed.

## Panel Contents

- Title: "Add Project" + ✕ close button (right-aligned)
- Platform `<select>` (GitHub, GitLab, NPM, PyPI, Docker Hub, Other / Custom URL)
- Repo/URL `<input>` with hint text below (same `onPlatformChange` / `autoFillName` logic)
- Display name `<input>`
- "Add Project" submit button (full width)

The existing `add-form` submit handler, `onPlatformChange`, and `autoFillName` functions are reused unchanged.

## Close Behavior

The panel closes when:
- The ✕ button is clicked
- The `+ Add Project` toggle button is clicked while the panel is open
- The Escape key is pressed (keydown listener on document)

## On Success

After a successful `POST /api/projects`:
1. Reset the form
2. Close the panel
3. Reload releases (`loadReleases()`)
4. Show toast: "Project added — fetching releases in background."

## Files Changed

- `ui.go` — all changes are in the HTML/CSS/JS string constant

## Out of Scope

- Webhook management UI (moved to "Inline project controls" task)
- Project list / refresh from the old Projects tab (same task)
- Any backend changes
