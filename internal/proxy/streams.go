package proxy

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/aylith-labs/pintle/internal/logger"
)

const (
	// On shutdown, a response still sending its body after this long is treated as a stream.
	streamGrace = 1 * time.Second
	// How often a shutdown re-checks for responses that became streams.
	streamSweepInterval = 250 * time.Millisecond
)

// bodyPhase tracks responses whose headers went out and whose body is still being copied. A
// Server-Sent-Events stream or a long download never finishes by itself, so without this a
// shutdown would hold every listener closed for its whole grace period.
type bodyPhase struct {
	mu      sync.Mutex
	started map[*requestState]time.Time
	waiting map[*requestState]string // requests not yet answered, by host and path
}

func newBodyPhase() *bodyPhase {
	return &bodyPhase{started: make(map[*requestState]time.Time), waiting: make(map[*requestState]string)}
}

func (b *bodyPhase) begin(state *requestState, label string) {
	b.mu.Lock()
	b.waiting[state] = label
	b.mu.Unlock()
}

func (b *bodyPhase) enter(state *requestState) {
	b.mu.Lock()
	delete(b.waiting, state)
	b.started[state] = time.Now()
	b.mu.Unlock()
}

func (b *bodyPhase) leave(state *requestState) {
	b.mu.Lock()
	delete(b.waiting, state)
	delete(b.started, state)
	b.mu.Unlock()
}

// pending names the requests still waiting for an upstream answer, for the shutdown log.
func (b *bodyPhase) pending() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]string, 0, len(b.waiting))
	for _, label := range b.waiting {
		out = append(out, label)
	}
	return out
}

// cutOlderThan cancels every response that has been in its body phase for at least age and
// returns how many it cut.
func (b *bodyPhase) cutOlderThan(age time.Duration) int {
	cutoff := time.Now().Add(-age)
	b.mu.Lock()
	defer b.mu.Unlock()
	cut := 0
	for state, since := range b.started {
		if !since.After(cutoff) {
			state.cancel()
			delete(b.started, state)
			cut++
		}
	}
	return cut
}

// EndStreamsOnDone waits for ctx to end, then keeps closing responses that have been streaming
// for streamGrace, so requests that are still waiting for their headers get the listener's whole
// grace period while streams do not hold the shutdown open. It returns when stop is closed.
func (h *Handler) EndStreamsOnDone(ctx context.Context, stop <-chan struct{}) {
	select {
	case <-ctx.Done():
	case <-stop:
		return
	}
	ticker := time.NewTicker(streamSweepInterval)
	defer ticker.Stop()
	reported := time.Now()
	for {
		if cut := h.streams.cutOlderThan(streamGrace); cut > 0 {
			logger.Infof("Shutdown: closed %d streaming responses", cut)
		}
		if time.Since(reported) >= 2*time.Second {
			reported = time.Now()
			if waiting := h.streams.pending(); len(waiting) > 0 {
				logger.Infof("Shutdown: waiting for %d upstream answers: %s", len(waiting), strings.Join(waiting, ", "))
			}
		}
		select {
		case <-ticker.C:
		case <-stop:
			return
		}
	}
}
