# local-proxy

## Project
Bun-based HTTPS reverse proxy for local development only (not CI/production).
Configurable via `BASE_DOMAIN` env var (default: `lvh.me`), uses mkcert certs.
Coexists with Traefik/Caddy via SNI passthrough for configured domains.

## Architecture
- Two modes: Docker (recommended, `docker compose up -d`) or host-native (`bun run start`)
- SNI router on :9443, Docker handles port 443/80 mapping (or iptables/pfctl in host-native mode)
- mkcert wildcard cert for `*.${BASE_DOMAIN}` in `certs/` (filenames: `${BASE_DOMAIN}.pem`, `${BASE_DOMAIN}-key.pem`)
- SNI-based routing: passthrough domains → target proxy (TCP, no TLS termination), `*.${BASE_DOMAIN}` → local Bun HTTPS
- Passthrough domains configured in `routes.yaml` (not hardcoded)
- Dashboard at `proxy.${BASE_DOMAIN}`
- Docker auto-discovery via `local-proxy.*`, `traefik.*`, and `caddy*` labels (priority: local-proxy > Traefik > Caddy)
- Traefik container IP auto-discovered via Docker API
- Static routes in `routes.yaml` for non-Docker apps (equivalent to Traefik's file provider)
- WebSocket proxying for Vite HMR

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
- Docker: `docker compose up -d` (recommended, daemon mode)
- Dev: `bun run dev` (watches for changes, no port redirection)
- Start: `bun run start` (host-native, adds iptables/pfctl rules)
- Lint: `bun run lint`
- Type check: `bun run typecheck`

## Key Files
- `src/config.ts` — `BASE_DOMAIN`, `HOST_ADDRESS` (auto-detects Docker via `/.dockerenv`)
- `src/index.ts` — Entry: HTTPS + HTTP servers, WebSocket proxy
- `src/proxy.ts` — HTTP request handler
- `src/router.ts` — Route table (hostname+path -> target)
- `src/sni-router.ts` — SNI-based TCP router (TLS ClientHello parsing)
- `src/docker-watcher.ts` — Docker event listener + container discovery (local-proxy/Traefik/Caddy labels)
- `src/static-routes.ts` — YAML config loader (routes + passthrough domains)
- `routes.yaml` — Static routes + passthrough domains
- `scripts/start.sh` — port redirect rules (iptables/pfctl) + start bun
- `scripts/stop.sh` — remove port redirect rules

## Operational Notes
- Docker mode: stop Traefik/Caddy port bindings first (`docker stop traefik`), clean iptables (`sudo ./scripts/stop.sh`)
- Passthrough certs are optional — if missing, local-proxy warns and skips fallback TLS (passthrough still works when target proxy is running)
- Static route targets: port-only (`5174`) auto-resolves to `localhost` (host) or `host.docker.internal` (Docker)
- To switch back to Traefik: `docker compose down` then `docker start traefik`
