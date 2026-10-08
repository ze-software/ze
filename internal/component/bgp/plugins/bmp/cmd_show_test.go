package bmp

import (
	"strings"

	"testing"

	"github.com/stretchr/testify/assert"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func TestShowBMPRPCRegistration(t *testing.T) {
	allRPCs := pluginserver.AllBuiltinRPCs()

	var found []string
	for _, reg := range allRPCs {
		if strings.HasPrefix(reg.WireMethod, "ze-bgp:show-bmp-") {
			found = append(found, reg.WireMethod)
		}
	}

	assert.Len(t, found, 4, "expected 4 BMP proxy RPCs")

	byWire := make(map[string]bool, len(found))
	for _, w := range found {
		byWire[w] = true
	}

	for _, wire := range []string{
		"ze-bgp:show-bmp-sessions",
		"ze-bgp:show-bmp-peers",
		"ze-bgp:show-bmp-collectors",
		"ze-bgp:show-bmp-rib",
	} {
		assert.True(t, byWire[wire], "missing RPC: %s", wire)
	}
}

func TestShowBMPPluginCommands(t *testing.T) {
	allRPCs := pluginserver.AllBuiltinRPCs()

	expected := map[string]string{
		"ze-bgp:show-bmp-sessions":   "show bmp sessions",
		"ze-bgp:show-bmp-peers":      "show bmp peers",
		"ze-bgp:show-bmp-collectors": "show bmp collectors",
		"ze-bgp:show-bmp-rib":        "show bmp rib",
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
