# Release Tracker 🚀

A modern, real-time release tracking system that monitors software releases across multiple platforms (GitHub, GitLab, NPM, PyPI, Docker) with persistent state storage and automatic refresh capabilities.

## Features

✨ **Multi-Platform Support**
- GitHub releases with full release notes
- GitLab releases
- NPM package versions
- PyPI releases
- Docker image tags

🔄 **Smart Refresh System**
- **Auto-refresh on project add** - Immediately fetches releases when adding new projects
- **Stale data detection** - Automatically refreshes data older than 30 minutes on page load
- **Manual refresh** - One-click refresh button for any project
- Background goroutine-based refreshes to avoid blocking

💾 **Persistent State**
- All projects and releases saved to `state.yaml`
- State survives server restarts
- Human-readable YAML format for manual inspection/editing

📝 **Full Release Notes**
- Complete release notes captured from GitHub and GitLab
- Truncated previews (200 chars) for quick scanning
- Full text available via API for integration

🎨 **Modern Web Interface**
- Dark theme UI with responsive design
- Tabbed navigation (Releases & Projects)
- Real-time updates without page refresh
- One-click project refresh with timestamps
- Release metadata with platform badges and publish dates

## Prerequisites

- Go 1.24.9 or later
- No external database required (uses local YAML storage)

## Installation

### Clone the repository
```bash
git clone https://github.com/yourusername/newreleases.git
cd newreleases
```

### Download dependencies
```bash
go mod download
```

### Build the application
```bash
go build -o newreleases main.go
```

## Running

### Start the server
```bash
./newreleases
```

The application will start on `http://localhost:8080`

### First Run
On first run with no existing `state.yaml`:
- Server seeds demo data with 3 sample projects (kubernetes, react, golang)
- Automatically fetches releases for each project
- Saves state to `state.yaml`

### Subsequent Runs
- Server loads existing projects and releases from `state.yaml`
- Resumes tracking from where it left off

## Usage

### Web Interface

1. **Releases Tab** - View all releases across all tracked projects
   - Shows project name, version, platform, publish date
   - Displays truncated release notes/description
   - Organized by project

2. **Projects Tab** - Manage your tracked projects
   - Add new projects to track
   - See last refresh timestamp for each project
   - Manual refresh button for immediate data update
   - Link to project repository

### Adding Projects

Fill out the form in the Projects tab with:
- **Project Name** - Display name (e.g., "kubernetes")
- **Platform** - Select from: github, gitlab, npm, pypi, docker
- **Repository URL** - Full URL to the project
  - GitHub: `https://github.com/owner/repo`
  - GitLab: `https://gitlab.com/owner/repo`
  - NPM/PyPI: Package name or repo URL
  - Docker: Docker Hub repository name

Example:
```
Name: kubernetes
Platform: github
URL: https://github.com/kubernetes/kubernetes
```

### API Endpoints

#### Get all projects
```bash
curl http://localhost:8080/api/projects
```

Response:
```json
[
  {
    "id": "proj_1234567890",
    "name": "kubernetes",
    "platform": "github",
    "repo_url": "https://github.com/kubernetes/kubernetes",
    "last_refresh": "2025-11-10T12:00:00Z",
    "refresh_count": 5
  }
]
```

#### Add a new project
```bash
curl -X POST http://localhost:8080/api/projects \
  -H "Content-Type: application/json" \
  -d '{
    "name": "docker",
    "platform": "github",
    "repo_url": "https://github.com/moby/moby"
  }'
```

#### Get all releases
```bash
curl http://localhost:8080/api/releases
```

Response:
```json
[
  {
    "id": "gh_123456",
    "name": "kubernetes",
    "version": "v1.30.0",
    "platform": "github",
    "url": "https://github.com/kubernetes/kubernetes/releases/tag/v1.30.0",
    "published_at": "2025-11-10T10:30:00Z",
    "description": "Major release with new features and improvements...",
    "release_notes": "## Features\n- Feature A\n- Feature B\n\n## Bug Fixes\n- Fix A\n- Fix B"
  }
]
```

#### Check for stale data and refresh
```bash
curl http://localhost:8080/api/refresh-check
```

Response:
```json
{
  "refreshed_count": 2,
  "stale_projects": [...]
}
```

