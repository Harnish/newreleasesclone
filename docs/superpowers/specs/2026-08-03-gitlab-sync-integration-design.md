# GitLab Sync Integration Design

Date: 2026-08-03
Status: Approved (pending spec review)

## Summary

Port the core mirroring capability of the sibling project `syncrepos`
(`/home/jharnish/Work/syncrepos/syncrepos`) into `newreleases`, so a
newreleases user can optionally mirror a tracked GitHub/upstream repo to
their own GitLab instance on a daily/weekly/monthly schedule, and optionally
maintain an auto-generated "Awesome" list README of their synced projects as
a GitLab project under their namespace. This also implements the one
remaining unchecked item in syncrepos's own TODO.md (the Awesome-Clones page),
inside newreleases instead of syncrepos.

This is newreleases's TODO.md item: "Incorporate syncrepos project."

## Non-goals (explicitly out of scope for this pass)

- No CLI. This is a web-app-only feature, unlike syncrepos which is
  cobra-CLI-first with `serve` as one subcommand among several.
- No persistent local git clones, no `REPOS_DIR` volume/PVC.
- No drift detection / `repair` / `report changed|gone` / `scan` commands.
  Those exist in syncrepos to manage problems (stale local clones, orphaned
  directories) created by its persistent-clone model. Since this design uses
  ephemeral clones, that whole problem class doesn't arise.
- No multi-forge support beyond GitLab (TODO.md mentions future
  github/gitea targets — not this pass).
- No encryption at rest for the stored GitLab API token (matches existing
  webhook-secret storage pattern in this codebase).

## Source material being reused

From `/home/jharnish/Work/syncrepos/syncrepos`:

- `internal/gitlab/client.go` — hand-rolled GitLab REST v4 client
  (`PRIVATE-TOKEN` header auth, `GroupExists`, `EnsureGroupPath`,
  `ProjectExists`, `GetProjectHTTPURL`, `CreateProject`,
  `AuthenticatedPushURL`). Stdlib-only, already takes `baseURL`+`token` as
  constructor args, so per-instance/per-user use requires no changes to its
  shape — just instantiate one `Client` per user instead of one globally.
- `internal/gitops/gitops.go` — `Clone`, `PushMirror` (mirror-push mapping
  `refs/remotes/origin/*` → `refs/heads/*`), `redactArgs` (scrubs credentials
  from logged git command output). The persistent-clone-specific pieces
  (`AddRemote`, `FetchOrigin`, `HeadSHA`, `CheckRedirect`) are not needed
  under the ephemeral-clone model.
- The Awesome-list feature itself does not exist yet in syncrepos (confirmed
  via grep — only present as an unchecked TODO bullet), so `awesome.go` is
  new code, not a port, though it follows the grouping idea described in
  syncrepos's TODO.md.

## Architecture

Three new files, staying in `package main` per newreleases's existing flat
layout (no `internal/` package split — that structure belongs to syncrepos's
CLI-first design and isn't reflected in newreleases's current codebase):

1. **`gitlabclient.go`** — the ported GitLab REST client described above.
2. **`gitmirror.go`** — ephemeral mirror-sync: `git clone --mirror` to a
   `os.MkdirTemp` directory, push to the GitLab remote via
   `AuthenticatedPushURL`, then `os.RemoveAll` the temp directory
   regardless of outcome. Every sync is a full mirror clone+push; there is
   no incremental fetch and no local state carried between syncs.
3. **`awesome.go`** — builds a README.md grouped by `Project.Platform`
   (github/npm/pypi/docker/gitlab) from a user's GitLab-sync-enabled
   projects, and pushes it via the same client+mirror primitives (the
   Awesome project is just another GitLab project being mirror-pushed to,
   with generated content instead of an upstream clone).

Scheduling reuses newreleases's existing staleness-check pattern rather than
introducing `robfig/cron` (which syncrepos uses) or any other new dependency.
The existing `GET /api/refresh-check` mechanism compares
`last_refresh` against a fixed 30-minute threshold and fires background
`RefreshProject` goroutines for stale projects. This design adds an
analogous check: a project with `gitlab_sync_enabled=1` is "due" when
`now - last_gitlab_sync_at > frequencyDuration(gitlab_sync_frequency)`
(daily=24h, weekly=7d, monthly=30d — calendar-accurate month math is
unnecessary here). This check piggybacks on the same periodic tick that
already drives refresh-check, rather than adding a second ticker/goroutine
loop.

