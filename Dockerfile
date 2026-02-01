# Build stage - Frontend
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Build stage - Backend
FROM golang:1.22-alpine AS backend
WORKDIR /app
RUN apk add --no-cache git make

# Download Go dependencies
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
RUN CGO_ENABLED=0 go build \
    -ldflags "-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -o /muximux ./cmd/muximux

# Final stage
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN adduser -D -u 1000 muximux
USER muximux

WORKDIR /app
COPY --from=backend /muximux ./muximux

# Data directory for icons, config, etc.
VOLUME /app/data

# Default port
EXPOSE 8080

ENTRYPOINT ["./muximux"]
CMD ["--config", "/app/data/config.yaml"]