#### Manually refresh a project
```bash
curl -X POST "http://localhost:8080/api/refresh?id=proj_1234567890"
```

## Project Structure

```
newreleases/
├── main.go                    # Main application code
├── go.mod                     # Go module definition
├── go.sum                     # Dependency checksums
├── state.yaml                 # Persisted state (auto-generated)
├── README.md                  # This file
├── RELEASE_NOTES_FEATURE.md   # Release notes implementation details
├── PERSISTENCE_SUMMARY.md     # State persistence documentation
├── REFRESH_FEATURE.md         # Refresh system documentation
├── REFRESH_FLOW.md            # Detailed refresh flow diagrams
└── STATE_STORAGE.md           # State storage architecture
```

## Data Structures

### Release
```go
type Release struct {
    ID           string    // Unique identifier
    Name         string    // Project name
    Version      string    // Version/tag name
    Platform     string    // github, gitlab, npm, pypi, docker
    URL          string    // Link to release
    PublishedAt  time.Time // Publication timestamp
    Description  string    // Truncated (200 chars) for display
    ReleaseNotes string    // Full release notes text
}
```

### Project
```go
type Project struct {
    ID          string    // Unique project ID
    Name        string    // Project name
    Platform    string    // Platform type
    RepoURL     string    // Repository URL
    LastRefresh time.Time // Last refresh timestamp
    RefreshCount int      // Total number of refreshes
}
```

## State File Format

`state.yaml` stores all projects, releases, and refresh timestamps:

```yaml
projects:
  proj_1234567890:
    id: proj_1234567890
    name: kubernetes
    platform: github
    repo_url: https://github.com/kubernetes/kubernetes
    last_refresh: 2025-11-10T12:00:00Z
    refresh_count: 5

releases:
  proj_1234567890:
    - id: gh_123456
      name: kubernetes
      version: v1.30.0
      platform: github
      url: https://github.com/kubernetes/kubernetes/releases/tag/v1.30.0
      published_at: 2025-11-10T10:30:00Z
      description: "Major release..."
      release_notes: "## Features\n- Feature A\n..."

refreshed:
  proj_1234567890: 2025-11-10T12:00:00Z
```

## Configuration

### Stale Data Threshold
Projects are considered stale if not refreshed within **30 minutes**. This is checked:
- On page load (via `/api/refresh-check`)
- Manual refresh always fetches latest data

### Limits
- Maximum releases stored per project: **50**
- NPM versions displayed: **10 latest**
- PyPI releases displayed: **10 latest**
- Docker tags displayed: **10 latest**
- GitHub/GitLab releases: **30 latest** (with stable release priority)

To modify these limits, edit the constants in `main.go`:
- Line ~85: `if len(s.releases[projectID]) > 50`
- Line ~315: `if i >= 10`
- Line ~376: `if i >= 10`
- Line ~180: `?per_page=30`

## Troubleshooting

### Port already in use
Change the port in `main.go` line 890:
```go
log.Fatal(http.ListenAndServe(":8081", nil)) // Use 8081 instead
```

### No releases found
- Ensure the project URL is correct for the platform
- Check network connectivity to the API endpoints
- Some platforms may have rate limits (GitHub API)

### State file corrupted
Delete `state.yaml` and restart the server:
```bash
rm state.yaml
./newreleases
```
Server will reseed with demo data.

### Empty releases list
Projects may not have completed their initial refresh. Wait a few seconds or:
1. Click "Refresh" on a project in the Projects tab
2. Or navigate to Releases tab which triggers auto-refresh

## Performance

### Concurrent Operations
- Each project refresh runs in its own goroutine (non-blocking)
- State saves happen asynchronously
- Multiple projects can be refreshed simultaneously

### API Rate Limiting
- GitHub API: 60 requests/hour (unauthenticated)
- GitLab API: 10 requests/second (public API)
- NPM/PyPI/Docker: No strict limits for basic access

For higher limits, consider adding API authentication tokens.

## Development

### Requirements
- Go 1.24.9 or later
- `gopkg.in/yaml.v3` (automatically managed by `go.mod`)

### Build
```bash
go build main.go
```

### Run with verbose logging
```bash
./newreleases 2>&1 | grep -E "^(✓|⚠|🚀)"
```

## Containerization & Deployment

### Docker

