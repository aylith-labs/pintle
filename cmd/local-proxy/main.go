package main

import (
	"context"
	"net"
	"net/http"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/steven-pribilinskiy/local-proxy/internal/aggregator"
	"github.com/steven-pribilinskiy/local-proxy/internal/api"
	"github.com/steven-pribilinskiy/local-proxy/internal/config"
	"github.com/steven-pribilinskiy/local-proxy/internal/logger"
	"github.com/steven-pribilinskiy/local-proxy/internal/provider"
	"github.com/steven-pribilinskiy/local-proxy/internal/provider/docker"
	"github.com/steven-pribilinskiy/local-proxy/internal/provider/file"
	"github.com/steven-pribilinskiy/local-proxy/internal/proxy"
	"github.com/steven-pribilinskiy/local-proxy/internal/router"
	"github.com/steven-pribilinskiy/local-proxy/internal/server"
	"github.com/steven-pribilinskiy/local-proxy/internal/stats"
	tlsmgr "github.com/steven-pribilinskiy/local-proxy/internal/tls"
)

func main() {
	cfg := config.Load()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger.Info("local-proxy starting...")

	// Core components
	tlsManager := tlsmgr.NewManager()
	statsCollector := stats.NewCollector(cfg.DashboardHost)
	rtr := router.New()
	dockerProv := docker.New(cfg.DockerNetwork)

	// Configuration pipeline
	configCh := make(chan provider.Message, 4)
	agg, aggCh := aggregator.New()

	// Track all TCP routes for API
	var allTcpRoutes []provider.TcpRoute
	var tcpMu sync.RWMutex

	getTcpRoutes := func() []provider.TcpRoute {
		tcpMu.RLock()
		defer tcpMu.RUnlock()
		routes := make([]provider.TcpRoute, len(allTcpRoutes))
		copy(routes, allTcpRoutes)
		return routes
	}

	// TCP router (started dynamically based on config)
	tcpRouter := server.NewTCPRouter(nil, getTcpRoutes)

	// Start file provider
	fileProv := file.New(cfg.RoutesFile, cfg.HostAddress)
	go fileProv.Run(ctx, configCh)

	// Wait for initial file provider config (contains passthrough domains)
	initialMsg := <-configCh
	agg.Update(initialMsg)
	initialCfg := <-aggCh

	// Load TLS certs (needs passthrough from file provider)
	tlsManager.LoadCerts(cfg.CertsDir, cfg.BaseDomain, initialCfg.Passthrough)

	// Update router with initial routes
	rtr.Update(initialCfg.Routes)

	// Update TCP routes
	tcpMu.Lock()
	allTcpRoutes = initialCfg.TcpRoutes
	tcpMu.Unlock()

	// Start Docker provider
	go dockerProv.Run(ctx, configCh)

	// Configuration watcher: aggregates provider messages -> updates router + TLS
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-configCh:
				agg.Update(msg)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case merged := <-aggCh:
				rtr.Update(merged.Routes)
				tlsManager.LoadCerts(cfg.CertsDir, cfg.BaseDomain, merged.Passthrough)

				tcpMu.Lock()
				allTcpRoutes = merged.TcpRoutes
				tcpMu.Unlock()

				// Update TCP router certs
				rawCerts := tlsManager.GetRawCerts(cfg.CertsDir, cfg.BaseDomain, merged.Passthrough)
				var tcpCerts []server.TCPCert
				for _, rc := range rawCerts {
					tcpCerts = append(tcpCerts, server.TCPCert{
						Cert:   rc.Cert,
						Key:    rc.Key,
						Domain: rc.Domain,
					})
				}
				tcpRouter.UpdateCerts(tcpCerts)

				// Start TCP listeners for any new ports
				ports := make(map[int]bool)
				for _, r := range merged.TcpRoutes {
					ports[r.ListenPort] = true
				}
				for port := range ports {
					tcpRouter.StartPort(ctx, port)
				}
			}
		}
	}()

	// Build HTTP handler mux
	apiHandler := api.NewHandler(rtr, statsCollector, dockerProv, cfg, getTcpRoutes, agg.GetCurrentPassthrough)
	dashboardHandler := api.NewDashboardHandler(cfg.ViteDevURL)
	proxyHandler := proxy.NewHandler(rtr, statsCollector)

	// Main HTTPS handler: dashboard host goes to API/UI, everything else to proxy
	mainHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hostname := strings.Split(r.Host, ":")[0]

		if hostname == cfg.DashboardHost {
			// API endpoints
			if apiHandler.IsAPIRequest(r.URL.Path) || r.Method == "OPTIONS" {
				apiHandler.ServeHTTP(w, r)
				return
			}

			// Dashboard UI (handles WebSocket upgrades natively in dev mode)
			dashboardHandler.ServeHTTP(w, r)
			return
		}

		// Regular proxy (httputil.ReverseProxy handles WebSocket upgrades natively)
		proxyHandler.ServeHTTP(w, r)
	})

	// Determine if SNI router is needed
	passthroughDomains := initialCfg.Passthrough
	needsSNI := len(passthroughDomains) > 0

	var httpsPort int
	var httpsHostname string

	if needsSNI {
		httpsPort = 9444 // Internal port behind SNI router
		httpsHostname = "127.0.0.1"
	} else {
		httpsPort = cfg.ListenPort
		httpsHostname = "0.0.0.0"
	}

	// Start HTTPS server
	if err := server.StartHTTPS(ctx, httpsPort, httpsHostname, tlsManager, mainHandler); err != nil {
		logger.Errorf("Failed to start HTTPS server: %v", err)
		return
	}

	// Start HTTP redirect
	if err := server.StartHTTPRedirect(ctx, cfg.HTTPPort); err != nil {
		logger.Errorf("Failed to start HTTP redirect: %v", err)
		return
	}

	// Start SNI router if needed
	if needsSNI {
		sniRouter := &server.SNIRouter{
			Port: cfg.ListenPort,
			LocalTarget: &net.TCPAddr{
				IP:   net.ParseIP("127.0.0.1"),
				Port: 9444,
			},
			ForwardTargets: buildSNITargets(passthroughDomains, dockerProv, cfg),
			HasLocalRoute:  rtr.HasHost,
		}
		go sniRouter.Start(ctx)
	}

	// Start initial TCP routers
	rawCerts := tlsManager.GetRawCerts(cfg.CertsDir, cfg.BaseDomain, passthroughDomains)
	var tcpCerts []server.TCPCert
	for _, rc := range rawCerts {
		tcpCerts = append(tcpCerts, server.TCPCert{
			Cert:   rc.Cert,
			Key:    rc.Key,
			Domain: rc.Domain,
		})
	}
	tcpRouter.UpdateCerts(tcpCerts)

	ports := make(map[int]bool)
	for _, r := range initialCfg.TcpRoutes {
		ports[r.ListenPort] = true
	}
	for port := range ports {
		if err := tcpRouter.StartPort(ctx, port); err != nil {
			logger.Errorf("Failed to start TCP router on :%d: %v", port, err)
		}
	}
	if len(ports) == 0 {
		logger.Info("No TCP routes discovered, skipping TCP routers")
	}

	logger.Infof("local-proxy ready on *.%s (dashboard: %s)", cfg.BaseDomain, cfg.DashboardHost)

	// Wait for shutdown
	<-ctx.Done()
	logger.Info("Shutting down...")
}

func buildSNITargets(passthrough []provider.PassthroughDomain, dockerProv *docker.DockerProvider, cfg *config.Config) []server.SNIForwardTarget {
	var targets []server.SNIForwardTarget

	for _, pt := range passthrough {
		domain := pt.Domain
		targets = append(targets, server.SNIForwardTarget{
			Match: func(hostname string) bool {
				return strings.HasSuffix(hostname, "."+domain) || hostname == domain
			},
			Resolve: func() *net.TCPAddr {
				ip, port := dockerProv.GetTraefikTarget()
				if ip == "" {
					return nil
				}
				return &net.TCPAddr{
					IP:   net.ParseIP(ip),
					Port: port,
				}
			},
			Label: "*." + domain + " -> " + pt.Target + " container",
		})
	}

	return targets
}
