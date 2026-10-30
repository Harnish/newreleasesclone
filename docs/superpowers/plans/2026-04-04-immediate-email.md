# Immediate Email on New Release — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Send a plain-text email immediately when a new release is detected for a tracked project, opt-in per project per user.

**Architecture:** Adds an `email_immediate` column to `user_repos` (schema v8), a new store method to find opted-in users, and a goroutine dispatch in `RefreshProject` parallel to the existing push/webhook goroutines. Email composition follows the same pattern as the existing daily digest in `email.go`. The UI adds a toggle in the per-project settings panel, built with DOM API (same pattern as the account settings panel).

**Tech Stack:** Go, SQLite (`modernc.org/sqlite`), existing `SMTPConfig`/`SendMail` in `email.go`, no new dependencies.

---

### Task 1: Schema v8, Model Field, and Store Methods

**Files:**
- Modify: `store.go`
- Modify: `models.go`
- Modify: `main_test.go`

- [ ] **Step 1: Write failing tests**

Add to `main_test.go` after `TestStoreSetProjectPushEnabled`:

```go
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

	users := s.getImmediateEmailUsersForRepo(repoID)
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].ID != userA {
		t.Errorf("expected user A, got %s", users[0].ID)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test -v -run "TestStoreSetProjectEmailImmediate|TestGetImmediateEmailUsers" ./...
```

Expected: FAIL — `s.SetProjectEmailImmediate` undefined, `EmailImmediate` field missing.

- [ ] **Step 3: Add `EmailImmediate bool` to the `Project` struct in `models.go`**

In `models.go`, extend `Project`:

```go
type Project struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Platform       string    `json:"platform"`
	RepoURL        string    `json:"repo_url"`
	LastRefresh    time.Time `json:"last_refresh"`
	RefreshCount   int       `json:"refresh_count"`
	PushEnabled    bool      `json:"push_enabled"`
	EmailImmediate bool      `json:"email_immediate"`
}
```

- [ ] **Step 4: Add schema v8 migration to `store.go`**

In `migrate()` in `store.go`, after the `version < 7` block (around line 212), add:

```go
if version < 8 {
	if _, err := db.Exec(`ALTER TABLE user_repos ADD COLUMN email_immediate INTEGER NOT NULL DEFAULT 0`); err != nil {
		return fmt.Errorf("failed to migrate to v8 (email_immediate): %w", err)
	}
	if _, err := db.Exec("PRAGMA user_version = 8"); err != nil {
		return fmt.Errorf("failed to set schema version: %w", err)
	}
}
```

- [ ] **Step 5: Add `SetProjectEmailImmediate` to `store.go`**

Add after `SetProjectPushEnabled` (around line 354):

```go
func (s *Store) SetProjectEmailImmediate(userID, repoID string, enabled bool) (bool, error) {
	val := 0
	if enabled {
		val = 1
	}
	res, err := s.db.Exec(
		"UPDATE user_repos SET email_immediate = ? WHERE user_id = ? AND repo_id = ?",
		val, userID, repoID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}
```

- [ ] **Step 6: Update `GetUserRepos` to scan `email_immediate`**

In `GetUserRepos` (around line 713), update the SELECT and scan:

