// Design: docs/architecture/route-selection.md -- AIGP bus requirement checks.
// Related: bus_requirements.go -- checkExplicitAIGPRequirements, peerExplicitAIGP.
package bgpconfig

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/opererr"
)

const aigpDeliveryConfig = `
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer upstream {
        connection { remote { ip 10.0.0.2; } local { ip 10.0.0.1; } }
        session { asn { local 65000; remote 65001; } aigp { enabled true; } }
        %s
    }
    peer other {
        connection { remote { ip 10.0.0.3; } local { ip 10.0.0.1; } }
        session { asn { local 65000; remote 65002; } }
    }
}
%s`

// TestAIGPDefaultActivatesNoRequirement validates AC-15 and S26: an iBGP peer
// using the RFC 7311 Section 3.3 default (no `aigp` block written) activates
// no bus requirement, so cutting the Loc-RIB from that peer commits cleanly;
// writing `session { aigp { ... } }` activates the requirement and refuses the
// same cut with a typed operational fix.
//
// VALIDATES: AC-15, S26 -- default iBGP AIGP activates no bus requirement,
// while explicit `session { aigp { ... } }` enforces Loc-RIB reception.
// PREVENTS: a default iBGP capability from silently rejecting valid bus
// narrowing (S-8) or an explicit AIGP config from loading without a RIB feed.
func TestAIGPDefaultActivatesNoRequirement(t *testing.T) {
	const base = `
bus {
    endpoint collector {
        external { run "/bin/true"; }
        receive { allow [ bgp:update bgp:state ]; }
    }
}
bgp {
    router-id 192.0.2.254;
    session { asn { local 65000; } }
    peer i1 {
        connection { remote { ip 192.0.2.11; } local { ip 192.0.2.254; } }
        session { asn { local 65000; remote 65000; } %s }
        bus { send { to [ /bus/endpoint/collector ]; } }
    }
    peer i2 {
        connection { remote { ip 192.0.2.12; } local { ip 192.0.2.254; } }
        session { asn { local 65000; remote 65000; } }
    }
}`

	t.Run("default iBGP without written aigp allows cutting Loc-RIB", func(t *testing.T) {
		cfg := fmt.Sprintf(base, "")
		res, err := config.LoadConfig(cfg, "", nil)
		require.NoError(t, err, "default iBGP AIGP must not activate a bus requirement")
		peers, err := PeersFromConfigTree(res.Tree.Clone())
		require.NoError(t, err)
		require.Len(t, peers, 2)
		require.True(t, peers[0].AIGPEnabled(), "RFC 7311 Section 3.3 iBGP session default remains enabled")
	})

	t.Run("explicit aigp written refuses cutting Loc-RIB", func(t *testing.T) {
		cfg := fmt.Sprintf(base, "aigp { enabled true; }")
		_, err := config.LoadConfig(cfg, "", nil)
		require.Error(t, err, "explicit aigp must refuse cutting Loc-RIB reception")
		var oe *opererr.Error
		require.ErrorAs(t, err, &oe)
		require.Contains(t, err.Error(), "AIGP")
		require.Contains(t, err.Error(), "i1")
		require.NotEmpty(t, oe.Fix)
	})
}

// TestAIGPFeedsSelectionOnExplicitAIGP checks that explicit AIGP configuration
// feeds the selecting RIB from both the AIGP peer and competing peers under
// the default compiled bus.
//
// VALIDATES: explicit AIGP receives candidate updates and peer state from all
// competing peers on the compiled DeliveryGraph.
// PREVENTS: AIGP selection missing competing routes from non-AIGP peers.
func TestAIGPFeedsSelectionOnExplicitAIGP(t *testing.T) {
	input := fmt.Sprintf(aigpDeliveryConfig, "", "")
	graph := graphFromConfig(t, input)
	for _, peer := range []string{"10.0.0.2", "10.0.0.3"} {
		consumers := fedBy(graph, bgpevents.EventUpdate, events.DirReceived, peer)
		require.Contains(t, consumers, "bgp-rib", "selection must also see competing routes from non-AIGP peers")
		require.Contains(t, fedBy(graph, bgpevents.EventState, events.DirUnspecified, peer), "bgp-rib")
	}
}

