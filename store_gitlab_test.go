package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSaveAndGetGitLabSettings(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	got, err := s.GetGitLabSettings(userID)
	if err != nil {
		t.Fatalf("GetGitLabSettings on unconfigured user: %v", err)
	}
	if got.GitLabURL != "" || got.GitLabToken != "" {
		t.Errorf("expected zero-value settings for unconfigured user, got %+v", got)
	}

	if err := s.SaveGitLabSettings(userID, "https://gitlab.example.com", "tok123", ""); err != nil {
		t.Fatalf("SaveGitLabSettings: %v", err)
	}
	got, err = s.GetGitLabSettings(userID)
	if err != nil {
		t.Fatalf("GetGitLabSettings after save: %v", err)
	}
	if got.GitLabURL != "https://gitlab.example.com" || got.GitLabToken != "tok123" {
		t.Errorf("GetGitLabSettings = %+v, want url/token set", got)
	}

	// Save again should update, not duplicate.
	if err := s.SaveGitLabSettings(userID, "https://gitlab2.example.com", "tok456", ""); err != nil {
		t.Fatalf("SaveGitLabSettings update: %v", err)
	}
	got, _ = s.GetGitLabSettings(userID)
	if got.GitLabURL != "https://gitlab2.example.com" || got.GitLabToken != "tok456" {
		t.Errorf("GetGitLabSettings after update = %+v, want updated url/token", got)
	}
}

func TestSetAwesomeConfigRequiresGitLabSettings(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)

	if err := s.SetAwesomeConfig(userID, "my-awesome", true); err == nil {
		t.Error("SetAwesomeConfig should error when no gitlab_settings row exists yet")
	}

	s.SaveGitLabSettings(userID, "https://gitlab.example.com", "tok", "")
	if err := s.SetAwesomeConfig(userID, "my-awesome", true); err != nil {
		t.Fatalf("SetAwesomeConfig after configuring gitlab: %v", err)
	}
	got, _ := s.GetGitLabSettings(userID)
	if !got.AwesomeEnabled || got.AwesomeRepoName != "my-awesome" {
		t.Errorf("GetGitLabSettings = %+v, want awesome enabled with name set", got)
	}
}

func TestSetProjectGitLabSyncValidatesPlatform(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)
	repoID, _ := s.AddRepo(userID, Project{Name: "npm-thing", Platform: "npm", RepoURL: "https://npmjs.com/package/thing"})

	if _, err := s.SetProjectGitLabSync(userID, repoID, true, "daily"); err == nil {
		t.Error("SetProjectGitLabSync should reject enabling sync for a non-git platform (npm)")
	}
}

func TestSetProjectGitLabSyncEnableDisable(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)
	repoID, _ := s.AddRepo(userID, Project{Name: "repo", Platform: "github", RepoURL: "https://github.com/o/r"})

	ok, err := s.SetProjectGitLabSync(userID, repoID, true, "weekly")
	if err != nil || !ok {
		t.Fatalf("SetProjectGitLabSync enable = (%v, %v), want (true, nil)", ok, err)
	}

	projects := s.GetUserRepos(userID)
	if len(projects) != 1 || !projects[0].GitLabSyncEnabled || projects[0].GitLabSyncFrequency != "weekly" {
		t.Errorf("GetUserRepos after enable = %+v, want gitlab_sync_enabled=true frequency=weekly", projects)
	}

	ok, err = s.SetProjectGitLabSync(userID, repoID, false, "")
	if err != nil || !ok {
		t.Fatalf("SetProjectGitLabSync disable = (%v, %v), want (true, nil)", ok, err)
	}
	projects = s.GetUserRepos(userID)
	if projects[0].GitLabSyncEnabled {
		t.Error("expected gitlab_sync_enabled=false after disable")
	}
}

func TestRecordGitLabSyncAndTargets(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)
	repoID, _ := s.AddRepo(userID, Project{Name: "repo", Platform: "github", RepoURL: "https://github.com/o/r"})
	s.SaveGitLabSettings(userID, "https://gitlab.example.com", "tok", "")
	s.SetProjectGitLabSync(userID, repoID, true, "daily")

	targets := s.GetAllEnabledGitLabSyncTargets()
	if len(targets) != 1 {
		t.Fatalf("GetAllEnabledGitLabSyncTargets returned %d targets, want 1", len(targets))
	}
	tgt := targets[0]
	if tgt.UserID != userID || tgt.RepoID != repoID || tgt.GitLabURL != "https://gitlab.example.com" || tgt.GitLabToken != "tok" {
		t.Errorf("target = %+v, want matching user/repo/gitlab settings", tgt)
	}
	if !tgt.LastSyncAt.IsZero() {
		t.Errorf("LastSyncAt = %v, want zero before first sync", tgt.LastSyncAt)
	}

	if err := s.RecordGitLabSync(userID, repoID, nil); err != nil {
		t.Fatalf("RecordGitLabSync success: %v", err)
	}
	targets = s.GetUserGitLabSyncTargets(userID)
	if len(targets) != 1 || targets[0].LastSyncAt.IsZero() {
		t.Errorf("GetUserGitLabSyncTargets after successful sync = %+v, want LastSyncAt set", targets)
	}
	projects := s.GetUserRepos(userID)
	if projects[0].LastGitLabSyncError != "" {
		t.Errorf("LastGitLabSyncError = %q, want empty after successful sync", projects[0].LastGitLabSyncError)
	}

	if err := s.RecordGitLabSync(userID, repoID, fmt.Errorf("boom")); err != nil {
		t.Fatalf("RecordGitLabSync failure: %v", err)
	}
	projects = s.GetUserRepos(userID)
	if projects[0].LastGitLabSyncError != "boom" {
		t.Errorf("LastGitLabSyncError = %q, want %q", projects[0].LastGitLabSyncError, "boom")
	}
}

