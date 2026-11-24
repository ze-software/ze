// Design: docs/architecture/api/architecture.md -- received-only UPDATE guard for BGP plugins.
//
// The guard verifies the four BGP plugins that must never receive the UPDATEs
// ze sends. Everything it judges is compiled out of a build without ze_bgp:
// the four plugins, their YANG modules, and internal/component/plugin/all
// (all_ze_bgp.go carries this same tag).
//
//go:build ze_bgp

// The guard lives in an external test package so it can import the composition
// root without introducing an import cycle.
package reactor_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"

	// Registers every built-in plugin and YANG module so registry.Lookup and
	// config.ParseTreeForValidation see the real daemon registrations.
	_ "github.com/ze-software/ze/internal/component/plugin/all"
)

// noSentUpdates names the in-tree plugins that must never be fed the UPDATEs ze
// sends, with the reason each one states in its own source.
var noSentUpdates = map[string]string{
	"bgp-rr": "forwarding an UPDATE raises the sent event the forward is waiting on " +
		"(rr.go: ForwardUpdate -> onMessageSent -> deliver -> block)",
	"bgp-rs": "same circular deadlock as bgp-rr (server.go, rsBusReceives)",
	"bgp-rpki": "origin validation judges what the peer announced; ze's own announcements " +
		"carry no origin to validate (rpki.go declares update direction received)",
	"bgp-rpki-decorator": "it decorates a received UPDATE with its validation state " +
		"(decorator.go declares update direction received)",
}

const (
	bgpUpdateMsgName = bgpevents.Namespace + ":" + bgpevents.EventUpdate
	testPeerPath     = "/bgp/peer/edge1"
)

