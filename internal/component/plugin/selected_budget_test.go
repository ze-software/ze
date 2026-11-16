// Design: docs/architecture/api/process-protocol.md -- selected final-receipt budgets.
package plugin

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestSelectedBatchRejectsOversizedFinalReceipt proves admission reserves the
// final answer before any writer can apply the batch. Method: fill an absence
// request to the request limit, whose required revision echoes make its answer
// larger, and require refusal rather than an unencodable completed write.
func TestSelectedBatchRejectsOversizedFinalReceipt(t *testing.T) {
	input := rpc.SelectRoutesInput{Destination: "192.0.2.7", DestinationSession: 3}
	for range 10 {
		input.Routes = append(input.Routes, rpc.SelectedRoute{Kind: rpc.SelectedRouteAbsent,
			Family: "opaque", KeyNLRI: []byte{1}, Revision: math.MaxUint64})
	}
	size, err := rpc.SelectRoutesFrameSize(input)
	if err != nil {
		t.Fatal(err)
	}
	input.Routes[0].Family += strings.Repeat("x", rpc.MaxMessageSize-size)
	size, err = rpc.SelectRoutesFrameSize(input)
	if err != nil {
		t.Fatal(err)
	}
	if size != rpc.MaxMessageSize {
		t.Fatalf("request boundary = %d, want %d", size, rpc.MaxMessageSize)
	}
	output := rpc.SelectRoutesOutput{}
	for _, route := range input.Routes {
		output.Results = append(output.Results, rpc.SelectedRouteResult{Family: route.Family,
			KeyNLRI: route.KeyNLRI, Revision: route.Revision, CurrentRevision: math.MaxUint64,
			Outcome: rpc.SelectedAbsent})
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		t.Fatal(err)
	}
	answer := rpc.AppendResult(nil, math.MaxUint64, encoded)
	if len(answer) <= rpc.MaxMessageSize {
		t.Fatalf("fixture answer fits: %d", len(answer))
	}
	if _, err := NewSelectedBatch(input); err == nil {
		t.Fatal("admitted request whose required final receipt exceeds the wire limit")
	}
}
