# local-proxy

A Bun-based HTTPS reverse proxy for local development. Routes `*.lvh.me` (configurable) domains via SNI, auto-discovers Docker containers, and supports passthrough to other proxies like Traefik.

## Architecture

```
                        iptables/pfctl: 443 -> 9443, 80 -> 9080

  :443 ──> SNI Router (:9443) ──┬──> Bun HTTPS (:9444) ──> Docker/static services
           TLS ClientHello      │    *.lvh.me              home.lvh.me -> 172.19.0.6:5173
           hostname parsing     │                          linux-settings.lvh.me -> localhost:5174
                                │
                                └──> Passthrough (TCP, no TLS termination)
                                     Configured domains -> Traefik/other proxy

  :80  ──> HTTP Redirect (:9080) ──> 301 -> https://
```

- **SNI routing** parses TLS ClientHello without terminating TLS, so passthrough targets keep their own certificates
- **Port redirection** via iptables (Linux) or pfctl (macOS) redirects standard ports to high ports, coexisting with Docker's port bindings
- **mkcert** wildcard cert for `*.lvh.me` (or custom `BASE_DOMAIN`) in `certs/`

## Prerequisites

- [Bun](https://bun.sh) runtime
- [mkcert](https://github.com/FiloSottile/mkcert) for local TLS certificates
- Docker (for container auto-discovery)
- `sudo` access (for iptables/pfctl rules)

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

### Development (no port redirection)

Two terminals:

```bash
# Terminal 1: Backend (auto-restarts on changes)
bun run dev

# Terminal 2: Dashboard UI (Vite HMR)
cd ui && bun run dev
```

Access services at `https://localhost:9444` or configure port redirection for standard ports.

### Production (with port redirection)

```bash
# Adds iptables/pfctl rules, starts proxy, cleans up on exit
bun run start

# Or manually remove rules
./scripts/stop.sh
```

With port redirection active, services are accessible at `https://home.lvh.me`, `https://proxy.lvh.me`, etc.

## Routing

### Docker auto-discovery

Containers on the Docker network (default: `traefik`) are auto-discovered via labels. Three label formats are supported:

```yaml
services:
  my-app:
    networks:
      - traefik
    labels:
      # Native format (preferred)
      local-proxy.host: my-app.lvh.me    # required: hostname (comma-separated for multiple)
      local-proxy.port: "5173"           # optional: defaults to first EXPOSE port
      local-proxy.path: /api             # optional: path prefix match
      local-proxy.strip: "true"          # optional: strip path prefix before forwarding

      # Traefik format (also supported)
      traefik.enable: "true"
      traefik.http.routers.my-app.rule: "Host(`my-app.lvh.me`) && PathPrefix(`/api`)"
      traefik.http.services.my-app.loadbalancer.server.port: "5173"
      traefik.http.middlewares.my-app-strip.stripprefix.prefixes: /api

      # Caddy format (caddy-docker-proxy compatible)
      caddy: my-app.lvh.me
      caddy.reverse_proxy: "{{upstreams 5173}}"
      caddy.handle_path: /api/*          # path prefix + strip
```

Label priority: `local-proxy.*` > `traefik.*` > `caddy`. If a container has multiple label formats, only the highest-priority one is used. Routes update automatically when containers start/stop.

### Static routes (routes.yaml)

For non-Docker services, define routes in `routes.yaml` (auto-reloaded on changes):

```yaml
routes:
  - host: linux-settings.lvh.me
    target: http://localhost:5174
```

### Passthrough domains

Domains that should be forwarded to another proxy (e.g., Traefik) without TLS termination are configured in `routes.yaml`:

```yaml
passthrough:
  - domain: internal.example
    target: traefik              # auto-discovers Traefik container IP
```

Traffic for `*.internal.example` is passed through at the TCP level — local-proxy reads the SNI hostname from the TLS ClientHello but does not decrypt the traffic. The target proxy's container IP is auto-discovered on the shared Docker network.

Passthrough domains also need mkcert certs in `certs/` (used as fallback when the target proxy is unavailable):
```bash
mkcert -cert-file certs/internal.example.pem -key-file certs/internal.example-key.pem "*.internal.example"
```

## Coexistence with Traefik / Caddy

local-proxy is designed to work **alongside** existing proxies, not replace them in production:

- **Traefik/Caddy containers keep running** — local-proxy just becomes the entry point on port 443
- **No re-labeling needed** — local-proxy reads Traefik and Caddy labels natively
- **Passthrough for separate domains** — domains managed by another proxy are forwarded at the TCP level without TLS termination
- **Gradual migration** — you can move services one at a time from Traefik/Caddy labels to `local-proxy.*` labels, or keep using the original labels indefinitely

### What local-proxy is NOT

local-proxy is a **local development tool**. It is not a replacement for Traefik or Caddy in production or CI:

- No automatic Let's Encrypt / ACME (uses mkcert for local certs only)
- No load balancing across replicas
- No health checks or circuit breaking
- No rate limiting or auth middleware
- Single-threaded Bun runtime (not designed for high concurrency)

For anything beyond local dev routing, use Traefik or Caddy directly.

## Dashboard

Live architecture visualization at `https://proxy.lvh.me`:

- **Flow diagram** - Interactive topology map (React Flow) showing SNI router, Bun HTTPS, Traefik, and all service nodes
- **Stats** - Total requests, uptime, active routes, error rate
- **Request log** - Recent requests with method, host, path, status, and duration
- **Light/dark theme** - Follows system preference with manual toggle

## Commands

| Command | Description |
|---------|-------------|
| `bun run dev` | Watch mode, backend only (no port redirection) |
| `bun run start` | Production: port redirection + proxy (requires sudo) |
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
  docker-watcher.ts  Docker event listener + container discovery (local-proxy/traefik/caddy labels)
  static-routes.ts   YAML config loader with file watcher (routes + passthrough)
  api.ts             Dashboard API + Vite dev server proxy
  stats.ts           In-memory request metrics
ui/                  React + Vite + Tailwind + React Flow dashboard
routes.yaml          Static routes + passthrough domains
scripts/
  start.sh           Add port redirect rules (iptables/pfctl) + start proxy
  stop.sh            Remove port redirect rules
certs/               mkcert wildcard certs (not committed)
```
