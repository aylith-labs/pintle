# Benchmark: local-proxy vs Traefik

## Status: Unresolved — Bun fetch stalls under concurrent load

local-proxy works fine for normal dev usage (sequential page loads, HMR, API calls). The issue only manifests under sustained concurrent load (ab/bombardier benchmarks).

## Test Setup

- Bun 1.3.9 in Docker
- Target: home.lvh.me → home-dashboard Vite container
- Tools: `ab` (ApacheBench), `curl` parallel
- Traefik as comparison baseline

## Results

### Direct endpoints (no upstream fetch) — fast

```
ab -n 100 -c 10 https://proxy.lvh.me/api/health
→ ~16,000 req/s, 0.6ms/req — Bun.serve itself is fast
```

### Proxy path (fetch to upstream) — stalls

```
ab -n 100 -c 5 https://home.lvh.me/
→ Times out after ~50 requests (70s timeout)

ab -n 10000 -c 50 https://home.lvh.me/
→ Times out — processes in bursts of ~50, then stalls for seconds
```

### Traefik comparison

```
ab -n 10000 -c 50 https://home.lvh.me/  (via Traefik)
→ 2,156 req/s, completes in ~4.6s
```

### Parallel curl (works)

```
50 parallel curl -sk https://home.lvh.me/ → completes in ~5s
```

## Root Cause

Bun's `fetch()` has known issues with connection reuse under concurrent load:

1. **Keep-alive socket stalls** — Bun reuses keep-alive connections but the socket sometimes becomes unresponsive, forcing timeouts and reconnections. See [oven-sh/bun#9034](https://github.com/oven-sh/bun/issues/9034).

2. **Default 256 concurrent fetch limit** — `BUN_CONFIG_MAX_HTTP_REQUESTS` defaults to 256 (max 65,536). Not the bottleneck here since we stall well below 256.

3. **Burst-then-stall pattern** — Under sustained load, Bun processes ~50 requests in a fast burst, then the event loop stalls for several seconds before the next burst. This suggests a socket pool or event loop scheduling issue in Bun's HTTP client.

## What Was Tried (none solved it)

| Approach | Result |
|----------|--------|
| `keepalive: false` on fetch | Still stalls |
| `connection: close` header | Still stalls |
| `idleTimeout: 120` on Bun.serve | Still stalls |
| SNI router bypass (Bun direct on :9443) | Still stalls — SNI router wasn't the bottleneck |
| Node.js `http.Agent` with keepAlive pooling | Still stalls |
| `BUN_CONFIG_MAX_HTTP_REQUESTS=1024` | Not tested yet |

## What's Kept in the Code

These changes are committed as they improve the architecture regardless of the benchmark:

- `idleTimeout: 120` on Bun.serve (prevents premature connection drops)
- SNI router bypass when no passthrough domains (Bun listens directly on :9443)
- `decompress: false` on fetch (correct proxy behavior — pass compressed bytes through)

## Next Steps to Try

1. **`BUN_CONFIG_MAX_HTTP_REQUESTS=1024`** — Set as env var in docker-compose.yaml
2. **Bun upgrade** — Check if newer Bun versions fix the fetch pooling issue
3. **Node.js `http.request` instead of `fetch`** — Bypass Bun's fetch entirely using Node.js compat layer for the upstream call. This is the most promising approach since Node.js (via Traefik's Go runtime comparison) handles the same load fine.
4. **Undici** — Use the `undici` HTTP client library directly instead of Bun's built-in fetch
5. **Accept the limitation** — For local dev, the proxy handles real-world usage fine. The stall only happens under synthetic benchmarks with sustained concurrency.

## Relevant Bun Issues

- [oven-sh/bun#9034](https://github.com/oven-sh/bun/issues/9034) — Wildly bad networking performance with fetch when switching between endpoints
- [oven-sh/bun#12665](https://github.com/oven-sh/bun/issues/12665) — fetch with proxy crashes after hundreds of requests
- [oven-sh/bun#17434](https://github.com/oven-sh/bun/issues/17434) — fetch not working with proxy (Bun 1.2.2+)
