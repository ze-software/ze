// VALIDATES: the isis-net custom validator is reachable via the explicit
// Validate(path, value) entry point (the editor/path-validation route),
// complementing the leaf-list walkTree coverage in isis_net_validate_test.go,
// and the isis-system-id name stays declared for completion with no ValidateFn.
// Pins that both ze:validate names resolve through the registry built by
// RegisterValidators. The system-id refusal is the YANG pattern's, proven in
// isis_system_id_message_test.go.
// PREVENTS: a regression where the isis-net validator is defined but not
// registered, so a path-based validate silently accepts a bad NET; and a
// system-id ValidateFn coming back to speak a second refusal behind the pattern.
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config/yang"
)

func TestISISValidatorsRegisteredAndReachable(t *testing.T) {
	reg := yang.NewValidatorRegistry()
	RegisterValidators(reg)
	reg.MergeGlobalCompletions()

	// Both ze:validate names resolve to a registered validator.
	netV := reg.Get("isis-net")
	require.NotNil(t, netV, "isis-net validator must be registered")
	sysV := reg.Get("isis-system-id")
	require.NotNil(t, sysV, "isis-system-id validator must be registered")

	assert.NoError(t, netV.ValidateFn("isis/net", "49.0001.0000.0000.0001.00"))
	assert.Error(t, netV.ValidateFn("isis/net", "49.0001.00"))
	assert.Nil(t, sysV.ValidateFn, "isis-system-id must be completion-only: the YANG pattern is its one guard")
}
