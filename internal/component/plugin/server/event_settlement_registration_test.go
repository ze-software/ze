// Design: docs/architecture/api/process-protocol.md -- event settlement declaration.
package server

import (
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestEventSettlementRegistrationPreservesDeclaredSupport checks both Stage-1
// decisions and that later mutation of the wire value cannot change the copy.
func TestEventSettlementRegistrationPreservesDeclaredSupport(t *testing.T) {
	for _, support := range []bool{false, true} {
		input := rpc.DeclareRegistrationInput{EventSettlement: support}
		registration := registrationFromRPC(&input)
		input.EventSettlement = !support
		if registration.EventSettlement != support {
			t.Fatalf("copied support = %t, want %t", registration.EventSettlement, support)
		}
	}
}
