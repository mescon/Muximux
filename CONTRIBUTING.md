# Contributing to Muximux

Thank you for considering contributing to Muximux! This guide covers development setup, building, testing, and the PR process.

## Prerequisites

- **Go** 1.26+ (check with `go version`)
- **Node.js** 20+ with npm (check with `node --version`)
- **golangci-lint** (`go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`)

## Getting Started

```bash
git clone https://github.com/mescon/Muximux.git
cd Muximux

# Enable git hooks (pre-push runs tests with coverage checks)
git config core.hooksPath .githooks
```

## Development

Muximux uses a Go backend with an embedded Svelte frontend. During development, you run them separately:

**Terminal 1 — Backend** (serves API on :8080, falls back to `web/dist/` for static files):

```bash
go run ./cmd/muximux
```

**Terminal 2 — Frontend** (Vite dev server with hot reload, proxies API to :8080):

```bash
cd web && npm install && npm run dev
```

The backend detects when no embedded assets are present (dev mode) and automatically falls back to serving files from `web/dist/` on disk.

## Building

### Development build (no frontend embedding)

```bash
go build -o muximux ./cmd/muximux
```

This compiles without the `embed_web` build tag, so the binary won't contain frontend assets. It will serve from `web/dist/` at runtime (you need to build the frontend separately or use the Vite dev server).

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
- Follow standard Go conventions — `gofmt` is assumed

### Svelte / TypeScript

- Run `npm run check` and `npm run lint` in `web/`
- TypeScript types live in `web/src/lib/types.ts`

## Testing

### Backend

```bash
go test -race ./...
```

### Frontend

```bash
cd web && npm run test
```

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
5. Keep PRs focused — one feature or fix per PR

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
  handlers/           HTTP handlers (API, auth, health, icons, themes, etc.)
  health/             Health monitoring
  icons/              Icon providers (Dashboard Icons, Lucide, custom)
  logging/            Structured logging
  proxy/              Embedded Caddy reverse proxy
  server/             HTTP server, routing, middleware, embed handling
  websocket/          WebSocket hub for real-time events
web/
  src/lib/            Svelte components, stores, types
  src/routes/         SvelteKit pages
data/                 Runtime data directory (config, themes, icons)
```
