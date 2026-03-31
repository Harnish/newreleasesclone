# Release Sorting & Limit Update

## Changes Made

### 1. Release Limit Changed from 50 to 10
**File**: `main.go`, Line 85

**Change**:
```go
// Before
if len(s.releases[projectID]) > 50 {
    s.releases[projectID] = s.releases[projectID][:50]
}

// After
if len(s.releases[projectID]) > 10 {
    s.releases[projectID] = s.releases[projectID][:10]
}
```

Each project now stores a maximum of **10 latest versions** instead of 50.

### 2. Releases Sorted by Date (Newest First)
**File**: `main.go`, Lines 89-101 (GetReleases method)

**Change**:
```go
// Before
func (s *Store) GetReleases(projectID string) []Release {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.releases[projectID]
}

// After
func (s *Store) GetReleases(projectID string) []Release {
    s.mu.RLock()
    rs := make([]Release, len(s.releases[projectID]))
    copy(rs, s.releases[projectID])
    s.mu.RUnlock()
    
    // Sort by published date (newest first)
    sort.Slice(rs, func(i, j int) bool {
        return rs[i].PublishedAt.After(rs[j].PublishedAt)
    })
    return rs
}
```

### 3. All Releases Sorted by Date (Newest First)
**File**: `main.go`, Lines 103-115 (GetAllReleases method)

**Change**:
```go
// Before
func (s *Store) GetAllReleases() []Release {
    s.mu.RLock()
    defer s.mu.RUnlock()
    var all []Release
    for _, rs := range s.releases {
        all = append(all, rs...)
    }
    return all
}

// After
func (s *Store) GetAllReleases() []Release {
    s.mu.RLock()
    var all []Release
    for _, rs := range s.releases {
        all = append(all, rs...)
    }
    s.mu.RUnlock()
    
    // Sort by published date (newest first)
    sort.Slice(all, func(i, j int) bool {
        return all[i].PublishedAt.After(all[j].PublishedAt)
    })
    return all
}
```

## Impact

### UI Behavior
- **Latest Releases Tab**: Now shows exactly 10 newest releases per project
- **Sorting**: All releases are sorted by publication date, newest at the top
- **Consistency**: Across all pages and API calls, releases appear in the same order

### Performance
- **Memory**: Reduced memory footprint (storing 10 instead of 50 per project)
- **Sort Time**: Minimal overhead (sorting only happens on read, not on write)
- **API Response**: Slightly faster responses due to smaller payloads

### Data Storage
- **state.yaml**: Smaller file size (fewer releases per project)
- **Persistence**: Auto-save operates on smaller dataset

## Testing

### Verification Steps
1. Build: `go build` ✅
2. Start server: `./newreleases`
3. Check API: `curl http://localhost:8080/api/releases | jq '.[] | {name, version, published_at}' | head -20`
4. Verify: First releases have latest `published_at` timestamps
5. Count: Verify each project has at most 10 releases

### Expected Results
- Newest releases appear at the top of the list
- Each project has maximum 10 versions
- Sorting is consistent across all API calls and UI displays
- Time-since calculation works correctly on sorted dates

## Backward Compatibility

### Changes to Existing Data
- If `state.yaml` has >10 releases per project, they will be trimmed on next refresh
- Sorting applies on read, so old data will be re-sorted
- No data corruption, just pruning of older releases

### API Compatibility
- Response format unchanged (still returns array of Release objects)
- Endpoint paths unchanged (`/api/releases`, `/api/projects`)
- Frontend code unchanged (already handles variable-length arrays)

## Documentation Updates

Updated references in:
- `.claude` (release limits documentation)
- IMPLEMENTATION_SUMMARY.md (Store architecture)
- README.md (Feature list)
- UI_RESTRUCTURE.md (UI display limits)

## Deployment Notes

### For Existing Installations
1. Backup current `state.yaml`
2. Deploy new binary (either rebuild from source or receive new binary)
3. Restart service
4. Data will be automatically trimmed to 10 per project on next refresh

### No Data Loss
- Trimming only affects display/storage
- Original API data still available in GitHub, NPM, PyPI (can re-fetch anytime)
- Backups recommended but not critical for this change

## Related Features

This update complements the existing features:
- **3-Trigger Refresh System**: Still works, now with cleaner dataset
- **Time-Since Display**: Now shows only 10 most relevant releases
- **Manual Refresh**: Button still functional, manages smaller dataset
- **YAML Persistence**: More efficient with smaller files
- **Collapsible UI**: Lists now top-10 versions instead of top-50
