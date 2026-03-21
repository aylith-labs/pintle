# local-proxy

A Go-based HTTPS reverse proxy for local development. Single static binary (~9 MB) with an embedded React dashboard. Routes `*.lvh.me` (configurable) domains via SNI, auto-discovers Docker containers, and supports passthrough to other proxies like Traefik.

## Architecture

```
  Docker mode:  443 -> container:9443,  80 -> container:9080
  Host-native:  iptables/pfctl 443 -> 9443,  80 -> 9080

  :443 ──> SNI Router (:9443) ──┬──> HTTPS Server (:9444) ──> Docker/static services
           TLS ClientHello      │    *.lvh.me                  notes.lvh.me -> 172.19.0.6:5173
           hostname parsing     │                              app.lvh.me -> localhost:5174
                                │
                                └──> Passthrough (TCP, no TLS termination)
                                     Configured domains -> Traefik/other proxy

  :80  ──> HTTP Redirect (:9080) ──> 301 -> https://
```

- **Go binary** with `//go:embed` — dashboard UI compiled into the binary, no separate container
- **Provider pattern** — Docker and File providers push config through an aggregator to the router (Traefik-inspired architecture)
- **SNI routing** parses TLS ClientHello without terminating TLS, so passthrough targets keep their own certificates
- **Port redirection** via iptables (Linux) or pfctl (macOS) redirects standard ports to high ports
- **mkcert** wildcard cert for `*.lvh.me` (or custom `BASE_DOMAIN`) in `certs/`

## Performance

Rewritten from Bun to Go to fix connection pooling stalls under concurrent load (see [benchmark history](docs/benchmark-findings.md)).

### Throughput (ab, 1000 requests, concurrency 50)

| Target | Req/s | Failures |
|--------|-------|----------|
| Direct (localhost:5770) | 186 | 0 |
| Via Go proxy | 260 | 0 |
| Via Bun proxy (previous) | stalls after ~50 | timeout |

The Go proxy is faster than direct access due to `http.Transport` connection pooling reusing upstream connections.

### Full page load (Playwright, 250 resources, 5 runs averaged)

| Metric | Direct | Via Go proxy | Overhead |
|--------|--------|-------------|----------|
| DOMContentLoaded | 1142ms | 1227ms | +85ms (7%) |
| Load complete | 1147ms | 1232ms | +85ms (7%) |
| Network idle | 2509ms | 2604ms | +95ms (4%) |

~0.3ms per-request overhead — equivalent to Traefik/Caddy (same Go `net/http` stack, same TLS termination cost).

### Docker image

| | Bun (previous) | Go (current) |
|--|----------------|--------------|
| Image size | ~200 MB (oven/bun + node_modules) | 10 MB (FROM scratch) |
| Containers | 2 (proxy + Vite UI) | 1 (binary with embedded UI) |
| Runtime deps | Bun + node_modules | None (static binary) |

## Prerequisites

