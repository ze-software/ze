// Design: docs/architecture/api/process-protocol.md -- complete selected receipts.
package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"
)

type selectedContextKey struct{}

func selectedResultFixture() (SelectRoutesInput, *SelectRoutesOutput) {
	input := SelectRoutesInput{Destination: "192.0.2.1", DestinationSession: 3,
		Routes: []SelectedRoute{{Kind: SelectedRouteReceived, Family: "ipv4/unicast",
			KeyNLRI: []byte{24, 10, 0, 0}, Revision: 7}}}
	output := &SelectRoutesOutput{Results: []SelectedRouteResult{{Family: "ipv4/unicast",
		KeyNLRI: []byte{24, 10, 0, 0}, Revision: 7, CurrentRevision: 7, Outcome: SelectedWritten}}}
	return input, output
}

// Complete wire receipts remain useful, while omission, reordering and unknown
// outcomes cannot be mistaken for downstream completion.
func TestSelectedFinalResultCompleteness(t *testing.T) {
	input, output := selectedResultFixture()
	if err := output.Validate(input); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*SelectRoutesOutput)
	}{
		{"missing", func(out *SelectRoutesOutput) { out.Results = nil }},
		{"key", func(out *SelectRoutesOutput) { out.Results[0].KeyNLRI = []byte{0} }},
		{"revision", func(out *SelectRoutesOutput) { out.Results[0].Revision = 8 }},
		{"frontier", func(out *SelectRoutesOutput) { out.Results[0].CurrentRevision = 8 }},
		{"unspecified", func(out *SelectRoutesOutput) { out.Results[0].Outcome = SelectedOutcomeUnspecified }},
		{"unknown", func(out *SelectRoutesOutput) { out.Results[0].Outcome = 99 }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, output := selectedResultFixture()
			test.mutate(output)
			if err := output.Validate(input); err == nil {
				t.Fatal("incomplete final result was accepted")
			}
		})
	}
	var missing *SelectRoutesOutput
	if err := missing.Validate(input); err == nil {
		t.Fatal("nil final result was accepted")
	}
}

// Typed dispatch retains context cancellation and validates the same final
// result as the socket adapter, rather than accepting queue admission.
func TestSelectedTypedBridgeCancellationAndFinalReceipt(t *testing.T) {
	input, output := selectedResultFixture()
	bridge := NewDirectBridge()
	called := false
	bridge.SetSelectRoutes(func(ctx context.Context, got SelectRoutesInput) (*SelectRoutesOutput, error) {
		called = true
		if ctx.Value(selectedContextKey{}) != "retained" {
			t.Fatal("typed selected call discarded its context")
		}
		if got.Routes[0].Revision != 7 {
			t.Fatal("typed selected call changed its input")
		}
		return output, nil
	})
	bridge.SetReady()
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := bridge.SelectRoutes(canceled, input); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled selected call: %v", err)
	}
	if called {
		t.Fatal("canceled selected call reached its handler")
	}
	ctx := context.WithValue(t.Context(), selectedContextKey{}, "retained")
	if _, err := bridge.SelectRoutes(ctx, input); err != nil {
		t.Fatal(err)
	}
	output.Results = nil
	if _, err := bridge.SelectRoutes(ctx, input); err == nil {
		t.Fatal("typed selected call accepted a missing final receipt")
	}
}

// Byte accounting includes the actual selected JSON and the widest RPC ID.
func TestSelectedFrameSizeMatchesWireEnvelope(t *testing.T) {
	input, _ := selectedResultFixture()
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	got, err := SelectRoutesFrameSize(input)
	if err != nil {
		t.Fatal(err)
	}
	if want := len(AppendRequest(nil, math.MaxUint64, MethodSelectRoutes, encoded)); got != want {
		t.Fatalf("selected frame size %d, wire size %d", got, want)
	}
}
