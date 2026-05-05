package file

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/steven-pribilinskiy/local-proxy/internal/logger"
	"github.com/steven-pribilinskiy/local-proxy/internal/provider"
	"gopkg.in/yaml.v3"
)

type staticRouteConfig struct {
	Host   string      `yaml:"host"`
	Target interface{} `yaml:"target"` // string or int
	Path   string      `yaml:"path"`
	Strip  bool        `yaml:"strip"`
}

type staticTcpRouteConfig struct {
	Host   string      `yaml:"host"`
	Target interface{} `yaml:"target"` // string or int
	Listen int         `yaml:"listen"`
}

type routesFile struct {
	Routes      []staticRouteConfig         `yaml:"routes"`
	Passthrough []provider.PassthroughDomain `yaml:"passthrough"`
	TCP         []staticTcpRouteConfig      `yaml:"tcp"`
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

func (f *FileProvider) loadFile() provider.Message {
	msg := provider.Message{
		ProviderName: "file",
	}

	data, err := os.ReadFile(f.filePath)
	if err != nil {
		logger.Errorf("Failed to load routes file: %s: %v", f.filePath, err)
		return msg
	}

	var parsed routesFile
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		logger.Errorf("Failed to parse routes file: %s: %v", f.filePath, err)
		return msg
	}

	for _, r := range parsed.Routes {
		path := r.Path
		if path == "" {
			path = "/"
		}
		msg.Routes = append(msg.Routes, provider.Route{
			Hostname:  r.Host,
			Path:      path,
			Target:    f.resolveTarget(r.Target),
			StripPath: r.Strip,
			Source:    "static",
		})
	}

	msg.Passthrough = parsed.Passthrough

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

	return msg
}

func (f *FileProvider) Run(ctx context.Context, configCh chan<- provider.Message) error {
	// Initial load
	msg := f.loadFile()
	logger.Infof("Loaded %d static route(s), %d TCP route(s), %d passthrough domain(s)",
		len(msg.Routes), len(msg.TcpRoutes), len(msg.Passthrough))
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
