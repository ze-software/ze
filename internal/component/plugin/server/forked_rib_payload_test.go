package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"reflect"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// forkedBGPRouteChange is the handle the BGP redistribute producer registers
// (internal/component/bgp/redistribute/producer.go RouteChange): the same
// namespace, event type and payload type, so the engine decodes the forked
// emit exactly as it does in the daemon.
var forkedBGPRouteChange = events.Register[*redistevents.RouteChangeBatch]("bgp", redistevents.EventType)

// errOneGateArgument refuses a gate command that did not arrive as the one
// JSON argument its caller marshaled. The handler runs on the target's reader
// goroutine, so it answers an error rather than failing the test there.
var errOneGateArgument = errors.New("gate command arrived without exactly one JSON argument")

// forkedRIBFlowSpecFamily registers ipv4/flow, which in ze the FlowSpec plugin
// registers; this test binary links no such plugin.
func forkedRIBFlowSpecFamily(t *testing.T) family.Family {
	t.Helper()
	fam, err := family.RegisterFamily(family.AFIIPv4, family.SAFIFlowSpec, "ipv4", "flow")
	require.NoError(t, err)
	return fam
}

// TestForkedRIBEventsDecodeForEngineSubscribers proves every event a forked
// bgp-rib emits through its SDK bus (rib/rib_event_sdk.go sdkEventBus) reaches
// an engine-side subscriber as the Go value the RIB emitted. Method: each
// value is marshaled as sdkEventBus marshals it, carried as the emit-event
// RPC's JSON string (rpc.EmitEventInput.Event), handed to emitEvent, and the
// value the engine subscriber receives is compared field by field with the
// original. NLRI bytes at and above 0x80 are included, which a JSON string
// would corrupt without the hex encoding ValidationRoute carries.
func TestForkedRIBEventsDecodeForEngineSubscribers(t *testing.T) {
	flow := forkedRIBFlowSpecFamily(t)
	peer := netip.MustParseAddr("192.0.2.1")
	flowNLRI := string([]byte{7, 1, 24, 10, 0, 0, 0x81, 0xff})
	routes := []ribevents.ValidationRoute{
		{Peer: peer, Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("10.0.0.0/24"), PathID: 3},
		{Peer: peer, Family: flow, NLRI: flowNLRI},
	}
	cases := []struct {
		name      string
		namespace string
		eventType string
		value     any
	}{
		{"validation-gate", ribevents.Namespace, ribevents.EventValidationGate,
			ribevents.ValidationGateState{Enabled: true, Command: "request bgp rib eligibility"}},
		{"flowspec-gate", ribevents.Namespace, ribevents.EventFlowSpecGate,
			ribevents.FlowSpecGateState{Command: "request bgp rib flowspec-gate"}},
		{"gate-query", ribevents.Namespace, ribevents.EventGateQuery, ribevents.GateQueryRequest{}},
		{"validation-change", ribevents.Namespace, ribevents.EventValidationChange, routes},
		{"flowspec-change", ribevents.FlowSpecChanged.Namespace(), ribevents.FlowSpecChanged.EventType(),
			&ribevents.FlowSpecChange{Family: flow, NLRI: []byte(flowNLRI),
				ExtendedCommunities: []byte{0x80, 0x06, 0, 0, 0, 0, 0, 0}, Withdraw: true}},
		{"best-change", ribevents.Namespace, ribevents.EventBestChange,
			&ribevents.BestChangeBatch{Protocol: "bgp", Family: flow, ReplayID: replay.Broadcast,
				Changes: []ribevents.BestChangeEntry{{Action: routeaction.Add, NLRI: []byte(flowNLRI),
					NextHop: netip.MustParseAddr("192.0.2.9"), Priority: 20, Metric: 5, ASPath: []uint32{65001, 65002}}}}},
		{"route-change", forkedBGPRouteChange.Namespace(), forkedBGPRouteChange.EventType(),
			&redistevents.RouteChangeBatch{Protocol: 7, AFI: uint16(family.AFIIPv4), SAFI: uint8(family.SAFIUnicast),
				ReplayID: 11, OriginASN: 65010, Community: []uint32{65010<<16 | 1},
				Entries: []redistevents.RouteChangeEntry{{Action: redistevents.ActionAdd,
					Prefix: netip.MustParsePrefix("10.1.0.0/16"), NextHop: peer, Metric: 9, OriginAS: 65020}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Server{engineSubscribers: newEngineEventSubscribers()}
			var received []any
			unsubscribe := s.SubscribeEngineEvent(tc.namespace, tc.eventType, func(payload any) {
				received = append(received, payload)
			})
			defer unsubscribe()

			event, err := json.Marshal(tc.value)
			require.NoError(t, err)
			params, err := json.Marshal(rpc.EmitEventInput{Namespace: tc.namespace, EventType: tc.eventType, Event: string(event)})
			require.NoError(t, err)
			_, err = s.emitEvent(nil, params)
			require.NoError(t, err)

			require.Len(t, received, 1)
			if !reflect.DeepEqual(received[0], tc.value) {
				t.Fatalf("engine subscriber received %T %#v; want %#v", received[0], received[0], tc.value)
			}
		})
	}
}

// TestForkedRIBGateCommandsCrossTheServer proves the gate commands a forked
// bgp-rib serves answer a caller in another process through the server's
// dispatch-command-args: the request reaches the RIB as the one JSON argument
// the caller marshaled, and the answer's verdicts, path attributes and NLRI
// keys decode on the caller's side unchanged. Method: the RIB end is a
// plugin connection answering execute-command (registerExecuteCommandTarget)
// with ribevents.AnswerEligibility over a fixed lookup, and with a FlowSpec
// answer built from canned results; the caller end is ribevents' own remote
// client (NoteValidationGate, NoteFlowSpecGate) routed through
// s.dispatchCommandArgs by RegisterGateDispatch.
func TestForkedRIBGateCommandsCrossTheServer(t *testing.T) {
	const eligibilityCommand = "request bgp rib eligibility"
	const flowSpecCommand = "request bgp rib flowspec-gate"
	flow := forkedRIBFlowSpecFamily(t)
	peer := netip.MustParseAddr("192.0.2.1")
	accepted := ribevents.ValidationRoute{Peer: peer, Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("10.0.0.0/24")}
	refused := ribevents.ValidationRoute{Peer: peer, Family: family.IPv4Unicast, Prefix: netip.MustParsePrefix("10.0.1.0/24")}
	rule := ribevents.ValidationRoute{Peer: peer, Family: flow, NLRI: string([]byte{7, 1, 24, 10, 0, 0, 0x81, 0xff})}
	attributes := []byte{0x40, 1, 1, 0, 0xc0, 16, 8, 0x80, 0x06, 0, 0, 0, 0, 0, 0}

	d := NewDispatcher()
	eligibilityDone := registerExecuteCommandTarget(t, d, eligibilityCommand, 1, func(_ int, input *rpc.ExecuteCommandInput) (*rpc.ExecuteCommandOutput, error) {
		if len(input.Args) != 1 {
			return nil, errOneGateArgument
		}
		answer, err := ribevents.AnswerEligibility(input.Args[0], func(route ribevents.ValidationRoute, _ uint64) bool {
			return route == accepted
		}, func(route ribevents.ValidationRoute) bool { return route == accepted })
		if err != nil {
			return nil, err
		}
		data, err := json.Marshal(answer)
		return &rpc.ExecuteCommandOutput{Status: plugin.StatusDone, Data: data}, err
	})
	flowSpecDone := registerExecuteCommandTarget(t, d, flowSpecCommand, 2, func(_ int, input *rpc.ExecuteCommandInput) (*rpc.ExecuteCommandOutput, error) {
		if len(input.Args) != 1 {
			return nil, errOneGateArgument
		}
		var request ribevents.FlowSpecRequest
		if err := json.Unmarshal([]byte(input.Args[0]), &request); err != nil {
			return nil, err
		}
		answer := ribevents.FlowSpecAnswer{Results: make([]ribevents.FlowSpecResult, len(request.Queries))}
		if request.Routes {
			answer.Routes = []ribevents.ValidationRoute{rule}
		}
		for i, query := range request.Queries {
			if query.Route == rule && query.Operation == ribevents.FlowSpecPathOf {
				answer.Results[i] = ribevents.FlowSpecResult{Verdict: true,
					Path: ribevents.FlowSpecPath{Attributes: attributes, MsgID: 42, SourcePolicyEpoch: 6}}
			}
		}
		data, err := json.Marshal(answer)
		return &rpc.ExecuteCommandOutput{Status: plugin.StatusDone, Data: data}, err
	})

	s := &Server{dispatcher: d}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	defer s.cancel()
	caller := process.NewProcess(plugin.PluginConfig{Name: "bgp-rr"})
	release := ribevents.RegisterGateDispatch(func(ctx context.Context, command string, args ...string) (string, json.RawMessage, error) {
		out, err := s.dispatchCommandArgs(ctx, caller, command, args, "")
		if err != nil {
			return "", nil, err
		}
		return out.Status, out.Data, nil
	})
	t.Cleanup(release)
	ribevents.RegisterValidationLookup(nil, nil)
	ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil)
	ribevents.NoteValidationGate(ribevents.ValidationGateState{Enabled: true, Command: eligibilityCommand})
	ribevents.NoteFlowSpecGate(ribevents.FlowSpecGateState{Command: flowSpecCommand})
	t.Cleanup(func() {
		ribevents.NoteValidationGate(ribevents.ValidationGateState{})
		ribevents.NoteFlowSpecGate(ribevents.FlowSpecGateState{})
	})

	answers := make([]bool, 2)
	ribevents.RouteEligibleBatch([]ribevents.EligibilityQuery{{Route: accepted, MsgID: 1}, {Route: refused, MsgID: 2}}, answers)
	if !slices.Equal(answers, []bool{true, false}) {
		t.Fatalf("eligibility verdicts across the server = %v; want [true false]", answers)
	}
	require.NoError(t, <-eligibilityDone)

	path, found := ribevents.LookupFlowSpecPath(rule)
	if !found || !slices.Equal(path.Attributes, attributes) || path.MsgID != 42 || path.SourcePolicyEpoch != 6 {
		t.Fatalf("FlowSpec path across the server = %+v found=%v; want the RIB's attributes, msg-id 42, epoch 6", path, found)
	}
	if got := ribevents.FlowSpecRoutes(); !slices.Equal(got, []ribevents.ValidationRoute{rule}) {
		t.Fatalf("FlowSpec routes across the server = %+v; want the RIB's rule with its NLRI bytes", got)
	}
	require.NoError(t, <-flowSpecDone)
}
