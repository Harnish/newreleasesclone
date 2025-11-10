# Persistent State Storage (YAML)

## Overview
The Release Tracker now persists all project and release data to a `state.yaml` file on disk. This ensures data survives server restarts and provides a single source of truth for the application state.

## Implementation

### Storage Format
**File**: `state.yaml` in the repository root

**Structure**:
```yaml
projects:
  "project_id":
    id: "project_id"
    name: "project_name"
    platform: "github"
    repo_url: "https://..."
    last_refresh: 2025-11-10T17:42:41Z
    refresh_count: 5

releases:
  "project_id":
    - id: "release_id"
      name: "project_name"
      version: "v1.0.0"
      platform: "github"
      url: "https://..."
      published_at: 2025-11-10T15:00:00Z
      description: "Release description"

refreshed:
  "project_id": 2025-11-10T17:42:41Z
```

### Auto-Save Mechanism
- **Trigger**: Every time `AddProject()`, `AddRelease()`, or `MarkRefreshed()` is called
- **Method**: Async background goroutine via `asyncSave()` to avoid blocking
- **Error handling**: Logs warnings if save fails; does not crash the server

### Load on Startup
1. Server calls `store.LoadState("state.yaml")` in `main()`
2. If file exists: Deserialize YAML and populate Store with all projects, releases, and refresh timestamps
3. If file missing: Seed with demo data and save to disk
4. If both file and projects are empty: User sees empty state and can add projects

## Data Flow

### On Add Project
```
POST /api/projects {name, platform, repo_url}
  ↓
handleProjects() → AddProject()
  ↓
Store.projects[id] = project
  ↓
asyncSave() calls SaveState("state.yaml") in background
  ↓
state.yaml updated with new project
```

### On Server Restart
```
main() calls LoadState("state.yaml")
  ↓
Read YAML from disk
  ↓
Unmarshal into Store maps
  ↓
All projects, releases, refresh times restored
  ↓
User sees exact same state as before restart
```

### On Manual Refresh
```
POST /api/refresh?id=project_id
  ↓
RefreshProject() creates new Release
  ↓
AddRelease() → asyncSave() writes state.yaml
  ↓
MarkRefreshed() → asyncSave() writes state.yaml
  ↓
state.yaml updated with new release and refresh time
```

## Methods

### SaveState(filename string) error
- Manually save state to file
- Called automatically by `asyncSave()`
- Logs success/failure to console

### LoadState(filename string) error
- Load state from file at startup
- Gracefully handles missing file
- Restores projects, releases, and refresh timestamps

### asyncSave()
- Internal helper that calls SaveState() in a background goroutine
- Prevents blocking on I/O operations
- Logs errors but doesn't crash

## YAML Struct Tags
All fields in `Release` and `Project` now have both `json` and `yaml` tags:
```go
type Release struct {
  ID          string    `json:"id" yaml:"id"`
  Name        string    `json:"name" yaml:"name"`
  // ...
}
```

This enables bidirectional serialization (HTTP JSON ↔ YAML storage).

## Behavior Examples

### Example 1: Add Project → Restart → Verify Persistence
```bash
# 1. Start server (empty, seeds demo data)
$ ./newreleases
# Output: State saved to state.yaml (3 projects from demo)

# 2. Add new project via curl
$ curl -X POST http://localhost:8080/api/projects \
  -H "Content-Type: application/json" \
  -d '{"name":"pypi-pkg","platform":"pypi","repo_url":"https://pypi.org"}'

# 3. Check state.yaml was updated
$ grep "pypi-pkg" state.yaml
# Result: pypi-pkg found in projects section

# 4. Kill server and restart
$ pkill newreleases
$ ./newreleases
# Output: ✓ State loaded from state.yaml (4 projects, X releases)

# 5. Verify pypi-pkg is still there
$ curl http://localhost:8080/api/projects | jq '.[] | select(.name=="pypi-pkg")'
# Result: pypi-pkg returned; persisted successfully!
```

### Example 2: Manual Refresh Updates State
```bash
# Click "Refresh" button for a project
POST /api/refresh?id=proj_123
  ↓
New release added
  ↓
asyncSave() runs → state.yaml updated
  ↓
state.yaml now contains new release + updated last_refresh timestamp
```

## Error Handling
- **Missing file**: LoadState() returns nil; server seeds demo data
- **Invalid YAML**: LoadState() returns error; server continues with empty state (no crash)
- **Failed writes**: asyncSave() logs warning; state persists in memory but not on disk
- **Concurrent access**: Store.mu protects all reads/writes during serialization

## Performance Considerations
- **Async saves**: Non-blocking; users don't wait for disk I/O
- **File size**: ~1-2 KB per project + releases (YAML is text-based)
- **Write frequency**: Up to 3 per operation (add project, add release, mark refreshed) in worst case
- **Scaling**: For 1000+ projects, consider upgrading to database (SQLite, PostgreSQL)

## Future Improvements
1. **Backup**: Before overwriting state.yaml, save previous version as state.yaml.bak
2. **Compression**: Use gzip for large state files (state.yaml.gz)
3. **Atomic writes**: Write to temp file, then rename to avoid corruption on crash
4. **Database**: SQLite for better querying and indexing at scale
5. **Snapshot API**: `GET /api/state` to download current state as YAML/JSON

## Testing Checklist
- ✓ Build without errors: `go build`
- ✓ Start server: `./newreleases` → state.yaml created with demo data
- ✓ Add new project → appears in state.yaml within 1 second
- ✓ Restart server → all projects still there
- ✓ Delete state.yaml → server reseeds demo data on restart
- ✓ Manual refresh → new release in state.yaml
- ✓ Page load after 30+ min → stale projects auto-refresh and persist
