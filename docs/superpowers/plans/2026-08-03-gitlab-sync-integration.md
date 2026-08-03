# GitLab Sync Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a newreleases user mirror a tracked GitHub/GitLab repo to their own GitLab instance on a daily/weekly/monthly schedule, and optionally maintain an auto-generated "Awesome" README of their synced repos as a GitLab project under their namespace.

**Architecture:** Three new files (`gitlabclient.go`, `gitmirror.go`, `awesome.go`) plus a `gitlabsync.go` orchestrator, all `package main`, following newreleases's flat-file convention. GitLab REST client and git mirror-clone/push logic are ported from `/home/jharnish/Work/syncrepos/syncrepos` (`internal/gitlab`, `internal/gitops`), simplified for ephemeral (no persistent local clone) and single-user-namespace use. Scheduling is a new hourly ticker goroutine (modeled on `runDailyDigest` in `scheduler.go`), not a port of syncrepos's `robfig/cron`-based scheduler.

**Tech Stack:** Go (`package main`, no new dependencies), SQLite via existing `modernc.org/sqlite` store, `git` CLI via `os/exec` (already available in the Docker builder stage; added to the final stage in Task 9), vanilla JS in `ui.go`'s embedded frontend.

## Global Constraints

- No CLI — this is web-app-only. Do not port syncrepos's `cobra` command structure.
- No new Go dependencies (no `robfig/cron`, no GitLab SDK).
- No persistent local git clones / `REPOS_DIR`. Every sync is an ephemeral `git clone --mirror` to a temp dir, followed by push, followed by cleanup.
- GitLab instance URL + API token are stored **per user** (table `gitlab_settings`, one row per `user_id`), not globally.
- GitLab sync fields (`gitlab_sync_enabled`, `gitlab_sync_frequency`, `gitlab_project_path`, `last_gitlab_sync_at`, `last_gitlab_sync_error`) live on the existing **`user_repos`** join table, not on `repos` — `repos` is deduplicated and shared across users (two users tracking the same repo share one row), but GitLab sync targets a specific user's own GitLab account, so it must be per-user-per-repo.
- GitLab sync is only offered for projects with `platform` = `github` or `gitlab` (the only platforms with a git-clonable `repo_url`).
- The Awesome page lists only a user's GitLab-sync-**enabled** projects, grouped by `platform`.
- `gitlab_token` is stored in plaintext, matching this codebase's existing webhook-secret storage pattern. API responses never include the raw token — only a `has_token` boolean, matching the existing `Webhook.HasSecret` pattern in `models.go`.
- Follow existing store/handler conventions exactly: string IDs (`fmt.Sprintf("prefix_%s", generateToken()[:12])` where applicable), `time.RFC3339` timestamp storage, versioned `PRAGMA user_version` migrations in `store.go`'s `migrate()`, `requireAuth`-wrapped handlers, `t.Helper()` + `newTestStore(t)` / `newTestAuth(t, s)` in tests.
- New tests go in new `*_test.go` files (one per new source file), not appended to the existing monolithic `main_test.go` — that file is already ~1900 lines and this is a large, distinct subsystem.

---

### Task 1: Schema migration + core GitLab settings/sync storage

**Files:**
- Modify: `store.go` (add to `migrate()`, add new `GitLabSettings`/sync-target CRUD methods)
- Modify: `models.go` (add `GitLabSettings` and `GitLabSyncTarget` types; extend `Project`)
- Test: `store_gitlab_test.go` (new file)

**Interfaces:**
- Produces (used by later tasks):
  - `type GitLabSettings struct { GitLabURL, GitLabToken string; AwesomeEnabled bool; AwesomeRepoName, AwesomeGitLabPath string }`
  - `type GitLabSyncTarget struct { UserID, RepoID, RepoName, RepoURL, Platform, GitLabURL, GitLabToken, GitLabProjectPath, Frequency string; LastSyncAt time.Time }`
  - `Project` gains: `GitLabSyncEnabled bool`, `GitLabSyncFrequency string`, `LastGitLabSyncAt time.Time`, `LastGitLabSyncError string` (all `json:` tagged, `omitempty` where zero-valued is the common case)
  - `(s *Store) SaveGitLabSettings(userID, gitlabURL, token string) error`
  - `(s *Store) GetGitLabSettings(userID string) (GitLabSettings, error)` — zero-value + nil error if no row exists (not-configured is a normal state, not an error)
  - `(s *Store) SetAwesomeConfig(userID, repoName string, enabled bool) error` — errors if no `gitlab_settings` row exists yet
  - `(s *Store) SetAwesomeGitLabPath(userID, httpURL string) error`
  - `(s *Store) SetProjectGitLabSync(userID, repoID string, enabled bool, frequency string) (bool, error)` — validates `platform` is `github`/`gitlab` when enabling; `bool` is false if no matching `user_repos` row
  - `(s *Store) SetProjectGitLabPath(userID, repoID, httpURL string) error`
  - `(s *Store) RecordGitLabSync(userID, repoID string, syncErr error) error`
  - `(s *Store) GetAllEnabledGitLabSyncTargets() []GitLabSyncTarget`
  - `(s *Store) GetUserGitLabSyncTargets(userID string) []GitLabSyncTarget`

- [ ] **Step 1: Write the failing tests**

Create `store_gitlab_test.go`:

```go
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

	if err := s.SaveGitLabSettings(userID, "https://gitlab.example.com", "tok123"); err != nil {
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
	if err := s.SaveGitLabSettings(userID, "https://gitlab2.example.com", "tok456"); err != nil {
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

	s.SaveGitLabSettings(userID, "https://gitlab.example.com", "tok")
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
	s.SaveGitLabSettings(userID, "https://gitlab.example.com", "tok")
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run 'TestSaveAndGetGitLabSettings|TestSetAwesomeConfigRequiresGitLabSettings|TestSetProjectGitLabSyncValidatesPlatform|TestSetProjectGitLabSyncEnableDisable|TestRecordGitLabSyncAndTargets|TestGetAllEnabledGitLabSyncTargetsRequiresGitLabSettings' -v .`
Expected: FAIL — `s.GetGitLabSettings` (and the rest) undefined.

- [ ] **Step 3: Add the migration**

In `store.go`, add after the `if version < 10 { ... }` block (before `return nil` that closes `migrate`):

