# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build -o newreleases main.go

# Run all tests with race detection
go test -v -race

# Run a single test
go test -v -run TestStoreAddProject ./...

# Test coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Lint (via golangci-lint, used in CI)
golangci-lint run

# Docker
docker build -t newreleases:latest .
docker-compose up

# Buildah/Podman build+push
./build-podman.sh [<repo> <tag> <registry>]
```

## Architecture

Single `main` package, split by concern: `main.go` (wiring/routes), `handlers.go` (HTTP), `store.go` (SQLite + migrations), `fetchers.go` (platform APIs), `ui.go` (`go:embed` of `web/`), `email.go`, `scheduler.go`, `gitlab*.go`/`gitmirror.go`/`awesome.go` (GitLab sync), `models.go`.

**Core types:**
- `Store` — thread-safe state container using `sync.RWMutex`; wraps the SQLite DB (users, sessions, repos, releases, webhooks, settings)
- `Project` — a tracked repo/package (ID, name, platform, URL, refresh stats)
- `Release` — a version entry with truncated description (200 chars) and full `ReleaseNotes`

**HTTP API:**
- `GET/POST /api/projects`, `DELETE /api/projects?id=<id>`
- `GET /api/releases`
- `POST /api/refresh?id=<id>`, `GET /api/refresh-check`

The HTML/CSS/JS frontend lives in `web/` (`index.html`, `app.css`, `app.js`), embedded with `go:embed` and served at `/` and `/static/` — no build step, no bundler. Edit those files directly; they are plain browser JS (inline `onclick=` handlers rely on globals).

**Refresh system:** Projects are "stale" after 30 minutes. `GET /api/refresh-check` triggers background goroutines for all stale projects. All `RefreshProject` calls are non-blocking (`go store.RefreshProject(...)`). After each refresh, `SaveState()` persists to `state.json`.

**Platform fetchers** (`fetchGitHubReleases`, `fetchNPMVersions`, `fetchPyPIReleases`, `fetchDockerTags`, `fetchGitLabReleases`) are standalone functions called from `RefreshProject` based on `project.Platform`. GitHub returns up to 30 releases (stable-first); others return 10.

**State persistence:** SQLite database at `data/newreleases.db`. Created on startup; seeded with 3 demo projects (kubernetes, react, golang) if fewer than 3 exist. Uses `modernc.org/sqlite` (pure Go, no CGO required). WAL mode enabled; writes serialized through a single connection.

**Concurrency:** All store mutations hold a write lock; reads use read lock. Goroutines are fire-and-forget for refresh operations.

**Prereleases:** `releases.is_prerelease` is set by the fetchers (`isPrerelease` heuristic where the API has no flag); `user_repos.include_prereleases` (default off) is per user per project. Read queries filter in SQL; notification senders filter per recipient with `Store.releasesFor`. Releases themselves are shared and always stored.

**Release IDs** are platform-prefixed (`gh_<id>`, `npm_<version>`, `docker_<tag>`) to prevent duplicates on re-fetch.

**Port:** 8080 (hardcoded in `main.go`).

**Security:** webhook and GitLab outbound requests go through the SSRF-guarded `gitLabHTTPTransport` (`GITLAB_ALLOWED_HOSTS` allowlists internal hosts). Login/register are rate limited in-memory (`allow`); set `TRUSTED_PROXY_CIDRS` behind a proxy so limits key on `X-Forwarded-For`.

**Max releases stored per project:** 50 (enforced in `AddRelease`).

## CI/CD

- **GitHub Actions** (`.github/workflows/docker.yml`): tests → Docker build → push to `ghcr.io` → Trivy scan → Codecov
- **GitLab CI** (`.gitlab-ci.yml`): tests → golangci-lint → Buildah/Podman build → push to GitLab registry → Trivy scan → manual deploy stages
