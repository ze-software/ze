// VALIDATES: EmitEngineEvent reaches a plugin-process subscriber registered
// through the SubscriptionManager -- the exact leg the .ci engine-step
// executor depends on for expect=event (spec-test-coverage-gaps AC-2/AC-3:
// ike sa-up emitted via the engine bus must land in a subscribed external
// plugin's delivery queue).
// PREVENTS: typed engine emissions silently delivering to zero plugin
// subscribers while engine-side handlers keep working.
package server

import (
	"context"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/events"
)

func TestEmitEngineEventDeliversToProcessSubscriber(t *testing.T) {
	// Fresh namespace/event pair so this test does not depend on any protocol
	// component being linked in (mirrors events.Register in the ike engine).
	evt := events.Register[*struct {
		PeerName string `json:"peer-name"`
	}]("test-deliver-ns", "sa-up")
	_ = evt

	s := &Server{
		subscriptions:     newSubscriptionManager(),
		engineSubscribers: newEngineEventSubscribers(),
	}

	proc := process.NewProcess(plugin.PluginConfig{Name: "deliver-probe"})
	proc.StartDelivery(context.Background())

	s.subscriptions.Add(proc, &Subscription{
		Namespace: events.LookupNamespaceID("test-deliver-ns"),
		EventType: events.LookupEventTypeID("sa-up"),
		Direction: events.DirBoth,
	})

	delivered, err := s.EmitEngineEvent("test-deliver-ns", "sa-up", &struct {
		PeerName string `json:"peer-name"`
	}{PeerName: "peer-1"})
	if err != nil {
		t.Fatalf("EmitEngineEvent: %v", err)
	}
	if delivered != 1 {
		t.Fatalf("EmitEngineEvent delivered = %d, want 1 (subscription must match)", delivered)
	}
}
