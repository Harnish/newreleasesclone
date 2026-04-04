# Email Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add email collection and verification to registration — unverified users can log in but email notification features are disabled until verified.

**Architecture:** All code is `package main`. `email.go` is a new file for SMTP infrastructure. Schema migrates to v6 (adds `email`/`email_verified` to users, new `email_verification_tokens` table). Handlers and store follow existing patterns. SMTP is configured via env vars; if missing, the app starts normally with email sending disabled.

**Tech Stack:** Go stdlib (`net/smtp`, `os`, `sync`), SQLite (modernc), vanilla JS/CSS in `ui.go`

---

### Task 1: Add email fields to User model and migrate schema to v6

**Files:**
- Modify: `models.go`
- Modify: `store.go` — `migrate()` function and `GetUserByID`, `AuthenticateUser`
- Modify: `main_test.go` — add `TestStoreEmailFields`

- [ ] **Step 1: Write the failing test**

Add to `main_test.go`:

```go
func TestStoreEmailFields(t *testing.T) {
	s := newTestStore(t)
	user, err := s.CreateUser("emailuser", "password123")
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	got, err := s.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if got.Email != "" {
		t.Errorf("expected empty email, got %q", got.Email)
	}
	if got.EmailVerified {
		t.Error("expected email_verified false for new user")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test -v -run TestStoreEmailFields ./...
```
Expected: FAIL — `got.Email` field does not exist on User struct yet.

- [ ] **Step 3: Update `models.go`**

Replace the `User` struct:

```go
type User struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}
```

- [ ] **Step 4: Add migration v6 to `store.go`**

In `migrate()`, after the `version < 5` block (around line 175), add:

```go
	if version < 6 {
		if _, err := db.Exec(`ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("failed to migrate to v6 (email): %w", err)
		}
		if _, err := db.Exec(`ALTER TABLE users ADD COLUMN email_verified INTEGER NOT NULL DEFAULT 0`); err != nil {
			return fmt.Errorf("failed to migrate to v6 (email_verified): %w", err)
		}
		if _, err := db.Exec(`
			CREATE TABLE IF NOT EXISTS email_verification_tokens (
				token      TEXT PRIMARY KEY,
				user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				expires_at TEXT NOT NULL
			)
		`); err != nil {
			return fmt.Errorf("failed to migrate to v6 (tokens table): %w", err)
		}
		if _, err := db.Exec("PRAGMA user_version = 6"); err != nil {
			return fmt.Errorf("failed to set schema version: %w", err)
		}
	}
```

- [ ] **Step 5: Update `GetUserByID` in `store.go`**

Replace the existing `GetUserByID` method (~line 500):

```go
func (s *Store) GetUserByID(userID string) (*User, error) {
	var u User
	var emailVerified int
	err := s.db.QueryRow(
		"SELECT id, username, email, email_verified FROM users WHERE id = ?", userID,
	).Scan(&u.ID, &u.Username, &u.Email, &emailVerified)
	if err != nil {
		return nil, err
	}
	u.EmailVerified = emailVerified == 1
	return &u, nil
}
```

- [ ] **Step 6: Update `AuthenticateUser` in `store.go`**

Replace the existing `AuthenticateUser` method (~line 455):

```go
func (s *Store) AuthenticateUser(username, password string) (*User, error) {
	var u User
	var hash string
	var emailVerified int
	err := s.db.QueryRow(
		"SELECT id, username, password_hash, email, email_verified FROM users WHERE username = ?", username,
	).Scan(&u.ID, &u.Username, &hash, &u.Email, &emailVerified)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("invalid credentials")
	}
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	u.EmailVerified = emailVerified == 1
	return &u, nil
}
```

- [ ] **Step 7: Run the test to verify it passes**

```bash
go test -v -run TestStoreEmailFields ./...
```
Expected: PASS

- [ ] **Step 8: Run all tests**

```bash
go test -v ./...
```
Expected: all PASS

- [ ] **Step 9: Commit**

```bash
git add models.go store.go main_test.go
git commit -m "feat: add email/email_verified to User model and migrate schema to v6"
```

---

### Task 2: Add store methods for email verification tokens

**Files:**
- Modify: `store.go` — add `SetUserEmail`, `CreateVerificationToken`, `VerifyEmailToken`, `DeleteVerificationTokensForUser`, error sentinels
- Modify: `main_test.go` — add token store tests

- [ ] **Step 1: Write the failing tests**

Add to `main_test.go`:

```go
func TestStoreSetUserEmail(t *testing.T) {
	s := newTestStore(t)
	user, _ := s.CreateUser("emailtest", "password123")

	if err := s.SetUserEmail(user.ID, "test@example.com"); err != nil {
		t.Fatalf("SetUserEmail failed: %v", err)
	}
	got, _ := s.GetUserByID(user.ID)
	if got.Email != "test@example.com" {
		t.Errorf("expected email test@example.com, got %q", got.Email)
	}
}

