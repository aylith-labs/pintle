# Binding to Privileged Ports (80/443)

Ports below 1024 are "privileged" on Linux/macOS — only root can bind to them. local-proxy supports two deployment modes: Docker (primary) and host-native, each handling privileged ports differently.

## Approaches

### 1. Port redirection (what local-proxy uses in host-native mode)

Listen on unprivileged ports (9443/9080) and redirect standard ports via OS firewall rules:

- **Linux**: `iptables -t nat -A PREROUTING/OUTPUT ... --dport 443 -j REDIRECT --to-port 9443`
- **macOS**: `pfctl` with a named anchor

**Pros**: No modification to the binary, rules are temporary (cleaned up on exit), coexists with Docker's own port mappings.

**Cons**: Requires `sudo` to add/remove rules, two-step setup (redirect + listen).

The binary supports `--port-redirect` to manage these rules automatically.

### 2. Linux capabilities (`setcap`)

Grant the specific "bind low ports" capability to the binary:

```bash
sudo setcap cap_net_bind_service=+ep ./local-proxy
```

Then the binary can directly bind to port 443 without sudo or iptables.

**Pros**: Simplest setup, no iptables rules needed, no sudo at runtime.

**Cons**: Applies to the specific binary file. Must be re-applied after each rebuild. For a personal dev machine this is fine; on shared systems it's overly broad.

### 3. `authbind`

Per-port permission files that allow specific users to bind specific ports:

```bash
sudo apt install authbind
sudo touch /etc/authbind/byport/443
sudo chmod 500 /etc/authbind/byport/443
sudo chown $USER /etc/authbind/byport/443
authbind ./local-proxy
```

**Pros**: Fine-grained (per-port, per-user), doesn't modify the binary.

**Cons**: Extra dependency, Linux-only (no macOS support), less commonly used.

### 4. Kernel sysctl

Lower the privileged port threshold system-wide:

```bash
sudo sysctl net.ipv4.ip_unprivileged_port_start=80
```

**Pros**: One command, no binary modification, all processes benefit.

**Cons**: System-wide — any process can bind to ports 80+. Persists until reboot (or add to `/etc/sysctl.conf` to survive reboots). Linux-only.

### 5. Docker + port mapping (what local-proxy uses by default)

Containerize the proxy and let Docker handle port binding (Docker daemon runs as root):

```yaml
services:
  local-proxy:
    ports:
      - "443:9443"
      - "80:9080"
```

**Pros**: No sudo at runtime, standard Docker workflow, single 10 MB image.

**Cons**: Host processes require `host.docker.internal` (configured via `extra_hosts` on Linux). local-proxy handles this automatically — port-only targets in `routes.yaml` resolve to `host.docker.internal` when running in Docker.

This is the approach local-proxy uses by default (`docker compose up -d`).

## How other proxies handle this

| Proxy | Typical approach |
|-------|-----------------|
| **Traefik** | Runs in Docker; `dockerd` (root) handles host port binding |
| **Caddy** | Docker, or `setcap` on the binary |
| **nginx** | Docker, or starts as root then drops privileges after binding |
| **local-proxy** | Docker (primary), or `--port-redirect` for host-native |

## What local-proxy uses

- **Docker** (primary): `docker compose up -d` — Docker handles port 443/80 binding, `restart: unless-stopped` for daemon behavior. Same pattern as Traefik.
- **Host-native** (alternative): `./local-proxy --port-redirect` — iptables/pfctl port redirection for development without Docker overhead.
