// Design: docs/plugin-development/protocol.md -- selected-route final receipts.
package sdk

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Full selected receipts survive both SDK adapters; missing results never turn
// an admitted RPC into a claim that the destination writer completed.
func TestSelectRoutesSDKResultParity(t *testing.T) {
	input := rpc.SelectRoutesInput{Destination: "192.0.2.7", DestinationSession: 3,
		Routes: []rpc.SelectedRoute{{Kind: rpc.SelectedRouteAbsent, Family: "ipv4/unicast",
			KeyNLRI: []byte{24, 10, 0, 0}, Revision: 7}}}
	for _, typed := range []bool{false, true} {
		name := "json"
		if typed {
			name = "typed"
		}
		t.Run(name, func(t *testing.T) {
			bridge := rpc.NewDirectBridge()
			output := &rpc.SelectRoutesOutput{Results: []rpc.SelectedRouteResult{{
				Family: "ipv4/unicast", KeyNLRI: []byte{24, 10, 0, 0}, Revision: 7,
				CurrentRevision: 7, Outcome: rpc.SelectedAbsent}}}
			bridge.SetDispatchRPC(func(_ context.Context, method string, params json.RawMessage) (json.RawMessage, error) {
				if method != rpc.MethodSelectRoutes {
					t.Fatalf("SDK selected method = %s", method)
				}
				var got rpc.SelectRoutesInput
				if err := json.Unmarshal(params, &got); err != nil {
					return nil, err
				}
				if got.DestinationSession != 3 || got.Routes[0].Revision != 7 {
					t.Fatalf("SDK changed selected input: %+v", got)
				}
				return json.Marshal(output)
			})
			if typed {
				bridge.SetSelectRoutes(func(context.Context, rpc.SelectRoutesInput) (*rpc.SelectRoutesOutput, error) {
					return output, nil
				})
			}
			bridge.SetReady()
			client, engine := net.Pipe()
			p := NewWithConn("selected-test", rpc.NewBridgedConn(client, bridge))
			t.Cleanup(func() {
				if err := p.Close(); err != nil {
					t.Error(err)
				}
				if err := engine.Close(); err != nil {
					t.Error(err)
				}
			})
			got, err := p.SelectRoutes(t.Context(), input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Results[0].Outcome != rpc.SelectedAbsent {
				t.Fatalf("SDK lost final absence receipt: %+v", got)
			}
			output = nil
			if _, err := p.SelectRoutes(t.Context(), input); err == nil {
				t.Fatal("SDK accepted a missing final selected result")
			}
		})
	}
}
