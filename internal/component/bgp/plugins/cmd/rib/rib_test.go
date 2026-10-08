package rib

import (
	"strings"

	"testing"

	"github.com/stretchr/testify/assert"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestRibProxyRPCRegistration verifies all RIB proxy RPCs are registered
// with correct wire methods.
//
// VALIDATES: init() registers 6 RIB proxy RPCs with correct metadata.
// PREVENTS: Misspelled wire methods or CLI commands silently breaking dispatch.
func TestRibProxyRPCRegistration(t *testing.T) {
	allRPCs := pluginserver.AllBuiltinRPCs()

	// Collect RIB RPCs (wire method starts with "ze-bgp:rib-")
	var found []string
	for _, reg := range allRPCs {
		if strings.HasPrefix(reg.WireMethod, "ze-bgp:rib-") {
			found = append(found, reg.WireMethod)
		}
	}

	assert.Len(t, found, 11, "expected 11 RIB proxy RPCs")

	// Build lookup for assertions
	byWire := make(map[string]bool, len(found))
	for _, w := range found {
		byWire[w] = true
	}

	// All expected wire methods present
	for _, wire := range []string{
		"ze-bgp:rib-status",
		"ze-bgp:rib-routes",
		"ze-bgp:rib-best",
		"ze-bgp:rib-best-status",
		"ze-bgp:rib-clear-in",
		"ze-bgp:rib-clear-out",
		"ze-bgp:rib-rpf",
		"ze-bgp:rib-inject",
		"ze-bgp:rib-withdraw",
		"ze-bgp:rib-protocol",
		"ze-bgp:rib-fastpath",
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
		"rpf":        forwardRibRPF,
	}
	for name, h := range handlers {
		assert.NotNil(t, h, "handler %s must not be nil", name)
	}
}
