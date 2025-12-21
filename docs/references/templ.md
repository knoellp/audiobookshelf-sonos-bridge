# Templ Reference

Templ is an HTML templating language for Go that provides excellent developer tooling and a clean syntax for building dynamic web interfaces.

## Overview

### Key Features

- **Server-side rendering**: Deploy as a serverless function, Docker container, or standard Go program
- **Static rendering**: Create static HTML files to deploy however you choose
- **Compiled code**: Components are compiled into performant Go code
- **No JavaScript**: Does not require any client or server-side JavaScript
- **Great developer experience**: Ships with IDE autocompletion

## Installation

### Global Installation with Go

```bash
go install github.com/a-h/templ/cmd/templ@latest
```

### Local Tool Installation (Go 1.24+)

```bash
go get -tool github.com/a-h/templ/cmd/templ@latest
```

### Using Nix

```bash
# Run directly
nix run github:a-h/templ

# Development shell
nix develop github:a-h/templ
```

### Using Docker

```bash
docker run -v `pwd`:/app -w=/app ghcr.io/a-h/templ:latest generate
```

## Multi-stage Dockerfile

```dockerfile
# Fetch
FROM golang:latest AS fetch-stage
COPY go.mod go.sum /app
WORKDIR /app
RUN go mod download

# Generate
FROM ghcr.io/a-h/templ:latest AS generate-stage
COPY --chown=65532:65532 . /app
WORKDIR /app
RUN ["templ", "generate"]

# Build
FROM golang:latest AS build-stage
COPY --from=generate-stage /app /app
WORKDIR /app
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/app

# Test
FROM build-stage AS test-stage
RUN go test -v ./...

# Deploy
FROM gcr.io/distroless/base-debian12 AS deploy-stage
WORKDIR /
COPY --from=build-stage /app/app /app
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app"]
```

## Code Examples

### HTTP Server Setup

```go
package main

import (
    "fmt"
    "net/http"
    "os"
    "time"

    "github.com/a-h/templ/examples/counter/db"
    "github.com/a-h/templ/examples/counter/handlers"
    "github.com/a-h/templ/examples/counter/services"
    "github.com/a-h/templ/examples/counter/session"
    "golang.org/x/exp/slog"
)

func main() {
    log := slog.New(slog.NewJSONHandler(os.Stderr))
    s, err := db.NewCountStore(os.Getenv("TABLE_NAME"), os.Getenv("AWS_REGION"))
    if err != nil {
        log.Error("failed to create store", slog.Any("error", err))
        os.Exit(1)
    }
    cs := services.NewCount(log, s)
    h := handlers.New(log, cs)

    var secureFlag = true
    if os.Getenv("SECURE_FLAG") == "false" {
        secureFlag = false
    }

    // Add session middleware.
    sh := session.NewMiddleware(h, session.WithSecure(secureFlag))

    server := &http.Server{
        Addr:         "localhost:9000",
        Handler:      sh,
        ReadTimeout:  time.Second * 10,
        WriteTimeout: time.Second * 10,
    }

    fmt.Printf("Listening on %v\n", server.Addr)
    server.ListenAndServe()
}
```

### GoFiber Integration

```go
go run .
```

## Development

### Build and Install Current Snapshot

```bash
# Remove templ from the non-standard ~/bin/templ path
rm -f ~/bin/templ

# Clear LSP logs
rm -f cmd/templ/lspcmd/*.txt

# Update version
version set --template="0.3.%d"

# Install to $GOPATH/bin or $HOME/go/bin
cd cmd/templ && go install
```

### Install Documentation Dependencies

```bash
yarn
```

## Best Practices

### Render Once

> **Warning**: Don't write `@templ.NewOnceHandle().Once()` - this creates a new `*OnceHandler` each time the `Once` method is called, and will result in the content being rendered multiple times.

### Project Structure

> **Tip**: As with most things, taking the layering approach to an extreme level can have a negative effect. Ask yourself whether what you're doing is really helping to make the code understandable, or is just spreading application logic across lots of files.

### Forms and Validation

> **Tip**: The [Hypermedia Systems](https://hypermedia.systems/) book covers the main concepts of building web applications. If you're new to web development, or have only ever used JavaScript frameworks, it may be worth reading.

## Resources

- [Templ Documentation](https://templ.guide/)
- [Go Time Podcast: Go templating using Templ](https://changelog.com/gotime/291)
- [Hypermedia Systems Book](https://hypermedia.systems/)
