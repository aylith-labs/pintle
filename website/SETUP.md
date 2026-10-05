# Pintle Linux x64

This archive is a genuine statically linked Go binary with its embedded React dashboard.
Verify the adjacent SHA256 file before extraction. No automatic service, port redirect,
Docker mutation or certificate trust installation runs as part of downloading/extracting it.

## Loopback file routes

Prerequisites: a local HTTP service, mkcert (https://github.com/FiloSottile/mkcert),
and a deliberate local certificate trust installation according to mkcert's instructions.
The `lvh.me` domain resolves to loopback. Other operating systems can build the public source;
this particular archive is Linux x64.

```sh
mkdir certs
mkcert -install
mkcert -cert-file certs/lvh.me.pem -key-file certs/lvh.me-key.pem '*.lvh.me'
```

`mkcert -install` changes your local trust store; understand that step before running it.
Keep the generated keys private. Never distribute your CA/private key or ignore TLS warnings.
Start your own service on 127.0.0.1:3000 and create routes.yaml:

```yaml
routes:
  - host: app.lvh.me
    target: http://127.0.0.1:3000
```

```sh
./pintle --static-only --listen-port 9443 --http-port 9080 --routes-file ./routes.yaml --certs-dir ./certs
```

Open https://app.lvh.me:9443/ and https://pintle.lvh.me:9443/ after installing the correct
local trust. Stop with Ctrl+C. No Docker, TCP routing or SNI passthrough runs in static-only mode.
Invalid route reloads preserve the previously accepted routes.

## Full modes

The public source's README documents Docker label discovery, shared networks, SNI passthrough,
TCP database routing and host-native port redirection. Docker daemon access is privileged;
port redirection requires administrator access. Do not run those modes merely to try this archive.
https://github.com/aylith-labs/pintle#setup

The CLI includes `./pintle --help` and `./pintle doctor`.
Do not expose a local-development dashboard as an authenticated public management service.
Public certificate automation, replica load balancing, health checks, rate limiting and auth
middleware are not supplied. License originals/build provenance are alongside the binary.
