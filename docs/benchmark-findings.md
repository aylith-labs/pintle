# Benchmark: local-proxy performance

## Status: Resolved — Rewritten in Go (March 2026)

The Bun implementation had a fundamental connection pooling bug that caused stalls under concurrent load. Rewriting in Go with `net/http/httputil.ReverseProxy` resolved the issue completely.

## Go proxy results

### Throughput (ab, 1000 requests, concurrency 50)

```
Target: myapp.example-local.com (Vite dev server, 250 resources)

Direct (localhost:5770):   186 req/s, 0 failures
Via Go proxy:              260 req/s, 0 failures
```

The Go proxy is faster than direct access due to `http.Transport` connection pooling — it reuses upstream connections across requests, while direct access opens new connections each time.

### Full page load (Playwright, 250 resources, 5 runs averaged)

```
                    Direct       Via Go proxy    Overhead
DOMContentLoaded    1142ms       1227ms          +85ms (7%)
Load complete       1147ms       1232ms          +85ms (7%)
Network idle        2509ms       2604ms          +95ms (4%)
```

~0.3ms per-request overhead for proxying through TLS + SNI routing + reverse proxy.

### Comparison with other proxies

The overhead is equivalent to Traefik and Caddy because:
- Same language (Go), same `net/http` stack
- Same TLS termination cost (mkcert certs)
- Same reverse proxy mechanics (`httputil.ReverseProxy`)
- Traefik does more work per request (middleware chains, metrics, tracing) so would be slightly slower

### Docker image size

```
Bun (previous):  ~200 MB (oven/bun image + node_modules + separate Vite UI container)
Go (current):     10 MB (FROM scratch, static binary with embedded UI)
```

---

## Historical: Bun implementation issues (pre-March 2026)

### Test setup

- Bun 1.3.9 in Docker
- Target: home.lvh.me -> home-dashboard Vite container
- Tools: `ab` (ApacheBench), `curl` parallel
- Traefik as comparison baseline

### Results

#### Direct endpoints (no upstream fetch) — fast

```
ab -n 100 -c 10 https://proxy.lvh.me/api/health
-> ~16,000 req/s, 0.6ms/req — Bun.serve itself is fast
```

#### Proxy path (fetch to upstream) — stalls

```
ab -n 100 -c 5 https://home.lvh.me/
-> Times out after ~50 requests (70s timeout)

ab -n 10000 -c 50 https://home.lvh.me/
-> Times out — processes in bursts of ~50, then stalls for seconds
```

#### Traefik comparison

```
ab -n 10000 -c 50 https://home.lvh.me/  (via Traefik)
-> 2,156 req/s, completes in ~4.6s
```

### Root cause

Bun's `fetch()` has known issues with connection reuse under concurrent load:

1. **Keep-alive socket stalls** — Bun reuses keep-alive connections but the socket sometimes becomes unresponsive, forcing timeouts and reconnections. See [oven-sh/bun#9034](https://github.com/oven-sh/bun/issues/9034).

2. **Burst-then-stall pattern** — Under sustained load, Bun processes ~50 requests in a fast burst, then the event loop stalls for several seconds before the next burst.

### What was tried (none solved it)

| Approach | Result |
|----------|--------|
| `keepalive: false` on fetch | Still stalls |
| `connection: close` header | Still stalls |
| `idleTimeout: 120` on Bun.serve | Still stalls |
| SNI router bypass (Bun direct on :9443) | Still stalls |
| Node.js `http.Agent` with keepAlive pooling | Still stalls |

### Resolution

Rewrote the entire proxy in Go. Go's `http.Transport` provides proper connection pooling with configurable `MaxIdleConns`, `MaxIdleConnsPerHost`, and `IdleConnTimeout`. The stall issue does not exist in Go's networking stack.

### Relevant Bun issues

- [oven-sh/bun#9034](https://github.com/oven-sh/bun/issues/9034) — Wildly bad networking performance with fetch when switching between endpoints
- [oven-sh/bun#12665](https://github.com/oven-sh/bun/issues/12665) — fetch with proxy crashes after hundreds of requests
- [oven-sh/bun#17434](https://github.com/oven-sh/bun/issues/17434) — fetch not working with proxy (Bun 1.2.2+)
