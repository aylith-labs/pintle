package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	BaseDomain    string
	DashboardHost string
	ListenPort    int
	HTTPPort      int
	DockerNetwork string
	CertsDir      string
	RoutesFile    string
	HostAddress   string
	ViteDevURL    string
	LogLevel      string
	LogFormat     string
	PortRedirect  bool
}

// TCPEntrypoints maps entrypoint names to default ports.
var TCPEntrypoints = map[string]int{
	"redis":    6379,
	"postgres": 5432,
	"mysql":    3306,
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envIntOrDefault(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

func isDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

func Load() *Config {
	cfg := &Config{
		BaseDomain:    envOrDefault("BASE_DOMAIN", "lvh.me"),
		ListenPort:    envIntOrDefault("LISTEN_PORT", 9443),
		HTTPPort:      envIntOrDefault("HTTP_PORT", 9080),
		DockerNetwork: envOrDefault("DOCKER_NETWORK", "traefik"),
		CertsDir:      envOrDefault("CERTS_DIR", "./certs"),
		RoutesFile:    envOrDefault("ROUTES_FILE", "./routes.yaml"),
		ViteDevURL:    os.Getenv("VITE_DEV_URL"),
		LogLevel:      envOrDefault("LOG_LEVEL", "info"),
		LogFormat:     envOrDefault("LOG_FORMAT", "text"),
	}

	flag.StringVar(&cfg.BaseDomain, "base-domain", cfg.BaseDomain, "Base domain for routing")
	flag.IntVar(&cfg.ListenPort, "listen-port", cfg.ListenPort, "HTTPS listen port")
	flag.IntVar(&cfg.HTTPPort, "http-port", cfg.HTTPPort, "HTTP redirect listen port")
	flag.StringVar(&cfg.CertsDir, "certs-dir", cfg.CertsDir, "Path to certificates directory")
	flag.StringVar(&cfg.RoutesFile, "routes-file", cfg.RoutesFile, "Path to routes.yaml")
	flag.BoolVar(&cfg.PortRedirect, "port-redirect", false, "Add iptables/pfctl rules on start")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level (debug, info, warn, error)")
	flag.StringVar(&cfg.LogFormat, "log-format", cfg.LogFormat, "Log format (text, json)")
	flag.Parse()

	cfg.DashboardHost = fmt.Sprintf("proxy.%s", cfg.BaseDomain)

	if isDocker() {
		cfg.HostAddress = "host.docker.internal"
	} else {
		cfg.HostAddress = "localhost"
	}

	return cfg
}
