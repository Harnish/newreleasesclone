# Daily Email Summary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Send a daily opt-in email at 7:00 UTC listing each user's previous day's releases, with an account settings panel to toggle the preference.

**Architecture:** A goroutine started at app boot sleeps until the next 7:00 UTC, fires the digest, then ticks every 24 hours. User preference (`email_digest`) is stored on the `users` table. An account settings slide-over panel in the header lets users toggle it.

**Tech Stack:** Go stdlib (`time.Timer`, `time.NewTicker`), existing SQLite store, existing `SMTPConfig.SendMail`, vanilla JS with DOM API for panel rendering.

---

## Files

| File | Change |
|---|---|
| `store.go` | v7 migration + `SetEmailDigest`, `GetUserDigestEnabled`, `GetDigestUsers`, `GetReleasesPublishedOn` |
| `email.go` | `buildDailySummaryBody` (testable), `SendDailySummary` |
| `scheduler.go` | New: `nextSevenAMUTC`, `runDailyDigest`, `sendDailyDigestToAll` |
| `handlers.go` | `handleAccountSettings` |
| `main.go` | Register `/api/account-settings`, start digest goroutine |
| `ui.go` | Account panel CSS, HTML, JS |
| `main_test.go` | Tests for all new store methods, handler, body builder, scheduler util |

---

### Task 1: Schema v7 migration and store methods

**Files:**
- Modify: `store.go`
- Modify: `main_test.go`

**Context:** Schema is at v6 (see `migrate()` around line 77). Existing pattern: `if version < N { db.Exec(...); db.Exec("PRAGMA user_version = N") }`. `GetUserByID` at line 523 shows the user scan pattern (id, username, email, email_verified as int). `SetProjectPushEnabled` at line 331 shows the bool-to-int pattern for SQLite.

- [ ] **Step 1: Write failing tests**

Add to `main_test.go`:

```go
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

func TestGetDigestUsers(t *testing.T) {
	s := newTestStore(t)

	// u1: digest + verified + email => should appear
	u1, _ := s.CreateUser("digest1", "password123")
	s.SetUserEmail(u1.ID, "digest1@example.com")
	s.db.Exec("UPDATE users SET email_verified = 1 WHERE id = ?", u1.ID)
	s.SetEmailDigest(u1.ID, true)

	// u2: digest enabled but not verified => should NOT appear
	u2, _ := s.CreateUser("digest2", "password123")
	s.SetUserEmail(u2.ID, "digest2@example.com")
	s.SetEmailDigest(u2.ID, true)

	// u3: verified but digest off => should NOT appear
	u3, _ := s.CreateUser("digest3", "password123")
	s.SetUserEmail(u3.ID, "digest3@example.com")
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
	u2, _ := s.CreateUser("other", "password123")
	releases2 := s.GetReleasesPublishedOn(u2.ID, yesterday)
	if len(releases2) != 0 {
		t.Errorf("expected 0 releases for other user, got %d", len(releases2))
	}
}
```

- [ ] **Step 2: Run tests — expect failures**

```
go test -v -run "TestSetGetEmailDigest|TestGetDigestUsers|TestGetReleasesPublishedOn" ./...
```

Expected: `FAIL` — methods do not exist yet.

- [ ] **Step 3: Add v7 migration to `store.go`**

In `migrate()`, after the `if version < 6 { ... }` block (around line 203), add:

```go
	if version < 7 {
		if _, err := db.Exec(`ALTER TABLE users ADD COLUMN email_digest INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("failed to migrate to v7 (email_digest): %w", err)
		}
		if _, err := db.Exec("PRAGMA user_version = 7"); err != nil {
			return fmt.Errorf("failed to set schema version: %w", err)
		}
	}
