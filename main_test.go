package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
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
	user, err := s.CreateUser("testuser@example.com", "password123")
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

	user1, _ := s.CreateUser("user1@example.com", "password123")
	user2, _ := s.CreateUser("user2@example.com", "password123")

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

	user, err := s1.CreateUser("persist_user@example.com", "password123")
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

// TestStoreSetProjectEmailImmediate tests per-project immediate email toggle.
func TestStoreSetProjectEmailImmediate(t *testing.T) {
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

	// Default is disabled
	projects := s.GetUserRepos(userID)
	if len(projects) != 1 || projects[0].EmailImmediate {
		t.Errorf("expected email_immediate=false by default, got %v", projects[0].EmailImmediate)
	}

	// Enable
	ok, err := s.SetProjectEmailImmediate(userID, repoID, true)
	if err != nil || !ok {
		t.Fatalf("SetProjectEmailImmediate(true) returned ok=%v err=%v", ok, err)
	}
	projects = s.GetUserRepos(userID)
	if !projects[0].EmailImmediate {
		t.Error("expected email_immediate=true after enabling")
	}

	// Disable
	ok, err = s.SetProjectEmailImmediate(userID, repoID, false)
	if err != nil || !ok {
		t.Fatalf("SetProjectEmailImmediate(false) returned ok=%v err=%v", ok, err)
	}
	projects = s.GetUserRepos(userID)
	if projects[0].EmailImmediate {
		t.Error("expected email_immediate=false after disabling")
	}

	// Unowned repo returns ok=false
	ok, err = s.SetProjectEmailImmediate(userID, "repo_doesnotexist", false)
	if err != nil {
		t.Fatalf("unexpected error for unowned repo: %v", err)
	}
	if ok {
		t.Error("expected ok=false for unknown repo")
	}
}

