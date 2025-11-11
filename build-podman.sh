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
    print_header "Release Tracker - Buildah/Podman Build"
    echo ""

    # Step 1: Check Buildah or Podman
    print_step "Checking Buildah/Podman installation..."
    local builder_cmd=""
    
    if command -v buildah &> /dev/null; then
        builder_cmd="buildah"
        print_success "Buildah found: $(buildah --version)"
    elif command -v podman &> /dev/null; then
        builder_cmd="podman"
        print_success "Podman found: $(podman --version)"
    else
        print_error "Neither Buildah nor Podman is installed"
        exit 1
    fi
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

    # Step 3: Build with Buildah
    if [ "$builder_cmd" = "buildah" ]; then
        build_with_buildah
    else
        build_with_podman
    fi
}

build_with_buildah() {
    # Step 3: Build Docker image with Buildah
    print_step "Building image with Buildah: ${FULL_IMAGE}"
    
    if buildah bud -t "${FULL_IMAGE}" -f Dockerfile .; then
        print_success "Buildah image built successfully"
    else
        print_error "Buildah build failed"
        exit 1
    fi
    echo ""

    # Step 4: Get image info
    print_step "Image information:"
    buildah images | grep "${IMAGE_NAME}" | head -1 | awk '{
        printf "  Image: %s\n  Size: %s\n  Created: %s\n", $1":"$2, $7, $4
    }'
    echo ""

    # Step 5: Push to registry (if registry provided)
    if [ -n "${REGISTRY}" ]; then
        print_step "Pushing image to registry: ${REGISTRY}"
        
        # Determine the transport if not already specified
        local transport=""
        if [[ "${REGISTRY}" != "localhost"* ]]; then
            transport="docker://"
        else
            transport="docker-daemon://"
        fi
        
        if buildah push "${FULL_IMAGE}" "${transport}${FULL_IMAGE}"; then
            print_success "Image pushed successfully"
        else
            print_error "Failed to push image"
            exit 1
        fi
    else
        echo -e "${YELLOW}ℹ${NC} Registry not specified. Skipping push."
        echo "   To push, use: buildah push ${FULL_IMAGE} docker://${FULL_IMAGE}"
    fi
    echo ""

    # Final summary
    print_header "Build Summary"
    echo -e "${GREEN}✓ Buildah Build Complete${NC}"
    echo ""
    echo "Image: ${FULL_IMAGE}"
    echo ""
    echo "Quick Commands:"
    echo "  Run:          buildah run ${FULL_IMAGE}"
    echo "  Inspect:      buildah inspect ${FULL_IMAGE}"
    echo "  Push:         buildah push ${FULL_IMAGE} docker://${FULL_IMAGE}"
    echo "  Delete:       buildah rmi ${FULL_IMAGE}"
    echo ""
}

build_with_podman() {
    # Step 3: Build Docker image with Podman
    print_step "Building image with Podman: ${FULL_IMAGE}"
    
    if podman build -t "${FULL_IMAGE}" -f Dockerfile .; then
        print_success "Podman image built successfully"
    else
        print_error "Podman build failed"
        exit 1
    fi
    echo ""

    # Step 4: Get image info
    print_step "Image information:"
    podman images | grep "${IMAGE_NAME}" | head -1 | awk '{
        printf "  Image: %s\n  Size: %s\n  Created: %s\n", $1":"$2, $7, $4
    }'
    echo ""

    # Step 5: Test image
    print_step "Testing Podman image..."
    if podman run --rm "${FULL_IMAGE}" /app/newreleases &
    then
        sleep 2
        pkill -f "/app/newreleases" 2>/dev/null || true
        print_success "Podman image tests passed"
    else
        print_error "Podman image test failed"
        exit 1
    fi
    echo ""

    # Step 6: Push to registry (if registry provided)
    if [ -n "${REGISTRY}" ]; then
        print_step "Pushing image to registry: ${REGISTRY}"
        
        if podman push "${FULL_IMAGE}"; then
            print_success "Image pushed successfully"
        else
            print_error "Failed to push image"
            exit 1
        fi
    else
        echo -e "${YELLOW}ℹ${NC} Registry not specified. Skipping push."
        echo "   To push, use: podman push ${FULL_IMAGE}"
    fi
    echo ""

    # Final summary
    print_header "Build Summary"
    echo -e "${GREEN}✓ Podman Build Complete${NC}"
    echo ""
    echo "Image: ${FULL_IMAGE}"
    echo ""
    echo "Quick Commands:"
    echo "  Run:          podman run -p 8080:8080 ${FULL_IMAGE}"
    echo "  Run detached: podman run -d -p 8080:8080 --name app ${FULL_IMAGE}"
    echo "  View logs:    podman logs app"
    echo "  Stop:         podman stop app"
    echo "  Inspect:      podman inspect ${FULL_IMAGE}"
    echo "  Push:         podman push ${FULL_IMAGE}"
    echo ""
}

# Show usage
show_usage() {
    cat << EOF
Usage: $0 [IMAGE_NAME] [IMAGE_TAG] [REGISTRY]

Build OCI images using Buildah or Podman (auto-detects available tool)

Arguments:
  IMAGE_NAME   - OCI image name (default: newreleases)
  IMAGE_TAG    - OCI image tag (default: latest)
  REGISTRY     - Registry URL (optional, e.g., docker.io/myuser or localhost:5000)

Examples:
  $0                                    # Build as newreleases:latest
  $0 release-tracker v1.0.0             # Build as release-tracker:v1.0.0
  $0 release-tracker latest myregistry  # Build and push to registry

Environment Variables:
  BUILDAH_FORMAT        - Set to 'docker' for Docker images (buildah only)
  PODMAN_USERNS         - User namespace mode (podman only)

Tools:
  - Buildah: https://buildah.io/
  - Podman: https://podman.io/

Note: This script auto-detects Buildah or Podman.
Buildah is preferred for building images.
Podman is preferred for running containers.

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
