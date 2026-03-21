package docker

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/steven-pribilinskiy/local-proxy/internal/config"
	"github.com/steven-pribilinskiy/local-proxy/internal/logger"
	"github.com/steven-pribilinskiy/local-proxy/internal/provider"
)

type DockerProvider struct {
	networkName string
	mu          sync.RWMutex
	traefikIP   string
	dockerRoutes    []provider.Route
	dockerTcpRoutes []provider.TcpRoute
}

func New(networkName string) *DockerProvider {
	return &DockerProvider{
		networkName: networkName,
	}
}

func (d *DockerProvider) GetTraefikTarget() (string, int) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.traefikIP == "" {
		return "", 0
	}
	return d.traefikIP, 443
}

func (d *DockerProvider) GetDockerRoutes() []provider.Route {
	d.mu.RLock()
	defer d.mu.RUnlock()
	routes := make([]provider.Route, len(d.dockerRoutes))
	copy(routes, d.dockerRoutes)
	return routes
}

func (d *DockerProvider) GetDockerTcpRoutes() []provider.TcpRoute {
	d.mu.RLock()
	defer d.mu.RUnlock()
	routes := make([]provider.TcpRoute, len(d.dockerTcpRoutes))
	copy(routes, d.dockerTcpRoutes)
	return routes
}

func (d *DockerProvider) resolveContainerRoute(info types.Container, parsed *parsedLabels, source string) []provider.Route {
	networks := info.NetworkSettings.Networks
	networkInfo, ok := networks[d.networkName]
	name := "unknown"
	if len(info.Names) > 0 {
		name = strings.TrimPrefix(info.Names[0], "/")
	}

	if !ok || networkInfo.IPAddress == "" {
		logger.Errorf("Container %s (%s) has no IP on network '%s'", name, source, d.networkName)
		return nil
	}

	port := parsed.Port
	if port == 0 && len(info.Ports) > 0 {
		port = int(info.Ports[0].PrivatePort)
	}
	if port == 0 {
		logger.Errorf("Container %s (%s) has no port configured", name, source)
		return nil
	}

	target := "http://" + networkInfo.IPAddress + ":" + strconv.Itoa(port)

	var routes []provider.Route
	for _, hostname := range parsed.Hosts {
		routes = append(routes, provider.Route{
			Hostname:      hostname,
			Path:          parsed.Path,
			Target:        target,
			StripPath:     parsed.Strip,
			Source:        source,
			ContainerName: name,
		})
	}
	return routes
}

func (d *DockerProvider) discoverRoutes(ctx context.Context, cli *client.Client) ([]provider.Route, []provider.TcpRoute) {
	// 1. Discover proxy-labeled containers
	proxyContainers, err := cli.ContainerList(ctx, types.ContainerListOptions{
		Filters: filters.NewArgs(filters.Arg("label", "local-proxy.host")),
	})
	if err != nil {
		logger.Error("Failed to list proxy containers", err)
		return nil, nil
	}

	var routes []provider.Route
	proxyContainerNames := make(map[string]bool)

	for _, info := range proxyContainers {
		parsed := parseProxyLabels(info.Labels)
		if parsed == nil {
			continue
		}
		containerRoutes := d.resolveContainerRoute(info, parsed, "docker")
		routes = append(routes, containerRoutes...)
		name := "unknown"
		if len(info.Names) > 0 {
			name = strings.TrimPrefix(info.Names[0], "/")
		}
		proxyContainerNames[name] = true
	}

	// 2. Discover traefik-labeled containers
	traefikContainers, err := cli.ContainerList(ctx, types.ContainerListOptions{
		Filters: filters.NewArgs(filters.Arg("label", "traefik.enable=true")),
	})
	if err != nil {
		logger.Error("Failed to list traefik containers", err)
		return routes, nil
	}

	handledContainers := make(map[string]bool)
	for k, v := range proxyContainerNames {
		handledContainers[k] = v
	}

	var tcpRoutes []provider.TcpRoute

	for _, info := range traefikContainers {
		name := "unknown"
		if len(info.Names) > 0 {
			name = strings.TrimPrefix(info.Names[0], "/")
		}

		if proxyContainerNames[name] {
			continue
		}

		// Skip Traefik container itself
		if strings.Contains(info.Image, "traefik") {
			continue
		}

		// HTTP routes
		parsed := parseTraefikLabels(info.Labels)
		if parsed != nil {
			containerRoutes := d.resolveContainerRoute(info, parsed, "traefik")
			routes = append(routes, containerRoutes...)
			handledContainers[name] = true
		}

		// TCP routes
		tcpParsed := parseTraefikTcpLabels(info.Labels)
		if tcpParsed != nil {
			listenPort, ok := config.TCPEntrypoints[tcpParsed.Entrypoint]
			if !ok {
				continue
			}
			networks := info.NetworkSettings.Networks
			networkInfo, ok := networks[d.networkName]
			if !ok || networkInfo.IPAddress == "" {
				continue
			}
			tcpRoutes = append(tcpRoutes, provider.TcpRoute{
				Hostname:      tcpParsed.Hostname,
				TargetHost:    networkInfo.IPAddress,
				TargetPort:    tcpParsed.ContainerPort,
				ListenPort:    listenPort,
				Source:        "traefik",
				ContainerName: name,
			})
		}
	}

	// 3. Discover caddy-labeled containers
	allContainers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		logger.Error("Failed to list all containers", err)
		return routes, tcpRoutes
	}

	for _, info := range allContainers {
		name := "unknown"
		if len(info.Names) > 0 {
			name = strings.TrimPrefix(info.Names[0], "/")
		}

		if handledContainers[name] {
			continue
		}

		// Check for caddy labels
		hasCaddy := false
		for key := range info.Labels {
			if caddyPrefixRe.MatchString(key) {
				hasCaddy = true
				break
			}
		}
		if !hasCaddy {
			continue
		}

		parsedList := parseCaddyLabels(info.Labels)
		for _, parsed := range parsedList {
			containerRoutes := d.resolveContainerRoute(info, &parsed, "caddy")
			routes = append(routes, containerRoutes...)
		}
	}

	// Log summary
	proxyCount := 0
	traefikCount := 0
	caddyCount := 0
	for _, r := range routes {
		switch r.Source {
		case "docker":
			proxyCount++
		case "traefik":
			traefikCount++
		case "caddy":
			caddyCount++
		}
	}

	var counts []string
	if proxyCount > 0 {
		counts = append(counts, strconv.Itoa(proxyCount)+" proxy")
	}
	if traefikCount > 0 {
		counts = append(counts, strconv.Itoa(traefikCount)+" traefik")
	}
	if caddyCount > 0 {
		counts = append(counts, strconv.Itoa(caddyCount)+" caddy")
	}
	if len(counts) > 1 {
		logger.Infof("Discovered %s route(s)", strings.Join(counts, " + "))
	}

	return routes, tcpRoutes
}

