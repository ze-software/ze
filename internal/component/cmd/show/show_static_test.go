package show

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

func TestShowStaticRPCRegistration(t *testing.T) {
	allRPCs := pluginserver.AllBuiltinRPCs()

	var found []pluginserver.RPCRegistration
	for _, reg := range allRPCs {
		if reg.WireMethod == "ze-show:static" {
			found = append(found, reg)
		}
	}

	assert.Len(t, found, 1, "expected 1 static proxy RPC")
	assert.Equal(t, "show static", found[0].PluginCommand)
}

func TestShowStaticHandlerNonNil(t *testing.T) {
	assert.NotNil(t, forwardShowStatic, "forwardShowStatic must not be nil")
}
