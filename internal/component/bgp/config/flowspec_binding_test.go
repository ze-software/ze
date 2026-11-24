// Design: docs/architecture/api/architecture.md -- FlowSpec bus requirement checks.
// Related: bus_requirements.go -- checkFlowSpecRequirements.
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
}
%s`

// TestFlowSpecFeedsUnicastAuthorizationWithoutFirewall verifies that under the
// default compiled bus, the selecting RIB receives updates and state from both
// the FlowSpec peer and unicast-only peers.
//
// VALIDATES: FlowSpec authorization feeds reach bgp-rib from all peers by default.
// PREVENTS: unicast peers failing to feed the selecting RIB for FlowSpec validation.
func TestFlowSpecFeedsUnicastAuthorizationWithoutFirewall(t *testing.T) {
	graph := graphFromConfig(t, fmt.Sprintf(flowSpecDeliveryConfig, "", ""))
	for _, peer := range []string{"10.0.0.2", "10.0.0.3"} {
		require.Contains(t, fedBy(graph, bgpevents.EventUpdate, events.DirReceived, peer), "bgp-rib")
		require.Contains(t, fedBy(graph, bgpevents.EventState, events.DirUnspecified, peer), "bgp-rib")
	}
}

// TestFlowSpecRefusesIncompleteRIBFeed verifies that cutting received updates
// or peer state from the selecting RIB is refused when FlowSpec is configured.
//
// VALIDATES: AC-14, S34 -- incomplete RIB feeds are refused with a fix command,
// while a complete narrowed feed (bgp:update received + bgp:state) is accepted.
// PREVENTS: a partial RIB authorizing a FlowSpec rule without seeing disqualifying unicast routes.
func TestFlowSpecRefusesIncompleteRIBFeed(t *testing.T) {
	for _, tc := range []struct {
		name     string
		busBlock string
	}{
		{
			name:     "state only",
			busBlock: `bus { endpoint bgp-rib { internal bgp-rib; receive { allow [ bgp:state ]; } } }`,
		},
		{
			name: "sent updates and state without received",
			busBlock: `bus { endpoint bgp-rib { internal bgp-rib;
				receive { allow [ bgp:update bgp:state ]; message bgp:update { tag direction [ sent ]; } } } }`,
		},
		{
			name:     "update only without state",
			busBlock: `bus { endpoint bgp-rib { internal bgp-rib; receive { allow [ bgp:update ]; } } }`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := config.LoadConfig(fmt.Sprintf(flowSpecDeliveryConfig, "", tc.busBlock), "", nil)
			require.Error(t, err, "a partial RIB can authorize a rule without seeing a disqualifying unicast route")
			var oe *opererr.Error
			require.ErrorAs(t, err, &oe)
			require.NotEmpty(t, oe.Fix)
		})
	}

	complete := `bus { endpoint bgp-rib { internal bgp-rib;
		receive { allow [ bgp:update bgp:state ]; message bgp:update { tag direction [ received ]; } }
		send { allow none; } } }`
	_, err := config.LoadConfig(fmt.Sprintf(flowSpecDeliveryConfig, "", complete), "", nil)
	require.NoError(t, err, "an explicit complete authorization feed remains valid")
}

// TestFlowSpecAuthorizationRequiresInternalRIB verifies that a FlowSpec config
// accepts an in-daemon selecting RIB and refuses a separate-process one.
//
// VALIDATES: checkFlowSpecRequirements enforces in-daemon mode for bgp-rib.
// PREVENTS: separate-process RIB deployment breaking synchronous FlowSpec authorization.
func TestFlowSpecAuthorizationRequiresInternalRIB(t *testing.T) {
	for _, tc := range []struct {
		name    string
		process string
		reject  bool
	}{
		{name: "in-daemon process", process: "in-daemon", reject: false},
		{name: "separate process", process: "separate", reject: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			busBlock := fmt.Sprintf(`bus { endpoint routes { internal bgp-rib; process %s; } }`, tc.process)
			_, err := config.LoadConfig(fmt.Sprintf(flowSpecDeliveryConfig, "", busBlock), "", nil)
			if tc.reject {
				require.Error(t, err, "separate-process event delivery cannot supply the synchronous authorization gate")
				var oe *opererr.Error
				require.ErrorAs(t, err, &oe)
				require.Contains(t, err.Error(), "in-daemon")
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestFlowSpecReplayRequiresReachableSelectingRIB verifies that a separate-process
// replay claimant serving a FlowSpec peer is refused, while in-daemon replay
// owners, observers, non-update senders, and replay owners restricted to unicast
// peers remain accepted.
//
// VALIDATES: a replay-capable process on a FlowSpec peer can reach the selecting
// RIB without banning unrelated separate-process unicast forwarders or observers.
// PREVENTS: accepted separate-process replay owners silently losing feasible rules at peer-up.
func TestFlowSpecReplayRequiresReachableSelectingRIB(t *testing.T) {
	for _, tc := range []struct {
		name      string
		role      string
		process   string
		epFilters string
		reject    bool
	}{
		{
			name:    "separate route server on FlowSpec peer",
			role:    "bgp-rs",
			process: "separate",
			reject:  true,
		},
		{
			name:    "separate route reflector on FlowSpec peer",
			role:    "bgp-rr",
			process: "separate",
			reject:  true,
		},
		{
			name:    "separate receive-store replay on FlowSpec peer",
			role:    "bgp-adj-rib-in",
			process: "separate",
			reject:  true,
		},
		{
			name:      "separate route server scoped to unicast peer only",
			role:      "bgp-rs",
			process:   "separate",
			epFilters: `receive { from [ /bgp/peer/unicast-only ]; } send { to [ /bgp/peer/unicast-only ]; }`,
			reject:    false,
		},
		{
			name:      "separate route reflector scoped to unicast peer only",
			role:      "bgp-rr",
			process:   "separate",
			epFilters: `receive { from [ /bgp/peer/unicast-only ]; } send { to [ /bgp/peer/unicast-only ]; }`,
			reject:    false,
		},
		{
			name:      "separate receive-store scoped to unicast peer only",
			role:      "bgp-adj-rib-in",
			process:   "separate",
			epFilters: `receive { from [ /bgp/peer/unicast-only ]; } send { to [ /bgp/peer/unicast-only ]; }`,
			reject:    false,
		},
		{
			name:      "separate observer without send",
			role:      "bgp-rs",
			process:   "separate",
			epFilters: `receive { allow [ bgp:update bgp:state ]; } send { allow none; }`,
			reject:    false,
		},
		{
			name:      "separate non-update sender",
			role:      "bgp-rs",
			process:   "separate",
			epFilters: `receive { allow [ bgp:state ]; } send { allow [ "cli:request peer" ]; }`,
			reject:    false,
		},
		{
			name:      "separate sender without peer state",
			role:      "bgp-adj-rib-in",
			process:   "separate",
			epFilters: `receive { allow [ bgp:update ]; }`,
			reject:    false,
		},
		{
			name:    "in-daemon route reflector on FlowSpec peer",
			role:    "bgp-rr",
			process: "in-daemon",
			reject:  false,
		},
		{
			name:    "in-daemon route server on FlowSpec peer",
			role:    "bgp-rs",
			process: "in-daemon",
			reject:  false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			busBlock := fmt.Sprintf(`bus { endpoint replay-owner { internal %s; process %s; %s } }`,
				tc.role, tc.process, tc.epFilters)
			_, err := config.LoadConfig(fmt.Sprintf(flowSpecDeliveryConfig, "", busBlock), "", nil)
			if tc.reject {
				require.Error(t, err, "the replay owner cannot query the process-local selecting RIB")
				var oe *opererr.Error
				require.ErrorAs(t, err, &oe)
				require.NotEmpty(t, oe.Fix)
				return
			}
			require.NoError(t, err)
		})
	}
}
