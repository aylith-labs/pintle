# local-proxy

## Project
Go-based HTTPS reverse proxy for local development (not CI/production).
Single static binary with embedded React dashboard (~9 MB, 10 MB Docker image).
Configurable via `BASE_DOMAIN` env var (default: `lvh.me`), uses mkcert certs.
Coexists with Traefik/Caddy via SNI passthrough for configured domains.

## Architecture
- **Go binary** with `//go:embed` React dashboard
- Provider pattern: Docker provider + File provider → Aggregator → Router
- SNI router on :9443, Docker handles port 443/80 mapping (or iptables/pfctl in host-native mode)
- mkcert wildcard cert for `*.${BASE_DOMAIN}` in `certs/`
- SNI-based routing: passthrough domains → target proxy (TCP, no TLS termination), `*.${BASE_DOMAIN}` → local HTTPS
- Dashboard at `proxy.${BASE_DOMAIN}` (embedded in production, Vite HMR in dev)
- Docker auto-discovery via `local-proxy.*`, `traefik.*`, and `caddy*` labels (priority: local-proxy > Traefik > Caddy)
- Traefik container IP auto-discovered via Docker API
- Static routes in `routes.yaml` (equivalent to Traefik's file provider)
- WebSocket proxying (Vite HMR)
- TCP service routing (Redis, PostgreSQL, MySQL) with TLS termination

## Docker Labels
```yaml
# Native format (preferred)
labels:
  - local-proxy.host=app.lvh.me      # required: hostname(s), comma-separated
  - local-proxy.port=5173            # optional: defaults to first EXPOSE port
  - local-proxy.path=/api            # optional: path prefix match
  - local-proxy.strip=true           # optional: strip path prefix

# Traefik format (also supported)
labels:
  - traefik.enable=true
  - traefik.http.routers.app.rule=Host(`app.lvh.me`) && PathPrefix(`/api`)
  - traefik.http.services.app.loadbalancer.server.port=5173
  - traefik.http.middlewares.app-strip.stripprefix.prefixes=/api

# Caddy format (caddy-docker-proxy compatible)
labels:
  - caddy=app.lvh.me
  - caddy.reverse_proxy={{upstreams 5173}}
  - caddy.handle_path=/api/*               # path prefix + strip
```

## Commands
- **Production**: `docker compose up -d --build` (single container, embedded UI, restart: unless-stopped)
- **Dev with HMR**: `make docker-dev` (adds Vite HMR container via compose profile)
- **Stop dev**: `make docker-dev-down` (stops both, then `docker compose up -d` for production)
- **Local dev** (requires Go on host): `make dev` + `cd ui && bun run dev` (two terminals)
- **Build binary**: `make build` (builds UI + Go binary)
- **Build binary only**: `make build-only` (skip UI rebuild)
- **Test**: `make test`
- **Lint**: `make lint`
- **Sync hosts**: `make sync-hosts` (updates Windows hosts file from current routes, requires Admin)

## Configuration
- `routes.yaml` — auto-discovered from `./routes.yaml` or `~/.config/local-proxy/routes.yaml`
- `routes.example.yaml` — template to copy to `~/.config/local-proxy/routes.yaml`
- `certs/` — mkcert wildcard certs (not committed, gitignored)
- `routes.yaml` is gitignored (machine-specific, may contain private domains)

## Key Files (Go)
- `cmd/local-proxy/main.go` — Entry point, wiring, signal handling
- `internal/config/config.go` — `BASE_DOMAIN`, `HOST_ADDRESS`, CLI flags, env vars, routes.yaml auto-discovery
- `internal/proxy/proxy.go` — HTTP reverse proxy (`httputil.ReverseProxy`)
- `internal/proxy/websocket.go` — WebSocket upgrade + bidirectional pipe
- `internal/router/router.go` — Route table (hostname+path → target, RWMutex)
- `internal/server/sni.go` — SNI router (TLS ClientHello parsing + TCP passthrough)
- `internal/server/tcp.go` — TCP service router (Redis, PostgreSQL, MySQL)
- `internal/server/https.go` — HTTPS server with dynamic TLS cert selection
- `internal/server/http.go` — HTTP → HTTPS redirect
- `internal/provider/docker/docker.go` — Docker event watcher + container discovery
- `internal/provider/docker/labels.go` — Label parsers (local-proxy, traefik, caddy)
- `internal/provider/file/file.go` — routes.yaml loader + fsnotify watcher
- `internal/aggregator/aggregator.go` — Merges provider configs, non-blocking channel
- `internal/tls/manager.go` — Certificate loading + SNI callback
- `internal/stats/stats.go` — Request metrics (circular buffer, per-host/edge)
- `internal/api/api.go` — REST API endpoints
- `internal/api/dashboard.go` — Embedded UI serving + Vite dev proxy fallback
- `internal/logger/logger.go` — Colored terminal logger

## Key Files (UI)
- `ui/` — React 19 + TypeScript + Tailwind v4 + Vite dashboard
- `routes.example.yaml` — Static routes template
- `scripts/start.sh` — port redirect rules (iptables/pfctl)
- `scripts/stop.sh` — remove port redirect rules

## Operational Notes
- Docker mode: stop Traefik/Caddy port bindings first (`docker stop traefik`), clean iptables (`sudo ./scripts/stop.sh`)
- Passthrough certs optional — if missing, warns and skips (passthrough still works when target proxy is running)
- Static route targets: port-only (`5174`) auto-resolves to `localhost` (host) or `host.docker.internal` (Docker)
- To switch back to Traefik: `docker compose down` then `docker start traefik`
- Production: single container, embedded UI, `restart: unless-stopped`
- Dev: `make docker-dev` adds Vite HMR container via compose `dev` profile
- `.dockerignore` uses Go `filepath.Match` rules (NOT gitignore) — use `**` for recursive patterns
