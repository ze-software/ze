// Design: docs/architecture/api/process-protocol.md -- engine result failure semantics
package sdk

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// An engine acknowledgement without a result cannot stand for completed route
// validation or a usable metric snapshot. Both operations must fail explicitly.
func TestRouteEngineCallsRejectMissingResults(t *testing.T) {
	for _, result := range []json.RawMessage{nil, json.RawMessage("null")} {
		bridge := rpc.NewDirectBridge()
		bridge.SetDispatchRPC(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			return result, nil
		})
		bridge.SetReady()
		client, engine := net.Pipe()
		p := NewWithConn("missing-result", rpc.NewBridgedConn(client, bridge))
		t.Cleanup(func() {
			if err := p.Close(); err != nil {
				t.Error(err)
			}
			if err := engine.Close(); err != nil {
				t.Error(err)
			}
		})
		if _, err := p.BatchValidate(context.Background(), nil); err == nil {
			t.Fatalf("batch validation accepted missing result %q", result)
		}
		if _, err := p.RouteMetrics(context.Background(), nil); err == nil {
			t.Fatalf("metric snapshot accepted missing result %q", result)
		}
	}
}

// A partial metric vector would leave some routes using an old or zero metric.
// The SDK must reject it while accepting an explicit unresolved result.
func TestRouteMetricsRequiresOneDistancePerAddress(t *testing.T) {
	result := json.RawMessage(`{"revision":1}`)
	bridge := rpc.NewDirectBridge()
	bridge.SetDispatchRPC(func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
		return result, nil
	})
	bridge.SetReady()
	client, engine := net.Pipe()
	p := NewWithConn("metric-result", rpc.NewBridgedConn(client, bridge))
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
		if err := engine.Close(); err != nil {
			t.Error(err)
		}
	})
	addresses := []string{"192.0.2.1"}
	if _, err := p.RouteMetrics(context.Background(), addresses); err == nil {
		t.Fatal("metric snapshot accepted a missing address result")
	}
	result = json.RawMessage(`{"revision":1,"distances":[{"cost":0,"resolved":false,"missing-aigp":false}]}`)
	out, err := p.RouteMetrics(context.Background(), addresses)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Distances) != 1 || out.Distances[0].Resolved {
		t.Fatalf("unresolved next hop lost its explicit result: %+v", out)
	}
}