Build and run with Docker:
```bash
# Build image
docker build -t newreleases:latest .

# Run container
docker run -p 8080:8080 newreleases:latest

# Or use the provided build script
./build-docker.sh
```

See [DOCKER.md](DOCKER.md) for complete Docker setup guide.

### Buildah/Podman

Build OCI-compatible images with Buildah or Podman:
```bash
# Make script executable
chmod +x build-podman.sh

# Build with defaults
./build-podman.sh

# Build and push to registry
./build-podman.sh newreleases latest docker.io/yourusername
```

See [BUILDAH_PODMAN.md](BUILDAH_PODMAN.md) for complete Buildah/Podman guide.

### Docker-Compose

Quick local development setup:
```bash
# Start all services
docker-compose up

# In background
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

## CI/CD Pipelines

### GitHub Actions

Automated build and push to Docker Hub on every push:
```bash
# Configure secrets in GitHub:
# - DOCKER_USERNAME
# - DOCKER_TOKEN

# Workflow file: .github/workflows/docker.yml
# Automatically triggered on push/pull requests
```

See [.github/workflows/docker.yml](.github/workflows/docker.yml) for details.

### GitLab CI/CD

Comprehensive multi-stage pipeline with testing, building, and deployment:
```bash
# Configure variables in GitLab Settings > CI/CD:
# - DOCKERHUB_USER (optional)
# - DOCKERHUB_TOKEN (optional)
# - STAGING_HOST, STAGING_USER, STAGING_PATH
# - PRODUCTION_HOST, PRODUCTION_USER, PRODUCTION_PATH
# - SSH_PRIVATE_KEY

# Pipeline stages:
# 1. Build: Buildah and Podman builds
# 2. Test: Unit tests, linting, security scanning
# 3. Push: Push to GitLab Registry and Docker Hub
# 4. Deploy: Manual deployment to staging/production
```

**GitLab Pipeline Features**:
- ✅ Build with Buildah (fast, efficient)
- ✅ Build with Podman (Docker-compatible)
- ✅ Run unit tests with coverage reporting
- ✅ Lint with golangci-lint
- ✅ Security scan with Trivy
- ✅ Push to GitLab Registry automatically
- ✅ Push to Docker Hub on tags (manual)
- ✅ Deploy to staging/production (manual)
- ✅ Create releases on tags

See [.gitlab-ci.yml](.gitlab-ci.yml) for complete pipeline configuration.

**Quick GitLab Setup**:
1. Push project to GitLab
2. Set CI/CD variables in **Settings > CI/CD > Variables**
3. Pipeline runs automatically on push
4. View pipeline in **CI/CD > Pipelines**

**Running Deployments**:
```
# Manual deployment available via GitLab UI:
1. Go to CI/CD > Pipelines
2. Click on pipeline
3. Find deploy_staging or deploy_production job
4. Click "Play" button
5. Watch logs in real-time
```

## Testing

Run comprehensive test suite:
```bash
# Run all tests
go test -v ./...

# Run with race detection
go test -v -race ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific test
go test -v -run TestStoreAddProject ./...
```

See [TESTING.md](TESTING.md) for detailed testing documentation.

## Future Enhancements

- [x] Docker image for easy deployment
- [x] Buildah/Podman OCI image support
- [x] GitHub Actions CI/CD pipeline
- [x] GitLab CI/CD pipeline
- [ ] API authentication and security
- [ ] Release notes markdown rendering in UI
- [ ] Notification system (email, Slack, Discord)
- [ ] Release comparison between versions
- [ ] Search and advanced filtering
- [ ] Configurable refresh intervals per project
- [ ] Webhook support for external integrations
- [ ] Database backend option (PostgreSQL, SQLite)
- [ ] Multiple users/teams support

## Contributing

Contributions are welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Submit a pull request

## License

MIT License - See LICENSE file for details

## Support

For issues or questions:
- Check existing documentation files (PERSISTENCE_SUMMARY.md, REFRESH_FEATURE.md, etc.)
- Review the troubleshooting section above
- Submit an issue on GitHub

## Changelog

### v0.1.0 (Current)
- Initial release
- Multi-platform release tracking
- Persistent state storage with YAML
- Smart refresh system with stale detection
- Full release notes support
- Web-based management interface
- RESTful API endpoints

---

Made with ❤️ for tracking software releases