```go
func (s *Store) GetUserRepos(userID string) []Project {
	rows, err := s.db.Query(`
		SELECT r.id, r.name, r.platform, r.repo_url, r.last_refresh, r.refresh_count, ur.push_enabled, ur.email_immediate
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
		var lastRefresh string
		var pushEnabled, emailImmediate int
		if err := rows.Scan(&p.ID, &p.Name, &p.Platform, &p.RepoURL, &lastRefresh, &p.RefreshCount, &pushEnabled, &emailImmediate); err != nil {
			log.Printf("⚠ Failed to scan project: %v", err)
			continue
		}
		if lastRefresh != "" {
			p.LastRefresh, _ = time.Parse(time.RFC3339, lastRefresh)
		}
		p.PushEnabled = pushEnabled != 0
		p.EmailImmediate = emailImmediate != 0
		projects = append(projects, p)
	}
	return projects
}
```

- [ ] **Step 7: Add `getImmediateEmailUsersForRepo` to `store.go`**

Add as an unexported method (anywhere after `SetProjectEmailImmediate`):

```go
func (s *Store) getImmediateEmailUsersForRepo(repoID string) []User {
	rows, err := s.db.Query(`
		SELECT u.id, u.username, u.email, u.email_verified
		FROM users u
		INNER JOIN user_repos ur ON ur.user_id = u.id
		WHERE ur.repo_id = ?
		  AND ur.email_immediate = 1
		  AND u.email_verified = 1
		  AND u.email != ''`, repoID)
	if err != nil {
		log.Printf("⚠ getImmediateEmailUsersForRepo query failed: %v", err)
		return nil
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		var verified int
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &verified); err != nil {
			log.Printf("⚠ failed to scan user: %v", err)
			continue
		}
		u.EmailVerified = verified != 0
		users = append(users, u)
	}
	return users
}
```

- [ ] **Step 8: Add `sendImmediateEmails` to `store.go`**

Add after `getImmediateEmailUsersForRepo`:

```go
func (s *Store) sendImmediateEmails(repoID string, project Project, newReleases []Release) {
	for _, release := range newReleases {
		users := s.getImmediateEmailUsersForRepo(repoID)
		if users == nil {
			log.Printf("⚠ sendImmediateEmails: skipping release %s (query failed)", release.Version)
			continue
		}
		for _, u := range users {
			if err := smtpCfg.SendReleaseEmail(u.Email, project, release); err != nil {
				log.Printf("⚠ sendImmediateEmails: failed to send to %s: %v", u.Email, err)
			}
		}
	}
}
```

- [ ] **Step 9: Dispatch `sendImmediateEmails` from `RefreshProject`**

In `RefreshProject` (around line 983), update the new-releases block:

```go
if len(newReleases) > 0 {
	go s.sendPushNotifications(repoID, project.Name, newReleases)
	go s.sendWebhooks(repoID, project, newReleases)
	if smtpEnabled {
		go s.sendImmediateEmails(repoID, project, newReleases)
	}
}
```

- [ ] **Step 10: Run tests to confirm they pass**

```bash
go test -v -run "TestStoreSetProjectEmailImmediate|TestGetImmediateEmailUsers" ./...
```

Expected: PASS

- [ ] **Step 11: Run all tests to check for regressions**

```bash
go test -v ./...
```

Expected: All tests pass.

- [ ] **Step 12: Commit**

```bash
git add store.go models.go main_test.go
git commit -m "feat: schema v8, email_immediate model/store/dispatch"
```

---

### Task 2: Email Builder (`email.go`)

**Files:**
- Modify: `email.go`
- Modify: `main_test.go`

- [ ] **Step 1: Write failing tests**

Add to `main_test.go` after `TestBuildDailySummaryBody`:

```go
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
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test -v -run "TestBuildReleaseEmailBody|TestSendReleaseEmailSubject" ./...
```

Expected: FAIL — `buildReleaseEmailBody` undefined.

- [ ] **Step 3: Add `buildReleaseEmailBody` and `SendReleaseEmail` to `email.go`**

Append to `email.go`:

```go
// buildReleaseEmailBody formats the plain-text body for a single release notification.
func buildReleaseEmailBody(project Project, release Release) string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "%s %s has been released.\n\n%s", project.Name, release.Version, release.URL)
	if release.ReleaseNotes != "" {
		fmt.Fprintf(&buf, "\n\n%s", release.ReleaseNotes)
	}
	return buf.String()
}

