package rr

import (
	"testing"

	"github.com/stretchr/testify/assert"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func TestShowRRRPCRegistration(t *testing.T) {
	allRPCs := pluginserver.AllBuiltinRPCs()

	var found []string
	for _, reg := range allRPCs {
		if len(reg.WireMethod) > 9 && reg.WireMethod[:9] == "ze-show:r" && reg.WireMethod[9:11] == "r-" {
			found = append(found, reg.WireMethod)
		}
	}

	assert.Len(t, found, 2, "expected 2 RR proxy RPCs")

	byWire := make(map[string]bool, len(found))
	for _, w := range found {
		byWire[w] = true
	}

	for _, wire := range []string{
		"ze-show:rr-status",
		"ze-show:rr-peers",
	} {
		assert.True(t, byWire[wire], "missing RPC: %s", wire)
	}
}

func TestShowRRPluginCommands(t *testing.T) {
	allRPCs := pluginserver.AllBuiltinRPCs()

	expected := map[string]string{
		"ze-show:rr-status": "show rr status",
		"ze-show:rr-peers":  "show rr peers",
	}

	for _, reg := range allRPCs {
		if cmd, ok := expected[reg.WireMethod]; ok {
			assert.Equal(t, cmd, reg.PluginCommand, "wrong PluginCommand for %s", reg.WireMethod)
		}
	}
}

func TestShowRRHandlersNonNil(t *testing.T) {
	handlers := map[string]any{
		"status": forwardShowRRStatus,
		"peers":  forwardShowRRPeers,
	}
	for name, h := range handlers {
		assert.NotNil(t, h, "handler %s must not be nil", name)
	}
}
