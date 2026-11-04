// Design: docs/plugin-development/protocol.md -- The filter-update batch.
package reactor

import (
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestLibraryCallerActivatedSlotAvoidsRouteTimer measures the full production
// libraryCaller -> Server -> PluginConn -> activated slot route, not the slot
// alone. Startup, route storage, evaluator verdicts and bridge allocation all
// precede AllocsPerRun; the evaluator enforces a peer-AS allow rule.
func TestLibraryCallerActivatedSlotAvoidsRouteTimer(t *testing.T) {
	reactor := New(&Config{RecentUpdateMax: 16})
	server := flowForwardPluginServer(t, reactor)
	process := server.ProcessManager().GetProcess("bgp-rs")
	if process == nil || process.Conn() == nil {
		t.Fatal("running route-server process has no connection")
	}
	accept := rpc.FilterUpdateOutput{Action: rpc.FilterAccept}
	reject := rpc.FilterUpdateOutput{Action: rpc.FilterReject}
	calls := 0
	bridge := rpc.NewDirectBridge()
	bridge.SetFilterSlot(func(route *rpc.FilterRoute) (*rpc.FilterUpdateOutput, error) {
		calls++
		if route.PeerAS() == 65001 {
			return &accept, nil
		}
		return &reject, nil
	})
	bridge.SetReady()
	t.Cleanup(bridge.CloseCallbacks)
	process.Conn().SetBridge(bridge)
	caller := libraryCaller{api: server}
	allowed := rpc.InputFilterRoute(&rpc.FilterUpdateInput{Filter: "allow-peer", PeerAS: 65001})
	denied := rpc.InputFilterRoute(&rpc.FilterUpdateInput{Filter: "allow-peer", PeerAS: 65002})
	out, err := caller.CallFilter("bgp-rs", &denied)
	if err != nil || out == nil || out.Action != rpc.FilterReject {
		t.Fatalf("denied peer verdict=%+v error=%v, want reject", out, err)
	}
	allocs := testing.AllocsPerRun(100, func() {
		out, err := caller.CallFilter("bgp-rs", &allowed)
		if err != nil || out == nil || out.Action != rpc.FilterAccept {
			t.Fatalf("allowed peer verdict=%+v error=%v, want accept", out, err)
		}
	})
	if calls != 102 {
		t.Fatalf("evaluator calls=%d, want denied route plus allocation warmup and 100 measured routes", calls)
	}
	if allocs != 0 {
		t.Fatalf("activated library route allocated %.0f objects per call, want no unused route context/timer", allocs)
	}
}
