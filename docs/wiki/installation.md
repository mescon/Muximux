# Installation

Muximux is distributed as a single binary with the frontend embedded. There is nothing else to install -- no database, no external dependencies, no separate web server. Choose whichever installation method suits your setup.

---

## Docker (Recommended)

The Docker image is the simplest way to run Muximux. It is published to the GitHub Container Registry.

```bash
mkdir -p data
docker run -d \
  --name muximux \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  --restart unless-stopped \
  ghcr.io/mescon/muximux:latest
```

Then open `http://your-server:8080` in your browser.

### What the volume mount does

The `-v $(pwd)/data:/app/data` flag maps a local `data/` directory into the container at `/app/data`. This is where Muximux stores all persistent state:

- **config.yaml** -- your main configuration file (created automatically on first run)
- **themes/** -- user-created custom CSS theme files
- **icons/** -- cached and uploaded icons

Without this volume mount, your configuration would be lost every time the container is recreated.

### Environment variables

| Variable | Description |
|----------|-------------|
| `MUXIMUX_DATA` | Base data directory (default: `data`). In Docker, the CMD sets this to `/app/data`. |
| `MUXIMUX_CONFIG` | Override config file path (default: derived from data dir as `<data>/config.yaml`). |
| `MUXIMUX_LISTEN` | Override listen address (e.g., `:9090`). |
| `TZ` | Timezone (e.g., `America/New_York`, `Europe/London`). Defaults to `UTC`. |

---

## Docker Compose

For a more declarative setup, use Docker Compose.

```yaml
services:
  muximux:
    image: ghcr.io/mescon/muximux:latest
    container_name: muximux
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
    environment:
      - TZ=UTC
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/api/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s
```

Start it with:

```bash
docker compose up -d
```

---

## Binary

Download a pre-built binary from the [Releases](https://github.com/mescon/Muximux/releases) page, or build from source (see below).

Run it directly:

```bash
./muximux
```

By default, Muximux uses `data/` beside the binary as its base data directory, which contains `config.yaml`, themes, and icon caches. If the config file does not exist, Muximux starts with default settings and no apps configured.

### Command-line flags

| Flag | Default | Description |
|------|---------|-------------|
| `--data` | `data/` beside binary | Base data directory for config, themes, and icons (env: `MUXIMUX_DATA`) |
| `--config` | *(derived from data dir)* | Override config file path (env: `MUXIMUX_CONFIG`). Defaults to `<data>/config.yaml`. |
| `--listen` | from config | Override listen address (env: `MUXIMUX_LISTEN`) |
| `--base-path` | *(empty)* | Base URL path for reverse proxy subpath, e.g. `/muximux` (env: `MUXIMUX_BASE_PATH`) |
| `--version` | | Print version information and exit |

---

## Building from Source

Building from source requires:

- **Go 1.26** or newer
- **Node.js 20** or newer (with npm)

```bash
git clone https://github.com/mescon/Muximux.git
cd Muximux

# 1. Build the frontend
cd web
npm install
npm run build
cd ..

# 2. Build the binary (the frontend is embedded at compile time)
go build -tags embed_web -o muximux ./cmd/muximux

# 3. Run (uses data/ directory by default)
./muximux
```

The `npm run build` step compiles the Svelte frontend and outputs it to `internal/server/dist/`. The `go build -tags embed_web` step then embeds those files into the Go binary. The `-tags embed_web` flag is required -- without it, the binary will not include the web UI. The result is a single, self-contained executable.

> **Note:** There is no Makefile. The two-step build (frontend, then backend) is all that is needed.

---

## Data Directory Structure

Whether you use Docker or run the binary directly, Muximux stores all mutable data (config, themes, icon caches) in a single directory. In Docker, this is `/app/data` (mapped via volume). When running the binary, this defaults to `data/` relative to the binary's location (not the working directory). You can change it with `--data` or the `MUXIMUX_DATA` environment variable.

```
data/
  config.yaml              Main configuration file
  themes/                  User-created custom CSS themes
  icons/
    dashboard/             Cached icons from Dashboard Icons
    lucide/                Cached Lucide icon SVGs
    custom/                User-uploaded icon files
```

### config.yaml

The main configuration file. Contains all settings: server, authentication, navigation, apps, groups, health monitoring, keyboard shortcuts, and icon settings. See the [Configuration Reference](configuration.md) for the full format.

If this file does not exist when Muximux starts, defaults are used and the onboarding wizard is shown on first visit.

### themes/

Custom CSS theme files created through the Settings UI or placed here manually. Built-in themes (dark and light) are bundled into the binary and do not appear in this directory.

### icons/dashboard/

Locally cached copies of icons fetched from the [Dashboard Icons](https://github.com/homarr-labs/dashboard-icons) project. These are downloaded on demand (or prefetched, depending on your configuration) and cached according to the configured TTL (default: 7 days).

### icons/lucide/

Cached SVGs from the [Lucide](https://lucide.dev/) icon set, used for group icons and other UI elements.

### icons/custom/

Icons uploaded by the user through the Settings UI. These can be PNG, SVG, or any other image format.
