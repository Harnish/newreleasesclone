# Docker Setup Guide

## Overview

This guide explains how to build, test, and deploy the Release Tracker application using Docker.

## Files

### `Dockerfile`
Multi-stage build that:
1. **Builder Stage**: Compiles and tests the application
   - Uses Go 1.24.9 Alpine image
   - Downloads dependencies
   - Runs full test suite with race detection
   - Builds statically-linked binary

2. **Final Stage**: Creates minimal production image
   - Uses Alpine Linux (small footprint)
   - Only includes the compiled binary
   - Includes CA certificates for HTTPS
   - Health checks enabled

### `.dockerignore`
Excludes unnecessary files from Docker build context:
- Git files and documentation
- Test files and build artifacts
- IDE configuration
- CI/CD files

### `docker-compose.yml`
Development environment with:
- Single service configuration
- Port mapping (8080:8080)
- Volume for persistent state
- Health checks
- Automatic restart

### `build-docker.sh`
Automated build script with:
- Docker verification
- File validation
- Image building
- Testing
- Vulnerability scanning (optional)
- Registry push support

## Quick Start

### 1. Build Docker Image

```bash
# Build with default name (newreleases:latest)
./build-docker.sh

# Build with custom name and tag
./build-docker.sh myapp v1.0.0

# Build and push to registry
./build-docker.sh myapp v1.0.0 docker.io/myusername
```

Or using Docker directly:

```bash
docker build -t newreleases:latest .
```

### 2. Run Container

```bash
# Run with default settings
docker run -p 8080:8080 newreleases:latest

# Run with state persistence
docker run -p 8080:8080 -v $(pwd)/state.yaml:/app/state.yaml newreleases:latest

# Run in background
docker run -d -p 8080:8080 --name newreleases newreleases:latest

# Run with environment variables
docker run -p 8080:8080 -e LOG_LEVEL=debug newreleases:latest
```

### 3. Using Docker Compose

```bash
# Start application
docker-compose up

# Start in background
docker-compose up -d

# Stop application
docker-compose down

# View logs
docker-compose logs -f

# Rebuild image
docker-compose up --build
```

## Build Options

### Using Build Script

```bash
# Make script executable
chmod +x build-docker.sh

# Build with defaults
./build-docker.sh

# Build with custom settings
./build-docker.sh myimage v1.0.0

# Push to registry
./build-docker.sh myimage v1.0.0 docker.io/username

# Show help
./build-docker.sh --help
```

### Using Docker BuildKit (Faster)

```bash
export DOCKER_BUILDKIT=1
docker build -t newreleases:latest .
```

### Using Docker Buildx (Multi-platform)

```bash
docker buildx build --platform linux/amd64,linux/arm64 \
  -t myregistry/newreleases:latest \
  --push .
```

## Image Details

### Image Specifications

- **Base Image**: Alpine Linux 3.x (minimal, ~5MB)
- **Go Version**: 1.24.9
- **Binary**: Statically linked, no dependencies
- **Size**: ~15-20 MB (production image)
- **Port**: 8080 (configurable)
- **Health Check**: Enabled (30s interval)

### Build Process

```
┌─────────────────────────────────────┐
│  Builder Stage (golang:1.24.9)      │
├─────────────────────────────────────┤
│ 1. Download dependencies            │
│ 2. Run test suite (go test -race)   │
│ 3. Compile binary (CGO_ENABLED=0)   │
│ 4. Strip debug symbols              │
└─────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────┐
│  Final Stage (alpine:latest)        │
├─────────────────────────────────────┤
│ 1. Add CA certificates              │
│ 2. Copy binary from builder         │
│ 3. Configure health check           │
│ 4. Set entry point                  │
└─────────────────────────────────────┘
                    ↓
        [Production Image Ready]
```

## Pushing to Registry

### Docker Hub

```bash
# Login to Docker Hub
docker login

# Tag image
docker tag newreleases:latest docker.io/username/newreleases:latest

# Push to Docker Hub
docker push docker.io/username/newreleases:latest

# Or use build script
./build-docker.sh newreleases latest docker.io/username
```

### GitHub Container Registry

```bash
# Login to GitHub Container Registry
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Push image
docker push ghcr.io/username/newreleases:latest
```

### Private Registry

```bash
# Tag for private registry
docker tag newreleases:latest myregistry.com/newreleases:latest

# Login (if required)
docker login myregistry.com

# Push
docker push myregistry.com/newreleases:latest
```

## Container Management

### Running Container

```bash
# Basic run
docker run -p 8080:8080 newreleases:latest

# With name
docker run --name app -p 8080:8080 newreleases:latest

# Detached (background)
docker run -d -p 8080:8080 --name app newreleases:latest

# With volume for state persistence
docker run -v state:/app/state -p 8080:8080 newreleases:latest

# With environment variables
docker run -e LOG_LEVEL=debug -p 8080:8080 newreleases:latest

# With resource limits
docker run -m 256m -p 8080:8080 newreleases:latest
```

### Managing Containers

```bash
# List running containers
docker ps

# List all containers
docker ps -a

# View logs
docker logs app

# Follow logs (live)
docker logs -f app

# Stop container
docker stop app

# Start container
docker start app

# Remove container
docker rm app

# Restart container
docker restart app

# Inspect container
docker inspect app
```

