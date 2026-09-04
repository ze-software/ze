package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
)

// bindingsFor runs the config path a running daemon runs and returns the named
// peer's process bindings.
func bindingsFor(t *testing.T, input, peerName string) []reactor.ProcessBinding {
	t.Helper()
	r, err := LoadReactor(input)
	require.NoError(t, err)
	for _, p := range r.Peers() {
		settings := p.Settings()
		if settings.Name == peerName {
			return settings.ProcessBindings
		}
	}
	t.Fatalf("peer %q not built from the config", peerName)
	return nil
}

// bindingNamed returns the one binding for a process, and fails when the peer
// holds none or holds more than one. Two bindings for one process is the
// failure AC-4 names, so counting is part of the assertion rather than a
// lookup detail.
func bindingNamed(t *testing.T, bindings []reactor.ProcessBinding, process string) *reactor.ProcessBinding {
	t.Helper()
	var found []*reactor.ProcessBinding
	for i := range bindings {
		if bindings[i].PluginName == process {
			found = append(found, &bindings[i])
		}
	}
	require.Lenf(t, found, 1, "expected exactly one binding for process %q, got %d", process, len(found))
	return found[0]
}

const redistOSPFImportsBGP = `
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer upstream {
        connection {
            remote { ip 10.0.0.2; }
            local  { ip 10.0.0.1; }
        }
        session {
            asn { local 65000; remote 65001; }
        }
    }
}

ospf {
    router-id 10.0.0.1
}

redistribute {
    destination ospf {
        import bgp
    }
}
`

// TestRedistributeBGPSourceWiresLocRIBBinding is the wiring test for AC-1. A
// config names a BGP redistribution source and writes no `plugin` block and no
// `attach process` block, and it still feeds the built-in Loc-RIB.
//
// VALIDATES: AC-1, AC-7 -- the binding exists, names the process the plugin
// server auto-loads, and grants the three event types the Loc-RIB declares.
// PREVENTS: the silent stage-1 drop, where DeliveryPeersFromSettings builds a
// graph from ProcessBindings alone and a peer with no attach block grants
// nothing, so PeerScopedProcs answers empty and the UPDATE is discarded.
func TestRedistributeBGPSourceWiresLocRIBBinding(t *testing.T) {
	binding := bindingNamed(t, bindingsFor(t, redistOSPFImportsBGP, "upstream"), "bgp-rib")

	require.False(t, binding.ReceiveAll, "the derived grant names its types; the wildcard would grant a type a plugin registers later")
	require.Equal(t, events.DirBoth, binding.Receive[bgpevents.EventUpdate])
	require.Equal(t, events.DirBoth, binding.Receive[bgpevents.EventState])
	require.Equal(t, events.DirBoth, binding.Receive[bgpevents.EventRefresh])
	require.False(t, binding.SendAll, "the Loc-RIB is fed, it does not push routes onto this peer's wire")
	require.Empty(t, binding.Send)
}

// TestPeerWithAutoWiredLocRIBReceivesUpdate asks the delivery index the exact
// question the peer-scoped delivery site asks (Server.PeerScopedProcs, through
// DeliveryGraph.Receivers). The assertion is then about what a received UPDATE
// reaches, rather than about a struct field.
//
// VALIDATES: AC-1 -- the derived binding survives DeliveryPeersFromSettings and
// resolves the Loc-RIB process for an UPDATE in the received direction.
func TestPeerWithAutoWiredLocRIBReceivesUpdate(t *testing.T) {
	g := graphFromConfig(t, redistOSPFImportsBGP)
	require.Equal(t, []string{"bgp-rib"}, fedBy(g, bgpevents.EventUpdate, events.DirReceived, "10.0.0.2"))
}

