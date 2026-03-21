package router

import (
	"sort"
	"strings"
	"sync"

	"github.com/steven-pribilinskiy/local-proxy/internal/logger"
	"github.com/steven-pribilinskiy/local-proxy/internal/provider"
)

type ResolveResult struct {
	Target        string
	RewrittenPath string
	Route         provider.Route
}

type Router struct {
	mu     sync.RWMutex
	routes map[string][]provider.Route
}

func New() *Router {
	return &Router{
		routes: make(map[string][]provider.Route),
	}
}

func (r *Router) GetAllRoutes() []provider.Route {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var all []provider.Route
	for _, routes := range r.routes {
		all = append(all, routes...)
	}
	return all
}

func (r *Router) Update(routes []provider.Route) {
	r.mu.Lock()
	defer r.mu.Unlock()

	prev := r.routes
	next := make(map[string][]provider.Route)

	for _, route := range routes {
		next[route.Hostname] = append(next[route.Hostname], route)
	}

	// Sort by path specificity (longest first)
	for _, hostRoutes := range next {
		sort.Slice(hostRoutes, func(i, j int) bool {
			return len(hostRoutes[i].Path) > len(hostRoutes[j].Path)
		})
	}

	// Log changes
	prevSet := make(map[string]struct{})
	for _, hostRoutes := range prev {
		for _, route := range hostRoutes {
			prevSet[route.Hostname+route.Path+"->"+route.Target] = struct{}{}
		}
	}

	nextSet := make(map[string]struct{})
	for _, route := range routes {
		key := route.Hostname + route.Path + "->" + route.Target
		nextSet[key] = struct{}{}
		if _, exists := prevSet[key]; !exists {
			logger.RouteChange("add", route.Hostname, route.Path, route.Target)
		}
	}

	for _, hostRoutes := range prev {
		for _, route := range hostRoutes {
			key := route.Hostname + route.Path + "->" + route.Target
			if _, exists := nextSet[key]; !exists {
				logger.RouteChange("remove", route.Hostname, route.Path, route.Target)
			}
		}
	}

	r.routes = next
}

func (r *Router) Resolve(hostname, path string) *ResolveResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routes, ok := r.routes[hostname]
	if !ok {
		return nil
	}

	for _, route := range routes {
		if route.Path == "/" || path == route.Path || strings.HasPrefix(path, route.Path+"/") {
			rewrittenPath := path
			if route.StripPath {
				rewrittenPath = strings.TrimPrefix(path, route.Path)
				if rewrittenPath == "" {
					rewrittenPath = "/"
				}
			}
			return &ResolveResult{
				Target:        route.Target,
				RewrittenPath: rewrittenPath,
				Route:         route,
			}
		}
	}

	return nil
}
