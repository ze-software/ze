// Design: docs/architecture/api/architecture.md -- complete compiled-bus dissemination validation.
package server

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/events"
)

// TestDisseminationValidationCompletesConfiguredInstance proves a narrowed
// source or destination bus filter cannot silently become an inactive transit
// request, and returns a bus-syntax fix command.
//
// VALIDATES: ValidateDisseminationBindings rejects incomplete source,
// incomplete destination, and split-instance configurations with bus fix lines.
// PREVENTS: partial bus filters silently dropping dissemination.
func TestDisseminationValidationCompletesConfiguredInstance(t *testing.T) {
	srcPath := "/" + disseminationTestNamespace + "/entry/source"
	dstPath := "/" + disseminationTestNamespace + "/entry/destination"

	for _, defect := range []string{"none", "source state only", "destination send only", "split instances"} {
		t.Run(defect, func(t *testing.T) {
			table := disseminationEndpointInput("table", plugin.BusBlock{})
			other := disseminationEndpointInput("other-table", plugin.BusBlock{
				Receive: plugin.BusDirectionBlock{TargetsPresent: true, TargetsNone: true},
				Send:    plugin.BusDirectionBlock{TargetsPresent: true, TargetsNone: true},
			})
			switch defect {
			case "none":
			case "source state only":
				table.OwnBlock.Receive.AllowPresent = true
				table.OwnBlock.Receive.Allow = []string{disseminationMsg("state")}
				table.OwnBlock.Send.TargetsPresent = true
				table.OwnBlock.Send.TargetsNone = true
			case "destination send only":
				table.OwnBlock.Receive.AllowPresent = true
				table.OwnBlock.Receive.AllowNone = true
			case "split instances":
				other.OwnBlock = plugin.BusBlock{
					Receive: plugin.BusDirectionBlock{TargetsPresent: true, Targets: []string{srcPath}},
					Send:    plugin.BusDirectionBlock{TargetsPresent: true, TargetsNone: true},
				}
				table.OwnBlock = plugin.BusBlock{
					Receive: plugin.BusDirectionBlock{
						TargetsPresent: true,
						Targets:        []string{dstPath},
						Messages: map[string]*plugin.BusMessageRule{
							disseminationMsg("update"): {Tags: map[string][]string{events.TagDirection: {events.DirectionSent}}},
						},
					},
					Send: plugin.BusDirectionBlock{TargetsPresent: true, Targets: []string{dstPath}},
				}
			}
			graph, err := BuildDeliveryGraph(disseminationTestNamespace, []DeliveryPeer{
				{Addr: "192.0.2.1", Name: "source", Path: srcPath},
				{Addr: "192.0.2.2", Name: "destination", Path: dstPath},
			}, plugin.BusConfig{}, []BusParticipantInput{table, other})
			require.NoError(t, err)
			err = graph.ValidateDisseminationBindings([]DisseminationProcess{
				disseminationTestProcess("table", false), disseminationTestProcess("other-table", false),
			})
			if defect == "none" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, "table")
			require.ErrorContains(t, err, "bus { endpoint table {")
			if defect == "source state only" {
				require.ErrorContains(t, err, "source")
				require.ErrorContains(t, err, "receive { allow [ "+disseminationMsg("update")+" "+disseminationMsg("state")+" ]; }")
				return
			}
			require.ErrorContains(t, err, "destination")
		})
	}
}