// TestExplicitBindingWins holds the derived binding to AC-4: an operator who
// wrote a narrower grant keeps it, and gains no second binding for that process.
//
// VALIDATES: AC-4.
// PREVENTS: a derived binding appended beside the operator's, which would grant
// update and refresh to a process the operator granted state alone.
func TestExplicitBindingWins(t *testing.T) {
	const input = `
plugin {
    internal rib {
        use bgp-rib;
    }
}

bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer upstream {
        connection {
            remote { ip 10.0.0.2; }
            local  { ip 10.0.0.1; }
        }
        session {
            asn { local 65000; remote 65001; }
        }
        attach process rib {
            receive [ state ];
        }
    }
}

ospf {
    router-id 10.0.0.1
}

redistribute {
    destination ospf {
        import bgp
    }
}
`
	bindings := bindingsFor(t, input, "upstream")
	binding := bindingNamed(t, bindings, "rib")
	require.Equal(t, events.DirBoth, binding.Receive[bgpevents.EventState])
	require.Equal(t, events.DirUnspecified, binding.Receive[bgpevents.EventUpdate], "the operator granted state alone")
	require.Equal(t, events.DirUnspecified, binding.Receive[bgpevents.EventRefresh], "the operator granted state alone")

	for _, b := range bindings {
		require.NotEqual(t, "bgp-rib", b.PluginName,
			"the alias `rib` is the process the config declares for bgp-rib; a second binding under the registry name would name a process nothing runs")
	}
}

// TestDerivedBindingNamesTheConfiguredAlias covers the other half of the alias.
// The operator declared the plugin under a name of their own, and attached it
// to no peer. The derived binding is owed, and it MUST name the alias.
//
// VALIDATES: AC-1 for the two OSPF interop scenarios, whose ze.conf declares
// `plugin { internal rib { use bgp-rib; } }` and no attach block.
// PREVENTS: a derived binding hardcoded to the registry name, which would name
// a process the plugin server never starts under that config.
func TestDerivedBindingNamesTheConfiguredAlias(t *testing.T) {
	const input = `
plugin {
    internal rib {
        use bgp-rib;
    }
}

bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer upstream {
        connection {
            remote { ip 10.0.0.2; }
            local  { ip 10.0.0.1; }
        }
        session {
            asn { local 65000; remote 65001; }
        }
    }
}

ospf {
    router-id 10.0.0.1
}

redistribute {
    destination ospf {
        import bgp
    }
}
`
	binding := bindingNamed(t, bindingsFor(t, input, "upstream"), "rib")
	require.Equal(t, events.DirBoth, binding.Receive[bgpevents.EventUpdate])
}

// TestNoRedistributeNoBinding holds AC-6: a config that asks for no
// redistribution pays nothing for this feature.
//
// VALIDATES: AC-6 -- no binding is added, so the peer's delivery set stays what
// the config grants and the per-UPDATE work is unchanged.
func TestNoRedistributeNoBinding(t *testing.T) {
	const input = `
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer upstream {
        connection {
            remote { ip 10.0.0.2; }
            local  { ip 10.0.0.1; }
        }
        session {
            asn { local 65000; remote 65001; }
        }
    }
}

ospf {
    router-id 10.0.0.1
}
`
	require.Empty(t, bindingsFor(t, input, "upstream"))
}

// TestNonBGPRedistributeSourceAddsNoBinding is the negative that separates the
// SOURCE from the block. A `redistribute` block exists, none of its sources is
// the Loc-RIB, and no Loc-RIB delivery is derived.
//
// The config below does derive the orchestrator's send permission, because it
// carries `destination bgp`. That is the other rule's binding, and
// TestDestinationBGPWiresOrchestratorSendPermission is what asserts it. What
// this test holds is that the two are independent.
//
// VALIDATES: AC-6 -- the Loc-RIB gate is the SOURCE, not the presence of the
// block.
// PREVENTS: wiring the Loc-RIB into every peer of a daemon that only exports
// IS-IS routes into BGP, which R-1 names as the per-UPDATE cost risk. Such a
// peer pays no per-UPDATE delivery: the orchestrator's grant carries `state`,
// which fires once per session change.
func TestNonBGPRedistributeSourceAddsNoBinding(t *testing.T) {
	const input = `
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer upstream {
        connection {
            remote { ip 10.0.0.2; }
            local  { ip 10.0.0.1; }
        }
        session {
            asn { local 65000; remote 65001; }
        }
    }
}

isis {
    net 49.0001.0000.0000.0001.00
}

redistribute {
    destination bgp {
        import isis
    }
}
`
	for _, b := range bindingsFor(t, input, "upstream") {
		require.NotEqual(t, "bgp-rib", b.PluginName,
			"no rule names a BGP source, so no peer owes the Loc-RIB a delivery")
	}
}

