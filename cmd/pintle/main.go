package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aylith-labs/pintle/internal/aggregator"
	"github.com/aylith-labs/pintle/internal/api"
	"github.com/aylith-labs/pintle/internal/config"
	"github.com/aylith-labs/pintle/internal/logger"
	"github.com/aylith-labs/pintle/internal/provider"
	"github.com/aylith-labs/pintle/internal/provider/docker"
	"github.com/aylith-labs/pintle/internal/provider/file"
	"github.com/aylith-labs/pintle/internal/proxy"
	"github.com/aylith-labs/pintle/internal/router"
	"github.com/aylith-labs/pintle/internal/server"
	"github.com/aylith-labs/pintle/internal/stats"
	tlsmgr "github.com/aylith-labs/pintle/internal/tls"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "doctor" {
		runDoctor()
		return
	}

	startedAt := time.Now()
	cfg, err := config.Load()
	if err == nil {
		err = run(cfg, startedAt)
	}
	if err != nil {
		logger.Errorf("pintle: %v", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config, startedAt time.Time) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	logger.Info("pintle starting...")

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
	var tcpRouter *server.TCPRouter
	if !cfg.StaticOnly {
		tcpRouter = server.NewTCPRouter(nil, getTcpRoutes)
	}

	// Start file provider
	fileProv := file.New(cfg.RoutesFile, cfg.HostAddress)
	go fileProv.Run(ctx, configCh)

	// Wait for initial file provider config (contains passthrough domains)
	initialMsg := <-configCh
	if err := cfg.ValidateMessage(initialMsg); err != nil {
		return err
	}
	agg.Update(initialMsg)
	initialCfg := <-aggCh

	// Load TLS certs (needs passthrough from file provider)
	tlsManager.LoadCerts(cfg.CertsDir, cfg.BaseDomain)

	// Update router with initial routes
	rtr.Update(initialCfg.Routes)

	// Update TCP routes
	tcpMu.Lock()
	allTcpRoutes = initialCfg.TcpRoutes
	tcpMu.Unlock()

	// Start Docker provider
	if !cfg.StaticOnly {
		go dockerProv.Run(ctx, configCh)
	} else {
		logger.Info("Static-only loopback mode: Docker discovery, TCP routes and SNI passthrough disabled")
	}

	// Configuration watcher: aggregates provider messages -> updates router + TLS
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-configCh:
				if err := cfg.ValidateMessage(msg); err != nil {
					logger.Errorf("Rejected route update (keeping last accepted routes): %v", err)
					continue
				}
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
				tlsManager.LoadCerts(cfg.CertsDir, cfg.BaseDomain)
				if cfg.StaticOnly {
					continue
				}

				tcpMu.Lock()
				allTcpRoutes = merged.TcpRoutes
				tcpMu.Unlock()

				// Update TCP router certs
				rawCerts := tlsManager.GetRawCerts(cfg.CertsDir, cfg.BaseDomain)
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

	// Decide the listener topology before anything reports it, so the API describes
	// what actually started rather than what the defaults would have been.
	passthroughDomains := initialCfg.Passthrough
	needsSNI := !cfg.StaticOnly && len(passthroughDomains) > 0

	httpsPort := cfg.ListenPort
	httpsHostname := "0.0.0.0"
	if needsSNI {
		httpsPort = 9444 // internal port behind the SNI router
		httpsHostname = "127.0.0.1"
	} else if cfg.StaticOnly {
		httpsHostname = cfg.LoopbackAddress
	}

	runtimeFacts := api.Runtime{
		Version:    version,
		StartedAt:  startedAt,
		HTTPSPort:  httpsPort,
		SNIEnabled: needsSNI,
		SNIPort:    cfg.ListenPort,
	}

	// Build HTTP handler mux
	apiHandler := api.NewHandler(rtr, statsCollector, dockerProv, cfg, runtimeFacts,
		tlsManager.LoadedDomains, getTcpRoutes, agg.GetCurrentPassthrough, agg.GetCurrentExpected)
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

	// Start HTTPS server
	if err := server.StartHTTPS(ctx, httpsPort, httpsHostname, tlsManager, mainHandler); err != nil {
		return fmt.Errorf("failed to start HTTPS server: %w", err)
	}

	// Start HTTP redirect
	var httpErr error
	if cfg.StaticOnly {
		httpErr = server.StartLoopbackHTTPRedirect(ctx, cfg.HTTPPort, cfg.LoopbackAddress, cfg.ListenPort)
	} else {
		httpErr = server.StartHTTPRedirect(ctx, cfg.HTTPPort)
	}
	if httpErr != nil {
		return fmt.Errorf("failed to start HTTP redirect: %w", httpErr)
	}

	// Start SNI router if needed
	if needsSNI {
		sniRouter := &server.SNIRouter{
			Port:       cfg.ListenPort,
			BaseDomain: cfg.BaseDomain,
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
	if !cfg.StaticOnly {
		rawCerts := tlsManager.GetRawCerts(cfg.CertsDir, cfg.BaseDomain)
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
	}

	logger.Infof("pintle ready on *.%s (dashboard: %s)", cfg.BaseDomain, cfg.DashboardHost)

	// Wait for shutdown
	<-ctx.Done()
	logger.Info("Shutting down...")
	return nil
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
