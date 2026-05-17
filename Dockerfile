# Automatic platform args. buildx sets these per target; the
# defaults keep a plain `docker build` (no buildx, no --platform)
# working on the host that runs it.
ARG BUILDPLATFORM=linux/amd64
ARG TARGETPLATFORM=linux/amd64
ARG TARGETOS=linux
ARG TARGETARCH=amd64

# Build stage - Frontend
#
# Pinned to $BUILDPLATFORM so the npm install + vite build runs on
# the runner's native architecture (typically amd64) rather than
# under QEMU emulation when assembling a multi-arch image. The
# bundled JS output is platform-independent, so the same dist/
# directory ships to every target arch.
FROM --platform=$BUILDPLATFORM node:25-alpine AS frontend
WORKDIR /app/web

# Cache npm dependencies
COPY web/package*.json ./
RUN npm ci --no-audit --ignore-scripts

# Build frontend
COPY web/ ./
RUN npm run build

# Build stage - Backend
#
# Same trick: build natively on $BUILDPLATFORM and cross-compile
# to $TARGETOS/$TARGETARCH via Go's built-in cross-compiler. With
# CGO_ENABLED=0 the binary is statically linked, so this is far
# faster than emulating the whole Go toolchain under QEMU for
# each target arch.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS backend
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Cache Go dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ ./cmd/
COPY internal/ ./internal/

# Copy frontend build
COPY --from=frontend /app/internal/server/dist ./internal/server/dist

# Build binary. TARGETOS / TARGETARCH are populated automatically
# by buildx for each entry in --platform; defaults keep a plain
# `docker build` (no buildx, no --platform) working on amd64.
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown
ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" go build \
    -tags embed_web \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -o /muximux ./cmd/muximux

# Final stage - minimal runtime
FROM alpine:3.23

# Install runtime dependencies and create data directory
RUN apk add --no-cache ca-certificates shadow su-exec tzdata wget && \
    mkdir -p /app/data

WORKDIR /app

# Copy binary and entrypoint
COPY --from=backend /muximux ./muximux
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# PUID/PGID for bind-mount permission matching (linuxserver.io convention)
ENV PUID=1000 PGID=1000

# Data directory for config, icons, etc.
VOLUME /app/data

# Default port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/health || exit 1

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["./muximux", "--data", "/app/data"]
