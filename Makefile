BINARY = local-proxy
VERSION = $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -s -w -X main.version=$(VERSION)
GO = go

.PHONY: all ui build test lint clean dev docker docker-dev docker-dev-down

all: build

ui:
	cd ui && bun install && bun run build
	rm -rf internal/api/ui/dist
	cp -r ui/dist internal/api/ui/dist

build: ui
	CGO_ENABLED=0 $(GO) build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/local-proxy

build-only:
	CGO_ENABLED=0 $(GO) build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/local-proxy

test:
	$(GO) test ./...

lint:
	$(GO) vet ./...
	cd ui && bun run lint

clean:
	rm -f $(BINARY)
	rm -rf internal/api/ui/dist

dev:
	VITE_DEV_URL=http://localhost:5175 $(GO) run ./cmd/local-proxy

docker:
	docker compose up -d --build

docker-dev:
	VITE_DEV_URL=http://ui:5175 docker compose --profile dev up -d --build

docker-dev-down:
	docker compose --profile dev down