## Data model

New table, one row per user:

```sql
CREATE TABLE gitlab_settings (
  user_id INTEGER PRIMARY KEY REFERENCES users(id),
  gitlab_url TEXT NOT NULL,
  gitlab_token TEXT NOT NULL,
  awesome_enabled INTEGER NOT NULL DEFAULT 0,
  awesome_repo_name TEXT,
  awesome_gitlab_path TEXT,   -- populated once the GitLab project exists
  updated_at TEXT NOT NULL
);
```

New columns on the existing `projects` table:

```sql
ALTER TABLE projects ADD COLUMN gitlab_sync_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE projects ADD COLUMN gitlab_sync_frequency TEXT;        -- 'daily'|'weekly'|'monthly'
ALTER TABLE projects ADD COLUMN gitlab_project_path TEXT;          -- populated once the mirror project exists
ALTER TABLE projects ADD COLUMN last_gitlab_sync_at TEXT;
ALTER TABLE projects ADD COLUMN last_gitlab_sync_error TEXT;
```

`gitlab_token` is stored in plaintext in sqlite, matching how this codebase
already stores webhook HMAC secrets. It is a more powerful credential than a
webhook secret (write access to the user's GitLab account), but encrypting
it is explicitly deferred — noted here so it isn't forgotten, not because
it's considered a non-issue.

## UI / flow

**Settings panel** (existing per-user ⚙ panel in the header): new "GitLab
Sync" section with GitLab URL + API token fields (save → upsert
`gitlab_settings`). Below that, an "Generate Awesome page" toggle: turning it
on prompts for a repo name, then synchronously creates the GitLab project
(`EnsureGroupPath` + `CreateProject` under the user's own namespace) and
performs an initial push of the generated README.

**Project three-dot menu** (existing per-project menu in the Releases view):

- When `gitlab_sync_enabled=0`: "Enable GitLab sync" → inline frequency
  dropdown (daily/weekly/monthly) → on confirm, creates the mirror GitLab
  project and performs the first sync immediately.
- When `gitlab_sync_enabled=1`: "Sync now" (manual trigger, same code path
  as the scheduled sync) and "Disable GitLab sync" (flips the flag off only
  — the GitLab-side project is left alone, not deleted).

**Sync flow** (scheduled or manual, same function): ephemeral mirror-clone →
push → on success, update `last_gitlab_sync_at`, clear
`last_gitlab_sync_error`, then regenerate+push the user's Awesome README if
`awesome_enabled`. On failure, store the error in
`last_gitlab_sync_error` and surface it next to the project using the same
UI pattern already used for release-fetch errors.

## Error handling

All sync operations run in background goroutines (matching
`RefreshProject`'s fire-and-forget discipline) and must never panic the
process on a bad token, unreachable GitLab instance, or git failure. Errors
are caught, logged, and written to `last_gitlab_sync_error` for display.

## Deployment

`git` must be present in the final container image. The current
`Dockerfile`'s final stage (`FROM alpine:latest`) only installs
`ca-certificates` — it needs `RUN apk --no-cache add git` added alongside
that. (The builder stage already has git, but that's a separate, discarded
layer.) No PVC changes are needed: ephemeral clones live under
`os.TempDir()` and are removed after each sync, so disk usage does not grow
over time the way syncrepos's persistent `REPOS_DIR` does.

## Testing

- Port/adapt the existing table-driven tests from
  `syncrepos/internal/gitlab` and `syncrepos/internal/gitops` for the copied
  client/mirror code.
- Add store tests for the new schema and columns, following the existing
  `TestStoreAddProject`-style patterns already in `main_test.go`.
- A full end-to-end mirror-and-push against a real GitLab instance isn't
  practical to automate in CI; this is called out as a manual verification
  step in the implementation plan rather than an automated test.