### Connecting to Container

```bash
# Interactive shell
docker run -it newreleases:latest /bin/sh

# Execute command
docker exec -it app wget http://localhost:8080/

# Copy from container
docker cp app:/app/state.yaml ./state.yaml

# Copy to container
docker cp ./state.yaml app:/app/state.yaml
```

## Testing

### Tests in Build

The Dockerfile automatically runs:

```bash
go test -v -race
```

If tests fail, the build fails. This ensures:
- ✅ All functionality verified
- ✅ No race conditions
- ✅ Production-ready code

### Testing Running Container

```bash
# Check health
docker exec app wget -O- http://localhost:8080/ || echo "Unhealthy"

# Check version
docker run newreleases:latest ./newreleases --version 2>/dev/null || echo "Running..."

# Load test
docker run --network container:app alpine sh -c \
  'for i in $(seq 1 10); do wget -q -O- http://localhost:8080/api/releases; done'
```

## Security

### Best Practices

1. **Minimal Base Image**: Uses Alpine Linux (smaller attack surface)
2. **Non-root User**: Consider adding non-root user
3. **Read-only Filesystem**: Can be configured with `--read-only`
4. **Resource Limits**: Set memory/CPU limits
5. **Health Checks**: Enabled by default

### Scanning for Vulnerabilities

Using Trivy:

```bash
# Install Trivy
# Visit: https://github.com/aquasecurity/trivy

# Scan image
trivy image newreleases:latest

# Scan and fail on HIGH/CRITICAL
trivy image --severity HIGH,CRITICAL \
  --exit-code 1 newreleases:latest
```

## Performance Optimization

### Reduce Image Size

```bash
# Current sizes
- Builder stage: ~350 MB
- Final image: ~15-20 MB

# Minimize base image
FROM alpine:3.19 (minimal)

# Use distroless (even smaller)
FROM gcr.io/distroless/base-debian12:nonroot
```

### Improve Build Speed

```bash
# Enable BuildKit
export DOCKER_BUILDKIT=1

# Use multi-stage caching
docker build --cache-from newreleases:latest .

# Use build arguments for cache busting
docker build --build-arg CACHE_BUST=$(date +%s) .
```

## Deployment

### Local Deployment

```bash
# Using Docker Compose
docker-compose up -d

# Check status
docker-compose ps

# View logs
docker-compose logs -f

# Stop
docker-compose down
```

### Kubernetes Deployment

```yaml
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
        image: myregistry/newreleases:latest
        ports:
        - containerPort: 8080
        livenessProbe:
          httpGet:
            path: /
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
```

### Docker Swarm Deployment

```bash
# Join swarm
docker swarm init

# Deploy service
docker service create \
  --name newreleases \
  -p 8080:8080 \
  myregistry/newreleases:latest

# Scale service
docker service scale newreleases=3

# View service
docker service ls
```

## Troubleshooting

### Build Issues

**Problem**: Build fails with "go test" errors
```
Solution: Check main_test.go is not present in final binary
The Dockerfile runs tests, so all must pass
Run: go test -v locally first
```

**Problem**: "Cannot find Docker"
```
Solution: Install Docker or add to PATH
https://docs.docker.com/get-docker/
```

### Runtime Issues

**Problem**: Container exits immediately
```
Troubleshoot:
docker logs app
docker run -it newreleases:latest /bin/sh
```

**Problem**: Port already in use
```
Use different port:
docker run -p 8081:8080 newreleases:latest
```

**Problem**: Cannot access application
```
Check:
docker ps
docker logs app
docker inspect app | grep IPAddress
```

## Commands Reference

### Build

```bash
./build-docker.sh                              # Default
./build-docker.sh myapp v1.0.0                # Custom tag
./build-docker.sh myapp v1.0.0 docker.io/user # Push
```

### Run

```bash
docker run -p 8080:8080 newreleases:latest
docker-compose up
```

### Manage

```bash
docker ps                    # List containers
docker logs app              # View logs
docker exec -it app /bin/sh # Shell access
docker stop app              # Stop
docker rm app                # Remove
```

### Push

```bash
docker push myregistry/newreleases:latest
```

## CI/CD Integration

### GitHub Actions

```yaml
name: Build and Push Docker Image

on:
  push:
    branches: [ main ]

jobs:
  docker:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Build Docker image
        run: docker build -t newreleases:latest .
      
      - name: Login to registry
        uses: docker/login-action@v1
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      
      - name: Push image
        run: docker push ghcr.io/${{ github.actor }}/newreleases:latest
```

## FAQ

**Q: Why Alpine Linux?**
A: Minimal, secure, and fast. Perfect for containerized applications.

**Q: Why multi-stage build?**
A: Keeps final image small by excluding build tools and dependencies.

**Q: How do I persist data?**
A: Use volumes: `docker run -v state:/app/state ...`

**Q: Can I run this in Kubernetes?**
A: Yes, see Deployment section above.

**Q: Is the binary statically linked?**
A: Yes, `CGO_ENABLED=0` ensures portability.

---

**Created**: November 10, 2025
**Docker Version**: Compatible with Docker 20.10+
**Go Version**: 1.24.9
