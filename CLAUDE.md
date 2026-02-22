# local-proxy

## Project
Bun-based HTTPS reverse proxy for `*.lvh.me` domains using mkcert certs.
Replaces Traefik for personal projects only. Work apps continue using Traefik.

## Architecture
- SNI router on :9443, iptables redirects 443→9443 and 80→9080
- mkcert wildcard cert for `*.lvh.me` in `certs/`
- SNI-based routing: `*.internal.example` → Traefik container IP (passthrough), `*.lvh.me` → local Bun HTTPS
- Docker auto-discovery via `proxy.*` labels on containers
- Traefik container IP auto-discovered via Docker API
- Static routes in `routes.yaml` for non-Docker apps (equivalent to Traefik's file provider)
- WebSocket proxying for Vite HMR

## Docker Labels
```yaml
labels:
  - proxy.host=app.lvh.me          # required: hostname(s), comma-separated
  - proxy.port=5173                # optional: defaults to first EXPOSE port
  - proxy.path=/api                # optional: path prefix match
  - proxy.strip=true               # optional: strip path prefix
```

## Commands
- Dev: `bun run dev` (watches for changes, no iptables)
- Start: `bun run start` (adds iptables rules, runs proxy)
- Lint: `bun run lint`
- Type check: `bun run typecheck`

## Key Files
- `src/index.ts` — Entry: HTTPS + HTTP servers, WebSocket proxy
- `src/proxy.ts` — HTTP request handler
- `src/router.ts` — Route table (hostname+path -> target)
- `src/sni-router.ts` — SNI-based TCP router (TLS ClientHello parsing)
- `src/docker-watcher.ts` — Docker event listener + container/Traefik discovery
- `src/static-routes.ts` — YAML config loader
- `routes.yaml` — Static routes for non-Docker apps
- `scripts/start.sh` — iptables rules + start bun
- `scripts/stop.sh` — remove iptables rules
