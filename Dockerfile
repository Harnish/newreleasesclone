# Build stage
FROM golang:1.26-alpine AS builder

# Install dependencies
RUN apk add --no-cache git ca-certificates

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Run tests
RUN go test -v ./...

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o newreleases .

# Final stage
FROM alpine:latest

# ca-certificates for HTTPS; git for GitLab mirror sync (ephemeral clone/push)
RUN apk --no-cache add ca-certificates git

# Create app and data directories
WORKDIR /app
RUN mkdir -p /app/data

# Copy binary from builder
COPY --from=builder /build/newreleases .

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/ || exit 1

# Run the application
CMD ["./newreleases"]
