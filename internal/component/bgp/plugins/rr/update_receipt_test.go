// Design: docs/architecture/api/process-protocol.md -- source-bound UPDATE receipts.
package rr

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"slices"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRRUpdateReceiptDispatch reaches the reflector's real text dispatcher and
// SDK engine calls. Matching receipts retain reflection and DOWN inventory;
// mismatches release the payload's cache entry without forwarding it. Missing
// or null mandatory members cannot invent an accepted forwarding receipt.
func TestRRUpdateReceiptDispatch(t *testing.T) {
	type receiptCase struct {
		name  string
		field string
		mode  string
	}
	cases := []receiptCase{{name: "matching"}}
	for _, field := range []string{"peer", "message-id", "source-policy-epoch", "reactor-forwarded"} {
		cases = append(cases,
			receiptCase{field + "-missing", field, "missing"},
			receiptCase{field + "-null", field, "null"})
		if field != "reactor-forwarded" {
			cases = append(cases, receiptCase{field + "-mismatch", field, "mismatch"})
		}
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			bridge := rpc.NewDirectBridge()
			pluginEnd, engineEnd := net.Pipe()
			t.Cleanup(func() { _ = engineEnd.Close() })
			p := sdk.NewWithConn("rr-receipt-test", rpc.NewBridgedConn(pluginEnd, bridge))
			t.Cleanup(func() { _ = p.Close() })
			var forwarded []string
			var released []uint64
			// RR uses UpdateRouteWithMeta, which reaches the generic engine RPC
			// dispatcher; SetUpdateRouteSel observes a different SDK entry point.
			bridge.SetDispatchRPC(func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
				if method != rpc.MethodUpdateRoute {
					return nil, fmt.Errorf("unexpected engine method %q", method)
				}
				var input rpc.UpdateRouteInput
				if err := json.Unmarshal(params, &input); err != nil {
					return nil, err
				}
				if input.PeerSelector != "*" {
					t.Errorf("reflection selector = %q, want all peers", input.PeerSelector)
				}
				forwarded = append(forwarded, input.Command)
				return json.Marshal(rpc.UpdateRouteOutput{Announced: 1})
			})
			bridge.SetReleaseCached(func(_ context.Context, ids []uint64) error {
				released = append(released, ids...)
				return nil
			})
			bridge.SetReady()
			rr := &routeReflector{plugin: p, peers: make(map[string]*peerState), withdrawals: make(map[string]map[string]withdrawalInfo)}
			const source = "192.0.2.1"
			const payload = "peer 192.0.2.1 remote as 65001 received update 40 source-policy-epoch 7 origin igp next 192.0.2.1 nlri ipv4/unicast add 10.4.0.0/24\n"
			metadata := map[string]any{"peer": source, "message-id": uint64(40), "source-policy-epoch": uint64(7), "reactor-forwarded": false}
			switch test.mode {
			case "missing":
				delete(metadata, test.field)
			case "null":
				metadata[test.field] = nil
			case "mismatch":
				if test.field == "peer" {
					metadata[test.field] = "192.0.2.99"
				} else {
					metadata[test.field] = uint64(99)
				}
			}
			encoded, err := json.Marshal(map[string]any{"namespace": "bgp", "event": "update", "payload": payload, "metadata": metadata})
			if err != nil {
				t.Fatal(err)
			}
			rr.dispatchText(string(encoded))
			if test.mode == "" {
				if !slices.Equal(forwarded, []string{"cached 40"}) || len(released) != 0 {
					t.Fatalf("matching receipt forwarded=%v released=%v, want one cached reflection", forwarded, released)
				}
				if len(rr.withdrawals[source]) != 1 {
					t.Fatalf("matching receipt inventory = %v, want its source announcement", rr.withdrawals[source])
				}
				for _, route := range rr.withdrawals[source] {
					if route.SourcePolicyEpoch != 7 {
						t.Fatalf("inventory epoch = %d, want 7", route.SourcePolicyEpoch)
					}
				}
				return
			}
			if len(forwarded) != 0 || len(rr.withdrawals) != 0 {
				t.Fatalf("refused receipt forwarded=%v inventory=%v", forwarded, rr.withdrawals)
			}
			if test.mode == "mismatch" {
				if !slices.Equal(released, []uint64{40}) {
					t.Fatalf("mismatched receipt released=%v, want payload ID 40", released)
				}
			} else if len(released) != 0 {
				t.Fatalf("unparsed receipt invented cache release %v", released)
			}
		})
	}
}
