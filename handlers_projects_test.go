package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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
