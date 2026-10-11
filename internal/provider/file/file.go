package file

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/aylith-labs/pintle/internal/logger"
	"github.com/aylith-labs/pintle/internal/provider"
	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

type staticRouteConfig struct {
	// Host and Hosts are both accepted and unioned: one service reachable under several names
	// (stith.lvh.me, stith.localhost, ...) is one route, not one copy per name.
	Host    string              `yaml:"host"`
	Hosts   []string            `yaml:"hosts"`
	Target  interface{}         `yaml:"target"`  // string or int
	Targets []interface{}       `yaml:"targets"` // explicit replica pool, mutually exclusive with target
	Path    string              `yaml:"path"`
	Strip   bool                `yaml:"strip"`
	Health  *staticHealthConfig `yaml:"health"`
}

type staticHealthConfig struct {
	Path     string      `yaml:"path"`
	Interval interface{} `yaml:"interval_seconds"`
	Timeout  interface{} `yaml:"timeout_seconds"`
}

// hostnames returns every name this route answers to, deduplicated and in declaration order.
// Blank entries are dropped so a stray "- " in the YAML cannot register an empty hostname.
func (r staticRouteConfig) hostnames() []string {
	seen := make(map[string]bool)
	var out []string
	for _, hostname := range append([]string{r.Host}, r.Hosts...) {
		hostname = strings.TrimSpace(hostname)
		if hostname == "" || seen[hostname] {
			continue
		}
		seen[hostname] = true
		out = append(out, hostname)
	}
	return out
}

type staticTcpRouteConfig struct {
	Host   string      `yaml:"host"`
	Target interface{} `yaml:"target"` // string or int
	Listen int         `yaml:"listen"`
}

type routesFile struct {
	Routes      []staticRouteConfig          `yaml:"routes"`
	Passthrough []provider.PassthroughDomain `yaml:"passthrough"`
	TCP         []staticTcpRouteConfig       `yaml:"tcp"`
	Expect      []provider.ExpectedHost      `yaml:"expect"`
}

var portOnlyRegex = regexp.MustCompile(`^\d+$`)

type FileProvider struct {
	filePath    string
	hostAddress string
}

func New(filePath, hostAddress string) *FileProvider {
	return &FileProvider{
		filePath:    filePath,
		hostAddress: hostAddress,
	}
}

func (f *FileProvider) resolveTarget(target interface{}) string {
	switch v := target.(type) {
	case int:
		return fmt.Sprintf("http://%s:%d", f.hostAddress, v)
	case float64:
		return fmt.Sprintf("http://%s:%d", f.hostAddress, int(v))
	case string:
		if portOnlyRegex.MatchString(v) {
			return fmt.Sprintf("http://%s:%s", f.hostAddress, v)
		}
		return f.rewriteHostInURL(v)
	default:
		return fmt.Sprintf("%v", target)
	}
}

// rewriteHostInURL replaces "host.docker.internal" in a full URL with the
// detected host address when that detection produced a real IP. This lets
// existing routes.yaml files keep working on Docker CE / WSL, where the
// default host-gateway mapping points at an unreachable bridge IP.
//
// `host.wsl.internal` is intentionally NOT rewritten — it's an opt-in
// sentinel for routes that need the WSL2 Linux host (not the Windows side).
// It resolves via the container's /etc/hosts (added in docker-compose
// extra_hosts as `host.wsl.internal:host-gateway`).
func (f *FileProvider) rewriteHostInURL(raw string) string {
	if f.hostAddress == "" || f.hostAddress == "host.docker.internal" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	host := u.Hostname()
	if host != "host.docker.internal" {
		return raw
	}
	// Preserve port.
	if port := u.Port(); port != "" {
		u.Host = net.JoinHostPort(f.hostAddress, port)
	} else {
		u.Host = f.hostAddress
	}
	return u.String()
}

func (f *FileProvider) resolveTcpTarget(target interface{}) (string, int) {
	switch v := target.(type) {
	case int:
		return f.hostAddress, v
	case float64:
		return f.hostAddress, int(v)
	case string:
		if portOnlyRegex.MatchString(v) {
			port, _ := strconv.Atoi(v)
			return f.hostAddress, port
		}
		parts := strings.SplitN(v, ":", 2)
		if len(parts) == 2 {
			port, _ := strconv.Atoi(parts[1])
			return parts[0], port
		}
		return v, 0
	default:
		return f.hostAddress, 0
	}
}

