package aggregator

import (
	"sync"

	"github.com/steven-pribilinskiy/local-proxy/internal/provider"
)

type Configuration struct {
	Routes      []provider.Route
	TcpRoutes   []provider.TcpRoute
	Passthrough []provider.PassthroughDomain
}

type Aggregator struct {
	mu       sync.Mutex
	configs  map[string]provider.Message
	outputCh chan Configuration
}

func New() (*Aggregator, <-chan Configuration) {
	ch := make(chan Configuration, 1)
	return &Aggregator{
		configs:  make(map[string]provider.Message),
		outputCh: ch,
	}, ch
}

func (a *Aggregator) Update(msg provider.Message) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.configs[msg.ProviderName] = msg
	merged := a.merge()

	// Non-blocking send (Traefik pattern): always deliver latest config
	select {
	case a.outputCh <- merged:
	default:
		// Drain old value
		select {
		case <-a.outputCh:
		default:
		}
		a.outputCh <- merged
	}
}

func (a *Aggregator) merge() Configuration {
	var cfg Configuration

	// Collect all routes and passthrough from all providers
	for _, msg := range a.configs {
		cfg.Routes = append(cfg.Routes, msg.Routes...)
		cfg.TcpRoutes = append(cfg.TcpRoutes, msg.TcpRoutes...)
		if len(msg.Passthrough) > 0 {
			cfg.Passthrough = msg.Passthrough
		}
	}

	return cfg
}

// GetCurrentPassthrough returns the latest passthrough config without waiting for updates.
func (a *Aggregator) GetCurrentPassthrough() []provider.PassthroughDomain {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, msg := range a.configs {
		if len(msg.Passthrough) > 0 {
			return msg.Passthrough
		}
	}
	return nil
}
