// Design: docs/architecture/api/process-protocol.md -- selected exchange budgets.
package rpc

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// TestSelectedExchangeSizeMatchesWorstCaseWire checks actual JSON escaping,
// a stale frontier wider than the requested revision, and widest correlation.
// Synthetic family text here tests byte accounting, not family admission.
func TestSelectedExchangeSizeMatchesWorstCaseWire(t *testing.T) {
	input := SelectRoutesInput{Destination: "192.0.2.7", DestinationSession: 3,
		Routes: []SelectedRoute{{Kind: SelectedRouteAbsent, Family: "opaque<&\n\u2028",
			KeyNLRI: []byte{1, 2, 3}, Revision: 1}}}
	for range 8 {
		input.Routes = append(input.Routes, input.Routes[0])
	}
	request := input
	request.DeadlineMS = math.MaxInt64
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	want := len(AppendRequest(nil, math.MaxUint64, MethodSelectRoutes, encoded))
	output := SelectRoutesOutput{Results: make([]SelectedRouteResult, len(input.Routes))}
	for i, route := range input.Routes {
		output.Results[i] = SelectedRouteResult{Family: route.Family, KeyNLRI: route.KeyNLRI,
			Revision: route.Revision, CurrentRevision: math.MaxUint64, Outcome: SelectedStaleRevision}
	}
	encoded, err = json.Marshal(output)
	if err != nil {
		t.Fatal(err)
	}
	response := len(AppendResult(nil, math.MaxUint64, encoded))
	if response > want {
		want = response
	}
	got, err := SelectRoutesExchangeSize(input)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("exchange bound=%d, canonical worst-case wire=%d", got, want)
	}
}

// TestSelectedExchangeSizeExactLimit proves one-byte overflow is visible to
// chunking. Family padding is deliberately synthetic byte-counter input.
func TestSelectedExchangeSizeExactLimit(t *testing.T) {
	input := SelectRoutesInput{Destination: "192.0.2.7", DestinationSession: 3,
		Routes: []SelectedRoute{{Kind: SelectedRouteAbsent, Family: "opaque", KeyNLRI: []byte{1}, Revision: 1}}}
	size, err := SelectRoutesExchangeSize(input)
	if err != nil {
		t.Fatal(err)
	}
	input.Routes[0].Family += strings.Repeat("x", MaxMessageSize-size)
	size, err = SelectRoutesExchangeSize(input)
	if err != nil {
		t.Fatal(err)
	}
	if size != MaxMessageSize {
		t.Fatalf("exact limit=%d", size)
	}
	input.Routes[0].Family += "x"
	size, err = SelectRoutesExchangeSize(input)
	if err != nil {
		t.Fatal(err)
	}
	if size != MaxMessageSize+1 {
		t.Fatalf("one-byte overflow=%d", size)
	}
}
