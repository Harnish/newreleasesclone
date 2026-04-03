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

// newTestStore creates an in-memory SQLite store for testing.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create test store: %v", err)
	}
	return s
}

// TestStoreAddProject tests adding a project to the store
func TestStoreAddProject(t *testing.T) {
	s := newTestStore(t)
	project := Project{
		ID:       "test-1",
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	}

	s.AddProject(project)
	projects := s.GetProjects()

	if len(projects) != 1 {
		t.Errorf("Expected 1 project, got %d", len(projects))
	}

	if projects[0].Name != "Test Project" {
		t.Errorf("Expected project name 'Test Project', got %s", projects[0].Name)
	}
}

// TestStoreAddRelease tests adding releases to the store
func TestStoreAddRelease(t *testing.T) {
	s := newTestStore(t)
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

	s.AddRelease(projectID, release)
	releases := s.GetReleases(projectID)

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

// TestStoreReleaseLimit tests that the store limits releases per project to releaseLimit
func TestStoreReleaseLimit(t *testing.T) {
	s := newTestStore(t)
	projectID := "test-1"

	for i := 0; i < releaseLimit+10; i++ {
		s.AddRelease(projectID, Release{
			ID:          fmt.Sprintf("rel-%d", i),
			Name:        "Test Project",
			Version:     fmt.Sprintf("v%d.0.0", i),
			Platform:    "github",
			PublishedAt: time.Now().Add(time.Duration(i) * time.Second),
		})
	}

	releases := s.GetReleases(projectID)
	if len(releases) != releaseLimit {
		t.Errorf("Expected %d releases (limit), got %d", releaseLimit, len(releases))
	}
}

// TestStoreMarkRefreshed tests marking a project as refreshed
func TestStoreMarkRefreshed(t *testing.T) {
	s := newTestStore(t)
	projectID := "test-1"
	project := Project{
		ID:       projectID,
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	}

	s.AddProject(project)
	s.MarkRefreshed(projectID)

	if s.IsStale(projectID) {
		t.Error("Expected project to not be stale after refresh")
	}

	projects := s.GetProjects()
	if projects[0].RefreshCount != 1 {
		t.Errorf("Expected RefreshCount to be 1, got %d", projects[0].RefreshCount)
	}
}

// TestStoreIsStale tests stale data detection
func TestStoreIsStale(t *testing.T) {
	s := newTestStore(t)
	projectID := "test-1"
	project := Project{
		ID:       projectID,
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	}

	s.AddProject(project)

	if !s.IsStale(projectID) {
		t.Error("Expected new project to be stale")
	}

	s.MarkRefreshed(projectID)
	if s.IsStale(projectID) {
		t.Error("Expected refreshed project to not be stale")
	}
}

// TestStoreGetAllReleases tests getting all releases across projects
func TestStoreGetAllReleases(t *testing.T) {
	s := newTestStore(t)

	for p := 1; p <= 3; p++ {
		projectID := fmt.Sprintf("proj-%d", p)
		s.AddProject(Project{
			ID:       projectID,
			Name:     fmt.Sprintf("Project %d", p),
			Platform: "github",
			RepoURL:  "https://github.com/test/repo",
		})
		for r := 1; r <= 2; r++ {
			s.AddRelease(projectID, Release{
				ID:       fmt.Sprintf("rel-p%d-r%d", p, r),
				Name:     fmt.Sprintf("Project %d", p),
				Version:  fmt.Sprintf("v%d.%d.0", p, r),
				Platform: "github",
			})
		}
	}

	allReleases := s.GetAllReleases()
	if len(allReleases) != 6 {
		t.Errorf("Expected 6 total releases, got %d", len(allReleases))
	}
}

// TestStorePersistence tests that data survives closing and reopening the DB
func TestStorePersistence(t *testing.T) {
	dbPath := t.TempDir() + "/test.db"

	s1, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	project := Project{
		ID:       "test-1",
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	}
	s1.AddProject(project)
	s1.MarkRefreshed(project.ID)
	s1.AddRelease(project.ID, Release{
		ID:           "rel-1",
		Name:         "Test Project",
		Version:      "v1.0.0",
		Platform:     "github",
		URL:          "https://github.com/test/repo/releases/v1.0.0",
		PublishedAt:  time.Now(),
		ReleaseNotes: "Full release notes here",
	})
	s1.db.Close()

	s2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to reopen store: %v", err)
	}

	projects := s2.GetProjects()
	if len(projects) != 1 {
		t.Errorf("Expected 1 project after reopen, got %d", len(projects))
	}
	if projects[0].Name != "Test Project" {
		t.Errorf("Project name not preserved: got %s", projects[0].Name)
	}

	releases := s2.GetReleases(project.ID)
	if len(releases) != 1 {
		t.Errorf("Expected 1 release after reopen, got %d", len(releases))
	}
	if releases[0].ReleaseNotes != "Full release notes here" {
		t.Errorf("Release notes not preserved: got %s", releases[0].ReleaseNotes)
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

// TestHandleProjectsGET tests the GET /api/projects endpoint
func TestHandleProjectsGET(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()

	store = newTestStore(t)
	store.AddProject(Project{
		ID:       "test-1",
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})

	req, _ := http.NewRequest("GET", "/api/projects", nil)
	w := httptest.NewRecorder()
	http.HandlerFunc(handleProjects).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %v", w.Code)
	}

	var projects []Project
	if err := json.NewDecoder(w.Body).Decode(&projects); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("Expected 1 project in response, got %d", len(projects))
	}
}

