# Release Tracker

Track software releases across GitHub, GitLab, NPM, PyPI, and Docker Hub. Multi-user with shared release data.

## Features

- **Multi-platform**: GitHub, GitLab, NPM, PyPI, Docker Hub
- **User accounts**: Register/login with session-based auth; each user tracks their own set of projects
- **Shared release data**: Two users adding the same repo share one set of fetched releases
- **Smart add form**: Platform-aware input (e.g. `owner/repo` for GitHub, package name for npm); "Other" option auto-detects platform from any pasted URL
- **Browser push notifications**: Click 🔔 in the header to subscribe; notifications fire when a new release is detected on any tracked repo
- **Webhooks**: Per-project outbound webhooks with optional HMAC-SHA256 signing; managed inline from the Projects tab
- **Auto-refresh**: Stale repos (>30 min) are refreshed in the background on page load
- **Full release notes**: GitHub/GitLab release bodies captured and expandable in the UI
- **SQLite persistence**: All data stored in `data/newreleases.db`

## Running

```bash
go build -o newreleases .
./newreleases
# → http://localhost:8080
```

Register an account on first visit, then add projects to track.

## Adding Projects

Select a platform, enter the identifier, and a display name is auto-filled:

| Platform          | Enter                  | Example                                     |
|-------------------|------------------------|---------------------------------------------|
| GitHub            | `owner/repo`           | `kubernetes/kubernetes`                     |
| GitLab            | `owner/repo`           | `gitlab-org/gitlab`                         |
| NPM               | package name           | `react`                                     |
| PyPI              | package name           | `requests`                                  |
| Docker Hub        | image name             | `nginx` or `user/image`                     |
| Other / Custom URL | paste any supported URL | `https://github.com/owner/repo` — platform auto-detected |

Full URLs are also accepted for GitHub and GitLab in their dedicated fields. The "Other" option accepts any URL from a supported platform and detects it automatically.

## API

All data endpoints require a valid session cookie (set by `/api/login` or `/api/register`).

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/webhooks?repo_id=<id>` | List webhooks for a project |
| `POST` | `/api/webhooks` | Add webhook `{repo_id, url, secret?}` |
| `DELETE` | `/api/webhooks?id=<id>` | Remove a webhook |
| `GET` | `/api/push/vapid-key` | VAPID public key for subscription |
| `POST` | `/api/push/subscribe` | Save push subscription `{endpoint, keys}` |
| `DELETE` | `/api/push/subscribe` | Remove push subscription `{endpoint}` |


|--------|------|-------------|
| `POST` | `/api/register` | Create account |
| `POST` | `/api/login` | Sign in |
| `POST` | `/api/logout` | Sign out |
| `GET` | `/api/me` | Current user |
| `GET` | `/api/projects` | List tracked projects |
| `POST` | `/api/projects` | Add project `{name, platform, repo_url}` |
| `DELETE` | `/api/projects?id=<id>` | Stop tracking a project |
| `GET` | `/api/releases` | All releases for tracked projects |
| `POST` | `/api/refresh?id=<id>` | Refresh a specific project |
| `GET` | `/api/refresh-check` | Trigger background refresh of stale repos |

## Development

```bash
go test -v ./...                          # all tests
go test -v -run TestStoreAddRepo ./...    # single test
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
golangci-lint run
```

## Docker / Buildah

```bash
docker build -t newreleases:latest .
docker run -p 8080:8080 -v newreleases-data:/app/data newreleases:latest

# Buildah/Podman (used in GitLab CI)
./build-podman.sh [<repo> <tag> <registry>]
```

## Kubernetes

```bash
kubectl apply -f k8s/newreleases.yaml
```

Uses a `local-path` PVC to persist `data/` across pod restarts.

## Architecture

| File | Responsibility |
|------|---------------|
| `main.go` | Route registration, server startup |
| `handlers.go` | HTTP handlers, `requireAuth` middleware |
| `store.go` | SQLite layer — users, sessions, repos, releases |
| `fetchers.go` | Per-platform release fetchers |
| `models.go` | `User`, `Project`, `Release` structs |
| `ui.go` | Embedded HTML/CSS/JS frontend |

**Schema (v2):** `users` → `sessions` → `user_repos` ↔ `repos` ← `releases`.  
Repos are deduped by `UNIQUE(platform, repo_url)` so shared repos are fetched once.

## CI/CD

- **GitHub Actions** (`.github/workflows/docker.yml`): test → build → push `ghcr.io` → Trivy scan
- **GitLab CI** (`.gitlab-ci.yml`): test → lint → Buildah build → push GitLab registry → Trivy scan → manual deploy stages