func TestGetAllEnabledGitLabSyncTargetsRequiresGitLabSettings(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)
	repoID, _ := s.AddRepo(userID, Project{Name: "repo", Platform: "github", RepoURL: "https://github.com/o/r"})
	// Enable sync WITHOUT saving gitlab_settings first.
	s.SetProjectGitLabSync(userID, repoID, true, "daily")

	targets := s.GetAllEnabledGitLabSyncTargets()
	if len(targets) != 0 {
		t.Errorf("GetAllEnabledGitLabSyncTargets = %d targets, want 0 when gitlab_settings is unconfigured", len(targets))
	}
}

func TestGitLabTokenInvalidFlag(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)
	if err := s.SaveGitLabSettings(userID, "https://gitlab.example.com/", "old", ""); err != nil {
		t.Fatal(err)
	}
	if got := s.GitLabTokenInvalidURL(userID); got != "" {
		t.Fatalf("url = %q before any failure", got)
	}

	s.noteGitLabAuth(userID, fmt.Errorf("gitlab: create project x failed, status 500")) // not an auth problem
	if got := s.GitLabTokenInvalidURL(userID); got != "" {
		t.Fatalf("a non-auth error flagged the token: %q", got)
	}

	s.noteGitLabAuth(userID, fmt.Errorf("check mirror project: %w: nope", errGitLabUnauthorized))
	want := "https://gitlab.example.com/-/user_settings/personal_access_tokens?name=newreleases&scopes=api"
	if got := s.GitLabTokenInvalidURL(userID); got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}

	if err := s.RecordGitLabSync(userID, "r", nil); err != nil { // a sync that worked clears it
		t.Fatal(err)
	}
	if got := s.GitLabTokenInvalidURL(userID); got != "" {
		t.Fatalf("a successful sync left the flag: %q", got)
	}

	s.noteGitLabAuth(userID, fmt.Errorf("git push: remote: HTTP Basic: Access denied"))
	s.SaveGitLabSettings(userID, "https://gitlab.example.com/", "new", "") // saving a new token clears it
	if got := s.GitLabTokenInvalidURL(userID); got != "" {
		t.Fatalf("saving a new token left the flag: %q", got)
	}
}

func TestGetGitLabSyncQueue(t *testing.T) {
	s := newTestStore(t)
	userID, _ := newTestAuth(t, s)
	mk := func(name string) string {
		id, err := s.AddRepo(userID, Project{Name: name, Platform: "github", RepoURL: "https://github.com/o/" + name})
		if err != nil {
			t.Fatal(err)
		}
		s.SetProjectGitLabSync(userID, id, true, "daily")
		return id
	}
	if got, _ := s.GetGitLabSyncQueue(userID); len(got) != 0 {
		t.Fatalf("no enabled projects: got %d items", len(got))
	}
	fresh, failed, run := mk("fresh"), mk("failed"), mk("run")
	mk("due")
	s.RecordGitLabSync(userID, fresh, nil)
	s.RecordGitLabSync(userID, failed, errors.New("push rejected"))
	s.gitlabSyncing.Store(userID+"/"+run, struct{}{})

	// Without GitLab settings everything not running is blocked.
	got, _ := s.GetGitLabSyncQueue(userID)
	for _, it := range got {
		if it.Status != "running" && it.Status != "blocked" {
			t.Errorf("%s: status %q, want blocked without settings", it.Name, it.Status)
		}
	}

	s.SaveGitLabSettings(userID, "https://gitlab.example.com", "tok", "")
	got, _ = s.GetGitLabSyncQueue(userID)
	var order []string
	for _, it := range got {
		order = append(order, it.Name+":"+it.Status)
	}
	// "due" was never synced so it is queued; "failed" and "fresh" were just synced so not due yet.
	want := []string{"run:running", "due:queued", "failed:failed", "fresh:ok"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("queue order = %v, want %v", order, want)
	}
	if got[2].LastError != "push rejected" {
		t.Errorf("last error = %q", got[2].LastError)
	}
}

func TestHandleGitLabSyncQueue(t *testing.T) {
	s := newTestStore(t)
	oldStore := store
	store = s
	defer func() { store = oldStore }()
	_, cookie := newTestAuth(t, s)

	req, _ := http.NewRequest("GET", "/api/gitlab-sync-queue", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleGitLabSyncQueue)(w, req)
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("empty queue: code=%d body=%q, want 200 and []", w.Code, w.Body.String())
	}

	req, _ = http.NewRequest("POST", "/api/gitlab-sync-queue", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	requireAuth(handleGitLabSyncQueue)(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST: code=%d, want 405", w.Code)
	}
}
