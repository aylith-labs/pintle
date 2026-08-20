package api

import (
	"testing"
	"time"

	"github.com/aylith-labs/pintle/internal/provider"
	"github.com/aylith-labs/pintle/internal/router"
	"github.com/aylith-labs/pintle/internal/stats"
)

// The dashboard reported a 9444 + SNI-router topology for weeks after the SNI router
// stopped being started, because both values were constants. These assert the report
// follows what actually ran.
func TestSNITopologyReflectsWhatStarted(t *testing.T) {
	if got := sniTopology(Runtime{SNIEnabled: false}); got != nil {
		t.Errorf("with no SNI router running, sniTopology = %v, want nil", got)
	}

	got, ok := sniTopology(Runtime{SNIEnabled: true, SNIPort: 9443}).(map[string]int)
	if !ok {
		t.Fatalf("with an SNI router running, sniTopology returned %T, want map[string]int", got)
	}
	if got["port"] != 9443 || got["listenPort"] != 443 {
		t.Errorf("sniTopology = %v, want port 9443 listenPort 443", got)
	}
}

func TestExpectedStatusesMarkUnroutedHosts(t *testing.T) {
	rtr := router.New()
	rtr.Update([]provider.Route{{Hostname: "up.lvh.me", Path: "/", Target: "http://127.0.0.1:1", Source: "static"}})

	h := &Handler{
		router: rtr,
		stats:  stats.NewCollector("pintle.lvh.me"),
		rt:     Runtime{StartedAt: time.Now()},
		getExpected: func() []provider.ExpectedHost {
			return []provider.ExpectedHost{
				{Host: "up.lvh.me"},
				{Host: "down.lvh.me", Why: "hooks post here"},
			}
		},
	}

	got := h.expectedStatuses()
	if len(got) != 2 {
		t.Fatalf("expectedStatuses returned %d entries, want 2", len(got))
	}
	if !got[0].Routed {
		t.Errorf("%s has a route but was reported unrouted", got[0].Host)
	}
	if got[1].Routed {
		t.Errorf("%s has no route but was reported routed", got[1].Host)
	}
	if got[1].Why == "" {
		t.Error("the why is what makes a missing host actionable; it was dropped")
	}
}

// A nil declaration list must not be reported as "nothing missing" by way of a panic.
func TestExpectedStatusesWithNoDeclarations(t *testing.T) {
	h := &Handler{router: router.New()}
	if got := h.expectedStatuses(); got != nil {
		t.Errorf("with no expectations declared, got %v, want nil", got)
	}
}
