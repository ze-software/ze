package rib

import (
	"strings"

	"testing"

	"github.com/stretchr/testify/assert"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// TestRibProxyRPCRegistration verifies the RIB proxy RPCs are registered
// with the wire methods the YANG declares, and that no other RPC claims to be one.
//
// VALIDATES: init() registers exactly the RIB proxy RPCs listed below. A proxy
// RPC is one that forwards to the bgp-rib plugin, so it carries a
// PluginCommand; an RPC under the same "ze-bgp:rib-" prefix answered by
// another package (rib-clear-advertised, cmd/peer) is not one.
// PREVENTS: Misspelled wire methods or CLI commands silently breaking dispatch,
// and a proxy RPC added or dropped without this list noticing.
func TestRibProxyRPCRegistration(t *testing.T) {
	allRPCs := pluginserver.AllBuiltinRPCs()

	var found []string
	for _, reg := range allRPCs {
		if !strings.HasPrefix(reg.WireMethod, "ze-bgp:rib-") {
			continue
		}
		if reg.PluginCommand == "" {
			continue
		}
		found = append(found, reg.WireMethod)
	}

	assert.ElementsMatch(t, []string{
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
	}, found, "RIB proxy RPCs")
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