// TestDisseminationPeerOptOutIsAllowed proves a participant whose own send > to
// excludes a dissemination endpoint does not trigger a false dissemination
// refusal when the endpoint itself is un-narrowed (S2, S3, S26).
//
// VALIDATES: peer-side send > to exclusion is valid when no feature requires it.
// PREVENTS: S2/S26 configs failing dissemination validation.
func TestDisseminationPeerOptOutIsAllowed(t *testing.T) {
	srcPath := "/" + disseminationTestNamespace + "/entry/edge1"
	labPath := "/" + disseminationTestNamespace + "/entry/lab"
	collector := BusParticipantInput{
		Path:         plugin.EndpointPath("collector"),
		EndpointName: "collector",
		Receives:     disseminationTestReceives(),
	}
	table := disseminationEndpointInput("table", plugin.BusBlock{})
	graph, err := BuildDeliveryGraph(disseminationTestNamespace, []DeliveryPeer{
		{Addr: "192.0.2.1", Name: "edge1", Path: srcPath},
		{
			Addr: "192.0.2.9",
			Name: "lab",
			Path: labPath,
			Bus: plugin.BusBlock{
				Send: plugin.BusDirectionBlock{
					TargetsPresent: true,
					Targets:        []string{plugin.EndpointPath("collector")},
				},
			},
		},
	}, plugin.BusConfig{}, []BusParticipantInput{table, collector})
	require.NoError(t, err)
	require.NoError(t, graph.ValidateDisseminationBindings([]DisseminationProcess{disseminationTestProcess("table", false)}))
	ownerEdge, err := graph.ResolveDisseminationOwner("192.0.2.1", disseminationTestRole, []DisseminationProcess{disseminationTestProcess("table", false)})
	require.NoError(t, err)
	require.Equal(t, "table", ownerEdge.Name())
	ownerLab, err := graph.ResolveDisseminationOwner("192.0.2.9", disseminationTestRole, []DisseminationProcess{disseminationTestProcess("table", false)})
	require.NoError(t, err)
	require.False(t, ownerLab.IsSet())
}

// TestDisseminationExactDestinationPreservesSources proves siblings sharing
// an address cannot supply destination grants or hide another valid source.
//
// VALIDATES: retained disseminationPeers preserve distinct source entries.
// PREVENTS: address-index collapse hiding a valid source or leaking grants.
func TestDisseminationExactDestinationPreservesSources(t *testing.T) {
	processes := []DisseminationProcess{disseminationTestProcess("table", false)}
	source := DeliveryPeer{Addr: "192.0.2.1", Name: "src", Path: "/" + disseminationTestNamespace + "/entry/src"}
	sibling := DeliveryPeer{
		Addr: source.Addr,
		Name: "sib",
		Path: "/" + disseminationTestNamespace + "/entry/sib",
		Bus: plugin.BusBlock{
			Send: plugin.BusDirectionBlock{TargetsPresent: true, TargetsNone: true},
		},
	}
	ep := disseminationEndpointInput("table", plugin.BusBlock{})
	graph, err := BuildDeliveryGraph(disseminationTestNamespace, []DeliveryPeer{source, sibling}, plugin.BusConfig{}, []BusParticipantInput{ep})
	require.NoError(t, err)

	exact := DeliveryPeer{Addr: "destination", Name: "dst", Path: "/" + disseminationTestNamespace + "/entry/dst"}
	inputs := cloneDeliveryPeers(graph.disseminationPeers)
	for i := range inputs {
		inputs[i].Addr = string(rune('a' + i))
	}
	inputs = append(inputs, exact)
	built, err := BuildDeliveryGraph(disseminationTestNamespace, inputs, plugin.BusConfig{}, []BusParticipantInput{ep})
	require.NoError(t, err)
	owner, err := built.ResolveDisseminationOwner("destination", disseminationTestRole, processes)
	require.NoError(t, err)
	require.Equal(t, "table", owner.Name(), "same-address sibling must not hide the complete source")

	exact.Bus = plugin.BusBlock{Send: plugin.BusDirectionBlock{TargetsPresent: true, TargetsNone: true}}
	inputs[len(inputs)-1] = exact
	built, err = BuildDeliveryGraph(disseminationTestNamespace, inputs, plugin.BusConfig{}, []BusParticipantInput{ep})
	require.NoError(t, err)
	owner, err = built.ResolveDisseminationOwner("destination", disseminationTestRole, processes)
	require.NoError(t, err)
	require.False(t, owner.IsSet(), "a sibling's grants cannot complete an opted-out destination")
}
