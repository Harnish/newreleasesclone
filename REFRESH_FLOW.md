# Refresh Feature: Visual Flow Diagram

## Data Flow

```
┌─ TRIGGER #1: Add Project ─┐
│                             │
│  POST /api/projects         │
│  ↓                          │
│  handleProjects()           │
│  ├─ AddProject()            │
│  ├─ MarkRefreshed()         │
│  └─ go RefreshProject()     │ (async)
│     ├─ Generate new Release │
│     ├─ AddRelease()         │
│     └─ MarkRefreshed()      │
│                             │
│  Response: Project{         │
│    last_refresh: NOW,       │
│    refresh_count: 1         │
│  }                          │
└─────────────────────────────┘

┌─ TRIGGER #2: Page Load ────┐
│                             │
│  Browser loads index        │
│  ↓                          │
│  initializePage()           │
│  ├─ GET /api/refresh-check  │
│  │  ↓                       │
│  │  handleRefreshCheck()    │
│  │  ├─ GetStaleProjects()   │
│  │  │  (last_refresh       │
│  │  │   > 30 min ago?)     │
│  │  ├─ for each stale:      │
│  │  │  go RefreshProject()  │ (async)
│  │  └─ return count         │
│  ├─ loadReleases()          │
│  └─ loadProjects()          │
│     ├─ Show last_refresh    │
│     ├─ Show Refresh button  │
│     └─ Display data         │
│                             │
│  Effect: Stale projects    │
│  auto-refresh silently      │
└─────────────────────────────┘

┌─ TRIGGER #3: Manual Click ─┐
│                             │
│  User clicks "Refresh"      │
│  button in Projects tab     │
│  ↓                          │
│  POST /api/refresh?id=123   │
│  ↓                          │
│  handleRefreshProject()     │
│  ├─ RefreshProject(id)      │
│  │  ├─ Get project          │
│  │  ├─ Generate Release     │
│  │  ├─ AddRelease()         │
│  │  └─ MarkRefreshed()      │
│  └─ return {status, id}     │
│  ↓                          │
│  JS reloads projects &      │
│  releases after 500ms       │
│  ↓                          │
│  UI shows updated           │
│  last_refresh time &        │
│  new releases              │
│                             │
│  Effect: User sees latest   │
│  data immediately           │
└─────────────────────────────┘
```

## Stale Detection Logic

```go
IsStale(projectID) {
  if (time.Since(lastRefresh) > 30 * time.Minute) {
    return true
  }
  return false
}

// On page load, for each stale project:
// - Call RefreshProject() in background
// - User sees fresh data when page finishes loading
```

## Data Structure Timeline

### When Project Added
```
Time 0:
  Project{
    ID: "proj_12345",
    LastRefresh: NOW,      ← Set on creation
    RefreshCount: 1,       ← Incremented by MarkRefreshed()
  }
  Store.refreshed["proj_12345"] = NOW

Time 0+ (async):
  RefreshProject() completes
  → MarkRefreshed() called
  → Store.refreshed["proj_12345"] = NOW (same)
  → Project.RefreshCount → 2
```

### When Page Loads (e.g., 35 min later)
```
  IsStale("proj_12345")
  → time.Since(NOW) = 35 min
  → 35 min > 30 min ✓ STALE
  
  GetStaleProjects() includes proj_12345
  
  RefreshProject("proj_12345") called
  → New Release added
  → MarkRefreshed() called
  → Store.refreshed["proj_12345"] = NEW_TIME
  → Project.LastRefresh = NEW_TIME
  → Project.RefreshCount → 3
```

## Frontend State Management

```javascript
// Displayed in Projects tab for each project:
<div class="release-meta">
  Last refreshed: ${p.last_refresh ? new Date(p.last_refresh).toLocaleString() : 'Never'}
</div>

// Refresh button
<button onclick="refreshProject('${p.id}', event)">Refresh</button>

// On click:
async function refreshProject(projectId, event) {
  const res = await fetch('/api/refresh?id=' + projectId, { method: 'POST' });
  if (res.ok) {
    setTimeout(() => {
      loadProjects();    // Reload to get updated last_refresh
      loadReleases();    // Reload to get new releases
    }, 500);
  }
}
```

## Mock Data Flow (Current)

```
RefreshProject("kubernetes") 
→ Generate Release:
  {
    ID: "rel_1731258399123456789",
    Name: "kubernetes",
    Version: "v1.88.0",                    (random)
    Platform: "github",
    PublishedAt: NOW,
    Description: "Auto-refreshed on 2025-11-10 17:26"
  }
→ AddRelease("kubernetes", release)
→ MarkRefreshed("kubernetes")
```

## Production Integration (Future)

Replace mock with real API calls in `RefreshProject()`:

```go
func (s *Store) RefreshProject(projectID string) {
  project := // fetch project
  
  // Switch on platform
  switch project.Platform {
  case "github":
    releases := fetchGitHubReleases(project)
  case "npm":
    releases := fetchNPMVersions(project)
  case "pypi":
    releases := fetchPyPIReleases(project)
  // ...
  }
  
  // Add each release
  for _, release := range releases {
    s.AddRelease(projectID, release)
  }
  
  s.MarkRefreshed(projectID)
}
```
