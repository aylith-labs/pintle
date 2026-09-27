---
name: Pintle
tagline: Route local development domains through one HTTPS reverse proxy
description: A Go reverse proxy for local development with Docker and file routes, TLS termination, SNI passthrough and an embedded dashboard. Its optional static-only mode serves loopback routes without Docker discovery.
category: developer-tools
features:
  - Docker route discovery from Pintle, Traefik and Caddy labels
  - File-defined routes, SNI passthrough and TLS termination for local services
  - Embedded dashboard and runtime self-description for local route inspection
targetUser: Developers running several local services behind HTTPS domains
onboarding:
  access: public-source
  url: https://github.com/aylith-labs/pintle#setup
  prerequisites:
    - Local TLS certificates from mkcert and route configuration
    - Docker for the recommended mode, or a locally built Go binary for host-native use
  limitations:
    - Host-native port redirection requires administrator privileges
    - Static-only mode serves loopback HTTP routes without Docker discovery or TCP routing
---

## A local proxy that reads existing labels

Pintle routes local HTTPS domains to development services. Its source accepts Pintle, Traefik and Caddy label formats, reads static route files, and can pass an SNI domain through to another proxy. The embedded dashboard and `GET /api/self` describe the running local configuration.

The [setup guide](https://github.com/aylith-labs/pintle#setup) covers certificates, route files, build and Docker use. The optional static-only mode serves loopback HTTP routes and omits Docker discovery, TCP listeners and SNI passthrough.
