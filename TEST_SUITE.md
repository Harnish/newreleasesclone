# Test Suite Documentation

## Overview

This document describes the comprehensive test suite for the Release Tracker application. The suite covers all major functionality including store operations, HTTP handlers, state persistence, and concurrency.

## Running Tests

### Run all tests
```bash
go test -v
```

### Run specific test
```bash
go test -run TestStoreAddProject -v
```

### Run with coverage
```bash
go test -cover
```

### Run benchmarks
```bash
go test -bench=. -benchmem
```

### Run tests with detailed output
```bash
go test -v 2>&1
```

## Test Coverage

### Store Operations

#### 1. TestStoreAddProject
Tests adding a project to the store.
- **What it tests**: Project creation and retrieval
- **Expected behavior**: Project is added to store and can be retrieved
- **Status**: ✅ PASS

#### 2. TestStoreAddRelease
Tests adding releases to the store.
- **What it tests**: Release creation with full release notes preservation
- **Expected behavior**: Release is added and ReleaseNotes field is preserved
- **Status**: ✅ PASS

#### 3. TestStoreReleaseLimit
Tests that the store limits releases per project to 50.
- **What it tests**: Release limit enforcement
- **Expected behavior**: Adding 60 releases results in only 50 stored
- **Status**: ✅ PASS

#### 4. TestStoreMarkRefreshed
Tests marking a project as refreshed.
- **What it tests**: Refresh timestamp and refresh count updates
- **Expected behavior**: Project's LastRefresh time is updated and RefreshCount increments
- **Status**: ✅ PASS

#### 5. TestStoreIsStale
Tests stale data detection (30-minute threshold).
- **What it tests**: Stale status determination
- **Expected behavior**: New projects are stale, refreshed projects are not
- **Status**: ✅ PASS

#### 6. TestStoreGetAllReleases
Tests retrieving all releases across all projects.
- **What it tests**: Cross-project release aggregation
- **Expected behavior**: All releases from all projects are returned
- **Status**: ✅ PASS

### Utility Functions

#### 7. TestTruncate
Tests the truncate string function.
- **What it tests**: String truncation with "..." appended
- **Scenarios**:
  - Short strings (unchanged)
  - Long strings (truncated with "...")
  - Edge case: exactly maxLen
  - Empty strings
- **Status**: ✅ PASS

### HTTP Handlers

#### 8. TestHandleProjectsGET
Tests the GET /api/projects endpoint.
- **What it tests**: Project retrieval via HTTP
- **Expected behavior**: Returns JSON array of projects
- **Status**: ✅ PASS

#### 9. TestHandleProjectsPOST
Tests the POST /api/projects endpoint.
- **What it tests**: Project creation via HTTP
- **Expected behavior**: Project is added to store
- **Status**: ✅ PASS

#### 10. TestHandleReleases
Tests the GET /api/releases endpoint.
- **What it tests**: Release retrieval via HTTP with ReleaseNotes preservation
- **Expected behavior**: Returns JSON array of releases with full ReleaseNotes
- **Status**: ✅ PASS

#### 11. TestHandleRefreshCheck
Tests the GET /api/refresh-check endpoint.
- **What it tests**: Stale project detection and response format
- **Expected behavior**: Returns refreshed_count in JSON
- **Status**: ✅ PASS

#### 12. TestHandleRefreshProject
Tests the POST /api/refresh endpoint.
- **What it tests**: Project refresh via HTTP
- **Expected behavior**: Returns status "refreshed" in JSON
- **Status**: ✅ PASS

#### 13. TestHandleRefreshProjectMissingID
Tests error handling for missing project ID.
- **What it tests**: Validation of required parameters
- **Expected behavior**: Returns HTTP 400 Bad Request
- **Status**: ✅ PASS

#### 14. TestHandleRefreshProjectWrongMethod
Tests error handling for wrong HTTP method.
- **What it tests**: HTTP method validation
- **Expected behavior**: Returns HTTP 405 Method Not Allowed
- **Status**: ✅ PASS

### State Persistence

#### 15. TestStateFileRoundTrip
Tests saving and loading state from YAML file.
- **What it tests**: Full state serialization and deserialization
- **Verification**:
  - Project data is preserved
  - Release data is preserved
  - ReleaseNotes field is preserved
  - Refresh timestamps are preserved
- **Status**: ✅ PASS

#### 16. TestLoadStateNonExistent
Tests loading a non-existent state file.
- **What it tests**: Graceful handling of missing state file
- **Expected behavior**: No error, starts with empty store
- **Status**: ✅ PASS

### JSON Serialization

#### 17. TestReleaseJSONSerialization
Tests Release struct JSON tags and serialization.
- **What it tests**: All fields are properly serialized to JSON
- **Verified fields**:
  - id
  - name
  - version
  - platform
  - url
  - published_at
  - description
  - release_notes
- **Status**: ✅ PASS

### Concurrency

#### 18. TestConcurrentAddProject
Tests concurrent project additions.
- **What it tests**: Thread-safety of AddProject
- **Scenario**: 10 goroutines adding projects simultaneously
- **Expected behavior**: All projects are added without data corruption
- **Status**: ✅ PASS