// SendReleaseEmail sends an immediate release notification to the given address.
func (c SMTPConfig) SendReleaseEmail(to string, project Project, release Release) error {
	subject := fmt.Sprintf("%s %s released", project.Name, release.Version)
	return c.SendMail(to, subject, buildReleaseEmailBody(project, release))
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test -v -run "TestBuildReleaseEmailBody|TestSendReleaseEmailSubject" ./...
```

Expected: PASS

- [ ] **Step 5: Run all tests to check for regressions**

```bash
go test -v ./...
```

Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add email.go main_test.go
git commit -m "feat: add buildReleaseEmailBody and SendReleaseEmail"
```

---

### Task 3: Extend `POST /api/project-settings` Handler

**Files:**
- Modify: `handlers.go`
- Modify: `main_test.go`

- [ ] **Step 1: Write failing tests**

Add to `main_test.go` after `TestHandleProjectSettings`:

```go
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
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test -v -run "TestHandleProjectSettingsEmailImmediate" ./...
```

Expected: FAIL — handler ignores `email_immediate`.

- [ ] **Step 3: Update `handleProjectSettings` in `handlers.go`**

Replace the current request struct and handler body (around line 396):

```go
// POST /api/project-settings — update per-project settings (push_enabled, email_immediate).
func handleProjectSettings(w http.ResponseWriter, r *http.Request, userID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		RepoID         string `json:"repo_id"`
		PushEnabled    bool   `json:"push_enabled"`
		EmailImmediate bool   `json:"email_immediate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.RepoID == "" {
		http.Error(w, "repo_id is required", http.StatusBadRequest)
		return
	}
	ok, err := store.SetProjectPushEnabled(userID, req.RepoID, req.PushEnabled)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	if smtpEnabled {
		if _, err := store.SetProjectEmailImmediate(userID, req.RepoID, req.EmailImmediate); err != nil {
			log.Printf("⚠ SetProjectEmailImmediate failed: %v", err)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test -v -run "TestHandleProjectSettingsEmailImmediate|TestHandleProjectSettings" ./...
```

Expected: PASS (both old and new tests).

- [ ] **Step 5: Run all tests to check for regressions**

```bash
go test -v ./...
```

Expected: All tests pass.

- [ ] **Step 6: Commit**

```bash
git add handlers.go main_test.go
git commit -m "feat: extend project-settings handler to accept email_immediate"
```

---

### Task 4: UI — Email Toggle in Project Settings Panel

**Files:**
- Modify: `ui.go`

No new tests — this is JavaScript embedded in a Go string constant. Verified manually via browser.

The changes are all in the JavaScript embedded in `ui.go`. Each sub-step targets a specific function.

**Overview of changes:**

| Location | Change |
|---|---|
| `buildReleasesHTML` | Add `data-email-immediate` attribute to the settings button |
| `openSettingsPanel` | Read `data-email-immediate` and pass to `loadSettingsPanel` |
| `loadReleases` | Pass `fresh.email_immediate` to `loadSettingsPanel` |
| `renderSettingsPanelBody` | Add `id` attribute to push checkbox (`push-cb-${id}`) |
| `loadSettingsPanel` | Add `emailImmediate` parameter; call `addEmailToggleToPanel` after body renders |
| `doTogglePush` | Cross-read `ei-cb-${id}` and include `email_immediate` in POST body |
| New: `addEmailToggleToPanel` | Create email toggle section via DOM API and append to panel body |
| New: `doToggleEmailImmediate` | Cross-read `push-cb-${id}` and POST both settings |

- [ ] **Step 1: Add `data-email-immediate` attribute to the settings button in `buildReleasesHTML`**

In `buildReleasesHTML` (around line 866), update the settings button `data-` attributes.

Find:
```
'data-push="' + (proj && proj.push_enabled ? '1' : '0') + '" ' +
'onclick="openSettingsPanel(this)">&#x2699;</button>' +
```

Replace with:
```
'data-push="' + (proj && proj.push_enabled ? '1' : '0') + '" ' +
'data-email-immediate="' + (proj && proj.email_immediate ? '1' : '0') + '" ' +
'onclick="openSettingsPanel(this)">&#x2699;</button>' +
```

- [ ] **Step 2: Update `openSettingsPanel` to read and forward `email_immediate`**

Find the function (around line 917):
```js
function openSettingsPanel(btn) {
    var id = btn.dataset.id;
    var name = btn.dataset.name;
    var pushEnabled = btn.dataset.push === '1';
    _settingsRepoID = id;
    document.getElementById('settings-panel-title').textContent = name;
    closeAddPanel();
    closeAccountPanel();
    document.getElementById('settings-panel').classList.add('open');
    loadSettingsPanel(id, pushEnabled);
}
```

Replace with:
```js
function openSettingsPanel(btn) {
    var id = btn.dataset.id;
    var name = btn.dataset.name;
    var pushEnabled = btn.dataset.push === '1';
    var emailImmediate = btn.dataset.emailImmediate === '1';
    _settingsRepoID = id;
    document.getElementById('settings-panel-title').textContent = name;
    closeAddPanel();
    closeAccountPanel();
    document.getElementById('settings-panel').classList.add('open');
    loadSettingsPanel(id, pushEnabled, emailImmediate);
}
```

- [ ] **Step 3: Update `loadReleases` to forward `email_immediate` when refreshing the open panel**

In `loadReleases` (around line 901), update the call:

Find:
```js
loadSettingsPanel(_settingsRepoID, fresh.push_enabled);
```

Replace with:
```js
loadSettingsPanel(_settingsRepoID, fresh.push_enabled, !!fresh.email_immediate);
```

- [ ] **Step 4: Add an `id` to the push checkbox in `renderSettingsPanelBody`**

In `renderSettingsPanelBody` (around line 953), update the checkbox input line.

Find:
```
'<input type="checkbox" ' + (pushEnabled ? 'checked' : '') + ' onchange="doTogglePush(this,\'' + esc(id) + '\')">' +
```

Replace with:
```
'<input type="checkbox" id="push-cb-' + esc(id) + '" ' + (pushEnabled ? 'checked' : '') + ' onchange="doTogglePush(this,\'' + esc(id) + '\')">' +
```

- [ ] **Step 5: Update `loadSettingsPanel` to accept `emailImmediate` and call `addEmailToggleToPanel`**

Find:
```js
function loadSettingsPanel(id, pushEnabled) {
    var body = document.getElementById('settings-panel-body');
    body.innerHTML = '<div style="font-size:0.85rem;color:#64748b">Loading...</div>';
    fetch('/api/webhooks?repo_id=' + encodeURIComponent(id))
        .then(function(r) { return r.json(); })
        .then(function(hooks) {
            body.innerHTML = renderSettingsPanelBody(id, pushEnabled, hooks || []);
        })
        .catch(function() {
            body.innerHTML = '<div style="color:#f87171;font-size:0.85rem">Failed to load settings.</div>';
        });
}
```

Replace with:
```js
function loadSettingsPanel(id, pushEnabled, emailImmediate) {
    var body = document.getElementById('settings-panel-body');
    body.innerHTML = '<div style="font-size:0.85rem;color:#64748b">Loading...</div>';
    fetch('/api/webhooks?repo_id=' + encodeURIComponent(id))
        .then(function(r) { return r.json(); })
        .then(function(hooks) {
            body.innerHTML = renderSettingsPanelBody(id, pushEnabled, hooks || []);
            addEmailToggleToPanel(id, !!emailImmediate);
        })
        .catch(function() {
            body.innerHTML = '<div style="color:#f87171;font-size:0.85rem">Failed to load settings.</div>';
        });
}
```

- [ ] **Step 6: Add `addEmailToggleToPanel` function**

Add after `renderSettingsPanelBody`, before `doTogglePush`:

```js
function addEmailToggleToPanel(id, emailImmediate) {
    var panelBody = document.getElementById('settings-panel-body');

    var section = document.createElement('div');
    section.className = 'settings-section';

    var sectionTitle = document.createElement('div');
    sectionTitle.className = 'settings-section-title';
    sectionTitle.textContent = 'Email Notifications';
    section.appendChild(sectionTitle);

    var row = document.createElement('div');
    row.className = 'toggle-row';

    var labelSpan = document.createElement('span');
    labelSpan.textContent = 'Notify on new releases';
    row.appendChild(labelSpan);

    var label = document.createElement('label');
    label.className = 'toggle';

    var cb = document.createElement('input');
    cb.type = 'checkbox';
    cb.id = 'ei-cb-' + id;
    cb.checked = emailImmediate;

    var smtpOk = currentUser && currentUser.smtp_enabled;
    var verified = currentUser && currentUser.email_verified;

    if (!smtpOk) {
        cb.disabled = true;
        cb.title = 'Email not configured';
    } else if (!verified) {
        cb.disabled = true;
        cb.title = 'Verify your email to enable';
    } else {
        cb.addEventListener('change', function() {
            doToggleEmailImmediate(id, cb.checked);
        });
    }

    var slider = document.createElement('span');
    slider.className = 'toggle-slider';
    label.appendChild(cb);
    label.appendChild(slider);
    row.appendChild(label);
    section.appendChild(row);

    panelBody.appendChild(section);
}
```

- [ ] **Step 7: Add `doToggleEmailImmediate` function**

Add after `doTogglePush`:

```js
function doToggleEmailImmediate(id, checked) {
    var pushCb = document.getElementById('push-cb-' + id);
    var pushEnabled = pushCb ? pushCb.checked : false;
    fetch('/api/project-settings', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({repo_id: id, push_enabled: pushEnabled, email_immediate: checked})
    }).then(function(r) {
        if (r.ok) {
            toast('Notification setting updated.', 'ok');
        } else {
            var cb = document.getElementById('ei-cb-' + id);
            if (cb) cb.checked = !checked;
            toast('Failed to update setting.', 'err');
        }
    }).catch(function() {
        var cb = document.getElementById('ei-cb-' + id);
        if (cb) cb.checked = !checked;
        toast('Error updating setting.', 'err');
    });
}
```

- [ ] **Step 8: Update `doTogglePush` to cross-read email checkbox and include `email_immediate` in POST**

Find:
```js
function doTogglePush(checkbox, id) {
    fetch('/api/project-settings', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({repo_id: id, push_enabled: checkbox.checked})
    }).then(function(r) {
        if (r.ok) {
            toast('Notification setting updated.', 'ok');
        } else {
            checkbox.checked = !checkbox.checked;
            toast('Failed to update setting.', 'err');
        }
    }).catch(function() {
        checkbox.checked = !checkbox.checked;
        toast('Error updating setting.', 'err');
    });
}
```

Replace with:
```js
function doTogglePush(checkbox, id) {
    var eiCb = document.getElementById('ei-cb-' + id);
    var emailImmediate = eiCb ? eiCb.checked : false;
    fetch('/api/project-settings', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({repo_id: id, push_enabled: checkbox.checked, email_immediate: emailImmediate})
    }).then(function(r) {
        if (r.ok) {
            toast('Notification setting updated.', 'ok');
        } else {
            checkbox.checked = !checkbox.checked;
            toast('Failed to update setting.', 'err');
        }
    }).catch(function() {
        checkbox.checked = !checkbox.checked;
        toast('Error updating setting.', 'err');
    });
}
```

- [ ] **Step 9: Build and verify compilation**

```bash
go build -o newreleases .
```

Expected: no errors.

- [ ] **Step 10: Run all tests**

```bash
go test -v ./...
```

Expected: All tests pass.

- [ ] **Step 11: Manual smoke test**

Start the server: `./newreleases`

1. Open the app, open a project's settings panel (⚙).
2. Verify "Email Notifications" section appears below "Webhooks".
3. If SMTP is not configured, the toggle should be disabled with tooltip "Email not configured".
4. If SMTP is configured but email unverified, disabled with "Verify your email to enable".
5. If SMTP configured and email verified, toggle works and shows toast.

- [ ] **Step 12: Commit**

```bash
git add ui.go
git commit -m "feat: add immediate email toggle to project settings panel"
```

---

## Self-Review

**Spec coverage:**

| Spec requirement | Task |
|---|---|
| Schema v8, `email_immediate` column on `user_repos` | Task 1 |
| `EmailImmediate bool` on `Project` struct | Task 1 |
| `SetProjectEmailImmediate` store method | Task 1 |
| `getImmediateEmailUsersForRepo` — verified + opted-in users only | Task 1 |
| `sendImmediateEmails` — per-release, per-user, logs errors | Task 1 |
| `GetUserRepos` scans `email_immediate` | Task 1 |
| `RefreshProject` dispatches goroutine when `smtpEnabled` | Task 1 |
| `buildReleaseEmailBody` — with/without release notes | Task 2 |
| `SendReleaseEmail` — subject format | Task 2 |
| Handler accepts `email_immediate`, calls `SetProjectEmailImmediate` | Task 3 |
| Handler silently ignores `email_immediate` when SMTP disabled | Task 3 |
| UI toggle disabled when SMTP not configured | Task 4 |
| UI toggle disabled when email unverified | Task 4 |
| Settings button `data-email-immediate` attribute | Task 4 |
| `doTogglePush` includes current `email_immediate` in POST | Task 4 |
| `doToggleEmailImmediate` includes current `push_enabled` in POST | Task 4 |

All spec requirements are covered.

**Placeholder scan:** No TBD, TODO, or vague requirements found.

**Type consistency:** `SetProjectEmailImmediate(userID, repoID string, enabled bool) (bool, error)` defined in Task 1 step 5 and called in Task 3 step 3. `sendImmediateEmails(repoID string, project Project, newReleases []Release)` defined in Task 1 step 8 and dispatched in step 9. `SendReleaseEmail` defined in Task 2 step 3 and called in Task 1 step 8. Consistent throughout.
