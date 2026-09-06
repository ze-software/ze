package rpc

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFailurePolicyValidAcceptsOnlyTheThreeSpellings: the wire is a closed set.
//
// VALIDATES: AC-5 -- a value that is not one of the three is refused, and the
// refusal names the value and what to write instead.
//
// PREVENTS: a fourth spelling being read as the zero value. FailurePolicy is
// what decides whether ze restarts a plugin or stops the router, so a typo that
// decoded silently to "declared nothing" would give the plugin's author the
// opposite of what they wrote, with nothing said.
func TestFailurePolicyValidAcceptsOnlyTheThreeSpellings(t *testing.T) {
	accepted := map[string]FailurePolicy{
		"restart": FailureRestart,
		"ignore":  FailureIgnore,
		"fatal":   FailureFatal,
	}

	for wire, want := range accepted {
		t.Run(wire, func(t *testing.T) {
			var got FailurePolicy
			require.NoError(t, got.UnmarshalText([]byte(wire)))
			assert.Equal(t, want, got)
			assert.Equal(t, wire, got.String())
		})
	}

	for _, wire := range []string{"", "RESTART", "restarts", "unspecified", "stop", "true"} {
		t.Run("refused "+wire, func(t *testing.T) {
			var got FailurePolicy
			err := got.UnmarshalText([]byte(wire))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "restart", "the refusal must say what to write instead")
			assert.Equal(t, FailureUnspecified, got, "a refused value must leave the policy undeclared")
		})
	}
}

// TestFailurePolicyAllowsARestartOnlyForRestart: one value answers both
// questions ze asks about a plugin's failure.
//
// VALIDATES: AC-1, AC-2, AC-3 -- "what happens when this plugin fails" and "may
// this plugin be started again" are the same declaration, because ze starts a
// plugin again for exactly one reason.
//
// PREVENTS: silence being read as consent. A plugin that declared nothing must
// not be restarted, so the unspecified sentinel answers false here as firmly as
// ignore and fatal do.
func TestFailurePolicyAllowsARestartOnlyForRestart(t *testing.T) {
	assert.True(t, FailureRestart.AllowsRestart())
	assert.False(t, FailureIgnore.AllowsRestart())
	assert.False(t, FailureFatal.AllowsRestart())
	assert.False(t, FailureUnspecified.AllowsRestart())
}

// TestDeclareRegistrationCarriesTheFailurePolicy: the declaration survives the
// wire an external plugin speaks.
//
// VALIDATES: AC-1 -- a plugin in another language declares its policy with one
// JSON key, and a plugin that declares nothing sends no key at all.
//
// PREVENTS: the field being engine-only. An external plugin is in no
// compile-time registry, so this key is its only route to the declaration.
func TestDeclareRegistrationCarriesTheFailurePolicy(t *testing.T) {
	encoded, err := json.Marshal(DeclareRegistrationInput{FailurePolicy: FailureFatal})
	require.NoError(t, err)
	assert.JSONEq(t, `{"failure-policy":"fatal"}`, string(encoded))

	var decoded DeclareRegistrationInput
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, FailureFatal, decoded.FailurePolicy)

	silent, err := json.Marshal(DeclareRegistrationInput{})
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(silent), "a plugin that declares nothing sends no key")

	var refused DeclareRegistrationInput
	assert.Error(t, json.Unmarshal([]byte(`{"failure-policy":"maybe"}`), &refused),
		"an unknown policy must fail the whole registration rather than decode to silence")
}
