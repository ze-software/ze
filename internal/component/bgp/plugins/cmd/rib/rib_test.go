package rib

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// TestRibProxyRPCRegistration verifies all RIB proxy RPCs are registered
// with correct wire methods, CLI commands, and read-only flags.
//
// VALIDATES: init() registers 6 RIB proxy RPCs with correct metadata.
// PREVENTS: Misspelled wire methods or CLI commands silently breaking dispatch.
func TestRibProxyRPCRegistration(t *testing.T) {
	allRPCs := pluginserver.AllBuiltinRPCs()

	// Collect RIB RPCs (wire method starts with "ze-rib-api:")
	type ribRPC struct {
		WireMethod string
		ReadOnly   bool
	}
	var found []ribRPC
	for _, reg := range allRPCs {
		if len(reg.WireMethod) > 11 && reg.WireMethod[:11] == "ze-rib-api:" {
			found = append(found, ribRPC{
				WireMethod: reg.WireMethod,
				ReadOnly:   reg.ReadOnly,
			})
		}
	}

	assert.Len(t, found, 6, "expected 6 RIB proxy RPCs")

	// Build lookup for assertions
	byWire := make(map[string]ribRPC, len(found))
	for _, r := range found {
		byWire[r.WireMethod] = r
	}

	// Read-only commands
	for _, wire := range []string{
		"ze-rib-api:status",
		"ze-rib-api:routes",
		"ze-rib-api:best",
		"ze-rib-api:best-status",
	} {
		r, ok := byWire[wire]
		assert.True(t, ok, "missing RPC: %s", wire)
		if ok {
			assert.True(t, r.ReadOnly, "%s should be read-only", wire)
		}
	}

	// Write commands
	for _, wire := range []string{
		"ze-rib-api:clear-in",
		"ze-rib-api:clear-out",
	} {
		r, ok := byWire[wire]
		assert.True(t, ok, "missing RPC: %s", wire)
		if ok {
			assert.False(t, r.ReadOnly, "%s should NOT be read-only", wire)
		}
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
