# pintle

The pin a gate turns on. A Go HTTPS reverse proxy that replaces Traefik and Caddy — it reads **their** labels and **their** config files, so services keep the labels they already have and the proxy underneath them changes. Single static binary (~9 MB) with an embedded React dashboard. Routes domains via SNI with HTTP/2, auto-discovers Docker containers through three label dialects, terminates TLS for database connections, and passes any domain it does not own through to another proxy untouched.

## Architecture

```
  Docker mode:  443 -> container:9443,  80 -> container:9080
  Host-native:  iptables/pfctl 443 -> 9443,  80 -> 9080

  :443 ──> SNI Router (:9443) ──┬──> HTTPS Server (:9444, h2 + http/1.1) ──> Docker/static services
           TLS ClientHello      │    *.lvh.me                                notes.lvh.me -> 172.19.0.6:5173
           hostname parsing     │                                            app.lvh.me   -> localhost:5174
                                │
                                └──> Passthrough (TCP, no TLS termination)
                                     Configured domains -> Traefik/other proxy

  :80  ──> HTTP Redirect (:9080) ──> 301 -> https://

  TCP Router (per port) ──> TLS terminate (SNI) ──> plaintext -> database container
  db.lvh.me:5432/6379/3306                          postgres / redis / mysql
```

When no passthrough domains are configured, the SNI router is skipped and the HTTPS server binds `:9443` directly (the `:9444` indirection only exists to share port 443 with Traefik).

- **Go binary** with `//go:embed` — dashboard UI compiled into the binary, no separate container
- **Provider pattern** — Docker and File providers push config through an aggregator to the router (Traefik-inspired architecture)
- **SNI routing** parses TLS ClientHello without terminating TLS, so passthrough targets keep their own certificates
- **HTTP/2** — the HTTPS server advertises `h2` via ALPN; upstreams can opt into HTTP/2 cleartext (`h2c`, e.g. gRPC) per route
- **TCP routing** — terminates TLS for database connections (PostgreSQL, Redis, MySQL) and forwards plaintext to the container
- **Host header preserved** — the client `Host` is forwarded upstream (matches Traefik/Caddy defaults) so backends that build URLs from the request host work
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

# Create routes config (auto-discovered by the binary)
mkdir -p ~/.config/pintle
cp routes.example.yaml ~/.config/pintle/routes.yaml

# Build
make build
```

Cert filenames must match `certs/${BASE_DOMAIN}.pem` and `certs/${BASE_DOMAIN}-key.pem`. Passthrough domain certs are optional — if missing, pintle logs a warning and skips the fallback TLS entry.

## Usage

### Docker (recommended)

```bash
# Stop any proxy binding ports 443/80 (Traefik, Caddy, etc.)
docker stop traefik  # or: docker stop caddy

# Clean any leftover iptables rules from host-native mode
sudo ./scripts/stop.sh

# Start pintle
docker compose up -d
```

Runs as a daemon with `restart: unless-stopped`. Docker handles port 443/80 binding — no sudo or iptables needed. Services are accessible at `https://notes.lvh.me`, `https://pintle.lvh.me`, etc.

Traefik keeps running (without host port bindings) — pintle passes through `*.example-local.com` traffic to Traefik's container IP via SNI.

Rebuild after code changes:
```bash
docker compose up -d --build
```

### Host-native (alternative)

```bash
# Build and run directly
make build
./pintle --port-redirect

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
| Code changes | `docker compose up -d --build` | `make build && ./pintle` |
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
      pintle.host: my-app.lvh.me    # required: hostname (comma-separated for multiple)
      pintle.port: "5173"           # optional: defaults to first EXPOSE port
      pintle.path: /api             # optional: path prefix match
      pintle.strip: "true"          # optional: strip path prefix before forwarding

      # Traefik format (also supported)
      traefik.enable: "true"
      traefik.http.routers.my-app.rule: "Host(`my-app.lvh.me`) && PathPrefix(`/api`)"
      traefik.http.services.my-app.loadbalancer.server.port: "5173"
      traefik.http.services.my-app.loadbalancer.server.scheme: h2c   # optional: HTTP/2-cleartext upstream (gRPC)
      traefik.http.middlewares.my-app-strip.stripprefix.prefixes: /api

      # Caddy format (caddy-docker-proxy compatible)
      caddy: my-app.lvh.me
      caddy.reverse_proxy: "{{upstreams 5173}}"
      caddy.handle_path: /api/*          # path prefix + strip
```

Label priority: `pintle.*` > `traefik.*` > `caddy*`. If a container has multiple label formats, only the highest-priority one is used. Routes update automatically when containers start/stop.

Traefik rules are parsed for `Host(...)`, `HostRegexp(...)` (regex hostnames), and `PathPrefix(...)`. The `loadbalancer.server.scheme: h2c` label routes to HTTP/2-cleartext upstreams (e.g. gRPC).

### Static routes (routes.yaml)

For non-Docker services, define routes in `routes.yaml` (auto-reloaded on changes via fsnotify):

```yaml
routes:
  - host: app.lvh.me
    target: 5174                          # port-only: auto-resolves host
  - host: remote-app.lvh.me
    target: http://192.168.1.50:3000      # full URL: used as-is
  - host: win-app.lvh.me
    target: host.docker.internal:3000     # Windows-side service (WSL → Windows gateway)
  - host: wsl-app.lvh.me
    target: host.wsl.internal:3000        # WSL2 Linux-side service (no gateway rewrite)
```

