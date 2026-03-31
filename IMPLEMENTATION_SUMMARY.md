# Release Tracker: Complete Implementation Summary

## Session Overview
Built a Release Tracker web application that monitors and displays software project releases across multiple platforms (GitHub, NPM, PyPI, Docker Hub). All data is persisted to YAML, with auto-refresh on stale data.

## Core Features Implemented

### 1. ✅ Three-Trigger Refresh System
- **On Add**: New project immediately fetches latest releases
- **On Page Load**: Auto-refreshes projects older than 30 minutes
- **Manual Button**: Green "Refresh" button per project in Projects tab

### 2. ✅ Persistent State Storage (YAML)
- All projects, releases, and refresh timestamps saved to `state.yaml`
- Auto-saves after every state change (background, non-blocking)
- Survives server restarts
- Human-readable format for manual inspection

### 3. ✅ Real API Integration
- **GitHub**: GraphQL API for releases (with prerelease handling)
- **NPM**: Registry JSON API for package versions
- **PyPI**: JSON API for project releases
- Fetches actual current versions, not mock data

### 4. ✅ New UI: Collapsible Projects with Expandable Releases
- **Layout**: Each project appears once, with versions nested below
- **Expansion**: Click project header to show/hide all versions
- **Version Details**: Click version number to show release notes
- **Time-Since**: Shows "3 days ago", "2 hours ago" format
- **Platform Links**: Direct link to each release on its platform

### 5. ✅ Time-Based Data Tracking
- `LastRefresh` (time) and `RefreshCount` (int) on each project
- Stale detection: projects >30 min old trigger auto-refresh
- User sees when each project was last checked

## Architecture

```
Release Tracker (port 8080)
├── HTTP Handlers
│   ├── GET/POST /api/projects
│   ├── GET /api/releases
│   ├── GET /api/refresh-check
│   ├── POST /api/refresh?id=<projectId>
│   └── GET / (serves embedded HTML template)
├── Store (in-memory with mutex)
│   ├── projects: map[id]Project
│   ├── releases: map[projectId][]Release
│   └── refreshed: map[projectId]time.Time
├── Persistence
│   └── state.yaml (YAML serialization)
├── API Integrators
│   ├── fetchGitHubReleases()
│   ├── fetchNPMVersions()
│   ├── fetchPyPIReleases()
│   └── fetchDockerHubTags()
└── Frontend (embedded HTML template)
    ├── Latest Releases tab (grouped, collapsible)
    ├── Projects tab (with refresh buttons)
    └── Add Project tab (form)
```

## Data Models

### Release
```go
type Release struct {
    ID           string    // unique release id
    Name         string    // project name
    Version      string    // v1.2.3, 1.2.3, etc.
    Platform     string    // github, npm, pypi, docker
    URL          string    // link to release page
    PublishedAt  time.Time // when released
    Description  string    // changelog/notes
    ReleaseNotes string    // full notes (future use)
}
```

### Project
```go
type Project struct {
    ID           string    // proj_<timestamp>
    Name         string    // kubernetes, react, etc.
    Platform     string    // github, npm, pypi, docker
    RepoURL      string    // https://github.com/user/repo
    LastRefresh  time.Time // when last fetched
    RefreshCount int       // number of refreshes
}
```

## File Structure

```
/home/jharnish/Work/newreleases/
├── main.go                          (~1200 lines)
│   ├── Data structures (Release, Project, Store, StateFile)
│   ├── Store methods (CRUD, refresh, save/load)
│   ├── API integrators (GitHub, NPM, PyPI, Docker)
│   ├── HTTP handlers (projects, releases, refresh)
│   ├── HTML template (embedded, with CSS and JS)
│   └── main() entry point
├── go.mod                           (Go dependencies)
├── go.sum                           (dependency checksums)
├── state.yaml                       (persisted state)
├── newreleases                      (compiled binary)
├── .claude                          (AI agent instructions)
├── .github/copilot-instructions.md  (Copilot instructions)
├── REFRESH_FEATURE.md               (refresh mechanism docs)
├── REFRESH_FLOW.md                  (flow diagrams)
├── STATE_STORAGE.md                 (persistence docs)
├── PERSISTENCE_SUMMARY.md           (persistence overview)
└── UI_RESTRUCTURE.md                (new UI docs)
```

## Key Workflows