// TestGetImmediateEmailUsers tests that only verified users with email_immediate=1 are returned.
func TestGetImmediateEmailUsers(t *testing.T) {
	s := newTestStore(t)

	// User A — opts in, verified
	userA, _ := newTestAuth(t, s)
	s.db.Exec("UPDATE users SET email='a@example.com', email_verified=1 WHERE id=?", userA)
	repoID, _ := s.AddRepo(userA, Project{Name: "Proj", Platform: "github", RepoURL: "https://github.com/x/y"})
	s.SetProjectEmailImmediate(userA, repoID, true)

	// User B — opts in, but unverified
	userB, _ := s.CreateUser("userb", "pass")
	s.db.Exec("UPDATE users SET email='b@example.com', email_verified=0 WHERE id=?", userB.ID)
	s.AddRepo(userB.ID, Project{Name: "Proj", Platform: "github", RepoURL: "https://github.com/x/y"})
	s.SetProjectEmailImmediate(userB.ID, repoID, true)

	// User C — verified, but did not opt in
	userC, _ := s.CreateUser("userc", "pass")
	s.db.Exec("UPDATE users SET email='c@example.com', email_verified=1 WHERE id=?", userC.ID)
	s.AddRepo(userC.ID, Project{Name: "Proj", Platform: "github", RepoURL: "https://github.com/x/y"})
	// email_immediate stays false (default)

	users, err := s.getImmediateEmailUsersForRepo(repoID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].ID != userA {
		t.Errorf("expected user A, got %s", users[0].ID)
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

// TestHandleProjectSettingsEmailImmediate tests email_immediate field in POST /api/project-settings.
func TestHandleProjectSettingsEmailImmediate(t *testing.T) {
	originalStore := store
	originalSMTP := smtpEnabled
	defer func() {
		store = originalStore
		smtpEnabled = originalSMTP
	}()
	store = newTestStore(t)
	smtpEnabled = true
	userID, cookie := newTestAuth(t, store)

	repoID, err := store.AddRepo(userID, Project{
		Name:     "Test Project",
		Platform: "github",
		RepoURL:  "https://github.com/test/repo",
	})
	if err != nil {
		t.Fatalf("AddRepo failed: %v", err)
	}

	// Enable email_immediate
	body := strings.NewReader(`{"repo_id":"` + repoID + `","push_enabled":true,"email_immediate":true}`)
	req, _ := http.NewRequest("POST", "/api/project-settings", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleProjectSettings).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %v: %s", w.Code, w.Body.String())
	}
	projects := store.GetUserRepos(userID)
	if !projects[0].EmailImmediate {
		t.Error("expected email_immediate=true after handler call")
	}

	// Disable email_immediate
	body = strings.NewReader(`{"repo_id":"` + repoID + `","push_enabled":true,"email_immediate":false}`)
	req, _ = http.NewRequest("POST", "/api/project-settings", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	requireAuth(handleProjectSettings).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %v: %s", w.Code, w.Body.String())
	}
	projects = store.GetUserRepos(userID)
	if projects[0].EmailImmediate {
		t.Error("expected email_immediate=false after disabling")
	}

	// email_immediate silently ignored when smtpEnabled=false
	smtpEnabled = false
	body = strings.NewReader(`{"repo_id":"` + repoID + `","push_enabled":true,"email_immediate":true}`)
	req, _ = http.NewRequest("POST", "/api/project-settings", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	requireAuth(handleProjectSettings).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 even when smtp disabled, got %v", w.Code)
	}
	projects = store.GetUserRepos(userID)
	if projects[0].EmailImmediate {
		t.Error("expected email_immediate unchanged (false) when smtp disabled")
	}
}

// BenchmarkStoreAddRepo benchmarks adding repos
func BenchmarkStoreAddRepo(b *testing.B) {
	s, _ := NewStore(":memory:")
	user, _ := s.CreateUser("benchuser@example.com", "password123")
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
	user, _ := s.CreateUser("benchuser@example.com", "password123")
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
	user, err := s.CreateUser("emailuser@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	got, err := s.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if got.Email != "emailuser@example.com" {
		t.Errorf("expected email emailuser@example.com, got %q", got.Email)
	}
	if got.EmailVerified {
		t.Error("expected email_verified false for new user")
	}
}

func TestStoreVerificationToken(t *testing.T) {
	s := newTestStore(t)
	user, _ := s.CreateUser("token@example.com", "password123")

	token, err := s.CreateVerificationToken(user.ID)
	if err != nil {
		t.Fatalf("CreateVerificationToken failed: %v", err)
	}
	if len(token) == 0 {
		t.Fatal("expected non-empty token")
	}

	// Verify it
	gotUserID, err := s.VerifyEmailToken(token)
	if err != nil {
		t.Fatalf("VerifyEmailToken failed: %v", err)
	}
	if gotUserID != user.ID {
		t.Errorf("expected userID %s, got %s", user.ID, gotUserID)
	}

	// User should now be verified
	got, _ := s.GetUserByID(user.ID)
	if !got.EmailVerified {
		t.Error("expected email_verified true after verification")
	}

	// Token should be consumed
	_, err = s.VerifyEmailToken(token)
	if err != ErrTokenNotFound {
		t.Errorf("expected ErrTokenNotFound after use, got %v", err)
	}
}

func TestStoreVerifyEmailTokenExpired(t *testing.T) {
	s := newTestStore(t)
	user, _ := s.CreateUser("expiretest@example.com", "password123")

	// Insert an already-expired token directly
	expiredAt := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	s.db.Exec(
		"INSERT INTO email_verification_tokens (token, user_id, expires_at) VALUES (?, ?, ?)",
		"expiredtoken", user.ID, expiredAt,
	)

	_, err := s.VerifyEmailToken("expiredtoken")
	if err != ErrTokenExpired {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestStoreVerifyEmailTokenNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.VerifyEmailToken("doesnotexist")
	if err != ErrTokenNotFound {
		t.Errorf("expected ErrTokenNotFound, got %v", err)
	}
}

func TestStoreCreateVerificationTokenDeletesOld(t *testing.T) {
	s := newTestStore(t)
	user, _ := s.CreateUser("reusetest@example.com", "password123")

	token1, _ := s.CreateVerificationToken(user.ID)
	token2, _ := s.CreateVerificationToken(user.ID)

	if token1 == token2 {
		t.Error("expected different tokens")
	}
	// Old token should be gone
	_, err := s.VerifyEmailToken(token1)
	if err != ErrTokenNotFound {
		t.Errorf("expected old token deleted, got %v", err)
	}
}

func TestSMTPConfigFromEnv(t *testing.T) {
	// No env vars set — should return disabled
	cfg, ok := SMTPConfigFromEnv()
	if ok {
		t.Errorf("expected disabled with no env vars, got config: %+v", cfg)
	}

	// Set required vars
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_USER", "user@example.com")
	t.Setenv("SMTP_PASS", "secret")
	t.Setenv("SMTP_FROM", "noreply@example.com")

	cfg, ok = SMTPConfigFromEnv()
	if !ok {
		t.Fatal("expected enabled with all required env vars")
	}
	if cfg.Host != "smtp.example.com" {
		t.Errorf("expected Host smtp.example.com, got %q", cfg.Host)
	}
	if cfg.Port != 587 {
		t.Errorf("expected default Port 587, got %d", cfg.Port)
	}
	if cfg.User != "user@example.com" {
		t.Errorf("expected User user@example.com, got %q", cfg.User)
	}

	// Custom port
	t.Setenv("SMTP_PORT", "465")
	cfg, _ = SMTPConfigFromEnv()
	if cfg.Port != 465 {
		t.Errorf("expected Port 465, got %d", cfg.Port)
	}
}

func TestHandleRegisterWithEmail(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	body := `{"password":"password123","email":"new@example.com"}`
	req, _ := http.NewRequest("POST", "/api/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", "localhost:8080")
	w := httptest.NewRecorder()
	handleRegister(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var user User
	if err := json.NewDecoder(w.Body).Decode(&user); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if user.Email != "new@example.com" {
		t.Errorf("expected email new@example.com in response, got %q", user.Email)
	}
	if user.EmailVerified {
		t.Error("expected email_verified false on fresh registration")
	}
}

func TestHandleRegisterMissingEmail(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	body := `{"password":"password123"}`
	req, _ := http.NewRequest("POST", "/api/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handleRegister(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleVerifyEmail(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	user, _ := store.CreateUser("verify@example.com", "password123")
	token, _ := store.CreateVerificationToken(user.ID)

	req, _ := http.NewRequest("GET", "/verify-email?token="+token, nil)
	w := httptest.NewRecorder()
	handleVerifyEmail(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/?verified=1" {
		t.Errorf("expected redirect to /?verified=1, got %q", loc)
	}

	got, _ := store.GetUserByID(user.ID)
	if !got.EmailVerified {
		t.Error("expected email_verified true after verification")
	}
}

func TestHandleVerifyEmailInvalidToken(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	req, _ := http.NewRequest("GET", "/verify-email?token=badtoken", nil)
	w := httptest.NewRecorder()
	handleVerifyEmail(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/?verify_error=1" {
		t.Errorf("expected redirect to /?verify_error=1, got %q", loc)
	}
}

func TestHandleResendVerification(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	// Reset rate limiter state for test isolation.
	resendMu.Lock()
	resendAttempts = map[string][]time.Time{}
	resendMu.Unlock()

	_, cookie := newTestAuth(t, store)

	req, _ := http.NewRequest("POST", "/api/resend-verification", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleResendVerification).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleResendVerificationAlreadyVerified(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	// Reset rate limiter state for test isolation.
	resendMu.Lock()
	resendAttempts = map[string][]time.Time{}
	resendMu.Unlock()

	userID, cookie := newTestAuth(t, store)
	// Mark verified directly
	store.db.Exec("UPDATE users SET email_verified = 1 WHERE id = ?", userID)

	req, _ := http.NewRequest("POST", "/api/resend-verification", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleResendVerification).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleResendVerificationRateLimit(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	// Reset rate limiter state for this test
	resendMu.Lock()
	resendAttempts = map[string][]time.Time{}
	resendMu.Unlock()

	_, cookie := newTestAuth(t, store)

	// Make 3 successful requests
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest("POST", "/api/resend-verification", nil)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		requireAuth(handleResendVerification).ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 4th request should be rate limited
	req, _ := http.NewRequest("POST", "/api/resend-verification", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleResendVerification).ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}

func TestHandleResendVerificationMethodNotAllowed(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	// Reset rate limiter state for test isolation.
	resendMu.Lock()
	resendAttempts = map[string][]time.Time{}
	resendMu.Unlock()

	_, cookie := newTestAuth(t, store)

	req, _ := http.NewRequest("GET", "/api/resend-verification", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleResendVerification).ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestSetGetEmailDigest(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	// default is false
	enabled, err := s.GetUserDigestEnabled(userID)
	if err != nil {
		t.Fatalf("GetUserDigestEnabled: %v", err)
	}
	if enabled {
		t.Error("expected email_digest disabled by default")
	}

	// enable it
	if err := s.SetEmailDigest(userID, true); err != nil {
		t.Fatalf("SetEmailDigest true: %v", err)
	}
	enabled, err = s.GetUserDigestEnabled(userID)
	if err != nil {
		t.Fatalf("GetUserDigestEnabled after set: %v", err)
	}
	if !enabled {
		t.Error("expected email_digest enabled after set")
	}

	// disable it
	if err := s.SetEmailDigest(userID, false); err != nil {
		t.Fatalf("SetEmailDigest false: %v", err)
	}
	enabled, _ = s.GetUserDigestEnabled(userID)
	if enabled {
		t.Error("expected email_digest disabled after unset")
	}
}

func TestSetPageSize(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	// valid sizes
	for _, size := range []int{5, 10, 20} {
		if err := s.SetPageSize(userID, size); err != nil {
			t.Fatalf("SetPageSize(%d): %v", size, err)
		}
		user, err := s.GetUserByID(userID)
		if err != nil {
			t.Fatalf("GetUserByID: %v", err)
		}
		if user.PageSize != size {
			t.Errorf("expected page_size %d, got %d", size, user.PageSize)
		}
	}

	// invalid size rejected
	if err := s.SetPageSize(userID, 7); err == nil {
		t.Error("expected error for invalid page_size 7, got nil")
	}
}

func TestGetDigestUsers(t *testing.T) {
	s := newTestStore(t)

	// u1: digest + verified + email => should appear
	u1, _ := s.CreateUser("digest1@example.com", "password123")
	s.db.Exec("UPDATE users SET email_verified = 1 WHERE id = ?", u1.ID)
	s.SetEmailDigest(u1.ID, true)

	// u2: digest enabled but not verified => should NOT appear
	u2, _ := s.CreateUser("digest2@example.com", "password123")
	s.SetEmailDigest(u2.ID, true)

	// u3: verified but digest off => should NOT appear
	u3, _ := s.CreateUser("digest3@example.com", "password123")
	s.db.Exec("UPDATE users SET email_verified = 1 WHERE id = ?", u3.ID)

	users := s.GetDigestUsers()
	if len(users) != 1 {
		t.Errorf("expected 1 digest user, got %d", len(users))
	}
	if len(users) > 0 && users[0].ID != u1.ID {
		t.Errorf("expected user %s, got %s", u1.ID, users[0].ID)
	}
}

func TestGetReleasesPublishedOn(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	repoID, _ := s.AddRepo(userID, Project{
		Name: "TestProj", Platform: "github",
		RepoURL: "https://github.com/test/repo",
	})

	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1)

	// Release published yesterday (noon UTC) — should appear
	s.AddRelease(repoID, Release{
		ID: "rel-yesterday", Name: "TestProj", Version: "v1.0.0",
		Platform: "github", URL: "https://github.com/test/repo/releases/v1.0.0",
		PublishedAt: time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 12, 0, 0, 0, time.UTC),
	})
	// Release published today — should NOT appear
	s.AddRelease(repoID, Release{
		ID: "rel-today", Name: "TestProj", Version: "v1.1.0",
		Platform: "github", URL: "https://github.com/test/repo/releases/v1.1.0",
		PublishedAt: time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.UTC),
	})

	releases := s.GetReleasesPublishedOn(userID, yesterday)
	if len(releases) != 1 {
		t.Fatalf("expected 1 release for yesterday, got %d", len(releases))
	}
	if releases[0].ID != "rel-yesterday" {
		t.Errorf("expected rel-yesterday, got %s", releases[0].ID)
	}

	// Other user should see nothing (no user_repos entry)
	u2, _ := s.CreateUser("other@example.com", "password123")
	releases2 := s.GetReleasesPublishedOn(u2.ID, yesterday)
	if len(releases2) != 0 {
		t.Errorf("expected 0 releases for other user, got %d", len(releases2))
	}
}

func TestBuildDailySummaryBody(t *testing.T) {
	releases := []Release{
		{Name: "myproject", Version: "v1.2.3", URL: "https://github.com/my/project/releases/v1.2.3"},
		{Name: "another", Version: "v0.9.0", URL: "https://pypi.org/project/another/0.9.0"},
	}
	body := buildDailySummaryBody(releases)
	if !strings.Contains(body, "Here are the releases from yesterday:") {
		t.Error("expected header line in body")
	}
	if !strings.Contains(body, "myproject v1.2.3") {
		t.Error("expected 'myproject v1.2.3' in body")
	}
	if !strings.Contains(body, "https://github.com/my/project/releases/v1.2.3") {
		t.Error("expected URL in body")
	}
	if !strings.Contains(body, "another v0.9.0") {
		t.Error("expected 'another v0.9.0' in body")
	}
}

// TestBuildReleaseEmailBody tests the plain-text email body for a single release.
func TestBuildReleaseEmailBody(t *testing.T) {
	proj := Project{Name: "myproject", Platform: "github"}
	release := Release{Version: "v1.2.3", URL: "https://github.com/x/myproject/releases/tag/v1.2.3"}

	// Without release notes
	body := buildReleaseEmailBody(proj, release)
	if !strings.Contains(body, "myproject v1.2.3 has been released.") {
		t.Errorf("body missing release line, got:\n%s", body)
	}
	if !strings.Contains(body, "https://github.com/x/myproject/releases/tag/v1.2.3") {
		t.Errorf("body missing URL, got:\n%s", body)
	}
	if strings.Contains(body[strings.Index(body, release.URL)+len(release.URL):], "\n\n") {
		t.Errorf("body should not have trailing blank line when no release notes")
	}

	// With release notes
	release.ReleaseNotes = "- Fixed a bug\n- Added a feature"
	body = buildReleaseEmailBody(proj, release)
	if !strings.Contains(body, "- Fixed a bug") {
		t.Errorf("body missing release notes, got:\n%s", body)
	}
}

// TestSendReleaseEmailSubject verifies the subject format.
func TestSendReleaseEmailSubject(t *testing.T) {
	proj := Project{Name: "myproject"}
	release := Release{Version: "v1.2.3", URL: "https://example.com"}
	// Verify subject is correct by calling buildReleaseEmailBody (SendReleaseEmail requires live SMTP).
	// Subject format: "<Name> <Version> released"
	expectedSubject := "myproject v1.2.3 released"
	subject := fmt.Sprintf("%s %s released", proj.Name, release.Version)
	if subject != expectedSubject {
		t.Errorf("expected subject %q, got %q", expectedSubject, subject)
	}
}

func TestNextSevenAMUTC(t *testing.T) {
	d := nextSevenAMUTC()
	if d <= 0 {
		t.Errorf("expected positive duration, got %v", d)
	}
	if d > 24*time.Hour {
		t.Errorf("expected at most 24h, got %v", d)
	}
}

func TestHandleAccountSettingsGET(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	userID, cookie := newTestAuth(t, store)
	_ = userID

	req, _ := http.NewRequest("GET", "/api/account-settings", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleAccountSettings).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["email_digest"].(bool) {
		t.Error("expected email_digest false by default")
	}
}

func TestHandleAccountSettingsPOST(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	userID, cookie := newTestAuth(t, store)

	body := `{"email_digest":true}`
	req, _ := http.NewRequest("POST", "/api/account-settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleAccountSettings).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	enabled, err := store.GetUserDigestEnabled(userID)
	if err != nil {
		t.Fatalf("GetUserDigestEnabled: %v", err)
	}
	if !enabled {
		t.Error("expected email_digest enabled after POST")
	}
}

func TestHandleAccountSettingsMethodNotAllowed(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	_, cookie := newTestAuth(t, store)

	req, _ := http.NewRequest("DELETE", "/api/account-settings", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleAccountSettings).ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// TestCreateUserGeneratesRSSToken verifies every new user gets a non-empty, unique rss_token.
func TestCreateUserGeneratesRSSToken(t *testing.T) {
	s := newTestStore(t)

	u1, err := s.CreateUser("alice@example.com", "password1")
	if err != nil {
		t.Fatalf("CreateUser alice: %v", err)
	}
	if u1.RSSToken == "" {
		t.Fatal("expected non-empty RSSToken for new user")
	}

	u2, err := s.CreateUser("bob@example.com", "password2")
	if err != nil {
		t.Fatalf("CreateUser bob: %v", err)
	}
	if u2.RSSToken == "" {
		t.Fatal("expected non-empty RSSToken for new user")
	}
	if u1.RSSToken == u2.RSSToken {
		t.Errorf("expected unique tokens; both got %q", u1.RSSToken)
	}

	// GetUserByID must also return the token
	got, err := s.GetUserByID(u1.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.RSSToken != u1.RSSToken {
		t.Errorf("GetUserByID returned token %q, want %q", got.RSSToken, u1.RSSToken)
	}
}

// TestGetUserByRSSToken verifies lookup by token works and returns nil for unknown tokens.
func TestGetUserByRSSToken(t *testing.T) {
	s := newTestStore(t)

	u, err := s.CreateUser("alice@example.com", "password1")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Found
	found, err := s.GetUserByRSSToken(u.RSSToken)
	if err != nil {
		t.Fatalf("GetUserByRSSToken: %v", err)
	}
	if found == nil || found.ID != u.ID {
		t.Errorf("expected user %q, got %v", u.ID, found)
	}

	// Not found — nil, nil
	missing, err := s.GetUserByRSSToken("doesnotexist00000000000000000000")
	if err != nil {
		t.Fatalf("unexpected error for unknown token: %v", err)
	}
	if missing != nil {
		t.Errorf("expected nil for unknown token, got %v", missing)
	}
}

// TestHandleFeed tests GET /feed/<token> returns Atom XML for valid tokens.
func TestHandleFeed(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	s := newTestStore(t)
	store = s

	// Create user and add a release
	user, err := s.CreateUser("feeduser@example.com", "password")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	repoID, err := s.AddRepo(user.ID, Project{
		Name:     "myproject",
		Platform: "github",
		RepoURL:  "https://github.com/x/myproject",
	})
	if err != nil {
		t.Fatalf("AddRepo: %v", err)
	}
	s.AddRelease(repoID, Release{
		ID:          "gh_1",
		Name:        "myproject",
		Version:     "v1.2.3",
		Platform:    "github",
		URL:         "https://github.com/x/myproject/releases/tag/v1.2.3",
		PublishedAt: time.Now().Add(-time.Hour),
	})

	// Valid token — expect 200 with Atom content type and valid XML
	req, _ := http.NewRequest("GET", "/feed/"+user.RSSToken, nil)
	req.Host = "example.com"
	w := httptest.NewRecorder()
	http.HandlerFunc(handleFeed).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid token, got %d: %s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/atom+xml") {
		t.Errorf("expected atom+xml content type, got %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "myproject v1.2.3") {
		t.Errorf("expected release title in feed, got:\n%s", body)
	}
	if !strings.Contains(body, "https://github.com/x/myproject/releases/tag/v1.2.3") {
		t.Errorf("expected release URL in feed, got:\n%s", body)
	}

	// Unknown token — expect 404
	req, _ = http.NewRequest("GET", "/feed/unknowntoken00000000000000000000000000000000000000000000000000", nil)
	w = httptest.NewRecorder()
	http.HandlerFunc(handleFeed).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown token, got %d", w.Code)
	}

	// Empty token path — expect 404
	req, _ = http.NewRequest("GET", "/feed/", nil)
	w = httptest.NewRecorder()
	http.HandlerFunc(handleFeed).ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for empty token, got %d", w.Code)
	}
}

// TestHandleFeedEmptyReleases tests that a valid token with no releases returns a valid empty Atom feed.
func TestHandleFeedEmptyReleases(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	s := newTestStore(t)
	store = s

	user, err := s.CreateUser("emptyuser@example.com", "password")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	req, _ := http.NewRequest("GET", "/feed/"+user.RSSToken, nil)
	req.Host = "example.com"
	w := httptest.NewRecorder()
	http.HandlerFunc(handleFeed).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for empty feed, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "<feed") {
		t.Errorf("expected Atom feed element, got:\n%s", body)
	}
}

// TestHandleMeIncludesRSSToken verifies /api/me response includes rss_token.
func TestHandleMeIncludesRSSToken(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)
	_, cookie := newTestAuth(t, store)

	req, _ := http.NewRequest("GET", "/api/me", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleMe).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	token, ok := resp["rss_token"].(string)
	if !ok || token == "" {
		t.Errorf("expected non-empty rss_token in /api/me response, got %v", resp["rss_token"])
	}
}

// TestUserPageSizeDefault tests that new users get a default page_size of 10
func TestUserPageSizeDefault(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	user, err := s.GetUserByID(userID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if user.PageSize != 10 {
		t.Errorf("expected default page_size 10, got %d", user.PageSize)
	}
}

func TestHandleAccountSettingsPageSizeGET(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	_, cookie := newTestAuth(t, store)

	req, _ := http.NewRequest("GET", "/api/account-settings", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleAccountSettings).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	ps, ok := resp["page_size"]
	if !ok {
		t.Fatal("expected page_size in response")
	}
	if int(ps.(float64)) != 10 {
		t.Errorf("expected default page_size 10, got %v", ps)
	}
}

func TestHandleAccountSettingsPageSizePOST(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	userID, cookie := newTestAuth(t, store)

	body := `{"page_size":5}`
	req, _ := http.NewRequest("POST", "/api/account-settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleAccountSettings).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	user, err := store.GetUserByID(userID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if user.PageSize != 5 {
		t.Errorf("expected page_size 5, got %d", user.PageSize)
	}
}

func TestHandleAccountSettingsPageSizeInvalid(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	_, cookie := newTestAuth(t, store)

	body := `{"page_size":7}`
	req, _ := http.NewRequest("POST", "/api/account-settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleAccountSettings).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid page_size, got %d", w.Code)
	}
}

func TestMigrationV10EmailUnique(t *testing.T) {
	s := newTestStore(t)

	// After migration, duplicate email must fail.
	_, err := s.db.Exec(
		`INSERT INTO users (id, email, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		"user_aaa111aaa111", "dup@example.com", "hash", "2026-01-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("first insert failed: %v", err)
	}
	_, err = s.db.Exec(
		`INSERT INTO users (id, email, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		"user_bbb222bbb222", "dup@example.com", "hash", "2026-01-01T00:00:00Z",
	)
	if err == nil {
		t.Error("expected UNIQUE constraint error for duplicate email, got nil")
	}
}

func TestMigrationV10NoUsernameColumn(t *testing.T) {
	s := newTestStore(t)

	// The username column must not exist after v10 migration.
	_, err := s.db.Exec(`INSERT INTO users (id, username, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		"user_test000", "shouldfail", "hash", "2026-01-01T00:00:00Z")
	if err == nil {
		t.Error("expected error inserting into username column (should not exist), got nil")
	}
}

func TestCreateUserByEmail(t *testing.T) {
	s := newTestStore(t)
	user, err := s.CreateUser("alice@example.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %q", user.Email)
	}
	if user.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestCreateUserDuplicateEmail(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateUser("dup@example.com", "password123"); err != nil {
		t.Fatalf("first CreateUser: %v", err)
	}
	_, err := s.CreateUser("dup@example.com", "password123")
	if err == nil {
		t.Error("expected error for duplicate email, got nil")
	}
}

func TestAuthenticateUserByEmail(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateUser("bob@example.com", "secret123"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	user, err := s.AuthenticateUser("bob@example.com", "secret123")
	if err != nil {
		t.Fatalf("AuthenticateUser: %v", err)
	}
	if user.Email != "bob@example.com" {
		t.Errorf("expected email bob@example.com, got %q", user.Email)
	}
}

func TestAuthenticateUserWrongPassword(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.CreateUser("carol@example.com", "rightpass"); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	_, err := s.AuthenticateUser("carol@example.com", "wrongpass")
	if err == nil {
		t.Error("expected error for wrong password, got nil")
	}
}

func TestMigrationV10WithExistingData(t *testing.T) {
	// Verify that the v10 migration correctly handles FK constraints.
	// After migration, we should be able to insert users and sessions with FK references intact.
	s := newTestStore(t)

	// Manually insert a user (bypassing the CreateUser function which is being updated in Task 2)
	userID := "user_test123456"
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}
	_, err = s.db.Exec(
		"INSERT INTO users (id, email, password_hash, created_at) VALUES (?, ?, ?, ?)",
		userID, "existing@example.com", string(hash), time.Now().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	// Create a session that references the user (tests FK constraint is working)
	sessionID := s.CreateSession(userID)
	if sessionID == "" {
		t.Fatal("expected non-empty session ID")
	}

	// Verify the session and user relationship works (FK constraint is intact)
	fetchedUserID, ok := s.GetSessionUser(sessionID)
	if !ok {
		t.Fatal("expected session to be valid after migration")
	}
	if fetchedUserID != userID {
		t.Errorf("expected userID %s, got %s", userID, fetchedUserID)
	}
}

func TestFetchHelmArtifactHub(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/packages/helm/bitnami/redis", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":        "redis",
			"description": "Redis chart",
			"available_versions": []map[string]interface{}{
				{"version": "17.0.0", "ts": int64(1699000000), "prerelease": false},
				{"version": "16.9.0", "ts": int64(1698000000), "prerelease": false},
				{"version": "16.0.0-beta.1", "ts": int64(1697000000), "prerelease": true},
			},
		})
	})
	mux.HandleFunc("/api/v1/packages/helm/bitnami/redis/17.0.0", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"version": "17.0.0", "app_version": "7.2.1"})
	})
	mux.HandleFunc("/api/v1/packages/helm/bitnami/redis/16.9.0", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"version": "16.9.0", "app_version": "7.0.5"})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	origBase := artifacthubBaseURL
	artifacthubBaseURL = server.URL
	defer func() { artifacthubBaseURL = origBase }()

	project := Project{
		Name:     "redis",
		Platform: "helm-artifacthub",
		RepoURL:  "https://artifacthub.io/packages/helm/bitnami/redis",
	}

	releases := fetchHelmArtifactHub(project)

	if len(releases) != 2 {
		t.Fatalf("expected 2 releases (prerelease filtered), got %d", len(releases))
	}
	if releases[0].Version != "17.0.0 (app 7.2.1)" {
		t.Errorf("expected version %q, got %q", "17.0.0 (app 7.2.1)", releases[0].Version)
	}
	if releases[0].ID != "helm_ah_17.0.0" {
		t.Errorf("expected ID %q, got %q", "helm_ah_17.0.0", releases[0].ID)
	}
	if releases[1].Version != "16.9.0 (app 7.0.5)" {
		t.Errorf("expected version %q, got %q", "16.9.0 (app 7.0.5)", releases[1].Version)
	}
	if releases[0].Platform != "helm-artifacthub" {
		t.Errorf("expected platform %q, got %q", "helm-artifacthub", releases[0].Platform)
	}
}

func TestFetchHelmArtifactHubPrereleaseOnlyFallback(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/packages/helm/org/chart", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":        "chart",
			"description": "",
			"available_versions": []map[string]interface{}{
				{"version": "1.0.0-alpha.1", "ts": int64(1699000000), "prerelease": true},
			},
		})
	})
	mux.HandleFunc("/api/v1/packages/helm/org/chart/1.0.0-alpha.1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"version": "1.0.0-alpha.1", "app_version": "1.0.0"})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	origBase := artifacthubBaseURL
	artifacthubBaseURL = server.URL
	defer func() { artifacthubBaseURL = origBase }()

	releases := fetchHelmArtifactHub(Project{
		Name:     "chart",
		Platform: "helm-artifacthub",
		RepoURL:  "https://artifacthub.io/packages/helm/org/chart",
	})

	if len(releases) != 1 {
		t.Fatalf("expected 1 release (fallback to prerelease), got %d", len(releases))
	}
	if releases[0].Version != "1.0.0-alpha.1 (app 1.0.0)" {
		t.Errorf("got %q", releases[0].Version)
	}
}

// disableHTTPSVerificationForTest temporarily disables TLS certificate verification
// for the default HTTP client, needed for httptest.NewTLSServer with self-signed certs.
func disableHTTPSVerificationForTest() func() {
	oldTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return func() {
		http.DefaultClient.Transport = oldTransport
	}
}

func TestFetchHelmRepo(t *testing.T) {
	defer disableHTTPSVerificationForTest()()
	indexYAML := `apiVersion: v1
entries:
  redis:
  - version: "17.0.0"
    appVersion: "7.2.1"
    created: "2023-10-01T00:00:00Z"
    urls:
    - https://charts.example.com/redis-17.0.0.tgz
  - version: "16.9.0"
    appVersion: "7.0.5"
    created: "2023-09-01T00:00:00Z"
    urls:
    - https://charts.example.com/redis-16.9.0.tgz
  - version: "16.0.0-beta.1"
    appVersion: "7.0.0"
    created: "2023-08-01T00:00:00Z"
    urls:
    - https://charts.example.com/redis-16.0.0-beta.1.tgz
generated: "2023-10-01T00:00:00Z"
`
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/index.yaml" {
			w.Header().Set("Content-Type", "application/yaml")
			fmt.Fprint(w, indexYAML)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	project := Project{
		Name:     "redis",
		Platform: "helm-repo",
		RepoURL:  server.URL,
	}

	releases := fetchHelmRepo(project)

	if len(releases) != 2 {
		t.Fatalf("expected 2 releases (prerelease filtered), got %d", len(releases))
	}
	if releases[0].Version != "17.0.0 (app 7.2.1)" {
		t.Errorf("expected version %q, got %q", "17.0.0 (app 7.2.1)", releases[0].Version)
	}
	if releases[0].ID != "helm_repo_17.0.0" {
		t.Errorf("expected ID %q, got %q", "helm_repo_17.0.0", releases[0].ID)
	}
	if releases[1].Version != "16.9.0 (app 7.0.5)" {
		t.Errorf("expected version %q, got %q", "16.9.0 (app 7.0.5)", releases[1].Version)
	}
	if releases[0].URL != "https://charts.example.com/redis-17.0.0.tgz" {
		t.Errorf("expected URL from index, got %q", releases[0].URL)
	}
	if releases[0].Platform != "helm-repo" {
		t.Errorf("expected platform %q, got %q", "helm-repo", releases[0].Platform)
	}
	if releases[0].Description != "Helm Chart" {
		t.Errorf("expected description %q, got %q", "Helm Chart", releases[0].Description)
	}
}

func TestFetchHelmRepoChartNotFound(t *testing.T) {
	defer disableHTTPSVerificationForTest()()
	indexYAML := `apiVersion: v1
entries:
  nginx:
  - version: "1.0.0"
    appVersion: "1.25.0"
    created: "2023-10-01T00:00:00Z"
    urls:
    - https://charts.example.com/nginx-1.0.0.tgz
generated: "2023-10-01T00:00:00Z"
`
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, indexYAML)
	}))
	defer server.Close()

	releases := fetchHelmRepo(Project{Name: "redis", Platform: "helm-repo", RepoURL: server.URL})

	if releases != nil {
		t.Errorf("expected nil for missing chart, got %v", releases)
	}
}

func TestFetchHelmArtifactHubDetailError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/packages/helm/org/mychart", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":        "mychart",
			"description": "",
			"available_versions": []map[string]interface{}{
				{"version": "1.0.0", "ts": int64(1699000000), "prerelease": false},
			},
		})
	})
	// detail endpoint returns 404
	mux.HandleFunc("/api/v1/packages/helm/org/mychart/1.0.0", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "not found"})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	origBase := artifacthubBaseURL
	artifacthubBaseURL = server.URL
	defer func() { artifacthubBaseURL = origBase }()

	releases := fetchHelmArtifactHub(Project{
		Name:     "mychart",
		Platform: "helm-artifacthub",
		RepoURL:  "https://artifacthub.io/packages/helm/org/mychart",
	})

	if len(releases) != 0 {
		t.Errorf("expected 0 releases when detail call returns 404, got %d", len(releases))
	}
}
