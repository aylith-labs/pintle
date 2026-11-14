package stats

import (
	"testing"
	"time"
)

func TestRecordAndRetrieve(t *testing.T) {
	c := NewCollector("pintle.lvh.me")

	c.Record(RequestRecord{
		Timestamp:  time.Now().UnixMilli(),
		Method:     "GET",
		Hostname:   "app.lvh.me",
		Path:       "/",
		Target:     "http://localhost:3000",
		Status:     200,
		DurationMs: 15.5,
	})

	requests := c.GetRecentRequests(50)
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
	if requests[0].Status != 200 {
		t.Errorf("status = %d, want 200", requests[0].Status)
	}
}

func TestDashboardHostFiltered(t *testing.T) {
	c := NewCollector("pintle.lvh.me")

	c.Record(RequestRecord{
		Timestamp: time.Now().UnixMilli(),
		Method:    "GET",
		Hostname:  "pintle.lvh.me",
		Path:      "/api/health",
		Target:    "internal",
		Status:    200,
	})

	requests := c.GetRecentRequests(50)
	if len(requests) != 0 {
		t.Errorf("expected 0 requests (dashboard filtered), got %d", len(requests))
	}
}

func TestHostStats(t *testing.T) {
	c := NewCollector("pintle.lvh.me")

	for i := 0; i < 5; i++ {
		c.Record(RequestRecord{
			Timestamp:  time.Now().UnixMilli(),
			Method:     "GET",
			Hostname:   "app.lvh.me",
			Path:       "/",
			Target:     "http://localhost:3000",
			Status:     200,
			DurationMs: 10,
		})
	}

	c.Record(RequestRecord{
		Timestamp:  time.Now().UnixMilli(),
		Method:     "GET",
		Hostname:   "app.lvh.me",
		Path:       "/",
		Target:     "http://localhost:3000",
		Status:     500,
		DurationMs: 10,
	})

	stats := c.GetHostStats()
	hs := stats["app.lvh.me"]
	if hs == nil {
		t.Fatal("expected host stats for app.lvh.me")
	}
	if hs.TotalRequests != 6 {
		t.Errorf("totalRequests = %d, want 6", hs.TotalRequests)
	}
	if hs.ErrorCount != 1 {
		t.Errorf("errorCount = %d, want 1", hs.ErrorCount)
	}
}

func TestCircularBuffer(t *testing.T) {
	c := NewCollector("pintle.lvh.me")

	for i := 0; i < 1100; i++ {
		c.Record(RequestRecord{
			Timestamp: int64(i),
			Method:    "GET",
			Hostname:  "app.lvh.me",
			Path:      "/",
			Target:    "http://localhost:3000",
			Status:    200,
		})
	}

	requests := c.GetRecentRequests(2000)
	if len(requests) != 1000 {
		t.Errorf("buffer size = %d, want 1000 (max)", len(requests))
	}

	// Most recent should be first (reversed)
	if requests[0].Timestamp != 1099 {
		t.Errorf("most recent timestamp = %d, want 1099", requests[0].Timestamp)
	}
}

func TestTotalRequests(t *testing.T) {
	c := NewCollector("pintle.lvh.me")

	c.Record(RequestRecord{Timestamp: 1, Method: "GET", Hostname: "a.lvh.me", Status: 200})
	c.Record(RequestRecord{Timestamp: 2, Method: "GET", Hostname: "b.lvh.me", Status: 200})
	c.Record(RequestRecord{Timestamp: 3, Method: "GET", Hostname: "a.lvh.me", Status: 200})

	total := c.GetTotalRequests()
	if total != 3 {
		t.Errorf("totalRequests = %d, want 3", total)
	}
}

func TestErrorCountSkipsClientCancelAndCountsAborts(t *testing.T) {
	c := NewCollector("pintle.lvh.me")
	for _, record := range []RequestRecord{
		{Hostname: "app.lvh.me", Target: "http://localhost:3000", Status: 499},
		{Hostname: "app.lvh.me", Target: "http://localhost:3000", Status: 200, Aborted: true},
		{Hostname: "app.lvh.me", Target: "http://localhost:3000", Status: 503},
		{Hostname: "app.lvh.me", Target: "http://localhost:3000", Status: 200},
	} {
		c.Record(record)
	}
	if got := c.GetHostStats()["app.lvh.me"]; got.TotalRequests != 4 || got.ErrorCount != 2 {
		t.Fatalf("total %d errors %d; want 4 total, 2 errors (abort and 503, not the 499)", got.TotalRequests, got.ErrorCount)
	}
}

func TestUnroutedHostsDoNotGrowMaps(t *testing.T) {
	c := NewCollector("pintle.lvh.me")
	for i := 0; i < 500; i++ {
		c.Record(RequestRecord{Hostname: "random-" + string(rune('a'+i%26)) + time.Now().String(), Target: "none", Status: 404, Unrouted: true})
	}
	if hosts, edges := c.GetHostStats(), c.GetEdgeStats(); len(hosts) != 1 || len(edges) != 1 || hosts[UnroutedHost].TotalRequests != 500 {
		t.Fatalf("hosts %d edges %d; want one key each holding all 500", len(hosts), len(edges))
	}
}
