// Design: docs/architecture/api/architecture.md -- compiled bus delivery graph from BGP config.
// Related: bus_requirements.go -- compileBGPBusContext, buildBGPDeliveryPeersFromTree.
package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
)

// noInsertingPluginLoaded sets a daemon that runs no compiled-in plugin, so
// config load places no declared filter insertion.
func noInsertingPluginLoaded(t *testing.T) {
	t.Helper()
	t.Cleanup(registry.SetRuntimeLoadedPlugins(func() []string { return nil }))
}

// graphFromConfig runs the full config load and compiled-bus build path and
// returns the DeliveryGraph compiled from the configuration tree.
func graphFromConfig(t *testing.T, input string) *pluginserver.DeliveryGraph {
	t.Helper()
	result, err := config.LoadConfig(input, "", nil)
	require.NoError(t, err)

	peers, groups, resolvedBGPTree, err := buildPeerPopulationsWithResolved(result.Tree.Clone())
	require.NoError(t, err)

	busCtx, err := compileBGPBusContext(result.Tree, peers, groups, resolvedBGPTree, nil)
	require.NoError(t, err)
	return busCtx.graph
}

// fedBy returns the processes a peer feeds for one event type, asked as the
// peer-scoped delivery sites ask it (bgp/server/events.go).
func fedBy(g *pluginserver.DeliveryGraph, eventType string, dir events.Direction, peerAddr string) []string {
	return g.Receivers(
		events.LookupNamespaceID(bgpevents.Namespace),
		events.LookupEventTypeID(eventType),
		dir,
		peerAddr,
	)
}

// TestGraphMemberListReplacesGroupList verifies AC-2 / M-14 in BOTH directions:
// the member that restates a bus item replaces its group's item for itself,
// while every sibling member inherits the group's item intact.
//
// VALIDATES: item-by-item bus block inheritance from group to peer.
// PREVENTS: a member's override leaking to siblings or the group's bus block
// being dropped during compiled-bus construction.
func TestGraphMemberListReplacesGroupList(t *testing.T) {
	noInsertingPluginLoaded(t)
	g := graphFromConfig(t, `
bus {
    endpoint looking-glass {
        external { run "/bin/true"; }
        receive { allow [ bgp:update bgp:state ]; }
    }
}
bgp {
    router-id 10.0.0.1;
    session {
        asn {
            local 65000;
        }
    }
    group transit {
        bus {
            send {
                allow [ bgp:update bgp:state ];
                to [ /bus/endpoint/looking-glass ];
                message bgp:update { tag direction [ received ]; }
            }
        }
        peer inherits {
            connection {
                remote { ip 192.0.2.1; }
                local { ip auto; }
            }
            session { asn { remote 65001; } }
        }
        peer restates {
            connection {
                remote { ip 192.0.2.2; }
                local { ip auto; }
            }
            session { asn { remote 65002; } }
            bus {
                send {
                    allow [ bgp:state ];
                }
            }
        }
    }
}
`)

	// The member that restates send > allow replaces only send > allow while
	// inheriting send > to [ /bus/endpoint/looking-glass ] from its group.
	assert.Equal(t, []string{"looking-glass"}, fedBy(g, bgpevents.EventState, events.DirUnspecified, "192.0.2.2"))
	assert.Empty(t, fedBy(g, bgpevents.EventUpdate, events.DirReceived, "192.0.2.2"),
		"the member's allow list replaces the group's, so update is gone")

	// The sibling member keeps the group's full bus rules intact.
	assert.Equal(t, []string{"looking-glass"}, fedBy(g, bgpevents.EventUpdate, events.DirReceived, "192.0.2.1"))
	assert.Empty(t, fedBy(g, bgpevents.EventUpdate, events.DirSent, "192.0.2.1"))
	assert.Equal(t, []string{"looking-glass"}, fedBy(g, bgpevents.EventState, events.DirUnspecified, "192.0.2.1"))
}

