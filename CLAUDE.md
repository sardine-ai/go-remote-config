# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build -v ./...

# Run all tests
go test -v ./...

# Run tests with race detector (required before submitting — CI runs this)
go test -race -v ./...

# Run a single test
go test -race -v ./client -run TestName
go test -race -v ./source -run TestName

# Coverage (matches CI)
go test -race -coverprofile=coverage.txt -coverpkg=./... -v github.com/sardine-ai/go-remote-config/client github.com/sardine-ai/go-remote-config/model github.com/sardine-ai/go-remote-config/source

# S3 repository tests need LocalStack running first
docker run -d -p 4566:4566 localstack/localstack:3
```

CI (`.github/workflows/go.yml`) runs on Go 1.26, brings up a LocalStack `s3` service, builds, then runs the coverage command above scoped to `client`, `model`, `source` (server package tests run separately, not included in the coverage command).

## Architecture

Module: `github.com/sardine-ai/go-remote-config`. Four packages:

- **`source`** — defines the `Repository` interface (`GetName`, `GetData`, `GetRawData`, `Refresh`) and its backends: `FileRepository`, `WebRepository`, `AwsS3Repository`, `GcpStorageRepository`, and the deprecated `GitRepository`. Every new backend must implement `Refresh()` to fetch bytes from its source, unmarshal YAML, and store the result under lock — `GetData`/`GetRawData` are read concurrently by client/server so mutations must be synchronized.
- **`model`** — `Config{Name, Data}`, the shared struct a repository's YAML unmarshals into.
- **`client`** — `Client` wraps a single `Repository`, runs a background goroutine that calls `Refresh()` on an interval, and exposes typed getters (`GetConfigString`, `GetConfigInt`, `GetConfigFloat`, `GetConfigArrayOfStrings`, `GetConfig` for structs) plus health tracking (`IsHealthy`, `GetRefreshStatus`). Also exposes package-level global functions (`client.GetConfigString`, etc.) that proxy to whichever client was last passed to `NewClient`/`SetDefaultClient` — useful for app-wide singleton config access.
- **`server`** — `Server` wraps multiple `source.Repository` instances behind HTTP. Routes: `/health`, `/ready` (no auth), `/status`, `/{repo-name}` (auth via `AuthKey` if set, constant-time compared). Supports ETag caching and graceful shutdown (`StartWithGracefulShutdown`).

Data flow: source backend → `Repository.Refresh()` fetches + unmarshals YAML into `model.Config` → stored under a mutex → `Client`/`Server` read it via `GetData`/`GetRawData`. Client and server both poll `Refresh()` on their own interval in a background goroutine tied to a `context.Context`; cancel/`Close()` stops the goroutine.

When adding a new `source.Repository` implementation, follow the existing backends' lock pattern (unmarshal outside the lock, only hold the lock while swapping the stored data — see commit history note "Unmarshal outside lock to prevent data corruption") to avoid blocking readers during network/disk I/O.
