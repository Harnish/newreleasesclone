# Email as Identity Design

**Date:** 2026-04-09  
**Status:** Approved

## Overview

Replace the arbitrary `username` field with `email` as the sole user identity and login credential. Usernames are removed entirely from the schema, code, and UI.

## Pre-Deploy Requirement

Before deploying the v10 migration, manually ensure every existing user row has a non-empty, unique email address. The migration will fail with a constraint error if any row has a blank or duplicate email, making it safe — it will not silently corrupt data.

## Database Migration (v9 → v10)

SQLite does not support `DROP COLUMN` or `ADD CONSTRAINT` on existing tables. The migration uses a table-rebuild pattern:

1. Create `users_new` with the target schema: `email TEXT NOT NULL UNIQUE` instead of `username TEXT NOT NULL UNIQUE`, no `username` column.
2. `INSERT INTO users_new SELECT id, email, password_hash, created_at, email_verified, email_digest, rss_token, page_size FROM users` — mapping email to the new primary identifier column.
3. `DROP TABLE users`.
4. `ALTER TABLE users_new RENAME TO users`.
5. Recreate dependent indexes (e.g. `idx_users_rss_token`).
6. `PRAGMA user_version = 10`.

Foreign key references from `sessions`, `user_repos`, `webhooks`, `push_subscriptions`, and `email_verification_tokens` all reference `users(id)` — the `id` column is unchanged, so no cascade issues.

## Backend Changes

### models.go

Remove `Username string` from `User` struct. The struct becomes:

```go
type User struct {
    ID            string `json:"id"`
    Email         string `json:"email"`
    EmailVerified bool   `json:"email_verified"`
    RSSToken      string `json:"rss_token"`
    PageSize      int    `json:"page_size"`
}
```

### store.go

- `CreateUser(username, password string)` → `CreateUser(email, password string)`: inserts into the `email` column; no `username` column.
- `AuthenticateUser(username, password string)` → `AuthenticateUser(email, password string)`: queries `WHERE email = ?`.
- `GetUserByID`: remove `username` from SELECT and Scan.
- `GetUserByRSSToken`: remove `username` from SELECT and Scan.
- `AuthenticateUser`: remove `username` from SELECT and Scan.
- `GetDigestUsers`: remove `username` from SELECT and Scan.
- `SetUserEmail`: becomes a no-op or is removed — email is now set at registration, not separately. Remove it.

### handlers.go

- `handleRegister`: remove `Username` from request struct and validation. Remove the "Username must be at least 3 characters" check. Change `UNIQUE constraint failed` error message to "Email already registered". Call `store.CreateUser(req.Email, req.Password)` — no separate `store.SetUserEmail` call needed.
- `handleLogin`: change request struct field `Username string` → `Email string`. Pass `req.Email` to `AuthenticateUser`.
- `handleFeed`: change Atom feed title from `user.Username + "'s releases"` to `user.Email + "'s releases"`.

### handlers_test / main_test.go

- `newTestAuth` currently calls `store.CreateUser("testuser", "password123")`. Change to `store.CreateUser("test@example.com", "password123")`.
- Update any test that references `user.Username` or checks for "Username already taken" error.

## Frontend Changes (ui.go)

### Login form

Change the username input to an email input:

```html
<label>Email</label>
<input type="email" name="email" required autocomplete="email">
```

### Registration form

Remove the username field entirely (the label + input for username). The form collects only `email` and `password`.

### Header display

```js
document.getElementById('username-display').textContent = currentUser.email;
```

### Error message

The login error toast/message "Invalid username or password" → "Invalid email or password".

## Error Handling

- Registration with duplicate email: 409 "Email already registered"
- Registration with invalid email format: 400 "Invalid email address" (existing check unchanged)
- Login with unknown email or wrong password: 401 "Invalid email or password"
- Migration fails if any user has blank/duplicate email: operator must fix data before redeploying

## Testing

- `TestCreateUser`: create user with email, verify no username field
- `TestAuthenticateUser`: authenticate with email + password
- `TestDuplicateEmail`: second registration with same email returns 409
- `TestHandleRegister`: no username in request body required
- `TestHandleLogin`: login with email field
- Update `newTestAuth` to use email
- All existing tests updated to remove username references
