# Build stage - Frontend
FROM node:20-alpine AS frontend
WORKDIR /app/web

# Cache npm dependencies
COPY web/package*.json ./
RUN npm ci --no-audit

# Build frontend
COPY web/ ./
RUN npm run build

# Build stage - Backend
FROM golang:1.25-alpine AS backend
WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Cache Go dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Copy frontend build
COPY --from=frontend /app/web/dist ./internal/server/dist

# Build binary
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -o /muximux ./cmd/muximux

# Final stage - minimal runtime
FROM alpine:3.20

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata wget

# Create non-root user
RUN addgroup -g 1000 muximux && \
    adduser -D -u 1000 -G muximux muximux

# Create data directory
RUN mkdir -p /app/data && chown -R muximux:muximux /app

USER muximux
WORKDIR /app

# Copy binary
COPY --from=backend /muximux ./muximux

# Data directory for config, icons, etc.
VOLUME /app/data

# Default ports: main UI and proxy
EXPOSE 8080 8443

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/health || exit 1

ENTRYPOINT ["./muximux"]
CMD ["--config", "/app/data/config.yaml"]