// TestDerivedBindingReachesDynamicGroupMembers holds the derived binding to the
// rule peersAndDynamicGroups exists to enforce. One walk builds both peer
// populations, so a layer applied to the statically configured peers reaches a
// dynamic group's template by the same line.
//
// VALIDATES: AC-1 for an IXP route server, whose members arrive on the listener
// and are built from the group template.
// PREVENTS: the shape that left dynamic members with no policy at all, where a
// second walk read the layers somebody remembered.
func TestDerivedBindingReachesDynamicGroupMembers(t *testing.T) {
	const input = `
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    group members {
        connection {
            remote { ip dynamic; connect false; range 10.0.0.0/24; }
            local  { ip 10.0.0.1; accept true; }
        }
        session {
            asn { local 65000; remote 65001; }
        }
    }
}

ospf {
    router-id 10.0.0.1
}

redistribute {
    destination ospf {
        import bgp
    }
}
`
	schema, err := config.YANGSchema()
	require.NoError(t, err)
	tree, err := config.NewParser(schema).Parse(input)
	require.NoError(t, err)

	_, groups, err := peersAndDynamicGroups(tree)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	binding := bindingNamed(t, groups[0].Settings.ProcessBindings, "bgp-rib")
	require.Equal(t, events.DirBoth, binding.Receive[bgpevents.EventUpdate])
}

// TestDerivedBindingRaisesNoDeliveryDisagreement holds AC-7 through the
// producing function rather than through a log line. reconcileDelivery's
// judgement is exposed as data, so the test asserts the finding set is empty
// for a peer holding only the derived binding.
//
// VALIDATES: AC-7 -- no `no peer attaches it` warning, and no grant the plugin
// never declared.
// PREVENTS: a derived grant wider than the Loc-RIB's own declaration, which
// would trade a silent drop for a permanent startup warning.
func TestDerivedBindingRaisesNoDeliveryDisagreement(t *testing.T) {
	g := graphFromConfig(t, redistOSPFImportsBGP)
	for _, peer := range g.Inspect() {
		for _, edges := range peer.Processes {
			require.Equal(t, "bgp-rib", edges.Process)
			require.Empty(t, edges.Unresolved, "every derived receive token resolves to a registered event type")
		}
	}
	require.Equal(t, 1, g.Len())

	// Each type is asked in the direction its own delivery site asks it in
	// (internal/component/bgp/server/events.go). A state event carries none,
	// and a ROUTE-REFRESH ze received carries the received one.
	require.Equal(t, []string{"bgp-rib"}, fedBy(g, bgpevents.EventState, events.DirUnspecified, "10.0.0.2"))
	require.Equal(t, []string{"bgp-rib"}, fedBy(g, bgpevents.EventRefresh, events.DirReceived, "10.0.0.2"))
}

