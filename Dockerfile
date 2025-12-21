# Build stage
FROM golang:1.22-alpine AS build

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files first for better layer caching
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bridge ./cmd/bridge

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ffmpeg \
    ca-certificates \
    tzdata

# Create non-root user
RUN addgroup -g 1000 bridge && \
    adduser -u 1000 -G bridge -s /bin/sh -D bridge

# Create directories
RUN mkdir -p /media /cache /config && \
    chown -R bridge:bridge /media /cache /config

# Copy binary from build stage
COPY --from=build /bridge /usr/local/bin/bridge

# Copy web assets
COPY --chown=bridge:bridge web/ /app/web/

WORKDIR /app

# Switch to non-root user
USER bridge

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the binary
ENTRYPOINT ["/usr/local/bin/bridge"]
