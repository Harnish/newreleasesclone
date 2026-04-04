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

// newTestAuth creates a user + session in s and returns the userID and session cookie.
func newTestAuth(t *testing.T, s *Store) (string, *http.Cookie) {
	t.Helper()
	user, err := s.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	sessionID := s.CreateSession(user.ID)
	cookie := &http.Cookie{Name: "session", Value: sessionID}
	return user.ID, cookie
}

// TestStoreAddRepo tests adding a repo to the store
func TestStoreAddRepo(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	repoID, err := s.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}
	if repoID == "" {
		t.Fatal("expected non-empty repoID")
	}

	projects := s.GetUserRepos(userID)
	if len(projects) != 1 {
		t.Errorf("Expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "Test Project" {
		t.Errorf("Expected project name 'Test Project', got %s", projects[0].Name)
	}
}

// TestStoreAddRepoDedupe tests that two users adding the same repo share one record
func TestStoreAddRepoDedupe(t *testing.T) {
	s := newTestStore(t)

	user1, _ := s.CreateUser("user1", "password123")
	user2, _ := s.CreateUser("user2", "password123")

	id1, err := s.AddRepo(user1.ID, Project{Name: "React", Platform: "npm", RepoURL: "react"})
	if err != nil {
		t.Fatalf("AddRepo user1 failed: %v", err)
	}
	id2, err := s.AddRepo(user2.ID, Project{Name: "React", Platform: "npm", RepoURL: "react"})
	if err != nil {
		t.Fatalf("AddRepo user2 failed: %v", err)
	}

	if id1 != id2 {
		t.Errorf("Expected shared repo ID, got %s and %s", id1, id2)
	}
	if len(s.GetProjects()) != 1 {
		t.Errorf("Expected 1 shared repo, got %d", len(s.GetProjects()))
	}
}

// TestStoreAddRelease tests adding releases to the store
func TestStoreAddRelease(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	repoID, err := s.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	s.AddRelease(repoID, Release{
		ID:           "rel-1",
		Name:         "Test Project",
		Version:      "v1.0.0",
		Platform:     "github",
		URL:          "https://github.com/test/repo/releases/v1.0.0",
		PublishedAt:  time.Now(),
		Description:  "Test release",
		ReleaseNotes: "Full release notes here",
	})

	releases := s.GetReleases(repoID)
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
	userID, _ := newTestAuth(t, s)

	repoID, err := s.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	for i := 0; i < releaseLimit+10; i++ {
		s.AddRelease(repoID, Release{
			ID:          fmt.Sprintf("rel-%d", i),
			Name:        "Test Project",
			Version:     fmt.Sprintf("v%d.0.0", i),
			Platform:    "github",
			PublishedAt: time.Now().Add(time.Duration(i) * time.Second),
		})
	}

	releases := s.GetReleases(repoID)
	if len(releases) != releaseLimit {
		t.Errorf("Expected %d releases (limit), got %d", releaseLimit, len(releases))
	}
}

