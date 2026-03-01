#!/usr/bin/env bash
set -euo pipefail

echo "Note: 'docker compose up -d' is the recommended way to run local-proxy."
echo "This script is for host-native mode (without Docker)."
echo ""

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

HTTPS_PORT="${LISTEN_PORT:-9443}"
HTTP_PORT="${HTTP_PORT:-9080}"
PF_ANCHOR="com.local-proxy"

OS="$(uname -s)"

# --- Linux (iptables) ---

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

# --- WSL (Windows port proxy) ---

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

remove_windows_portproxy() {
	if ! command -v netsh.exe &>/dev/null; then
		return
	fi
	echo "Removing Windows port proxy rules..."
	netsh.exe interface portproxy delete v4tov4 listenport=443 listenaddress=127.0.0.1 >/dev/null 2>&1 || true
	netsh.exe interface portproxy delete v4tov4 listenport="$HTTPS_PORT" listenaddress=127.0.0.1 >/dev/null 2>&1 || true
}

# --- macOS (pfctl) ---

add_pf_rules() {
	echo "Adding pf redirect rules (443→$HTTPS_PORT, 80→$HTTP_PORT)..."
	cat <<-EOF | sudo pfctl -a "$PF_ANCHOR" -f -
		rdr pass on lo0 inet proto tcp from any to any port 443 -> 127.0.0.1 port $HTTPS_PORT
		rdr pass on lo0 inet proto tcp from any to any port 80 -> 127.0.0.1 port $HTTP_PORT
	EOF
	sudo pfctl -e 2>/dev/null || true
}

remove_pf_rules() {
	echo "Removing pf redirect rules..."
	sudo pfctl -a "$PF_ANCHOR" -F all 2>/dev/null || true
}

# --- OS dispatch ---

add_rules() {
	case "$OS" in
		Linux)
			add_iptables_rules
			add_windows_portproxy
			;;
		Darwin)
			add_pf_rules
			;;
		*)
			echo "Unsupported OS: $OS" >&2
			exit 1
			;;
	esac
}

remove_rules() {
	case "$OS" in
		Linux)
			remove_iptables_rules
			remove_windows_portproxy
			;;
		Darwin)
			remove_pf_rules
			;;
	esac
}

cleanup() {
	remove_rules
	echo "Cleaned up redirect rules."
}

trap cleanup EXIT INT TERM

add_rules
exec bun run "$PROJECT_DIR/src/index.ts"
