package process

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestMixedBatchKeepsFIFOOrder proves a batch holding text and structured
// items reaches the plugin in channel order. bgp-gr emits bgp-gr/retention (a
// text item) inside its DOWN handler, and the engine enqueues the DOWN (a
// structured item) to bgp-rr after it; a batch that drained both must deliver
// the verdict first, or bgp-rr decides the DOWN without it.
func TestMixedBatchKeepsFIFOOrder(t *testing.T) {
	proc := NewProcess(plugin.PluginConfig{Name: t.Name()})
	proc.bridge = rpc.NewDirectBridge()
	var seen []string
	proc.bridge.SetDeliverStructured(func(events []any) error {
		for _, event := range events {
			name, ok := event.(string)
			if !ok {
				t.Fatalf("structured item %v is not the test's string", event)
			}
			seen = append(seen, name)
		}
		return nil
	})
	proc.bridge.SetDeliverEvents(func(events []string) error {
		seen = append(seen, events...)
		return nil
	})
	proc.bridge.SetReady()

	batch := []EventDelivery{
		{Output: "text-1"},
		{Event: "structured-2"},
		{Event: "structured-3"},
		{Output: "text-4"},
		{Event: "structured-5"},
	}
	if err := proc.deliverMixedBatch(batch); err != nil {
		t.Fatal(err)
	}
	want := []string{"text-1", "structured-2", "structured-3", "text-4", "structured-5"}
	if !slices.Equal(seen, want) {
		t.Fatalf("delivered %v, want channel order %v", seen, want)
	}
}
