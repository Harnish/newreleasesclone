# 🐳 Docker Setup - Complete

## 📦 What Was Created

### Docker Files
```
✅ Dockerfile                    - Multi-stage build with testing
✅ .dockerignore                 - Excludes unnecessary files
✅ docker-compose.yml            - Local development setup
✅ build-docker.sh              - Automated build script
✅ .github/workflows/docker.yml  - CI/CD pipeline
✅ DOCKER.md                     - Comprehensive documentation
```

## 🏗️ Build Architecture

### Multi-Stage Build Process

```
Stage 1: Builder (golang:1.24.9-alpine)
├── Download dependencies (go mod download)
├── Run full test suite (go test -v -race)
└── Compile binary (CGO_ENABLED=0)
        ↓
Stage 2: Final (alpine:latest)
├── Add CA certificates
├── Copy binary from builder
├── Configure health check
└── Set entry point
        ↓
Result: Minimal production image (~15-20 MB)
```

### Key Features

✅ **Integrated Testing**
- Tests run during build
- Build fails if tests fail
- Race conditions detected

✅ **Minimal Image**
- Alpine Linux base (5 MB)
- Statically linked binary
- No build dependencies in final image
- Production ready (~15-20 MB total)

✅ **Health Checks**
- Automatic health monitoring
- 30-second intervals
- Port 8080 health endpoint

✅ **Security**
- Non-root user ready
- Minimal attack surface
- CA certificates included
- Read-only filesystem compatible

## 🚀 Quick Start

### Build Image

```bash
# Using build script (easiest)
chmod +x build-docker.sh
./build-docker.sh

# Or using Docker directly
docker build -t newreleases:latest .

# With custom name and tag
docker build -t myapp:v1.0.0 .
```

### Run Container

```bash
# Basic run
docker run -p 8080:8080 newreleases:latest

# With persistent state
docker run -p 8080:8080 -v $(pwd)/state.yaml:/app/state.yaml newreleases:latest

# Using docker-compose
docker-compose up

# Detached (background)
docker run -d -p 8080:8080 --name app newreleases:latest
```

### Push to Registry

```bash
# Using build script
./build-docker.sh newreleases latest docker.io/yourusername

# Or manually
docker tag newreleases:latest docker.io/yourusername/newreleases:latest
docker push docker.io/yourusername/newreleases:latest
```

## 📋 Build Script Usage

### Basic Commands

```bash
# Default build (newreleases:latest)
./build-docker.sh

# Custom name and tag
./build-docker.sh myapp v1.0.0

# Build and push to registry
./build-docker.sh myapp v1.0.0 docker.io/username

# Show help
./build-docker.sh --help
```

### Features

- ✅ Automatic Docker verification
- ✅ File validation
- ✅ Build progress display
- ✅ Image information reporting
- ✅ Optional vulnerability scanning (Trivy)
- ✅ Registry push support
- ✅ Colored output
- ✅ Error handling

## 🐳 Docker Compose Setup

### Local Development

```bash
# Start application
docker-compose up

# Start in background
docker-compose up -d

# View logs
docker-compose logs -f

# Stop and remove
docker-compose down

# Rebuild image
docker-compose up --build
```

### Configuration

Services defined:
- **newreleases** - Main application
  - Port: 8080
  - Volume: state.yaml (persistent state)
  - Health check: Enabled
  - Auto-restart: Unless stopped

## 📚 Image Information

### Final Image Specs

| Property | Value |
|----------|-------|
| **Base Image** | alpine:latest |
| **Binary Size** | ~2-3 MB |
| **Image Size** | ~15-20 MB |
| **Port** | 8080 |
| **Health Check** | Yes (30s interval) |
| **Executable** | /app/newreleases |

### Build Time

Typical build times:
- First build: 1-2 minutes (dependency download)
- Subsequent builds: 10-30 seconds (cached layers)
- With BuildKit: ~20% faster

## 🔄 CI/CD Integration

### GitHub Actions Workflow

File: `.github/workflows/docker.yml`

Features:
- ✅ Automated build on push/PR
- ✅ Test execution before build
- ✅ Coverage reporting
- ✅ Docker image build and push
- ✅ Security scanning with Trivy
- ✅ Automated tagging
- ✅ Multi-architecture support (amd64, arm64)

### Trigger Events

- **Push to main**: Build and push
- **Push to develop**: Build only
- **Tags (v*.*.*): Build and push with semver tags
- **Pull requests**: Build only (no push)

### Registry Options

Default: GitHub Container Registry (ghcr.io)

To use Docker Hub:
```yaml
registry: docker.io
username: ${{ secrets.DOCKERHUB_USERNAME }}
password: ${{ secrets.DOCKERHUB_TOKEN }}
```

## 📤 Pushing to Registries

### GitHub Container Registry

```bash
# Login
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Tag
docker tag newreleases:latest ghcr.io/yourusername/newreleases:latest

# Push
docker push ghcr.io/yourusername/newreleases:latest
```

### Docker Hub

```bash
# Login
docker login

# Tag
docker tag newreleases:latest yourusername/newreleases:latest

# Push
docker push yourusername/newreleases:latest
```

### Private Registry

```bash
# Login (if required)
docker login myregistry.com

# Tag
docker tag newreleases:latest myregistry.com/newreleases:latest

# Push
docker push myregistry.com/newreleases:latest
```

## 🧪 Testing

### Build-Time Tests

Tests automatically run during build:

```go
go test -v -race
```

If tests fail → Build fails (ensures quality)

