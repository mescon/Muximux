# Contributing to Muximux

Thank you for considering contributing to Muximux! This guide covers development setup, building, testing, and the PR process.

## Development Tooling

The primary development environment for Muximux is Claude Code with MCP servers for browser automation, GitHub integration, and validation. All code is reviewed, tested, and committed by the sole maintainer, [@mescon](https://github.com/mescon) - AI output is treated the same way as any other pull request. Contributors are welcome to use whatever tools they prefer.

## Prerequisites

- **Go** 1.26+ (check with `go version`)
- **Node.js** 20+ with npm (check with `node --version`)
- **golangci-lint v2** (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`) -- `.golangci.yml` is a v2 config and v1 cannot parse it

## Getting Started

```bash
git clone https://github.com/mescon/Muximux.git
cd Muximux

# Enable git hooks (pre-push runs tests with coverage checks)
git config core.hooksPath .githooks
```

## Development

Muximux uses a Go backend with an embedded Svelte frontend. During development, you run them separately:

**Terminal 1 -- Backend** (serves API on :8080, falls back to `internal/server/dist/` for static files):

```bash
go run ./cmd/muximux
```

**Terminal 2 -- Frontend** (Vite dev server with hot reload, proxies API to :8080):

```bash
cd web && npm install && npm run dev
```

The backend detects when no embedded assets are present (dev mode) and automatically falls back to serving files from `internal/server/dist/` on disk. That is the same directory Vite builds into, so a `npm run build` is immediately visible to a non-embedded binary without any copy step.

## Building

### Development build (no frontend embedding)

```bash
go build -o muximux ./cmd/muximux
```

This compiles without the `embed_web` build tag, so the binary won't contain frontend assets. It serves from `internal/server/dist/` at runtime, so run `cd web && npm run build` first, or use the Vite dev server instead. Note that the path is relative to the working directory, so run the binary from the repository root.

### Production build (embedded frontend)

```bash
# Build frontend first (outputs to internal/server/dist/)
cd web && npm run build && cd ..

# Build binary with embedded assets
go build -tags embed_web -o muximux ./cmd/muximux
```

## Code Style

### Go

- Run `golangci-lint run` before submitting
- All exported identifiers must have doc comments (enforced in CI at 80% threshold)
- Follow standard Go conventions -- `gofmt` is assumed

### Svelte / TypeScript

- Run `npm run check` and `npm run lint` in `web/`
- TypeScript types live in `web/src/lib/types.ts`

## Testing

### Coverage Target

Both backend (Go) and frontend (Svelte/TypeScript) maintain **85% statement/line coverage**. This is enforced by:

- **Pre-push hook** -- blocks pushes below threshold (run `git config core.hooksPath .githooks` once to enable it)
- **Codecov** -- gates PRs at 85% on new code
- **Vitest** -- fails `npm run test:coverage` if coverage drops below threshold

Scope: the backend figure covers `./internal/...` (the `cmd/` entrypoints are
excluded). The frontend figure covers the unit-testable surface -- `src/lib`
and `src/components`. The app shell (`src/App.svelte`, `src/main.ts`) is
integration code verified by manual/browser testing and is excluded from the
percentage; see `web/vitest.config.ts`. Tests themselves are excluded from
coverage metrics.

### Backend

```bash
go test -race ./...

# With coverage report
go test -race -coverprofile=coverage.txt ./internal/...
go tool cover -func=coverage.txt | tail -1
```

### Frontend

```bash
cd web && npm run test

# With coverage report
cd web && npx vitest run --coverage
```

### Fuzzing

The proxy parses bytes that come from backend responses and from the request
line, so the parts of it that do so have fuzz targets in
`internal/handlers/reverse_proxy_fuzz_test.go`:
`FuzzResolveBackendRequestPath`, `FuzzContentRewriter`, `FuzzRewriteScript` and
`FuzzExpandHeaderValue`. The seed corpora live in `internal/handlers/testdata/fuzz/`.

They run as ordinary unit tests against the corpus during `go test`. To
actually fuzz, run one target at a time:

```bash
go test ./internal/handlers/ -run=Fuzz -fuzz=FuzzResolveBackendRequestPath -fuzztime=60s
```

If you change path joining, header expansion or the content rewriter, run the
relevant target before pushing. New crashers are written to `testdata/fuzz/`
and should be committed with the fix.

### Full pre-push check (mirrors CI)

The pre-push hook runs automatically if you configured `.githooks`. To run manually:

```bash
.githooks/pre-push
```

## Pull Request Process

1. Create a feature branch from `main`
2. Make your changes with tests
3. Ensure all checks pass locally (`go test -race ./...`, `npm run test`, `golangci-lint run`)
4. Write a clear PR description explaining the "why"
5. Keep PRs focused -- one feature or fix per PR

## Reporting a Vulnerability

Do not open a public issue or pull request for a security problem. See
[SECURITY.md](SECURITY.md) for how to report one privately.

## Commit Style

- Use imperative mood: "Add feature" not "Added feature"
- Keep the first line under 72 characters
- Add a blank line before the body if more detail is needed

## Project Structure

```
cmd/muximux/          Entry point (main.go)
internal/
  auth/               Authentication (sessions, users, OIDC, middleware)
  config/             YAML config loading and types
  discovery/          Docker container discovery and refresh polling
  handlers/           HTTP handlers (API, auth, health, icons, themes, etc.)
  health/             Health monitoring
  icons/              Icon providers (Dashboard Icons, Lucide, custom)
  logging/            Structured logging
  proxy/              Embedded Caddy reverse proxy
  server/             HTTP server, routing, middleware, embed handling
  server/dist/        Frontend build output (generated; embedded at build time)
  websocket/          WebSocket hub for real-time events
web/
  src/components/     Svelte components
  src/lib/            Stores, types, helpers, generated Paraglide locales
  src/test/           Test setup and shared fixtures
data/                 Runtime data directory (config, themes, icons)
```