// TestNoConfigFeedsSentUpdatesToAReceivedOnlyPlugin proves R-15 under the
// message bus: bgp-rr, bgp-rs, bgp-rpki, and bgp-rpki-decorator register
// bgp:update with direction=[received] only, default bus delivery feeds them
// received UPDATEs and never sent UPDATEs, and no operator bus config (on the
// endpoint, in a central participant block, or on a peer) can widen delivery
// to include sent UPDATEs.
//
// VALIDATES: R-15. Direction is enforced by each plugin's registered Receives
// and by the bus compiler's tag-narrowing intersection.
// PREVENTS: a registration regression dropping the direction=[received] tag on
// bgp:update, or a bus compiler regression allowing operator config to widen a
// received-only plugin into receiving sent UPDATEs.
func TestNoConfigFeedsSentUpdatesToAReceivedOnlyPlugin(t *testing.T) {
	nsID := events.LookupNamespaceID(bgpevents.Namespace)
	updateID := events.LookupEventTypeID(bgpevents.EventUpdate)
	require.NotZero(t, nsID, "bgp event namespace must be registered")
	require.NotZero(t, updateID, "bgp:update event type must be registered")

	pluginNames := make([]string, 0, len(noSentUpdates))
	for name := range noSentUpdates {
		pluginNames = append(pluginNames, name)
	}
	slices.Sort(pluginNames)

	for _, name := range pluginNames {
		reason := noSentUpdates[name]
		t.Run(name, func(t *testing.T) {
			reg := registry.Lookup(name)
			require.NotNil(t, reg, "%s must be registered in the ze_bgp build", name)
			requireReceivedOnlyUpdateDecl(t, name, reason, reg.Receives)

			// Default wiring (no bus overrides): received UPDATEs arrive, sent UPDATEs do not.
			defaultBus := compilePeerAndEndpointBus(t, name, reg.Receives, plugin.BusBlock{}, plugin.BusBlock{}, plugin.BusBlock{})
			require.Contains(t, defaultBus.Receivers(testPeerPath, nsID, updateID, events.DirReceived, nil, nil), name,
				"%s must receive received UPDATEs by default", name)
			require.NotContains(t, defaultBus.Receivers(testPeerPath, nsID, updateID, events.DirSent, nil, nil), name,
				"%s must never receive sent UPDATEs (%s)", name, reason)

			// Operator config explicitly requesting direction=[sent] on the endpoint is refused.
			sentOnlyBlock := parseEndpointBusBlock(t, name, `
bus {
    endpoint alias1 {
        internal `+name+`;
        receive {
            message bgp:update {
                tag direction { value [ sent ]; }
            }
        }
    }
}
`, "alias1")
			_, err := pluginserver.CompileBus(nil, buildPeerAndEndpointParticipants("alias1", reg.Receives, plugin.BusBlock{}, sentOnlyBlock, plugin.BusBlock{}))
			require.Error(t, err, "%s: endpoint receive direction=[sent] must be refused", name)
			require.Contains(t, err.Error(), "shares no value with registered filter")

			// Operator config requesting direction=[received sent] narrows to [received] only.
			bothBlock := parseEndpointBusBlock(t, name, `
bus {
    endpoint alias1 {
        internal `+name+`;
        receive {
            message bgp:update {
                tag direction { value [ received sent ]; }
            }
        }
    }
}
`, "alias1")
			narrowedBus := compilePeerAndEndpointBus(t, "alias1", reg.Receives, plugin.BusBlock{}, bothBlock, plugin.BusBlock{})
			require.Contains(t, narrowedBus.Receivers(testPeerPath, nsID, updateID, events.DirReceived, nil, nil), "alias1")
			require.NotContains(t, narrowedBus.Receivers(testPeerPath, nsID, updateID, events.DirSent, nil, nil), "alias1",
				"%s must never receive sent UPDATEs even when operator writes [received sent] (%s)", name, reason)

			// Central bus > participant override attempting direction=[sent] is also refused.
			centralSentBlock := parseParticipantBusBlock(t, `
bus {
    endpoint alias1 {
        internal `+name+`;
    }
    participant /bus/endpoint/alias1 {
        receive {
            message bgp:update {
                tag direction { value [ sent ]; }
            }
        }
    }
}
`, "/bus/endpoint/alias1")
			_, err = pluginserver.CompileBus(nil, buildPeerAndEndpointParticipants("alias1", reg.Receives, plugin.BusBlock{}, plugin.BusBlock{}, centralSentBlock))
			require.Error(t, err, "%s: central participant receive direction=[sent] must be refused", name)
			require.Contains(t, err.Error(), "shares no value with registered filter")

			// A peer sending bgp:update with direction=[sent] targeted at the endpoint
			// still never delivers sent UPDATEs to the received-only plugin.
			peerSentBlock := parsePeerBusBlock(t, `
bgp {
    router-id 192.0.2.254;
    session { asn { local 65000; } }
    peer edge1 {
        connection {
            remote { ip 192.0.2.1; }
            local { ip 192.0.2.2; }
        }
        session { asn { remote 65001; } }
        bus {
            send {
                message bgp:update {
                    to [ /bus/endpoint/alias1 ];
                    tag direction { value [ sent ]; }
                }
            }
        }
    }
}
`, "edge1")
			peerSentBus := compilePeerAndEndpointBus(t, "alias1", reg.Receives, peerSentBlock, plugin.BusBlock{}, plugin.BusBlock{})
			require.NotContains(t, peerSentBus.Receivers(testPeerPath, nsID, updateID, events.DirSent, nil, nil), "alias1",
				"%s must never receive sent UPDATEs even when a peer targets it with direction=[sent] (%s)", name, reason)
		})
	}
}

func requireReceivedOnlyUpdateDecl(t *testing.T, name, reason string, receives []rpc.MessageReceiveDecl) {
	t.Helper()
	foundUpdate := false
	for _, decl := range receives {
		if decl.Message != bgpUpdateMsgName {
			continue
		}
		foundUpdate = true
		var dirValues []string
		for _, tag := range decl.Tags {
			if tag.Name == events.TagDirection {
				dirValues = append(dirValues, tag.Values...)
			}
		}
		require.Equal(t, []string{events.DirectionReceived}, dirValues,
			"%s must restrict bgp:update reception to direction=[received] (%s)", name, reason)
	}
	require.True(t, foundUpdate, "%s must declare bgp:update in Receives", name)
}

