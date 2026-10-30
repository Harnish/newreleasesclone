# TODO

## In Progress

## Planned
- [x] **Add Send Email Summary** - daily digest email at 7am UTC of previous day's releases; opt-in toggle in account settings panel (⚙ in header)
- [x] **Add immediate email** - to send an when a new release is detected. (user option per project)
- [x] **Main page sort options** - sort by date added, latest release, and alphabetically.
- [x] **Determine if a release is a prerelease or not** - allow the user per project to be notified about prereleases.
- [x] **Use project icon** - Put the project icon to the left of the project name.  
- [x] **add commit sha to build info** - need a way to determine which build is being run from the ui
- [x] **user email as username** - usernames can end up being duplicate.  But email addresses have to be unique. Usernames should probably just be removed completely and only use email addresses.

## Done

- [x] **One GitLab sync per project at a time** — enabling sync, "sync now" and the scheduler each launched their own sync with no guard, so two overlapping runs both saw "no mirror project yet" and raced to create it (one got a 400 even though GitLab had created the project); a second sync for a project already syncing is now skipped
- [x] **GitLab "project already exists" 400** — with no group set, the mirror/Awesome project lookup used a bare name, which GitLab's API always answers 404 for, so an existing project was re-created and rejected with a 400. Lookups now use `<token owner>/<name>` (via `GET /user`), so an existing project is adopted; any failed GitLab call now reports GitLab's own reason (e.g. `path: has already been taken`)
- [x] **GitLab token-expired banner** — a 401 from GitLab (or git rejecting the token on push) flags the user's token (`gitlab_settings.token_invalid`, migration 17); a red banner at the top of the app links to `<their GitLab>/-/user_settings/personal_access_tokens?name=newreleases&scopes=api` and to Account settings. Cleared by saving a new token or by a sync that succeeds. A 403 does not flag it (that can be one project's permissions). Shows after the next sync attempt, not instantly
- [x] **Favicon** — `web/favicon.svg` (white up-arrow on a blue rounded square, matching the UI's blue), linked from `index.html` and served at `/static/favicon.svg`
- [x] **Project description in popup** — each repo's upstream description (GitHub, GitLab, npm, PyPI, Docker Hub, Artifact Hub, Helm repo index) is looked up in the background after a refresh, stored in `repos.description` (migration 16; capped at 500 chars) and shown at the top of the project popup. Re-checked at most daily so GitHub's 60/hour unauthenticated limit isn't burned; a failed lookup keeps the old text. Existing projects fill in on their next refresh
- [x] **Project detail popup** — clicking a project name opens a dialog with its last 10 versions (date, `pre` badge, release notes, link to the release); built from data already loaded in the page. descriptions came later (see above)
- [x] **Wide-screen layout** — at ≥1100px the Add Project form is a permanent sticky column to the right of the project list (list width and Account/project settings overlays unchanged); below that it stays a slide-over
- [x] **Helm repo prereleases** — `fetchHelmRepo` dropped every version containing `-` before the prerelease logic ran, so the include-prereleases toggle did nothing for Helm repos; it now keeps the newest 10 stable + 5 prereleases by publish date (SemVer `-suffix` counts as prerelease) and skips undated entries
- [x] **Icon downloads can't reach allowlisted hosts** — icon URLs come from third-party Helm indexes, so they use a strict client that ignores `GITLAB_ALLOWED_HOSTS`, except for the Helm repo's own host
- [x] **Split `main_test.go`** — 2,129 lines into `testhelpers_test.go`, `store_test.go`, `handlers_projects_test.go`, `email_test.go`, `account_test.go`, `feed_test.go`, `auth_test.go`, `migrations_v10_test.go` and `fetchers_test.go`; identical set of 146 tests/benchmarks
- [x] **Project icons** — shown left of each project name. Fetched server-side (GitHub owner avatar, GitLab project avatar, Artifact Hub logo, Helm chart `icon`), validated (PNG/JPEG/GIF/WebP/SVG, 512 KB cap, SSRF-guarded client), cached in `repo_icons` and served from `/api/icon` so browsers never contact third-party hosts; npm/PyPI/Docker and any failed lookup get a letter badge in the platform colour. Fetchers now send an identifying User-Agent
- [x] **Fix npm/PyPI version window** — both fetchers sorted version strings as text (so `9.0.0` outranked `10.0.0`) and could keep old versions while dropping new ones; they now keep the newest 10 stable + 5 prereleases by publish time (`newestReleases`), skip versions with no publish date instead of stamping them "now", and `experimental` builds count as prereleases (React's `0.0.0-experimental-*` channel was filling the whole window)
- [x] **Helm repo SSRF guard** — Helm repo index fetches now use the dial-time-guarded transport (private/loopback/metadata addresses refused, https-only redirects, `GITLAB_ALLOWED_HOSTS` for internal repos); adding a `helm-repo` project with an unsafe URL is rejected up front
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