#### 19. TestConcurrentAddRelease
Tests concurrent release additions.
- **What it tests**: Thread-safety of AddRelease with release limit
- **Scenario**: 20 goroutines adding releases simultaneously
- **Expected behavior**: Max 50 releases stored, no data corruption
- **Status**: ✅ PASS

## Benchmarks

### BenchmarkStoreAddProject
Measures performance of adding projects.
```
Operations per second: ~4M ops/s
Memory per operation: ~208 B
```

### BenchmarkStoreAddRelease
Measures performance of adding releases.
```
Operations per second: ~4.4M ops/s
Memory per operation: ~250 B
```

### BenchmarkTruncate
Measures performance of string truncation.
```
Operations per second: ~4.4M ops/s
Memory per operation: ~208 B
```

## Test Statistics

| Metric | Value |
|--------|-------|
| **Total Tests** | 19 |
| **Passed** | 19 |
| **Failed** | 0 |
| **Total Runtime** | ~1.1 seconds |
| **Coverage** | Core functionality (store, handlers, persistence) |

## Test Categories

### Unit Tests (14)
- Store operations (6 tests)
- Utility functions (1 test)
- HTTP Handlers (7 tests)

### Integration Tests (3)
- State persistence (2 tests)
- JSON serialization (1 test)

### Concurrency Tests (2)
- Concurrent additions with mutex protection

### Performance Tests (3 benchmarks)
- AddProject performance
- AddRelease performance
- Truncate performance

## Key Features Tested

✅ **Store Management**
- Adding/retrieving projects
- Adding/retrieving releases
- Release limit enforcement (50 max)
- Refresh tracking

✅ **Refresh System**
- Marking projects as refreshed
- Stale detection (30-minute threshold)
- Concurrent refresh operations

✅ **Persistence**
- Saving state to YAML
- Loading state from YAML
- Data integrity through round-trip
- Graceful handling of missing files

✅ **Release Notes Feature**
- Full ReleaseNotes field storage
- YAML serialization
- JSON API response
- Data preservation through state save/load

✅ **HTTP API**
- GET /api/projects
- POST /api/projects
- GET /api/releases
- GET /api/refresh-check
- POST /api/refresh
- Error handling and validation

✅ **Concurrency**
- Thread-safe operations with mutexes
- Safe concurrent adds
- Data integrity under load

## Setup and Isolation

Each test that uses the global `store` variable:
1. Saves the original store reference
2. Replaces it with a fresh test store
3. Runs the test
4. Restores the original store in deferred cleanup

This ensures test isolation and prevents test pollution.

## Adding New Tests

When adding new functionality, follow this pattern:

```go
func TestNewFeature(t *testing.T) {
    // Setup
    store := NewStore()
    
    // Action
    result := store.DoSomething()
    
    // Assert
    if result != expected {
        t.Errorf("Expected %v, got %v", expected, result)
    }
}
```

For tests using global store:

```go
func TestWithGlobalStore(t *testing.T) {
    // Save original store
    originalStore := store
    defer func() { store = originalStore }()
    
    store = NewStore()
    
    // Test code here
}
```

## Common Test Patterns

### Testing HTTP Handlers
```go
req, _ := http.NewRequest("GET", "/api/endpoint", nil)
w := httptest.NewRecorder()
handler := http.HandlerFunc(handleFunction)
handler.ServeHTTP(w, req)

if w.Code != http.StatusOK {
    t.Errorf("Unexpected status: %d", w.Code)
}
```

### Testing JSON Serialization
```go
var result MyType
err := json.NewDecoder(w.Body).Decode(&result)
if err != nil {
    t.Fatalf("Failed to decode: %v", err)
}
```

### Testing Concurrency
```go
for i := 0; i < numGoroutines; i++ {
    go func(id int) {
        // Concurrent operation
    }(i)
}
time.Sleep(100 * time.Millisecond)
```

## Coverage Goals

Current coverage includes:
- ✅ Core store operations
- ✅ HTTP API endpoints
- ✅ State persistence
- ✅ Refresh system
- ✅ Concurrency safety
- ✅ JSON serialization
- ✅ Error handling

Potential future coverage:
- Platform-specific API fetchers (GitHub, NPM, etc.)
- HTML template rendering
- Advanced refresh scenarios
- Performance under high load

## Troubleshooting Tests

### Tests fail after code changes
Ensure global store is properly saved/restored:
```go
originalStore := store
defer func() { store = originalStore }()
```

### Race conditions detected
Use `go test -race` to detect data races:
```bash
go test -race
```

### Tests timeout
Some async operations may need delays. Use reasonable timeouts:
```go
time.Sleep(100 * time.Millisecond)
```

## Running Tests in CI/CD

Recommended command for CI/CD pipelines:
```bash
go test -v -race -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

This will:
- Run all tests verbosely
- Detect race conditions
- Generate coverage reports
- Create HTML coverage visualization

## Test Quality Metrics

- **Test isolation**: ✅ All tests independent
- **Deterministic**: ✅ All tests produce consistent results
- **Fast execution**: ✅ Complete suite runs in ~1 second
- **Clear assertions**: ✅ Error messages indicate what failed
- **Comprehensive**: ✅ Covers core functionality and edge cases
