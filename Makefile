.PHONY: all build dev clean frontend backend install test

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)"

# Default target
all: build

# Install dependencies
install:
	cd web && npm install
	go mod download

# Build everything
build: frontend backend

# Build frontend
frontend:
	cd web && npm run build
	mkdir -p internal/server/dist
	cp -r web/dist/* internal/server/dist/

# Build Go backend
backend:
	go build $(LDFLAGS) -o bin/muximux ./cmd/muximux

# Development mode - run frontend and backend separately
dev:
	@echo "Starting development servers..."
	@echo "Frontend: http://localhost:5173"
	@echo "Backend:  http://localhost:8080"
	@echo ""
	@echo "Run in separate terminals:"
	@echo "  make dev-frontend"
	@echo "  make dev-backend"

dev-frontend:
	cd web && npm run dev

dev-backend:
	go run ./cmd/muximux --config config.yaml

# Run tests
test:
	go test -v ./...
	cd web && npm run check

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf web/dist/
	rm -rf internal/server/dist/

# Build for multiple platforms
release:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/muximux-linux-amd64 ./cmd/muximux
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/muximux-linux-arm64 ./cmd/muximux
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/muximux-darwin-amd64 ./cmd/muximux
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/muximux-darwin-arm64 ./cmd/muximux
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/muximux-windows-amd64.exe ./cmd/muximux

# Docker build
docker:
	docker build -t muximux3:$(VERSION) .
	docker tag muximux3:$(VERSION) muximux3:latest
