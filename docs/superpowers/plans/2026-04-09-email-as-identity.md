# Email as Identity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the arbitrary `username` field with `email` as the sole user identity and login credential, removing `username` entirely from the schema, code, and UI.

**Architecture:** A v10 SQLite migration rebuilds the `users` table (dropping `username`, adding `UNIQUE` on `email`). The `User` struct, all store methods, handlers, tests, and the frontend login/register forms are updated to use email throughout. Pre-deploy requirement: all existing users must have a non-empty unique email before migration runs.

**Tech Stack:** Go, SQLite via `modernc.org/sqlite`, vanilla JS embedded in `ui.go`.

---

## File Map

| File | Change |
|------|--------|
| `store.go` | v10 migration (table rebuild); `CreateUser`, `AuthenticateUser`, `GetUserByID`, `GetUserByRSSToken`, `GetDigestUsers` updated; `SetUserEmail` removed |
| `models.go` | Remove `Username string` from `User` struct |
| `handlers.go` | `handleRegister`: drop username field, call `CreateUser(email, password)`; `handleLogin`: use email field; `handleFeed`: use `user.Email` for title |
| `main_test.go` | `newTestAuth` uses email; `TestStoreAddRepoDedupe` uses emails; `TestHandleRegister` body updated; all `user.Username` references removed |
| `ui.go` | Login form: username input → email input; Register form: remove username field; header: show `currentUser.email`; account panel: show email |
| `TODO.md` | Mark email-as-username item done |

---

## Task 1: DB migration v10 (table rebuild, email UNIQUE NOT NULL)

**Files:**
- Modify: `store.go`
- Modify: `main_test.go`

- [ ] **Step 1.1: Write failing test**

Add to `main_test.go`:

```go
func TestMigrationV10EmailUnique(t *testing.T) {
    s := newTestStore(t)

    // After migration, duplicate email must fail.
    // newTestStore runs all migrations, so the schema is already v10 after this task.
    // Insert two rows with the same email directly to verify the UNIQUE constraint.
    _, err := s.db.Exec(
        `INSERT INTO users (id, email, password_hash, created_at) VALUES (?, ?, ?, ?)`,
        "user_aaa111aaa111", "dup@example.com", "hash", "2026-01-01T00:00:00Z",
    )
    if err != nil {
        t.Fatalf("first insert failed: %v", err)
    }
    _, err = s.db.Exec(
        `INSERT INTO users (id, email, password_hash, created_at) VALUES (?, ?, ?, ?)`,
        "user_bbb222bbb222", "dup@example.com", "hash", "2026-01-01T00:00:00Z",
    )
    if err == nil {
        t.Error("expected UNIQUE constraint error for duplicate email, got nil")
    }
}

func TestMigrationV10NoUsernameColumn(t *testing.T) {
    s := newTestStore(t)

    // The username column must not exist after v10 migration.
    _, err := s.db.Exec(`INSERT INTO users (id, username, password_hash, created_at) VALUES (?, ?, ?, ?)`,
        "user_test000", "shouldfail", "hash", "2026-01-01T00:00:00Z")
    if err == nil {
        t.Error("expected error inserting into username column (should not exist), got nil")
    }
}
```

- [ ] **Step 1.2: Run tests to verify they fail**

```bash
go test -v -run "TestMigrationV10" ./...
```

Expected: `TestMigrationV10NoUsernameColumn` FAIL — username column still exists.

- [ ] **Step 1.3: Add v10 migration block in `store.go`**

After the `version < 9` block and before `return nil`, add:

