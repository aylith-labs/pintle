package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/aylith-labs/pintle/internal/provider"
)

type healthState struct {
	route     provider.Route
	healthy   bool
	next      time.Time
	checkedAt time.Time
}

func healthKey(route provider.Route) string {
	return route.Hostname + "\x00" + route.BackendPool + "\x00" + route.Path + "\x00" + route.Target
}

// Caller holds the route lock. Every reload invalidates earlier probe results.
func (r *Router) resetHealthLocked() {
	r.healthGeneration++
	if r.healthCancel != nil {
		r.healthCancel()
	}
	r.healthContext, r.healthCancel = context.WithCancel(r.healthParent)
	r.health = make(map[string]*healthState)
	for _, route := range r.flattenLocked() {
		if route.Health != nil && route.BackendPool != "" && route.Source == "static" {
			r.health[healthKey(route)] = &healthState{route: route}
		}
	}
	select {
	case r.healthWake <- struct{}{}:
	default:
	}
}

// RunHealth owns at most four concurrent read-only probes. It never sends user
// request bodies or credentials, follows redirects, or retries a user request.
// The interval is a minimum between probe completions; a busy sweep may run later.
// The process context and reload context cancel in-flight standard HTTP requests.
func (r *Router) RunHealth(ctx context.Context, transport http.RoundTripper) {
	r.mu.Lock()
	if r.healthRunning {
		r.mu.Unlock()
		return
	}
	r.healthRunning = true
	r.healthParent = ctx
	r.resetHealthLocked()
	r.mu.Unlock()
	if transport == nil {
		owned := &http.Transport{Proxy: nil, MaxIdleConns: 4, MaxIdleConnsPerHost: 1, IdleConnTimeout: 30 * time.Second, ResponseHeaderTimeout: 10 * time.Second, MaxResponseHeaderBytes: 16 * 1024, DisableKeepAlives: true}
		defer owned.CloseIdleConnections()
		transport = owned
	}
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer func() {
		r.mu.Lock()
		r.healthCancel()
		for _, state := range r.health {
			state.healthy = false
		}
		r.healthRunning = false
		r.mu.Unlock()
	}()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.healthWake:
		case <-ticker.C:
		}
		r.mu.RLock()
		generation := r.healthContext
		var jobs []*healthState
		for _, state := range r.health {
			if !time.Now().Before(state.next) {
				jobs = append(jobs, state)
			}
		}
		r.mu.RUnlock()
		sort.Slice(jobs, func(i, j int) bool { return healthKey(jobs[i].route) < healthKey(jobs[j].route) })
		slots := make(chan struct{}, 4)
		var workers sync.WaitGroup
		for _, state := range jobs {
			select {
			case <-generation.Done():
				break
			case slots <- struct{}{}:
				workers.Add(1)
				go func(state *healthState) {
					defer workers.Done()
					defer func() { <-slots }()
					r.probe(generation, state, client)
				}(state)
			}
			if generation.Err() != nil {
				break
			}
		}
		workers.Wait()
	}
}

func (r *Router) probe(generation context.Context, state *healthState, client *http.Client) {
	if generation.Err() != nil {
		return
	}
	ctx, cancel := context.WithTimeout(generation, time.Duration(state.route.Health.TimeoutSeconds)*time.Second)
	defer cancel()
	target, err := url.Parse(state.route.Target)
	healthy := false
	if err == nil {
		path, parseErr := url.ParseRequestURI(state.route.Health.Path)
		if parseErr == nil {
			target.Path, target.RawPath, target.RawQuery, target.Fragment = path.Path, path.RawPath, "", ""
			req, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
			if requestErr == nil {
				req.Host = state.route.Hostname
				response, requestErr := client.Do(req)
				if response != nil {
					response.Body.Close()
				}
				healthy = requestErr == nil && response != nil && response.StatusCode >= 200 && response.StatusCode < 300 && ctx.Err() == nil
			}
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if generation.Err() != nil || r.healthContext != generation || r.health[healthKey(state.route)] != state {
		return
	}
	state.healthy = healthy
	state.checkedAt = time.Now()
	state.next = state.checkedAt.Add(time.Duration(state.route.Health.IntervalSeconds) * time.Second)
}

// BackendHealth is the latest admitted probe observation, not a request-error ratio.
type BackendHealth struct {
	Hostname       string `json:"hostname"`
	Pool           string `json:"pool"`
	Path           string `json:"path"`
	TargetOrigin   string `json:"targetOrigin"`
	State          string `json:"state"`
	LastCheckAgeMs *int64 `json:"lastCheckAgeMs"`
	AgeCapped      bool   `json:"ageCapped"`
}

type HealthSnapshot struct {
	Generation uint64          `json:"generation"`
	Running    bool            `json:"running"`
	Backends   []BackendHealth `json:"backends"`
}

// SnapshotHealth reads one current generation atomically and omits target paths,
// query strings, credentials, response bodies and provider errors.
func (r *Router) SnapshotHealth() HealthSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := HealthSnapshot{Generation: r.healthGeneration, Running: r.healthRunning, Backends: []BackendHealth{}}
	for _, state := range r.health {
		u, err := url.Parse(state.route.Target)
		origin := ""
		if err == nil {
			origin = u.Scheme + "://" + u.Host
		}
		row := BackendHealth{Hostname: state.route.Hostname, Pool: state.route.BackendPool, Path: state.route.Path, TargetOrigin: origin, State: "unknown"}
		if !state.checkedAt.IsZero() {
			age := time.Since(state.checkedAt).Milliseconds()
			if age < 0 {
				age = 0
			}
			if age > 24*60*60*1000 {
				age = 24 * 60 * 60 * 1000
				row.AgeCapped = true
			}
			row.LastCheckAgeMs = &age
			if r.healthRunning {
				if state.healthy {
					row.State = "ready"
				} else {
					row.State = "down"
				}
			}
		}
		result.Backends = append(result.Backends, row)
	}
	sort.Slice(result.Backends, func(i, j int) bool {
		a, b := result.Backends[i], result.Backends[j]
		return a.Hostname+a.Pool+a.Path+a.TargetOrigin < b.Hostname+b.Pool+b.Path+b.TargetOrigin
	})
	return result
}

// WriteHealth serves the same snapshot used by the operator topology response.
// It is a handler boundary; it does not bind or start a server.
func (r *Router) WriteHealth(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(r.SnapshotHealth())
}
