package bgpconfig

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
)

const flowSpecDeliveryConfig = `
bgp {
    router-id 10.0.0.1;
    session { asn { local 65000; } }
    peer upstream {
        connection { remote { ip 10.0.0.2; } local { ip 10.0.0.1; } }
        session {
            asn { local 65000; remote 65001; }
            family { ipv4/flow { prefix { maximum 1000; } } }
        }
        %s
    }
    peer unicast-only {
        connection { remote { ip 10.0.0.3; } local { ip 10.0.0.1; } }
        session {
            asn { local 65000; remote 65002; }
            family { ipv4/unicast { prefix { maximum 1000; } } }
        }
    }
}`

func TestFlowSpecFeedsUnicastAuthorizationWithoutFirewall(t *testing.T) {
	graph := graphFromConfig(t, fmt.Sprintf(flowSpecDeliveryConfig, ""))
	for _, peer := range []string{"10.0.0.2", "10.0.0.3"} {
		require.Contains(t, fedBy(graph, bgpevents.EventUpdate, events.DirReceived, peer), "bgp-rib")
		require.Contains(t, fedBy(graph, bgpevents.EventState, events.DirUnspecified, peer), "bgp-rib")
		require.NotContains(t, fedBy(graph, bgpevents.EventUpdate, events.DirSent, peer), "bgp-rib", "authorization must not create sent-route replay state")
		require.NotContains(t, fedBy(graph, bgpevents.EventRefresh, events.DirReceived, peer), "bgp-rib", "authorization alone does not own refresh replay")
	}
}

func TestFlowSpecRefusesIncompleteExplicitRIBFeed(t *testing.T) {
	for _, receive := range []string{"state", "update-sent state", "update-received"} {
		t.Run(receive, func(t *testing.T) {
			_, err := LoadReactor(fmt.Sprintf(flowSpecDeliveryConfig, "attach process bgp-rib { receive [ "+receive+" ]; }"))
			require.Error(t, err, "a partial RIB can authorize a rule without seeing a disqualifying unicast route")
		})
	}
	_, err := LoadReactor(fmt.Sprintf(flowSpecDeliveryConfig, "attach process bgp-rib { receive [ update-received state ]; }"))
	require.NoError(t, err, "an explicit complete authorization feed remains valid")
}

func TestFlowSpecAuthorizationRequiresInternalRIB(t *testing.T) {
	for _, tc := range []struct {
		name, declaration string
		external          bool
	}{
		{"internal process", "internal routes { use bgp-rib; }", false},
		{"external stanza using an internal process", "external routes { use bgp-rib; }", false},
		{"external subprocess", `external routes { run "ze plugin bgp-rib"; }`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := fmt.Sprintf(flowSpecDeliveryConfig, "") + "plugin { " + tc.declaration + " }"
			schema, err := config.YANGSchema()
			require.NoError(t, err)
			_, err = config.NewParser(schema).Parse(input)
			require.NoError(t, err, "both plugin modes are valid configuration syntax")
			_, err = LoadReactor(input)
			if tc.external {
				require.Error(t, err, "external event delivery cannot supply the synchronous authorization gate")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// VALIDATES: a replay-capable process on a FlowSpec peer can reach the selecting
// RIB, without banning unrelated external unicast forwarders or observers.
// PREVENTS: accepted external replay owners silently losing feasible rules at peer-up.
func TestFlowSpecReplayRequiresReachableSelectingRIB(t *testing.T) {
	const replay = "attach process replay-owner { receive [ update-received state ]; send [ update ]; }"
	for _, tc := range []struct {
		name, role, kind, flowBinding, unicastBinding string
		reject                                        bool
		use                                           bool
	}{
		{name: "external route server", role: "bgp-rs", kind: "external", flowBinding: replay, reject: true},
		{name: "external route reflector", role: "bgp-rr", kind: "external", flowBinding: replay, reject: true},
		{name: "external receive-store replay", role: "bgp-adj-rib-in", kind: "external", flowBinding: replay, reject: true},
		{name: "external unicast route server", role: "bgp-rs", kind: "external", unicastBinding: replay},
		{name: "external unicast route reflector", role: "bgp-rr", kind: "external", unicastBinding: replay},
		{name: "external unicast receive store", role: "bgp-adj-rib-in", kind: "external", unicastBinding: replay},
		{name: "external observer", role: "bgp-rs", kind: "external",
			flowBinding: "attach process replay-owner { receive [ update-received state ]; }"},
		{name: "external raw-only sender", role: "bgp-rs", kind: "external",
			flowBinding: "attach process replay-owner { receive [ state ]; send [ raw ]; }"},
		{name: "external sender without peer state", role: "bgp-adj-rib-in", kind: "external",
			flowBinding: "attach process replay-owner { receive [ update-received ]; send [ update ]; }"},
		{name: "internal replay owner", role: "bgp-rr", kind: "internal", flowBinding: replay},
		{name: "external stanza using an internal owner", role: "bgp-rs", kind: "external", flowBinding: replay, use: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := fmt.Sprintf(flowSpecDeliveryConfig, tc.flowBinding)
			input = strings.Replace(input, "peer unicast-only {", "peer unicast-only {\n"+tc.unicastBinding, 1)
			execution := fmt.Sprintf("run %q;", "ze plugin "+tc.role)
			if tc.kind == "internal" || tc.use {
				execution = "use " + tc.role + ";"
			}
			input += fmt.Sprintf("\nplugin { %s replay-owner { %s } }", tc.kind, execution)
			schema, err := config.YANGSchema()
			require.NoError(t, err)
			_, err = config.NewParser(schema).Parse(input)
			require.NoError(t, err, "the process mode and bindings are valid syntax")
			_, err = LoadReactor(input)
			if tc.reject {
				require.Error(t, err, "the replay owner cannot query the process-local selecting RIB")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
