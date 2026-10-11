package router

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/aylith-labs/pintle/internal/logger"
	"github.com/aylith-labs/pintle/internal/provider"
)

type ResolveResult struct {
	Unavailable   bool
	Target        string
	RewrittenPath string
	Route         provider.Route
}

// regexRoute holds the compiled form of a Traefik HostRegexp rule and the
// routes that share its pattern, sorted by path specificity.
type regexRoute struct {
	re     *regexp.Regexp
	routes []provider.Route
}

type Router struct {
	mu               sync.RWMutex
	routes           map[string][]provider.Route
	regexRoutes      []regexRoute
	pools            map[string]*atomic.Uint64
	health           map[string]*healthState
	healthContext    context.Context
	healthCancel     context.CancelFunc
	healthParent     context.Context
	healthRunning    bool
	healthGeneration uint64
	healthWake       chan struct{}
}

func New() *Router {
	return &Router{
		routes:       make(map[string][]provider.Route),
		pools:        make(map[string]*atomic.Uint64),
		health:       make(map[string]*healthState),
		healthParent: context.Background(),
		healthWake:   make(chan struct{}, 1),
	}
}

func (r *Router) GetAllRoutes() []provider.Route {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.flattenLocked()
}

// flattenLocked returns every route (exact + regex). Caller must hold r.mu.
func (r *Router) flattenLocked() []provider.Route {
	var all []provider.Route
	for _, routes := range r.routes {
		all = append(all, routes...)
	}
	for _, rr := range r.regexRoutes {
		all = append(all, rr.routes...)
	}
	return all
}

// HasHost reports whether a route is configured for the hostname, matching
// both exact-host routes and HostRegexp routes.
func (r *Router) HasHost(hostname string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.routes[hostname]) > 0 {
		return true
	}
	for _, rr := range r.regexRoutes {
		if rr.re.MatchString(hostname) {
			return true
		}
	}
	return false
}

func (r *Router) Update(routes []provider.Route) {
	r.mu.Lock()
	defer r.mu.Unlock()

	prevRoutes := r.flattenLocked()

	next := make(map[string][]provider.Route)
	var nextRegex []regexRoute
	regexIndex := make(map[string]int) // pattern -> index in nextRegex

	for _, route := range routes {
		if route.Health != nil {
			copied := *route.Health
			route.Health = &copied
		}
		if route.IsRegexp {
			idx, ok := regexIndex[route.Hostname]
			if !ok {
				compiled, err := regexp.Compile(route.Hostname)
				if err != nil {
					logger.Errorf("invalid HostRegexp %q: %v", route.Hostname, err)
					continue
				}
				nextRegex = append(nextRegex, regexRoute{re: compiled})
				idx = len(nextRegex) - 1
				regexIndex[route.Hostname] = idx
			}
			nextRegex[idx].routes = append(nextRegex[idx].routes, route)
			continue
		}
		next[route.Hostname] = append(next[route.Hostname], route)
	}

	// Sort by path specificity (longest first) so exact prefixes win.
	byPathDesc := func(routes []provider.Route) {
		sort.SliceStable(routes, func(i, j int) bool {
			return len(routes[i].Path) > len(routes[j].Path)
		})
	}
	for _, hostRoutes := range next {
		byPathDesc(hostRoutes)
	}
	for i := range nextRegex {
		byPathDesc(nextRegex[i].routes)
	}

	// Log changes
	routeKey := func(route provider.Route) string {
		return route.Hostname + route.Path + "->" + route.Target
	}
	prevSet := make(map[string]struct{}, len(prevRoutes))
	for _, route := range prevRoutes {
		prevSet[routeKey(route)] = struct{}{}
	}
	nextSet := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		key := routeKey(route)
		nextSet[key] = struct{}{}
		if _, exists := prevSet[key]; !exists {
			logger.RouteChange("add", route.Hostname, route.Path, route.Target)
		}
	}
	for _, route := range prevRoutes {
		if _, exists := nextSet[routeKey(route)]; !exists {
			logger.RouteChange("remove", route.Hostname, route.Path, route.Target)
		}
	}

	r.pools = make(map[string]*atomic.Uint64)
	for _, route := range routes {
		if route.BackendPool != "" {
			r.pools[route.Hostname+"\x00"+route.BackendPool] = &atomic.Uint64{}
		}
	}
	r.routes = next
	r.regexRoutes = nextRegex
	r.resetHealthLocked()
}

func (r *Router) Resolve(hostname, path string) *ResolveResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Exact-host routes take precedence over regex routes.
	if result := r.matchPool(r.routes[hostname], path); result != nil {
		return result
	}
	for _, rr := range r.regexRoutes {
		if rr.re.MatchString(hostname) {
			if result := r.matchPool(rr.routes, path); result != nil {
				return result
			}
		}
	}
	return nil
}

// matchRoutes returns the first route whose path matches, applying StripPath.
// routes must be pre-sorted by path specificity (longest first).
func matchRoutes(routes []provider.Route, path string) *ResolveResult {
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

// Each admitted request selects exactly one backend. Failures never replay a request.
func (r *Router) matchPool(routes []provider.Route, path string) *ResolveResult {
	result := matchRoutes(routes, path)
	if result == nil || result.Route.BackendPool == "" {
		return result
	}
	var members []provider.Route
	for _, route := range routes {
		if route.Hostname == result.Route.Hostname && route.BackendPool == result.Route.BackendPool && route.Path == result.Route.Path && route.StripPath == result.Route.StripPath && route.H2C == result.Route.H2C {
			if route.Health != nil {
				state := r.health[healthKey(route)]
				if state == nil || !state.healthy {
					continue
				}
			}
			members = append(members, route)
		}
	}
	counter := r.pools[result.Route.Hostname+"\x00"+result.Route.BackendPool]
	if counter == nil || len(members) == 0 {
		result.Unavailable = true
		result.Target = ""
		return result
	}
	selected := members[(counter.Add(1)-1)%uint64(len(members))]
	result.Target = selected.Target
	result.Route = selected
	return result
}
