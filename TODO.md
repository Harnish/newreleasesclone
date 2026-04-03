# TODO

## In Progress

## Planned
- [ ] **Custom URL option** — "Other" platform entry where a full URL is pasted and the platform is auto-detected
- [ ] **Browser push notifications** — service worker + VAPID keys + push subscriptions stored per user; notify on new release
- [ ] **Webhooks** — per-project outbound webhook URLs; HMAC-SHA256 signed POST payload on new release
- [ ] **Add Project side panel** — move the Add Project form to a slide-in panel on the right side of the Releases page (eliminate the Projects tab)
- [ ] **Limit releases per project** — show only the 5 most recent releases per project card, with a "Show all" expand link
- [ ] **Inline project controls** — add Refresh and Delete buttons directly on the project headers in the Releases view

## Done

- [x] **User accounts with auth** — register/login/logout, session cookies, per-user project tracking
- [x] **Shared release data** — deduplicated repos table; two users adding the same project share one release record
- [x] **Smart URL autofill** — platform-aware input: `owner/repo` for GitHub/GitLab, package name for npm/PyPI, image name for Docker; auto-fills display name
- [x] Split monolith `main.go` into focused files (`handlers.go`, `store.go`, `fetchers.go`, `models.go`, `ui.go`)
- [x] Migrate state storage from JSON/YAML to SQLite (WAL mode, `modernc.org/sqlite`)
- [x] Fix delete bug (was re-seeding on restart, wiping unrelated projects)
- [x] Rewrite UI from scratch (no blocking alerts, toast notifications, reliable DOM updates)
- [x] Add Kubernetes manifests with PVC for data persistence (`k8s/newreleases.yaml`)
- [x] Docker multi-stage build (non-root user, healthcheck, pinned base image)
- [x] GitHub Actions + GitLab CI pipelines (build, test, push, Trivy scan)
