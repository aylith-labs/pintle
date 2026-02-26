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

add_windows_portproxy() {
	if ! command -v netsh.exe &>/dev/null; then
		return
	fi
	local wsl_ip
	wsl_ip="$(hostname -I | awk '{print $1}')"
	echo "Adding Windows port proxy rules (→ WSL $wsl_ip:$HTTPS_PORT)..."
	netsh.exe interface portproxy set v4tov4 listenport=443 listenaddress=127.0.0.1 connectport="$HTTPS_PORT" connectaddress="$wsl_ip" >/dev/null 2>&1 || true
	netsh.exe interface portproxy set v4tov4 listenport="$HTTPS_PORT" listenaddress=127.0.0.1 connectport="$HTTPS_PORT" connectaddress="$wsl_ip" >/dev/null 2>&1 || true
}

remove_iptables_rules() {
	echo "Removing iptables redirect rules..."
	sudo iptables -t nat -D PREROUTING -p tcp --dport 443 -m addrtype --dst-type LOCAL -j REDIRECT --to-port "$HTTPS_PORT" 2>/dev/null || true
	sudo iptables -t nat -D PREROUTING -p tcp --dport 80 -m addrtype --dst-type LOCAL -j REDIRECT --to-port "$HTTP_PORT" 2>/dev/null || true
	sudo iptables -t nat -D OUTPUT -p tcp --dport 443 -m addrtype --dst-type LOCAL -j REDIRECT --to-port "$HTTPS_PORT" 2>/dev/null || true
	sudo iptables -t nat -D OUTPUT -p tcp --dport 80 -m addrtype --dst-type LOCAL -j REDIRECT --to-port "$HTTP_PORT" 2>/dev/null || true
}

remove_windows_portproxy() {
	if ! command -v netsh.exe &>/dev/null; then
		return
	fi
	echo "Removing Windows port proxy rules..."
	netsh.exe interface portproxy delete v4tov4 listenport=443 listenaddress=127.0.0.1 >/dev/null 2>&1 || true
	netsh.exe interface portproxy delete v4tov4 listenport="$HTTPS_PORT" listenaddress=127.0.0.1 >/dev/null 2>&1 || true
}

cleanup() {
	remove_iptables_rules
	remove_windows_portproxy
	echo "Cleaned up iptables and port proxy rules."
}

trap cleanup EXIT INT TERM

add_iptables_rules
add_windows_portproxy
exec bun run "$PROJECT_DIR/src/index.ts"
