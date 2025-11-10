# Release Refresh System

## Overview
Implemented a three-trigger refresh mechanism to keep release data current:

### Trigger #1: Auto-Refresh on Add
- **When**: A new project is added via `POST /api/projects`
- **How**: `RefreshProject(projectID)` runs in a goroutine immediately after `AddProject()`
- **Effect**: New releases are fetched and added to the store; `last_refresh` timestamp is set

### Trigger #2: Auto-Refresh on Page Load (Stale Check)
- **When**: Page loads or user navigates to the Releases/Projects tab
- **How**: `initializePage()` calls `GET /api/refresh-check` 
- **Logic**: Checks all projects for stale data (>30 minutes old); auto-refreshes stale ones
- **Effect**: Users always see recent data without manual action

### Trigger #3: Manual Refresh Button
- **When**: User clicks "Refresh" button in Projects tab
- **How**: Frontend calls `POST /api/refresh?id={projectID}`
- **Effect**: Single project is refreshed immediately; UI updates after 500ms

## Implementation Details

### Data Structures
```go
type Project struct {
  // ... existing fields
  LastRefresh  time.Time `json:"last_refresh"`    // Timestamp of last refresh
  RefreshCount int       `json:"refresh_count"`   // Total refreshes for this project
}

type Store struct {
  // ... existing fields
  refreshed map[string]time.Time  // Tracks when each project was last refreshed
}
```

### Key Methods
- `MarkRefreshed(projectID)` — Mark a project as refreshed now; updates `Store.refreshed` and `Project.LastRefresh`
- `IsStale(projectID) bool` — Check if >30 min since last refresh
- `GetStaleProjects() []Project` — Return all projects needing refresh
- `RefreshProject(projectID)` — Fetch new release data (mock for now; ready for real APIs)

### New Endpoints
- `GET /api/refresh-check` → auto-refreshes stale projects, returns count
- `POST /api/refresh?id={projectID}` → force refresh a single project

### Frontend Changes
- **Projects tab**: Added "Refresh" button (green) and "Last refreshed" timestamp for each project
- **Page load**: `initializePage()` calls stale-check endpoint before loading data
- **After add**: Reloads projects/releases after form submit to show new data

## Current Status
- **Mock refresh**: `RefreshProject()` generates synthetic release data (version bumps, timestamps)
- **Ready for real APIs**: Code structure supports GitHub, NPM, PyPI, Docker Hub (switch on `project.Platform`)

## Next Steps (Production)
1. Add platform-specific API integrations in `RefreshProject()`:
   - GitHub: GraphQL or REST API for releases
   - NPM: Registry API for package versions
   - PyPI: JSON API for project releases
   - Docker Hub: API for image tags

2. Add error handling & retry logic for failed fetches

3. Consider caching to avoid rate limits (e.g., track last refresh per platform)

4. Optional: Background job to refresh popular projects periodically

## Testing
- Build: `go build` ✓
- Run: `./newreleases` (listens on `:8080`)
- Manual test: Add a project → releases appear → refresh button shows recent "refreshed" time
- Stale test: Wait 30 min or modify mock data to test auto-refresh on page load
