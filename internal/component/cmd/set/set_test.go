package set

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// TestSetRPCsRegistered verifies the set verb RPCs are registered via init().
//
// VALIDATES: Wire methods "ze-set:bgp-peer-with" and "ze-set:bgp-peer-save" are registered with handlers.
// PREVENTS: Set verb RPCs silently missing from dispatch.
func TestSetRPCsRegistered(t *testing.T) {
	rpcs := pluginserver.AllBuiltinRPCs()

	expected := []string{"ze-set:bgp-peer-with", "ze-set:bgp-peer-save"}
	for _, wm := range expected {
		var found bool
		for _, reg := range rpcs {
			if reg.WireMethod != wm {
				continue
			}
			found = true
			assert.NotNil(t, reg.Handler, "%s handler must not be nil", wm)
			assert.NotEmpty(t, reg.Help, "%s help text must not be empty", wm)
			assert.True(t, reg.RequiresSelector, "%s requires peer selector", wm)
			break
		}
		assert.True(t, found, "%s RPC must be registered", wm)
	}
}

// TestSetRPCCount verifies exactly two ze-set: RPCs are registered.
//
// VALIDATES: No duplicate or missing ze-set: registrations.
// PREVENTS: Accidental double-registration or missing registration.
func TestSetRPCCount(t *testing.T) {
	rpcs := pluginserver.AllBuiltinRPCs()

	var setPrefixCount int
	for _, reg := range rpcs {
		if strings.HasPrefix(reg.WireMethod, "ze-set:") {
			setPrefixCount++
		}
	}
	assert.Equal(t, 2, setPrefixCount, "expected exactly 2 ze-set: RPCs")
}
