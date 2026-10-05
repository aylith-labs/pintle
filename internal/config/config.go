package config

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/aylith-labs/pintle/internal/hostdetect"
	"github.com/aylith-labs/pintle/internal/provider"
)

type Config struct {
	BaseDomain      string
	DashboardHost   string
	ListenPort      int
	HTTPPort        int
	DockerNetwork   string
	CertsDir        string
	RoutesFile      string
	HostAddress     string
	InDocker        bool
	ViteDevURL      string
	LogLevel        string
	LogFormat       string
	PortRedirect    bool
	StaticOnly      bool
	LoopbackAddress string
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

func Load() (*Config, error) {
	return load(flag.CommandLine, os.Args[1:])
}

// LoadArgs parses an explicit argument list. A subcommand name is a non-flag argument,
// and flag parsing stops at the first of those, so a subcommand must pass the arguments
// that follow its own name or every flag after it is silently ignored.
func LoadArgs(args []string) (*Config, error) {
	return load(flag.NewFlagSet("pintle", flag.ContinueOnError), args)
}

func load(flags *flag.FlagSet, args []string) (*Config, error) {
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

	flags.StringVar(&cfg.BaseDomain, "base-domain", cfg.BaseDomain, "Base domain for routing")
	flags.IntVar(&cfg.ListenPort, "listen-port", cfg.ListenPort, "HTTPS listen port")
	flags.IntVar(&cfg.HTTPPort, "http-port", cfg.HTTPPort, "HTTP redirect listen port")
	flags.StringVar(&cfg.CertsDir, "certs-dir", cfg.CertsDir, "Path to certificates directory")
	flags.StringVar(&cfg.RoutesFile, "routes-file", cfg.RoutesFile, "Path to routes.yaml")
	flags.BoolVar(&cfg.PortRedirect, "port-redirect", false, "Add iptables/pfctl rules on start")
	flags.BoolVar(&cfg.StaticOnly, "static-only", false, "Loopback HTTP/HTTPS with file routes only; no Docker, TCP or SNI passthrough")
	flags.StringVar(&cfg.LoopbackAddress, "loopback-address", "", "HTTP/HTTPS loopback IP (requires --static-only; defaults to 127.0.0.1)")
	flags.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level (debug, info, warn, error)")
	flags.StringVar(&cfg.LogFormat, "log-format", cfg.LogFormat, "Log format (text, json)")
	if err := flags.Parse(args); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	cfg.DashboardHost = fmt.Sprintf("pintle.%s", cfg.BaseDomain)

	// Resolve routes file: ./routes.yaml → ~/.config/pintle/routes.yaml
	if cfg.RoutesFile == "./routes.yaml" {
		if _, err := os.Stat(cfg.RoutesFile); os.IsNotExist(err) {
			if home, err := os.UserHomeDir(); err == nil {
				xdgPath := filepath.Join(home, ".config", "pintle", "routes.yaml")
				if _, err := os.Stat(xdgPath); err == nil {
					cfg.RoutesFile = xdgPath
				}
			}
		}
	}

	if cfg.StaticOnly {
		// A native Windows process reaches Windows loopback directly. Do not
		// infer a WSL/Docker gateway or inherit HOST_GATEWAY_IP in this mode.
		cfg.HostAddress = "127.0.0.1"
	} else {
		cfg.HostAddress = hostdetect.Detect()
	}
	_, err := os.Stat("/.dockerenv")
	cfg.InDocker = err == nil

	return cfg, nil
}

func (c *Config) validate() error {
	if !c.StaticOnly {
		if c.LoopbackAddress != "" {
			return fmt.Errorf("--loopback-address requires --static-only")
		}
		return nil
	}
	if c.LoopbackAddress == "" {
		c.LoopbackAddress = "127.0.0.1"
	}
	ip := net.ParseIP(c.LoopbackAddress)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("--static-only requires --loopback-address to be a loopback IP (127.0.0.0/8 or ::1)")
	}
	if c.PortRedirect {
		return fmt.Errorf("--static-only cannot be combined with --port-redirect")
	}
	if c.ViteDevURL != "" {
		return fmt.Errorf("--static-only requires VITE_DEV_URL to be unset; dashboard uses embedded files")
	}
	if c.ListenPort < 1 || c.ListenPort > 65535 || c.HTTPPort < 1 || c.HTTPPort > 65535 || c.ListenPort == c.HTTPPort {
		return fmt.Errorf("--static-only requires distinct HTTP/HTTPS ports in 1..65535")
	}
	return nil
}

// ValidateMessage runs before aggregation, for both initial load and every
// reload. Reject the entire incompatible update so the last accepted routes
// remain active and no dynamic message can enable additional listeners.
func (c *Config) ValidateMessage(msg provider.Message) error {
	if msg.Err != nil {
		return fmt.Errorf("provider %s update: %w", msg.ProviderName, msg.Err)
	}
	if !c.StaticOnly {
		return nil
	}
	if msg.ProviderName != "file" {
		return fmt.Errorf("--static-only accepts only the file provider")
	}
	if len(msg.TcpRoutes) != 0 || len(msg.Passthrough) != 0 {
		return fmt.Errorf("--static-only forbids TCP routes and SNI passthrough; remove tcp and passthrough entries")
	}
	return nil
}