// TestGraphInheritsGroupBusBlock verifies a peer that states no bus block of
// its own inherits the group's bus block, while a peer that cuts bus delivery
// feeds nobody.
//
// VALIDATES: compiled bus delivery resolves group > bus inheritance onto member peers.
// PREVENTS: a group-configured fleet going silent or losing group send/receive rules.
func TestGraphInheritsGroupBusBlock(t *testing.T) {
	noInsertingPluginLoaded(t)
	g := graphFromConfig(t, `
bus {
    endpoint looking-glass {
        external { run "/bin/true"; }
        receive { allow [ bgp:update ]; }
        send { allow [ bgp:update ]; }
    }
}
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    group transit {
        bus {
            send {
                allow [ bgp:update ];
                to [ /bus/endpoint/looking-glass ];
                message bgp:update { tag direction [ received ]; }
            }
            receive {
                from [ /bus/endpoint/looking-glass ];
            }
        }
        peer first {
            connection {
                remote { ip 192.0.2.1; }
                local { ip auto; }
            }
            session { asn { remote 65001; } }
        }
    }
    peer standalone {
        connection {
            remote { ip 198.51.100.1; }
            local { ip auto; }
        }
        session { asn { remote 65003; } }
        bus {
            send { to none; }
            receive { from none; }
        }
    }
}
`)

	assert.Equal(t, []string{"looking-glass"}, fedBy(g, bgpevents.EventUpdate, events.DirReceived, "192.0.2.1"))
	assert.Empty(t, fedBy(g, bgpevents.EventUpdate, events.DirReceived, "198.51.100.1"),
		"a peer with send > to none feeds nobody")
	assert.True(t, g.MaySend("192.0.2.1", "looking-glass", bgpevents.SendUpdate))
	assert.False(t, g.MaySend("198.51.100.1", "looking-glass", bgpevents.SendUpdate))

	peers := g.Inspect()
	require.Len(t, peers, 2)
	byAddr := map[string][]pluginserver.ProcessEdges{}
	for _, p := range peers {
		byAddr[p.Peer] = p.Processes
	}
	require.Len(t, byAddr["192.0.2.1"], 1)
	assert.Equal(t, "looking-glass", byAddr["192.0.2.1"][0].Process)
	assert.Equal(t, []string{"update-received"}, byAddr["192.0.2.1"][0].Receive)
	assert.Equal(t, []string{"update"}, byAddr["192.0.2.1"][0].Send)
	assert.Empty(t, byAddr["198.51.100.1"])
}

const storyOneConfig = `
bus {
    endpoint bgp-rib {
        internal bgp-rib;
        receive { allow none; }
        send { allow none; }
    }
    endpoint looking-glass {
        external { run "/bin/true"; }
        receive {
            allow [ bgp:update bgp:state ];
            from [ /bgp/group/transit /bgp/peer/both-programs ];
        }
    }
    endpoint route-injector {
        external { run "/bin/true"; }
        receive {
            allow [ bgp:state ];
            from [ /bgp/peer/injector-only ];
        }
        send {
            allow [ bgp:update ];
            to [ /bgp/peer/both-programs /bgp/peer/injector-only ];
        }
    }
}
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    group transit {
        peer transit-a {
            connection { remote { ip 192.0.2.1; } local { ip auto; } }
            session { asn { remote 65001; } }
        }
        peer transit-b {
            connection { remote { ip 192.0.2.2; } local { ip auto; } }
            session { asn { remote 65002; } }
        }
        peer transit-c {
            connection { remote { ip 192.0.2.3; } local { ip auto; } }
            session { asn { remote 65003; } }
            bus {
                send { allow [ bgp:state ]; }
            }
        }
    }
    peer both-programs {
        connection { remote { ip 198.51.100.1; } local { ip auto; } }
        session { asn { remote 65004; } }
    }
    peer injector-only {
        connection { remote { ip 198.51.100.2; } local { ip auto; } }
        session { asn { remote 65005; } }
    }
    peer unattached {
        connection { remote { ip 203.0.113.1; } local { ip auto; } }
        session { asn { remote 65006; } }
    }
}
`