func TestStoreVerificationToken(t *testing.T) {
	s := newTestStore(t)
	user, _ := s.CreateUser("tokentest", "password123")
	s.SetUserEmail(user.ID, "token@example.com")

	token, err := s.CreateVerificationToken(user.ID)
	if err != nil {
		t.Fatalf("CreateVerificationToken failed: %v", err)
	}
	if len(token) == 0 {
		t.Fatal("expected non-empty token")
	}

	// Verify it
	gotUserID, err := s.VerifyEmailToken(token)
	if err != nil {
		t.Fatalf("VerifyEmailToken failed: %v", err)
	}
	if gotUserID != user.ID {
		t.Errorf("expected userID %s, got %s", user.ID, gotUserID)
	}

	// User should now be verified
	got, _ := s.GetUserByID(user.ID)
	if !got.EmailVerified {
		t.Error("expected email_verified true after verification")
	}

	// Token should be consumed
	_, err = s.VerifyEmailToken(token)
	if err != ErrTokenNotFound {
		t.Errorf("expected ErrTokenNotFound after use, got %v", err)
	}
}

func TestStoreVerifyEmailTokenExpired(t *testing.T) {
	s := newTestStore(t)
	user, _ := s.CreateUser("expiretest", "password123")

	// Insert an already-expired token directly
	expiredAt := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	s.db.Exec(
		"INSERT INTO email_verification_tokens (token, user_id, expires_at) VALUES (?, ?, ?)",
		"expiredtoken", user.ID, expiredAt,
	)

	_, err := s.VerifyEmailToken("expiredtoken")
	if err != ErrTokenExpired {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}
}

func TestStoreVerifyEmailTokenNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.VerifyEmailToken("doesnotexist")
	if err != ErrTokenNotFound {
		t.Errorf("expected ErrTokenNotFound, got %v", err)
	}
}

