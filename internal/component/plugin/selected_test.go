// Design: docs/architecture/api/process-protocol.md -- selected operation admission.
package plugin

import (
	"math"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func selectedBoundaryInput() rpc.SelectRoutesInput {
	return rpc.SelectRoutesInput{Destination: "192.0.2.7", DestinationSession: 3,
		Routes: []rpc.SelectedRoute{{Kind: rpc.SelectedRouteReceived,
			Family: "ipv4/unicast", KeyNLRI: []byte{24, 10, 0, 0}, Revision: math.MaxUint64,
			SourceReceipt: 5, SourceAddPath: true, StaleLevel: rpc.SelectedFresh,
			Route: &rpc.StoredRoute{SourcePeer: "192.0.2.8", Family: "ipv4/unicast",
				NLRIHex: "180a0000", AttrHex: "40010100", NextHopHex: "c0000201",
				MsgID: 7, SourcePolicyEpoch: 11, NLRIFraming: rpc.NLRIFramingPrefixOnly}}}}
}

// A valid captured path, including ADD-PATH identifier zero and maximum revision,
// survives DTO mutation; malformed variants never become a validated batch.
func TestSelectedBatchAdmissionAndOwnedSnapshot(t *testing.T) {
	input := selectedBoundaryInput()
	batch, err := NewSelectedBatch(input)
	if err != nil {
		t.Fatal(err)
	}
	input.Routes[0].KeyNLRI[1] = 99
	input.Routes[0].Route.SourcePolicyEpoch = 99
	got := batch.Input().Routes[0]
	if got.KeyNLRI[1] != 10 || got.Route.SourcePolicyEpoch != 11 {
		t.Fatalf("validated snapshot borrowed mutable caller state: %+v", got)
	}
	if got.Route.PathID != 0 || !got.SourceAddPath {
		t.Fatalf("ADD-PATH zero lost its explicit framing: %+v", got)
	}
	cases := []struct {
		name   string
		mutate func(*rpc.SelectRoutesInput)
	}{
		{"destination", func(in *rpc.SelectRoutesInput) { in.Destination = "bad" }},
		{"session", func(in *rpc.SelectRoutesInput) { in.DestinationSession = 0 }},
		{"revision", func(in *rpc.SelectRoutesInput) { in.Routes[0].Revision = 0 }},
		{"key", func(in *rpc.SelectRoutesInput) { in.Routes[0].KeyNLRI = nil }},
		{"unknown-kind", func(in *rpc.SelectRoutesInput) { in.Routes[0].Kind = 99 }},
		{"source-receipt", func(in *rpc.SelectRoutesInput) { in.Routes[0].SourceReceipt = 0 }},
		{"mixed-receipts", func(in *rpc.SelectRoutesInput) { in.Routes[0].OriginReceipt = 6 }},
		{"source-epoch", func(in *rpc.SelectRoutesInput) { in.Routes[0].Route.SourcePolicyEpoch = 0 }},
		{"unknown-framing", func(in *rpc.SelectRoutesInput) { in.Routes[0].Route.NLRIFraming = 99 }},
		{"unknown-stale", func(in *rpc.SelectRoutesInput) { in.Routes[0].StaleLevel = 99 }},
		{"invalid-hex", func(in *rpc.SelectRoutesInput) { in.Routes[0].Route.NLRIHex = "zz" }},
		{"withdraw-as-announce", func(in *rpc.SelectRoutesInput) { in.Routes[0].Route.Withdraw = true }},
		{"absence-with-route", func(in *rpc.SelectRoutesInput) { in.Routes[0].Kind = rpc.SelectedRouteAbsent }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			input := selectedBoundaryInput()
			test.mutate(&input)
			if _, err := NewSelectedBatch(input); err == nil {
				t.Fatal("malformed selected input was admitted")
			}
		})
	}
}

// Scoped origins and selected absence use separate receipts; neither acquires
// received authority by leaving original source provenance empty.
func TestSelectedOriginAndAbsenceAdmission(t *testing.T) {
	input := selectedBoundaryInput()
	route := &input.Routes[0]
	route.Kind, route.SourceReceipt, route.OriginReceipt = rpc.SelectedRouteOrigin, 0, 9
	route.SourceAddPath = false
	route.Route.SourcePeer, route.Route.SourcePolicyEpoch, route.Route.MsgID = "", 0, 0
	if _, err := NewSelectedBatch(input); err != nil {
		t.Fatal(err)
	}
	route.Kind, route.Route, route.OriginReceipt = rpc.SelectedRouteAbsent, nil, 0
	route.StaleLevel = rpc.SelectedStaleUnspecified
	if _, err := NewSelectedBatch(input); err != nil {
		t.Fatal(err)
	}
}

// Reusing a configured name cannot revive an old selecting lifetime.
func TestSelectedOwnerIncarnationAndRevocation(t *testing.T) {
	oldDone, newDone := make(chan struct{}), make(chan struct{})
	old, err := NewSelectedOwner(ProcessSender("rib"), oldDone, nil)
	if err != nil {
		t.Fatal(err)
	}
	current, err := NewSelectedOwner(ProcessSender("rib"), newDone, nil)
	if err != nil {
		t.Fatal(err)
	}
	if old.SameIncarnation(current) {
		t.Fatal("same-named replacement inherited old process authority")
	}
	close(oldDone)
	if err := old.Current(); err == nil {
		t.Fatal("stopped owner remains current")
	}
	if err := current.Current(); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSelectedOwner(OperatorSender(), newDone, nil); err == nil {
		t.Fatal("operator was admitted as a selecting process")
	}
	ended := make(chan struct{})
	finished, err := NewSelectedOwner(ProcessSender("rib"), newDone, ended)
	if err != nil {
		t.Fatal(err)
	}
	close(ended)
	if err := finished.Current(); err == nil {
		t.Fatal("returned in-process engine remains current before Stop")
	}
}
