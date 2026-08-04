package main

import (
	"fmt"
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
