# 🔗 Quick Links - Release Tracker

## 📖 Documentation by Purpose

### 🚀 I want to get started quickly
→ **[README.md](README.md)** - Project overview and features

### 🐳 I want to use Docker
→ **[DOCKER.md](DOCKER.md)** - Complete Docker guide  
→ **[DOCKER_SETUP.md](DOCKER_SETUP.md)** - Quick Docker reference

### 📦 I want to use Buildah/Podman
→ **[BUILDAH_PODMAN.md](BUILDAH_PODMAN.md)** - Complete Buildah/Podman guide

### 🔄 I want to set up GitLab CI/CD
→ **[GITLAB_CI_CD_SUMMARY.md](GITLAB_CI_CD_SUMMARY.md)** - Quick reference  
→ **[GITLAB_CI_CD.md](GITLAB_CI_CD.md)** - Complete setup guide  
→ **[.gitlab-ci.yml](.gitlab-ci.yml)** - Pipeline configuration

### 🧪 I want to understand testing
→ **[TESTING.md](TESTING.md)** - Testing overview  
→ **[TEST_SUITE.md](TEST_SUITE.md)** - Detailed test documentation

### 🏗️ I want to understand the architecture
→ **[RELEASE_NOTES_FEATURE.md](RELEASE_NOTES_FEATURE.md)** - Release notes  
→ **[REFRESH_FEATURE.md](REFRESH_FEATURE.md)** - Refresh system  
→ **[PERSISTENCE_SUMMARY.md](PERSISTENCE_SUMMARY.md)** - State storage

### 📚 I want to see all documentation
→ **[PROJECT_INDEX.md](PROJECT_INDEX.md)** - Complete documentation index

---

## 🔧 Build & Deploy Scripts

```bash
# Docker build
./build-docker.sh [name] [tag] [registry]

# Buildah/Podman build
./build-podman.sh [name] [tag] [registry]

# Run tests
go test -v ./...
go test -v -race ./...

# Local dev environment
docker-compose up
```

---

## 📋 Configuration Files

| File | Purpose |
|------|---------|
| `.gitlab-ci.yml` | GitLab CI/CD pipeline |
| `.github/workflows/docker.yml` | GitHub Actions workflow |
| `Dockerfile` | Container image |
| `docker-compose.yml` | Local development |
| `go.mod` | Go dependencies |

---

## 🎯 Common Tasks

### Build and Push to Docker Hub
1. `chmod +x build-docker.sh`
2. `./build-docker.sh myapp latest docker.io/yourusername`

### Build with Buildah/Podman
1. `chmod +x build-podman.sh`
2. `./build-podman.sh myapp latest docker.io/yourusername`

### Run Locally
1. `docker-compose up`
2. Open http://localhost:8080

### Run Tests
1. `go test -v ./...`
2. `go test -v -race ./...` (with race detection)

### Set Up GitLab CI/CD
1. Push to GitLab
2. Set CI/CD variables in GitLab Settings
3. Pipeline runs automatically
4. See [GITLAB_CI_CD_SUMMARY.md](GITLAB_CI_CD_SUMMARY.md)

---

## 📊 Project Stats

- **Tests**: 19 (all passing)
- **Coverage**: 44.5%
- **Documentation**: 17 files, 5000+ lines
- **Image Size**: 15-20 MB
- **Build Time**: 2-3 minutes

---

## ✨ Features

✅ Multi-platform release tracking (GitHub, GitLab, NPM, PyPI, Docker)  
✅ Release notes capture  
✅ Smart refresh system  
✅ Persistent state storage  
✅ Docker containerization  
✅ Buildah/Podman support  
✅ GitLab CI/CD pipeline  
✅ GitHub Actions workflow  
✅ Comprehensive testing  
✅ RESTful API  
✅ Web-based UI  

---

**Status**: ✅ Production Ready

**Get started**: Pick a task above and follow the link! 🚀
