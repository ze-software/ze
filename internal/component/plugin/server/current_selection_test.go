// Design: docs/architecture/api/process-protocol.md -- current selection capability.
package server

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func TestCurrentSelectionDeclarationCarriesSupportOnly(t *testing.T) {
	registration := registrationFromRPC(&rpc.DeclareRegistrationInput{CurrentSelection: true})
	require.True(t, registration.CurrentSelection)
	require.Empty(t, registration.Dissemination)
	require.False(t, registration.SourcePolicyOwner)
	require.False(t, registration.SourcePolicy)
}
