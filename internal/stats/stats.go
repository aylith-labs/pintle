package stats

import (
	"sync"
	"time"
)

type RequestRecord struct {
	Timestamp  int64  `json:"timestamp"`
	Method     string `json:"method"`
	Hostname   string `json:"hostname"`
	Path       string `json:"path"`
	Target     string `json:"target"`
	Status     int    `json:"status"`
	DurationMs float64 `json:"durationMs"`
}

type RouteStats struct {
	TotalRequests int     `json:"totalRequests"`
	ErrorCount    int     `json:"errorCount"`
	AvgDurationMs float64 `json:"avgDurationMs"`
	LastRequestAt int64   `json:"lastRequestAt"`
}

const maxBufferSize = 1000

type Collector struct {
	mu            sync.Mutex
	buffer        []RequestRecord
	hostStats     map[string]*RouteStats
	edgeStats     map[string]*RouteStats
	startedAt     time.Time
	dashboardHost string
}

func NewCollector(dashboardHost string) *Collector {
	return &Collector{
		buffer:        make([]RequestRecord, 0, maxBufferSize),
		hostStats:     make(map[string]*RouteStats),
		edgeStats:     make(map[string]*RouteStats),
		startedAt:     time.Now(),
		dashboardHost: dashboardHost,
	}
}

func updateStats(stats *RouteStats, record RequestRecord) {
	total := stats.TotalRequests + 1
	stats.AvgDurationMs = (stats.AvgDurationMs*float64(stats.TotalRequests) + record.DurationMs) / float64(total)
	stats.TotalRequests = total
	if record.Status >= 400 {
		stats.ErrorCount++
	}
	stats.LastRequestAt = record.Timestamp
}

func (c *Collector) Record(record RequestRecord) {
	if record.Hostname == c.dashboardHost {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.buffer = append(c.buffer, record)
	if len(c.buffer) > maxBufferSize {
		c.buffer = c.buffer[1:]
	}

	// Host stats
	hs, ok := c.hostStats[record.Hostname]
	if !ok {
		hs = &RouteStats{}
		c.hostStats[record.Hostname] = hs
	}
	updateStats(hs, record)

	// Edge stats
	edgeKey := record.Hostname + "->" + record.Target
	es, ok := c.edgeStats[edgeKey]
	if !ok {
		es = &RouteStats{}
		c.edgeStats[edgeKey] = es
	}
	updateStats(es, record)
}

func (c *Collector) GetRecentRequests(limit int) []RequestRecord {
	c.mu.Lock()
	defer c.mu.Unlock()

	if limit <= 0 {
		limit = 50
	}

	start := len(c.buffer) - limit
	if start < 0 {
		start = 0
	}

	slice := c.buffer[start:]
	// Reverse order (newest first)
	result := make([]RequestRecord, len(slice))
	for i, r := range slice {
		result[len(slice)-1-i] = r
	}
	return result
}

func (c *Collector) GetHostStats() map[string]*RouteStats {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := make(map[string]*RouteStats, len(c.hostStats))
	for k, v := range c.hostStats {
		cp := *v
		result[k] = &cp
	}
	return result
}

func (c *Collector) GetEdgeStats() map[string]*RouteStats {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := make(map[string]*RouteStats, len(c.edgeStats))
	for k, v := range c.edgeStats {
		cp := *v
		result[k] = &cp
	}
	return result
}

func (c *Collector) GetUptime() int {
	return int(time.Since(c.startedAt).Seconds())
}

func (c *Collector) GetTotalRequests() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	total := 0
	for _, s := range c.hostStats {
		total += s.TotalRequests
	}
	return total
}