// loadFile applies the same destination checks as `pintle check`, so an invalid
// reload is rejected as a whole and the last accepted routes stay active.
func (f *FileProvider) loadFile() provider.Message {
	msg, err := f.Check()
	if err != nil {
		msg.Err = err
	}
	return msg
}

func (f *FileProvider) loadFileParsed() (provider.Message, routesFile) {
	msg := provider.Message{
		ProviderName: "file",
	}

	data, err := os.ReadFile(f.filePath)
	if err != nil {
		logger.Errorf("Failed to load routes file: %s: %v", f.filePath, err)
		msg.Err = err
		return msg, routesFile{}
	}

	if len(bytes.TrimSpace(data)) == 0 {
		msg.Err = fmt.Errorf("routes document is empty; use routes: [] to explicitly clear routes")
		return msg, routesFile{}
	}

	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		msg.Err = err
		return msg, routesFile{}
	}
	if len(document.Content) != 1 || document.Content[0].Kind != yaml.MappingNode {
		msg.Err = fmt.Errorf("routes document requires an explicit collection")
		return msg, routesFile{}
	}
	mapping := document.Content[0]
	explicitCollection := false
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		key, value := mapping.Content[i].Value, mapping.Content[i+1]
		if key == "routes" || key == "tcp" || key == "passthrough" || key == "expect" {
			if value.Kind != yaml.SequenceNode {
				msg.Err = fmt.Errorf("%s must be an explicit sequence; use [] to clear it", key)
				return msg, routesFile{}
			}
			explicitCollection = true
		}
	}
	if !explicitCollection {
		msg.Err = fmt.Errorf("routes document requires routes, tcp, passthrough or expect; use routes: [] to clear routes")
		return msg, routesFile{}
	}
	var parsed routesFile
	if err := document.Decode(&parsed); err != nil {
		logger.Errorf("Failed to parse routes file: %s: %v", f.filePath, err)
		msg.Err = err
		return msg, routesFile{}
	}

	for index, r := range parsed.Routes {
		var health *provider.HealthCheck
		if r.Health != nil {
			interval, intervalOK := r.Health.Interval.(int)
			timeout, timeoutOK := r.Health.Timeout.(int)
			h := &provider.HealthCheck{Path: r.Health.Path, IntervalSeconds: interval, TimeoutSeconds: timeout}
			u, err := url.ParseRequestURI(h.Path)
			if !intervalOK || !timeoutOK || r.Targets == nil || err != nil || u == nil || !strings.HasPrefix(h.Path, "/") || strings.HasPrefix(h.Path, "//") || strings.ContainsAny(h.Path, "?#\r\n") || len(h.Path) > 256 || u.RawQuery != "" || u.Fragment != "" || u.Host != "" || h.IntervalSeconds < 5 || h.IntervalSeconds > 300 || h.TimeoutSeconds < 1 || h.TimeoutSeconds > 10 || h.TimeoutSeconds > h.IntervalSeconds {
				msg.Err = fmt.Errorf("route %d health requires targets, an origin path, interval_seconds 5..300 and timeout_seconds 1..10 no greater than interval", index+1)
				return msg, routesFile{}
			}
			health = h
		}
		path := r.Path
		if path == "" {
			path = "/"
		}
		values := []interface{}{r.Target}
		pool := ""
		if r.Targets != nil {
			if r.Target != nil || len(r.Targets) < 1 || len(r.Targets) > 32 {
				msg.Err = fmt.Errorf("route %d requires either target or 1..32 targets", index+1)
				return msg, routesFile{}
			}
			values = r.Targets
			pool = fmt.Sprintf("static:%d", index)
		}
		seen := make(map[string]bool)
		for _, value := range values {
			if pool != "" {
				switch value.(type) {
				case int, string:
				default:
					msg.Err = fmt.Errorf("route %d replica targets must be integer ports or URL strings", index+1)
					return msg, routesFile{}
				}
			}
			target := f.resolveTarget(value)
			u, err := url.Parse(target)
			if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
				msg.Err = fmt.Errorf("route %d has an invalid HTTP target", index+1)
				return msg, routesFile{}
			}
			if port := u.Port(); port != "" {
				n, err := strconv.Atoi(port)
				if err != nil || n < 1 || n > 65535 {
					msg.Err = fmt.Errorf("route %d has an invalid target port", index+1)
					return msg, routesFile{}
				}
			}
			if seen[target] {
				msg.Err = fmt.Errorf("route %d has duplicate replica targets", index+1)
				return msg, routesFile{}
			}
			seen[target] = true
			for _, hostname := range r.hostnames() {
				msg.Routes = append(msg.Routes, provider.Route{
					Hostname:    hostname,
					Path:        path,
					Target:      target,
					BackendPool: pool,
					Health:      health,
					StripPath:   r.Strip,
					Source:      "static",
				})
			}
		}

	}
	checked := 0
	for _, route := range msg.Routes {
		if route.Health != nil {
			checked++
		}
	}
	if checked > 128 {
		msg.Err = fmt.Errorf("health probes are limited to 128 configured host/backend pairs")
		return msg, routesFile{}
	}
	msg.Passthrough = parsed.Passthrough
	msg.Expected = parsed.Expect

	for _, t := range parsed.TCP {
		host, port := f.resolveTcpTarget(t.Target)
		msg.TcpRoutes = append(msg.TcpRoutes, provider.TcpRoute{
			Hostname:   t.Host,
			TargetHost: host,
			TargetPort: port,
			ListenPort: t.Listen,
			Source:     "static",
		})
	}

	return msg, parsed
}

