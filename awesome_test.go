package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildAwesomeReadmeGroupsByPlatform(t *testing.T) {
	targets := []GitLabSyncTarget{
		{RepoName: "kubernetes", RepoURL: "https://github.com/kubernetes/kubernetes", Platform: "github"},
		{RepoName: "zeta", RepoURL: "https://github.com/o/zeta", Platform: "github"},
		{RepoName: "some-gl-project", RepoURL: "https://gitlab.com/o/some-gl-project", Platform: "gitlab"},
	}
	out := buildAwesomeReadme(targets)

	githubIdx := strings.Index(out, "## Github")
	gitlabIdx := strings.Index(out, "## Gitlab")
	if githubIdx == -1 || gitlabIdx == -1 {
		t.Fatalf("expected both ## Github and ## Gitlab sections, got:\n%s", out)
	}
	// Alphabetical within a group: kubernetes before zeta.
	if strings.Index(out, "kubernetes") > strings.Index(out, "zeta") {
		t.Errorf("expected kubernetes to sort before zeta within the Github section:\n%s", out)
	}
	if !strings.Contains(out, "[kubernetes](https://github.com/kubernetes/kubernetes)") {
		t.Errorf("expected a markdown link for kubernetes, got:\n%s", out)
	}
}

func TestBuildAwesomeReadmeEmpty(t *testing.T) {
	out := buildAwesomeReadme(nil)
	if !strings.Contains(out, "No synced repos yet") {
		t.Errorf("expected empty-state message, got:\n%s", out)
	}
}

func TestSyncAwesomePageNoOpWhenDisabled(t *testing.T) {
	s := newTestStore(t)
	oldStore := store
	store = s
	defer func() { store = oldStore }()

	userID, _ := newTestAuth(t, s)
	if err := s.syncAwesomePage(userID); err != nil {
		t.Fatalf("syncAwesomePage() with no gitlab_settings row = %v, want nil (no-op)", err)
	}
}

func TestSyncAwesomePageCreatesAndPushes(t *testing.T) {
	s := newTestStore(t)
	oldStore := store
	store = s
	defer func() { store = oldStore }()

	userID, _ := newTestAuth(t, s)
	repoID, _ := s.AddRepo(userID, Project{Name: "repo", Platform: "github", RepoURL: "https://github.com/o/r"})

	bareDir := t.TempDir()
	runGitCmd(t, bareDir, "init", "--bare", "-b", "main")
	bareURL := "file://" + bareDir

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/my-awesome", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/api/v4/projects", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"http_url_to_repo": bareURL})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s.SaveGitLabSettings(userID, srv.URL, "tok")
	s.SetAwesomeConfig(userID, "my-awesome", true)
	s.SetProjectGitLabSync(userID, repoID, true, "daily")

	if err := s.syncAwesomePage(userID); err != nil {
		t.Fatalf("syncAwesomePage() error = %v", err)
	}

	settings, _ := s.GetGitLabSettings(userID)
	if settings.AwesomeGitLabPath != bareURL {
		t.Errorf("AwesomeGitLabPath = %q, want %q saved after creation", settings.AwesomeGitLabPath, bareURL)
	}

	out := runGitCmdOutput(t, bareDir, "log", "--oneline")
	if out == "" {
		t.Error("awesome mirror target has no commits after syncAwesomePage")
	}
}
