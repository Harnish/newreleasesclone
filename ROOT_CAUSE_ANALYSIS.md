# Root Cause Analysis: State Persistence Issue

## Problem Statement

Custom projects added via API are not persisting across server restarts. Only the demo projects (kubernetes, react, golang) survive restarts.

## Root Cause Identified

The state.yaml file is becoming corrupted with YAML parsing errors (line 24: did not find expected key) during or after the addition of custom projects.

### Sequence of Events

1. User adds a custom project via `/api/projects` POST
2. `AddProject()` is called, which synchronously saves state to state.yaml
3. `RefreshProject()` runs as a goroutine, fetching API data
4. RefreshProject tries to call AddRelease() for each release found
5. But if the API returns 404 (invalid repo), no releases are added
6. Server continues running normally with the custom project in memory
7. User restarts server
8. `LoadState()` tries to parse state.yaml
9. YAML parsing fails due to corruption at line 24
10. LoadState() returns an error
11. main() sees 0 projects (because LoadState failed), so re-seeds demo data
12. Demo data overwrites the custom project

## YAML Corruption Root Cause

When `yaml.Marshal()` is called on a StateFile with projects that have NO releases:
- Project "proj_xyz" exists in the projects map
- But releases["proj_xyz"] doesn't exist (not in the map)
- Or releases["proj_xyz"] = nil / [] (empty)
- The YAML marshaler may produce invalid YAML for these edge cases

## Solution Options

### Option 1: Ensure All Projects Have Release Entries (CHOSEN)
When adding a project, initialize it with an empty releases slice in the map:

```go
func (s *Store) AddProject(p Project) {
    s.mu.Lock()
    s.projects[p.ID] = p
    // Ensure project has a releases entry even if empty
    if _, exists := s.releases[p.ID]; !exists {
        s.releases[p.ID] = []Release{}
    }
    s.mu.Unlock()
    if err := s.SaveState(s.stateFile); err != nil {
        log.Printf("⚠ Failed to save state after AddProject: %v", err)
    }
}
```

### Option 2: Validate YAML Before Saving
Add validation to ensure the file is parseable after writing.

### Option 3: Switch to JSON Format
Use JSON instead of YAML for better reliability with edge cases.

### Option 4: Better Error Recovery
If LoadState() fails, don't immediately seed demo data. Instead, merge loaded projects with missing ones.

## Implementation

Apply Option 1 + Option 4 (hybrid approach):
1. Initialize empty releases slice for new projects
2. Enhance LoadState() error handling to preserve already-loaded projects
3. Only seed demo projects that don't already exist

## Changed Files

- `main.go`:
  - AddProject(): Initialize empty releases entry
  - LoadState(): Add detailed error logging
  - main(): Preserve loaded projects even if LoadState had issues

## Testing Required

1. Add custom project via API
2. Verify it saves to state.yaml
3. Verify YAML is valid (can parse manually)
4. Restart server
5. Verify custom project still appears in API
6. Repeat 5 more times to ensure no gradual corruption
