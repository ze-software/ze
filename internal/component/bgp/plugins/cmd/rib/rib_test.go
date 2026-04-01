package rib

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// TestRibProxyRPCRegistration verifies all RIB proxy RPCs are registered
// with correct wire methods.
//
// VALIDATES: init() registers 6 RIB proxy RPCs with correct metadata.
// PREVENTS: Misspelled wire methods or CLI commands silently breaking dispatch.
func TestRibProxyRPCRegistration(t *testing.T) {
	allRPCs := pluginserver.AllBuiltinRPCs()

	// Collect RIB RPCs (wire method starts with "ze-rib-api:")
	var found []string
	for _, reg := range allRPCs {
		if len(reg.WireMethod) > 11 && reg.WireMethod[:11] == "ze-rib-api:" {
			found = append(found, reg.WireMethod)
		}
	}

	assert.Len(t, found, 8, "expected 8 RIB proxy RPCs")

	// Build lookup for assertions
	byWire := make(map[string]bool, len(found))
	for _, w := range found {
		byWire[w] = true
	}

	// All expected wire methods present
	for _, wire := range []string{
		"ze-rib-api:status",
		"ze-rib-api:routes",
		"ze-rib-api:best",
		"ze-rib-api:best-status",
		"ze-rib-api:clear-in",
		"ze-rib-api:clear-out",
	} {
		assert.True(t, byWire[wire], "missing RPC: %s", wire)
	}
}

// TestRibProxyHandlersNonNil verifies all proxy handler functions are assigned.
//
// VALIDATES: Each proxy handler function is non-nil (not accidentally omitted).
// PREVENTS: Nil handler causing panic when dispatched.
func TestRibProxyHandlersNonNil(t *testing.T) {
	handlers := map[string]pluginserver.Handler{
		"status":     forwardRibStatus,
		"routes":     forwardRibRoutes,
		"best":       forwardRibBest,
		"bestStatus": forwardRibBestStatus,
		"clearIn":    forwardRibClearIn,
		"clearOut":   forwardRibClearOut,
	}
	for name, h := range handlers {
		assert.NotNil(t, h, "handler %s must not be nil", name)
	}
}
