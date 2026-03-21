package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/steven-pribilinskiy/local-proxy/internal/config"
	"github.com/steven-pribilinskiy/local-proxy/internal/provider"
	"github.com/steven-pribilinskiy/local-proxy/internal/provider/docker"
	"github.com/steven-pribilinskiy/local-proxy/internal/router"
	"github.com/steven-pribilinskiy/local-proxy/internal/stats"
)

type Handler struct {
	router         *router.Router
	stats          *stats.Collector
	dockerProv     *docker.DockerProvider
	cfg            *config.Config
	getTcpRoutes   func() []provider.TcpRoute
	getPassthrough func() []provider.PassthroughDomain
}

func NewHandler(r *router.Router, s *stats.Collector, dp *docker.DockerProvider, cfg *config.Config, getTcpRoutes func() []provider.TcpRoute, getPassthrough func() []provider.PassthroughDomain) *Handler {
	return &Handler{
		router:         r,
		stats:          s,
		dockerProv:     dp,
		cfg:            cfg,
		getTcpRoutes:   getTcpRoutes,
		getPassthrough: getPassthrough,
	}
}

func jsonResponse(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func passthroughDomainList(passthrough []provider.PassthroughDomain) []string {
	var domains []string
	for _, pt := range passthrough {
		domains = append(domains, "*."+pt.Domain)
	}
	return domains
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// CORS preflight
	if r.Method == "OPTIONS" {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "content-type")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch r.URL.Path {
	case "/api/topology":
		h.handleTopology(w, r)
	case "/api/stats":
		h.handleStats(w, r)
	case "/api/requests":
		h.handleRequests(w, r)
	case "/api/health":
		jsonResponse(w, map[string]string{"status": "ok"}, http.StatusOK)
	default:
		// Not an API request — return false to allow dashboard serving
		http.NotFound(w, r)
	}
}

func (h *Handler) IsAPIRequest(path string) bool {
	switch path {
	case "/api/topology", "/api/stats", "/api/requests", "/api/health":
		return true
	}
	return false
}

func (h *Handler) handleTopology(w http.ResponseWriter, r *http.Request) {
	routes := h.router.GetAllRoutes()
	traefikIP, traefikPort := h.dockerProv.GetTraefikTarget()
	dockerRoutes := h.dockerProv.GetDockerRoutes()
	tcpRoutes := h.getTcpRoutes()

	// Filter static routes
	var staticRoutes []provider.Route
	for _, route := range routes {
		if route.Source == "static" {
			staticRoutes = append(staticRoutes, route)
		}
	}

	mode := "host-native"
	if h.cfg.HostAddress == "host.docker.internal" {
		mode = "docker"
	}

	type topologyRoute struct {
		Hostname      string `json:"hostname"`
		Path          string `json:"path"`
		Target        string `json:"target"`
		StripPath     bool   `json:"stripPath"`
		Source        string `json:"source"`
		ContainerName string `json:"containerName,omitempty"`
	}

	type topologyContainer struct {
		Name     string `json:"name"`
		Hostname string `json:"hostname"`
		Target   string `json:"target"`
		Source   string `json:"source"`
	}

	type topologyStaticRoute struct {
		Hostname string `json:"hostname"`
		Target   string `json:"target"`
		Source   string `json:"source"`
	}

	type topologyTcpRoute struct {
		Hostname      string `json:"hostname"`
		ListenPort    int    `json:"listenPort"`
		Target        string `json:"target"`
		Source        string `json:"source"`
		ContainerName string `json:"containerName,omitempty"`
	}

	var traefikIPPtr *string
	if traefikIP != "" {
		traefikIPPtr = &traefikIP
	}

	topo := map[string]interface{}{
		"mode":         mode,
		"sniRouter":    map[string]int{"port": 9443, "listenPort": 443},
		"httpsServer":  map[string]int{"port": 9444},
		"httpRedirect": map[string]int{"port": h.cfg.HTTPPort, "redirectPort": 80},
		"traefik": map[string]interface{}{
			"ip":      traefikIPPtr,
			"port":    traefikPort,
			"domains": passthroughDomainList(h.getPassthrough()),
		},
	}

	// Routes
	var topoRoutes []topologyRoute
	for _, route := range routes {
		topoRoutes = append(topoRoutes, topologyRoute{
			Hostname:      route.Hostname,
			Path:          route.Path,
			Target:        route.Target,
			StripPath:     route.StripPath,
			Source:        route.Source,
			ContainerName: route.ContainerName,
		})
	}
	topo["routes"] = topoRoutes

	// Containers
	var containers []topologyContainer
	for _, route := range dockerRoutes {
		name := route.ContainerName
		if name == "" {
			name = "unknown"
		}
		containers = append(containers, topologyContainer{
			Name:     name,
			Hostname: route.Hostname,
			Target:   route.Target,
			Source:    "docker",
		})
	}
	topo["containers"] = containers

	// Static routes
	var statics []topologyStaticRoute
	for _, route := range staticRoutes {
		statics = append(statics, topologyStaticRoute{
			Hostname: route.Hostname,
			Target:   route.Target,
			Source:    "static",
		})
	}
	topo["staticRoutes"] = statics

	// TCP routes
	var tcps []topologyTcpRoute
	for _, route := range tcpRoutes {
		tcps = append(tcps, topologyTcpRoute{
			Hostname:      route.Hostname,
			ListenPort:    route.ListenPort,
			Target:        route.TargetHost + ":" + strconv.Itoa(route.TargetPort),
			Source:        route.Source,
			ContainerName: route.ContainerName,
		})
	}
	topo["tcpRoutes"] = tcps

	jsonResponse(w, topo, http.StatusOK)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]interface{}{
		"uptime":        h.stats.GetUptime(),
		"totalRequests": h.stats.GetTotalRequests(),
		"hostStats":     h.stats.GetHostStats(),
		"edgeStats":     h.stats.GetEdgeStats(),
	}, http.StatusOK)
}

func (h *Handler) handleRequests(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
	}
	jsonResponse(w, h.stats.GetRecentRequests(limit), http.StatusOK)
}