// Check reads a routes file and validates the destinations the providers would
// publish. It does not open a listener or contact an upstream.
func (f *FileProvider) Check() (provider.Message, error) {
	msg, parsed := f.loadFileParsed()
	if msg.Err != nil {
		return msg, msg.Err
	}
	for _, route := range parsed.Routes {
		if len(route.hostnames()) == 0 {
			return msg, fmt.Errorf("HTTP route has no host")
		}
	}
	for _, route := range msg.Routes {
		u, err := url.Parse(route.Target)
		if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil {
			return msg, fmt.Errorf("invalid HTTP target %q for %s", route.Target, route.Hostname)
		}
		if port := u.Port(); port != "" {
			n, err := strconv.Atoi(port)
			if err != nil || n < 1 || n > 65535 {
				return msg, fmt.Errorf("invalid HTTP target port %q for %s", port, route.Hostname)
			}
		}
	}
	for _, route := range msg.TcpRoutes {
		if route.Hostname == "" || route.TargetHost == "" || route.TargetPort < 1 || route.TargetPort > 65535 || route.ListenPort < 1 || route.ListenPort > 65535 {
			return msg, fmt.Errorf("invalid TCP destination or listen port for %q", route.Hostname)
		}
	}
	for _, pt := range msg.Passthrough {
		if pt.Domain == "" || !validPassthroughTarget(pt.Target) {
			return msg, fmt.Errorf("invalid passthrough destination %q for %q: use traefik or host:port", pt.Target, pt.Domain)
		}
	}
	return msg, nil
}

// validPassthroughTarget mirrors the runtime resolver without DNS lookups: an
// empty target or traefik uses discovery, otherwise an explicit host:port.
func validPassthroughTarget(target string) bool {
	if target == "" || target == "traefik" {
		return true
	}
	host, rawPort, err := net.SplitHostPort(target)
	if err != nil || host == "" {
		return false
	}
	port, err := strconv.Atoi(rawPort)
	return err == nil && port >= 1 && port <= 65535
}

func (f *FileProvider) Run(ctx context.Context, configCh chan<- provider.Message) error {
	// Initial load
	msg := f.loadFile()
	logger.Infof("Loaded %d static route(s), %d TCP route(s), %d passthrough domain(s), %d expected host(s)",
		len(msg.Routes), len(msg.TcpRoutes), len(msg.Passthrough), len(msg.Expected))
	configCh <- msg

	// Watch for changes
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Errorf("Failed to create file watcher: %v", err)
		<-ctx.Done()
		return nil
	}
	defer watcher.Close()

	if err := watcher.Add(f.filePath); err != nil {
		logger.Errorf("Failed to watch routes file: %v", err)
		<-ctx.Done()
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				logger.Info("Routes file changed, reloading...")
				msg := f.loadFile()
				configCh <- msg
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			logger.Errorf("File watcher error: %v", err)
		}
	}
}
