# UI Restructure: Projects with Collapsible Releases

## What's Been Implemented

✅ **Collapsible Project Cards**
- Each project appears once at the top level
- Clicking a project header expands/collapses its releases
- Shows project name, platform badge, and version count

✅ **Expandable Version List**
- All releases for a project listed under it
- Shows version number, time-since-release (e.g., "2 days ago")
- Clicking a version shows release notes
- Direct link to view on platform (GitHub, NPM, PyPI, etc.)

✅ **Time-Since-Release Formatting**
- Helper function `timeSinceRelease()` calculates:
  - "X minutes ago" (< 1 hour)
  - "X hours ago" (< 1 day)
  - "X days ago" (< 30 days)
  - Full date after 30 days

✅ **Real API Data**
- Integrated GitHub API (releases)
- Integrated NPM Registry API (versions)
- Integrated PyPI JSON API (releases)
- Fetches actual current versions (not mock data)

## New CSS Classes

```css
.project-section       /* Wrapper for each project group */
.project-header        /* Clickable header (toggles expand) */
.project-name          /* Large project title */
.project-meta          /* Badges and version count */
.expand-icon           /* ▼ arrow (rotates when expanded) */
.versions-list         /* Container for releases (show/hide) */
.version-item          /* Individual version entry */
.version-number        /* Version string (v1.2.3, 19.3.0, etc.) */
.version-date          /* Time since release */
.release-notes         /* Expandable notes section */
```

## JavaScript Flow

1. **Page Load**
   - `initializePage()` → calls `/api/refresh-check` for stale projects
   - Loads all projects and releases
   - Groups releases by projectID

2. **Render Releases Tab**
   - `loadReleases()` fetches `/api/releases` and `/api/projects`
   - Groups releases by project
   - Renders each project as a collapsible card
   - Versions hidden by default

3. **User Interactions**
   - Click project header → `toggleProject(projectId)` expands/collapses
   - Click version → `toggleVersion(element)` shows/hides release notes
   - "Refresh" button → `POST /api/refresh?id=projectId` 
   - "Add Project" form → posts to `/api/projects`, auto-refreshes

## Example UI Layout

```
🚀 Release Tracker

┌─ kubernetes ▼ [github] 5 versions
│  └─ v1.34.1 (3 days ago)
│     Release Notes: [Details...] [View on github]
│  └─ v1.34.0 (5 days ago)
│  └─ v1.33.5 (1 week ago)
│  └─ v1.33.4 (2 weeks ago)
│  └─ v1.33.3 (3 weeks ago)
│
├─ react ▼ [npm] 10 versions
│  └─ 19.3.0-canary-ead92181 (2 days ago)
│     Release Notes: [Details...] [View on npm]
│  └─ 19.3.0-canary-dd048c3b (3 days ago)
│  └─ 19.3.0-canary (1 week ago)
│  └─ ...
│
└─ golang ▼ [github] 5 versions
   └─ v1.24.3 (1 day ago)
      Release Notes: [Details...] [View on github]
   └─ v1.24.2 (1 week ago)
   └─ ...
```

## Key Improvements Over Original

| Feature | Before | After |
|---------|--------|-------|
| **Layout** | Flat list of all releases | Grouped by project, collapsible |
| **Versions** | All shown at once | Hidden until project expanded |
| **Release Notes** | Static description | Click to expand |
| **Time** | Just date | "3 days ago" format |
| **Data** | Mock/demo data | Real API data (GitHub, NPM, PyPI) |
| **Platform Links** | Sometimes missing | Always present, clickable |

## Next Steps (Optional)

- Add search/filter by project name or version
- Add sorting (newest first, by update date, etc.)
- Add statistics (total projects, versions tracked, etc.)
- Add export to CSV/JSON
- Dark mode toggle
- Remember collapsed/expanded state in localStorage

## Testing

Build and run:
```bash
cd /home/jharnish/Work/newreleases
go build
./newreleases
# Open http://localhost:8080 in browser
```

Navigate to "Latest Releases" tab to see the new collapsible layout.