- [mkcert](https://github.com/FiloSottile/mkcert) for local TLS certificates
- Docker (for container auto-discovery and recommended Docker mode)
- [Go](https://go.dev) 1.23+ (only for building from source)
- [Bun](https://bun.sh) (only for building the dashboard UI from source)

## Setup

```bash
# Install mkcert root CA (one-time)
mkcert -install

# Generate wildcard certificate (replace lvh.me with your BASE_DOMAIN if customized)
mkcert -cert-file certs/lvh.me.pem -key-file certs/lvh.me-key.pem "*.lvh.me"

# Optional: certs for passthrough domains (used as fallback when target proxy is unavailable)
mkcert -cert-file certs/example-local.com.pem -key-file certs/example-local.com-key.pem "*.example-local.com"

# Create routes config
cp routes.example.yaml routes.yaml

# Build
make build
```

Cert filenames must match `certs/${BASE_DOMAIN}.pem` and `certs/${BASE_DOMAIN}-key.pem`. Passthrough domain certs are optional — if missing, local-proxy logs a warning and skips the fallback TLS entry.

## Usage

### Docker (recommended)

```bash
# Stop any proxy binding ports 443/80 (Traefik, Caddy, etc.)
docker stop traefik  # or: docker stop caddy

# Clean any leftover iptables rules from host-native mode
sudo ./scripts/stop.sh

# Start local-proxy
docker compose up -d
```

Runs as a daemon with `restart: unless-stopped`. Docker handles port 443/80 binding — no sudo or iptables needed. Services are accessible at `https://notes.lvh.me`, `https://proxy.lvh.me`, etc.

Traefik keeps running (without host port bindings) — local-proxy passes through `*.example-local.com` traffic to Traefik's container IP via SNI.

Rebuild after code changes:
```bash
docker compose up -d --build
```

### Host-native (alternative)

```bash
# Build and run directly
make build
./local-proxy --port-redirect

# Or use the shell script wrapper
./scripts/start.sh

# Remove port redirect rules
./scripts/stop.sh
```

### Comparison

| | Docker | Host-native |
|--|--------|-------------|
| Port 443/80 | Docker handles binding | iptables/pfctl (requires sudo) |
| Auto-start | `restart: unless-stopped` | Manual or systemd |
| Code changes | `docker compose up -d --build` | `make build && ./local-proxy` |
| Static routes | `host.docker.internal` (auto) | `localhost` (auto) |

### Development

Two terminals:

```bash
# Terminal 1: Go backend (proxies dashboard to Vite dev server)
make dev

# Terminal 2: Dashboard UI (Vite HMR)
cd ui && bun run dev
```

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

Label priority: `local-proxy.*` > `traefik.*` > `caddy*`. If a container has multiple label formats, only the highest-priority one is used. Routes update automatically when containers start/stop.

### Static routes (routes.yaml)

For non-Docker services, define routes in `routes.yaml` (auto-reloaded on changes via fsnotify):

```yaml
routes:
  - host: app.lvh.me
    target: 5174                          # port-only: auto-resolves host
  - host: remote-app.lvh.me
    target: http://192.168.1.50:3000      # full URL: used as-is
```

Port-only targets resolve to `localhost` (host-native) or `host.docker.internal` (Docker) automatically.

### Passthrough domains

Domains that should be forwarded to another proxy (e.g., Traefik) without TLS termination are configured in `routes.yaml`:

```yaml
passthrough:
  - domain: example-local.com
    target: traefik              # auto-discovers Traefik container IP
```

Traffic for `*.example-local.com` is passed through at the TCP level — local-proxy reads the SNI hostname from the TLS ClientHello but does not decrypt the traffic. The target proxy's container IP is auto-discovered on the shared Docker network.

Passthrough domains also need mkcert certs in `certs/` (used as fallback when the target proxy is unavailable):
```bash
mkcert -cert-file certs/example-local.com.pem -key-file certs/example-local.com-key.pem "*.example-local.com"
```

## Coexistence with Traefik / Caddy

local-proxy is designed to work **alongside** existing proxies, not replace them:

- **local-proxy takes over ports 443/80** — any existing proxy must stop binding these ports
- **Traefik container keeps running** — local-proxy passes through configured domains to Traefik at the TCP level
- **No re-labeling needed** — local-proxy reads Traefik and Caddy labels natively
- **Gradual migration** — move services one at a time, or keep using original labels indefinitely

To switch back to Traefik: `docker compose down` then `docker start traefik`.

### What local-proxy is NOT

local-proxy is a **local development tool**. It is not a replacement for Traefik or Caddy in production or CI:

- No automatic Let's Encrypt / ACME (uses mkcert for local certs only)
- No load balancing across replicas
- No health checks or circuit breaking
- No rate limiting or auth middleware

For anything beyond local dev routing, use Traefik or Caddy directly.

## Docs

- [Privileged Ports](docs/privileged-ports.md) — Why local-proxy uses iptables/pfctl and how other approaches compare
- [Benchmark Findings](docs/benchmark-findings.md) — Performance history: Bun stall issue and Go rewrite resolution

## Dashboard

Live architecture visualization at `https://proxy.lvh.me`:

- **Flow diagram** — Interactive topology map (React Flow) showing SNI router, HTTPS server, Traefik, and all service nodes
- **Stats** — Total requests, uptime, active routes, error rate
- **Request log** — Recent requests with method, host, path, status, and duration
- **Light/dark theme** — Follows system preference with manual toggle

## Commands

| Command | Description |
|---------|-------------|
| `make build` | Build UI + Go binary |
| `make build-only` | Build Go binary only (skip UI) |
| `make dev` | Go backend with Vite dev proxy |
| `make test` | Run Go tests |
| `make lint` | Go vet + UI lint |
| `docker compose up -d` | Docker daemon mode (recommended) |
| `docker compose up -d --build` | Rebuild and restart |
| `./local-proxy --port-redirect` | Host-native with port redirection |

## CLI Flags

```
--base-domain     Base domain for routing (default: lvh.me, env: BASE_DOMAIN)
--listen-port     HTTPS listen port (default: 9443, env: LISTEN_PORT)
--http-port       HTTP redirect port (default: 9080, env: HTTP_PORT)
--certs-dir       Path to certificates (default: ./certs, env: CERTS_DIR)
--routes-file     Path to routes.yaml (default: ./routes.yaml, env: ROUTES_FILE)
--port-redirect   Add iptables/pfctl rules on start, remove on exit
--log-level       debug, info, warn, error (default: info, env: LOG_LEVEL)
--log-format      text or json (default: text, env: LOG_FORMAT)
```

CLI flags override environment variables. Environment variables override defaults.

## Key Files

```
cmd/local-proxy/
  main.go              Entry point, provider wiring, signal handling
internal/
  config/              BASE_DOMAIN, HOST_ADDRESS, CLI flags, env vars
  proxy/               HTTP reverse proxy (httputil.ReverseProxy) + WebSocket
  router/              Route table (hostname+path -> target, RWMutex)
  server/              SNI router, HTTPS/HTTP servers, TCP router
  provider/docker/     Docker event watcher + label parsers (3 formats)
  provider/file/       routes.yaml loader + fsnotify watcher
  aggregator/          Merges provider configs (non-blocking channel)
  tls/                 Certificate loading + dynamic SNI callback
  stats/               Request metrics (circular buffer, per-host/edge)
  api/                 REST API endpoints + embedded dashboard UI
  logger/              Colored terminal logger
ui/                    React 19 + TypeScript + Tailwind v4 + Vite dashboard
routes.yaml            Static routes + passthrough domains
certs/                 mkcert wildcard certs (not committed)
scripts/               Port redirect rules (iptables/pfctl)
```
