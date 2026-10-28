# TODO

## In Progress

## Planned
- [x] **Add Send Email Summary** - daily digest email at 7am UTC of previous day's releases; opt-in toggle in account settings panel (⚙ in header)
- [x] **Add immediate email** - to send an when a new release is detected. (user option per project)
- [x] **Main page sort options** - sort by date added, latest release, and alphabetically.
- [x] **Determine if a release is a prerelease or not** - allow the user per project to be notified about prereleases.
- [ ] **Use project icon** - Put the project icon to the left of the project name.  
- [ ] **screen usage** - on the phone it looks great but on the computer there can be more usable space  if the width is computer wide put the add repo to the right of the releases without shrinking the repos space and alway have it open  
- [x] **add commit sha to build info** - need a way to determine which build is being run from the ui
- [x] **user email as username** - usernames can end up being duplicate.  But email addresses have to be unique. Usernames should probably just be removed completely and only use email addresses.

## Done

- [x] **Fetcher response size caps** — all platform fetchers read through a capped `fetch()` (16 MiB; 64 MiB for npm packuments and Helm `index.yaml`, vs ~16 MB / ~7 MB largest seen), so a hostile or broken upstream can't exhaust memory
- [x] **Migration table** — the 13 hand-written blocks in `store.go` became a `{version, stmts}` table in `migrations.go`; each step is transactional (a failure no longer leaves a half-applied column with the version unbumped); schema verified identical to the old code from every starting version
- [x] **Prerelease handling** — releases are flagged as prereleases (GitHub/Artifact Hub API flags, GitLab `upcoming_release`, a version/tag heuristic for npm, PyPI, Docker and Helm repos); per-project "Include prereleases" toggle (default off) controls whether they appear in the list, feed and digest and whether push, immediate email and webhooks fire; shown with a `pre` badge when included
- [x] **Build info in UI** — short commit SHA shown bottom-right on every page and logged at startup; injected via `GIT_SHA` build-arg → `-ldflags -X main.version` in the Dockerfile (build.sh, build-podman.sh, GitLab CI, GitHub Actions and docker-compose pass it); plain `go build` falls back to the embedded VCS revision
- [x] **Frontend out of Go strings** — `ui.go` split into `web/index.html`, `web/app.css`, `web/app.js`, embedded with `go:embed`; byte-identical to the old inline page, served at `/` and `/static/`
- [x] **Project sort options** — Sort selector on the main page (latest release, date added, name A–Z); applied before pagination, remembered per browser, default latest release
- [x] **Security hardening** — webhook SSRF guard, `Secure` session cookie behind TLS/`X-Forwarded-Proto`, HTTP server/client timeouts, in-memory login/register rate limits, `.gitignore` for local artifacts
- [x] **UI hardening** — AA contrast for muted text, keyboard focus rings, aria labels, touch-visible project controls, 16px inputs on mobile, long-text truncation, session-expiry handling and retry on load failure; added PRODUCT.md / DESIGN.md design context
- [x] **Immediate email on new release** — plain-text email sent the moment a new release is detected (opt-in per project per user, gated on SMTP configured + verified email); toggle in the project settings panel alongside push notifications
- [x] **GitLab backup badge on Releases tab** — the `[BACKUP]` link to the mirrored GitLab project (already shown on Projects-tab cards) now also shows on the Releases-tab project header, for consistency across both views
- [x] **Incorporate syncrepos project** — per-user GitLab instance URL + API token (Account Settings panel); per-project "Enable GitLab sync" with daily/weekly/monthly frequency (project settings panel), ported GitLab REST client + ephemeral mirror clone/push from `syncrepos`; optional auto-generated "Awesome" README (grouped by platform) pushed to a GitLab project under the user's namespace, implementing syncrepos's own unfinished TODO item
- [x] **Pagenate releases and projects** - Allow the user to set the number of projects per page between 5,10 and 20.  Pagenate beyond that number.
- [x] **User accounts with auth** — register/login/logout, session cookies, per-user project tracking
- [x] **Shared release data** — deduplicated repos table; two users adding the same project share one release record
- [x] **Smart URL autofill** — platform-aware input: `owner/repo` for GitHub/GitLab, package name for npm/PyPI, image name for Docker; auto-fills display name
- [x] **Other / Custom URL** — paste any supported URL; platform auto-detected with live feedback, name auto-filled
- [x] **Browser push notifications** — VAPID keys auto-generated and persisted; service worker at `/sw.js`; 🔔 button in header; notifies all subscribed users when a new release is detected on refresh
- [x] **Webhooks** — per-project webhook URLs with optional HMAC-SHA256 secret; managed inline from the Projects tab; fired alongside push notifications on new releases
- [x] **Add Project side panel** — Add Project form in a slide-in panel on the right side of the Releases page (Projects tab eliminated)
- [x] **Inline project controls** — Refresh and Delete buttons directly on project headers in Releases view; settings panel for webhooks, push notifications, email
- [x] **Email validation of users** — email collected on registration, verification link sent via SMTP; unverified users see banner with resend option; email features gated behind verification
- [x] **Disable email if not configured** — verification banner hidden when SMTP env vars not set; `smtp_enabled` exposed on `/api/me`
- [x] **Email case insensitive** — email address lowercased at registration
- [x] **Limit releases per project** — inline chip line shows 5 most recent (age: version); +N more ▾ expands full stacked list
- [x] Split monolith `main.go` into focused files (`handlers.go`, `store.go`, `fetchers.go`, `models.go`, `ui.go`)
- [x] Migrate state storage from JSON/YAML to SQLite (WAL mode, `modernc.org/sqlite`)
- [x] Fix delete bug (was re-seeding on restart, wiping unrelated projects)
- [x] Rewrite UI from scratch (no blocking alerts, toast notifications, reliable DOM updates)
- [x] Add Kubernetes manifests with PVC for data persistence (`k8s/newreleases.yaml`)
- [x] Docker multi-stage build (non-root user, healthcheck, pinned base image)
- [x] GitHub Actions + GitLab CI pipelines (build, test, push, Trivy scan)
