#!/usr/bin/env bash
set -euo pipefail

HTTPS_PORT="${LISTEN_PORT:-9443}"
HTTP_PORT="${HTTP_PORT:-9080}"

echo "Removing iptables redirect rules..."
sudo iptables -t nat -D PREROUTING -p tcp --dport 443 -m addrtype --dst-type LOCAL -j REDIRECT --to-port "$HTTPS_PORT" 2>/dev/null || true
sudo iptables -t nat -D PREROUTING -p tcp --dport 80 -m addrtype --dst-type LOCAL -j REDIRECT --to-port "$HTTP_PORT" 2>/dev/null || true
sudo iptables -t nat -D OUTPUT -p tcp --dport 443 -m addrtype --dst-type LOCAL -j REDIRECT --to-port "$HTTPS_PORT" 2>/dev/null || true
sudo iptables -t nat -D OUTPUT -p tcp --dport 80 -m addrtype --dst-type LOCAL -j REDIRECT --to-port "$HTTP_PORT" 2>/dev/null || true
echo "Done."