// TestStoryOneProducesItsDeliveryTable verifies the six-peer delivery table
// across group paths, member allow overrides, and endpoint-side scoping.
//
// VALIDATES: DeliveryGraph.Inspect reports the exact per-peer receive and send
// table compiled from bus endpoint and participant blocks.
// PREVENTS: group path scoping or peer-level send overrides diverging from the
// compiled delivery table.
func TestStoryOneProducesItsDeliveryTable(t *testing.T) {
	noInsertingPluginLoaded(t)
	g := graphFromConfig(t, storyOneConfig)

	byAddr := map[string]map[string]pluginserver.ProcessEdges{}
	for _, peer := range g.Inspect() {
		procs := map[string]pluginserver.ProcessEdges{}
		for _, p := range peer.Processes {
			procs[p.Process] = p
		}
		byAddr[peer.Peer] = procs
	}
	require.Len(t, byAddr, 6)

	for _, row := range []struct {
		addr             string
		glassFed         []string
		injectorFed      []string
		injectorAnnounce []string
	}{
		{addr: "192.0.2.1", glassFed: []string{"state", "update"}},
		{addr: "192.0.2.2", glassFed: []string{"state", "update"}},
		{addr: "192.0.2.3", glassFed: []string{"state"}},
		{addr: "198.51.100.1", glassFed: []string{"state", "update"}, injectorAnnounce: []string{"update"}},
		{addr: "198.51.100.2", injectorFed: []string{"state"}, injectorAnnounce: []string{"update"}},
		{addr: "203.0.113.1"},
	} {
		procs := byAddr[row.addr]
		assert.Equal(t, row.glassFed, procs["looking-glass"].Receive, "%s feeds looking-glass", row.addr)
		assert.Equal(t, row.injectorFed, procs["route-injector"].Receive, "%s feeds route-injector", row.addr)
		assert.Equal(t, row.injectorAnnounce, procs["route-injector"].Send, "%s lets route-injector announce", row.addr)
		assert.Empty(t, procs["looking-glass"].Send, "%s lets looking-glass announce", row.addr)
	}
}

// TestStoryTwoProducesItsTwoIndependentDirections verifies one peer and one
// endpoint with independent receive and send directions on the compiled bus.
//
// VALIDATES: receive and send directions remain independent and plain
// bgp:update admits both received and sent update directions.
// PREVENTS: receive and send filters collapsing into one coupled permission.
func TestStoryTwoProducesItsTwoIndependentDirections(t *testing.T) {
	noInsertingPluginLoaded(t)
	g := graphFromConfig(t, `
bus {
    endpoint bgp-rib {
        internal bgp-rib;
        receive { allow none; }
        send { allow none; }
    }
    endpoint policy-engine {
        external { run "/bin/true"; }
        receive { allow [ bgp:update bgp:state ]; }
        send { allow [ bgp:update ]; }
    }
}
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer policy {
        connection {
            remote { ip 198.51.100.3; }
            local { ip auto; }
        }
        session { asn { remote 65007; } }
    }
}
`)

	peers := g.Inspect()
	require.Len(t, peers, 1)
	assert.Equal(t, "198.51.100.3", peers[0].Peer)
	var policyEdge *pluginserver.ProcessEdges
	for i := range peers[0].Processes {
		if peers[0].Processes[i].Process == "policy-engine" {
			policyEdge = &peers[0].Processes[i]
			break
		}
	}
	require.NotNil(t, policyEdge, "policy-engine edge must be present in Inspect()")
	assert.Equal(t, []string{"state", "update"}, policyEdge.Receive)
	assert.Equal(t, []string{"update"}, policyEdge.Send)

	assert.Contains(t, fedBy(g, bgpevents.EventUpdate, events.DirSent, "198.51.100.3"), "policy-engine")
	assert.Contains(t, fedBy(g, bgpevents.EventUpdate, events.DirReceived, "198.51.100.3"), "policy-engine")
}
