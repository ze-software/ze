// Design: docs/architecture/api/update-syntax.md -- the replay-eor meta on an End-of-RIB
// Related: server_handlers.go, rr.go -- sendEOR

package rs

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestReplayEORCarriesReplayMeta pins that the End-of-RIB closing this
// plugin's replay reaches the engine marked with the replay-eor meta.
//
// VALIDATES: every replay End-of-RIB update-route RPC carries
// meta[replay-eor] == true after the JSON round trip an external plugin's
// call takes, so the engine picks AnnounceReplayEOR.
// PREVENTS: the replay close reaching AnnounceEOR, which sends every time, and
// so repeating the marker the reactor's initial sync already sent (RFC 4724
// Section 2).
func TestReplayEORCarriesReplayMeta(t *testing.T) {
	const target = "192.0.2.2"
	bridge := rpc.NewDirectBridge()
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() { _ = engineEnd.Close() })
	p := sdk.NewWithConn("rs-eor-test", rpc.NewBridgedConn(pluginEnd, bridge))
	t.Cleanup(func() { _ = p.Close() })
	var inputs []rpc.UpdateRouteInput
	bridge.SetDispatchRPC(func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
		if method != rpc.MethodUpdateRoute {
			return json.RawMessage(`{}`), nil
		}
		var input rpc.UpdateRouteInput
		if err := json.Unmarshal(params, &input); err != nil {
			return nil, err
		}
		inputs = append(inputs, input)
		return json.RawMessage(`{"announced":0,"withdrawn":0}`), nil
	})
	bridge.SetReady()
	rs := &routeServer{plugin: p, peers: map[string]*PeerState{
		target: {Address: target, ReplayGen: 3, Families: map[family.Family]bool{family.IPv4Unicast: true, family.IPv6Unicast: true}},
	}}
	rs.sendEOR(target, 3, 0)

	if len(inputs) != 2 {
		t.Fatalf("want one End-of-RIB per family, got %d: %+v", len(inputs), inputs)
	}
	for _, input := range inputs {
		replayEOR, ok := input.Meta[bgptypes.ReplayEORMeta].(bool)
		if !ok || !replayEOR {
			t.Errorf("%q to %s carries meta %v, want %s=true", input.Command, input.PeerSelector, input.Meta, bgptypes.ReplayEORMeta)
		}
	}
}
