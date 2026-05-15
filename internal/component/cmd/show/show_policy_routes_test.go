package show

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func TestShowPolicyRoutesRPCRegistration(t *testing.T) {
	allRPCs := pluginserver.AllBuiltinRPCs()

	var found []pluginserver.RPCRegistration
	for _, reg := range allRPCs {
		if reg.WireMethod == "ze-show:policy-routes" {
			found = append(found, reg)
		}
	}

	assert.Len(t, found, 1, "expected 1 policy-routes proxy RPC")
	assert.Equal(t, "show policy-routes", found[0].PluginCommand)
}

func TestShowPolicyRoutesHandlerNonNil(t *testing.T) {
	assert.NotNil(t, forwardShowPolicyRoutes, "forwardShowPolicyRoutes must not be nil")
}
