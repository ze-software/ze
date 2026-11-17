// Design: docs/architecture/api/architecture.md -- complete explicit transit grants.
package server

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/events"
)

// TestDisseminationValidationCompletesConfiguredInstance proves a partial
// source or destination cannot silently become an inactive transit request.
// Method: retain a complete opposite side, remove one grant, and inspect help.
func TestDisseminationValidationCompletesConfiguredInstance(t *testing.T) {
	for _, defect := range []string{"none", "source state only", "destination send only", "split instances"} {
		t.Run(defect, func(t *testing.T) {
			source := disseminationTestSource("table")
			destination := disseminationTestDestination("table")
			switch defect {
			case "none":
			case "source state only":
				source.Receive = map[string]events.Direction{"state": events.DirBoth}
			case "destination send only":
				destination.Receive = nil
			case "split instances":
				source.PluginName = "other-table"
			}
			graph := newDeliveryGraph(disseminationTestNamespace, []DeliveryPeer{
				{Addr: "192.0.2.1", Name: "source", Bindings: []plugin.PeerProcessBinding{source}},
				{Addr: "192.0.2.2", Name: "destination", Bindings: []plugin.PeerProcessBinding{destination}},
			})
			err := graph.ValidateDisseminationBindings([]DisseminationProcess{
				disseminationTestProcess("table", false), disseminationTestProcess("other-table", false),
			})
			if defect == "none" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, "table")
			require.ErrorContains(t, err, "attach process table {")
			if defect == "source state only" {
				require.ErrorContains(t, err, "source")
				require.ErrorContains(t, err, "update-received state")
				return
			}
			require.ErrorContains(t, err, "destination")
		})
	}
}

// TestDisseminationValidationDoesNotRequestDerivedTransit proves a legacy
// derived feed alone remains legal. Method: use incomplete non-explicit grants.
func TestDisseminationValidationDoesNotRequestDerivedTransit(t *testing.T) {
	binding := disseminationTestSource("table")
	binding.Explicit = false
	binding.Receive = map[string]events.Direction{"state": events.DirBoth}
	graph := newDeliveryGraph(disseminationTestNamespace, []DeliveryPeer{
		{Addr: "192.0.2.1", Bindings: []plugin.PeerProcessBinding{binding}},
	})
	require.NoError(t, graph.ValidateDisseminationBindings([]DisseminationProcess{disseminationTestProcess("table", false)}))
}

// TestDisseminationExactDestinationPreservesSources proves siblings sharing
// an address cannot supply destination grants or hide another valid source.
// Method: collapse the ordinary address index, then resolve an exact block.
func TestDisseminationExactDestinationPreservesSources(t *testing.T) {
	processes := []DisseminationProcess{disseminationTestProcess("table", false)}
	source := DeliveryPeer{Addr: "192.0.2.1", Bindings: []plugin.PeerProcessBinding{disseminationTestSource("table")}}
	sibling := DeliveryPeer{Addr: source.Addr, Bindings: []plugin.PeerProcessBinding{disseminationTestDestination("table")}}
	graph := newDeliveryGraph(disseminationTestNamespace, []DeliveryPeer{source, sibling})
	// Inputs are caller-owned. Later mutation must not alter the graph's
	// retained exact-source evidence.
	delete(source.Bindings[0].Receive, "update")
	server := &Server{}
	server.deliveryGraph.Store(graph)
	exact := DeliveryPeer{Addr: "destination", Bindings: []plugin.PeerProcessBinding{disseminationTestDestination("table")}}
	inputs := graph.disseminationPeers
	for i := range inputs {
		inputs[i].Addr = string(rune('a' + i))
	}
	inputs = append(inputs, exact)
	owner, err := newDeliveryGraph(disseminationTestNamespace, inputs).ResolveDisseminationOwner("destination", disseminationTestRole, processes)
	require.NoError(t, err)
	require.Equal(t, "table", owner.Name(), "same-address destination must not hide the complete source")
	exact.Bindings[0].Receive = nil
	inputs[len(inputs)-1] = exact
	owner, err = newDeliveryGraph(disseminationTestNamespace, inputs).ResolveDisseminationOwner("destination", disseminationTestRole, processes)
	require.NoError(t, err)
	require.False(t, owner.IsSet(), "a sibling's grants cannot complete this destination")
}
