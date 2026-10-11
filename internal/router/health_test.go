package router

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aylith-labs/pintle/internal/provider"
)

type healthTransport func(*http.Request) (*http.Response, error)

func (f healthTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func healthResponse(code int) *http.Response {
	return &http.Response{StatusCode: code, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}
}

func TestHealthRapidReloadKeepsGlobalProbeBound(t *testing.T) {
	r := New()
	routes := func(generation int) []provider.Route {
		out := []provider.Route{}
		for i := 0; i < 6; i++ {
			out = append(out, checkedRoute(fmt.Sprintf("http://generation%d-member%d:8000", generation, i)))
		}
		return out
	}
	r.Update(routes(0))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var active, max, calls atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.RunHealth(ctx, healthTransport(func(req *http.Request) (*http.Response, error) {
			n := active.Add(1)
			calls.Add(1)
			defer active.Add(-1)
			for m := max.Load(); n > m && !max.CompareAndSwap(m, n); m = max.Load() {
			}
			<-req.Context().Done()
			time.Sleep(10 * time.Millisecond)
			return healthResponse(200), nil
		}))
	}()
	waitHealth(t, func() bool { return active.Load() == 4 })
	for i := 1; i <= 30; i++ {
		r.Update(routes(i))
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("rapid-reload lifecycle failed to stop")
	}
	if max.Load() > 4 || active.Load() != 0 || calls.Load() < 4 {
		t.Fatalf("global probe bound/lifetime failed: max%d active%d calls%d", max.Load(), active.Load(), calls.Load())
	}
	for _, backend := range r.SnapshotHealth().Backends {
		if !strings.Contains(backend.TargetOrigin, "generation30-") || backend.State != "unknown" {
			t.Fatal("late old generation published")
		}
	}
}

func TestHealthObservableHTTPCurrentGenerationRecovery(t *testing.T) {
	r := New()
	r.Update([]provider.Route{checkedRoute("http://one:8000/private?secret=never-published")})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var ready atomic.Bool
	done := make(chan struct{})
	fixtures := map[string]HealthSnapshot{}
	read := func() HealthSnapshot {
		rec := httptest.NewRecorder()
		r.WriteHealth(rec)
		if rec.Code != 200 || rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("HTTP snapshot contract failed")
		}
		if strings.Contains(rec.Body.String(), "private") || strings.Contains(rec.Body.String(), "secret") {
			t.Fatal("health diagnostics leaked target path/query")
		}
		var result HealthSnapshot
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	fixtures["unknown"] = read()
	go func() {
		defer close(done)
		r.RunHealth(ctx, healthTransport(func(*http.Request) (*http.Response, error) {
			if ready.Load() {
				return healthResponse(204), nil
			}
			return healthResponse(503), nil
		}))
	}()
	waitHealth(t, func() bool { s := read(); return s.Running && s.Backends[0].State == "down" })
	fixtures["down"] = read()
	ready.Store(true)
	deadline := time.Now().Add(7 * time.Second)
	for read().Backends[0].State != "ready" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	fixtures["ready"] = read()
	if fixtures["ready"].Backends[0].State != "ready" || fixtures["ready"].Backends[0].LastCheckAgeMs == nil {
		t.Fatal("scheduled recovery was not observable")
	}
	r.Update([]provider.Route{checkedRoute("http://replacement:8000")})
	fixtures["reload"] = read()
	if fixtures["reload"].Generation <= fixtures["ready"].Generation || fixtures["reload"].Backends[0].TargetOrigin != "http://replacement:8000" {
		t.Fatal("old generation visible after reload")
	}
	waitHealth(t, func() bool { return read().Backends[0].State == "ready" })
	cancel()
	<-done
	fixtures["stopped"] = read()
	if fixtures["stopped"].Running || fixtures["stopped"].Backends[0].State != "unknown" {
		t.Fatal("stopped owner appears ready")
	}
	r.mu.Lock()
	for _, state := range r.health {
		state.checkedAt = time.Now().Add(-48 * time.Hour)
	}
	r.mu.Unlock()
	bounded := read()
	if !bounded.Backends[0].AgeCapped || *bounded.Backends[0].LastCheckAgeMs != 24*60*60*1000 {
		t.Fatal("unbounded last-check age")
	}
	if path := os.Getenv("PINTLE_HEALTH_FIXTURES"); path != "" {
		b, err := json.Marshal(fixtures)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
func checkedRoute(target string) provider.Route {
	return provider.Route{Hostname: "app.test", Path: "/api", Target: target, BackendPool: "static:0", Source: "static", Health: &provider.HealthCheck{Path: "/ready", IntervalSeconds: 5, TimeoutSeconds: 1}}
}
func waitHealth(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("health lifecycle did not reach expected state")
}

func TestHealthSelectionRecoveryAndReadOnlyContract(t *testing.T) {
	r := New()
	r.Update([]provider.Route{checkedRoute("http://one:8000/base?private=ignored"), checkedRoute("http://two:8000")})
	if got := r.Resolve("app.test", "/api/items"); got == nil || !got.Unavailable {
		t.Fatal("unknown backend admitted")
	}
	var twoReady atomic.Bool
	client := &http.Client{Transport: healthTransport(func(req *http.Request) (*http.Response, error) {
		if req.Method != "GET" || req.Host != "app.test" || req.URL.Path != "/ready" || req.URL.RawQuery != "" || req.Body != nil || req.Header.Get("Authorization") != "" {
			t.Errorf("probe carried user/config extras: %+v", req)
		}
		if req.URL.Host == "two:8000" && !twoReady.Load() {
			return healthResponse(503), nil
		}
		return healthResponse(204), nil
	})}
	probeAll := func() {
		r.mu.RLock()
		ctx := r.healthContext
		states := []*healthState{}
		for _, s := range r.health {
			states = append(states, s)
		}
		r.mu.RUnlock()
		for _, s := range states {
			r.probe(ctx, s, client)
		}
	}
	probeAll()
	for i := 0; i < 5; i++ {
		if got := r.Resolve("app.test", "/api"); got.Unavailable || got.Target != "http://one:8000/base?private=ignored" {
			t.Fatalf("down member selected: %+v", got)
		}
	}
	twoReady.Store(true)
	probeAll()
	seen := map[string]bool{}
	for i := 0; i < 2; i++ {
		got := r.Resolve("app.test", "/api")
		seen[got.Target] = true
		if got.Route.Target != got.Target {
			t.Fatal("selected provenance differs")
		}
	}
	if len(seen) != 2 {
		t.Fatal("recovered member not restored")
	}
}

func TestHealthReloadCancelsProbeAndFencesLateSuccess(t *testing.T) {
	r := New()
	r.Update([]provider.Route{checkedRoute("http://old:8000")})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	canceled := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		r.RunHealth(ctx, healthTransport(func(req *http.Request) (*http.Response, error) {
			if req.URL.Host == "old:8000" {
				close(started)
				<-req.Context().Done()
				close(canceled)
				<-release
				return healthResponse(200), nil
			}
			return healthResponse(200), nil
		}))
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("old probe not started")
	}
	r.Update([]provider.Route{checkedRoute("http://new:8000")})
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("reload did not cancel probe")
	}
	if got := r.Resolve("app.test", "/api"); !got.Unavailable {
		t.Fatal("new generation inherited old success")
	}
	close(release)
	waitHealth(t, func() bool {
		got := r.Resolve("app.test", "/api")
		return got != nil && !got.Unavailable && got.Target == "http://new:8000"
	})
	cancel()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("health owner did not stop")
	}
	if got := r.Resolve("app.test", "/api"); !got.Unavailable {
		t.Fatal("stopped lifecycle still reports healthy")
	}
}

