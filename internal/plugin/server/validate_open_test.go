package server

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

// TestRegistrationFromRPCWantsValidateOpen verifies WantsValidateOpen propagation.
//
// VALIDATES: WantsValidateOpen from RPC input is copied to PluginRegistration.
// PREVENTS: WantsValidateOpen being ignored during Stage 1 conversion.
func TestRegistrationFromRPCWantsValidateOpen(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input bool
		want  bool
	}{
		{"true_propagates", true, true},
		{"false_propagates", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			input := &rpc.DeclareRegistrationInput{
				WantsValidateOpen: tt.input,
			}
			reg := registrationFromRPC(input)
			assert.Equal(t, tt.want, reg.WantsValidateOpen)
		})
	}
}
