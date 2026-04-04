# TODO

## In Progress

## Planned
- [ ] **Email validation of users** - send an email and a verification link to the user to validate their email
- [ ] **Add Send Email Summary** - to send an email once a day of the previous day's updates (user Option)
- [ ] **Add immediate email** - to send an when a new release is detected. (user option per project)
- [ ] **Pagenate releases and projects** - Allow the user to set the number of projects per page between 5,10 and 20.  Pagenate beyond that number.
- [ ] **Main page sort options** - sort by date added, latest release, and alphabetically.
- [ ] **Determine if a release is a prerelease or not** - allow the user per project to be notified about prereleases.
- [ ] **Use project icon** - Put the project icon to the left of the project name.  



## Done

- [x] **User accounts with auth** — register/login/logout, session cookies, per-user project tracking
- [x] **Shared release data** — deduplicated repos table; two users adding the same project share one release record
- [x] **Smart URL autofill** — platform-aware input: `owner/repo` for GitHub/GitLab, package name for npm/PyPI, image name for Docker; auto-fills display name
- [x] **Other / Custom URL** — paste any supported URL; platform auto-detected with live feedback, name auto-filled
- [x] **Browser push notifications** — VAPID keys auto-generated and persisted; service worker at `/sw.js`; 🔔 button in header; notifies all subscribed users when a new release is detected on refresh
- [x] **Webhooks** — per-project webhook URLs with optional HMAC-SHA256 secret; managed inline from the Projects tab; fired alongside push notifications on new releases
- [x] **Add Project side panel** — Add Project form in a slide-in panel on the right side of the Releases page (Projects tab eliminated)
- [x] **Inline project controls** — Refresh and Delete buttons directly on project headers in Releases view; settings panel for webhooks, push notifications, email
- [x] **Limit releases per project** — inline chip line shows 5 most recent (age: version); +N more ▾ expands full stacked list
- [x] Split monolith `main.go` into focused files (`handlers.go`, `store.go`, `fetchers.go`, `models.go`, `ui.go`)
- [x] Migrate state storage from JSON/YAML to SQLite (WAL mode, `modernc.org/sqlite`)
- [x] Fix delete bug (was re-seeding on restart, wiping unrelated projects)
- [x] Rewrite UI from scratch (no blocking alerts, toast notifications, reliable DOM updates)
- [x] Add Kubernetes manifests with PVC for data persistence (`k8s/newreleases.yaml`)
- [x] Docker multi-stage build (non-root user, healthcheck, pinned base image)
- [x] GitHub Actions + GitLab CI pipelines (build, test, push, Trivy scan)
