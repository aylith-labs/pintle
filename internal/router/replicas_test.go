package router

import (
	"github.com/aylith-labs/pintle/internal/provider"
	"sync"
	"testing"
)

func TestExplicitReplicaPoolPreservesRoutingAndSelection(t *testing.T) {
	r := New()
	r.Update([]provider.Route{
		{Hostname: "app.test", Path: "/api", Target: "http://one:8000", StripPath: true, BackendPool: "static:0", Source: "static"},
		{Hostname: "app.test", Path: "/api", Target: "http://two:8000", StripPath: true, BackendPool: "static:0", Source: "static"},
		{Hostname: "app.test", Path: "/api/admin", Target: "http://admin:8000", Source: "static"},
	})
	for i := 0; i < 6; i++ {
		got := r.Resolve("app.test", "/api/items")
		expected := []string{"http://one:8000", "http://two:8000"}[i%2]
		if got == nil || got.Target != expected || got.Route.Target != expected || got.RewrittenPath != "/items" {
			t.Fatalf("bad selected provenance: %+v", got)
		}
	}
	if got := r.Resolve("app.test", "/api/admin/log"); got.Target != "http://admin:8000" {
		t.Fatalf("specific route lost: %+v", got)
	}
	if r.Resolve("foreign.test", "/api") != nil {
		t.Fatal("foreign host routed")
	}
	r.Update([]provider.Route{{Hostname: "app.test", Path: "/api", Target: "http://three:8000", BackendPool: "static:0"}})
	if got := r.Resolve("app.test", "/api/items"); got.Target != "http://three:8000" {
		t.Fatal("removed backend reused")
	}
}
func TestReplicaConcurrentSelection(t *testing.T) {
	r := New()
	r.Update([]provider.Route{{Hostname: "app.test", Path: "/", Target: "http://one", BackendPool: "pool"}, {Hostname: "app.test", Path: "/", Target: "http://two", BackendPool: "pool"}})
	counts := map[string]int{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); v := r.Resolve("app.test", "/"); mu.Lock(); counts[v.Target]++; mu.Unlock() }()
	}
	wg.Wait()
	if counts["http://one"] != 50 || counts["http://two"] != 50 {
		t.Fatalf("uneven selection: %v", counts)
	}
}

func TestReplicaPoolsHaveIndependentIdentityAcrossReload(t *testing.T) {
	r := New()
	routes := []provider.Route{
		{Hostname: "a.test", Path: "/", Target: "http://a-one", BackendPool: "shared-label"},
		{Hostname: "a.test", Path: "/", Target: "http://a-two", BackendPool: "shared-label"},
		{Hostname: "b.test", Path: "/", Target: "http://b-one", BackendPool: "shared-label"},
		{Hostname: "b.test", Path: "/", Target: "http://b-two", BackendPool: "shared-label"},
	}
	r.Update(routes)
	for _, pair := range [][2]string{{"a.test", "http://a-one"}, {"a.test", "http://a-two"}, {"a.test", "http://a-one"}, {"b.test", "http://b-one"}, {"b.test", "http://b-two"}} {
		if got := r.Resolve(pair[0], "/"); got.Target != pair[1] {
			t.Fatalf("pool identities coupled: %+v expected %s", got, pair[1])
		}
	}
	r.Update([]provider.Route{{Hostname: "a.test", Path: "/", Target: "http://replacement", BackendPool: "shared-label"}})
	if r.Resolve("b.test", "/") != nil {
		t.Fatal("removed route identity remains")
	}
	for i := 0; i < 4; i++ {
		if got := r.Resolve("a.test", "/"); got.Target != "http://replacement" {
			t.Fatalf("old selection reused after reload: %+v", got)
		}
	}
}

func TestReplicaSelectionDuringConcurrentReload(t *testing.T) {
	r := New()
	members := []provider.Route{{Hostname: "app.test", Path: "/", Target: "http://one", BackendPool: "pool"}, {Hostname: "app.test", Path: "/", Target: "http://two", BackendPool: "pool"}}
	r.Update(members)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 100; n++ {
				got := r.Resolve("app.test", "/")
				if got == nil || got.Route.Target != got.Target || (got.Target != "http://one" && got.Target != "http://two" && got.Target != "http://replacement") {
					t.Errorf("inconsistent concurrent selection: %+v", got)
				}
			}
		}()
	}
	for i := 0; i < 100; i++ {
		r.Update([]provider.Route{{Hostname: "app.test", Path: "/", Target: "http://replacement", BackendPool: "pool"}})
		r.Update(members)
	}
	wg.Wait()
	r.Update([]provider.Route{{Hostname: "app.test", Path: "/", Target: "http://replacement", BackendPool: "pool"}})
	if got := r.Resolve("app.test", "/"); got.Target != "http://replacement" {
		t.Fatal("completed reload did not fence old members")
	}
}
