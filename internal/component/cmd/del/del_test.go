package del

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// TestDelRPCRegistered verifies the del verb RPC is registered via init().
//
// VALIDATES: Wire method "ze-del:bgp-peer" is registered with a handler.
// PREVENTS: Del verb RPC silently missing from dispatch.
func TestDelRPCRegistered(t *testing.T) {
	rpcs := pluginserver.AllBuiltinRPCs()

	var found bool
	for _, reg := range rpcs {
		if reg.WireMethod != "ze-del:bgp-peer" {
			continue
		}
		found = true
		assert.NotNil(t, reg.Handler, "handler must not be nil")
		assert.True(t, reg.RequiresSelector, "del peer requires peer selector")
		break
	}
	assert.True(t, found, "ze-del:bgp-peer RPC must be registered")
}

// TestDelRPCCount verifies exactly one ze-del: RPC is registered.
//
// VALIDATES: No duplicate or missing ze-del: registrations.
// PREVENTS: Accidental double-registration or missing registration.
func TestDelRPCCount(t *testing.T) {
	rpcs := pluginserver.AllBuiltinRPCs()

	var delPrefixCount int
	for _, reg := range rpcs {
		if strings.HasPrefix(reg.WireMethod, "ze-del:") {
			delPrefixCount++
		}
	}
	assert.Equal(t, 1, delPrefixCount, "expected exactly 1 ze-del: RPC")
}
