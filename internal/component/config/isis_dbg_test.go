// VALIDATES: the isis-net / isis-system-id custom validators are reachable via
// the explicit Validate(path, value) entry point (the editor/path-validation
// route), complementing the leaf-list walkTree coverage in
// isis_net_validate_test.go. Pins that the ze:validate names resolve through the
// registry built by RegisterValidators.
// PREVENTS: a regression where the isis validators are defined but not
// registered, so a path-based validate silently accepts a bad NET / system-id.
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
	reg.MergeGlobalCompleteFns()

	// Both ze:validate names resolve to a registered validator.
	netV := reg.Get("isis-net")
	require.NotNil(t, netV, "isis-net validator must be registered")
	sysV := reg.Get("isis-system-id")
	require.NotNil(t, sysV, "isis-system-id validator must be registered")

	assert.NoError(t, netV.ValidateFn("isis/net", "49.0001.0000.0000.0001.00"))
	assert.Error(t, netV.ValidateFn("isis/net", "49.0001.00"))
	assert.NoError(t, sysV.ValidateFn("isis/system-id", "0000.0000.0001"))
	assert.Error(t, sysV.ValidateFn("isis/system-id", "zzzz.0000.0001"))
}