func TestStoreCreateVerificationTokenDeletesOld(t *testing.T) {
	s := newTestStore(t)
	user, _ := s.CreateUser("reusetest", "password123")

	token1, _ := s.CreateVerificationToken(user.ID)
	token2, _ := s.CreateVerificationToken(user.ID)

	if token1 == token2 {
		t.Error("expected different tokens")
	}
	// Old token should be gone
	_, err := s.VerifyEmailToken(token1)
	if err != ErrTokenNotFound {
		t.Errorf("expected old token deleted, got %v", err)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test -v -run "TestStoreSetUserEmail|TestStoreVerification|TestStoreVerifyEmail|TestStoreCreateVerification" ./...
```
Expected: FAIL — methods not defined yet.

- [ ] **Step 3: Add error sentinels and new methods to `store.go`**

Add after the `GetUserByID` method (after the `// ---- Repos ...` comment):

```go
var (
	ErrTokenNotFound = fmt.Errorf("token not found")
	ErrTokenExpired  = fmt.Errorf("token expired")
)

func (s *Store) SetUserEmail(userID, email string) error {
	_, err := s.db.Exec("UPDATE users SET email = ? WHERE id = ?", email, userID)
	return err
}

func (s *Store) CreateVerificationToken(userID string) (string, error) {
	s.db.Exec("DELETE FROM email_verification_tokens WHERE user_id = ?", userID)
	token := generateToken()
	expires := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
	_, err := s.db.Exec(
		"INSERT INTO email_verification_tokens (token, user_id, expires_at) VALUES (?, ?, ?)",
		token, userID, expires,
	)
	return token, err
}

func (s *Store) VerifyEmailToken(token string) (string, error) {
	var userID, expiresAt string
	err := s.db.QueryRow(
		"SELECT user_id, expires_at FROM email_verification_tokens WHERE token = ?", token,
	).Scan(&userID, &expiresAt)
	if err == sql.ErrNoRows {
		return "", ErrTokenNotFound
	}
	if err != nil {
		return "", err
	}
	t, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil || time.Now().After(t) {
		s.db.Exec("DELETE FROM email_verification_tokens WHERE token = ?", token)
		return "", ErrTokenExpired
	}
	if _, err = s.db.Exec("UPDATE users SET email_verified = 1 WHERE id = ?", userID); err != nil {
		return "", err
	}
	s.db.Exec("DELETE FROM email_verification_tokens WHERE token = ?", token)
	return userID, nil
}

func (s *Store) DeleteVerificationTokensForUser(userID string) error {
	_, err := s.db.Exec("DELETE FROM email_verification_tokens WHERE user_id = ?", userID)
	return err
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test -v -run "TestStoreSetUserEmail|TestStoreVerification|TestStoreVerifyEmail|TestStoreCreateVerification" ./...
```
Expected: all PASS

- [ ] **Step 5: Run all tests**

```bash
go test -v ./...
```
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add store.go main_test.go
git commit -m "feat: add email verification token store methods"
```

---

### Task 3: Create `email.go` — SMTP infrastructure

**Files:**
- Create: `email.go`
- Modify: `main_test.go` — add `TestSMTPConfigFromEnv`

- [ ] **Step 1: Write the failing test**

Add to `main_test.go`:

```go
func TestSMTPConfigFromEnv(t *testing.T) {
	// No env vars set — should return disabled
	cfg, ok := SMTPConfigFromEnv()
	if ok {
		t.Errorf("expected disabled with no env vars, got config: %+v", cfg)
	}

	// Set required vars
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_USER", "user@example.com")
	t.Setenv("SMTP_PASS", "secret")
	t.Setenv("SMTP_FROM", "noreply@example.com")

	cfg, ok = SMTPConfigFromEnv()
	if !ok {
		t.Fatal("expected enabled with all required env vars")
	}
	if cfg.Host != "smtp.example.com" {
		t.Errorf("expected Host smtp.example.com, got %q", cfg.Host)
	}
	if cfg.Port != 587 {
		t.Errorf("expected default Port 587, got %d", cfg.Port)
	}
	if cfg.User != "user@example.com" {
		t.Errorf("expected User user@example.com, got %q", cfg.User)
	}

	// Custom port
	t.Setenv("SMTP_PORT", "465")
	cfg, _ = SMTPConfigFromEnv()
	if cfg.Port != 465 {
		t.Errorf("expected Port 465, got %d", cfg.Port)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

```bash
go test -v -run TestSMTPConfigFromEnv ./...
```
Expected: FAIL — `SMTPConfigFromEnv` not defined.

- [ ] **Step 3: Create `email.go`**

```go
package main

import (
	"fmt"
	"net/smtp"
	"os"
	"strconv"
)

// SMTPConfig holds SMTP connection settings read from environment variables.
type SMTPConfig struct {
	Host string
	Port int
	User string
	Pass string
	From string
}

// SMTPConfigFromEnv reads SMTP settings from environment variables.
// Returns (config, true) if all required vars are present, (zero, false) otherwise.
func SMTPConfigFromEnv() (SMTPConfig, bool) {
	host := os.Getenv("SMTP_HOST")
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")
	from := os.Getenv("SMTP_FROM")
	if host == "" || user == "" || pass == "" || from == "" {
		return SMTPConfig{}, false
	}
	port := 587
	if p := os.Getenv("SMTP_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}
	return SMTPConfig{Host: host, Port: port, User: user, Pass: pass, From: from}, true
}

// SendMail sends a plain-text email via SMTP using AUTH PLAIN.
func (c SMTPConfig) SendMail(to, subject, body string) error {
	auth := smtp.PlainAuth("", c.User, c.Pass, c.Host)
	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s",
		c.From, to, subject, body,
	)
	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	return smtp.SendMail(addr, auth, c.From, []string{to}, []byte(msg))
}

// SendVerificationEmail sends a verification link email to the given address.
// baseURL should be e.g. "https://example.com" (no trailing slash).
func (c SMTPConfig) SendVerificationEmail(to, token, baseURL string) error {
	link := baseURL + "/verify-email?token=" + token
	body := fmt.Sprintf(
		"Hello,\n\nPlease verify your email address by clicking the link below:\n\n%s\n\nThis link expires in 24 hours.\n\nIf you did not create an account, you can ignore this email.\n",
		link,
	)
	return c.SendMail(to, "Verify your Release Tracker email", body)
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test -v -run TestSMTPConfigFromEnv ./...
```
Expected: PASS

- [ ] **Step 5: Run all tests**

```bash
go test -v ./...
```
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add email.go main_test.go
git commit -m "feat: add SMTP email infrastructure (email.go)"
```

---

### Task 4: Update `handleRegister` to accept email and send verification

**Files:**
- Modify: `handlers.go` — `handleRegister`, add `requestBaseURL` helper, add `smtpCfg`/`smtpEnabled` package vars (actually in main.go — see Task 6; for now declare them here so it compiles)
- Modify: `main_test.go` — update `TestHandleRegisterWithEmail`

**Note:** `smtpCfg` and `smtpEnabled` are package-level vars in `main.go` (added in Task 6). Since all files are `package main`, they're visible in `handlers.go` without import.

- [ ] **Step 1: Write the failing test**

Add to `main_test.go`:

```go
func TestHandleRegisterWithEmail(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	body := `{"username":"newuser","password":"password123","email":"new@example.com"}`
	req, _ := http.NewRequest("POST", "/api/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", "localhost:8080")
	w := httptest.NewRecorder()
	handleRegister(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var user User
	if err := json.NewDecoder(w.Body).Decode(&user); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if user.Email != "new@example.com" {
		t.Errorf("expected email new@example.com in response, got %q", user.Email)
	}
	if user.EmailVerified {
		t.Error("expected email_verified false on fresh registration")
	}
}

func TestHandleRegisterMissingEmail(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	body := `{"username":"nomail","password":"password123"}`
	req, _ := http.NewRequest("POST", "/api/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handleRegister(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test -v -run "TestHandleRegisterWithEmail|TestHandleRegisterMissingEmail" ./...
```
Expected: FAIL — handler doesn't accept/return email yet, and `smtpCfg`/`smtpEnabled` not declared.

- [ ] **Step 3: Add `smtpCfg`/`smtpEnabled` package vars temporarily**

At the top of `handlers.go`, add these vars so the package compiles while Task 6 (main.go wiring) is pending. They'll be initialized properly in Task 6.

Add after the imports in `handlers.go`:

```go
// smtpCfg and smtpEnabled are initialized in main() via SMTPConfigFromEnv.
var smtpCfg SMTPConfig
var smtpEnabled bool
```

- [ ] **Step 4: Add `requestBaseURL` helper to `handlers.go`**

Add after the `requireAuth` function:

```go
// requestBaseURL returns "http(s)://host" from the incoming request.
func requestBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
```

- [ ] **Step 5: Update `handleRegister` in `handlers.go`**

Replace the entire `handleRegister` function:

```go
// POST /api/register
func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(req.Email)
	if len(req.Username) < 3 {
		http.Error(w, "Username must be at least 3 characters", http.StatusBadRequest)
		return
	}
	if len(req.Password) < 8 {
		http.Error(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	if !strings.Contains(req.Email, "@") {
		http.Error(w, "Invalid email address", http.StatusBadRequest)
		return
	}
	user, err := store.CreateUser(req.Username, req.Password)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			http.Error(w, "Username already taken", http.StatusConflict)
		} else {
			http.Error(w, "Failed to create account", http.StatusInternalServerError)
		}
		return
	}
	if err := store.SetUserEmail(user.ID, req.Email); err != nil {
		log.Printf("⚠ failed to set email for %s: %v", user.ID, err)
	} else {
		user.Email = req.Email
	}
	token, err := store.CreateVerificationToken(user.ID)
	if err != nil {
		log.Printf("⚠ failed to create verification token for %s: %v", user.ID, err)
	} else if smtpEnabled {
		if err := smtpCfg.SendVerificationEmail(req.Email, token, requestBaseURL(r)); err != nil {
			log.Printf("⚠ failed to send verification email to %s: %v", req.Email, err)
		}
	}
	sessionID := store.CreateSession(user.ID)
	setSessionCookie(w, sessionID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}
```

- [ ] **Step 6: Add `"log"` to `handlers.go` imports**

The imports block in `handlers.go` currently is:
```go
import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)
```

Replace with:
```go
import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)
```

- [ ] **Step 7: Run tests to verify they pass**

```bash
go test -v -run "TestHandleRegisterWithEmail|TestHandleRegisterMissingEmail" ./...
```
Expected: PASS

- [ ] **Step 8: Run all tests**

```bash
go test -v ./...
```
Expected: all PASS

- [ ] **Step 9: Commit**

```bash
git add handlers.go main_test.go
git commit -m "feat: update handleRegister to accept email and send verification"
```

---

### Task 5: Add `handleVerifyEmail` and `handleResendVerification` handlers

**Files:**
- Modify: `handlers.go` — add handlers + resend rate limiter
- Modify: `main_test.go` — add tests

- [ ] **Step 1: Write failing tests**

Add to `main_test.go`:

```go
func TestHandleVerifyEmail(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	user, _ := store.CreateUser("verifyuser", "password123")
	store.SetUserEmail(user.ID, "verify@example.com")
	token, _ := store.CreateVerificationToken(user.ID)

	req, _ := http.NewRequest("GET", "/verify-email?token="+token, nil)
	w := httptest.NewRecorder()
	handleVerifyEmail(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/?verified=1" {
		t.Errorf("expected redirect to /?verified=1, got %q", loc)
	}

	got, _ := store.GetUserByID(user.ID)
	if !got.EmailVerified {
		t.Error("expected email_verified true after verification")
	}
}

func TestHandleVerifyEmailInvalidToken(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)

	req, _ := http.NewRequest("GET", "/verify-email?token=badtoken", nil)
	w := httptest.NewRecorder()
	handleVerifyEmail(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/?verify_error=1" {
		t.Errorf("expected redirect to /?verify_error=1, got %q", loc)
	}
}

func TestHandleResendVerification(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)
	userID, cookie := newTestAuth(t, store)
	store.SetUserEmail(userID, "resend@example.com")

	req, _ := http.NewRequest("POST", "/api/resend-verification", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleResendVerification).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleResendVerificationAlreadyVerified(t *testing.T) {
	originalStore := store
	defer func() { store = originalStore }()
	store = newTestStore(t)
	userID, cookie := newTestAuth(t, store)
	store.SetUserEmail(userID, "done@example.com")
	// Mark verified directly
	store.db.Exec("UPDATE users SET email_verified = 1 WHERE id = ?", userID)

	req, _ := http.NewRequest("POST", "/api/resend-verification", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	requireAuth(handleResendVerification).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Run to verify they fail**

```bash
go test -v -run "TestHandleVerifyEmail|TestHandleResendVerification" ./...
```
Expected: FAIL — handlers not defined.

- [ ] **Step 3: Add rate limiter vars and new handlers to `handlers.go`**

Add after the `smtpCfg`/`smtpEnabled` vars at the top of `handlers.go`:

```go
var (
	resendMu       sync.Mutex
	resendAttempts = map[string][]time.Time{}
)
```

Add after the `handleMe` function:

```go
// GET /verify-email?token=xxx — verifies email token, redirects with result query param.
func handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Redirect(w, r, "/?verify_error=1", http.StatusFound)
		return
	}
	if _, err := store.VerifyEmailToken(token); err != nil {
		http.Redirect(w, r, "/?verify_error=1", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/?verified=1", http.StatusFound)
}

// POST /api/resend-verification — resends verification email; requires auth.
func handleResendVerification(w http.ResponseWriter, r *http.Request, userID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	user, err := store.GetUserByID(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}
	if user.EmailVerified {
		http.Error(w, "Email already verified", http.StatusBadRequest)
		return
	}
	// Rate limit: max 3 resends per 10 minutes per user.
	resendMu.Lock()
	now := time.Now()
	cutoff := now.Add(-10 * time.Minute)
	var recent []time.Time
	for _, t := range resendAttempts[userID] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	if len(recent) >= 3 {
		resendMu.Unlock()
		http.Error(w, "Too many requests", http.StatusTooManyRequests)
		return
	}
	resendAttempts[userID] = append(recent, now)
	resendMu.Unlock()

	token, err := store.CreateVerificationToken(userID)
	if err != nil {
		http.Error(w, "Failed to create token", http.StatusInternalServerError)
		return
	}
	if smtpEnabled {
		if err := smtpCfg.SendVerificationEmail(user.Email, token, requestBaseURL(r)); err != nil {
			log.Printf("⚠ failed to resend verification email to %s: %v", user.Email, err)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test -v -run "TestHandleVerifyEmail|TestHandleResendVerification" ./...
```
Expected: all PASS

- [ ] **Step 5: Run all tests**

```bash
go test -v ./...
```
Expected: all PASS

- [ ] **Step 6: Commit**

```bash
git add handlers.go main_test.go
git commit -m "feat: add handleVerifyEmail and handleResendVerification handlers"
```

---

### Task 6: Register routes and initialize SMTP config in `main.go`

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Update `main.go`**

Replace the entire file:

```go
package main

import (
	"fmt"
	"log"
	"net/http"
)

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

- [ ] **Step 2: Remove the temporary vars from `handlers.go`**

In `handlers.go`, remove these two lines that were added temporarily in Task 4:

```go
// smtpCfg and smtpEnabled are initialized in main() via SMTPConfigFromEnv.
var smtpCfg SMTPConfig
var smtpEnabled bool
```

- [ ] **Step 3: Build to verify no compile errors**

```bash
go build -o newreleases .
```
Expected: exits 0.

- [ ] **Step 4: Run all tests**

```bash
go test -v ./...
```
Expected: all PASS

- [ ] **Step 5: Commit**

```bash
git add main.go handlers.go
git commit -m "feat: register email routes and initialize SMTP config in main"
```

---

### Task 7: Frontend changes in `ui.go`

**Files:**
- Modify: `ui.go` — email field in register form, verification banner HTML + CSS + JS, toast on page load

- [ ] **Step 1: Add email field to the register form**

In `ui.go`, find the register form (around line 414). Find:

```html
        <div class="form-group">
            <label>Password <span style="color:#475569">(min 8 chars)</span></label>
            <input type="password" name="password" required minlength="8" autocomplete="new-password">
        </div>
        <button type="submit" class="btn btn-primary" style="width:100%" id="register-btn">Create Account</button>
```

Replace with:

```html
        <div class="form-group">
            <label>Password <span style="color:#475569">(min 8 chars)</span></label>
            <input type="password" name="password" required minlength="8" autocomplete="new-password">
        </div>
        <div class="form-group">
            <label>Email</label>
            <input type="email" name="email" required autocomplete="email">
        </div>
        <button type="submit" class="btn btn-primary" style="width:100%" id="register-btn">Create Account</button>
```

- [ ] **Step 2: Add verification banner HTML to the app page**

In `ui.go`, find (around line 442):

```html
<div id="toast" class="toast"></div>
```

Replace with:

```html
<div id="toast" class="toast"></div>
<div id="verify-banner" style="display:none;background:#422006;color:#fbbf24;border-bottom:1px solid #92400e;padding:0.6rem 1.1rem;font-size:0.85rem;display:none">
    Please verify your email address.
    <button onclick="doResendVerification()" style="background:none;border:none;color:#fbbf24;text-decoration:underline;cursor:pointer;font-size:0.85rem;margin-left:0.5rem">Resend verification email</button>
    <button onclick="document.getElementById('verify-banner').style.display='none'" style="background:none;border:none;color:#fbbf24;cursor:pointer;float:right;font-size:1rem">&#x2715;</button>
</div>
```

- [ ] **Step 3: Add `updateVerifyBanner`, `doResendVerification`, and page-load toast JS**

In `ui.go`, find the `showApp` function:

```js
function showApp() {
    document.getElementById('auth-page').style.display = 'none';
    document.getElementById('app-page').style.display = '';
    document.getElementById('username-display').textContent = currentUser.username;
    fetch('/api/refresh-check').catch(function() {});
    initNotifications();
    loadReleases();
}
```

Replace with:

```js
function showApp() {
    document.getElementById('auth-page').style.display = 'none';
    document.getElementById('app-page').style.display = '';
    document.getElementById('username-display').textContent = currentUser.username;
    updateVerifyBanner();
    fetch('/api/refresh-check').catch(function() {});
    initNotifications();
    loadReleases();
}

function updateVerifyBanner() {
    var banner = document.getElementById('verify-banner');
    if (currentUser && !currentUser.email_verified && currentUser.email) {
        banner.style.display = '';
    } else {
        banner.style.display = 'none';
    }
}

function doResendVerification() {
    fetch('/api/resend-verification', { method: 'POST' })
        .then(function(r) {
            if (r.status === 429) throw new Error('Too many attempts. Try again later.');
            if (!r.ok) throw new Error('Failed to resend.');
            return r.json();
        })
        .then(function() {
            toast('Verification email sent! Check your inbox.', 'ok');
        })
        .catch(function(err) {
            toast(err.message, 'err');
        });
}
```

- [ ] **Step 4: Add page-load toast for verification result**

In `ui.go`, find the `init` function:

```js
function init() {
    fetch('/api/me')
        .then(function(r) {
            if (!r.ok) throw new Error('not authenticated');
            return r.json();
        })
        .then(function(user) {
            currentUser = user;
            showApp();
        })
        .catch(function() {
            document.getElementById('auth-page').style.display = '';
            document.getElementById('app-page').style.display = 'none';
        });
}
```

Replace with:

```js
function init() {
    var params = new URLSearchParams(window.location.search);
    if (params.get('verified') === '1') {
        history.replaceState(null, '', '/');
    } else if (params.get('verify_error') === '1') {
        history.replaceState(null, '', '/');
    }
    fetch('/api/me')
        .then(function(r) {
            if (!r.ok) throw new Error('not authenticated');
            return r.json();
        })
        .then(function(user) {
            currentUser = user;
            showApp();
            if (params.get('verified') === '1') {
                toast('Email verified!', 'ok');
            } else if (params.get('verify_error') === '1') {
                toast('Verification link expired or invalid. Please request a new one.', 'err');
            }
        })
        .catch(function() {
            document.getElementById('auth-page').style.display = '';
            document.getElementById('app-page').style.display = 'none';
        });
}
```

- [ ] **Step 5: Build to verify no compile errors**

```bash
go build -o newreleases .
```
Expected: exits 0.

- [ ] **Step 6: Run all tests**

```bash
go test -v ./...
```
Expected: all PASS

- [ ] **Step 7: Commit**

```bash
git add ui.go
git commit -m "feat: add email verification UI — register email field, banner, toasts"
```

---

### Task 8: Update TODO.md

**Files:**
- Modify: `TODO.md`

- [ ] **Step 1: Move item from Planned to Done**

In `TODO.md`, remove from Planned:
```
- [ ] **Email validation of users** - send an email and a verification link to the user to validate their email
```

Add to Done section:
```
- [x] **Email validation of users** — email collected on registration, verification link sent via SMTP; unverified users see banner with resend option; email features gated behind verification
```

- [ ] **Step 2: Commit**

```bash
git add TODO.md
git commit -m "chore: mark email validation complete in TODO"
```
