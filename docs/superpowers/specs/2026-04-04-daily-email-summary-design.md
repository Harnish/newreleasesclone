# Design: Daily Email Summary

**Date:** 2026-04-04  
**Status:** Approved

## Summary

Send a daily email at 7:00 UTC to opted-in users listing all releases published the previous calendar day across their tracked projects. Only sent when there is at least one release to report. Gated behind email verification and SMTP configuration.

## Goals

- Send one email per day per user with yesterday's release activity
- Opt-in toggle in a new account settings panel
- No new dependencies (pure stdlib scheduler)

## Non-Goals

- Per-user timezone support (UTC for all users)
- HTML email (plain text only)
- Immediate per-release email (separate future feature)
- Email if no releases were published yesterday

## Schema Changes

Schema bumps from v6 to v7. Single migration:

```sql
ALTER TABLE users ADD COLUMN email_digest INTEGER NOT NULL DEFAULT 0;
```

Applied idempotently in `migrate()` on startup.

## New File: `scheduler.go`

Single responsibility: background scheduler for timed jobs.

```go
func runDailyDigest()
func sendDailyDigestToAll()
func nextSevenAMUTC() time.Duration
```

**`nextSevenAMUTC()`** — returns duration from now until the next 7:00:00 UTC. If current time is before 7am UTC, waits until today's 7am; if after, waits until tomorrow's 7am.

**`runDailyDigest()`** — sleeps until next 7am UTC, calls `sendDailyDigestToAll()`, then loops every 24 hours using `time.NewTicker`.

**`sendDailyDigestToAll()`** — calls `store.GetDigestUsers()`, iterates users, calls `store.GetReleasesPublishedOn(userID, yesterday)` for each, skips if empty, calls `smtpCfg.SendDailySummary(user.Email, releases)` for non-empty results. Logs errors per user but continues.

### `main.go` change

After store and SMTP init:

```go
if smtpEnabled {
    go runDailyDigest()
}
```

## Store Changes (`store.go`)

### `GetDigestUsers() []User`

```sql
SELECT id, username, email, email_verified
FROM users
WHERE email_digest = 1
  AND email_verified = 1
  AND email != ''
```

Returns users who should receive the digest.

### `GetReleasesPublishedOn(userID string, date time.Time) []Release`

```sql
SELECT r.id, r.repo_id, r.name, r.version, r.platform, r.url,
       r.published_at, r.description, r.release_notes
FROM releases r
INNER JOIN user_repos ur ON ur.repo_id = r.repo_id
WHERE ur.user_id = ?
  AND r.published_at >= ?
  AND r.published_at < ?
ORDER BY r.name ASC, r.published_at DESC
```

Date range: `date` truncated to midnight UTC through the next midnight UTC. Returns releases the user's tracked projects published on that calendar day.

### `SetEmailDigest(userID string, enabled bool) error`

```sql
UPDATE users SET email_digest = ? WHERE id = ?
```

### `GetUserDigestEnabled(userID string) (bool, error)`

```sql
SELECT email_digest FROM users WHERE id = ?
```

## Email Changes (`email.go`)

### `SendDailySummary(to string, releases []Release) error`

Plain-text email. Subject: `"Your daily release summary"`.

Body format:
```
Here are the releases from yesterday:

[ProjectName] v1.2.3 — https://github.com/...
[ProjectName] v2.0.0 — https://github.com/...
[AnotherProject] v0.9.1 — https://pypi.org/...
```

Sorted alphabetically by project name. Uses existing `SendMail`.

## API Changes (`handlers.go`)

### `GET /api/account-settings` (requires auth)

Returns:
```json
{
  "email_digest": true
}
```

### `POST /api/account-settings` (requires auth)

Body:
```json
{
  "email_digest": true
}
```

Returns `{"status": "ok"}`. Does not enforce verification server-side (UI handles gating). Returns 400 if body is invalid, 404 if user not found.

Handler: `handleAccountSettings(w, r, userID)` dispatches on method.

Route registered in `main.go`:
```go
http.HandleFunc("/api/account-settings", requireAuth(handleAccountSettings))
```

## Frontend Changes (`ui.go`)

### Account settings panel

Same slide-over pattern as Add Project panel. Triggered by clicking the username in the header nav.

**Trigger:** Username element in header becomes a clickable button (styled as a link) that opens the panel.

**Panel contents:**
- Username (read-only text)
- "Daily email summary" toggle (`<input type="checkbox">`)
  - Disabled with title `"Verify your email to enable"` when `email_verified === false`
  - Disabled with title `"Email not configured"` when `smtp_enabled === false`
  - Otherwise enabled; toggling POSTs to `/api/account-settings`

**State loading:** When panel opens, `GET /api/account-settings` populates the toggle. On toggle change, `POST /api/account-settings` fires immediately (no save button).

**Toast on save:** Brief "Settings saved" toast on success; "Failed to save settings" on error.

## Error Handling

- SMTP send failure per user: log warning, continue to next user. No retry.
- `GetDigestUsers` failure: log error, skip the entire run.
- `GetReleasesPublishedOn` failure per user: log warning, skip that user.
- User with empty or unverified email filtered at query level (not in-code).

## Testing

- Store: `SetEmailDigest`, `GetUserDigestEnabled`, `GetReleasesPublishedOn` (correct date range, cross-user isolation)
- `nextSevenAMUTC()`: verify returns value in (0, 24h]
- `SendDailySummary`: verify subject/body format (no live SMTP)
- Handler: `GET /api/account-settings` returns correct value; `POST` persists change