```

- [ ] **Step 4: Add the four store methods to `store.go`**

Add after the `GetUserDigestEnabled` — place these after the `GetUserByID` function (after line ~534):

```go
func (s *Store) SetEmailDigest(userID string, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	_, err := s.db.Exec("UPDATE users SET email_digest = ? WHERE id = ?", val, userID)
	return err
}

func (s *Store) GetUserDigestEnabled(userID string) (bool, error) {
	var v int
	err := s.db.QueryRow("SELECT email_digest FROM users WHERE id = ?", userID).Scan(&v)
	return v == 1, err
}

func (s *Store) GetDigestUsers() []User {
	rows, err := s.db.Query(`
		SELECT id, username, email, email_verified
		FROM users
		WHERE email_digest = 1
		  AND email_verified = 1
		  AND email != ''`)
	if err != nil {
		log.Printf("⚠ GetDigestUsers failed: %v", err)
		return nil
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var u User
		var emailVerified int
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &emailVerified); err != nil {
			continue
		}
		u.EmailVerified = emailVerified == 1
		users = append(users, u)
	}
	return users
}

func (s *Store) GetReleasesPublishedOn(userID string, date time.Time) []Release {
	from := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	to := time.Date(date.Year(), date.Month(), date.Day()+1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	rows, err := s.db.Query(`
		SELECT r.id, r.repo_id, r.name, r.version, r.platform, r.url,
		       r.published_at, r.description, r.release_notes
		FROM releases r
		INNER JOIN user_repos ur ON ur.repo_id = r.repo_id
		WHERE ur.user_id = ?
		  AND r.published_at >= ?
		  AND r.published_at < ?
		ORDER BY r.name ASC, r.published_at DESC`, userID, from, to)
	if err != nil {
		log.Printf("⚠ GetReleasesPublishedOn failed for user %s: %v", userID, err)
		return []Release{}
	}
	defer rows.Close()
	return scanReleases(rows)
}
```

- [ ] **Step 5: Run tests — expect pass**

```
go test -v -run "TestSetGetEmailDigest|TestGetDigestUsers|TestGetReleasesPublishedOn" ./...
```

Expected: all `PASS`.

- [ ] **Step 6: Run full suite**

```
go test ./...
```

Expected: all pass, no regressions.

- [ ] **Step 7: Commit**

```bash
git add store.go main_test.go
git commit -m "feat: schema v7 email_digest column and store methods"
```

---

### Task 2: Email body builder and SendDailySummary

**Files:**
- Modify: `email.go`
- Modify: `main_test.go`

**Context:** `email.go` has `SMTPConfig.SendMail(to, subject, body string) error`. Current imports: `"fmt"`, `"net/smtp"`, `"os"`, `"strconv"`. Add `"strings"`. The body builder is a standalone function so it can be tested without a live SMTP server.

- [ ] **Step 1: Write failing test**

Add to `main_test.go`:

```go
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
```

- [ ] **Step 2: Run test — expect failure**

```
go test -v -run TestBuildDailySummaryBody ./...
```

Expected: `FAIL` — `buildDailySummaryBody` not defined.

- [ ] **Step 3: Add `buildDailySummaryBody` and `SendDailySummary` to `email.go`**

Add `"strings"` to the import block in `email.go`. Then append to the file:

```go
// buildDailySummaryBody formats the plain-text body for the daily digest email.
// Each release is rendered as: "<Name> <Version> — <URL>"
func buildDailySummaryBody(releases []Release) string {
	var buf strings.Builder
	buf.WriteString("Here are the releases from yesterday:\n\n")
	for _, r := range releases {
		fmt.Fprintf(&buf, "%s %s \u2014 %s\n", r.Name, r.Version, r.URL)
	}
	return buf.String()
}

// SendDailySummary sends the daily release digest to the given address.
func (c SMTPConfig) SendDailySummary(to string, releases []Release) error {
	return c.SendMail(to, "Your daily release summary", buildDailySummaryBody(releases))
}
```

Note: `\u2014` is the em-dash character `—` to avoid any encoding issues in the source file.

- [ ] **Step 4: Run test — expect pass**

```
go test -v -run TestBuildDailySummaryBody ./...
```

Expected: `PASS`.

- [ ] **Step 5: Run full suite**

```
go test ./...
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add email.go main_test.go
git commit -m "feat: add SendDailySummary and body builder to email.go"
```

---

### Task 3: Scheduler (`scheduler.go`) and main.go wire-up

**Files:**
- Create: `scheduler.go`
- Modify: `main.go`
- Modify: `main_test.go`

**Context:** `store` and `smtpCfg` are package-level vars (`store.go` line 24 and `main.go` line 10). `runDailyDigest` is a long-running goroutine — it never returns. The scheduler sleeps until the next 7:00 UTC, fires, then uses a ticker for subsequent daily fires.

- [ ] **Step 1: Write failing test**

Add to `main_test.go`:

```go
func TestNextSevenAMUTC(t *testing.T) {
	d := nextSevenAMUTC()
	if d <= 0 {
		t.Errorf("expected positive duration, got %v", d)
	}
	if d > 24*time.Hour {
		t.Errorf("expected at most 24h, got %v", d)
	}
}
```

- [ ] **Step 2: Run test — expect failure**

```
go test -v -run TestNextSevenAMUTC ./...
```

Expected: `FAIL` — `nextSevenAMUTC` not defined.

- [ ] **Step 3: Create `scheduler.go`**

```go
package main

import (
	"log"
	"time"
)

// nextSevenAMUTC returns the duration from now until the next 7:00:00 UTC.
// Always returns a value in the range (0, 24h].
func nextSevenAMUTC() time.Duration {
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day(), 7, 0, 0, 0, time.UTC)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return time.Until(next)
}

