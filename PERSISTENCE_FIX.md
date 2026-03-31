# Data Persistence Fix - Issue Analysis and Solution

## Root Cause

When adding a new project:
1. The project is added to the `Store.projects` map
2. `SaveState()` is called, which marshals ALL maps including empty ones
3. Empty release lists serialize as `releases: {}` in YAML
4. On restart, if there's a YAML parse error, LoadState() fails silently
5. The code then checks `len(store.GetProjects()) == 0` which is TRUE
6. Demo data is re-seeded, overwriting the custom project

## Problem with Current Approach

The synchronous save in AddProject() isn't enough because:
- If the Save fails, we don't handle it
- If the file gets partially written before the next change, corruption happens
- Demo projects and custom projects use different ID schemes ("1,2,3" vs "proj_nanosecond")

## Solution Implemented

### Changes Made to `main.go`:

1. **Enhanced Logging in main()** - Added debug messages to trace initialization flow
2. **Better Error Handling in LoadState()** - Initialize empty maps if nil after unmarshaling
3. **Synchronous Saves** - Changed AddProject(), AddRelease(), MarkRefreshed() from async to synchronous

### Verification Test:

```bash
# 1. Start fresh
rm state.yaml
./newreleases

# 2. Add custom project via API
curl -X POST http://localhost:8080/api/projects \
  -H "Content-Type: application/json" \
  -d '{"name":"test-repo","platform":"github","repo_url":"https://github.com/test/test"}'

# 3. Verify in state.yaml
grep "test-repo" state.yaml

# 4. Restart server
pkill -f ./newreleases
sleep 2
./newreleases

# 5. Verify project still exists
curl http://localhost:8080/api/projects | jq '.[] | .name'  # Should include "test-repo"
```

## Remaining Issue

The state.yaml file sometimes gets corrupted when:
- Releases are empty for a project (API returns 404)
- The YAML marshaler outputs empty maps as `{}`
- The YAML unmarshaler struggles with this format

## Next Steps

Option 1: Fix YAML marshaling to output proper YAML even for empty maps
Option 2: Ensure every project always has a releases entry (even if empty list)
Option 3: Use JSON instead of YAML (more reliable serialization)
Option 4: Add data validation after LoadState() to detect corruption and recover

## Files Modified

- `main.go`:
  - Lines 70-72: Synchronous save in AddProject()
  - Lines 86-93: Synchronous save in AddRelease()
  - Lines 127-134: Synchronous save in MarkRefreshed()
  - Lines 580-595: Enhanced nil handling in LoadState()
  - Lines 1115-1132: Enhanced logging in main()
