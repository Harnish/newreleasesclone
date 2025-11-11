#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
IMAGE_NAME="${1:-newreleases}"
IMAGE_TAG="${2:-latest}"
REGISTRY="${3:-}"
FULL_IMAGE="${REGISTRY:+$REGISTRY/}${IMAGE_NAME}:${IMAGE_TAG}"

# Helper functions
print_header() {
    echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║${NC} $1"
    echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"
}

print_step() {
    echo -e "${YELLOW}➜${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

# Main script
main() {
    print_header "Release Tracker - Docker Build"
    echo ""

    # Step 1: Check Docker
    print_step "Checking Docker installation..."
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed"
        exit 1
    fi
    print_success "Docker found: $(docker --version)"
    echo ""

    # Step 2: Verify files
    print_step "Verifying required files..."
    if [ ! -f "Dockerfile" ]; then
        print_error "Dockerfile not found"
        exit 1
    fi
    if [ ! -f "go.mod" ]; then
        print_error "go.mod not found"
        exit 1
    fi
    if [ ! -f "main.go" ]; then
        print_error "main.go not found"
        exit 1
    fi
    print_success "All required files found"
    echo ""

    # Step 3: Build Docker image
    print_step "Building Docker image: ${FULL_IMAGE}"
    if docker build -t "${FULL_IMAGE}" -f Dockerfile .; then
        print_success "Docker image built successfully"
    else
        print_error "Docker build failed"
        exit 1
    fi
    echo ""

    # Step 4: Get image info
    print_step "Image information:"
    docker images | grep "${IMAGE_NAME}" | head -1 | awk '{
        printf "  Image: %s\n  Size: %s\n  Created: %s\n", $1":"$2, $7, $4
    }'
    echo ""

    # Step 5: Test image
    print_step "Testing Docker image..."
    if docker run --rm "${FULL_IMAGE}" /app/newreleases &
    then
        sleep 2
        pkill -f "/app/newreleases" 2>/dev/null || true
        print_success "Docker image tests passed"
    else
        print_error "Docker image test failed"
        exit 1
    fi
    echo ""

    # Step 6: Scan for vulnerabilities (if trivy installed)
    if command -v trivy &> /dev/null; then
        print_step "Scanning image for vulnerabilities..."
        trivy image --severity HIGH,CRITICAL "${FULL_IMAGE}" || true
        echo ""
    fi

    # Step 7: Push to registry (if registry provided)
    if [ -n "${REGISTRY}" ]; then
        print_step "Pushing image to registry: ${REGISTRY}"
        if docker push "${FULL_IMAGE}"; then
            print_success "Image pushed successfully"
        else
            print_error "Failed to push image"
            exit 1
        fi
    else
        echo -e "${YELLOW}ℹ${NC} Registry not specified. Skipping push."
        echo "   To push, use: docker push ${FULL_IMAGE}"
    fi
    echo ""

    # Final summary
    print_header "Build Summary"
    echo -e "${GREEN}✓ Build Complete${NC}"
    echo ""
    echo "Image: ${FULL_IMAGE}"
    echo ""
    echo "Quick Commands:"
    echo "  Run:          docker run -p 8080:8080 ${FULL_IMAGE}"
    echo "  Compose:      docker-compose up"
    echo "  Push:         docker push ${FULL_IMAGE}"
    echo "  Shell:        docker run -it ${FULL_IMAGE} /bin/sh"
    echo ""
}

# Show usage
show_usage() {
    cat << EOF
Usage: $0 [IMAGE_NAME] [IMAGE_TAG] [REGISTRY]

Arguments:
  IMAGE_NAME   - Docker image name (default: newreleases)
  IMAGE_TAG    - Docker image tag (default: latest)
  REGISTRY     - Docker registry URL (optional, e.g., docker.io/myuser)

Examples:
  $0                                    # Build as newreleases:latest
  $0 release-tracker v1.0.0             # Build as release-tracker:v1.0.0
  $0 release-tracker latest myregistry  # Build and push to registry

Environment Variables:
  DOCKER_BUILDKIT - Set to 1 for faster builds (export DOCKER_BUILDKIT=1)

EOF
}

# Handle arguments
case "${1}" in
    -h|--help)
        show_usage
        exit 0
        ;;
    *)
        main
        ;;
esac
