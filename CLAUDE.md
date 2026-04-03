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

`main.go` (~1,300 lines) is the entire backend — no packages, no external frameworks.

**Core types:**
- `Store` — thread-safe state container using `sync.RWMutex`; holds projects, releases, and refresh timestamps; persists to `state.json`
- `Project` — a tracked repo/package (ID, name, platform, URL, refresh stats)
- `Release` — a version entry with truncated description (200 chars) and full `ReleaseNotes`

**HTTP API:**
- `GET/POST /api/projects`, `DELETE /api/projects?id=<id>`
- `GET /api/releases`
- `POST /api/refresh?id=<id>`, `GET /api/refresh-check`

The HTML/CSS/JS frontend is embedded as string constants in `main.go` — no build step, no bundler.

**Refresh system:** Projects are "stale" after 30 minutes. `GET /api/refresh-check` triggers background goroutines for all stale projects. All `RefreshProject` calls are non-blocking (`go store.RefreshProject(...)`). After each refresh, `SaveState()` persists to `state.json`.

**Platform fetchers** (`fetchGitHubReleases`, `fetchNPMVersions`, `fetchPyPIReleases`, `fetchDockerTags`, `fetchGitLabReleases`) are standalone functions called from `RefreshProject` based on `project.Platform`. GitHub returns up to 30 releases (stable-first); others return 10.

**State persistence:** SQLite database at `data/newreleases.db`. Created on startup; seeded with 3 demo projects (kubernetes, react, golang) if fewer than 3 exist. Uses `modernc.org/sqlite` (pure Go, no CGO required). WAL mode enabled; writes serialized through a single connection.

**Concurrency:** All store mutations hold a write lock; reads use read lock. Goroutines are fire-and-forget for refresh operations.

**Release IDs** are platform-prefixed (`gh_<id>`, `npm_<version>`, `docker_<tag>`) to prevent duplicates on re-fetch.

**Port:** 8080 (hardcoded near EOF of `main.go`).

**Max releases stored per project:** 50 (enforced in `AddRelease`).

## CI/CD

- **GitHub Actions** (`.github/workflows/docker.yml`): tests → Docker build → push to `ghcr.io` → Trivy scan → Codecov
- **GitLab CI** (`.gitlab-ci.yml`): tests → golangci-lint → Buildah/Podman build → push to GitLab registry → Trivy scan → manual deploy stages
