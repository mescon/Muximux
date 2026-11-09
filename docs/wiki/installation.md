# Installation

Muximux ships as a single binary. The frontend is embedded in it. There is nothing else to install: no database, no external dependencies, no separate web server. Choose whichever installation method suits your setup.

---

## Docker (Recommended)

The Docker image is the simplest way to run Muximux. We publish it to the GitHub Container Registry.

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

The `-v $(pwd)/data:/app/data` flag maps a local `data/` directory into the container at `/app/data`. Muximux stores all persistent state here:

- **config.yaml** -- your main configuration file (created automatically on first run)
- **themes/** -- user-created custom CSS theme files
- **icons/** -- cached and uploaded icons

Without this volume mount, you lose your configuration every time you recreate the container.

### Environment variables

| Variable | Description |
|----------|-------------|
| `MUXIMUX_DATA` | Base data directory (default: `data`). In Docker, the CMD sets this to `/app/data`. |
| `MUXIMUX_CONFIG` | Override config file path (default: derived from data dir as `<data>/config.yaml`). |
| `MUXIMUX_LISTEN` | Override listen address (e.g., `:9090`). |
| `TZ` | Timezone (e.g., `America/New_York`, `Europe/London`). Defaults to `UTC`. |

These are the common ones. You can also set logging, base path, Docker discovery, the auth method and every OIDC setting from the environment. See the full [Environment Variable Reference](configuration.md#environment-variable-reference).

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

By default, Muximux uses `data/` beside the binary as its base data directory. This directory contains `config.yaml`, themes and icon caches. If the config file does not exist, Muximux starts with default settings and no apps configured.

### Command-line flags

| Flag | Default | Description |
|------|---------|-------------|
| `--data` | `data/` beside binary | Base data directory for config, themes, and icons (env: `MUXIMUX_DATA`) |
| `--config` | *(derived from data dir)* | Override config file path (env: `MUXIMUX_CONFIG`). Defaults to `<data>/config.yaml`. |
| `--listen` | from config | Override listen address (env: `MUXIMUX_LISTEN`) |
| `--base-path` | *(empty)* | Base URL path for reverse proxy subpath, e.g. `/muximux` (env: `MUXIMUX_BASE_PATH`) |
| `--version` | | Print version information and exit |

---

## Verifying a Download

Every release binary and container image published from 3.3.2 onward carries
a signed [SLSA provenance](https://slsa.dev/provenance/v1) attestation (a signed
record of how the file was built). The attestation binds the file to the exact
workflow, repository and tag that produced it. The signing key is ephemeral
(made for one use only) and never stored. GitHub's OIDC token binds the
certificate to the individual workflow run.

Verification needs the [GitHub CLI](https://cli.github.com/) and no other setup.

### Binaries

```bash
gh attestation verify muximux-linux-amd64 \
  --repo mescon/Muximux \
  --signer-workflow mescon/Muximux/.github/workflows/release.yml
```

### Container images

```bash
gh attestation verify oci://ghcr.io/mescon/muximux:3.3.2 \
  --repo mescon/Muximux \
  --signer-workflow mescon/Muximux/.github/workflows/release.yml
```

An exit status of `0` means the artifact verified. Recent `gh` versions print
nothing on success. A failure exits non-zero and says why.

**Prefer `--signer-workflow` over bare `--repo`.** On its own, `--repo` only
asserts that *some* workflow in the repository signed the artifact. Naming the
workflow asserts *which* one. So a build produced by any other workflow in the
repo fails the check, instead of passing it. This includes a workflow added by
a compromised or mistaken change.

### Checksums

`checksums.txt` comes with every release. It covers the binaries and the SBOM:

```bash
sha256sum -c checksums.txt
```

Checksums confirm that the file is intact. The attestation confirms *where it
came from*. They answer different questions, so use both.

### SBOM

`muximux-sbom.spdx.json` is an SPDX 2.3 software bill of materials (a list of
every component in the build) for the release. With it, you can answer "is this
build affected by CVE-X" without rebuilding. You can also feed it straight to a
scanner:

```bash
grype sbom:muximux-sbom.spdx.json
```

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

The `npm run build` step compiles the Svelte frontend and writes it to `internal/server/dist/`. The `go build -tags embed_web` step then embeds those files into the Go binary. The `-tags embed_web` flag is required. Without it, the binary does not include the web UI. The result is a single, self-contained executable.

> **Note:** There is no Makefile. The two-step build (frontend, then backend) is all you need.

---

## Data Directory Structure

Muximux stores all mutable data (config, themes, icon caches) in a single directory. This is true whether you use Docker or run the binary directly. In Docker, this is `/app/data` (mapped through a volume). When you run the binary, the default is `data/` relative to the binary's location (not the working directory). You can change it with `--data` or the `MUXIMUX_DATA` environment variable.

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

The main configuration file. It contains all settings: server, authentication, navigation, apps, groups, health monitoring, keyboard shortcuts, and icon settings. See the [Configuration Reference](configuration.md) for the full format.

If this file does not exist when Muximux starts, Muximux uses defaults. It shows the onboarding wizard on the first visit.

### themes/

Custom CSS theme files that you create through the Settings UI or place here manually. The built-in Muximux palette (theme ids `muximux` and `muximux-light`) and the bundled third-party themes (Catppuccin, Nord, Dracula, etc.) are compiled into the binary. They do not appear in this directory.

### icons/dashboard/

Locally cached copies of icons fetched from the [Dashboard Icons](https://github.com/homarr-labs/dashboard-icons) project. Muximux downloads them on demand and caches them according to the configured TTL (time to live, default: 7 days).

### icons/lucide/

Cached SVGs from the [Lucide](https://lucide.dev/) icon set, used for group icons and other UI elements.

### icons/custom/

Icons that users upload through the Settings UI. These can be PNG, SVG, JPG, WebP or GIF.