```go
	if version < 10 {
		if _, err := db.Exec(`
			CREATE TABLE users_new (
				id             TEXT PRIMARY KEY,
				email          TEXT NOT NULL UNIQUE,
				password_hash  TEXT NOT NULL,
				created_at     TEXT NOT NULL DEFAULT '',
				email_verified INTEGER NOT NULL DEFAULT 0,
				email_digest   INTEGER NOT NULL DEFAULT 0,
				rss_token      TEXT,
				page_size      INTEGER NOT NULL DEFAULT 10
			)
		`); err != nil {
			return fmt.Errorf("failed to migrate to v10 (create users_new): %w", err)
		}
		if _, err := db.Exec(`
			INSERT INTO users_new (id, email, password_hash, created_at, email_verified, email_digest, rss_token, page_size)
			SELECT id, email, password_hash, created_at, email_verified, email_digest, rss_token, page_size
			FROM users
		`); err != nil {
			return fmt.Errorf("failed to migrate to v10 (copy users): %w", err)
		}
		if _, err := db.Exec(`DROP TABLE users`); err != nil {
			return fmt.Errorf("failed to migrate to v10 (drop users): %w", err)
		}
		if _, err := db.Exec(`ALTER TABLE users_new RENAME TO users`); err != nil {
			return fmt.Errorf("failed to migrate to v10 (rename users_new): %w", err)
		}
		if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_rss_token ON users(rss_token) WHERE rss_token IS NOT NULL`); err != nil {
			return fmt.Errorf("failed to migrate to v10 (rss_token index): %w", err)
		}
		if _, err := db.Exec("PRAGMA user_version = 10"); err != nil {
			return fmt.Errorf("failed to set schema version: %w", err)
		}
	}
```

- [ ] **Step 1.4: Run tests to verify they pass**

```bash
go test -v -run "TestMigrationV10" ./...
```

Expected: both PASS.

- [ ] **Step 1.5: Run full test suite**

```bash
go test -v ./...
```

Expected: all tests pass (migration is additive to in-memory test stores).

- [ ] **Step 1.6: Commit**

```bash
git add store.go main_test.go
git commit -m "feat: schema v10 — drop username, email UNIQUE NOT NULL"
```

---

## Task 2: Update User struct and all store methods

**Files:**
- Modify: `models.go`
- Modify: `store.go`

This task causes compile errors immediately upon removing `Username` from the struct. Fix all store-layer usages in the same step before running tests.

- [ ] **Step 2.1: Write failing tests**

Add to `main_test.go`:

```go
func TestCreateUserByEmail(t *testing.T) {
    s := newTestStore(t)
    user, err := s.CreateUser("alice@example.com", "password123")
    if err != nil {
        t.Fatalf("CreateUser: %v", err)
    }
    if user.Email != "alice@example.com" {
        t.Errorf("expected email alice@example.com, got %q", user.Email)
    }
    if user.ID == "" {
        t.Error("expected non-empty ID")
    }
}

func TestCreateUserDuplicateEmail(t *testing.T) {
    s := newTestStore(t)
    if _, err := s.CreateUser("dup@example.com", "password123"); err != nil {
        t.Fatalf("first CreateUser: %v", err)
    }
    _, err := s.CreateUser("dup@example.com", "password123")
    if err == nil {
        t.Error("expected error for duplicate email, got nil")
    }
}

func TestAuthenticateUserByEmail(t *testing.T) {
    s := newTestStore(t)
    if _, err := s.CreateUser("bob@example.com", "secret123"); err != nil {
        t.Fatalf("CreateUser: %v", err)
    }
    user, err := s.AuthenticateUser("bob@example.com", "secret123")
    if err != nil {
        t.Fatalf("AuthenticateUser: %v", err)
    }
    if user.Email != "bob@example.com" {
        t.Errorf("expected email bob@example.com, got %q", user.Email)
    }
}

