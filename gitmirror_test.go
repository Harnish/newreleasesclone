package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return string(out)
}

func makeOriginRepo(t *testing.T) (dir, url string) {
	t.Helper()
	dir = t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "init")
	return dir, "file://" + dir
}

func makeBareRepo(t *testing.T) (dir, url string) {
	t.Helper()
	dir = t.TempDir()
	runGit(t, dir, "init", "--bare", "-b", "main")
	return dir, "file://" + dir
}

func TestMirrorSyncRepo(t *testing.T) {
	_, originURL := makeOriginRepo(t)
	bareDir, bareURL := makeBareRepo(t)

	if err := mirrorSyncRepo(originURL, bareURL); err != nil {
		t.Fatalf("mirrorSyncRepo() error = %v", err)
	}

	out := runGit(t, bareDir, "log", "--oneline")
	if out == "" {
		t.Error("target bare repo has no commits after mirrorSyncRepo")
	}
}

func TestMirrorSyncRepoAdvancesOnSecondSync(t *testing.T) {
	originDir, originURL := makeOriginRepo(t)
	bareDir, bareURL := makeBareRepo(t)

	if err := mirrorSyncRepo(originURL, bareURL); err != nil {
		t.Fatalf("first mirrorSyncRepo() error = %v", err)
	}

	if err := os.WriteFile(filepath.Join(originDir, "new.txt"), []byte("more"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, originDir, "add", "new.txt")
	runGit(t, originDir, "commit", "-m", "second")
	runGit(t, originDir, "tag", "v1.0.0")
	wantSHA := strings.TrimSpace(runGit(t, originDir, "rev-parse", "refs/heads/main"))

	if err := mirrorSyncRepo(originURL, bareURL); err != nil {
		t.Fatalf("second mirrorSyncRepo() error = %v", err)
	}

	gotSHA := strings.TrimSpace(runGit(t, bareDir, "rev-parse", "refs/heads/main"))
	if gotSHA != wantSHA {
		t.Errorf("target refs/heads/main = %q, want %q after second sync", gotSHA, wantSHA)
	}
	if tags := runGit(t, bareDir, "tag", "-l"); !strings.Contains(tags, "v1.0.0") {
		t.Errorf("target tags = %q, want v1.0.0 mirrored", tags)
	}
}

func TestMirrorSyncRepoRedactsCredentials(t *testing.T) {
	_, originURL := makeOriginRepo(t)
	badPushURL := "https://oauth2:s3cr3t-token@127.0.0.1:1/nonexistent.git"

	err := mirrorSyncRepo(originURL, badPushURL)
	if err == nil {
		t.Fatal("mirrorSyncRepo() error = nil, want failure pushing to an unreachable host")
	}
	if strings.Contains(err.Error(), "s3cr3t-token") {
		t.Errorf("error leaks the gitlab token: %v", err)
	}
}

func TestPushGeneratedContent(t *testing.T) {
	_, bareURL := makeBareRepo(t)

	files := map[string]string{"README.md": "# Hello\n"}
	if err := pushGeneratedContent(bareURL, files); err != nil {
		t.Fatalf("pushGeneratedContent() error = %v", err)
	}

	clone := t.TempDir()
	runGit(t, "", "clone", bareURL, clone)
	content, err := os.ReadFile(filepath.Join(clone, "README.md"))
	if err != nil {
		t.Fatalf("read pushed README.md: %v", err)
	}
	if string(content) != "# Hello\n" {
		t.Errorf("README.md content = %q, want %q", content, "# Hello\n")
	}

	// A second push with different content must overwrite, not fail on
	// diverged history (the generated repo has no shared history across syncs).
	files["README.md"] = "# Updated\n"
	if err := pushGeneratedContent(bareURL, files); err != nil {
		t.Fatalf("second pushGeneratedContent() error = %v", err)
	}
}

func TestRedactArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"no credentials", []string{"fetch", "origin"}, "fetch origin"},
		{
			"https with token",
			[]string{"push", "--mirror", "--", "https://oauth2:tok@host/p.git"},
			"push --mirror -- https://oauth2@host/p.git",
		},
		{
			"plain https untouched",
			[]string{"clone", "--mirror", "https://github.com/o/r.git"},
			"clone --mirror https://github.com/o/r.git",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := redactArgs(tt.args); got != tt.want {
				t.Errorf("redactArgs() = %q, want %q", got, tt.want)
			}
		})
	}
}
