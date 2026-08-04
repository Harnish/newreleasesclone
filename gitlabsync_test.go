package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestDoSyncProjectToGitLabRequiresConfig(t *testing.T) {
	err := doSyncProjectToGitLab(GitLabSyncTarget{RepoURL: "https://github.com/o/r"})
	if err == nil {
		t.Error("doSyncProjectToGitLab() error = nil, want error when gitlab url/token are empty")
	}
}

func TestDoSyncProjectToGitLabCreatesAndPushes(t *testing.T) {
	s := newTestStore(t)
	oldStore := store
	store = s
	defer func() { store = oldStore }()
	userID, _ := newTestAuth(t, s)
	repoID, _ := s.AddRepo(userID, Project{Name: "my-repo", Platform: "github", RepoURL: "https://github.com/o/r"})

	// origin repo to mirror from
	originDir := t.TempDir()
	runGitCmd(t, originDir, "init", "-b", "main")
	runGitCmd(t, originDir, "config", "user.email", "test@example.com")
	runGitCmd(t, originDir, "config", "user.name", "Test")
	os.WriteFile(filepath.Join(originDir, "f.txt"), []byte("hi"), 0o644)
	runGitCmd(t, originDir, "add", "f.txt")
	runGitCmd(t, originDir, "commit", "-m", "init")
	originURL := "file://" + originDir

	// bare repo standing in for the gitlab-hosted mirror target
	bareDir := t.TempDir()
	runGitCmd(t, bareDir, "init", "--bare", "-b", "main")
	bareURL := "file://" + bareDir

	// fake gitlab API: project doesn't exist yet, CreateProject returns bareURL
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/my-repo", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/api/v4/projects", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{"http_url_to_repo": bareURL})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	target := GitLabSyncTarget{
		UserID:      userID,
		RepoID:      repoID,
		RepoName:    "my-repo",
		RepoURL:     originURL,
		Platform:    "github",
		GitLabURL:   srv.URL,
		GitLabToken: "tok",
	}
	if err := doSyncProjectToGitLab(target); err != nil {
		t.Fatalf("doSyncProjectToGitLab() error = %v", err)
	}

	out := runGitCmdOutput(t, bareDir, "log", "--oneline")
	if out == "" {
		t.Error("mirror target has no commits after sync")
	}
}

func TestSyncProjectToGitLabRecordsOutcome(t *testing.T) {
	s := newTestStore(t)
	oldStore := store
	store = s
	defer func() { store = oldStore }()

	userID, _ := newTestAuth(t, s)
	repoID, _ := s.AddRepo(userID, Project{Name: "repo", Platform: "github", RepoURL: "https://github.com/o/r"})
	// No gitlab_settings saved -> doSyncProjectToGitLab will fail fast.
	s.SetProjectGitLabSync(userID, repoID, true, "daily")

	target := GitLabSyncTarget{UserID: userID, RepoID: repoID, RepoURL: "https://github.com/o/r"}
	syncProjectToGitLab(target)

	projects := s.GetUserRepos(userID)
	if projects[0].LastGitLabSyncError == "" {
		t.Error("expected LastGitLabSyncError to be recorded after a failed sync")
	}
	if projects[0].LastGitLabSyncAt.IsZero() {
		t.Error("expected LastGitLabSyncAt to be set even on failure")
	}
}

func TestGitLabSyncFrequencyDuration(t *testing.T) {
	tests := []struct {
		freq string
		want time.Duration
	}{
		{"daily", 24 * time.Hour},
		{"weekly", 7 * 24 * time.Hour},
		{"monthly", 30 * 24 * time.Hour},
		{"", 24 * time.Hour},
		{"bogus", 24 * time.Hour},
	}
	for _, tt := range tests {
		if got := gitlabSyncFrequencyDuration(tt.freq); got != tt.want {
			t.Errorf("gitlabSyncFrequencyDuration(%q) = %v, want %v", tt.freq, got, tt.want)
		}
	}
}

func TestGitLabSyncDue(t *testing.T) {
	if !gitlabSyncDue(time.Time{}, "daily") {
		t.Error("gitlabSyncDue() with zero LastSyncAt should always be due")
	}
	if gitlabSyncDue(time.Now(), "daily") {
		t.Error("gitlabSyncDue() with a sync just now should not be due for daily")
	}
	if !gitlabSyncDue(time.Now().Add(-25*time.Hour), "daily") {
		t.Error("gitlabSyncDue() 25h ago should be due for daily")
	}
	if gitlabSyncDue(time.Now().Add(-25*time.Hour), "weekly") {
		t.Error("gitlabSyncDue() 25h ago should NOT be due for weekly")
	}
}

// runGitCmd/runGitCmdOutput are small test-only helpers distinct from the
// production gitRun() in gitmirror.go — kept local to this test file so it
// has no non-test dependency on gitmirror_test.go's runGit helper.
func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

func runGitCmdOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return string(out)
}
