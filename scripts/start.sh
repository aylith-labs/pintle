#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

HTTPS_PORT="${LISTEN_PORT:-9443}"
HTTP_PORT="${HTTP_PORT:-9080}"

add_iptables_rules() {
	echo "Adding iptables redirect rules (443→$HTTPS_PORT, 80→$HTTP_PORT)..."
	sudo iptables -t nat -I PREROUTING 1 -p tcp --dport 443 -m addrtype --dst-type LOCAL -j REDIRECT --to-port "$HTTPS_PORT"
	sudo iptables -t nat -I PREROUTING 1 -p tcp --dport 80 -m addrtype --dst-type LOCAL -j REDIRECT --to-port "$HTTP_PORT"
	sudo iptables -t nat -I OUTPUT 1 -p tcp --dport 443 -m addrtype --dst-type LOCAL -j REDIRECT --to-port "$HTTPS_PORT"
	sudo iptables -t nat -I OUTPUT 1 -p tcp --dport 80 -m addrtype --dst-type LOCAL -j REDIRECT --to-port "$HTTP_PORT"
}

remove_iptables_rules() {
	echo "Removing iptables redirect rules..."
	sudo iptables -t nat -D PREROUTING -p tcp --dport 443 -m addrtype --dst-type LOCAL -j REDIRECT --to-port "$HTTPS_PORT" 2>/dev/null || true
	sudo iptables -t nat -D PREROUTING -p tcp --dport 80 -m addrtype --dst-type LOCAL -j REDIRECT --to-port "$HTTP_PORT" 2>/dev/null || true
	sudo iptables -t nat -D OUTPUT -p tcp --dport 443 -m addrtype --dst-type LOCAL -j REDIRECT --to-port "$HTTPS_PORT" 2>/dev/null || true
	sudo iptables -t nat -D OUTPUT -p tcp --dport 80 -m addrtype --dst-type LOCAL -j REDIRECT --to-port "$HTTP_PORT" 2>/dev/null || true
}

cleanup() {
	remove_iptables_rules
	echo "Cleaned up iptables rules."
}

trap cleanup EXIT INT TERM

add_iptables_rules
exec bun run "$PROJECT_DIR/src/index.ts"