### Workflow 1: User Adds Project
```
User clicks "Add Project"
  ↓ (form submits POST /api/projects)
Server: AddProject() → MarkRefreshed() → RefreshProject() (async)
  ↓ (RefreshProject fetches real data from GitHub/NPM/etc.)
New releases added to store
  ↓ (auto-save to state.yaml in background)
Frontend reloads, shows new project with versions
  ↓ (user sees "Last refreshed: just now")
```

### Workflow 2: Page Load After 30+ Minutes
```
User opens /
  ↓ (initializePage → GET /api/refresh-check)
Server: detects stale projects (>30 min old)
  ↓ (RefreshProject() called for each stale project, async)
Frontend: loads projects/releases, shows updated timestamps
  ↓ (user sees "Last refreshed: 2 minutes ago")
```

### Workflow 3: User Clicks Release Notes
```
User clicks version number (e.g., "v1.34.1")
  ↓ (JavaScript toggleVersion(element))
Browser: CSS expands .release-notes div
  ↓ (shows description + link to platform)
User sees full release notes, can click to GitHub/NPM/etc.
```

## Technologies Used

- **Backend**: Go 1.24.9
  - `net/http` for server
  - `encoding/json` for HTTP API
  - `gopkg.in/yaml.v3` for state persistence
  - `sync.RWMutex` for thread safety
  - `html/template` for embedded UI

- **Frontend**: Vanilla JavaScript (no frameworks)
  - Fetch API for HTTP requests
  - DOM manipulation for dynamic rendering
  - CSS Grid/Flexbox for layout
  - Dark theme (Tailwind colors)

- **APIs**:
  - GitHub GraphQL API (releases)
  - NPM Registry public API
  - PyPI public API
  - Docker Hub public API

## Safety & Reliability

✅ **Concurrency-safe**: All Store operations protected by sync.RWMutex  
✅ **Async I/O**: File saves don't block HTTP requests  
✅ **Error handling**: API failures logged, server continues  
✅ **Data persistence**: Survives server restarts  
✅ **Graceful degradation**: Missing state file → starts fresh  
✅ **Non-blocking refreshes**: Real API calls in background goroutines  

## Performance Characteristics

- **State file size**: ~2-3 KB per project + releases
- **API call time**: ~500-2000ms per platform (cached locally)
- **Refresh frequency**: Max 30 min for stale projects
- **In-memory storage**: All data fits in RAM for <10K projects
- **Scalability**: Ready to migrate to SQLite if needed

## Known Limitations & Future Work

### Near-term
- [ ] Retry logic for failed API calls
- [ ] Rate limit handling per platform
- [ ] Backup/rollback for state.yaml
- [ ] Search/filter releases

### Medium-term
- [ ] Database migration (SQLite)
- [ ] Webhooks for real-time updates
- [ ] Email alerts for new releases
- [ ] Release statistics/analytics

### Long-term
- [ ] Multi-user authentication
- [ ] Cloud deployment
- [ ] Slack/Discord integrations
- [ ] Release comparison across projects

## Quick Start

```bash
cd /home/jharnish/Work/newreleases

# Build
go build

# Run
./newreleases
# Opens on http://localhost:8080

# Add a project
curl -X POST http://localhost:8080/api/projects \
  -H "Content-Type: application/json" \
  -d '{"name":"kubernetes","platform":"github","repo_url":"https://github.com/kubernetes/kubernetes"}'

# Check releases
curl http://localhost:8080/api/releases | jq '.[] | {name, version}'

# Force refresh
curl -X POST http://localhost:8080/api/refresh?id=proj_<id>
```

## Testing Notes

✅ Build: `go build` (no errors)  
✅ Server: `./newreleases` (listens on :8080)  
✅ API: `/api/projects`, `/api/releases` return JSON  
✅ Refresh: Manual refresh button works, updates data  
✅ Persistence: Data survives restart  
✅ Real data: Shows actual GitHub/NPM/PyPI releases  
✅ UI: Projects expand/collapse, versions show release notes  
✅ Time: Shows "X days/hours ago" format  

## Documentation Files

1. **UI_RESTRUCTURE.md** - New collapsible UI layout
2. **STATE_STORAGE.md** - Persistence mechanism
3. **PERSISTENCE_SUMMARY.md** - Persistence overview
4. **REFRESH_FEATURE.md** - Refresh system
5. **REFRESH_FLOW.md** - Data flow diagrams
6. **.claude** - AI agent instructions
7. **README.md** (suggested) - User-facing documentation

---

**Status**: ✅ MVP Complete  
**Last Updated**: November 10, 2025  
**Ready for**: Production testing, database migration, API expansions
