// Design: docs/architecture/api/process-protocol.md -- original accepted-record declaration.
package server

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// Stage 1 records data-only lookup support without deriving it from another
// callback. The original registration never manufactures an intake capability.
func TestOriginAdmissionLookupRegistrationIsIndependent(t *testing.T) {
	for _, declared := range []bool{false, true} {
		input := &rpc.DeclareRegistrationInput{OriginAdmissionLookup: declared}
		registration := registrationFromRPC(input)
		require.Equal(t, declared, registration.OriginAdmissionLookup)
		require.False(t, registration.OriginIntake)
		require.False(t, registration.CurrentSelection)
		require.False(t, registration.SourcePolicyOwner)
	}
}
