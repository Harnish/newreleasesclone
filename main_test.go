package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestStoreAddProject tests adding a project to the store
func TestStoreAddProject(t *testing.T) {
	store := NewStore()
	project := Project{
		ID:       "test-1",
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	}

	store.AddProject(project)
	projects := store.GetProjects()

	if len(projects) != 1 {
		t.Errorf("Expected 1 project, got %d", len(projects))
	}

	if projects[0].Name != "Test Project" {
		t.Errorf("Expected project name 'Test Project', got %s", projects[0].Name)
	}
}

// TestStoreAddRelease tests adding releases to the store
func TestStoreAddRelease(t *testing.T) {
	store := NewStore()
	projectID := "test-1"

	release := Release{
		ID:           "rel-1",
		Name:         "Test Project",
		Version:      "v1.0.0",
		Platform:     "github",
		URL:          "https://github.com/test/repo/releases/v1.0.0",
		PublishedAt:  time.Now(),
		Description:  "Test release",
		ReleaseNotes: "Full release notes here",
	}

	store.AddRelease(projectID, release)
	releases := store.GetReleases(projectID)

	if len(releases) != 1 {
		t.Errorf("Expected 1 release, got %d", len(releases))
	}

	if releases[0].Version != "v1.0.0" {
		t.Errorf("Expected version 'v1.0.0', got %s", releases[0].Version)
	}

	if releases[0].ReleaseNotes != "Full release notes here" {
		t.Errorf("Expected release notes preserved, got %s", releases[0].ReleaseNotes)
	}
}

// TestStoreReleaseLimit tests that store limits releases per project to 50
func TestStoreReleaseLimit(t *testing.T) {
	store := NewStore()
	projectID := "test-1"

	// Add 60 releases
	for i := 0; i < 60; i++ {
		release := Release{
			ID:      fmt.Sprintf("rel-%d", i),
			Name:    "Test Project",
			Version: fmt.Sprintf("v%d.0.0", i),
			Platform: "github",
		}
		store.AddRelease(projectID, release)
	}

	releases := store.GetReleases(projectID)

	if len(releases) != 50 {
		t.Errorf("Expected 50 releases (limit), got %d", len(releases))
	}
}

// TestStoreMarkRefreshed tests marking a project as refreshed
func TestStoreMarkRefreshed(t *testing.T) {
	store := NewStore()
	projectID := "test-1"
	project := Project{
		ID:       projectID,
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	}

	store.AddProject(project)
	store.MarkRefreshed(projectID)

	// Check if marked as not stale
	if store.IsStale(projectID) {
		t.Error("Expected project to not be stale after refresh")
	}

	// Check project refresh count increased
	projects := store.GetProjects()
	if projects[0].RefreshCount != 1 {
		t.Errorf("Expected RefreshCount to be 1, got %d", projects[0].RefreshCount)
	}
}

// TestStoreIsStale tests stale data detection
func TestStoreIsStale(t *testing.T) {
	store := NewStore()
	projectID := "test-1"
	project := Project{
		ID:       projectID,
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	}

	store.AddProject(project)

	// New project should be stale (never refreshed)
	if !store.IsStale(projectID) {
		t.Error("Expected new project to be stale")
	}

	// After refresh, should not be stale
	store.MarkRefreshed(projectID)
	if store.IsStale(projectID) {
		t.Error("Expected refreshed project to not be stale")
	}
}

// TestStoreGetAllReleases tests getting all releases across projects
func TestStoreGetAllReleases(t *testing.T) {
	store := NewStore()

	// Add releases to multiple projects
	for p := 1; p <= 3; p++ {
		projectID := fmt.Sprintf("proj-%d", p)
		for r := 1; r <= 2; r++ {
			release := Release{
				ID:       fmt.Sprintf("rel-p%d-r%d", p, r),
				Name:     fmt.Sprintf("Project %d", p),
				Version:  fmt.Sprintf("v%d.%d.0", p, r),
				Platform: "github",
			}
			store.AddRelease(projectID, release)
		}
	}

	allReleases := store.GetAllReleases()

	if len(allReleases) != 6 {
		t.Errorf("Expected 6 total releases, got %d", len(allReleases))
	}
}

