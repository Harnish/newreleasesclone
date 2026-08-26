# Release Tracker

Track software releases across GitHub, GitLab, NPM, PyPI, Docker Hub, and Helm charts. Multi-user with shared release data.

## Features

- **Multi-platform**: GitHub, GitLab, NPM, PyPI, Docker Hub, Helm (Artifact Hub & custom repos)
- **User accounts**: Register/login with session-based auth; each user tracks their own set of projects
- **Shared release data**: Two users adding the same repo share one set of fetched releases
- **Smart add form**: Platform-aware input (e.g. `owner/repo` for GitHub, package name for npm); "Other" option auto-detects platform from any pasted URL
- **Browser push notifications**: Click 🔔 in the header to subscribe; notifications fire when a new release is detected on any tracked repo
- **Email notifications**: Opt-in per project — a daily digest of the previous day's releases, and/or an immediate plain-text email the moment a new release is detected (requires SMTP configured and a verified email)
- **Webhooks**: Per-project outbound webhooks with optional HMAC-SHA256 signing; managed inline from the Projects tab
- **GitLab sync**: Register your own GitLab instance + API token (Account Settings), then opt any GitHub/GitLab project into a daily/weekly/monthly mirror push. Optional "Awesome" README, grouped by platform, auto-generated and pushed to a GitLab project under your namespace
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

### Environment Variables

| Variable | Purpose |
|----------|---------|
| `GITLAB_ALLOWED_HOSTS` | Comma-separated hostnames exempt from the GitLab sync SSRF guard (`gitlab_url` normally cannot resolve to a private/loopback/link-local/CGNAT address). Set this if your own GitLab instance resolves to a private IP from where newreleases runs — e.g. split-horizon DNS, or a Kubernetes service name/ClusterIP when both run in the same cluster. Example: `GITLAB_ALLOWED_HOSTS=gitlab.example.internal,gitlab.default.svc.cluster.local`. Operator-set only — not exposed to end users. |

## Adding Projects

Select a platform, enter the identifier, and a display name is auto-filled:

| Platform              | Enter                        | Example                                                         |
|-----------------------|------------------------------|-----------------------------------------------------------------|
| GitHub                | `owner/repo`                 | `kubernetes/kubernetes`                                         |
| GitLab                | `owner/repo`                 | `gitlab-org/gitlab`                                             |
| NPM                   | package name                 | `react`                                                         |
| PyPI                  | package name                 | `requests`                                                      |
| Docker Hub            | image name                   | `nginx` or `user/image`                                         |
| Helm (Artifact Hub)   | Artifact Hub chart URL       | `https://artifacthub.io/packages/helm/bitnami/redis`            |
| Helm (Repo)           | repo base URL + chart name   | URL: `https://charts.bitnami.com/bitnami`, Name: `redis`        |
| Other / Custom URL    | paste any supported URL      | `https://github.com/owner/repo` — platform auto-detected        |

Full URLs are also accepted for GitHub and GitLab in their dedicated fields. The "Other" option accepts any URL from a supported platform and detects it automatically.

### Helm notes

- **Artifact Hub**: paste the full chart page URL; chart name is auto-filled from the URL.
- **Helm Repo**: enter the repo's base URL (the `index.yaml` parent) and provide a display name matching the chart name. Fetches the latest 10 versions from `index.yaml`.

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
| `GET` | `/api/gitlab-settings` | Current user's GitLab instance config (token omitted) |
| `POST` | `/api/gitlab-settings` | Save GitLab instance `{gitlab_url, gitlab_token}` |
| `POST` | `/api/gitlab-settings/awesome` | Enable/disable the Awesome page `{enabled, repo_name}` |
| `POST` | `/api/project-gitlab-sync` | Enable/disable GitLab sync for a project `{repo_id, enabled, frequency}` |
| `POST` | `/api/project-gitlab-sync/sync-now?repo_id=<id>` | Manually trigger a GitLab sync |
| `POST` | `/api/project-settings` | Per-project notification settings `{repo_id, push_enabled, email_immediate}` |


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
| `models.go` | `User`, `Project`, `Release`, `GitLabSettings`, `GitLabSyncTarget` structs |
| `ui.go` | Embedded HTML/CSS/JS frontend |
| `gitlabclient.go` | GitLab REST v4 client (project create/lookup, authenticated push URL) |
| `gitmirror.go` | Ephemeral `git clone --mirror` / push primitives (`os/exec`, no persistent clone dir) |
| `gitlabsync.go` | Per-project sync orchestration |
| `awesome.go` | Awesome-page README generation, grouped by platform |

**Schema (v2):** `users` → `sessions` → `user_repos` ↔ `repos` ← `releases`.  
Repos are deduped by `UNIQUE(platform, repo_url)` so shared repos are fetched once.

## CI/CD

- **GitHub Actions** (`.github/workflows/docker.yml`): test → build → push `ghcr.io` → Trivy scan
- **GitLab CI** (`.gitlab-ci.yml`): test → lint → Buildah build → push GitLab registry → Trivy scan → manual deploy stages
