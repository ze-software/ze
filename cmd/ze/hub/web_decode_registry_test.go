//go:build ze_web

// Design: docs/architecture/web-interface.md -- native decode registration on standalone web.
package hub

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func decodedNativeMessageType(t *testing.T, output string) string {
	t.Helper()
	var envelope map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(output), &envelope))
	var decoded struct {
		Input    string `json:"input"`
		Messages []struct {
			Hex     string          `json:"hex"`
			Decoded json.RawMessage `json:"decoded"`
		} `json:"messages"`
	}
	require.NoError(t, json.Unmarshal(envelope["bgp-decode"], &decoded), "the native command envelope must be retained")
	require.Equal(t, "message", decoded.Input)
	require.Len(t, decoded.Messages, 1)
	require.Equal(t, keepaliveHex, decoded.Messages[0].Hex)
	return decodedMessageType(t, string(decoded.Messages[0].Decoded))
}

// TestWebOnlyDispatcherUsesRegisteredCommands preserves native decode and
// ordinary command forwarding with a real registry. An uncompiled decoder is
// not intercepted: another registered holder receives the same input.
func TestWebOnlyDispatcherUsesRegisteredCommands(t *testing.T) {
	server, dispatch := standaloneCommandFixture(t)
	innerCalled := false
	if pluginreg.GetPacketDecoder() == nil {
		require.NoError(t, server.Dispatcher().Register("show bgp decode", func(_ *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
			innerCalled = true
			return plugin.NewResponse(plugin.StatusDone, plugin.Map{"inner": "show bgp decode " + strings.Join(args.Tokens(), " ")}), nil
		}, "test holder for an uncompiled decoder"))
	}
	rendered, err := dispatch.JSON(context.Background(), wodCaller, "show bgp decode "+keepaliveHex)
	defer rendered.TransportComplete()
	require.NoError(t, err)
	if pluginreg.GetPacketDecoder() != nil {
		assert.False(t, innerCalled)
		assert.Equal(t, "keepalive", decodedNativeMessageType(t, rendered.Output))
	} else {
		assert.True(t, innerCalled)
		assert.JSONEq(t, `{"inner":"show bgp decode FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304"}`, rendered.Output)
	}
	require.NoError(t, server.Dispatcher().Register("show test-native-forwarding", func(_ *pluginserver.CommandContext, args command.ValidatedArgs) (*plugin.Response, error) {
		innerCalled = true
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"inner": "show test-native-forwarding " + strings.Join(args.Tokens(), " ")}), nil
	}, "test ordinary command forwarding"))
	innerCalled = false
	rendered, err = dispatch.JSON(context.Background(), wodCaller, "show test-native-forwarding 1.1.1.1")
	defer rendered.TransportComplete()
	require.NoError(t, err)
	assert.True(t, innerCalled)
	assert.JSONEq(t, `{"inner":"show test-native-forwarding 1.1.1.1"}`, rendered.Output)
}

// TestWebOnlyDispatcherDecodesWithoutADaemon retains the old no-upstream
// workflow through a constructed command server and its absent-feature refusal.
func TestWebOnlyDispatcherDecodesWithoutADaemon(t *testing.T) {
	_, dispatch := standaloneCommandFixture(t)
	rendered, err := dispatch.JSON(context.Background(), wodCaller, "show bgp decode "+keepaliveHex)
	defer rendered.TransportComplete()
	if pluginreg.GetPacketDecoder() != nil {
		require.NoError(t, err)
		assert.Equal(t, "keepalive", decodedNativeMessageType(t, rendered.Output))
	} else {
		require.ErrorIs(t, err, errWebOnlyUnavailable)
		assert.Contains(t, err.Error(), "running daemon")
	}
	_, err = dispatch.JSON(context.Background(), wodCaller, "show standalone-unavailable")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "running daemon")
}
