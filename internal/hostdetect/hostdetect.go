package hostdetect

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/steven-pribilinskiy/local-proxy/internal/logger"
)

// Detect returns the address local-proxy should use to reach host-native
// services (Vite dev servers, etc.) from static routes.
//
// Resolution order:
//  1. HOST_GATEWAY_IP env var (explicit override)
//  2. If not running in Docker → "localhost"
//  3. Read the host's default gateway from /proc/1/net/route. PID 1 lives in
//     the host's network namespace when the container is run with
//     `pid: host`, so this exposes the actual host routing table instead of
//     the container's bridge-only view.
//  4. Fall back to "host.docker.internal"
//
// On Docker CE / WSL, `host.docker.internal` is mapped via `host-gateway` to
// the default-bridge gateway (e.g. 172.16.0.1) which is not routable from
// containers attached to a non-default network. The WSL host's real default
// gateway (e.g. 172.22.144.1) IS routable and is what gets us back out to
// Windows.
//
// WSL2 caveat: services bound on the WSL2 Linux side (e.g. 0.0.0.0:7771)
// are NOT reachable through the WSL→Windows gateway IP — only `127.0.0.1`
// bindings are auto-forwarded by WSL2. To reach a Linux-side service from
// inside a container, target `host.wsl.internal` (added via extra_hosts in
// docker-compose). That hostname resolves to the bridge gateway via /etc/hosts
// and is intentionally NOT rewritten by file.rewriteHostInURL.
func Detect() string {
	if ip := strings.TrimSpace(os.Getenv("HOST_GATEWAY_IP")); ip != "" {
		logger.Infof("host address: %s (from HOST_GATEWAY_IP)", ip)
		return ip
	}

	if !isDocker() {
		return "localhost"
	}

	if ip := readHostRoute("/proc/1/net/route"); ip != "" {
		logger.Infof("host address: %s (auto-detected from host default gateway)", ip)
		return ip
	}

	logger.Warn("Could not auto-detect host gateway; falling back to host.docker.internal.")
	logger.Warn("On Docker CE / WSL this often resolves to an unreachable bridge IP.")
	logger.Warn("Run with `pid: host` or set HOST_GATEWAY_IP=<wsl-gateway> manually.")
	return "host.docker.internal"
}

func isDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

// readHostRoute parses the kernel's /proc/net/route format (little-endian hex)
// and returns the IPv4 default gateway as a dotted-quad, or "" if none found.
//
// Format (tab-separated, line 0 is a header):
//
//	Iface  Destination  Gateway   Flags  ...
//	eth0   00000000     0190A8C0  0003   ...
//
// Destination "00000000" = default route. Gateway is 4 bytes, little-endian.
func readHostRoute(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	for i, line := range strings.Split(string(data), "\n") {
		if i == 0 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		if fields[1] != "00000000" {
			continue
		}
		ip, ok := parseHexIPLE(fields[2])
		if !ok {
			continue
		}
		return ip
	}
	return ""
}

func parseHexIPLE(hex string) (string, bool) {
	if len(hex) != 8 {
		return "", false
	}
	b := make([]byte, 4)
	for i := 0; i < 4; i++ {
		v, err := strconv.ParseUint(hex[i*2:i*2+2], 16, 8)
		if err != nil {
			return "", false
		}
		b[3-i] = byte(v)
	}
	return fmt.Sprintf("%d.%d.%d.%d", b[0], b[1], b[2], b[3]), true
}

