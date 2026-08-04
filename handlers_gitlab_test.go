package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleGitLabSettingsGETEmpty(t *testing.T) {
	s := newTestStore(t)
	oldStore := store
	store = s
	defer func() { store = oldStore }()
	_, cookie := newTestAuth(t, s)

	req := httptest.NewRequest(http.MethodGet, "/api/gitlab-settings", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleGitLabSettings).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["has_token"] != false {
		t.Errorf("has_token = %v, want false for unconfigured user", body["has_token"])
	}
}

func TestHandleGitLabSettingsPOST(t *testing.T) {
	s := newTestStore(t)
	oldStore := store
	store = s
	defer func() { store = oldStore }()
	_, cookie := newTestAuth(t, s)

	body, _ := json.Marshal(map[string]string{"gitlab_url": "https://example.com", "gitlab_token": "tok"})
	req := httptest.NewRequest(http.MethodPost, "/api/gitlab-settings", bytes.NewReader(body))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleGitLabSettings).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/gitlab-settings", nil)
	getReq.AddCookie(cookie)
	getW := httptest.NewRecorder()
	requireAuth(handleGitLabSettings).ServeHTTP(getW, getReq)
	var got map[string]any
	json.NewDecoder(getW.Body).Decode(&got)
	if got["has_token"] != true || got["gitlab_url"] != "https://example.com" {
		t.Errorf("GET after POST = %+v, want has_token=true url set", got)
	}
}

func TestHandleGitLabSettingsPOSTRejectsBadURL(t *testing.T) {
	s := newTestStore(t)
	oldStore := store
	store = s
	defer func() { store = oldStore }()
	_, cookie := newTestAuth(t, s)

	body, _ := json.Marshal(map[string]string{"gitlab_url": "not-a-url", "gitlab_token": "tok"})
	req := httptest.NewRequest(http.MethodPost, "/api/gitlab-settings", bytes.NewReader(body))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleGitLabSettings).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for a gitlab_url without http(s) scheme", w.Code)
	}
}

func TestHandleProjectGitLabSyncRequiresGitLabConfigured(t *testing.T) {
	s := newTestStore(t)
	oldStore := store
	store = s
	defer func() { store = oldStore }()
	userID, cookie := newTestAuth(t, s)
	repoID, _ := s.AddRepo(userID, Project{Name: "repo", Platform: "github", RepoURL: "https://github.com/o/r"})

	body, _ := json.Marshal(map[string]any{"repo_id": repoID, "enabled": true, "frequency": "daily"})
	req := httptest.NewRequest(http.MethodPost, "/api/project-gitlab-sync", bytes.NewReader(body))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleProjectGitLabSync).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 when enabling sync before gitlab-settings are configured, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleProjectGitLabSyncEnable(t *testing.T) {
	s := newTestStore(t)
	oldStore := store
	store = s
	defer func() { store = oldStore }()
	userID, cookie := newTestAuth(t, s)
	repoID, _ := s.AddRepo(userID, Project{Name: "repo", Platform: "github", RepoURL: "https://github.com/o/r"})
	s.SaveGitLabSettings(userID, "https://gitlab.example.com", "tok", "")

	body, _ := json.Marshal(map[string]any{"repo_id": repoID, "enabled": true, "frequency": "weekly"})
	req := httptest.NewRequest(http.MethodPost, "/api/project-gitlab-sync", bytes.NewReader(body))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleProjectGitLabSync).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	projects := s.GetUserRepos(userID)
	if !projects[0].GitLabSyncEnabled || projects[0].GitLabSyncFrequency != "weekly" {
		t.Errorf("project after enable = %+v, want gitlab sync enabled weekly", projects[0])
	}
}

func TestHandleProjectGitLabSyncNowRequiresEnabled(t *testing.T) {
	s := newTestStore(t)
	oldStore := store
	store = s
	defer func() { store = oldStore }()
	userID, cookie := newTestAuth(t, s)
	repoID, _ := s.AddRepo(userID, Project{Name: "repo", Platform: "github", RepoURL: "https://github.com/o/r"})

	req := httptest.NewRequest(http.MethodPost, "/api/project-gitlab-sync/sync-now?repo_id="+repoID, nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleProjectGitLabSyncNow).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404 when sync isn't enabled for the project, body: %s", w.Code, w.Body.String())
	}
	_ = userID
}

func TestHandleGitLabAwesomeRequiresGitLabConfigured(t *testing.T) {
	s := newTestStore(t)
	oldStore := store
	store = s
	defer func() { store = oldStore }()
	_, cookie := newTestAuth(t, s)

	body, _ := json.Marshal(map[string]any{"enabled": true, "repo_name": "my-awesome"})
	req := httptest.NewRequest(http.MethodPost, "/api/gitlab-settings/awesome", bytes.NewReader(body))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleGitLabAwesome).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 when enabling awesome before gitlab-settings are configured, body: %s", w.Code, w.Body.String())
	}
}
