# Design: Email Validation of Users

**Date:** 2026-04-03  
**Status:** Approved

## Summary

Add email address collection and verification to the registration flow. Unverified users can log in and use all features except email notifications. A verification link is sent via SMTP on registration, with a resend option.

## Goals

- Collect and store a verified email address per user
- Gate email notification features (daily digest, immediate emails) behind verification
- Send verification emails via SMTP configured through environment variables

## Non-Goals

- Blocking login for unverified users
- Enforcing verification on non-email endpoints
- Email change after registration (future feature)

## Schema Changes

### `users` table

Add two columns via migration:

```sql
ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN email_verified INTEGER NOT NULL DEFAULT 0;
```

Schema version bumped to v3. `migrate()` applies these columns idempotently on startup.

### New `email_verification_tokens` table

```sql
CREATE TABLE IF NOT EXISTS email_verification_tokens (
    token      TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TEXT NOT NULL
);
```

- `token`: 32-char lowercase hex string
- `expires_at`: RFC3339, 24 hours from creation
- Old tokens for a user are deleted when a new one is created (resend)

## SMTP Configuration

Read from environment variables at startup. If any required variable is missing, email sending is disabled (logged as a warning); the app still starts normally.

| Env Var | Description | Required |
|---|---|---|
| `SMTP_HOST` | SMTP server hostname | Yes |
| `SMTP_PORT` | SMTP port (default: 587) | No |
| `SMTP_USER` | SMTP username | Yes |
| `SMTP_PASS` | SMTP password | Yes |
| `SMTP_FROM` | From address (e.g. `noreply@example.com`) | Yes |

## New File: `email.go`

Single responsibility: SMTP email sending.

```go
type SMTPConfig struct {
    Host string
    Port int
    User string
    Pass string
    From string
}

func SMTPConfigFromEnv() (SMTPConfig, bool)
func (c SMTPConfig) SendMail(to, subject, body string) error
func (c SMTPConfig) SendVerificationEmail(to, token, baseURL string) error
```

`SendVerificationEmail` composes a plain-text email with the verification link:
`<baseURL>/verify-email?token=<token>`

`baseURL` is derived from the incoming request's `Host` header + scheme.

## Store Changes (`store.go`)

New methods:

```go
func (s *Store) SetUserEmail(userID, email string) error
func (s *Store) CreateVerificationToken(userID string) (token string, err error)
func (s *Store) VerifyEmailToken(token string) (userID string, err error)
// VerifyEmailToken: looks up token, checks expiry, marks user verified,
// deletes token. Returns ErrTokenExpired or ErrTokenNotFound on failure.
func (s *Store) DeleteVerificationTokensForUser(userID string) error
func (s *Store) GetUserByID(userID string) (User, error)
```

`CreateVerificationToken` deletes any existing tokens for the user before inserting the new one.

## Model Changes (`models.go`)

Add fields to `User`:

```go
type User struct {
    ID            string `json:"id"`
    Username      string `json:"username"`
    Email         string `json:"email"`
    EmailVerified bool   `json:"email_verified"`
}
```

## API Changes (`handlers.go`)

### `POST /api/register`

- Accept `email` field (required, basic format validation: contains `@`)
- Store email via `SetUserEmail`
- Create verification token, send verification email (non-blocking — log error if send fails, don't fail registration)
- Response unchanged: HTTP 201, `{id, username}`

### `GET /verify-email?token=xxx` (no auth required)

- Call `VerifyEmailToken(token)`
- On success: redirect to `/?verified=1`
- On expired/invalid: redirect to `/?verify_error=1`
- Frontend shows a toast based on query param on page load

### `POST /api/resend-verification` (requires auth)

- Deletes old tokens, creates new token, sends new email
- Returns HTTP 200 `{ok: true}` or HTTP 429 if called more than 3 times in 10 minutes (simple in-memory rate limit per user ID; resets on server restart — acceptable for single-instance deployment)
- Returns HTTP 400 if user is already verified

### `GET /api/me`

- Returns `email` and `email_verified` fields (already on `User` struct)

## Frontend Changes (`ui.go`)

### Registration form

- Add `email` input field (type=email, required) below username

### Verification banner

- After login, if `me.email_verified === false`, show a dismissible yellow banner:
  > "Please verify your email address. [Resend verification email]"
- Clicking "Resend" calls `POST /api/resend-verification` and shows a toast

### Email feature gating

- When `email_verified` is false, email-related settings (daily digest toggle, immediate email toggle) are shown as disabled with a tooltip: "Verify your email to enable"

### Toast on verification

- On page load, if `?verified=1` in URL: show green toast "Email verified!"
- If `?verify_error=1`: show red toast "Verification link expired or invalid. Please request a new one."

## Error Handling

- SMTP send failure on registration: log warning, do not fail registration. User can resend.
- Token not found: generic "invalid or expired" message (no user enumeration)
- Missing SMTP config: app starts normally, email sending silently disabled, warning logged

## Testing

- Store tests: `CreateVerificationToken`, `VerifyEmailToken` (success, expired, not found), `DeleteVerificationTokensForUser`
- Handler tests: register with email, verify endpoint (success + expired), resend endpoint
- `email.go`: `SMTPConfigFromEnv` unit test with env var mocking; `SendMail` not tested (requires live SMTP)
