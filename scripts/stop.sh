#!/usr/bin/env bash
set -euo pipefail

HTTPS_PORT="${LISTEN_PORT:-9443}"
HTTP_PORT="${HTTP_PORT:-9080}"
PF_ANCHOR="com.local-proxy"

OS="$(uname -s)"

case "$OS" in
	Linux)
		echo "Removing iptables redirect rules..."
		sudo iptables -t nat -D PREROUTING -p tcp --dport 443 -m addrtype --dst-type LOCAL -j REDIRECT --to-port "$HTTPS_PORT" 2>/dev/null || true
		sudo iptables -t nat -D PREROUTING -p tcp --dport 80 -m addrtype --dst-type LOCAL -j REDIRECT --to-port "$HTTP_PORT" 2>/dev/null || true
		sudo iptables -t nat -D OUTPUT -p tcp --dport 443 -m addrtype --dst-type LOCAL -j REDIRECT --to-port "$HTTPS_PORT" 2>/dev/null || true
		sudo iptables -t nat -D OUTPUT -p tcp --dport 80 -m addrtype --dst-type LOCAL -j REDIRECT --to-port "$HTTP_PORT" 2>/dev/null || true

		if command -v netsh.exe &>/dev/null; then
			echo "Removing Windows port proxy rules..."
			netsh.exe interface portproxy delete v4tov4 listenport=443 listenaddress=127.0.0.1 >/dev/null 2>&1 || true
			netsh.exe interface portproxy delete v4tov4 listenport="$HTTPS_PORT" listenaddress=127.0.0.1 >/dev/null 2>&1 || true
		fi
		;;
	Darwin)
		echo "Removing pf redirect rules..."
		sudo pfctl -a "$PF_ANCHOR" -F all 2>/dev/null || true
		;;
	*)
		echo "Unsupported OS: $OS" >&2
		exit 1
		;;
esac

echo "Done."