// TestHandleProjectsPOST tests the POST /api/projects endpoint
func TestHandleProjectsPOST(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()

	store = newTestStore(t)

	body := `{"name":"New Project","platform":"github","repo_url":"https://github.com/test/repo"}`
	req, _ := http.NewRequest("POST", "/api/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	http.HandlerFunc(handleProjects).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %v", w.Code)
	}
	if len(store.GetProjects()) < 1 {
		t.Error("Expected at least 1 project to be added")
	}
}

// TestHandleReleases tests the GET /api/releases endpoint
func TestHandleReleases(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()

	store = newTestStore(t)
	store.AddProject(Project{
		ID:       "test-1",
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	store.AddRelease("test-1", Release{
		ID:           "rel-1",
		Name:         "Test Project",
		Version:      "v1.0.0",
		Platform:     "github",
		URL:          "https://github.com/test/repo/releases/v1.0.0",
		PublishedAt:  time.Now(),
		Description:  "Test release",
		ReleaseNotes: "Full notes",
	})

	req, _ := http.NewRequest("GET", "/api/releases", nil)
	w := httptest.NewRecorder()
	http.HandlerFunc(handleReleases).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %v", w.Code)
	}

	var releases []Release
	if err := json.NewDecoder(w.Body).Decode(&releases); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(releases) < 1 {
		t.Error("Expected at least 1 release in response")
	}
	if releases[0].ReleaseNotes != "Full notes" {
		t.Errorf("Expected ReleaseNotes preserved, got %s", releases[0].ReleaseNotes)
	}
}

// TestHandleRefreshCheck tests the GET /api/refresh-check endpoint
func TestHandleRefreshCheck(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()

	store = newTestStore(t)
	store.AddProject(Project{
		ID:       "test-1",
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})

	req, _ := http.NewRequest("GET", "/api/refresh-check", nil)
	w := httptest.NewRecorder()
	http.HandlerFunc(handleRefreshCheck).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %v", w.Code)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if _, ok := result["refreshed_count"]; !ok {
		t.Error("Expected 'refreshed_count' in response")
	}
}

// TestHandleRefreshProject tests the POST /api/refresh endpoint
func TestHandleRefreshProject(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()

	store = newTestStore(t)
	store.AddProject(Project{
		ID:       "test-1",
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})

	req, _ := http.NewRequest("POST", "/api/refresh?id=test-1", nil)
	w := httptest.NewRecorder()
	http.HandlerFunc(handleRefreshProject).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %v", w.Code)
	}

	var result map[string]string
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["status"] != "refreshed" {
		t.Errorf("Expected status 'refreshed', got %s", result["status"])
	}
}

// TestHandleRefreshProjectMissingID tests error handling for missing project ID
func TestHandleRefreshProjectMissingID(t *testing.T) {
	req, _ := http.NewRequest("POST", "/api/refresh", nil)
	w := httptest.NewRecorder()
	http.HandlerFunc(handleRefreshProject).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected BadRequest, got %v", w.Code)
	}
}

// TestHandleRefreshProjectWrongMethod tests error handling for wrong HTTP method
func TestHandleRefreshProjectWrongMethod(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/refresh?id=test-1", nil)
	w := httptest.NewRecorder()
	http.HandlerFunc(handleRefreshProject).ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected MethodNotAllowed, got %v", w.Code)
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
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		t.Fatalf("Failed to unmarshal Release: %v", err)
	}

	for _, field := range []string{"id", "name", "version", "platform", "url", "published_at", "description", "release_notes"} {
		if _, ok := result[field]; !ok {
			t.Errorf("Expected field '%s' in JSON output", field)
		}
	}

	if notes, ok := result["release_notes"].(string); !ok || notes != "Full notes here" {
		t.Error("ReleaseNotes field not properly serialized")
	}
}

// TestConcurrentAddProject tests concurrent project additions
func TestConcurrentAddProject(t *testing.T) {
	s := newTestStore(t)
	numGoroutines := 10

	done := make(chan struct{}, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			s.AddProject(Project{
				ID:       fmt.Sprintf("proj-%d", id),
				Name:     fmt.Sprintf("Project %d", id),
				Platform: "github",
				RepoURL:  fmt.Sprintf("https://github.com/test/repo%d", id),
			})
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	projects := s.GetProjects()
	if len(projects) != numGoroutines {
		t.Errorf("Expected %d projects from concurrent adds, got %d", numGoroutines, len(projects))
	}
}

// TestConcurrentAddRelease tests concurrent release additions
func TestConcurrentAddRelease(t *testing.T) {
	s := newTestStore(t)
	projectID := "test-1"
	numGoroutines := 20

	done := make(chan struct{}, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			s.AddRelease(projectID, Release{
				ID:       fmt.Sprintf("rel-%d", id),
				Name:     "Test Project",
				Version:  fmt.Sprintf("v%d.0.0", id),
				Platform: "github",
			})
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	releases := s.GetReleases(projectID)
	if len(releases) > releaseLimit {
		t.Errorf("Expected max %d releases, got %d", releaseLimit, len(releases))
	}
}

// BenchmarkStoreAddProject benchmarks adding a project
func BenchmarkStoreAddProject(b *testing.B) {
	s, _ := NewStore(":memory:")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.AddProject(Project{
			ID:       fmt.Sprintf("proj-%d", i),
			Name:     fmt.Sprintf("Project %d", i),
			Platform: "github",
			RepoURL:  "https://github.com/test/repo",
		})
	}
}

// BenchmarkStoreAddRelease benchmarks adding releases
func BenchmarkStoreAddRelease(b *testing.B) {
	s, _ := NewStore(":memory:")
	projectID := "test-1"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.AddRelease(projectID, Release{
			ID:       fmt.Sprintf("rel-%d", i),
			Name:     "Test Project",
			Version:  fmt.Sprintf("v%d.0.0", i),
			Platform: "github",
		})
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