// runDailyDigest sleeps until the next 7:00 UTC then fires the daily digest,
// repeating every 24 hours. Intended to run as a goroutine.
func runDailyDigest() {
	time.Sleep(nextSevenAMUTC())
	sendDailyDigestToAll()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		sendDailyDigestToAll()
	}
}

// sendDailyDigestToAll queries all opted-in verified users and sends each a
// summary of releases published the previous UTC calendar day. Skips users
// with no releases that day.
func sendDailyDigestToAll() {
	users := store.GetDigestUsers()
	if len(users) == 0 {
		return
	}
	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	for _, u := range users {
		releases := store.GetReleasesPublishedOn(u.ID, yesterday)
		if len(releases) == 0 {
			continue
		}
		if err := smtpCfg.SendDailySummary(u.Email, releases); err != nil {
			log.Printf("⚠ digest send failed for %s: %v", u.Email, err)
		}
	}
}
```

- [ ] **Step 4: Wire scheduler into `main.go`**

In `main()`, after the `smtpCfg, smtpEnabled = SMTPConfigFromEnv()` block and its warning log, add:

```go
	if smtpEnabled {
		go runDailyDigest()
	}
```

The full updated `main()` becomes:

```go
func main() {
	log.Println("Starting Release Tracker...")

	var err error
	store, err = NewStore("data/newreleases.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	smtpCfg, smtpEnabled = SMTPConfigFromEnv()
	if !smtpEnabled {
		log.Println("⚠ SMTP not configured — email features disabled")
	}

	if smtpEnabled {
		go runDailyDigest()
	}

	http.HandleFunc("/", handleHome)
	http.HandleFunc("/sw.js", handleServiceWorker)
	http.HandleFunc("/verify-email", handleVerifyEmail)
	http.HandleFunc("/api/me", requireAuth(handleMe))
	http.HandleFunc("/api/register", handleRegister)
	http.HandleFunc("/api/login", handleLogin)
	http.HandleFunc("/api/logout", handleLogout)
	http.HandleFunc("/api/resend-verification", requireAuth(handleResendVerification))
	http.HandleFunc("/api/projects", requireAuth(handleProjects))
	http.HandleFunc("/api/releases", requireAuth(handleReleases))
	http.HandleFunc("/api/refresh-check", requireAuth(handleRefreshCheck))
	http.HandleFunc("/api/refresh", requireAuth(handleRefreshProject))
	http.HandleFunc("/api/webhooks", requireAuth(handleWebhooks))
	http.HandleFunc("/api/project-settings", requireAuth(handleProjectSettings))
	http.HandleFunc("/api/push/vapid-key", requireAuth(handlePushVapidKey))
	http.HandleFunc("/api/push/subscribe", requireAuth(handlePushSubscribe))

	fmt.Println("Release Tracker running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
```

- [ ] **Step 5: Run test — expect pass**

```
go test -v -run TestNextSevenAMUTC ./...
```

Expected: `PASS`.

- [ ] **Step 6: Run full suite**

```
go test ./...
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add scheduler.go main.go main_test.go
git commit -m "feat: daily digest scheduler goroutine at 7am UTC"
```

---

### Task 4: Account settings API handler

**Files:**
- Modify: `handlers.go`
- Modify: `main.go`
- Modify: `main_test.go`

**Context:** Existing pattern: `handleWebhooks` in `handlers.go` dispatches on `r.Method` and returns JSON. Handler signature is `func(w http.ResponseWriter, r *http.Request, userID string)`. Route registered via `requireAuth(handleXxx)` in `main.go`. Tests use `requireAuth(handleXxx).ServeHTTP(w, req)` with a session cookie added to the request.

- [ ] **Step 1: Write failing tests**

Add to `main_test.go`:

```go
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
	var resp map[string]bool
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["email_digest"] {
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
```

- [ ] **Step 2: Run tests — expect failure**

```
go test -v -run "TestHandleAccountSettings" ./...
```

Expected: `FAIL` — `handleAccountSettings` not defined.

- [ ] **Step 3: Add `handleAccountSettings` to `handlers.go`**

Append to `handlers.go`:

```go
// GET /api/account-settings — returns account-level preferences for the current user.
// POST /api/account-settings — updates account-level preferences.
func handleAccountSettings(w http.ResponseWriter, r *http.Request, userID string) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		enabled, err := store.GetUserDigestEnabled(userID)
		if err != nil {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"email_digest": enabled})

	case http.MethodPost:
		var req struct {
			EmailDigest bool `json:"email_digest"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := store.SetEmailDigest(userID, req.EmailDigest); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
```

- [ ] **Step 4: Register route in `main.go`**

In `main()`, after `http.HandleFunc("/api/project-settings", requireAuth(handleProjectSettings))`, add:

```go
	http.HandleFunc("/api/account-settings", requireAuth(handleAccountSettings))
```

- [ ] **Step 5: Run tests — expect pass**

```
go test -v -run "TestHandleAccountSettings" ./...
```

Expected: all `PASS`.

- [ ] **Step 6: Run full suite**

```
go test ./...
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add handlers.go main.go main_test.go
git commit -m "feat: account settings API handler for email digest preference"
```

---

### Task 5: Account settings panel in UI

**Files:**
- Modify: `ui.go`

**Context:** `ui.go` is one large Go string constant (`tmplHTML`). All JS and HTML changes go inside this string. Key existing patterns to follow:

- **CSS slide-over panel pattern** (around line 155): `.settings-panel { max-width:0; overflow:hidden; transition:max-width 0.25s ease; flex-shrink:0; }` / `.settings-panel.open { max-width: 380px; }` with an `-inner` div for the content.
- **HTML panel structure** (around line 491): panels sit inside `<div class="releases-area">` alongside `<div id="releases-root">`.
- **Panel title structure**: uses class `add-panel-title` with flex layout (title text + close button).
- **JS cross-panel management**: `toggleAddPanel()` calls `closeSettingsPanel()`; `openSettingsPanel()` calls `closeAddPanel()`. Escape key closes all panels.
- **`currentUser`** global (set at line 610): has `username`, `email_verified`, `smtp_enabled` fields already loaded from `/api/me`.
- **Toast**: call `toast(msg, 'ok')` for success, `toast(msg, 'error')` for error.
- **DOM construction**: use `document.createElement` and `.textContent` for user-derived strings. Never put unsanitized user data into a string that gets assigned to `.innerHTML`.

No unit tests for this task — it's UI-only. Build and manually verify.

- [ ] **Step 1: Add CSS for `.account-panel`**

Find the line containing `.settings-panel-inner {` (around line 163). The block ends with `min-height: 100%;` followed by `}`. After that closing brace, add:

```css
/* ---- Account settings panel ---- */
.account-panel {
    max-width: 0;
    overflow: hidden;
    transition: max-width 0.25s ease;
    flex-shrink: 0;
}
.account-panel.open { max-width: 300px; }
.account-panel-inner {
    width: 280px;
    background: #1a2840;
    border-left: 1px solid #2d4a6e;
    padding: 1.1rem 1rem;
    min-height: 100%;
}
```

- [ ] **Step 2: Add account button to header**

Find the header `<div class="user-info">` block (around line 439). It currently reads:

```html
    <div class="user-info">
        <span class="username" id="username-display"></span>
        <button class="btn btn-ghost" id="notif-btn" ...>&#x1F514;</button>
        <button class="btn btn-ghost" onclick="doLogout()">Sign out</button>
    </div>
```

Insert the account button between `<span class="username" id="username-display"></span>` and the notif button:

```html
        <button class="btn btn-ghost" onclick="openAccountPanel()" title="Account settings" style="font-size:1rem;padding:0.3rem 0.5rem">&#x2699;</button>
```

- [ ] **Step 3: Add account panel HTML inside `.releases-area`**

Find the closing `</div>` of the `<div id="settings-panel" class="settings-panel">` block (the `</div>` right before the `</div>` that closes `class="releases-area"`). After the settings-panel closing tag, add:

```html
        <div id="account-panel" class="account-panel">
            <div class="account-panel-inner">
                <div class="add-panel-title">
                    Account Settings
                    <button class="btn btn-ghost" onclick="closeAccountPanel()" style="font-size:1rem;padding:0.2rem 0.4rem">&#x2715;</button>
                </div>
                <div id="account-panel-body"></div>
            </div>
        </div>
```

- [ ] **Step 4: Add account panel JS functions**

Find the `// ---- Add panel ----` comment (around line 675). After `function closeAddPanel()` and before the `document.addEventListener('keydown', ...)` line, add:

```js
// ---- Account panel ----
function openAccountPanel() {
    closeAddPanel();
    closeSettingsPanel();
    document.getElementById('account-panel').classList.add('open');
    loadAccountPanel();
}

function closeAccountPanel() {
    document.getElementById('account-panel').classList.remove('open');
}

function loadAccountPanel() {
    var body = document.getElementById('account-panel-body');

    // Show loading indicator using safe DOM construction
    body.textContent = '';
    var loading = document.createElement('div');
    loading.style.cssText = 'font-size:0.85rem;color:#64748b';
    loading.textContent = 'Loading...';
    body.appendChild(loading);

    fetch('/api/account-settings')
        .then(function(r) { return r.json(); })
        .then(function(settings) {
            var smtpOk = !!(currentUser && currentUser.smtp_enabled);
            var verifiedOk = !!(currentUser && currentUser.email_verified);
            var canUse = smtpOk && verifiedOk;
            var title = !smtpOk ? 'Email not configured' : (!verifiedOk ? 'Verify your email to enable' : '');

            body.textContent = '';

            // Username row
            var userSection = document.createElement('div');
            userSection.style.marginBottom = '1rem';
            var userLabel = document.createElement('div');
            userLabel.style.cssText = 'font-size:0.8rem;color:#64748b;margin-bottom:0.25rem';
            userLabel.textContent = 'Username';
            var userValue = document.createElement('div');
            userValue.style.fontSize = '0.9rem';
            userValue.textContent = currentUser ? currentUser.username : '';
            userSection.appendChild(userLabel);
            userSection.appendChild(userValue);
            body.appendChild(userSection);

            // Daily email summary toggle row
            var row = document.createElement('div');
            row.style.cssText = 'display:flex;align-items:center;justify-content:space-between;padding:0.6rem 0;border-top:1px solid #334155';

            var lbl = document.createElement('label');
            lbl.htmlFor = 'digest-toggle';
            lbl.style.cssText = 'font-size:0.85rem;cursor:' + (canUse ? 'pointer' : 'default');
            lbl.textContent = 'Daily email summary';

            var chk = document.createElement('input');
            chk.type = 'checkbox';
            chk.id = 'digest-toggle';
            chk.checked = !!settings.email_digest;
            chk.disabled = !canUse;
            if (title) { chk.title = title; }
            chk.onchange = function() { toggleEmailDigest(this); };

            row.appendChild(lbl);
            row.appendChild(chk);
            body.appendChild(row);
        })
        .catch(function() {
            body.textContent = '';
            var err = document.createElement('div');
            err.style.cssText = 'color:#f87171;font-size:0.85rem';
            err.textContent = 'Failed to load settings.';
            body.appendChild(err);
        });
}

function toggleEmailDigest(checkbox) {
    fetch('/api/account-settings', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({email_digest: checkbox.checked})
    }).then(function(r) {
        if (!r.ok) return r.text().then(function(t) { throw new Error(t || 'Failed to save'); });
        toast('Settings saved', 'ok');
    }).catch(function() {
        checkbox.checked = !checkbox.checked;
        toast('Failed to save settings', 'error');
    });
}
```

- [ ] **Step 5: Update cross-panel close calls and Escape handler**

**Escape handler** — find:

```js
document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') { closeAddPanel(); closeSettingsPanel(); }
});
```

Replace with:

```js
document.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') { closeAddPanel(); closeSettingsPanel(); closeAccountPanel(); }
});
```

**`toggleAddPanel`** — find:

```js
function toggleAddPanel() {
    closeSettingsPanel();
    var panel = document.getElementById('add-panel');
    panel.classList.toggle('open');
}
```

Replace with:

```js
function toggleAddPanel() {
    closeSettingsPanel();
    closeAccountPanel();
    var panel = document.getElementById('add-panel');
    panel.classList.toggle('open');
}
```

**`openSettingsPanel`** — find:

```js
function openSettingsPanel(btn) {
    var id = btn.dataset.id;
    var name = btn.dataset.name;
    var pushEnabled = btn.dataset.push === '1';
    _settingsRepoID = id;
    document.getElementById('settings-panel-title').textContent = name;
    closeAddPanel();
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
    _settingsRepoID = id;
    document.getElementById('settings-panel-title').textContent = name;
    closeAddPanel();
    closeAccountPanel();
    document.getElementById('settings-panel').classList.add('open');
    loadSettingsPanel(id, pushEnabled);
}
```

- [ ] **Step 6: Build and verify**

```
go build -o newreleases .
```

Expected: builds cleanly, no compile errors.

- [ ] **Step 7: Run full test suite**

```
go test ./...
```

Expected: all pass.

- [ ] **Step 8: Commit**

```bash
git add ui.go
git commit -m "feat: account settings panel with daily email digest toggle"
```

---

## Done

All tasks complete. The daily email digest feature is fully implemented:

- Users with `email_digest = 1`, a verified email address, and SMTP configured receive an email at 7:00 UTC listing yesterday's releases. No email is sent when there are no releases to report.
- The account settings panel (&#x2699; in header) lets users opt in and out, with the toggle disabled and labelled when email is not configured or email is unverified.