// TestAIGPRefusesIncompleteRIBFeed verifies that cutting either received
// updates or peer state from the selecting RIB is refused when AIGP is
// explicitly configured, while a complete narrowed feed is accepted.
//
// VALIDATES: checkExplicitAIGPRequirements enforces both bgp:update (received)
// and bgp:state on the selecting RIB.
// PREVENTS: partial candidate state silently breaking AIGP reselection.
func TestAIGPRefusesIncompleteRIBFeed(t *testing.T) {
	for _, tc := range []struct {
		name     string
		busBlock string
	}{
		{
			name:     "state only",
			busBlock: `bus { endpoint bgp-rib { internal bgp-rib; receive { allow [ bgp:state ]; } } }`,
		},
		{
			name:     "update only without state",
			busBlock: `bus { endpoint bgp-rib { internal bgp-rib; receive { allow [ bgp:update ]; } } }`,
		},
		{
			name: "sent updates only without received",
			busBlock: `bus { endpoint bgp-rib { internal bgp-rib;
				receive { allow [ bgp:update bgp:state ]; message bgp:update { tag direction [ sent ]; } } } }`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.LoadConfig(fmt.Sprintf(aigpDeliveryConfig, "", tc.busBlock), "", nil)
			require.Error(t, err, "incomplete retained candidate state cannot implement AIGP reselection")
		})
	}

	complete := `bus { endpoint bgp-rib { internal bgp-rib;
		receive { allow [ bgp:update bgp:state ]; message bgp:update { tag direction [ received ]; } }
		send { allow none; } } }`
	_, err := config.LoadConfig(fmt.Sprintf(aigpDeliveryConfig, "", complete), "", nil)
	require.NoError(t, err)
}

// TestAIGPAllowsSeparateProcessRIBMetricFeed verifies that AIGP does not
// require the selecting RIB to run in-daemon (unlike FlowSpec authorization).
//
// VALIDATES: a separate-process bgp-rib endpoint satisfies AIGP selection.
// PREVENTS: over-broad in-daemon refusal rejecting separate-process RIB for AIGP.
func TestAIGPAllowsSeparateProcessRIBMetricFeed(t *testing.T) {
	input := fmt.Sprintf(aigpDeliveryConfig, "", `
bus {
    endpoint routes {
        internal bgp-rib;
        process separate;
        receive { allow [ bgp:update bgp:state ]; message bgp:update { tag direction [ received ]; } }
        send { allow none; }
    }
}`)
	graph := graphFromConfig(t, input)
	for _, peer := range []string{"10.0.0.2", "10.0.0.3"} {
		require.Contains(t, fedBy(graph, bgpevents.EventUpdate, events.DirReceived, peer), "routes")
		require.Contains(t, fedBy(graph, bgpevents.EventState, events.DirUnspecified, peer), "routes")
		require.NotContains(t, fedBy(graph, bgpevents.EventUpdate, events.DirSent, peer), "routes")
		require.NotContains(t, fedBy(graph, bgpevents.EventRefresh, events.DirReceived, peer), "routes")
	}
}

const aigpUntouchedBase = `
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer upstream {
        connection { remote { ip 10.0.0.2; } local { ip 10.0.0.1; } }
        session { asn { local 65000; remote 65001; } }
    }
    %s
    group ix {
        connection {
            remote { ip dynamic; connect false; range 127.0.0.0/8; }
            local { ip 127.0.0.1; accept true; }
        }
        session { asn { local 65000; } }
    }
}`

// TestAddingIBGPPeerLeavesUntouchedPeerDeliveryUnchanged verifies that adding
// an iBGP peer relying on RFC 7311 default AIGP does not alter the compiled
// delivery graph edges of existing static peers or dynamic groups.
//
// VALIDATES: adding an iBGP peer without written AIGP injects no derived
// bindings and keeps untouched peers' compiled delivery identical.
// PREVENTS: reload churn on unrelated sessions when an iBGP peer is added.
func TestAddingIBGPPeerLeavesUntouchedPeerDeliveryUnchanged(t *testing.T) {
	const added = `peer spare {
        connection { remote { ip 10.0.0.3; } local { ip 10.0.0.1; } }
        session { asn { local 65000; remote 65000; } }
    }`

	before := graphFromConfig(t, fmt.Sprintf(aigpUntouchedBase, ""))
	after := graphFromConfig(t, fmt.Sprintf(aigpUntouchedBase, added))

	for _, addr := range []string{"10.0.0.2", "dynamic-group:ix"} {
		addedProcs, removedProcs := before.EdgeDiff(after, addr)
		require.Empty(t, addedProcs, "adding an iBGP peer must not add new delivery edges for %s", addr)
		require.Empty(t, removedProcs, "adding an iBGP peer must not remove delivery edges for %s", addr)
	}
}