// TestDestinationBGPWiresOrchestratorSendPermission is the other half of the
// derived plumbing. A `destination bgp` rule moves a route onto a peer's wire
// through the redistribute orchestrator, and a peer grants a process that right
// with `send [ update ]`.
//
// VALIDATES: `redistribute { destination bgp { import <src> } }` works as
// written, which is what test/interop/scenarios/isis-redist-frr and
// test/interop/scenarios/as112-redistribute-lab each ask for.
// PREVENTS: the mirror of the Loc-RIB defect. Peer.maySend
// (internal/component/bgp/reactor/send_permission.go) refuses a process the
// peer's attach block does not name, so the consumer's UpdateRoute was refused
// for every peer and the rule moved nothing.
func TestDestinationBGPWiresOrchestratorSendPermission(t *testing.T) {
	const input = `
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer upstream {
        connection {
            remote { ip 10.0.0.2; }
            local  { ip 10.0.0.1; }
        }
        session {
            asn { local 65000; remote 65001; }
        }
    }
}

isis {
    net 49.0001.0000.0000.0001.00
}

redistribute {
    destination bgp {
        import isis
    }
}
`
	binding := bindingNamed(t, bindingsFor(t, input, "upstream"), "redistribute-orchestrator")
	require.True(t, binding.MaySend("update"), "the orchestrator must be permitted to put the route on this peer's wire")
	require.True(t, binding.ReceivesPeerState(), "the peer-up edge is what fires the late-join replay")
	require.False(t, binding.SendAll, "the derived grant names its type")
	require.Equal(t, events.DirUnspecified, binding.Receive[bgpevents.EventUpdate],
		"the orchestrator declares only state; granting update would raise a delivery disagreement")
}

// TestOperatorOrchestratorBindingWins is AC-4 for the second derived binding.
//
// VALIDATES: an operator who granted the orchestrator a narrower send list
// keeps it, and gains no second binding.
func TestOperatorOrchestratorBindingWins(t *testing.T) {
	const input = `
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer upstream {
        connection {
            remote { ip 10.0.0.2; }
            local  { ip 10.0.0.1; }
        }
        session {
            asn { local 65000; remote 65001; }
        }
        attach process redistribute-orchestrator {
            receive [ state ];
        }
    }
}

isis {
    net 49.0001.0000.0000.0001.00
}

redistribute {
    destination bgp {
        import isis
    }
}
`
	binding := bindingNamed(t, bindingsFor(t, input, "upstream"), "redistribute-orchestrator")
	require.False(t, binding.MaySend("update"),
		"the operator granted no send, and a derived binding never widens what one says")
}

// TestDestinationOSPFAddsNoOrchestratorBinding is the negative that scopes the
// send permission to the destination that needs it.
//
// VALIDATES: a rule feeding an IGP grants nothing on a BGP peer's wire.
// PREVENTS: every peer of an IGP-only redistribution gaining a send permission
// the config never asked for.
func TestDestinationOSPFAddsNoOrchestratorBinding(t *testing.T) {
	for _, b := range bindingsFor(t, redistOSPFImportsBGP, "upstream") {
		require.NotEqual(t, "redistribute-orchestrator", b.PluginName,
			"no rule feeds bgp, so no peer owes the orchestrator a send permission")
	}
}

// TestBothDerivedBindingsCoexist covers the config the isis-redist-frr interop
// scenario carries: one `redistribute` block whose rules imply BOTH derived
// bindings on the same peer.
//
// VALIDATES: the two derivations are independent and neither displaces the
// other.
func TestBothDerivedBindingsCoexist(t *testing.T) {
	const input = `
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer upstream {
        connection {
            remote { ip 10.0.0.2; }
            local  { ip 10.0.0.1; }
        }
        session {
            asn { local 65000; remote 65001; }
        }
    }
}

isis {
    net 49.0001.0000.0000.0001.00
}

redistribute {
    destination bgp {
        import isis
    }
    destination isis {
        import bgp
    }
}
`
	bindings := bindingsFor(t, input, "upstream")
	require.Len(t, bindings, 2)
	require.True(t, bindingNamed(t, bindings, "redistribute-orchestrator").MaySend("update"))
	require.Equal(t, events.DirBoth, bindingNamed(t, bindings, "bgp-rib").Receive[bgpevents.EventUpdate])
}
