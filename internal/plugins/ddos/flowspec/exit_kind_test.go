// Design: docs/guide/ddos-mitigation.md -- what each end of the plugin does to a live FlowSpec rule
// Related: register.go (exitKind, stopResponder)

package flowspec

import (
	"testing"
)

// unsetExitKindPanic is the BUG panic stopResponder raises for an unset kind.
const unsetExitKindPanic = "BUG: ddos-flowspec: stopResponder called with no exit kind"

// TestStopResponderRefusesAnUnsetExitKindOnEveryPath proves that the zero
// exitKind panics at the entry of stopResponder, whatever the responder holds.
//
// VALIDATES: exitUnspecified is a state only a Ze defect produces, and the
// guard fires before any early return, so a caller that forgets the kind is
// caught on the paths tests run, not only when a withdrawal fails.
// METHOD: stopResponder with exitUnspecified on a nil responder, an idle one
// and one that announces; recover and compare the panic message.
// PREVENTS: a guard that sits below the `r == nil` and `!r.active` returns and
// never runs on the common path.
//
// MUTATION: with the check moved back below the early returns (into the
// switch), the "nil" and "inactive" rows do not panic and go red.
func TestStopResponderRefusesAnUnsetExitKindOnEveryPath(t *testing.T) {
	tests := []struct {
		name string
		make func(t *testing.T) *responder
	}{
		{name: "nil", make: func(*testing.T) *responder { return nil }},
		{name: "inactive", make: func(*testing.T) *responder {
			return newResponder(enforcingConfig(), &fakeDispatcher{})
		}},
		{name: "active", make: func(t *testing.T) *responder {
			r := newResponder(enforcingConfig(), &fakeDispatcher{})
			announceOnce(t, r, exitTestVictim)
			return r
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := tc.make(t)
			defer func() {
				got := recover()
				if got != unsetExitKindPanic {
					t.Errorf("stopResponder(%s, exitUnspecified) panic = %v, want %q", tc.name, got, unsetExitKindPanic)
				}
			}()
			stopResponder(r, exitUnspecified)
		})
	}
}
