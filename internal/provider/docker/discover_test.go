package docker

import (
	"context"
	"errors"
	"testing"

	"github.com/aylith-labs/pintle/internal/aggregator"
	"github.com/aylith-labs/pintle/internal/config"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

type fakeLister struct {
	containers []container.Summary
	err        error
}

func (f *fakeLister) ContainerList(context.Context, container.ListOptions) ([]container.Summary, error) {
	return f.containers, f.err
}

func appContainer() container.Summary {
	return container.Summary{
		Names:  []string{"/app"},
		Labels: map[string]string{"pintle.host": "app.lvh.me", "pintle.port": "5173"},
		NetworkSettings: &container.NetworkSettingsSummary{
			Networks: map[string]*network.EndpointSettings{"dev": {IPAddress: "172.18.0.5"}},
		},
	}
}

func TestFailedListKeepsPreviousRoutes(t *testing.T) {
	dp := New("dev")
	lister := &fakeLister{containers: []container.Summary{appContainer()}}
	cfg := &config.Config{}
	agg, out := aggregator.New()

	first := dp.discoverMessage(context.Background(), lister)
	if err := cfg.ValidateMessage(first); err != nil {
		t.Fatalf("good discovery rejected: %v", err)
	}
	agg.Update(first)
	if got := (<-out).Routes; len(got) != 1 {
		t.Fatalf("initial routes = %d, want 1", len(got))
	}

	lister.err = errors.New("docker socket unavailable")
	failed := dp.discoverMessage(context.Background(), lister)
	if failed.Err == nil {
		t.Fatal("failed container list produced no error")
	}
	// The consumer gates on ValidateMessage before the aggregator, as main does.
	if err := cfg.ValidateMessage(failed); err == nil {
		t.Fatal("failed discovery was accepted")
	}
	if got := agg.GetCurrentPassthrough(); got != nil {
		t.Fatalf("unexpected passthrough %v", got)
	}
	if got := dp.GetDockerRoutes(); len(got) != 1 {
		t.Fatalf("cached docker routes = %d after failed list, want 1", len(got))
	}

	// A later successful pass with no containers is a real removal, not an error.
	lister.err, lister.containers = nil, nil
	empty := dp.discoverMessage(context.Background(), lister)
	if empty.Err != nil || len(empty.Routes) != 0 {
		t.Fatalf("empty list: err=%v routes=%d", empty.Err, len(empty.Routes))
	}
	if got := dp.GetDockerRoutes(); len(got) != 0 {
		t.Fatalf("cached routes = %d after empty list, want 0", len(got))
	}
}
