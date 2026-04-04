# Design: Immediate Email on New Release

**Date:** 2026-04-04  
**Status:** Approved

## Summary

Send an email immediately when a new release is detected for a tracked project. Opt-in per project per user. Email includes the project name, version, URL, and release notes. Gated behind email verification and SMTP configuration.

## Goals

- Notify opted-in users by email as soon as a new release is detected during a refresh
- Per-project toggle in the existing project settings panel
- One email per release (not bundled)

## Non-Goals

- Bundling multiple simultaneous releases into one email
- HTML email (plain text only)
- Global on/off toggle (per-project only)

## Schema Changes

Schema bumps from v7 to v8. Single migration:

```sql
ALTER TABLE user_repos ADD COLUMN email_immediate INTEGER NOT NULL DEFAULT 0;
```

Applied idempotently in `migrate()` on startup.

## Model Changes (`models.go`)

Add `EmailImmediate bool` to `Project` struct:

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

## Store Changes (`store.go`)

### `SetProjectEmailImmediate(userID, repoID string, enabled bool) (bool, error)`

```sql
UPDATE user_repos SET email_immediate = ? WHERE user_id = ? AND repo_id = ?
```

Returns `(true, nil)` if a row was updated, `(false, nil)` if no row matched. Same pattern as `SetProjectPushEnabled`.

### `getImmediateEmailUsersForRepo(repoID string) []User`

```sql
SELECT u.id, u.username, u.email, u.email_verified
FROM users u
INNER JOIN user_repos ur ON ur.user_id = u.id
WHERE ur.repo_id = ?
  AND ur.email_immediate = 1
  AND u.email_verified = 1
  AND u.email != ''
```

Returns users who should receive immediate email notifications for this repo. Unexported — used only within `sendImmediateEmails`.

### `sendImmediateEmails(repoID string, project Project, newReleases []Release)`

For each release in `newReleases`, calls `getImmediateEmailUsersForRepo(repoID)` and calls `smtpCfg.SendReleaseEmail(u.Email, project, release)` for each user. Logs errors per user but continues. Only called when `smtpEnabled` is true.

### `GetUserRepos` update

Add `ur.email_immediate` to the SELECT and scan it into `p.EmailImmediate`.

### `RefreshProject` update

After the existing goroutine dispatches (lines 984-985), add:

```go
if smtpEnabled {
    go s.sendImmediateEmails(repoID, project, newReleases)
}
```

## Email Changes (`email.go`)

### `buildReleaseEmailBody(project Project, release Release) string`

Plain-text body:
```
<project.Name> <release.Version> has been released.

<release.URL>

<release.ReleaseNotes>
```

The release notes block is omitted (no trailing newline) if `release.ReleaseNotes == ""`.

### `SendReleaseEmail(to string, project Project, release Release) error`

Subject: `"<project.Name> <release.Version> released"`

Calls `SendMail(to, subject, buildReleaseEmailBody(project, release))`.

## API Changes (`handlers.go`)

### `POST /api/project-settings`

Extend the existing handler to also accept `email_immediate bool`:

```go
var req struct {
    RepoID         string `json:"repo_id"`
    PushEnabled    bool   `json:"push_enabled"`
    EmailImmediate bool   `json:"email_immediate"`
}
```

When `smtpEnabled` is true, calls `store.SetProjectEmailImmediate(userID, req.RepoID, req.EmailImmediate)` after the existing `SetProjectPushEnabled` call. No change in response format — still returns `{"status": "ok"}`.

`email_immediate` is silently ignored when `smtpEnabled` is false (DB not touched).

## Frontend Changes (`ui.go`)

### Project settings panel

In `renderSettingsPanelBody` (the JS function that builds per-project settings panel HTML), add an "Immediate email" toggle row below the existing "Push notifications" row.

The toggle is:
- Disabled with `title="Email not configured"` when `!currentUser.smtp_enabled`
- Disabled with `title="Verify your email to enable"` when `!currentUser.email_verified`
- Otherwise enabled; toggling POSTs `{repo_id, push_enabled: <current>, email_immediate: <new>}` to `/api/project-settings`

The `openSettingsPanel(btn)` already passes `data-push` on the button. Add `data-email-immediate` attribute to the settings button in `buildReleasesHTML`, populated from `proj.email_immediate`.

The POST to `/api/project-settings` must include both `push_enabled` and `email_immediate` since the handler currently applies both. Read the current push state from the DOM (the push checkbox value) when submitting the email toggle, and vice versa.

## Error Handling

- SMTP send failure per user per release: log warning, continue.
- `getImmediateEmailUsersForRepo` failure: log error, skip the send for that release.
- `smtpEnabled = false`: goroutine never dispatched (guard in `RefreshProject`).

## Testing

- Store: `SetProjectEmailImmediate` (enable, disable, unknown repo), `GetUserRepos` returns `email_immediate` field
- `buildReleaseEmailBody`: subject format, body with notes, body without notes
- Handler: `POST /api/project-settings` with `email_immediate` field persists correctly
- `GetUserRepos` scan includes new column
