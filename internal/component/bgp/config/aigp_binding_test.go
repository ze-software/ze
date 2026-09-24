package bgpconfig

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

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
