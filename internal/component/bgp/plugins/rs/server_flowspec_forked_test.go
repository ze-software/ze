package rs

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestForkedRSReplaysFlowSpecThroughGate proves a route server holding no
// FlowSpec lookup of its own (bgp-rib runs in another process) replays the
// RIB's authorized rules to a peer coming up, by asking through the command
// bgp-rib announced. Method: no lookup is registered here; the enveloped
// bgp-rib/flowspec-gate reaches dispatchText, the gate dispatch is the route
// server's own (as Run registers it), and the engine's dispatch-command-args
// answers the FlowSpec command from canned verdicts. The replay must carry the
// rule at or below the cut, then End-of-RIB. Without the gate it carries none.
func TestForkedRSReplaysFlowSpecThroughGate(t *testing.T) {
	const source, target = "192.0.2.1", "192.0.2.2"
	const gateCommand = "request bgp rib flowspec-gate"
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	key := ribevents.ValidationRoute{Peer: netip.MustParseAddr(source), Family: fam,
		NLRI: string([]byte{5, 1, 24, 10, 0, 0})}
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x80, 14, 11,
		0, 1, 133, 0, 0, 5, 1, 24, 10, 0, 0}
	ribevents.RegisterFlowSpecRedirectLookup(nil)
	ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil)
	t.Cleanup(func() { ribevents.NoteFlowSpecGate(ribevents.FlowSpecGateState{}) })

	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = engineEnd.Close() })
	plugin := sdk.NewWithConn("rs-flowspec-forked-test", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { _ = plugin.Close() })
	var delivered []string
	done := make(chan struct{}, 2)
	bridge.SetDispatchCommandArgs(func(_ context.Context, command string, args []string, _ string) (*rpc.DispatchCommandOutput, error) {
		if command != gateCommand {
			return nil, errors.New("unknown command: " + command)
		}
		var request ribevents.FlowSpecRequest
		if err := json.Unmarshal([]byte(args[0]), &request); err != nil {
			return nil, err
		}
		answer := ribevents.FlowSpecAnswer{Results: make([]ribevents.FlowSpecResult, len(request.Queries))}
		if request.Routes {
			answer.Routes = []ribevents.ValidationRoute{key}
		}
		for i, query := range request.Queries {
			if query.Route != key {
				continue
			}
			switch query.Operation {
			case ribevents.FlowSpecPathOf:
				answer.Results[i] = ribevents.FlowSpecResult{Verdict: true, Path: ribevents.FlowSpecPath{Attributes: attrs, MsgID: 42}}
			case ribevents.FlowSpecEligible, ribevents.FlowSpecPresent:
				answer.Results[i].Verdict = true
			case ribevents.FlowSpecRedirect:
			}
		}
		data, err := json.Marshal(answer)
		if err != nil {
			return nil, err
		}
		return &rpc.DispatchCommandOutput{Status: statusDone, Data: data}, nil
	})
	bridge.SetRelayStoredRoute(func(_ context.Context, destination string, routes []rpc.StoredRoute) error {
		if destination != target || len(routes) != 1 || routes[0].SourcePeer != source ||
			routes[0].NLRIHex != "0501180a0000" || routes[0].Withdraw {
			t.Errorf("wrong FlowSpec replay: %s %+v", destination, routes)
		}
		delivered = append(delivered, "rule-"+strconv.FormatUint(routes[0].MsgID, 10))
		return nil
	})
	bridge.SetDispatchRPC(func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		var input rpc.UpdateRouteInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, err
		}
		if method != rpc.MethodUpdateRoute || input.PeerSelector != target ||
			input.Command != "update text mp "+fam.String()+" eor" {
			t.Errorf("unexpected route RPC %q: %+v", method, input)
		}
		delivered = append(delivered, "eor")
		return json.RawMessage(`{"result":{"announced":0,"withdrawn":0}}`), nil
	})
	bridge.SetDispatchCommand(func(context.Context, string) (*rpc.DispatchCommandOutput, error) {
		delivered = append(delivered, "ready")
		done <- struct{}{}
		return &rpc.DispatchCommandOutput{Status: statusDone}, nil
	})
	bridge.SetReady()
	rs := &routeServer{plugin: plugin, seenMsgID: 42, peers: map[string]*PeerState{
		source: {Address: source, Up: true},
		target: {Address: target, Families: map[family.Family]bool{fam: true}},
	}}
	t.Cleanup(ribevents.RegisterGateDispatch(rs.dispatchCommand))

	payload, err := json.Marshal(ribevents.FlowSpecGateState{Command: gateCommand})
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := json.Marshal(rpc.EventEnvelope{Namespace: ribevents.Namespace, Event: ribevents.EventFlowSpecGate, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	rs.dispatchText(string(envelope))

	rs.handleState(&Event{PeerAddr: target, State: "up", InitialReplay: 17})
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("peer-up replay never completed")
	}
	if !slices.Equal(delivered, []string{"rule-42", "eor", "ready"}) {
		t.Fatalf("forked FlowSpec replay = %v; want the RIB's rule, then End-of-RIB", delivered)
	}
}
