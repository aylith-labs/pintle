# local-proxy

## Project
Bun-based HTTPS reverse proxy for `*.lvh.me` domains using mkcert certs.
Replaces Traefik for personal projects only. Work apps continue using Traefik.

## Architecture
- HTTPS on :443, HTTP on :80 (redirect)
- mkcert wildcard cert for `*.lvh.me` in `certs/`
- Docker auto-discovery via `proxy.*` labels on containers
- Static routes in `routes.yaml` for non-Docker apps
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
- Dev: `bun run dev` (watches for changes)
- Start: `bun run start`
- Lint: `bun run lint`
- Type check: `bun run typecheck`

## Key Files
- `src/index.ts` — Entry: HTTPS + HTTP servers, WebSocket proxy
- `src/proxy.ts` — HTTP request handler
- `src/router.ts` — Route table (hostname+path -> target)
- `src/docker-watcher.ts` — Docker event listener + container discovery
- `src/static-routes.ts` — YAML config loader
- `routes.yaml` — Static routes for non-Docker apps