// TestStoreMarkRefreshed tests marking a project as refreshed
func TestStoreMarkRefreshed(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	repoID, err := s.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	s.MarkRefreshed(repoID)

	if s.IsStale(repoID) {
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
	userID, _ := newTestAuth(t, s)

	repoID, err := s.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	if !s.IsStale(repoID) {
		t.Error("Expected new project to be stale")
	}

	s.MarkRefreshed(repoID)
	if s.IsStale(repoID) {
		t.Error("Expected refreshed project to not be stale")
	}
}

// TestStoreGetAllReleases tests getting all releases across projects
func TestStoreGetAllReleases(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	for p := 1; p <= 3; p++ {
		repoID, err := s.AddRepo(userID, Project{
			Name:     fmt.Sprintf("Project %d", p),
			Platform: "github",
			RepoURL:  fmt.Sprintf("https://github.com/test/repo%d", p),
		})
		if err != nil {
			t.Fatalf("AddRepo failed: %v", err)
		}
		for r := 1; r <= 2; r++ {
			s.AddRelease(repoID, Release{
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

	user, err := s1.CreateUser("persist_user", "password123")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	repoID, err := s1.AddRepo(user.ID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	s1.MarkRefreshed(repoID)
	s1.AddRelease(repoID, Release{
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

	releases := s2.GetReleases(repoID)
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
	userID, cookie := newTestAuth(t, store)

	if _, err := store.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	}); err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	req, _ := http.NewRequest("GET", "/api/projects", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleProjects).ServeHTTP(w, req)

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

// TestHandleProjectsGETUnauth tests that unauthenticated requests get 401
func TestHandleProjectsGETUnauth(t *testing.T) {
	req, _ := http.NewRequest("GET", "/api/projects", nil)
	w := httptest.NewRecorder()
	requireAuth(handleProjects).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %v", w.Code)
	}
}

// TestHandleProjectsPOST tests the POST /api/projects endpoint
func TestHandleProjectsPOST(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()

	store = newTestStore(t)
	_, cookie := newTestAuth(t, store)

	body := `{"name":"New Project","platform":"github","repo_url":"https://github.com/test/repo"}`
	req, _ := http.NewRequest("POST", "/api/projects", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleProjects).ServeHTTP(w, req)

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
	userID, cookie := newTestAuth(t, store)

	repoID, err := store.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}
	store.AddRelease(repoID, Release{
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
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleReleases).ServeHTTP(w, req)

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
	userID, cookie := newTestAuth(t, store)

	if _, err := store.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	}); err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	req, _ := http.NewRequest("GET", "/api/refresh-check", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleRefreshCheck).ServeHTTP(w, req)

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
	userID, cookie := newTestAuth(t, store)

	repoID, err := store.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	req, _ := http.NewRequest("POST", "/api/refresh?id="+repoID, nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleRefreshProject).ServeHTTP(w, req)

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
	originalStore := store
	defer func() { store = originalStore }()

	store = newTestStore(t)
	_, cookie := newTestAuth(t, store)

	req, _ := http.NewRequest("POST", "/api/refresh", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleRefreshProject).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected BadRequest, got %v", w.Code)
	}
}

// TestHandleRefreshProjectWrongMethod tests error handling for wrong HTTP method
func TestHandleRefreshProjectWrongMethod(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()

	store = newTestStore(t)
	_, cookie := newTestAuth(t, store)

	req, _ := http.NewRequest("GET", "/api/refresh?id=test-1", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleRefreshProject).ServeHTTP(w, req)

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

// TestConcurrentAddRepo tests concurrent repo additions
func TestConcurrentAddRepo(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)
	numGoroutines := 10

	done := make(chan struct{}, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			s.AddRepo(userID, Project{
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

	projects := s.GetUserRepos(userID)
	if len(projects) != numGoroutines {
		t.Errorf("Expected %d projects from concurrent adds, got %d", numGoroutines, len(projects))
	}
}

// TestConcurrentAddRelease tests concurrent release additions
func TestConcurrentAddRelease(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	repoID, err := s.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	numGoroutines := 20
	done := make(chan struct{}, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			s.AddRelease(repoID, Release{
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

	releases := s.GetReleases(repoID)
	if len(releases) > releaseLimit {
		t.Errorf("Expected max %d releases, got %d", releaseLimit, len(releases))
	}
}

// TestStoreSetProjectPushEnabled tests per-project push notification toggle.
func TestStoreSetProjectPushEnabled(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	repoID, err := s.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	// Default is enabled — verify via GetUserRepos
	projects := s.GetUserRepos(userID)
	if len(projects) != 1 || !projects[0].PushEnabled {
		t.Errorf("Expected push_enabled=true by default, got %v", projects[0].PushEnabled)
	}

	// Disable push
	ok, err := s.SetProjectPushEnabled(userID, repoID, false)
	if err != nil || !ok {
		t.Fatalf("SetProjectPushEnabled(false) returned ok=%v err=%v", ok, err)
	}
	projects = s.GetUserRepos(userID)
	if projects[0].PushEnabled {
		t.Error("Expected push_enabled=false after disabling")
	}

	// Re-enable push
	ok, err = s.SetProjectPushEnabled(userID, repoID, true)
	if err != nil || !ok {
		t.Fatalf("SetProjectPushEnabled(true) returned ok=%v err=%v", ok, err)
	}
	projects = s.GetUserRepos(userID)
	if !projects[0].PushEnabled {
		t.Error("Expected push_enabled=true after re-enabling")
	}

	// Unowned repo returns ok=false
	ok, err = s.SetProjectPushEnabled(userID, "repo_doesnotexist", false)
	if err != nil {
		t.Fatalf("unexpected error for unowned repo: %v", err)
	}
	if ok {
		t.Error("Expected ok=false for unowned repo ID")
	}
}

// TestHandleProjectSettings tests the POST /api/project-settings endpoint.
func TestHandleProjectSettings(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)
	userID, cookie := newTestAuth(t, store)

	repoID, err := store.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	// Valid request disables push
	body := strings.NewReader(`{"repo_id":"` + repoID + `","push_enabled":false}`)
	req, _ := http.NewRequest("POST", "/api/project-settings", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleProjectSettings).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %v: %s", w.Code, w.Body.String())
	}
	projects := store.GetUserRepos(userID)
	if projects[0].PushEnabled {
		t.Error("Expected push_enabled=false after handler call")
	}

	// Missing repo_id returns 400
	req, _ = http.NewRequest("POST", "/api/project-settings", strings.NewReader(`{"push_enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	requireAuth(handleProjectSettings).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing repo_id, got %v", w.Code)
	}

	// Unowned repo_id returns 404
	req, _ = http.NewRequest("POST", "/api/project-settings", strings.NewReader(`{"repo_id":"repo_doesnotexist","push_enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	requireAuth(handleProjectSettings).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unowned repo, got %v", w.Code)
	}

	// Wrong method returns 405
	req, _ = http.NewRequest("GET", "/api/project-settings", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	requireAuth(handleProjectSettings).ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 for GET, got %v", w.Code)
	}
}

// BenchmarkStoreAddRepo benchmarks adding repos
func BenchmarkStoreAddRepo(b *testing.B) {
	s, _ := NewStore(":memory:")
	user, _ := s.CreateUser("benchuser", "password123")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.AddRepo(user.ID, Project{
			Name:     fmt.Sprintf("Project %d", i),
			Platform: "github",
			RepoURL:  fmt.Sprintf("https://github.com/bench/repo%d", i),
		})
	}
}

// BenchmarkStoreAddRelease benchmarks adding releases
func BenchmarkStoreAddRelease(b *testing.B) {
	s, _ := NewStore(":memory:")
	user, _ := s.CreateUser("benchuser", "password123")
	repoID, _ := s.AddRepo(user.ID, Project{
		Name:     "Bench Project",
		Platform: "github",
		RepoURL:  "https://github.com/bench/repo",
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.AddRelease(repoID, Release{
			ID:       fmt.Sprintf("rel-%d", i),
			Name:     "Bench Project",
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

func TestStoreEmailFields(t *testing.T) {
	s := newTestStore(t)
	user, err := s.CreateUser("emailuser", "password123")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	got, err := s.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if got.Email != "" {
		t.Errorf("expected empty email, got %q", got.Email)
	}
	if got.EmailVerified {
		t.Error("expected email_verified false for new user")
	}
}
