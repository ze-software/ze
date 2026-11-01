// Design: docs/architecture/plugin/rib-storage-design.md -- validation eligibility between receive stores
// Related: validation.go -- NoteValidationGate, DecodeValidationGate

package ribevents

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestNoteValidationGateFailsClosedWithoutLookup proves a process that holds
// no receive-gate lookup refuses every route once the gate's owner announces
// it enabled validation elsewhere, and keeps a registered lookup untouched.
//
// VALIDATES: an enabled announcement with no local lookup turns
// ValidationEnabled on and RouteEligible to false; a disabled announcement
// restores "no validation"; a registered lookup is neither replaced by an
// announcement nor removed by a disabled one.
// PREVENTS: a forked bgp-adj-rib-in leaving RouteEligible answering true for
// every path in the daemon, so RPKI and ASPA gating stop with no error.
// Method: drive NoteValidationGate through each state and read the gate.
func TestNoteValidationGateFailsClosedWithoutLookup(t *testing.T) {
	t.Cleanup(func() { RegisterValidationLookup(nil, nil) })
	RegisterValidationLookup(nil, nil)
	route := ValidationRoute{
		Peer:   netip.MustParseAddr("192.0.2.1"),
		Family: family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast},
		Prefix: netip.MustParsePrefix("203.0.113.0/24"),
	}
	if !RouteEligible(route, 0) {
		t.Fatal("no gate anywhere: the route must be eligible")
	}

	NoteValidationGate(ValidationGateState{Enabled: true})
	if !ValidationEnabled() {
		t.Fatal("a gate enabled in another process must turn validation on here")
	}
	if RouteEligible(route, 0) {
		t.Fatal("a gate this process cannot read must refuse the route")
	}
	if RouteEligible(route, 7) {
		t.Fatal("a gate this process cannot read must refuse every generation")
	}

	NoteValidationGate(ValidationGateState{Enabled: false})
	if ValidationEnabled() {
		t.Fatal("a disabled gate must leave no validation here")
	}
	if !RouteEligible(route, 0) {
		t.Fatal("a disabled gate must make the route eligible again")
	}

	RegisterValidationLookup(func(ValidationRoute, uint64) bool { return true }, func(ValidationRoute) bool { return true })
	NoteValidationGate(ValidationGateState{Enabled: true})
	if !RouteEligible(route, 0) {
		t.Fatal("an announcement must not replace a registered lookup")
	}
	NoteValidationGate(ValidationGateState{Enabled: false})
	if !ValidationEnabled() {
		t.Fatal("a disabled announcement must not remove a registered lookup")
	}
}

// TestDecodeValidationGate proves the enveloped validation-gate event a
// forked subscriber receives decodes to its state, and that other events and
// text pass unmatched. Method: decode one envelope of each kind.
func TestDecodeValidationGate(t *testing.T) {
	envelope := func(event, payload string) string {
		b, err := json.Marshal(rpc.EventEnvelope{Namespace: Namespace, Event: event, Payload: json.RawMessage(payload)})
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	state, matched, err := DecodeValidationGate(envelope(EventValidationGate, `{"enabled":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Fatal("validation-gate envelope not matched")
	}
	if !state.Enabled {
		t.Fatal("enabled state lost")
	}
	if _, matched, _ := DecodeValidationGate(envelope(EventValidationChange, `[]`)); matched {
		t.Fatal("validation-change matched as validation-gate")
	}
	if _, matched, _ := DecodeValidationGate("peer 192.0.2.1 up"); matched {
		t.Fatal("text event matched")
	}
	if _, matched, err := DecodeValidationGate(envelope(EventValidationGate, `{"enabled":"yes"}`)); !matched || err == nil {
		t.Fatal("a payload that does not decode must be matched and refused")
	}
}