```go
	if version < 11 {
		if _, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS gitlab_settings (
				user_id             TEXT PRIMARY KEY REFERENCES users(id),
				gitlab_url          TEXT NOT NULL DEFAULT '',
				gitlab_token        TEXT NOT NULL DEFAULT '',
				awesome_enabled     INTEGER NOT NULL DEFAULT 0,
				awesome_repo_name   TEXT NOT NULL DEFAULT '',
				awesome_gitlab_path TEXT NOT NULL DEFAULT '',
				updated_at          TEXT NOT NULL DEFAULT ''
			);
		`); err != nil {
			return fmt.Errorf("failed to migrate to v11 (gitlab_settings table): %w", err)
		}
		if _, err := db.Exec(`ALTER TABLE user_repos ADD COLUMN gitlab_sync_enabled INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("failed to migrate to v11 (gitlab_sync_enabled): %w", err)
		}
		if _, err := db.Exec(`ALTER TABLE user_repos ADD COLUMN gitlab_sync_frequency TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("failed to migrate to v11 (gitlab_sync_frequency): %w", err)
		}
		if _, err := db.Exec(`ALTER TABLE user_repos ADD COLUMN gitlab_project_path TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("failed to migrate to v11 (gitlab_project_path): %w", err)
		}
		if _, err := db.Exec(`ALTER TABLE user_repos ADD COLUMN last_gitlab_sync_at TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("failed to migrate to v11 (last_gitlab_sync_at): %w", err)
		}
		if _, err := db.Exec(`ALTER TABLE user_repos ADD COLUMN last_gitlab_sync_error TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("failed to migrate to v11 (last_gitlab_sync_error): %w", err)
		}
		if _, err := db.Exec("PRAGMA user_version = 11"); err != nil {
			return fmt.Errorf("failed to set schema version: %w", err)
		}
	}
```

- [ ] **Step 4: Add the types to `models.go`**

Append to `models.go`:

```go
type GitLabSettings struct {
	GitLabURL         string `json:"gitlab_url"`
	GitLabToken       string `json:"-"`
	AwesomeEnabled    bool   `json:"awesome_enabled"`
	AwesomeRepoName   string `json:"awesome_repo_name"`
	AwesomeGitLabPath string `json:"awesome_gitlab_path"`
}

type GitLabSyncTarget struct {
	UserID            string
	RepoID            string
	RepoName          string
	RepoURL           string
	Platform          string
	GitLabURL         string
	GitLabToken       string
	GitLabProjectPath string
	Frequency         string
	LastSyncAt        time.Time
}
```

And extend `Project`:

```go
type Project struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Platform            string    `json:"platform"`
	RepoURL             string    `json:"repo_url"`
	LastRefresh         time.Time `json:"last_refresh"`
	RefreshCount        int       `json:"refresh_count"`
	PushEnabled         bool      `json:"push_enabled"`
	GitLabSyncEnabled   bool      `json:"gitlab_sync_enabled"`
	GitLabSyncFrequency string    `json:"gitlab_sync_frequency,omitempty"`
	LastGitLabSyncAt    time.Time `json:"last_gitlab_sync_at,omitempty"`
	LastGitLabSyncError string    `json:"last_gitlab_sync_error,omitempty"`
}
```

- [ ] **Step 5: Add the Store methods**

Append to `store.go` (near the webhook/project-settings methods, e.g. after `SetProjectPushEnabled`):

```go
// ---- GitLab sync ----

func (s *Store) SaveGitLabSettings(userID, gitlabURL, token string) error {
	_, err := s.db.Exec(`
		INSERT INTO gitlab_settings (user_id, gitlab_url, gitlab_token, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			gitlab_url = excluded.gitlab_url,
			gitlab_token = excluded.gitlab_token,
			updated_at = excluded.updated_at`,
		userID, gitlabURL, token, time.Now().Format(time.RFC3339))
	return err
}

func (s *Store) GetGitLabSettings(userID string) (GitLabSettings, error) {
	var g GitLabSettings
	var awesomeEnabled int
	err := s.db.QueryRow(`
		SELECT gitlab_url, gitlab_token, awesome_enabled, awesome_repo_name, awesome_gitlab_path
		FROM gitlab_settings WHERE user_id = ?`, userID,
	).Scan(&g.GitLabURL, &g.GitLabToken, &awesomeEnabled, &g.AwesomeRepoName, &g.AwesomeGitLabPath)
	if err == sql.ErrNoRows {
		return GitLabSettings{}, nil
	}
	if err != nil {
		return GitLabSettings{}, err
	}
	g.AwesomeEnabled = awesomeEnabled != 0
	return g, nil
}

// SetAwesomeConfig requires a gitlab_settings row to already exist (the user
// must save their GitLab URL/token before enabling the Awesome page).
// Clears awesome_gitlab_path so a renamed repo gets recreated rather than
// reusing a stale project path.
func (s *Store) SetAwesomeConfig(userID, repoName string, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	res, err := s.db.Exec(`
		UPDATE gitlab_settings
		SET awesome_enabled = ?, awesome_repo_name = ?, awesome_gitlab_path = ''
		WHERE user_id = ?`,
		val, repoName, userID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("configure your gitlab instance before enabling the awesome page")
	}
	return nil
}

func (s *Store) SetAwesomeGitLabPath(userID, httpURL string) error {
	_, err := s.db.Exec(`UPDATE gitlab_settings SET awesome_gitlab_path = ? WHERE user_id = ?`, httpURL, userID)
	return err
}

// SetProjectGitLabSync enables/disables GitLab mirror sync for a project.
// Enabling requires the repo's platform be github or gitlab (the only
// platforms with a git-clonable repo_url). Disabling clears the stored
// frequency so a stale value can't resurface on a future re-enable that
// omits it.
func (s *Store) SetProjectGitLabSync(userID, repoID string, enabled bool, frequency string) (bool, error) {
	if enabled {
		var platform string
		if err := s.db.QueryRow("SELECT platform FROM repos WHERE id = ?", repoID).Scan(&platform); err != nil {
			return false, fmt.Errorf("repo not found: %w", err)
		}
		if platform != "github" && platform != "gitlab" {
			return false, fmt.Errorf("gitlab sync is only supported for github and gitlab projects")
		}
	}
	enabledVal := 0
	freqVal := frequency
	if enabled {
		enabledVal = 1
	} else {
		freqVal = ""
	}
	res, err := s.db.Exec(
		"UPDATE user_repos SET gitlab_sync_enabled = ?, gitlab_sync_frequency = ? WHERE user_id = ? AND repo_id = ?",
		enabledVal, freqVal, userID, repoID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *Store) SetProjectGitLabPath(userID, repoID, httpURL string) error {
	_, err := s.db.Exec(
		"UPDATE user_repos SET gitlab_project_path = ? WHERE user_id = ? AND repo_id = ?",
		httpURL, userID, repoID,
	)
	return err
}

func (s *Store) RecordGitLabSync(userID, repoID string, syncErr error) error {
	errMsg := ""
	if syncErr != nil {
		errMsg = syncErr.Error()
	}
	_, err := s.db.Exec(`
		UPDATE user_repos
		SET last_gitlab_sync_at = ?, last_gitlab_sync_error = ?
		WHERE user_id = ? AND repo_id = ?`,
		time.Now().Format(time.RFC3339), errMsg, userID, repoID)
	return err
}

// GetAllEnabledGitLabSyncTargets returns every user's GitLab-sync-enabled
// projects, across all users. Requires the user to have saved gitlab_settings
// (inner join) — a project enabled before configuring GitLab simply won't
// appear here until settings are saved.
func (s *Store) GetAllEnabledGitLabSyncTargets() []GitLabSyncTarget {
	rows, err := s.db.Query(`
		SELECT ur.user_id, r.id, r.name, r.repo_url, r.platform,
		       gs.gitlab_url, gs.gitlab_token,
		       ur.gitlab_project_path, ur.gitlab_sync_frequency, ur.last_gitlab_sync_at
		FROM user_repos ur
		INNER JOIN repos r ON r.id = ur.repo_id
		INNER JOIN gitlab_settings gs ON gs.user_id = ur.user_id
		WHERE ur.gitlab_sync_enabled = 1`)
	if err != nil {
		log.Printf("⚠ Failed to query gitlab sync targets: %v", err)
		return nil
	}
	defer rows.Close()
	return scanGitLabSyncTargets(rows)
}

func (s *Store) GetUserGitLabSyncTargets(userID string) []GitLabSyncTarget {
	rows, err := s.db.Query(`
		SELECT ur.user_id, r.id, r.name, r.repo_url, r.platform,
		       gs.gitlab_url, gs.gitlab_token,
		       ur.gitlab_project_path, ur.gitlab_sync_frequency, ur.last_gitlab_sync_at
		FROM user_repos ur
		INNER JOIN repos r ON r.id = ur.repo_id
		INNER JOIN gitlab_settings gs ON gs.user_id = ur.user_id
		WHERE ur.gitlab_sync_enabled = 1 AND ur.user_id = ?`, userID)
	if err != nil {
		log.Printf("⚠ Failed to query user gitlab sync targets: %v", err)
		return nil
	}
	defer rows.Close()
	return scanGitLabSyncTargets(rows)
}

func scanGitLabSyncTargets(rows *sql.Rows) []GitLabSyncTarget {
	var targets []GitLabSyncTarget
	for rows.Next() {
		var t GitLabSyncTarget
		var lastSync string
		if err := rows.Scan(&t.UserID, &t.RepoID, &t.RepoName, &t.RepoURL, &t.Platform,
			&t.GitLabURL, &t.GitLabToken, &t.GitLabProjectPath, &t.Frequency, &lastSync); err != nil {
			log.Printf("⚠ Failed to scan gitlab sync target: %v", err)
			continue
		}
		if lastSync != "" {
			t.LastSyncAt, _ = time.Parse(time.RFC3339, lastSync)
		}
		targets = append(targets, t)
	}
	return targets
}
```

- [ ] **Step 6: Extend `GetUserRepos` to select the new columns**

In `store.go`, replace the existing `GetUserRepos` function body with:

```go
func (s *Store) GetUserRepos(userID string) []Project {
	rows, err := s.db.Query(`
		SELECT r.id, r.name, r.platform, r.repo_url, r.last_refresh, r.refresh_count, ur.push_enabled,
		       ur.gitlab_sync_enabled, ur.gitlab_sync_frequency, ur.last_gitlab_sync_at, ur.last_gitlab_sync_error
		FROM repos r
		INNER JOIN user_repos ur ON ur.repo_id = r.id
		WHERE ur.user_id = ?
		ORDER BY ur.created_at ASC`, userID)
	if err != nil {
		log.Printf("⚠ Failed to query user repos: %v", err)
		return []Project{}
	}
	defer rows.Close()
	projects := []Project{}
	for rows.Next() {
		var p Project
		var lastRefresh, lastGitLabSync string
		var pushEnabled, gitlabSyncEnabled int
		if err := rows.Scan(&p.ID, &p.Name, &p.Platform, &p.RepoURL, &lastRefresh, &p.RefreshCount, &pushEnabled,
			&gitlabSyncEnabled, &p.GitLabSyncFrequency, &lastGitLabSync, &p.LastGitLabSyncError); err != nil {
			log.Printf("⚠ Failed to scan project: %v", err)
			continue
		}
		if lastRefresh != "" {
			p.LastRefresh, _ = time.Parse(time.RFC3339, lastRefresh)
		}
		if lastGitLabSync != "" {
			p.LastGitLabSyncAt, _ = time.Parse(time.RFC3339, lastGitLabSync)
		}
		p.PushEnabled = pushEnabled != 0
		p.GitLabSyncEnabled = gitlabSyncEnabled != 0
		projects = append(projects, p)
	}
	return projects
}
```

- [ ] **Step 7: Run tests to verify they pass**

Run: `go test -run 'TestSaveAndGetGitLabSettings|TestSetAwesomeConfigRequiresGitLabSettings|TestSetProjectGitLabSyncValidatesPlatform|TestSetProjectGitLabSyncEnableDisable|TestRecordGitLabSyncAndTargets|TestGetAllEnabledGitLabSyncTargetsRequiresGitLabSettings' -v .`
Expected: PASS. Also run `go test ./...` to confirm nothing existing broke (the `GetUserRepos` signature/query change is the main risk).

- [ ] **Step 8: Commit**

```bash
git add store.go models.go store_gitlab_test.go
git commit -m "feat: add GitLab sync settings and per-project sync state to store"
```

---

### Task 2: GitLab REST client

**Files:**
- Create: `gitlabclient.go`
- Test: `gitlabclient_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks (standalone, stdlib-only).
- Produces:
  - `func newGitLabClient(baseURL, token string) *GitLabClient`
  - `(c *GitLabClient) ProjectExists(fullPath string) (bool, error)`
  - `(c *GitLabClient) GetProjectHTTPURL(fullPath string) (string, error)`
  - `(c *GitLabClient) CreateProject(name string) (httpURLToRepo string, err error)` — creates under the token owner's personal namespace
  - `(c *GitLabClient) AuthenticatedPushURL(httpURLToRepo string) (string, error)`
  - `func slugify(name string) string`

Ported from `/home/jharnish/Work/syncrepos/syncrepos/internal/gitlab/client.go`, with `GroupExists`/`EnsureGroupPath`/`createSubgroup`/`SetToken`/`HasToken` deliberately dropped: newreleases creates each mirror project directly under the token owner's own personal GitLab namespace (no subgroup nesting needed), and a fresh client is constructed per sync rather than reused with a live-swappable token.

- [ ] **Step 1: Write the failing tests**

Create `gitlabclient_test.go`:

```go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitLabProjectExistsAndCreateProject(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/my-repo", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"http_url_to_repo": "https://example.com/user/my-repo.git",
		})
	})
	mux.HandleFunc("/api/v4/projects/missing-repo", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	var createdBody map[string]any
	mux.HandleFunc("/api/v4/projects", func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&createdBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"http_url_to_repo": "https://example.com/user/new-repo.git",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newGitLabClient(srv.URL, "tok")

	exists, err := c.ProjectExists("my-repo")
	if err != nil || !exists {
		t.Fatalf("ProjectExists() = (%v, %v), want (true, nil)", exists, err)
	}
	exists, err = c.ProjectExists("missing-repo")
	if err != nil || exists {
		t.Fatalf("ProjectExists() = (%v, %v), want (false, nil)", exists, err)
	}

	httpURL, err := c.CreateProject("new-repo")
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if httpURL != "https://example.com/user/new-repo.git" {
		t.Errorf("CreateProject() = %q, want the http_url_to_repo value", httpURL)
	}
	if _, hasNamespace := createdBody["namespace_id"]; hasNamespace {
		t.Errorf("CreateProject() sent namespace_id = %v, want it omitted (personal namespace default)", createdBody["namespace_id"])
	}

	got, err := c.GetProjectHTTPURL("my-repo")
	if err != nil || got != "https://example.com/user/my-repo.git" {
		t.Fatalf("GetProjectHTTPURL() = (%q, %v), want the http_url_to_repo value", got, err)
	}
}

func TestGitLabAuthenticatedPushURL(t *testing.T) {
	c := newGitLabClient("https://gitlab.example.com", "glpat-secret")

	got, err := c.AuthenticatedPushURL("https://gitlab.example.com/user/repo.git")
	if err != nil {
		t.Fatalf("AuthenticatedPushURL() error = %v", err)
	}
	want := "https://oauth2:glpat-secret@gitlab.example.com/user/repo.git"
	if got != want {
		t.Errorf("AuthenticatedPushURL() = %q, want %q", got, want)
	}

	if _, err := c.AuthenticatedPushURL("://not a url"); err == nil {
		t.Error("AuthenticatedPushURL() error = nil, want parse error")
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct{ in, want string }{
		{"My Repo", "my-repo"},
		{"kubernetes", "kubernetes"},
		{"foo_bar/baz", "foo-bar-baz"},
		{"  leading and trailing  ", "leading-and-trailing"},
	}
	for _, tt := range tests {
		if got := slugify(tt.in); got != tt.want {
			t.Errorf("slugify(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run 'TestGitLabProjectExistsAndCreateProject|TestGitLabAuthenticatedPushURL|TestSlugify' -v .`
Expected: FAIL — `newGitLabClient` undefined.

- [ ] **Step 3: Write the implementation**

Create `gitlabclient.go`:

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GitLabClient is a minimal GitLab REST v4 client, ported from
// /home/jharnish/Work/syncrepos/syncrepos/internal/gitlab/client.go.
// ponytail: dropped GroupExists/EnsureGroupPath/SetToken/HasToken from the
// original — newreleases creates each mirror project directly under the
// token owner's personal GitLab namespace (namespace_id omitted from
// CreateProject), so subgroup management isn't needed, and a fresh client
// is constructed per sync rather than reused with a live-swappable token.
type GitLabClient struct {
	baseURL string
	token   string
	http    *http.Client
}

func newGitLabClient(baseURL, token string) *GitLabClient {
	return &GitLabClient{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *GitLabClient) ProjectExists(fullPath string) (bool, error) {
	status, err := c.do(http.MethodGet, "/projects/"+url.PathEscape(fullPath), nil, nil)
	if err != nil {
		return false, err
	}
	if status == http.StatusNotFound {
		return false, nil
	}
	if status != http.StatusOK {
		return false, fmt.Errorf("gitlab: unexpected status %d checking project %s", status, fullPath)
	}
	return true, nil
}

// GetProjectHTTPURL fetches an existing project's http_url_to_repo, used
// instead of CreateProject when ProjectExists already reported true.
func (c *GitLabClient) GetProjectHTTPURL(fullPath string) (string, error) {
	var body struct {
		HTTPURLToRepo string `json:"http_url_to_repo"`
	}
	status, err := c.do(http.MethodGet, "/projects/"+url.PathEscape(fullPath), nil, &body)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("gitlab: unexpected status %d getting project %s", status, fullPath)
	}
	return body.HTTPURLToRepo, nil
}

// CreateProject creates a private project named name directly under the
// token owner's personal GitLab namespace (namespace_id omitted — GitLab
// defaults to the authenticated user's own namespace).
func (c *GitLabClient) CreateProject(name string) (httpURLToRepo string, err error) {
	reqBody := map[string]any{
		"name":       name,
		"path":       name,
		"visibility": "private",
	}
	var body struct {
		HTTPURLToRepo string `json:"http_url_to_repo"`
	}
	status, err := c.do(http.MethodPost, "/projects", reqBody, &body)
	if err != nil {
		return "", err
	}
	if status != http.StatusCreated {
		return "", fmt.Errorf("gitlab: create project %s failed, status %d", name, status)
	}
	return body.HTTPURLToRepo, nil
}

// AuthenticatedPushURL embeds the client's token in a project's
// http_url_to_repo so git can push over HTTPS without SSH keys. The result
// contains a secret — keep it out of logs and error messages.
func (c *GitLabClient) AuthenticatedPushURL(httpURLToRepo string) (string, error) {
	u, err := url.Parse(httpURLToRepo)
	if err != nil {
		return "", fmt.Errorf("parse gitlab http url %q: %w", httpURLToRepo, err)
	}
	u.User = url.UserPassword("oauth2", c.token)
	return u.String(), nil
}

func (c *GitLabClient) do(method, path string, reqBody, out any) (int, error) {
	var bodyReader *bytes.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return 0, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req, err := http.NewRequest(method, c.baseURL+"/api/v4"+path, bodyReader)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("gitlab request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if out != nil && resp.StatusCode < 300 {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return resp.StatusCode, fmt.Errorf("decode response for %s %s: %w", method, path, err)
		}
	}
	return resp.StatusCode, nil
}

// slugify lowercases name and collapses runs of non-alphanumeric characters
// into a single hyphen, producing a GitLab-safe project path segment.
func slugify(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run 'TestGitLabProjectExistsAndCreateProject|TestGitLabAuthenticatedPushURL|TestSlugify' -v .`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add gitlabclient.go gitlabclient_test.go
git commit -m "feat: port GitLab REST client for personal-namespace project mirroring"
```

---

### Task 3: Ephemeral git mirror clone/push primitives

**Files:**
- Create: `gitmirror.go`
- Test: `gitmirror_test.go`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces:
  - `func mirrorSyncRepo(remoteURL, pushURL string) error`
  - `func pushGeneratedContent(pushURL string, files map[string]string) error`
  - `func redactArgs(args []string) string` (internal, but referenced by name in tests)

Adapted from `/home/jharnish/Work/syncrepos/syncrepos/internal/gitops/gitops.go`. Because each sync uses a fresh `git clone --mirror` (bare) rather than syncrepos's persistent non-bare working clone, a plain `git push --mirror` works correctly here — syncrepos needed the `refs/remotes/origin/* → refs/heads/*` remapping trick specifically to work around a non-bare clone's `fetch` only updating remote-tracking refs; a bare mirror clone's `refs/heads/*` **are** the origin's, so no remapping is needed.

- [ ] **Step 1: Write the failing tests**

Create `gitmirror_test.go`:

```go
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
	bareDir, bareURL := makeBareRepo(t)

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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run 'TestMirrorSyncRepo|TestMirrorSyncRepoAdvancesOnSecondSync|TestMirrorSyncRepoRedactsCredentials|TestPushGeneratedContent|TestRedactArgs' -v .`
Expected: FAIL — `mirrorSyncRepo` undefined.

- [ ] **Step 3: Write the implementation**

Create `gitmirror.go`:

```go
package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// mirrorSyncRepo does a full ephemeral mirror sync: bare mirror-clone
// remoteURL to a temp dir, push --mirror to pushURL, then remove the temp
// dir. Every sync transfers the full repo; there is no incremental fetch
// and no state carried between syncs.
// ponytail: no persistent REPOS_DIR (unlike syncrepos) — sync cadence here
// is daily/weekly/monthly, so re-cloning each time costs nothing that
// matters and avoids the drift/repair problem persistent clones create.
func mirrorSyncRepo(remoteURL, pushURL string) error {
	dir, err := os.MkdirTemp("", "glsync-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	if err := gitRun("", "clone", "--mirror", "--", remoteURL, dir); err != nil {
		return err
	}
	if err := gitRun(dir, "push", "--mirror", "--", pushURL); err != nil {
		return err
	}
	return nil
}

// pushGeneratedContent commits files (path -> content) into a fresh repo
// and force-pushes it to pushURL as the sole commit on main. Used for the
// Awesome-page README, which is regenerated fresh on every sync rather than
// incrementally edited, so a force-push is expected and safe.
func pushGeneratedContent(pushURL string, files map[string]string) error {
	dir, err := os.MkdirTemp("", "glsync-gen-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	if err := gitRun(dir, "init", "-b", "main"); err != nil {
		return err
	}
	if err := gitRun(dir, "config", "user.email", "newreleases@localhost"); err != nil {
		return err
	}
	if err := gitRun(dir, "config", "user.name", "newreleases"); err != nil {
		return err
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
		if err := gitRun(dir, "add", "--", name); err != nil {
			return err
		}
	}
	if err := gitRun(dir, "commit", "-m", "Update: "+time.Now().UTC().Format(time.RFC3339)); err != nil {
		return err
	}
	return gitRun(dir, "push", "--force", "--", pushURL, "HEAD:main")
}

// redactArgs strips userinfo passwords out of any URL-shaped argument so a
// failed git command can't echo the gitlab token into an error string. Git
// redacts credentials in its own output already; the args are ours to scrub.
func redactArgs(args []string) string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = a
		if !strings.Contains(a, "://") || !strings.Contains(a, "@") {
			continue
		}
		u, err := url.Parse(a)
		if err != nil {
			out[i] = "[redacted]"
			continue
		}
		if u.User != nil {
			u.User = url.User(u.User.Username())
			out[i] = u.String()
		}
	}
	return strings.Join(out, " ")
}

func gitRun(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git %s: %w: %s", redactArgs(args), err, strings.TrimSpace(string(out)))
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run 'TestMirrorSyncRepo|TestMirrorSyncRepoAdvancesOnSecondSync|TestMirrorSyncRepoRedactsCredentials|TestPushGeneratedContent|TestRedactArgs' -v .`
Expected: PASS. (Requires `git` on PATH — already true in this dev environment and in the Docker builder stage.)

- [ ] **Step 5: Commit**

```bash
git add gitmirror.go gitmirror_test.go
git commit -m "feat: add ephemeral git mirror clone/push primitives"
```

---

### Task 4: Sync orchestration + scheduler

**Files:**
- Create: `gitlabsync.go`
- Modify: `scheduler.go` (add frequency helpers + `runGitLabSyncScheduler`)
- Test: `gitlabsync_test.go`

**Interfaces:**
- Consumes:
  - `GitLabSyncTarget`, `(s *Store) RecordGitLabSync`, `(s *Store) SetProjectGitLabPath`, `(s *Store) GetAllEnabledGitLabSyncTargets` (Task 1)
  - `newGitLabClient`, `slugify` (Task 2)
  - `mirrorSyncRepo` (Task 3)
- Produces:
  - `func syncProjectToGitLab(t GitLabSyncTarget)` — the goroutine entrypoint; never panics, always records the outcome
  - `func doSyncProjectToGitLab(t GitLabSyncTarget) error` — the testable core (no logging/store side effects beyond what it returns)
  - `func gitlabSyncFrequencyDuration(freq string) time.Duration`
  - `func gitlabSyncDue(lastSyncAt time.Time, freq string) bool`
  - `func runGitLabSyncScheduler()` — goroutine, started from `main.go` in Task 6
  - `func checkDueGitLabSyncs()`

Task 6's `syncProjectToGitLab` will also call `syncAwesomePage` (Task 5) — that call is added in Task 5, not here, to keep this task's tests independent of the Awesome-page code.

- [ ] **Step 1: Write the failing tests**

Create `gitlabsync_test.go`:

```go
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
		UserID:    "u1",
		RepoID:    "r1",
		RepoName:  "my-repo",
		RepoURL:   originURL,
		Platform:  "github",
		GitLabURL: srv.URL,
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run 'TestDoSyncProjectToGitLab|TestSyncProjectToGitLabRecordsOutcome|TestGitLabSyncFrequencyDuration|TestGitLabSyncDue' -v .`
Expected: FAIL — `doSyncProjectToGitLab` undefined.

- [ ] **Step 3: Write `gitlabsync.go`**

```go
package main

import (
	"fmt"
	"log"
)

// syncProjectToGitLab performs one GitLab mirror sync for t and records the
// outcome. Errors are recorded on the target's user_repos row rather than
// returned loudly — callers run this in a goroutine and must never crash
// the process on a bad token or network failure.
func syncProjectToGitLab(t GitLabSyncTarget) {
	err := doSyncProjectToGitLab(t)
	if err != nil {
		log.Printf("⚠ gitlab sync failed for user=%s repo=%s: %v", t.UserID, t.RepoID, err)
	}
	if recErr := store.RecordGitLabSync(t.UserID, t.RepoID, err); recErr != nil {
		log.Printf("⚠ failed to record gitlab sync outcome for user=%s repo=%s: %v", t.UserID, t.RepoID, recErr)
	}
}

// doSyncProjectToGitLab ensures the mirror project exists under the user's
// GitLab namespace, then ephemerally clones+pushes the upstream repo. It has
// no side effects on the store beyond what the caller does with its return
// value, so it can be tested without an in-memory Store.
func doSyncProjectToGitLab(t GitLabSyncTarget) error {
	if t.GitLabURL == "" || t.GitLabToken == "" {
		return fmt.Errorf("gitlab instance not configured")
	}
	client := newGitLabClient(t.GitLabURL, t.GitLabToken)

	httpURL := t.GitLabProjectPath
	if httpURL == "" {
		path := slugify(t.RepoName)
		exists, err := client.ProjectExists(path)
		if err != nil {
			return fmt.Errorf("check mirror project: %w", err)
		}
		if exists {
			httpURL, err = client.GetProjectHTTPURL(path)
		} else {
			httpURL, err = client.CreateProject(path)
		}
		if err != nil {
			return fmt.Errorf("create/get mirror project: %w", err)
		}
		if err := store.SetProjectGitLabPath(t.UserID, t.RepoID, httpURL); err != nil {
			return fmt.Errorf("save mirror project path: %w", err)
		}
	}

	pushURL, err := client.AuthenticatedPushURL(httpURL)
	if err != nil {
		return err
	}
	return mirrorSyncRepo(t.RepoURL, pushURL)
}
```

- [ ] **Step 4: Add scheduling helpers to `scheduler.go`**

Append to `scheduler.go`:

```go
// ---- GitLab sync scheduling ----

func gitlabSyncFrequencyDuration(freq string) time.Duration {
	switch freq {
	case "weekly":
		return 7 * 24 * time.Hour
	case "monthly":
		return 30 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}

func gitlabSyncDue(lastSyncAt time.Time, freq string) bool {
	if lastSyncAt.IsZero() {
		return true
	}
	return time.Since(lastSyncAt) > gitlabSyncFrequencyDuration(freq)
}

// runGitLabSyncScheduler checks hourly for GitLab-sync-enabled projects that
// are due per their daily/weekly/monthly frequency and fires a background
// sync for each. Hourly (not once-daily like runDailyDigest) because
// frequency is per-project and the coarsest granularity (daily) needs finer
// polling than a once-a-day tick to fire promptly.
func runGitLabSyncScheduler() {
	checkDueGitLabSyncs()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		checkDueGitLabSyncs()
	}
}

func checkDueGitLabSyncs() {
	for _, t := range store.GetAllEnabledGitLabSyncTargets() {
		if gitlabSyncDue(t.LastSyncAt, t.Frequency) {
			go syncProjectToGitLab(t)
		}
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test -run 'TestDoSyncProjectToGitLab|TestSyncProjectToGitLabRecordsOutcome|TestGitLabSyncFrequencyDuration|TestGitLabSyncDue' -v .`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add gitlabsync.go gitlabsync_test.go scheduler.go
git commit -m "feat: add GitLab sync orchestration and hourly due-check scheduler"
```

---

### Task 5: Awesome page generator

**Files:**
- Create: `awesome.go`
- Test: `awesome_test.go`

**Interfaces:**
- Consumes:
  - `GitLabSyncTarget`, `(s *Store) GetGitLabSettings`, `(s *Store) SetAwesomeGitLabPath`, `(s *Store) GetUserGitLabSyncTargets` (Task 1)
  - `newGitLabClient`, `slugify` (Task 2)
  - `pushGeneratedContent` (Task 3)
- Produces:
  - `func buildAwesomeReadme(targets []GitLabSyncTarget) string` — pure function
  - `func syncAwesomePage(userID string) error`

Also modifies `syncProjectToGitLab` (Task 4) to call `syncAwesomePage` after a successful project sync.

- [ ] **Step 1: Write the failing tests**

Create `awesome_test.go`:

```go
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
	if err := syncAwesomePage(userID); err != nil {
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

	if err := syncAwesomePage(userID); err != nil {
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run 'TestBuildAwesomeReadme|TestSyncAwesomePage' -v .`
Expected: FAIL — `buildAwesomeReadme` undefined.

- [ ] **Step 3: Write `awesome.go`**

```go
package main

import (
	"fmt"
	"sort"
	"strings"
)

// buildAwesomeReadme renders a Markdown README listing targets grouped by
// platform (sorted alphabetically), alphabetically by repo name within each
// group. Pure function — no I/O — so output can be asserted directly.
func buildAwesomeReadme(targets []GitLabSyncTarget) string {
	groups := map[string][]GitLabSyncTarget{}
	for _, t := range targets {
		groups[t.Platform] = append(groups[t.Platform], t)
	}
	var platforms []string
	for p := range groups {
		platforms = append(platforms, p)
	}
	sort.Strings(platforms)

	var b strings.Builder
	b.WriteString("# Awesome Repos\n\n")
	b.WriteString("Auto-generated by [newreleases](https://github.com/Harnish/newreleases). Do not edit by hand.\n\n")
	if len(platforms) == 0 {
		b.WriteString("_No synced repos yet._\n")
		return b.String()
	}
	for _, p := range platforms {
		items := groups[p]
		sort.Slice(items, func(i, j int) bool { return items[i].RepoName < items[j].RepoName })
		title := strings.ToUpper(p[:1]) + p[1:]
		b.WriteString(fmt.Sprintf("## %s\n\n", title))
		for _, t := range items {
			b.WriteString(fmt.Sprintf("- [%s](%s)\n", t.RepoName, t.RepoURL))
		}
		b.WriteString("\n")
	}
	return b.String()
}

// syncAwesomePage regenerates and pushes the Awesome README for userID. A
// no-op returning nil if the user hasn't enabled the feature.
func syncAwesomePage(userID string) error {
	settings, err := store.GetGitLabSettings(userID)
	if err != nil {
		return err
	}
	if !settings.AwesomeEnabled {
		return nil
	}
	if settings.GitLabToken == "" {
		return fmt.Errorf("gitlab token not configured")
	}
	client := newGitLabClient(settings.GitLabURL, settings.GitLabToken)

	httpURL := settings.AwesomeGitLabPath
	if httpURL == "" {
		path := slugify(settings.AwesomeRepoName)
		exists, err := client.ProjectExists(path)
		if err != nil {
			return fmt.Errorf("check awesome project: %w", err)
		}
		if exists {
			httpURL, err = client.GetProjectHTTPURL(path)
		} else {
			httpURL, err = client.CreateProject(path)
		}
		if err != nil {
			return fmt.Errorf("create/get awesome project: %w", err)
		}
		if err := store.SetAwesomeGitLabPath(userID, httpURL); err != nil {
			return fmt.Errorf("save awesome project path: %w", err)
		}
	}

	pushURL, err := client.AuthenticatedPushURL(httpURL)
	if err != nil {
		return err
	}
	targets := store.GetUserGitLabSyncTargets(userID)
	content := buildAwesomeReadme(targets)
	return pushGeneratedContent(pushURL, map[string]string{"README.md": content})
}
```

- [ ] **Step 4: Wire Awesome regeneration into `syncProjectToGitLab`**

In `gitlabsync.go`, update `syncProjectToGitLab`:

```go
func syncProjectToGitLab(t GitLabSyncTarget) {
	err := doSyncProjectToGitLab(t)
	if err != nil {
		log.Printf("⚠ gitlab sync failed for user=%s repo=%s: %v", t.UserID, t.RepoID, err)
	}
	if recErr := store.RecordGitLabSync(t.UserID, t.RepoID, err); recErr != nil {
		log.Printf("⚠ failed to record gitlab sync outcome for user=%s repo=%s: %v", t.UserID, t.RepoID, recErr)
	}
	if err == nil {
		if awErr := syncAwesomePage(t.UserID); awErr != nil {
			log.Printf("⚠ awesome page sync failed for user=%s: %v", t.UserID, awErr)
		}
	}
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test -run 'TestBuildAwesomeReadme|TestSyncAwesomePage' -v .`
Expected: PASS. Then run `go test ./...` for the full suite (this step also re-verifies Task 4's `TestSyncProjectToGitLabRecordsOutcome`, which now also exercises the new `syncAwesomePage` call path when `err == nil` — in that test the sync itself fails, so `syncAwesomePage` is not reached; no behavior change expected there).

- [ ] **Step 6: Commit**

```bash
git add awesome.go awesome_test.go gitlabsync.go
git commit -m "feat: generate and push an Awesome-list README grouped by platform"
```

---

### Task 6: HTTP handlers + route/scheduler wiring

**Files:**
- Modify: `handlers.go` (add 4 new handlers)
- Modify: `main.go` (register routes, start scheduler goroutine)
- Test: `handlers_gitlab_test.go` (new file)

**Interfaces:**
- Consumes: everything from Tasks 1–5.
- Produces (routes):
  - `GET/POST /api/gitlab-settings`
  - `POST /api/gitlab-settings/awesome`
  - `POST /api/project-gitlab-sync`
  - `POST /api/project-gitlab-sync/sync-now?repo_id=...`

- [ ] **Step 1: Write the failing tests**

Create `handlers_gitlab_test.go`:

```go
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
	userID, cookie := newTestAuth(t, s)

	req := httptest.NewRequest(http.MethodGet, "/api/gitlab-settings", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleGitLabSettings)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["has_token"] != false {
		t.Errorf("has_token = %v, want false for unconfigured user %s", body["has_token"], userID)
	}
}

func TestHandleGitLabSettingsPOST(t *testing.T) {
	s := newTestStore(t)
	oldStore := store
	store = s
	defer func() { store = oldStore }()
	_, cookie := newTestAuth(t, s)

	body, _ := json.Marshal(map[string]string{"gitlab_url": "https://gitlab.example.com", "gitlab_token": "tok"})
	req := httptest.NewRequest(http.MethodPost, "/api/gitlab-settings", bytes.NewReader(body))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleGitLabSettings)(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/gitlab-settings", nil)
	getReq.AddCookie(cookie)
	getW := httptest.NewRecorder()
	requireAuth(handleGitLabSettings)(getW, getReq)
	var got map[string]any
	json.NewDecoder(getW.Body).Decode(&got)
	if got["has_token"] != true || got["gitlab_url"] != "https://gitlab.example.com" {
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
	requireAuth(handleGitLabSettings)(w, req)

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
	requireAuth(handleProjectGitLabSync)(w, req)

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
	s.SaveGitLabSettings(userID, "https://gitlab.example.com", "tok")

	body, _ := json.Marshal(map[string]any{"repo_id": repoID, "enabled": true, "frequency": "weekly"})
	req := httptest.NewRequest(http.MethodPost, "/api/project-gitlab-sync", bytes.NewReader(body))
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleProjectGitLabSync)(w, req)

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
	requireAuth(handleProjectGitLabSyncNow)(w, req)

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
	requireAuth(handleGitLabAwesome)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 when enabling awesome before gitlab-settings are configured, body: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run 'TestHandleGitLabSettings|TestHandleProjectGitLabSync|TestHandleGitLabAwesome' -v .`
Expected: FAIL — handlers undefined.

- [ ] **Step 3: Write the handlers**

Append to `handlers.go`:

```go
// GET /api/gitlab-settings — returns the user's GitLab instance config (token omitted).
// POST /api/gitlab-settings — saves gitlab_url + gitlab_token.
func handleGitLabSettings(w http.ResponseWriter, r *http.Request, userID string) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		settings, err := store.GetGitLabSettings(userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"gitlab_url":          settings.GitLabURL,
			"has_token":           settings.GitLabToken != "",
			"awesome_enabled":     settings.AwesomeEnabled,
			"awesome_repo_name":   settings.AwesomeRepoName,
			"awesome_gitlab_path": settings.AwesomeGitLabPath,
		})

	case http.MethodPost:
		var req struct {
			GitLabURL   string `json:"gitlab_url"`
			GitLabToken string `json:"gitlab_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.GitLabURL == "" || req.GitLabToken == "" {
			http.Error(w, "gitlab_url and gitlab_token are required", http.StatusBadRequest)
			return
		}
		if !strings.HasPrefix(req.GitLabURL, "http://") && !strings.HasPrefix(req.GitLabURL, "https://") {
			http.Error(w, "gitlab_url must start with http:// or https://", http.StatusBadRequest)
			return
		}
		if err := store.SaveGitLabSettings(userID, req.GitLabURL, req.GitLabToken); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// POST /api/gitlab-settings/awesome — enable/disable the Awesome page and set its repo name.
func handleGitLabAwesome(w http.ResponseWriter, r *http.Request, userID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Enabled  bool   `json:"enabled"`
		RepoName string `json:"repo_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Enabled && req.RepoName == "" {
		http.Error(w, "repo_name is required to enable the awesome page", http.StatusBadRequest)
		return
	}
	if err := store.SetAwesomeConfig(userID, req.RepoName, req.Enabled); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if req.Enabled {
		if err := syncAwesomePage(userID); err != nil {
			json.NewEncoder(w).Encode(map[string]string{"status": "saved", "warning": err.Error()})
			return
		}
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// POST /api/project-gitlab-sync — enable/disable GitLab mirror sync for a project and set its frequency.
func handleProjectGitLabSync(w http.ResponseWriter, r *http.Request, userID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		RepoID    string `json:"repo_id"`
		Enabled   bool   `json:"enabled"`
		Frequency string `json:"frequency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RepoID == "" {
		http.Error(w, "repo_id is required", http.StatusBadRequest)
		return
	}
	if req.Enabled {
		if req.Frequency != "daily" && req.Frequency != "weekly" && req.Frequency != "monthly" {
			http.Error(w, "frequency must be daily, weekly, or monthly", http.StatusBadRequest)
			return
		}
		settings, err := store.GetGitLabSettings(userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if settings.GitLabToken == "" {
			http.Error(w, "configure your GitLab instance in Account Settings first", http.StatusBadRequest)
			return
		}
	}
	ok, err := store.SetProjectGitLabSync(userID, req.RepoID, req.Enabled, req.Frequency)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !ok {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	if req.Enabled {
		for _, t := range store.GetUserGitLabSyncTargets(userID) {
			if t.RepoID == req.RepoID {
				go syncProjectToGitLab(t)
				break
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// POST /api/project-gitlab-sync/sync-now — manually trigger a sync for an already-enabled project.
func handleProjectGitLabSyncNow(w http.ResponseWriter, r *http.Request, userID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	repoID := r.URL.Query().Get("repo_id")
	if repoID == "" {
		http.Error(w, "missing repo_id", http.StatusBadRequest)
		return
	}
	for _, t := range store.GetUserGitLabSyncTargets(userID) {
		if t.RepoID == repoID {
			go syncProjectToGitLab(t)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "queued"})
			return
		}
	}
	http.Error(w, "project not found or gitlab sync not enabled", http.StatusNotFound)
}
```

- [ ] **Step 4: Register routes and start the scheduler in `main.go`**

In `main.go`, add to the route registrations:

```go
	http.HandleFunc("/api/gitlab-settings", requireAuth(handleGitLabSettings))
	http.HandleFunc("/api/gitlab-settings/awesome", requireAuth(handleGitLabAwesome))
	http.HandleFunc("/api/project-gitlab-sync", requireAuth(handleProjectGitLabSync))
	http.HandleFunc("/api/project-gitlab-sync/sync-now", requireAuth(handleProjectGitLabSyncNow))
```

And start the scheduler goroutine (unconditionally — unlike `runDailyDigest`, this has no SMTP-style external-config gate; it's a no-op when no user has `gitlab_sync_enabled`):

```go
	go runGitLabSyncScheduler()
```

placed next to the existing `if smtpEnabled { go runDailyDigest() }` block.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test -run 'TestHandleGitLabSettings|TestHandleProjectGitLabSync|TestHandleGitLabAwesome' -v .`
Expected: PASS. Then `go build -o newreleases .` and `go test ./...` for the full picture.

- [ ] **Step 6: Commit**

```bash
git add handlers.go main.go handlers_gitlab_test.go
git commit -m "feat: add GitLab sync HTTP API and wire up the scheduler"
```

---

### Task 7: Account settings panel UI — GitLab instance + Awesome toggle

**Files:**
- Modify: `ui.go` (`loadAccountPanel`, plus two new JS functions)

**Interfaces:**
- Consumes: `GET/POST /api/gitlab-settings`, `POST /api/gitlab-settings/awesome` (Task 6)
- No new exported Go symbols — this is JS embedded in the `uiJS` string constant in `ui.go`.

- [ ] **Step 1: Fetch GitLab settings alongside account settings**

In `ui.go`, replace the single `fetch('/api/account-settings')` call in `loadAccountPanel` with a `Promise.all`, matching the pattern already used in `loadReleases`:

```js
    Promise.all([
        fetch('/api/account-settings').then(function(r) { return r.json(); }),
        fetch('/api/gitlab-settings').then(function(r) { return r.json(); })
    ])
        .then(function(res) {
            var settings = res[0];
            var gitlabSettings = res[1];
            var smtpOk = !!(currentUser && currentUser.smtp_enabled);
```

(This replaces the existing `.then(function(settings) {` line — keep everything below it unchanged except for referencing `gitlabSettings` where noted next, and updating the final `.catch` to still apply to the combined promise.)

- [ ] **Step 2: Append the GitLab Sync section**

Immediately before the closing `})` of the `.then(function(res) { ... })` callback in `loadAccountPanel` (i.e., after the existing `if (rssToken) { ... }` block), add:

```js
            var glSection = document.createElement('div');
            glSection.style.cssText = 'padding:0.75rem 0;border-top:1px solid #334155';
            var glLabel = document.createElement('div');
            glLabel.style.cssText = 'font-size:0.8rem;color:#64748b;margin-bottom:0.4rem';
            glLabel.textContent = 'GitLab Sync';
            glSection.appendChild(glLabel);

            var glUrlInput = document.createElement('input');
            glUrlInput.type = 'url';
            glUrlInput.id = 'gitlab-url-input';
            glUrlInput.placeholder = 'https://gitlab.example.com';
            glUrlInput.value = gitlabSettings.gitlab_url || '';
            glUrlInput.className = 'webhook-input';
            glUrlInput.style.cssText = 'width:100%;margin-bottom:0.4rem;box-sizing:border-box';
            glSection.appendChild(glUrlInput);

            var glTokenInput = document.createElement('input');
            glTokenInput.type = 'password';
            glTokenInput.id = 'gitlab-token-input';
            glTokenInput.placeholder = gitlabSettings.has_token ? 'Token saved (enter to replace)' : 'API token';
            glTokenInput.className = 'webhook-input';
            glTokenInput.style.cssText = 'width:100%;margin-bottom:0.4rem;box-sizing:border-box';
            glSection.appendChild(glTokenInput);

            var glSaveBtn = document.createElement('button');
            glSaveBtn.className = 'btn btn-primary';
            glSaveBtn.style.cssText = 'font-size:0.78rem';
            glSaveBtn.textContent = 'Save GitLab Settings';
            glSaveBtn.onclick = function() { saveGitLabSettings(); };
            glSection.appendChild(glSaveBtn);

            var awesomeRow = document.createElement('div');
            awesomeRow.style.cssText = 'display:flex;align-items:center;justify-content:space-between;padding:0.6rem 0;margin-top:0.6rem;border-top:1px solid #1a2535';
            var awesomeLbl = document.createElement('label');
            awesomeLbl.htmlFor = 'awesome-toggle';
            awesomeLbl.style.cssText = 'font-size:0.85rem;cursor:pointer';
            awesomeLbl.textContent = 'Generate Awesome page';
            var awesomeChk = document.createElement('input');
            awesomeChk.type = 'checkbox';
            awesomeChk.id = 'awesome-toggle';
            awesomeChk.checked = !!gitlabSettings.awesome_enabled;
            awesomeChk.onchange = function() { onAwesomeToggle(this); };
            awesomeRow.appendChild(awesomeLbl);
            awesomeRow.appendChild(awesomeChk);
            glSection.appendChild(awesomeRow);

            var awesomeNameInput = document.createElement('input');
            awesomeNameInput.type = 'text';
            awesomeNameInput.id = 'awesome-repo-name-input';
            awesomeNameInput.className = 'webhook-input';
            awesomeNameInput.placeholder = 'Repo name (e.g. my-awesome-repos)';
            awesomeNameInput.value = gitlabSettings.awesome_repo_name || '';
            awesomeNameInput.style.cssText = 'width:100%;margin-top:0.3rem;box-sizing:border-box;display:' + (gitlabSettings.awesome_enabled ? 'block' : 'none');
            glSection.appendChild(awesomeNameInput);

            body.appendChild(glSection);
```

- [ ] **Step 3: Add the two JS handler functions**

Add near `toggleEmailDigest` in `ui.go`:

```js
function saveGitLabSettings() {
    var url = document.getElementById('gitlab-url-input').value.trim();
    var token = document.getElementById('gitlab-token-input').value;
    if (!url || !token) { toast('GitLab URL and token are required', 'err'); return; }
    fetch('/api/gitlab-settings', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({gitlab_url: url, gitlab_token: token})
    }).then(function(r) {
        if (!r.ok) return r.text().then(function(t) { throw new Error(t.trim()); });
        toast('GitLab settings saved', 'ok');
        document.getElementById('gitlab-token-input').value = '';
        document.getElementById('gitlab-token-input').placeholder = 'Token saved (enter to replace)';
    }).catch(function(err) { toast('Error: ' + err, 'err'); });
}

function onAwesomeToggle(checkbox) {
    var nameInput = document.getElementById('awesome-repo-name-input');
    var repoName = nameInput.value.trim();
    if (checkbox.checked && !repoName) {
        toast('Enter a repo name for the Awesome page', 'err');
        checkbox.checked = false;
        return;
    }
    nameInput.style.display = checkbox.checked ? 'block' : 'none';
    fetch('/api/gitlab-settings/awesome', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({enabled: checkbox.checked, repo_name: repoName})
    }).then(function(r) {
        if (!r.ok) return r.text().then(function(t) { throw new Error(t.trim()); });
        toast(checkbox.checked ? 'Awesome page enabled' : 'Awesome page disabled', 'ok');
    }).catch(function(err) {
        checkbox.checked = !checkbox.checked;
        nameInput.style.display = checkbox.checked ? 'block' : 'none';
        toast('Error: ' + err, 'err');
    });
}
```

- [ ] **Step 4: Manual verification**

Run: `go build -o newreleases . && ./newreleases`, then in a browser:
1. Register/log in.
2. Open the account panel (⚙ in header).
3. Confirm the "GitLab Sync" section renders with URL/token inputs and the Awesome toggle.
4. Enter a fake GitLab URL (`https://gitlab.example.com`) and a token, click "Save GitLab Settings" — confirm a success toast and that reopening the panel shows "Token saved (enter to replace)".
5. Toggle "Generate Awesome page" on with a repo name — since `https://gitlab.example.com` isn't real, expect a warning toast (or an error toast, depending on the failure path) rather than a crash; confirm the checkbox state and app remain usable afterward.

- [ ] **Step 5: Commit**

```bash
git add ui.go
git commit -m "feat: add GitLab instance and Awesome page settings to the account panel"
```

---

### Task 8: Project settings panel UI — per-project sync enable/frequency/sync-now

**Files:**
- Modify: `ui.go` (gear-button dataset attributes, `openSettingsPanel`, `loadSettingsPanel`, `renderSettingsPanelBody`, the `_settingsRepoID` refresh block in `loadReleases`, plus new render/action functions)

**Interfaces:**
- Consumes: `POST /api/project-gitlab-sync`, `POST /api/project-gitlab-sync/sync-now` (Task 6), and the `gitlab_sync_enabled`/`gitlab_sync_frequency`/`last_gitlab_sync_error` fields now present on `Project` JSON (Task 1).

- [ ] **Step 1: Add GitLab sync data attributes to the project header's settings button**

In `ui.go`, find the settings-button markup (in the function building `buildReleasesHTML`'s per-project header):

```js
                    '<button class="btn btn-ghost proj-ctrl" title="Settings" ' +
                        'data-id="' + esc(pid) + '" data-name="' + esc(pname) + '" ' +
                        'data-push="' + (proj && proj.push_enabled ? '1' : '0') + '" ' +
                        'onclick="openSettingsPanel(this)">&#x2699;</button>' +
```

Replace with:

```js
                    '<button class="btn btn-ghost proj-ctrl" title="Settings" ' +
                        'data-id="' + esc(pid) + '" data-name="' + esc(pname) + '" ' +
                        'data-push="' + (proj && proj.push_enabled ? '1' : '0') + '" ' +
                        'data-gitlab-enabled="' + (proj && proj.gitlab_sync_enabled ? '1' : '0') + '" ' +
                        'data-gitlab-freq="' + esc((proj && proj.gitlab_sync_frequency) || 'daily') + '" ' +
                        'data-gitlab-error="' + esc((proj && proj.last_gitlab_sync_error) || '') + '" ' +
                        'onclick="openSettingsPanel(this)">&#x2699;</button>' +
```

- [ ] **Step 2: Thread the new fields through `openSettingsPanel` / `loadSettingsPanel`**

Replace `openSettingsPanel`:

```js
function openSettingsPanel(btn) {
    var id = btn.dataset.id;
    var name = btn.dataset.name;
    var pushEnabled = btn.dataset.push === '1';
    var glEnabled = btn.dataset.gitlabEnabled === '1';
    var glFreq = btn.dataset.gitlabFreq || 'daily';
    var glError = btn.dataset.gitlabError || '';
    _settingsRepoID = id;
    document.getElementById('settings-panel-title').textContent = name;
    closeAddPanel();
    closeAccountPanel();
    document.getElementById('settings-panel').classList.add('open');
    loadSettingsPanel(id, pushEnabled, glEnabled, glFreq, glError);
}
```

Replace `loadSettingsPanel`:

```js
function loadSettingsPanel(id, pushEnabled, glEnabled, glFreq, glError) {
    var body = document.getElementById('settings-panel-body');
    body.innerHTML = '<div style="font-size:0.85rem;color:#64748b">Loading...</div>';
    fetch('/api/webhooks?repo_id=' + encodeURIComponent(id))
        .then(function(r) { return r.json(); })
        .then(function(hooks) {
            body.innerHTML = renderSettingsPanelBody(id, pushEnabled, hooks || [], glEnabled, glFreq, glError);
        })
        .catch(function() {
            body.innerHTML = '<div style="color:#f87171;font-size:0.85rem">Failed to load settings.</div>';
        });
}
```

- [ ] **Step 3: Extend `renderSettingsPanelBody` and add the GitLab Sync section renderer**

Replace `renderSettingsPanelBody`:

```js
function renderSettingsPanelBody(id, pushEnabled, hooks, glEnabled, glFreq, glError) {
    return '<div class="settings-section">' +
            '<div class="settings-section-title">Push Notifications</div>' +
            '<div class="toggle-row">' +
                '<span>Notify on new releases</span>' +
                '<label class="toggle">' +
                    '<input type="checkbox" ' + (pushEnabled ? 'checked' : '') + ' onchange="doTogglePush(this,\'' + esc(id) + '\')">' +
                    '<span class="toggle-slider"></span>' +
                '</label>' +
            '</div>' +
        '</div>' +
        '<div class="settings-section">' +
            '<div class="settings-section-title">GitLab Sync</div>' +
            renderGitLabSyncSection(id, glEnabled, glFreq, glError) +
        '</div>' +
        '<div class="settings-section">' +
            '<div class="settings-section-title">Webhooks</div>' +
            '<div id="whpanel-' + esc(id) + '">' + renderWebhookPanel(id, hooks) + '</div>' +
        '</div>';
}

function renderGitLabSyncSection(id, enabled, freq, err) {
    var freqOptions = ['daily', 'weekly', 'monthly'].map(function(f) {
        return '<option value="' + f + '"' + (f === freq ? ' selected' : '') + '>' +
            f.charAt(0).toUpperCase() + f.slice(1) + '</option>';
    }).join('');
    var errHtml = err ? '<div style="color:#f87171;font-size:0.75rem;margin-top:0.3rem">' + esc(err) + '</div>' : '';
    if (!enabled) {
        return '<div class="webhook-add" style="margin-top:0">' +
                '<select id="gl-freq-' + esc(id) + '" class="webhook-input">' + freqOptions + '</select>' +
                '<button class="btn btn-primary" style="font-size:0.78rem" onclick="doEnableGitLabSync(\'' + esc(id) + '\')">Enable Sync</button>' +
            '</div>' + errHtml;
    }
    return '<div class="toggle-row">' +
            '<span>Syncing ' + esc(freq) + '</span>' +
            '<button class="btn btn-ghost" style="font-size:0.75rem" onclick="doSyncGitLabNow(\'' + esc(id) + '\')">Sync now</button>' +
        '</div>' +
        '<button class="btn btn-outline" style="font-size:0.75rem;margin-top:0.4rem" onclick="doDisableGitLabSync(\'' + esc(id) + '\')">Disable</button>' +
        errHtml;
}
```

- [ ] **Step 4: Add the three action functions**

Add near `doTogglePush`:

```js
function doEnableGitLabSync(id) {
    var freq = document.getElementById('gl-freq-' + id).value;
    fetch('/api/project-gitlab-sync', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({repo_id: id, enabled: true, frequency: freq})
    }).then(function(r) {
        if (!r.ok) return r.text().then(function(t) { throw new Error(t.trim()); });
        toast('GitLab sync enabled', 'ok');
        loadReleases();
    }).catch(function(err) { toast('Error: ' + err, 'err'); });
}

function doDisableGitLabSync(id) {
    fetch('/api/project-gitlab-sync', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({repo_id: id, enabled: false})
    }).then(function(r) {
        if (!r.ok) return r.text().then(function(t) { throw new Error(t.trim()); });
        toast('GitLab sync disabled', 'ok');
        loadReleases();
    }).catch(function(err) { toast('Error: ' + err, 'err'); });
}

function doSyncGitLabNow(id) {
    fetch('/api/project-gitlab-sync/sync-now?repo_id=' + encodeURIComponent(id), {method: 'POST'})
        .then(function(r) {
            if (!r.ok) return r.text().then(function(t) { throw new Error(t.trim()); });
            toast('Sync queued', 'inf');
            setTimeout(function() { loadReleases(); }, 2000);
        }).catch(function(err) { toast('Error: ' + err, 'err'); });
}
```

- [ ] **Step 5: Update the `_settingsRepoID` refresh block in `loadReleases`**

Find:

```js
        if (_settingsRepoID) {
            var fresh = allProjects.filter(function(p) { return p.id === _settingsRepoID; })[0];
            if (fresh) {
                loadSettingsPanel(_settingsRepoID, fresh.push_enabled);
            } else {
                closeSettingsPanel();
                _settingsRepoID = null;
            }
        }
```

Replace with:

```js
        if (_settingsRepoID) {
            var fresh = allProjects.filter(function(p) { return p.id === _settingsRepoID; })[0];
            if (fresh) {
                loadSettingsPanel(_settingsRepoID, fresh.push_enabled,
                    fresh.gitlab_sync_enabled, fresh.gitlab_sync_frequency || 'daily', fresh.last_gitlab_sync_error || '');
            } else {
                closeSettingsPanel();
                _settingsRepoID = null;
            }
        }
```

- [ ] **Step 6: Manual verification**

Run: `go build -o newreleases . && ./newreleases`, then in a browser (continuing from Task 7's GitLab settings already saved):
1. Add a GitHub project.
2. Open its settings (⚙ gear icon) — confirm the "GitLab Sync" section shows a frequency dropdown and "Enable Sync" button.
3. Click "Enable Sync" — confirm a toast, and that the panel now shows "Syncing daily", a "Sync now" button, and a "Disable" button (the sync itself will fail against the fake GitLab URL from Task 7 — confirm the error surfaces via `last_gitlab_sync_error` after the background goroutine completes and you reopen the panel, rather than crashing the page).
4. Add a project with `platform: npm` (via the Add Project panel) and confirm its settings panel does NOT offer a working "Enable Sync" path that succeeds — the server-side 400 should surface as an error toast if attempted. (The UI doesn't hide the section per-platform in this plan — note this as a known follow-up, not a blocker.)
5. Click "Disable" — confirm it reverts to the frequency-dropdown/"Enable Sync" state.

- [ ] **Step 7: Commit**

```bash
git add ui.go
git commit -m "feat: add per-project GitLab sync controls to the settings panel"
```

---

### Task 9: Dockerfile git dependency + full verification

**Files:**
- Modify: `Dockerfile`

**Interfaces:** None — deployment-only change.

- [ ] **Step 1: Add `git` to the final image stage**

In `Dockerfile`, find:

```dockerfile
# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates
```

Replace with:

```dockerfile
# Final stage
FROM alpine:latest

# ca-certificates for HTTPS; git for GitLab mirror sync (ephemeral clone/push)
RUN apk --no-cache add ca-certificates git
```

- [ ] **Step 2: Build and run the full test suite**

Run: `go build -o newreleases .`
Expected: builds cleanly.

Run: `go vet ./...`
Expected: no issues.

Run: `go test -v -race ./...`
Expected: all tests pass, including every test added in Tasks 1–6.

- [ ] **Step 3: Build the Docker image**

Run: `docker build -t newreleases:gitlab-sync-test .`
Expected: succeeds, including the `RUN go test -v ./...` step inside the builder stage (git is present there too, from the pre-existing `RUN apk add --no-cache git ca-certificates` in the builder stage).

- [ ] **Step 4: Commit**

```bash
git add Dockerfile
git commit -m "chore: install git in the final image for GitLab mirror sync"
```

---

## Post-plan follow-ups (explicitly out of scope, noted for TODO.md)

- UI does not currently hide "Enable Sync" for non-git platforms (npm/pypi/docker) — it relies on the server's 400 response. A nicer UX would gate the button client-side too.
- No admin/user-facing way to delete a previously-created GitLab mirror or Awesome project from within newreleases — disabling sync only stops future pushes, matching the spec's explicit non-goal.
- `gitlab_token` is stored in plaintext (matches existing webhook-secret pattern) — flagged in the design spec as a deferred hardening item, not resolved here.