func (d *DockerProvider) discoverTraefik(ctx context.Context, cli *client.Client) {
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return
	}

	for _, c := range containers {
		if strings.Contains(c.Image, "traefik") {
			networks := c.NetworkSettings.Networks
			if networkInfo, ok := networks[d.networkName]; ok && networkInfo.IPAddress != "" {
				d.mu.Lock()
				oldIP := d.traefikIP
				d.traefikIP = networkInfo.IPAddress
				d.mu.Unlock()
				if oldIP != d.traefikIP {
					logger.Infof("Traefik container IP: %s (network '%s')", d.traefikIP, d.networkName)
				}
				return
			}
		}
	}

	d.mu.Lock()
	if d.traefikIP != "" {
		logger.Info("Traefik container not found, clearing cached IP")
		d.traefikIP = ""
	}
	d.mu.Unlock()
}

func (d *DockerProvider) Run(ctx context.Context, configCh chan<- provider.Message) error {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		logger.Error("Failed to create Docker client", err)
		<-ctx.Done()
		return nil
	}
	defer cli.Close()

	// Initial discovery
	routes, tcpRoutes := d.discoverRoutes(ctx, cli)
	d.discoverTraefik(ctx, cli)

	d.mu.Lock()
	d.dockerRoutes = routes
	d.dockerTcpRoutes = tcpRoutes
	d.mu.Unlock()

	logger.Infof("Discovered %d HTTP + %d TCP Docker route(s) on network '%s'",
		len(routes), len(tcpRoutes), d.networkName)

	configCh <- provider.Message{
		ProviderName: "docker",
		Routes:       routes,
		TcpRoutes:    tcpRoutes,
	}

	// Watch for container events
	d.watchEvents(ctx, cli, configCh)
	return nil
}

func (d *DockerProvider) watchEvents(ctx context.Context, cli *client.Client, configCh chan<- provider.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		eventCh, errCh := cli.Events(ctx, types.EventsOptions{
			Filters: filters.NewArgs(
				filters.Arg("type", "container"),
				filters.Arg("event", "start"),
				filters.Arg("event", "stop"),
				filters.Arg("event", "die"),
				filters.Arg("event", "destroy"),
			),
		})

		logger.Info("Watching Docker events for container changes")

		var debounceTimer *time.Timer

		for {
			select {
			case <-ctx.Done():
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				return
			case _, ok := <-eventCh:
				if !ok {
					goto reconnect
				}
				// Debounce: reset timer on each event
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(300*time.Millisecond, func() {
					routes, tcpRoutes := d.discoverRoutes(ctx, cli)
					d.discoverTraefik(ctx, cli)

					d.mu.Lock()
					d.dockerRoutes = routes
					d.dockerTcpRoutes = tcpRoutes
					d.mu.Unlock()

					configCh <- provider.Message{
						ProviderName: "docker",
						Routes:       routes,
						TcpRoutes:    tcpRoutes,
					}
				})
			case err, ok := <-errCh:
				if !ok {
					goto reconnect
				}
				logger.Error("Docker event stream error", err)
				goto reconnect
			}
		}

	reconnect:
		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		logger.Info("Docker event stream disconnected, reconnecting in 5s...")
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

// Ensure DockerProvider doesn't actually use the events.Message
var _ events.Message
