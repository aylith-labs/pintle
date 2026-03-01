# local-proxy

## Project
Bun-based HTTPS reverse proxy for `*.lvh.me` (configurable via `BASE_DOMAIN` env var) using mkcert certs.
Replaces Traefik for personal projects only. Work apps continue using Traefik.

## Architecture
- SNI router on :9443, port redirection via iptables (Linux) or pfctl (macOS)
- mkcert wildcard cert for `*.${BASE_DOMAIN}` in `certs/` (filenames: `${BASE_DOMAIN}.pem`, `${BASE_DOMAIN}-key.pem`)
- SNI-based routing: `*.internal.example` → Traefik container IP (passthrough), `*.${BASE_DOMAIN}` → local Bun HTTPS
- Dashboard at `proxy.${BASE_DOMAIN}` (derived from `BASE_DOMAIN`)
- Docker auto-discovery via `proxy.*`, `traefik.*`, and `caddy` labels (priority: proxy > traefik > caddy)
- Traefik container IP auto-discovered via Docker API
- Static routes in `routes.yaml` for non-Docker apps (equivalent to Traefik's file provider)
- WebSocket proxying for Vite HMR

## Docker Labels
```yaml
# Native format (preferred)
labels:
  - proxy.host=app.lvh.me          # required: hostname(s), comma-separated
  - proxy.port=5173                # optional: defaults to first EXPOSE port
  - proxy.path=/api                # optional: path prefix match
  - proxy.strip=true               # optional: strip path prefix

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
- Dev: `bun run dev` (watches for changes, no port redirection)
- Start: `bun run start` (adds iptables/pfctl rules, runs proxy)
- Lint: `bun run lint`
- Type check: `bun run typecheck`

## Key Files
- `src/config.ts` — `BASE_DOMAIN` and derived constants
- `src/index.ts` — Entry: HTTPS + HTTP servers, WebSocket proxy
- `src/proxy.ts` — HTTP request handler
- `src/router.ts` — Route table (hostname+path -> target)
- `src/sni-router.ts` — SNI-based TCP router (TLS ClientHello parsing)
- `src/docker-watcher.ts` — Docker event listener + container discovery (proxy/traefik/caddy labels)
- `src/static-routes.ts` — YAML config loader
- `routes.yaml` — Static routes for non-Docker apps
- `scripts/start.sh` — port redirect rules (iptables/pfctl) + start bun
- `scripts/stop.sh` — remove port redirect rules