func TestAuthenticateUserWrongPassword(t *testing.T) {
    s := newTestStore(t)
    if _, err := s.CreateUser("carol@example.com", "rightpass"); err != nil {
        t.Fatalf("CreateUser: %v", err)
    }
    _, err := s.AuthenticateUser("carol@example.com", "wrongpass")
    if err == nil {
        t.Error("expected error for wrong password, got nil")
    }
}
```

- [ ] **Step 2.2: Run tests to verify they fail**

```bash
go test -v -run "TestCreateUserByEmail|TestCreateUserDuplicateEmail|TestAuthenticateUserByEmail|TestAuthenticateUserWrongPassword" ./...
```

Expected: compile error — `CreateUser` still takes username as first arg.

- [ ] **Step 2.3: Remove `Username` from `User` struct in `models.go`**

Replace the `User` struct:

```go
type User struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	RSSToken      string `json:"rss_token"`
	PageSize      int    `json:"page_size"`
}
```

- [ ] **Step 2.4: Update `CreateUser` in `store.go`**

Replace:

```go
func (s *Store) CreateUser(username, password string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	id := fmt.Sprintf("user_%s", generateToken()[:12])
	token := generateToken()
	_, err = s.db.Exec(
		"INSERT INTO users (id, username, password_hash, created_at, rss_token) VALUES (?, ?, ?, ?, ?)",
		id, username, string(hash), time.Now().Format(time.RFC3339), token,
	)
	if err != nil {
		return nil, err
	}
	return &User{ID: id, Username: username, RSSToken: token}, nil
}
```

With:

```go
func (s *Store) CreateUser(email, password string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	id := fmt.Sprintf("user_%s", generateToken()[:12])
	token := generateToken()
	_, err = s.db.Exec(
		"INSERT INTO users (id, email, password_hash, created_at, rss_token) VALUES (?, ?, ?, ?, ?)",
		id, email, string(hash), time.Now().Format(time.RFC3339), token,
	)
	if err != nil {
		return nil, err
	}
	return &User{ID: id, Email: email, RSSToken: token}, nil
}
```

- [ ] **Step 2.5: Update `AuthenticateUser` in `store.go`**

Replace:

```go
func (s *Store) AuthenticateUser(username, password string) (*User, error) {
	var u User
	var hash string
	var emailVerified int
	err := s.db.QueryRow(
		"SELECT id, username, password_hash, email, email_verified, page_size FROM users WHERE username = ?", username,
	).Scan(&u.ID, &u.Username, &hash, &u.Email, &emailVerified, &u.PageSize)
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

With:

```go
func (s *Store) AuthenticateUser(email, password string) (*User, error) {
	var u User
	var hash string
	var emailVerified int
	err := s.db.QueryRow(
		"SELECT id, email, password_hash, email_verified, page_size FROM users WHERE email = ?", email,
	).Scan(&u.ID, &u.Email, &hash, &emailVerified, &u.PageSize)
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

- [ ] **Step 2.6: Update `GetUserByID` in `store.go`**

Replace:

```go
func (s *Store) GetUserByID(userID string) (*User, error) {
	var u User
	var emailVerified int
	err := s.db.QueryRow(
		"SELECT id, username, email, email_verified, COALESCE(rss_token,''), page_size FROM users WHERE id = ?", userID,
	).Scan(&u.ID, &u.Username, &u.Email, &emailVerified, &u.RSSToken, &u.PageSize)
	if err != nil {
		return nil, err
	}
	u.EmailVerified = emailVerified == 1
	return &u, nil
}
```

With:

```go
func (s *Store) GetUserByID(userID string) (*User, error) {
	var u User
	var emailVerified int
	err := s.db.QueryRow(
		"SELECT id, email, email_verified, COALESCE(rss_token,''), page_size FROM users WHERE id = ?", userID,
	).Scan(&u.ID, &u.Email, &emailVerified, &u.RSSToken, &u.PageSize)
	if err != nil {
		return nil, err
	}
	u.EmailVerified = emailVerified == 1
	return &u, nil
}
```

- [ ] **Step 2.7: Update `GetUserByRSSToken` in `store.go`**

Replace:

```go
func (s *Store) GetUserByRSSToken(token string) (*User, error) {
	var u User
	var emailVerified int
	err := s.db.QueryRow(
		"SELECT id, username, email, email_verified, COALESCE(rss_token,''), page_size FROM users WHERE rss_token = ?", token,
	).Scan(&u.ID, &u.Username, &u.Email, &emailVerified, &u.RSSToken, &u.PageSize)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.EmailVerified = emailVerified == 1
	return &u, nil
}
```

With:

```go
func (s *Store) GetUserByRSSToken(token string) (*User, error) {
	var u User
	var emailVerified int
	err := s.db.QueryRow(
		"SELECT id, email, email_verified, COALESCE(rss_token,''), page_size FROM users WHERE rss_token = ?", token,
	).Scan(&u.ID, &u.Email, &emailVerified, &u.RSSToken, &u.PageSize)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.EmailVerified = emailVerified == 1
	return &u, nil
}
```

- [ ] **Step 2.8: Update `GetDigestUsers` in `store.go`**

Find the SELECT and scan inside `GetDigestUsers`. Replace:

```go
	rows, err := s.db.Query(`
		SELECT id, username, email, email_verified, page_size
		FROM users
		WHERE email_digest = 1
		  AND email_verified = 1
		  AND email != ''`)
```

With:

```go
	rows, err := s.db.Query(`
		SELECT id, email, email_verified, page_size
		FROM users
		WHERE email_digest = 1
		  AND email_verified = 1
		  AND email != ''`)
```

And replace the scan inside the loop:

```go
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &emailVerified, &u.PageSize); err != nil {
```

With:

```go
		if err := rows.Scan(&u.ID, &u.Email, &emailVerified, &u.PageSize); err != nil {
```

- [ ] **Step 2.9: Remove `SetUserEmail` from `store.go`**

Delete the entire function:

```go
func (s *Store) SetUserEmail(userID, email string) error {
	_, err := s.db.Exec("UPDATE users SET email = ? WHERE id = ?", email, userID)
	return err
}
```

- [ ] **Step 2.10: Run new tests to verify they pass**

```bash
go test -v -run "TestCreateUserByEmail|TestCreateUserDuplicateEmail|TestAuthenticateUserByEmail|TestAuthenticateUserWrongPassword" ./...
```

Expected: all PASS. (Other tests will fail at compile — fix those in Task 4.)

- [ ] **Step 2.11: Commit**

```bash
git add models.go store.go main_test.go
git commit -m "feat: remove username, use email as identity in User model and store"
```

---

## Task 3: Update handlers

**Files:**
- Modify: `handlers.go`

- [ ] **Step 3.1: Update `handleRegister` in `handlers.go`**

Replace the existing `handleRegister` function:

```go
func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if len(req.Password) < 8 {
		http.Error(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}
	atIdx := strings.Index(req.Email, "@")
	if atIdx < 1 || atIdx == len(req.Email)-1 || !strings.Contains(req.Email[atIdx+1:], ".") {
		http.Error(w, "Invalid email address", http.StatusBadRequest)
		return
	}
	user, err := store.CreateUser(req.Email, req.Password)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			http.Error(w, "Email already registered", http.StatusConflict)
		} else {
			http.Error(w, "Failed to create account", http.StatusInternalServerError)
		}
		return
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

- [ ] **Step 3.2: Update `handleLogin` in `handlers.go`**

Replace the existing `handleLogin` function:

```go
func handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	user, err := store.AuthenticateUser(req.Email, req.Password)
	if err != nil {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}
	sessionID := store.CreateSession(user.ID)
	setSessionCookie(w, sessionID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}
```

- [ ] **Step 3.3: Update `handleFeed` Atom feed title in `handlers.go`**

Find:

```go
	feed := atomFeed{
		XMLNS:   "http://www.w3.org/2005/Atom",
		Title:   user.Username + "'s releases",
```

Replace with:

```go
	feed := atomFeed{
		XMLNS:   "http://www.w3.org/2005/Atom",
		Title:   user.Email + "'s releases",
```

- [ ] **Step 3.4: Run build to check compile errors**

```bash
go build ./... 2>&1
```

Expected: errors in `main_test.go` only (handler and store now compile clean). Proceed to Task 4.

- [ ] **Step 3.5: Commit**

```bash
git add handlers.go
git commit -m "feat: handleRegister/Login use email; feed title uses email"
```

---

## Task 4: Update tests

**Files:**
- Modify: `main_test.go`

- [ ] **Step 4.1: Update `newTestAuth`**

Replace:

```go
func newTestAuth(t *testing.T, s *Store) (string, *http.Cookie) {
	t.Helper()
	user, err := s.CreateUser("testuser", "password123")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	sessionID := s.CreateSession(user.ID)
	cookie := &http.Cookie{Name: "session", Value: sessionID}
	return user.ID, cookie
}
```

With:

```go
func newTestAuth(t *testing.T, s *Store) (string, *http.Cookie) {
	t.Helper()
	user, err := s.CreateUser("testuser@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	sessionID := s.CreateSession(user.ID)
	cookie := &http.Cookie{Name: "session", Value: sessionID}
	return user.ID, cookie
}
```

- [ ] **Step 4.2: Update `TestStoreAddRepoDedupe`**

Find:

```go
	user1, _ := s.CreateUser("user1", "password123")
	user2, _ := s.CreateUser("user2", "password123")
```

Replace with:

```go
	user1, _ := s.CreateUser("user1@example.com", "password123")
	user2, _ := s.CreateUser("user2@example.com", "password123")
```

- [ ] **Step 4.3: Update `TestHandleRegisterWithEmail`**

Find the test body:

```go
	body := `{"username":"newuser","password":"password123","email":"new@example.com"}`
```

Replace with:

```go
	body := `{"password":"password123","email":"new@example.com"}`
```

- [ ] **Step 4.4: Update `TestHandleRegisterMissingEmail`**

Find:

```go
	body := `{"username":"nomail","password":"password123"}`
```

Replace with:

```go
	body := `{"password":"password123"}`
```

- [ ] **Step 4.5: Search and fix any remaining `user.Username` or `CreateUser` with username references**

```bash
grep -n "Username\|\.Username\|\"testuser\"\|\"user1\"\|\"user2\"\|\"user3\"\|SetUserEmail" main_test.go
```

For each hit, update to use email equivalents. Common patterns:
- `user.Username` → `user.Email`
- `CreateUser("digest1", ...)` → `CreateUser("digest1@example.com", ...)`
- `CreateUser("digest2", ...)` → `CreateUser("digest2@example.com", ...)`
- `CreateUser("digest3", ...)` → `CreateUser("digest3@example.com", ...)`
- Any `store.SetUserEmail(...)` calls remain (SetUserEmail is removed — callers must use email at CreateUser time instead; check if any test used SetUserEmail after CreateUser and consolidate)

- [ ] **Step 4.6: Fix `TestGetDigestUsers` if it used `SetUserEmail`**

`TestGetDigestUsers` currently calls `s.SetUserEmail(u.ID, "...")` to set the email after creating the user. Since `SetUserEmail` is removed, set the email at `CreateUser` time instead.

Find in `TestGetDigestUsers`:

```go
	u1, _ := s.CreateUser("digest1", "password123")
	s.SetUserEmail(u1.ID, "digest1@example.com")
	s.db.Exec("UPDATE users SET email_verified = 1 WHERE id = ?", u1.ID)
	s.SetEmailDigest(u1.ID, true)

	u2, _ := s.CreateUser("digest2", "password123")
	s.SetUserEmail(u2.ID, "digest2@example.com")
	s.SetEmailDigest(u2.ID, true)

	u3, _ := s.CreateUser("digest3", "password123")
```

Replace with:

```go
	u1, _ := s.CreateUser("digest1@example.com", "password123")
	s.db.Exec("UPDATE users SET email_verified = 1 WHERE id = ?", u1.ID)
	s.SetEmailDigest(u1.ID, true)

	u2, _ := s.CreateUser("digest2@example.com", "password123")
	s.SetEmailDigest(u2.ID, true)

	u3, _ := s.CreateUser("digest3@example.com", "password123")
```

Apply the same pattern to any other test that called `SetUserEmail` after `CreateUser`.

- [ ] **Step 4.7: Fix tests that checked `user.Username` in response JSON**

Search for assertions against `username` in JSON responses or `currentUser.username` references in handler tests:

```bash
grep -n "username\|Username" main_test.go
```

Update any assertion like `resp["username"]` to `resp["email"]`, and any `user.Username` to `user.Email`.

- [ ] **Step 4.8: Run full test suite**

```bash
go test -v ./...
```

Expected: all tests pass.

- [ ] **Step 4.9: Commit**

```bash
git add main_test.go
git commit -m "test: update all tests to use email instead of username"
```

---

## Task 5: Update frontend (ui.go)

**Files:**
- Modify: `ui.go`

- [ ] **Step 5.1: Update login form — change username input to email input**

Find:

```html
    <form id="login-form" class="auth-form active" onsubmit="doLogin(event)">
        <div class="form-group">
            <label>Username</label>
            <input type="text" name="username" required autocomplete="username">
        </div>
```

Replace with:

```html
    <form id="login-form" class="auth-form active" onsubmit="doLogin(event)">
        <div class="form-group">
            <label>Email</label>
            <input type="email" name="email" required autocomplete="email">
        </div>
```

- [ ] **Step 5.2: Update registration form — remove username field**

Find:

```html
    <form id="register-form" class="auth-form" onsubmit="doRegister(event)">
        <div class="form-group">
            <label>Username <span style="color:#475569">(min 3 chars)</span></label>
            <input type="text" name="username" required minlength="3" autocomplete="username">
        </div>
        <div class="form-group">
            <label>Password <span style="color:#475569">(min 8 chars)</span></label>
            <input type="password" name="password" required minlength="8" autocomplete="new-password">
        </div>
        <div class="form-group">
            <label>Email</label>
            <input type="email" name="email" required autocomplete="email">
        </div>
```

Replace with:

```html
    <form id="register-form" class="auth-form" onsubmit="doRegister(event)">
        <div class="form-group">
            <label>Email</label>
            <input type="email" name="email" required autocomplete="email">
        </div>
        <div class="form-group">
            <label>Password <span style="color:#475569">(min 8 chars)</span></label>
            <input type="password" name="password" required minlength="8" autocomplete="new-password">
        </div>
```

- [ ] **Step 5.3: Update header to show email**

Find:

```js
    document.getElementById('username-display').textContent = currentUser.username;
```

Replace with:

```js
    document.getElementById('username-display').textContent = currentUser.email;
```

- [ ] **Step 5.4: Update account panel to show email**

Find:

```js
            userValue.textContent = currentUser ? currentUser.username : '';
```

Replace with:

```js
            userValue.textContent = currentUser ? currentUser.email : '';
```

- [ ] **Step 5.5: Build to verify no compile errors**

```bash
go build -o newreleases . && echo "BUILD OK"
```

Expected: BUILD OK

- [ ] **Step 5.6: Commit**

```bash
git add ui.go
git commit -m "feat: login/register forms use email; header shows email"
```

---

## Task 6: Update TODO.md

**Files:**
- Modify: `TODO.md`

- [ ] **Step 6.1: Mark item done**

In `TODO.md`, find in `## Planned`:

```
- [ ] **user email as username** - usernames can end up being duplicate.  But email addresses have to be unique. Usernames should probably just be removed completely and only use email addresses.
```

Move to `## Done`, changing `[ ]` to `[x]`.

- [ ] **Step 6.2: Commit**

```bash
git add TODO.md
git commit -m "chore: mark email-as-identity done in TODO"
```
