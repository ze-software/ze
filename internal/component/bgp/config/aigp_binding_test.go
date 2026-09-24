package bgpconfig

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
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
}`

func TestAIGPFeedsSelectionWithoutForwardingRole(t *testing.T) {
	input := fmt.Sprintf(aigpDeliveryConfig, "")
	graph := graphFromConfig(t, input)
	for _, peer := range []string{"10.0.0.2", "10.0.0.3"} {
		consumers := fedBy(graph, bgpevents.EventUpdate, events.DirReceived, peer)
		require.Contains(t, consumers, "bgp-rib", "selection must also see competing routes from non-AIGP peers")
		require.NotContains(t, consumers, "bgp-adj-rib-in", "AIGP selection does not enable a replay role")
		require.Contains(t, fedBy(graph, bgpevents.EventState, events.DirUnspecified, peer), "bgp-rib")
		require.NotContains(t, fedBy(graph, bgpevents.EventUpdate, events.DirSent, peer), "bgp-rib", "mandatory selection must not populate replay state from other exporters")
		require.NotContains(t, fedBy(graph, bgpevents.EventRefresh, events.DirReceived, peer), "bgp-rib", "mandatory selection does not answer route refresh")
	}
	require.False(t, bindingNamed(t, bindingsFor(t, input), "bgp-rib").MaySend("update"), "the mandatory metric consumer gains no route-export authority")
}

// Selection must not narrow a wider grant requested by redistribution or an
// operator's explicit exporter binding.
func TestAIGPSelectionPreservesRequestedRIBDelivery(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		send        bool
	}{
		{
			name: "redistribution",
			input: fmt.Sprintf(aigpDeliveryConfig, "") + `
ospf { router-id 10.0.0.1; }
redistribute { destination ospf { import bgp; } }`,
		},
		{
			name: "explicit exporter",
			input: fmt.Sprintf(aigpDeliveryConfig,
				"attach process bgp-rib { receive [ update state refresh ]; send [ update ]; }"),
			send: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			graph := graphFromConfig(t, tc.input)
			require.Contains(t, fedBy(graph, bgpevents.EventUpdate, events.DirReceived, "10.0.0.2"), "bgp-rib")
			require.Contains(t, fedBy(graph, bgpevents.EventUpdate, events.DirSent, "10.0.0.2"), "bgp-rib")
			require.Contains(t, fedBy(graph, bgpevents.EventState, events.DirUnspecified, "10.0.0.2"), "bgp-rib")
			require.Contains(t, fedBy(graph, bgpevents.EventRefresh, events.DirReceived, "10.0.0.2"), "bgp-rib")
			require.Equal(t, tc.send, bindingNamed(t, bindingsFor(t, tc.input), "bgp-rib").MaySend("update"))
		})
	}
}

func TestAIGPRefusesIncompleteExplicitRIBFeed(t *testing.T) {
	for _, receive := range []string{"state", "update-received"} {
		_, err := LoadReactor(fmt.Sprintf(aigpDeliveryConfig, "attach process bgp-rib { receive [ "+receive+" ]; }"))
		require.Error(t, err, "incomplete retained candidate state cannot implement AIGP reselection")
	}
	_, err := LoadReactor(fmt.Sprintf(aigpDeliveryConfig, "attach process bgp-rib { receive [ update-received state ]; }"))
	require.NoError(t, err)
}

func TestAIGPAllowsExternalRIBMetricFeed(t *testing.T) {
	input := fmt.Sprintf(aigpDeliveryConfig, "") +
		`plugin { external routes { run "ze plugin bgp-rib"; } }`
	graph := graphFromConfig(t, input)
	for _, peer := range []string{"10.0.0.2", "10.0.0.3"} {
		require.Contains(t, fedBy(graph, bgpevents.EventUpdate, events.DirReceived, peer), "routes")
		require.Contains(t, fedBy(graph, bgpevents.EventState, events.DirUnspecified, peer), "routes")
		require.NotContains(t, fedBy(graph, bgpevents.EventUpdate, events.DirSent, peer), "routes")
		require.NotContains(t, fedBy(graph, bgpevents.EventRefresh, events.DirReceived, peer), "routes")
	}
}

// aigpUntouchedBase is an eBGP peer and a dynamic group, neither of which
// enables AIGP. %s is where the edit adds a peer.
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

// TestAddingIBGPPeerLeavesOtherBindingsUnchanged holds the reload invariant
// for derived delivery: an edit never changes the derived settings of a peer
// it did not touch.
//
// VALIDATES: adding one iBGP peer, which enables AIGP by the RFC 7311 Section
// 3.3 default, leaves the ProcessBindings of the existing static peer and of
// the dynamic group template byte-for-byte what they were.
// PREVENTS: the reload restarting every established session. ProcessBindings
// is not hot-swappable, so a derived grant that follows another peer's AIGP
// state bounces every session (reload-dynamic-peer-survives.ci).
func TestAddingIBGPPeerLeavesOtherBindingsUnchanged(t *testing.T) {
	const added = `peer spare {
        connection { remote { ip 10.0.0.3; } local { ip 10.0.0.1; } }
        session { asn { local 65000; remote 65000; } }
    }`
	build := func(input string) (static, dynamic []reactor.ProcessBinding) {
		t.Helper()
		schema, err := config.YANGSchema()
		require.NoError(t, err)
		tree, err := config.NewParser(schema).Parse(input)
		require.NoError(t, err)
		peers, groups, err := peersAndDynamicGroups(tree)
		require.NoError(t, err)
		require.Len(t, groups, 1)
		for _, ps := range peers {
			if ps.Name == "upstream" {
				return ps.ProcessBindings, groups[0].Settings.ProcessBindings
			}
		}
		t.Fatal("peer upstream not built from the config")
		return nil, nil
	}

	staticBefore, dynamicBefore := build(fmt.Sprintf(aigpUntouchedBase, ""))
	staticAfter, dynamicAfter := build(fmt.Sprintf(aigpUntouchedBase, added))
	require.Equal(t, staticBefore, staticAfter, "adding an iBGP peer changed an untouched static peer's bindings")
	require.Equal(t, dynamicBefore, dynamicAfter, "adding an iBGP peer changed the dynamic group template's bindings")
}
