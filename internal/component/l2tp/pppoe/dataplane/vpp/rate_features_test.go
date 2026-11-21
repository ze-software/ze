// Design: docs/architecture/l2tp/bng-5-pppoe.md -- feature fence before policer removal
// VALIDATES: all native feature arcs are reconciled before object release.
package vpp

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"go.fd.io/govpp/api"

	feature "github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/feature"
)

// A two-direction cleanup must confirm all four patched input arcs, the
// native device input arc and both native output arcs off. A repeated fence
// reads their state without issuing another feature decrement.
func TestVPPPolicerFeatureFenceCoversAllNativeArcs(t *testing.T) {
	arcs := []string{"ip4-unicast", "ip4-multicast", "ip6-unicast", "ip6-multicast", "device-input", "ip4-output", "ip6-output"}
	enabled := make(map[string]bool)
	for _, arc := range arcs {
		enabled[arc] = true
	}
	var calls []string
	client := binaryAPI{open: func(ctx context.Context) (api.Stream, error) {
		return &rateFenceStream{policerFeatureStream: &policerFeatureStream{testStream: &testStream{ctx: ctx}, enabled: enabled, calls: &calls}}, nil
	}}
	if err := client.quarantinePolicers(t.Context(), 73, true, true); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, arcs) {
		t.Fatalf("fenced arcs = %v", calls)
	}
	for _, arc := range arcs {
		if enabled[arc] {
			t.Fatalf("arc %s remains enabled", arc)
		}
	}
	calls = nil
	if err := client.quarantinePolicers(t.Context(), 73, true, true); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 0 {
		t.Fatalf("repeated fence stacked decrements: %v", calls)
	}
}

// One failed arc must not prevent other arcs from closing. Its error still
// blocks the owner from unbinding or freeing either policer.
func TestVPPPolicerFeatureFenceAttemptsOtherArcsOnFailure(t *testing.T) {
	arcs := []string{"ip4-unicast", "ip4-multicast", "ip6-unicast", "ip6-multicast", "device-input", "ip4-output", "ip6-output"}
	enabled := make(map[string]bool)
	for _, arc := range arcs {
		enabled[arc] = true
	}
	var calls []string
	client := binaryAPI{open: func(ctx context.Context) (api.Stream, error) {
		return &rateFenceStream{policerFeatureStream: &policerFeatureStream{testStream: &testStream{ctx: ctx}, enabled: enabled, calls: &calls, failArc: "ip4-multicast"}}, nil
	}}
	if err := client.quarantinePolicers(t.Context(), 73, true, true); err == nil {
		t.Fatal("failed arc reported a completed fence")
	}
	if !reflect.DeepEqual(calls, arcs) {
		t.Fatalf("partial fence skipped other arcs: %v", calls)
	}
	if !enabled["ip4-multicast"] {
		t.Fatal("refused arc fixture became disabled")
	}
	if enabled["ip6-output"] {
		t.Fatal("late output arc was never closed")
	}
}

type rateFenceStream struct{ *policerFeatureStream }

func (s *rateFenceStream) SendMsg(message api.Message) error {
	var arc, node string
	var index uint32
	switch request := message.(type) {
	case *feature.FeatureIsEnabled:
		arc, node, index = request.ArcName, request.FeatureName, uint32(request.SwIfIndex)
	case *feature.FeatureEnableDisable:
		arc, node, index = request.ArcName, request.FeatureName, uint32(request.SwIfIndex)
	default:
		return fmt.Errorf("unexpected feature fence request %T", message)
	}
	want := "policer-input"
	if strings.HasSuffix(arc, "-output") {
		want = "policer-output"
	}
	if node != want {
		return fmt.Errorf("node %q on arc %q, want %q", node, arc, want)
	}
	if index != 73 {
		return fmt.Errorf("feature fence targeted foreign interface %d", index)
	}
	return s.policerFeatureStream.SendMsg(message)
}
