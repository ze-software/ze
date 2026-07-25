package bmp

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func TestShowBMPRPCRegistration(t *testing.T) {
	allRPCs := pluginserver.AllBuiltinRPCs()

	var found []string
	for _, reg := range allRPCs {
		if len(reg.WireMethod) > 9 && reg.WireMethod[:9] == "ze-show:b" && reg.WireMethod[9:12] == "mp-" {
			found = append(found, reg.WireMethod)
		}
	}

	assert.Len(t, found, 4, "expected 4 BMP proxy RPCs")

	byWire := make(map[string]bool, len(found))
	for _, w := range found {
		byWire[w] = true
	}

	for _, wire := range []string{
		"ze-show:bmp-sessions",
		"ze-show:bmp-peers",
		"ze-show:bmp-collectors",
		"ze-show:bmp-rib",
	} {
		assert.True(t, byWire[wire], "missing RPC: %s", wire)
	}
}

func TestShowBMPPluginCommands(t *testing.T) {
	allRPCs := pluginserver.AllBuiltinRPCs()

	expected := map[string]string{
		"ze-show:bmp-sessions":   "show bmp sessions",
		"ze-show:bmp-peers":      "show bmp peers",
		"ze-show:bmp-collectors": "show bmp collectors",
		"ze-show:bmp-rib":        "show bmp rib",
	}

	for _, reg := range allRPCs {
		if cmd, ok := expected[reg.WireMethod]; ok {
			assert.Equal(t, cmd, reg.PluginCommand, "wrong PluginCommand for %s", reg.WireMethod)
		}
	}
}

func TestShowBMPHandlersNonNil(t *testing.T) {
	handlers := map[string]any{
		"sessions":   forwardShowBMPSessions,
		"peers":      forwardShowBMPPeers,
		"collectors": forwardShowBMPCollectors,
		"rib":        forwardShowBMPRib,
	}
	for name, h := range handlers {
		assert.NotNil(t, h, "handler %s must not be nil", name)
	}
}
