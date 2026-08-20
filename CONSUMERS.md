# Contract surfaces

Things outside this repo bind to pintle. Each surface below is a name or path that
something else hardcodes, so changing one breaks that thing silently — a route that never
appears, a restart that never happens, a config read that finds nothing.

**Ask pintle before hardcoding.** `GET /api/self` reports the container name, the compose
project, the routes file in use (with its host-side path), the certs directory, the base
domain, the watched Docker network, and the label prefix. `pintle doctor` prints the same
and still works when pintle is down. A consumer that asks does not appear on this page.

**Search both filesystems.** This machine keeps repos in two places, and a sweep of one
looks complete:

```
~/projects/{personal,cloudbeds,aylith-labs}     # WSL
/mnt/c/Users/steve/projects                     # Windows
```

The rename that produced this file swept only the first and reported itself complete,
because the completeness check re-ran the same roots.

| Surface | Current value | Bound by |
|---|---|---|
| Native label prefix | `pintle.host` / `.port` / `.path` / `.strip` | `tabby-claude-status/web/docker-compose.yml` |
| Config path | `~/.config/pintle/routes.yaml` | `windows-control/scripts/pintle-domain/setup-domain.sh`, `linux-settings` backup entry `pintle` |
| Container name / compose project | `pintle-pintle-1` / `pintle` | `setup-domain.sh` restart, `stith`'s `make proxy`, `home-dashboard` container discovery |
| Dashboard host | `pintle.${BASE_DOMAIN}` | `home-dashboard` header link, `scripts/sync-hosts.sh` |
| Hosts-file marker | `# pintle` | `scripts/sync-hosts.sh` (also strips the pre-rename `# local-proxy` block) |
| Cert filenames | `certs/<domain>.pem` + `<domain>-key.pem` | cert discovery in `internal/tls/manager.go` |
| TCP entrypoint names | `redis` / `postgres` / `mysql` | containers carrying `traefik.tcp.routers.*.entrypoints` |
| Docker network | `traefik` | 20+ personal compose files and many work repos — **never rename this**; it is a network name, not a statement about which proxy runs |

## Foreign dialects

`traefik.*` and `caddy*` labels are parsed natively and are **not** legacy. They are the
reason a service can switch to pintle without being re-labelled, and every discovered route
on the maintainer's machine arrives through the Traefik dialect. Removing them would defeat
the point of the project.

## Declaring what should be served

A route only exists while the container behind it runs, so a host vanishing from the route
table is a symptom of its service being down — not evidence it was never configured. That
distinction cost a debugging session, so it is declarable:

```yaml
expect:
  - host: tabby-claude-status.lvh.me
    why: every Claude Code hook on this machine POSTs here; when it is down the hook fails silently
    project: /mnt/c/Users/steve/projects/tabby-claude-status/web
```

Declared hosts appear in `/api/self` and `pintle doctor` with a `routed` flag, and `doctor`
exits non-zero when any is missing. pintle deliberately does not probe health or start
anything: `windows-control`'s `localServices` registry already owns service health, start
commands and the attention UI, and reads `/api/self` for the routing half.

## Running shape

`docker-compose.yaml` sets `pid: host`, so the containerised binary is visible to
`pgrep pintle` on the host. **A PID hit is not evidence that pintle is running
host-native** — this inference has already been drawn and been wrong. `/api/self` reports
`inDocker` and the container identity directly; `docker-proxy` holding `:443` is the other
tell.