Port-only targets resolve to `localhost` (host-native) or `host.docker.internal` (Docker) automatically. On WSL, `host.docker.internal` is rewritten to the WSL → Windows gateway; use the `host.wsl.internal` sentinel to reach a service on the WSL Linux host instead.

### Passthrough domains

Domains that should be forwarded to another proxy (e.g., Traefik) without TLS termination are configured in `routes.yaml`:

```yaml
passthrough:
  - domain: example-local.com
    target: traefik              # auto-discovers Traefik container IP
```

Traffic for `*.example-local.com` is passed through at the TCP level — pintle reads the SNI hostname from the TLS ClientHello but does not decrypt the traffic. The target proxy's container IP is auto-discovered on the shared Docker network.

Passthrough domains also need mkcert certs in `certs/` (used as fallback when the target proxy is unavailable):
```bash
mkcert -cert-file certs/example-local.com.pem -key-file certs/example-local.com-key.pem "*.example-local.com"
```

### TCP services (databases)

pintle can front raw TCP services (PostgreSQL, Redis, MySQL) over TLS. It listens on the service port, reads the SNI hostname from the TLS ClientHello, terminates TLS with the matching mkcert certificate, and forwards plaintext to the upstream container.

Define TCP routes in `routes.yaml`:

```yaml
tcp:
  - host: db.lvh.me
    target: 5432        # upstream port (host auto-resolves like routes above)
    listen: 5432        # port pintle listens on
```

Or auto-discover from Docker via Traefik TCP labels:

```yaml
labels:
  traefik.enable: "true"
  traefik.tcp.routers.db.rule: "HostSNI(`db.lvh.me`)"
  traefik.tcp.routers.db.entrypoints: postgres                  # redis | postgres | mysql
  traefik.tcp.services.db.loadbalancer.server.port: "5432"
```

Entrypoint names map to default listen ports: `redis` → 6379, `postgres` → 5432, `mysql` → 3306. Connect with a TLS-capable client using the SNI hostname, e.g. `psql "host=db.lvh.me sslmode=require"`. In Docker mode, `docker-compose.yaml` maps these listen ports to high host ports (`15432`→5432, `16379`→6379, `13306`→3306) so they don't collide with databases already running on the host — connect to the high port with the SNI hostname.

## Replacing Traefik or Caddy

pintle is a **drop-in** for both, not a companion to either. It parses `traefik.*` and `caddy*`
container labels natively, so the migration is a proxy swap and not a re-labelling job:

- **pintle takes over ports 443/80** — stop the existing proxy's port bindings first
- **No re-labelling** — every `traefik.*` and `caddy*` label your services already carry keeps working
- **Nothing to migrate up front** — point it at the same Docker network and the route table rebuilds itself
- **Passthrough for anything it should not own** — configured domains are forwarded at the TCP
  level, SNI read but never decrypted, so another proxy can keep its own certificates on the same
  port during a staged cutover

Rolling back is symmetrical: `docker compose down`, then start the old proxy again.

### Current limits

pintle terminates TLS with mkcert certificates, which makes it complete for local development and
incomplete as a public edge. Not yet implemented:

- Automatic Let's Encrypt / ACME certificates
- Load balancing across replicas, health checks, circuit breaking
- Rate limiting and auth middleware beyond the label dialects it parses

Those are the roadmap, not the boundary — until they land, a public-facing deployment still wants
Traefik or Caddy.

## Docs

- [Privileged Ports](docs/privileged-ports.md) — Why pintle uses iptables/pfctl and how other approaches compare
- [Benchmark Findings](docs/benchmark-findings.md) — Performance history: Bun stall issue and Go rewrite resolution

## Dashboard

Embedded React UI at `https://pintle.lvh.me`:

- **Dashboard** — interactive topology map (React Flow) showing the SNI router, HTTPS server, Traefik, and all service nodes, plus a stats bar (total requests, uptime, active routes, error rate)
- **Activity** — filterable, sortable request log (method, host, path, status, duration; absolute or relative timestamps)
- **Architecture** — the in-app explainer for traffic flow, SNI routing, TLS, and service discovery, with a Docker / host-native mode toggle
- **Glossary** — abbreviations used across the UI
- **Endpoints** — reference for the proxy's own REST API
- **Theme & scale** — light / dark / system theme (follows OS preference) and font-size scaling

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
| `./pintle --port-redirect` | Host-native with port redirection |

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

Environment-only (no CLI flag): `DOCKER_NETWORK` (Docker network to watch, default `traefik`), `VITE_DEV_URL` (dashboard dev-proxy target for HMR), `HOST_GATEWAY_IP` (override host-gateway auto-detection).

## Key Files

```
cmd/pintle/
  main.go              Entry point, provider wiring, signal handling
internal/
  config/              BASE_DOMAIN, HOST_ADDRESS, CLI flags, env vars
  hostdetect/          Host-gateway detection (WSL / Docker, host.wsl.internal sentinel)
  proxy/               HTTP reverse proxy (httputil.ReverseProxy) + WebSocket + h2c upstreams
  router/              Route table (hostname+path -> target, RWMutex)
  server/              SNI router, HTTPS (h2 ALPN) / HTTP servers, TCP router
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
