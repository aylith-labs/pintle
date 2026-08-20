package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/aylith-labs/pintle/internal/config"
	"github.com/aylith-labs/pintle/internal/provider"
	"github.com/aylith-labs/pintle/internal/provider/docker"
	"github.com/aylith-labs/pintle/internal/router"
	"github.com/aylith-labs/pintle/internal/stats"
)

// Runtime carries what pintle actually did at startup, as opposed to what its
// configuration asked for. Reporting these from constants is how the dashboard came
// to describe an architecture that was not running.
type Runtime struct {
	Version    string
	StartedAt  time.Time
	HTTPSPort  int
	SNIEnabled bool
	SNIPort    int
}

type Handler struct {
	router         *router.Router
	stats          *stats.Collector
	dockerProv     *docker.DockerProvider
	cfg            *config.Config
	rt             Runtime
	certDomains    func() []string
	getTcpRoutes   func() []provider.TcpRoute
	getPassthrough func() []provider.PassthroughDomain
	getExpected    func() []provider.ExpectedHost
}

func NewHandler(r *router.Router, s *stats.Collector, dp *docker.DockerProvider, cfg *config.Config, rt Runtime, certDomains func() []string, getTcpRoutes func() []provider.TcpRoute, getPassthrough func() []provider.PassthroughDomain, getExpected func() []provider.ExpectedHost) *Handler {
	return &Handler{
		router:         r,
		stats:          s,
		dockerProv:     dp,
		cfg:            cfg,
		rt:             rt,
		certDomains:    certDomains,
		getTcpRoutes:   getTcpRoutes,
		getPassthrough: getPassthrough,
		getExpected:    getExpected,
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
	case "/api/self":
		h.handleSelf(w, r)
	case "/api/health":
		jsonResponse(w, map[string]string{"status": "ok"}, http.StatusOK)
	default:
		// Not an API request — return false to allow dashboard serving
		http.NotFound(w, r)
	}
}

func (h *Handler) IsAPIRequest(path string) bool {
	switch path {
	case "/api/topology", "/api/stats", "/api/requests", "/api/health", "/api/self":
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
	if h.cfg.InDocker {
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
		"sniRouter":    sniTopology(h.rt),
		"httpsServer":  map[string]int{"port": h.rt.HTTPSPort},
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
			Source:   "docker",
		})
	}
	topo["containers"] = containers

	// Static routes
	var statics []topologyStaticRoute
	for _, route := range staticRoutes {
		statics = append(statics, topologyStaticRoute{
			Hostname: route.Hostname,
			Target:   route.Target,
			Source:   "static",
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

// sniTopology reports the SNI router only when one is actually running. With no
// passthrough domains the router is skipped and the HTTPS server binds the public
// port directly, so a constant here would describe a topology that does not exist.
func sniTopology(rt Runtime) interface{} {
	if !rt.SNIEnabled {
		return nil
	}
	return map[string]int{"port": rt.SNIPort, "listenPort": 443}
}

// Self reports what pintle IS, so nothing has to infer it from the outside. Every
// field is read from runtime state.
type Self struct {
	Version   string `json:"version"`
	UptimeSec int64  `json:"uptimeSec"`

	InDocker  bool                `json:"inDocker"`
	Container docker.SelfIdentity `json:"container"`
	// PidHostNote is stated rather than left to be re-derived: docker-compose.yaml sets
	// `pid: host`, so the containerised binary is visible to `pgrep pintle` on the host
	// and a PID hit is not evidence that pintle is running host-native.
	PidHostNote string `json:"pidHostNote,omitempty"`

	RoutesFile string `json:"routesFile"`
	// RoutesFileHost is the path on the HOST that RoutesFile is mounted from. A reader
	// outside the container needs this one; RoutesFile alone points at nothing there.
	RoutesFileHost string   `json:"routesFileHost,omitempty"`
	CertsDir       string   `json:"certsDir"`
	CertsDirHost   string   `json:"certsDirHost,omitempty"`
	CertDomains    []string `json:"certDomains"`
	BaseDomain     string   `json:"baseDomain"`
	DashboardHost  string   `json:"dashboardHost"`
	DockerNetwork  string   `json:"dockerNetwork"`

	LabelPrefix   string   `json:"labelPrefix"`
	LabelDialects []string `json:"labelDialects"`

	RestartCommand string `json:"restartCommand"`

	Expected []ExpectedStatus `json:"expected"`
}

// ExpectedStatus pairs a declared expectation with whether it is actually routed.
type ExpectedStatus struct {
	provider.ExpectedHost
	Routed bool `json:"routed"`
}

func (h *Handler) handleSelf(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	self := Self{
		Version:       h.rt.Version,
		UptimeSec:     int64(time.Since(h.rt.StartedAt).Seconds()),
		InDocker:      h.cfg.InDocker,
		RoutesFile:    h.cfg.RoutesFile,
		CertsDir:      h.cfg.CertsDir,
		BaseDomain:    h.cfg.BaseDomain,
		DashboardHost: h.cfg.DashboardHost,
		DockerNetwork: h.cfg.DockerNetwork,
		LabelPrefix:   "pintle",
		LabelDialects: []string{"pintle.*", "traefik.*", "caddy*"},
	}

	if h.certDomains != nil {
		self.CertDomains = h.certDomains()
	}

	if h.cfg.InDocker {
		self.Container = h.dockerProv.Identify(ctx)
		self.RoutesFileHost = self.Container.Mounts[h.cfg.RoutesFile]
		self.CertsDirHost = self.Container.Mounts[h.cfg.CertsDir]
		self.PidHostNote = "compose sets `pid: host`, so this binary is visible to `pgrep pintle` on the host — a PID hit is not evidence of a host-native run"
		self.RestartCommand = "docker compose up -d --build"
		if dir := self.Container.WorkingDir; dir != "" {
			self.RestartCommand = "cd " + dir + " && docker compose up -d --build"
		}
	} else {
		self.RestartCommand = "./pintle --port-redirect"
	}

	for _, exp := range h.expectedStatuses() {
		self.Expected = append(self.Expected, exp)
	}

	jsonResponse(w, self, http.StatusOK)
}

// expectedStatuses answers the question the route table alone cannot: which hosts this
// machine is supposed to serve and does not.
func (h *Handler) expectedStatuses() []ExpectedStatus {
	if h.getExpected == nil {
		return nil
	}
	var out []ExpectedStatus
	for _, exp := range h.getExpected() {
		out = append(out, ExpectedStatus{ExpectedHost: exp, Routed: h.router.HasHost(exp.Host)})
	}
	return out
}
