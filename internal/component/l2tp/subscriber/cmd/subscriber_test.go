// VALIDATES: AC-6, AC-7, AC-8 (wiring: handlers call show.Enrich/EnrichBrief)
// PREVENTS: enrichment regression if handler is refactored

package cmd

import (
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/show"
)

func TestSubscriberDetailCallsEnrich(t *testing.T) {
	t.Cleanup(show.ResetForTest)

	subscriber.DefaultRegistry.Add(&subscriber.Session{
		ID:         "test-1",
		AccessType: subscriber.AccessPPPoE,
		State:      subscriber.StateActive,
	})
	t.Cleanup(func() { subscriber.DefaultRegistry.Remove("test-1") })

	enriched := false
	show.MustRegister("show subscriber detail", "test", show.Enricher{
		Detail: func(base map[string]any) {
			enriched = true
			base["test-enriched"] = true
		},
	})

	ctx := &pluginserver.CommandContext{
		Selectors: map[string]string{"id": "test-1"},
	}
	resp, err := handleDetail(ctx, nil)
	if err != nil {
		t.Fatalf("handleDetail error: %v", err)
	}
	if resp == nil {
		t.Fatal("handleDetail returned nil response")
	}
	if !enriched {
		t.Fatal("show.Enrich was not called by handleDetail")
	}
}

func TestSubscriberSummaryCallsEnrichBrief(t *testing.T) {
	t.Cleanup(show.ResetForTest)

	subscriber.DefaultRegistry.Add(&subscriber.Session{
		ID:         "test-2",
		AccessType: subscriber.AccessL2TP,
		State:      subscriber.StateActive,
	})
	t.Cleanup(func() { subscriber.DefaultRegistry.Remove("test-2") })

	briefCalled := 0
	show.MustRegister("show subscriber", "test", show.Enricher{
		Brief: func(base map[string]any) {
			briefCalled++
			base["test-brief"] = true
		},
	})

	resp, err := handleSummary(nil, nil)
	if err != nil {
		t.Fatalf("handleSummary error: %v", err)
	}
	if resp == nil {
		t.Fatal("handleSummary returned nil response")
	}
	if briefCalled != 1 {
		t.Fatalf("expected EnrichBrief called 1 time, got %d", briefCalled)
	}
}
