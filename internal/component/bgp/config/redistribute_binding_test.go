// Design: docs/architecture/api/architecture.md -- redistribution bus requirements.
// Related: bus_requirements.go -- checkRedistributionRequirements.
package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/opererr"
)

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

// TestRedistributeBGPSourceFeedsLocRIB verifies that a BGP redistribution
// source with no written bus block feeds the auto-loaded Loc-RIB with updates,
// state, and refresh via registered defaults.
//
// VALIDATES: AC-1 -- zero-boilerplate BGP-to-OSPF redistribution feeds bgp-rib.
// PREVENTS: silent stage-1 event drops when no explicit bus block is written.
func TestRedistributeBGPSourceFeedsLocRIB(t *testing.T) {
	g := graphFromConfig(t, redistOSPFImportsBGP)
	require.Contains(t, fedBy(g, bgpevents.EventUpdate, events.DirReceived, "10.0.0.2"), "bgp-rib")
	require.Contains(t, fedBy(g, bgpevents.EventUpdate, events.DirSent, "10.0.0.2"), "bgp-rib")
	require.Contains(t, fedBy(g, bgpevents.EventState, events.DirUnspecified, "10.0.0.2"), "bgp-rib")
	require.Contains(t, fedBy(g, bgpevents.EventRefresh, events.DirReceived, "10.0.0.2"), "bgp-rib")
}

// TestRedistributeBGPSourceRefusesNarrowedLocRIB verifies AC-14 / S18 / S33:
// cutting the Loc-RIB below what `source bgp` requires is refused with a typed
// operational fix instead of loading and moving no routes.
//
// VALIDATES: checkRedistributionRequirements refuses narrowed Loc-RIB feeds.
// PREVENTS: a narrowed bgp-rib endpoint silently dropping redistributed routes.
func TestRedistributeBGPSourceRefusesNarrowedLocRIB(t *testing.T) {
	const input = `
bus {
    endpoint rib {
        internal bgp-rib;
        receive { allow [ bgp:state ]; }
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
	_, err := config.LoadConfig(input, "", nil)
	require.Error(t, err, "cutting bgp:update from rib while redistribution imports bgp must be refused")
	var oe *opererr.Error
	require.ErrorAs(t, err, &oe)
	require.Contains(t, err.Error(), "redistribution")
	require.NotEmpty(t, oe.Fix)
}

// TestRedistributeBGPSourceUsesConfiguredAlias verifies that declaring an
// aliased Loc-RIB endpoint (`bus { endpoint rib { internal bgp-rib; } }`)
// satisfies BGP source redistribution under the alias name.
//
// VALIDATES: AC-13, AC-14 -- aliased Loc-RIB endpoints resolve by registration
// identity and feed the alias on the compiled DeliveryGraph.
// PREVENTS: requirement checks hardcoding "bgp-rib" when an alias is declared.
func TestRedistributeBGPSourceUsesConfiguredAlias(t *testing.T) {
	const input = `
bus {
    endpoint rib {
        internal bgp-rib;
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
	g := graphFromConfig(t, input)
	require.Contains(t, fedBy(g, bgpevents.EventUpdate, events.DirReceived, "10.0.0.2"), "rib")
	require.NotContains(t, fedBy(g, bgpevents.EventUpdate, events.DirReceived, "10.0.0.2"), "bgp-rib")
}

// TestRedistributeBGPSourceCoversDynamicGroupMembers verifies that dynamic
// groups are evaluated by checkRedistributionRequirements identically to static
// peers.
//
// VALIDATES: dynamic group members feed bgp-rib by default and cutting their
// bus send target refuses commit.
// PREVENTS: dynamic group peers silently bypassing redistribution requirements.
func TestRedistributeBGPSourceCoversDynamicGroupMembers(t *testing.T) {
	const okInput = `
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
	g := graphFromConfig(t, okInput)
	require.Contains(t, fedBy(g, bgpevents.EventUpdate, events.DirReceived, "dynamic-group:members"), "bgp-rib")

	const cutInput = `
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
        bus { send { to none; } }
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
	_, err := config.LoadConfig(cutInput, "", nil)
	require.Error(t, err, "cutting a dynamic group away from bgp-rib under source bgp must be refused")
}

// TestDestinationBGPPermitsOrchestratorSend verifies that `destination bgp`
// permits the auto-loaded redistribute orchestrator to send bgp:update and
// receive bgp:state without receiving bgp:update.
//
// VALIDATES: AC-1 -- zero-boilerplate IGP-to-BGP redistribution works via
// registered defaults.
// PREVENTS: redistribute-orchestrator missing send permission or receiving
// undeclared update traffic.
func TestDestinationBGPPermitsOrchestratorSend(t *testing.T) {
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
	g := graphFromConfig(t, input)
	require.True(t, g.MaySend("10.0.0.2", "redistribute-orchestrator", bgpevents.SendUpdate))
	require.True(t, g.ProcessReceivesState("10.0.0.2", "redistribute-orchestrator"))
	require.NotContains(t, fedBy(g, bgpevents.EventUpdate, events.DirReceived, "10.0.0.2"), "redistribute-orchestrator")
}

// TestDestinationBGPRefusesNarrowedOrchestrator verifies AC-14 / S18 / S33:
// cutting the orchestrator's send or state reception under `destination bgp`
// is refused with a typed operational fix.
//
// VALIDATES: checkRedistributionRequirements refuses narrowed orchestrator send
// or state reception.
// PREVENTS: a narrowed redistribute-orchestrator silently failing to originate routes.
func TestDestinationBGPRefusesNarrowedOrchestrator(t *testing.T) {
	const input = `
bus {
    endpoint redistribute-orchestrator {
        internal redistribute-orchestrator;
        send { allow none; }
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

isis {
    net 49.0001.0000.0000.0001.00
}

redistribute {
    destination bgp {
        import isis
    }
}
`
	_, err := config.LoadConfig(input, "", nil)
	require.Error(t, err, "cutting bgp:update send from redistribute-orchestrator under destination bgp must be refused")
	var oe *opererr.Error
	require.ErrorAs(t, err, &oe)
	require.Contains(t, err.Error(), "redistribution")
	require.NotEmpty(t, oe.Fix)
}

// TestBothRedistributeDirectionsCoexist verifies a config with both
// `source bgp` and `destination bgp` satisfies both checks simultaneously.
//
// VALIDATES: both redistribution directions coexist cleanly on one compiled graph.
// PREVENTS: one redistribution direction interfering with the other.
func TestBothRedistributeDirectionsCoexist(t *testing.T) {
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
    destination isis {
        import bgp
    }
    destination bgp {
        import isis
    }
}
`
	g := graphFromConfig(t, input)
	require.Contains(t, fedBy(g, bgpevents.EventUpdate, events.DirReceived, "10.0.0.2"), "bgp-rib")
	require.True(t, g.MaySend("10.0.0.2", "redistribute-orchestrator", bgpevents.SendUpdate))
	require.True(t, g.ProcessReceivesState("10.0.0.2", "redistribute-orchestrator"))
}
