# local-proxy

A Bun-based HTTPS reverse proxy for local development. Routes `*.lvh.me` (configurable) domains via SNI, auto-discovers Docker containers, and coexists with Traefik for `*.internal.example`.

## Architecture

```
                        iptables: 443 -> 9443, 80 -> 9080

  :443 ──> SNI Router (:9443) ──┬──> Bun HTTPS (:9444) ──> Docker/static services
           TLS ClientHello      │    *.lvh.me              home.lvh.me -> 172.19.0.6:5173
           hostname parsing     │                          linux-settings.lvh.me -> localhost:5174
                                │
                                └──> Traefik (TCP passthrough)
                                     *.internal.example -> 172.19.0.2:443

  :80  ──> HTTP Redirect (:9080) ──> 301 -> https://
```

- **SNI routing** parses TLS ClientHello without terminating TLS, so Traefik keeps its own certificates
- **Port redirection** via iptables (Linux) or pfctl (macOS) redirects standard ports to high ports, coexisting with Docker's port bindings
- **mkcert** wildcard cert for `*.lvh.me` (or custom `BASE_DOMAIN`) in `certs/`

## Prerequisites

- [Bun](https://bun.sh) runtime
- [mkcert](https://github.com/FiloSottile/mkcert) for local TLS certificates
- Docker (for container auto-discovery)
- `sudo` access (for iptables rules in production mode)

## Setup

```bash
# Generate wildcard certificate (replace lvh.me with your BASE_DOMAIN if customized)
mkcert -install
mkcert -cert-file certs/lvh.me.pem -key-file certs/lvh.me-key.pem "*.lvh.me"

# Install dependencies
bun install
cd ui && bun install
```

Cert filenames must match `certs/${BASE_DOMAIN}.pem` and `certs/${BASE_DOMAIN}-key.pem`.

## Usage

### Development (no iptables)

Two terminals:

```bash
# Terminal 1: Backend (auto-restarts on changes)
bun run dev

# Terminal 2: Dashboard UI (Vite HMR)
cd ui && bun run dev
```

Access services at `https://localhost:9444` or configure iptables for standard ports.

### Production (with iptables)

```bash
# Adds iptables rules, starts proxy, cleans up on exit
bun run start

# Or manually remove iptables rules
./scripts/stop.sh
```

With iptables active, services are accessible at `https://home.lvh.me`, `https://proxy.lvh.me`, etc.

## Routing

### Docker auto-discovery

Containers on the Docker network (default: `traefik`) with `proxy.*` labels are auto-discovered:

```yaml
services:
  my-app:
    networks:
      - traefik
    labels:
      proxy.host: my-app.lvh.me       # required: hostname (comma-separated for multiple)
      proxy.port: "5173"               # optional: defaults to first EXPOSE port
      proxy.path: /api                 # optional: path prefix match
      proxy.strip: "true"             # optional: strip path prefix before forwarding
```

Containers with Traefik labels (`traefik.enable=true`) or Caddy labels (`caddy`) are also auto-discovered:

```yaml
# Traefik labels
labels:
  traefik.enable: "true"
  traefik.http.routers.my-app.rule: "Host(`my-app.lvh.me`) && PathPrefix(`/api`)"
  traefik.http.services.my-app.loadbalancer.server.port: "5173"
  traefik.http.middlewares.my-app-strip.stripprefix.prefixes: /api

# Caddy labels (caddy-docker-proxy format)
labels:
  caddy: my-app.lvh.me
  caddy.reverse_proxy: "{{upstreams 5173}}"
  caddy.handle_path: /api/*                # path prefix + strip
```

Label priority: `proxy.*` > `traefik.*` > `caddy`. Routes update automatically when containers start/stop.

### Static routes (routes.yaml)

For non-Docker services, define routes in `routes.yaml` (auto-reloaded on changes):

```yaml
routes:
  - host: linux-settings.lvh.me
    target: http://localhost:5174
```

### Traefik coexistence

`*.internal.example` traffic is passed through to Traefik at the TCP level (no TLS termination). The Traefik container IP is auto-discovered on the shared Docker network.

## Dashboard

Live architecture visualization at `https://proxy.lvh.me`:

- **Flow diagram** - Interactive topology map (React Flow) showing SNI router, Bun HTTPS, Traefik, and all service nodes
- **Stats** - Total requests, uptime, active routes, error rate
- **Request log** - Recent requests with method, host, path, status, and duration
- **Light/dark theme** - Follows system preference with manual toggle

## Commands

| Command | Description |
|---------|-------------|
| `bun run dev` | Watch mode, backend only (no iptables) |
| `bun run start` | Production: iptables + proxy (requires sudo) |
| `bun run lint` | Format and lint with Biome |
| `bun run typecheck` | TypeScript strict mode check |
| `cd ui && bun run dev` | Dashboard UI with Vite HMR |
| `cd ui && bun run build` | Build dashboard for static serving |

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `BASE_DOMAIN` | `lvh.me` | Base domain for routing (`*.lvh.me`, `*.localtest.me`, etc.) |
| `LISTEN_PORT` | `9443` | SNI router listen port |
| `HTTP_PORT` | `9080` | HTTP redirect listen port |
| `DOCKER_NETWORK` | `traefik` | Docker network for container discovery |

## Key Files

```
src/
  config.ts          BASE_DOMAIN and derived constants
  index.ts           Entry: HTTPS + HTTP servers, WebSocket proxy
  sni-router.ts      SNI-based TCP router (TLS ClientHello parsing)
  proxy.ts           HTTP reverse proxy with timing instrumentation
  router.ts          Route table (hostname+path -> target)
  docker-watcher.ts  Docker event listener + Traefik IP discovery
  static-routes.ts   YAML config loader with file watcher
  api.ts             Dashboard API + Vite dev server proxy
  stats.ts           In-memory request metrics
ui/                  React + Vite + Tailwind + React Flow dashboard
routes.yaml          Static routes for non-Docker apps
scripts/
  start.sh           Add port redirect rules (iptables/pfctl) + start proxy
  stop.sh            Remove port redirect rules
certs/               mkcert wildcard certs (not committed)
```