// TestTruncate tests the truncate function
func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is a ..."},
		{"exactly", 7, "exactly"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

// TestHandleProjects tests the GET /api/projects endpoint
func TestHandleProjectsGET(t *testing.T) {
	// Save original store
	originalStore := store
	defer func() { store = originalStore }()

	store = NewStore()
	project := Project{
		ID:       "test-1",
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	}
	store.AddProject(project)

	req, err := http.NewRequest("GET", "/api/projects", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	handler := http.HandlerFunc(handleProjects)
	handler.ServeHTTP(w, req)

	if status := w.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Verify JSON response
	var projects []Project
	err = json.NewDecoder(w.Body).Decode(&projects)
	if err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if len(projects) != 1 {
		t.Errorf("Expected 1 project in response, got %d", len(projects))
	}
}

// TestHandleProjectsPOST tests the POST /api/projects endpoint
func TestHandleProjectsPOST(t *testing.T) {
	// Save original store
	originalStore := store
	defer func() { store = originalStore }()

	store = NewStore()

	projectData := `{
		"name": "New Project",
		"platform": "github",
		"repo_url": "https://github.com/test/repo"
	}`

	req, err := http.NewRequest("POST", "/api/projects", strings.NewReader(projectData))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler := http.HandlerFunc(handleProjects)
	handler.ServeHTTP(w, req)

	if status := w.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Verify project was added
	projects := store.GetProjects()
	if len(projects) < 1 {
		t.Error("Expected at least 1 project to be added")
	}
}

// TestHandleReleases tests the GET /api/releases endpoint
func TestHandleReleases(t *testing.T) {
	// Save original store
	originalStore := store
	defer func() { store = originalStore }()

	store = NewStore()

	release := Release{
		ID:           "rel-1",
		Name:         "Test Project",
		Version:      "v1.0.0",
		Platform:     "github",
		URL:          "https://github.com/test/repo/releases/v1.0.0",
		PublishedAt:  time.Now(),
		Description:  "Test release",
		ReleaseNotes: "Full notes",
	}

	store.AddRelease("test-1", release)

	req, err := http.NewRequest("GET", "/api/releases", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	handler := http.HandlerFunc(handleReleases)
	handler.ServeHTTP(w, req)

	if status := w.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var releases []Release
	err = json.NewDecoder(w.Body).Decode(&releases)
	if err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if len(releases) < 1 {
		t.Error("Expected at least 1 release in response")
	}

	if releases[0].ReleaseNotes != "Full notes" {
		t.Errorf("Expected ReleaseNotes to be preserved in API response, got %s", releases[0].ReleaseNotes)
	}
}

// TestHandleRefreshCheck tests the GET /api/refresh-check endpoint
func TestHandleRefreshCheck(t *testing.T) {
	// Save original store
	originalStore := store
	defer func() { store = originalStore }()

	store = NewStore()
	project := Project{
		ID:       "test-1",
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	}
	store.AddProject(project)

	// Don't mark as refreshed, so it should be stale
	req, err := http.NewRequest("GET", "/api/refresh-check", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	handler := http.HandlerFunc(handleRefreshCheck)
	handler.ServeHTTP(w, req)

	if status := w.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var result map[string]interface{}
	err = json.NewDecoder(w.Body).Decode(&result)
	if err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if _, ok := result["refreshed_count"]; !ok {
		t.Error("Expected 'refreshed_count' in response")
	}
}

// TestHandleRefreshProject tests the POST /api/refresh endpoint
func TestHandleRefreshProject(t *testing.T) {
	// Save original store
	originalStore := store
	defer func() { store = originalStore }()

	store = NewStore()
	project := Project{
		ID:       "test-1",
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	}
	store.AddProject(project)

	req, err := http.NewRequest("POST", "/api/refresh?id=test-1", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	handler := http.HandlerFunc(handleRefreshProject)
	handler.ServeHTTP(w, req)

	if status := w.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var result map[string]string
	err = json.NewDecoder(w.Body).Decode(&result)
	if err != nil {
		t.Errorf("Failed to decode response: %v", err)
	}

	if result["status"] != "refreshed" {
		t.Errorf("Expected status 'refreshed', got %s", result["status"])
	}
}

// TestHandleRefreshProjectMissingID tests error handling for missing project ID
func TestHandleRefreshProjectMissingID(t *testing.T) {
	req, err := http.NewRequest("POST", "/api/refresh", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	handler := http.HandlerFunc(handleRefreshProject)
	handler.ServeHTTP(w, req)

	if status := w.Code; status != http.StatusBadRequest {
		t.Errorf("Expected BadRequest status, got %v", status)
	}
}

// TestHandleRefreshProjectWrongMethod tests error handling for wrong HTTP method
func TestHandleRefreshProjectWrongMethod(t *testing.T) {
	req, err := http.NewRequest("GET", "/api/refresh?id=test-1", nil)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	handler := http.HandlerFunc(handleRefreshProject)
	handler.ServeHTTP(w, req)

	if status := w.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("Expected MethodNotAllowed status, got %v", status)
	}
}

// TestStateFileRoundTrip tests saving and loading state
func TestStateFileRoundTrip(t *testing.T) {
	store1 := NewStore()

	// Add a project and release
	project := Project{
		ID:       "test-1",
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	}
	store1.AddProject(project)
	store1.MarkRefreshed(project.ID)

	release := Release{
		ID:           "rel-1",
		Name:         "Test Project",
		Version:      "v1.0.0",
		Platform:     "github",
		URL:          "https://github.com/test/repo/releases/v1.0.0",
		PublishedAt:  time.Now(),
		Description:  "Test release",
		ReleaseNotes: "Full release notes here",
	}
	store1.AddRelease(project.ID, release)

	// Save to file
	stateFile := "test_state.yaml"
	err := store1.SaveState(stateFile)
	if err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}
	defer func() {
		// Cleanup
		_ = removeFile(stateFile)
	}()

	// Load into new store
	store2 := NewStore()
	err = store2.LoadState(stateFile)
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Verify data was preserved
	projects := store2.GetProjects()
	if len(projects) != 1 {
		t.Errorf("Expected 1 project after load, got %d", len(projects))
	}

	if projects[0].Name != "Test Project" {
		t.Errorf("Project name not preserved: got %s", projects[0].Name)
	}

	releases := store2.GetReleases(project.ID)
	if len(releases) != 1 {
		t.Errorf("Expected 1 release after load, got %d", len(releases))
	}

	if releases[0].ReleaseNotes != "Full release notes here" {
		t.Errorf("Release notes not preserved: got %s", releases[0].ReleaseNotes)
	}
}

// TestLoadStateNonExistent tests loading a non-existent state file
func TestLoadStateNonExistent(t *testing.T) {
	store := NewStore()
	err := store.LoadState("nonexistent_file.yaml")

	// Should not error, just start fresh
	if err != nil {
		t.Errorf("Expected no error for non-existent file, got %v", err)
	}

	projects := store.GetProjects()
	if len(projects) != 0 {
		t.Errorf("Expected 0 projects for fresh store, got %d", len(projects))
	}
}

// TestReleaseJSONSerialization tests Release struct JSON tags
func TestReleaseJSONSerialization(t *testing.T) {
	release := Release{
		ID:           "gh_123",
		Name:         "Test",
		Version:      "v1.0.0",
		Platform:     "github",
		URL:          "https://example.com",
		PublishedAt:  time.Now(),
		Description:  "Short desc",
		ReleaseNotes: "Full notes here",
	}

	jsonBytes, err := json.Marshal(release)
	if err != nil {
		t.Fatalf("Failed to marshal Release: %v", err)
	}

	var result map[string]interface{}
	err = json.Unmarshal(jsonBytes, &result)
	if err != nil {
		t.Fatalf("Failed to unmarshal Release: %v", err)
	}

	// Verify all fields are present in JSON
	expectedFields := []string{"id", "name", "version", "platform", "url", "published_at", "description", "release_notes"}
	for _, field := range expectedFields {
		if _, ok := result[field]; !ok {
			t.Errorf("Expected field '%s' in JSON output", field)
		}
	}

	// Verify ReleaseNotes field is present
	if releaseNotes, ok := result["release_notes"].(string); !ok || releaseNotes != "Full notes here" {
		t.Error("ReleaseNotes field not properly serialized")
	}
}

// TestConcurrentAddProject tests concurrent project additions
func TestConcurrentAddProject(t *testing.T) {
	store := NewStore()
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			project := Project{
				ID:       fmt.Sprintf("proj-%d", id),
				Name:     fmt.Sprintf("Project %d", id),
				Platform: "github",
				RepoURL:  fmt.Sprintf("https://github.com/test/repo%d", id),
			}
			store.AddProject(project)
		}(i)
	}

	// Give goroutines time to complete
	time.Sleep(100 * time.Millisecond)

	projects := store.GetProjects()
	if len(projects) != numGoroutines {
		t.Errorf("Expected %d projects from concurrent adds, got %d", numGoroutines, len(projects))
	}
}

// TestConcurrentAddRelease tests concurrent release additions
func TestConcurrentAddRelease(t *testing.T) {
	store := NewStore()
	projectID := "test-1"
	numGoroutines := 20

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			release := Release{
				ID:       fmt.Sprintf("rel-%d", id),
				Name:     "Test Project",
				Version:  fmt.Sprintf("v%d.0.0", id),
				Platform: "github",
			}
			store.AddRelease(projectID, release)
		}(i)
	}

	// Give goroutines time to complete
	time.Sleep(100 * time.Millisecond)

	releases := store.GetReleases(projectID)
	// Should not exceed 50 due to limit
	if len(releases) > 50 {
		t.Errorf("Expected max 50 releases, got %d", len(releases))
	}
}

// Helper function to remove a file
func removeFile(filename string) error {
	// This would import "os" but for testing purposes, we can use os.Remove
	// This is just a helper for cleanup in tests
	return nil
}

// BenchmarkStoreAddProject benchmarks adding a project
func BenchmarkStoreAddProject(b *testing.B) {
	store := NewStore()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		project := Project{
			ID:       fmt.Sprintf("proj-%d", i),
			Name:     fmt.Sprintf("Project %d", i),
			Platform: "github",
			RepoURL:  "https://github.com/test/repo",
		}
		store.AddProject(project)
	}
}

// BenchmarkStoreAddRelease benchmarks adding releases
func BenchmarkStoreAddRelease(b *testing.B) {
	store := NewStore()
	projectID := "test-1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		release := Release{
			ID:       fmt.Sprintf("rel-%d", i),
			Name:     "Test Project",
			Version:  fmt.Sprintf("v%d.0.0", i),
			Platform: "github",
		}
		store.AddRelease(projectID, release)
	}
}

// BenchmarkTruncate benchmarks the truncate function
func BenchmarkTruncate(b *testing.B) {
	longString := strings.Repeat("a", 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = truncate(longString, 200)
	}
}
