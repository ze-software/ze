package update

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// TestUpdateRPCRegistered verifies the update verb RPC is registered via init().
//
// VALIDATES: Wire method "ze-update:bgp-peer-prefix" is registered with a handler.
// PREVENTS: Update verb RPC silently missing from dispatch.
func TestUpdateRPCRegistered(t *testing.T) {
	rpcs := pluginserver.AllBuiltinRPCs()

	var found bool
	for _, reg := range rpcs {
		if reg.WireMethod != "ze-update:bgp-peer-prefix" {
			continue
		}
		found = true
		assert.NotNil(t, reg.Handler, "handler must not be nil")
		assert.NotEmpty(t, reg.Help, "help text must not be empty")
		assert.True(t, reg.RequiresSelector, "update prefix requires peer selector")
		break
	}
	assert.True(t, found, "ze-update:bgp-peer-prefix RPC must be registered")
}

// TestUpdateRPCCount verifies exactly one ze-update: RPC is registered.
//
// VALIDATES: No duplicate or missing ze-update: registrations.
// PREVENTS: Accidental double-registration or missing registration.
func TestUpdateRPCCount(t *testing.T) {
	rpcs := pluginserver.AllBuiltinRPCs()

	var updatePrefixCount int
	for _, reg := range rpcs {
		if strings.HasPrefix(reg.WireMethod, "ze-update:") {
			updatePrefixCount++
		}
	}
	assert.Equal(t, 1, updatePrefixCount, "expected exactly 1 ze-update: RPC")
}

// TestOldPathRemoved verifies the old wire method is no longer registered.
//
// VALIDATES: AC-2 -- "peer * prefix update" (old path) no longer dispatches.
// PREVENTS: Old wire method lingering after migration to update verb.
func TestOldPathRemoved(t *testing.T) {
	rpcs := pluginserver.AllBuiltinRPCs()

	for _, reg := range rpcs {
		if reg.WireMethod == "ze-bgp:peer-prefix-update" {
			t.Fatal("old wire method ze-bgp:peer-prefix-update must not be registered")
		}
	}
}