func TestHealthProbeTimeoutAndWorkerBound(t *testing.T) {
	r := New()
	routes := []provider.Route{}
	for _, host := range []string{"a", "b", "c", "d", "e", "f"} {
		routes = append(routes, checkedRoute("http://"+host+":8000"))
	}
	r.Update(routes)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var active, max, attempts atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.RunHealth(ctx, healthTransport(func(req *http.Request) (*http.Response, error) {
			n := active.Add(1)
			defer active.Add(-1)
			for m := max.Load(); n > m && !max.CompareAndSwap(m, n); m = max.Load() {
			}
			attempts.Add(1)
			<-req.Context().Done()
			return nil, req.Context().Err()
		}))
	}()
	waitHealth(t, func() bool { return attempts.Load() >= 6 })
	if max.Load() > 4 {
		t.Fatalf("unbounded probes: %d", max.Load())
	}
	if got := r.Resolve("app.test", "/api"); !got.Unavailable {
		t.Fatal("timeout admitted member")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown didn't cancel remaining probes")
	}
	if active.Load() != 0 {
		t.Fatal("probe worker leaked")
	}
}

func TestHealthRedirectIsNotFollowed(t *testing.T) {
	r := New()
	r.Update([]provider.Route{checkedRoute("http://one:8000")})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.RunHealth(ctx, healthTransport(func(*http.Request) (*http.Response, error) {
			calls.Add(1)
			resp := healthResponse(302)
			resp.Header.Set("Location", "http://foreign.test/private")
			return resp, nil
		}))
	}()
	waitHealth(t, func() bool { return calls.Load() > 0 })
	cancel()
	<-done
	if calls.Load() != 1 {
		t.Fatal("probe followed redirect")
	}
	if got := r.Resolve("app.test", "/api"); !got.Unavailable {
		t.Fatal("redirect marked healthy")
	}
}

func TestHealthConfiguredDestinationsOnly(t *testing.T) {
	for _, target := range []string{"http://192.168.1.50:3000/base?secret=discard", "http://host.docker.internal:3000/base", "http://host.wsl.internal:3000/base", "https://service.example:8443/base"} {
		r := New()
		r.Update([]provider.Route{checkedRoute(target)})
		calls := 0
		client := &http.Client{Transport: healthTransport(func(req *http.Request) (*http.Response, error) {
			calls++
			origin := strings.Split(strings.Split(target, "?")[0], "/base")[0]
			if req.URL.Scheme+"://"+req.URL.Host != origin || req.URL.Path != "/ready" || req.URL.RawQuery != "" || req.Method != "GET" || req.Host != "app.test" || req.Body != nil || req.Header.Get("Authorization") != "" {
				t.Fatalf("probe destination/authority changed: %+v", req)
			}
			return healthResponse(204), nil
		})}
		r.mu.RLock()
		ctx := r.healthContext
		var state *healthState
		for _, s := range r.health {
			state = s
		}
		r.mu.RUnlock()
		r.probe(ctx, state, client)
		if calls != 1 || r.Resolve("app.test", "/api/item").Unavailable {
			t.Fatal("configured probe failed or made additional requests")
		}
	}
}