func buildPeerAndEndpointParticipants(
	endpointName string,
	receives []rpc.MessageReceiveDecl,
	peerBlock, endpointOwnBlock, endpointBusSideBlock plugin.BusBlock,
) []pluginserver.BusParticipantInput {
	return []pluginserver.BusParticipantInput{
		{
			Path: testPeerPath,
			Sends: []rpc.MessageSendDecl{{
				Message: bgpUpdateMsgName,
				Tags: []rpc.MessageTagDecl{{
					Name:   events.TagDirection,
					Values: []string{events.DirectionReceived, events.DirectionSent},
				}},
			}},
			OwnBlock:  peerBlock,
			OwnSource: testPeerPath + " > bus",
		},
		{
			Path:          plugin.EndpointPath(endpointName),
			EndpointName:  endpointName,
			Receives:      receives,
			OwnBlock:      endpointOwnBlock,
			OwnSource:     plugin.EndpointPath(endpointName),
			BusSideBlock:  endpointBusSideBlock,
			BusSideSource: "bus > participant " + plugin.EndpointPath(endpointName),
		},
	}
}

func compilePeerAndEndpointBus(
	t *testing.T,
	endpointName string,
	receives []rpc.MessageReceiveDecl,
	peerBlock, endpointOwnBlock, endpointBusSideBlock plugin.BusBlock,
) *pluginserver.CompiledBus {
	t.Helper()
	compiled, err := pluginserver.CompileBus(nil, buildPeerAndEndpointParticipants(
		endpointName, receives, peerBlock, endpointOwnBlock, endpointBusSideBlock,
	))
	require.NoError(t, err)
	return compiled
}

func parseEndpointBusBlock(t *testing.T, pluginName, cfgText, endpointName string) plugin.BusBlock {
	t.Helper()
	tree, err := config.ParseTreeForValidation(cfgText)
	require.NoError(t, err, "config for %s must parse against the real YANG schema", pluginName)
	busMap := childMap(tree.ToMap(), "bus")
	epMap := childMap(childMap(busMap, "endpoint"), endpointName)
	require.NotNil(t, epMap, "parsed tree must contain bus > endpoint %s", endpointName)
	var loc textbuf.Buffer
	block, err := plugin.ParseBusBlockFromMap(epMap, loc.Str("bus > endpoint ").Str(endpointName).String())
	require.NoError(t, err)
	return block
}

func parseParticipantBusBlock(t *testing.T, cfgText, participantPath string) plugin.BusBlock {
	t.Helper()
	tree, err := config.ParseTreeForValidation(cfgText)
	require.NoError(t, err, "config with central participant block must parse against the real YANG schema")
	busMap := childMap(tree.ToMap(), "bus")
	pMap := childMap(childMap(busMap, "participant"), participantPath)
	require.NotNil(t, pMap, "parsed tree must contain bus > participant %s", participantPath)
	var loc textbuf.Buffer
	block, err := plugin.ParseBusBlockFromMap(pMap, loc.Str("bus > participant ").Str(participantPath).String())
	require.NoError(t, err)
	return block
}

func parsePeerBusBlock(t *testing.T, cfgText, peerName string) plugin.BusBlock {
	t.Helper()
	tree, err := config.ParseTreeForValidation(cfgText)
	require.NoError(t, err, "peer config with bus block must parse against the real YANG schema")
	bgpMap := childMap(tree.ToMap(), "bgp")
	peerMap := childMap(childMap(bgpMap, "peer"), peerName)
	require.NotNil(t, peerMap, "parsed tree must contain bgp > peer %s", peerName)
	busMap := childMap(peerMap, "bus")
	require.NotNil(t, busMap, "parsed tree must contain bgp > peer %s > bus", peerName)
	var loc textbuf.Buffer
	block, err := plugin.ParseBusBlockFromMap(busMap, loc.Str("bgp > peer ").Str(peerName).Str(" > bus").String())
	require.NoError(t, err)
	return block
}

func childMap(node map[string]any, key string) map[string]any {
	if node == nil {
		return nil
	}
	child, _ := node[key].(map[string]any)
	return child
}