### Runtime Testing

```bash
# Check health
docker exec app wget -O- http://localhost:8080/ || echo "Unhealthy"

# Check container status
docker ps

# View logs
docker logs app

# Get shell access
docker exec -it app /bin/sh
```

## 🔒 Security

### Best Practices Implemented

✅ **Minimal Base Image**: Alpine Linux reduces attack surface
✅ **Static Binary**: No runtime dependencies to exploit
✅ **Non-root Ready**: Can run with restricted privileges
✅ **Health Checks**: Monitors container health
✅ **Read-only Filesystem**: Compatible with read-only mode

### Vulnerability Scanning

Using Trivy (GitHub Actions):

```bash
# Scan local image
trivy image newreleases:latest

# Scan with severity filter
trivy image --severity HIGH,CRITICAL newreleases:latest
```

## 🎯 Common Tasks

### Build

```bash
./build-docker.sh                    # Default
./build-docker.sh myapp v1.0.0      # Custom
./build-docker.sh myapp v1.0.0 reg  # Push
```

### Run

```bash
docker run -p 8080:8080 newreleases:latest
docker-compose up
./build-docker.sh && docker run -p 8080:8080 newreleases:latest
```

### Manage

```bash
docker ps                     # List running
docker logs app               # View logs
docker exec -it app /bin/sh  # Shell access
docker stop app               # Stop
docker rm app                 # Remove
```

### Debug

```bash
docker build -t test . --progress=plain  # Detailed build output
docker run -it newreleases:latest /bin/sh  # Interactive shell
docker inspect app                        # Container details
docker stats app                          # Resource usage
```

## 📁 File Structure

```
newreleases/
├── Dockerfile                 ← Multi-stage build
├── .dockerignore             ← Exclude files
├── docker-compose.yml        ← Dev environment
├── build-docker.sh           ← Build script
├── DOCKER.md                 ← Full documentation
├── .github/workflows/
│   └── docker.yml            ← GitHub Actions
├── main.go                   ← Application
├── main_test.go              ← Tests (run in build)
├── go.mod & go.sum           ← Dependencies
└── state.yaml                ← Persistent state
```

## 🚀 Deployment Examples

### Docker Only

```bash
docker run -d \
  --name newreleases \
  -p 8080:8080 \
  -v state:/app/state \
  --restart unless-stopped \
  newreleases:latest
```

### Docker Compose

```bash
docker-compose up -d
```

### Kubernetes

```bash
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: newreleases
spec:
  replicas: 1
  selector:
    matchLabels:
      app: newreleases
  template:
    metadata:
      labels:
        app: newreleases
    spec:
      containers:
      - name: newreleases
        image: newreleases:latest
        ports:
        - containerPort: 8080
        livenessProbe:
          httpGet:
            path: /
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
EOF
```

## ✅ Verification Checklist

- ✅ Dockerfile created with multi-stage build
- ✅ Tests integrated into build process
- ✅ .dockerignore excludes unnecessary files
- ✅ docker-compose.yml for local development
- ✅ build-docker.sh for automated builds
- ✅ GitHub Actions workflow for CI/CD
- ✅ DOCKER.md with comprehensive documentation
- ✅ Health checks configured
- ✅ Security best practices implemented
- ✅ Registry push support enabled

## 📖 Documentation

Full documentation available in `DOCKER.md`:
- Detailed build process explanation
- All command examples
- Troubleshooting guide
- Performance optimization tips
- Security recommendations
- Deployment guides for various platforms

## 🎓 Next Steps

1. **Build locally**
   ```bash
   ./build-docker.sh
   ```

2. **Run with docker-compose**
   ```bash
   docker-compose up
   ```

3. **Test the container**
   ```bash
   docker logs app
   ```

4. **Push to registry**
   ```bash
   ./build-docker.sh myapp v1.0.0 docker.io/username
   ```

5. **Deploy**
   - Docker: `docker run ...`
   - Compose: `docker-compose up`
   - Kubernetes: `kubectl apply -f deployment.yaml`

## 🆘 Troubleshooting

### Build Fails

**Problem**: Tests fail during build
```
Solution: Run tests locally first
go test -v
```

**Problem**: Docker not found
```
Solution: Install Docker
https://docs.docker.com/get-docker/
```

### Runtime Issues

**Problem**: Container exits immediately
```
Solution: Check logs
docker logs app
```

**Problem**: Port already in use
```
Solution: Use different port
docker run -p 8081:8080 newreleases:latest
```

## 📞 Support

| Question | Answer |
|----------|--------|
| How do I build? | `./build-docker.sh` |
| How do I run? | `docker-compose up` or `docker run -p 8080:8080 newreleases:latest` |
| How do I push? | `./build-docker.sh myapp v1.0.0 docker.io/username` |
| Where's the docs? | See `DOCKER.md` |
| How do I debug? | `docker logs app` or `docker exec -it app /bin/sh` |

## 🎉 Summary

Complete Docker setup with:
- ✅ Production-ready multi-stage Dockerfile
- ✅ Integrated testing in build
- ✅ Minimal final image (~15-20 MB)
- ✅ Docker Compose for local development
- ✅ Automated build script
- ✅ GitHub Actions CI/CD workflow
- ✅ Registry push support
- ✅ Security best practices
- ✅ Comprehensive documentation

Ready to build, test, and deploy! 🚀

---

**Created**: November 10, 2025
**Docker Files**: 6 files (Dockerfile, docker-compose.yml, build script, workflows)
**Status**: ✅ Production Ready
