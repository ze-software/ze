// Design: docs/architecture/api/process-protocol.md -- accepted loser results.
package ls_export

import (
	"context"
	"errors"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type retainedExportDelivery struct {
	output *rpc.UpdateRouteOutput
	cause  error
}

func (d *retainedExportDelivery) UpdateRouteWithMetaResult(context.Context, string, string, map[string]any) (*rpc.UpdateRouteOutput, error) {
	return d.output, d.cause
}

// A retained native route is accepted with zero announcement count. Actual
// failures and a zero result without a receipt remain owed advertisement work.
func TestNativeTopologyRetainedResultIsAccepted(t *testing.T) {
	retained := rpc.RouteAdmission{Family: "bgp-ls/bgp-ls", KeyNLRI: []byte{0, 1}, Destination: "192.0.2.7", PeerOwner: 17,
		Generation: 3, Outcome: rpc.RouteAdmissionRetained, Disposition: rpc.RouteAdmissionSuppressedSelection}
	for _, test := range []struct {
		name   string
		output *rpc.UpdateRouteOutput
		cause  error
		fail   bool
	}{
		{name: "retained", output: &rpc.UpdateRouteOutput{Admissions: []rpc.RouteAdmission{retained}}},
		{name: "direct", output: &rpc.UpdateRouteOutput{Announced: 1}},
		{name: "zero", output: &rpc.UpdateRouteOutput{}, fail: true},
		{name: "missing", fail: true},
		{name: "transport", cause: errors.New("collector transport unavailable"), fail: true},
		{name: "command", output: &rpc.UpdateRouteOutput{Failure: "origin admission denied"}, fail: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			exporter := newTopologyExporter(&retainedExportDelivery{output: test.output, cause: test.cause})
			err := exporter.send(t.Context(), "192.0.2.7", exportedRoute{nlri: []byte{0, 1}}, false, false)
			if (err != nil) != test.fail {
				t.Fatalf("accepted/failure result = %v", err)
			}
			if test.cause != nil {
				if !errors.Is(err, test.cause) {
					t.Fatalf("transport cause lost: %v", err)
				}
			}
		})
	}
}
