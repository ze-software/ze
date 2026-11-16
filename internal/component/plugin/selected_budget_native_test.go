// Design: docs/architecture/api/process-protocol.md -- selected final-receipt budgets.
package plugin

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSelectedBatchNativeAbsenceReceiptBudget uses unique, valid IPv4 /32
// keys to prove the answer expansion is reachable without synthetic family
// padding. Method: derive the constant-width request count once, fill that
// bounded batch, and refuse it before any writer admission.
func TestSelectedBatchNativeAbsenceReceiptBudget(t *testing.T) {
	input := rpc.SelectRoutesInput{Destination: "192.0.2.7", DestinationSession: 3}
	base, err := rpc.SelectRoutesFrameSize(input)
	if err != nil {
		t.Fatal(err)
	}
	input.Routes = []rpc.SelectedRoute{{Kind: rpc.SelectedRouteAbsent, Family: "ipv4/unicast",
		KeyNLRI: []byte{32, 0, 0, 0, 0}, Revision: math.MaxUint64}}
	one, err := rpc.SelectRoutesFrameSize(input)
	if err != nil {
		t.Fatal(err)
	}
	// A nonnil empty list writes [] instead of null. Every additional route
	// has the same scalar/base64 widths and one comma; no JSON growth loop.
	input.Routes = []rpc.SelectedRoute{}
	empty, err := rpc.SelectRoutesFrameSize(input)
	if err != nil {
		t.Fatal(err)
	}
	if base <= empty {
		t.Fatal("fixture no longer distinguishes null and empty routes")
	}
	width := one - empty
	count := (rpc.MaxMessageSize - empty + 1) / (width + 1)
	input.Routes = make([]rpc.SelectedRoute, count)
	for i := range input.Routes {
		input.Routes[i] = rpc.SelectedRoute{Kind: rpc.SelectedRouteAbsent, Family: "ipv4/unicast",
			KeyNLRI:  []byte{32, 10, byte(i >> 16), byte(i >> 8), byte(i)},
			Revision: math.MaxUint64}
	}
	request, err := rpc.SelectRoutesFrameSize(input)
	if err != nil {
		t.Fatal(err)
	}
	if request > rpc.MaxMessageSize {
		t.Fatalf("native fixture request exceeds limit: %d", request)
	}
	results := make([]rpc.SelectedRouteResult, len(input.Routes))
	for i, route := range input.Routes {
		results[i] = rpc.SelectedRouteResult{Family: route.Family, KeyNLRI: route.KeyNLRI,
			Revision: route.Revision, CurrentRevision: math.MaxUint64, Outcome: rpc.SelectedAbsent}
	}
	encoded, err := json.Marshal(rpc.SelectRoutesOutput{Results: results})
	if err != nil {
		t.Fatal(err)
	}
	if len(rpc.AppendResult(nil, math.MaxUint64, encoded)) <= rpc.MaxMessageSize {
		t.Fatal("native fixture final receipt unexpectedly fits")
	}
	if _, err := NewSelectedBatch(input); err == nil {
		t.Fatal("native absence batch with oversized receipt was admitted")
	}
}
