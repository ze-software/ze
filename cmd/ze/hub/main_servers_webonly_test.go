//go:build ze_web

package hub

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	zeweb "github.com/ze-software/ze/internal/component/web"
)

// wodCaller is the fixed caller identity these web-only dispatcher tests pass.
var wodCaller = plugin.CallerIdentity{Username: "alice", RemoteAddr: "127.0.0.1"}

// TestWebOnlyDispatcherFriendlyError verifies that webOnlyDispatcher answers
// event commands from the local ring but returns a friendly, daemon-oriented
// message (not the raw "command not available in web-only mode: <cmd>" string)
// for operational commands it cannot serve. The web UI surfaces this verbatim
// (tools inline, log pages map it to an honest empty state), so it must read as
// guidance, not an internal error (F4/AC-10).
func TestWebOnlyDispatcherFriendlyError(t *testing.T) {
	ring := pluginserver.NewEventRing(16)
	dispatch := webOnlyDispatcher(ring)

	// Operational command the stub cannot serve.
	_, err := dispatch.JSON(context.Background(), wodCaller, "show ping 1.1.1.1")
	if err == nil {
		t.Fatal("expected an error for an unsupported operational command")
	}
	msg := err.Error()
	if strings.Contains(msg, "web-only mode") || strings.Contains(msg, "show ping") {
		t.Fatalf("error should not leak the mode jargon or raw command, got: %q", msg)
	}
	if !strings.Contains(msg, "running daemon") {
		t.Fatalf("error should explain a running daemon is required, got: %q", msg)
	}

	// Event commands still work from the local ring.
	ring.Append("web", "server.started")
	if _, nsErr := dispatch.JSON(context.Background(), wodCaller, "show event namespaces"); nsErr != nil {
		t.Fatalf("show event namespaces should succeed in web-only mode: %v", nsErr)
	}
}

// TestWithBGPDecodeInterceptsDecodeCommand verifies that withBGPDecode
// intercepts "show bgp decode <hex>" and produces real decoder output
// in-process, without forwarding to the inner dispatcher. Non-decode
// commands pass through to the inner dispatcher unchanged.
//
// VALIDATES: F5/AC-8 -- BGP decode works in both full-daemon and web-only modes.
// PREVENTS: decode command reaching a dispatcher that doesn't handle it.
func TestWithBGPDecodeInterceptsDecodeCommand(t *testing.T) {
	var innerCalled bool
	inner := zeweb.CommandDispatcher(func(_ context.Context, _ plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
		innerCalled = true
		return plugin.NewResponse(plugin.StatusDone, plugin.Map{"inner": cmd}), nil
	})
	dispatch := withBGPDecode(inner)

	// A valid KEEPALIVE hex produces real decoder output, not the inner
	// dispatcher -- but only when a BGP decoder is compiled in. With ze_bgp off
	// the registry seam is nil and the command is just another unknown command,
	// which is the honest behavior for a BGP-less binary.
	out, err := dispatch.JSON(context.Background(), wodCaller, "show bgp decode FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304")
	defer out.TransportComplete()
	require.NoError(t, err)
	if bgpDecodeLinked {
		assert.False(t, innerCalled, "decode must be handled in-process, not forwarded")
		assert.Equal(t, "keepalive", decodedMessageType(t, out.Output))
	} else {
		assert.True(t, innerCalled, "with BGP compiled out, decode must fall through to the dispatcher")
		assert.JSONEq(t, `{"inner":"show bgp decode FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304"}`, out.Output)
	}

	// A non-decode command passes through to the inner dispatcher.
	innerCalled = false
	out, err = dispatch.JSON(context.Background(), wodCaller, "show ping 1.1.1.1")
	defer out.TransportComplete()
	require.NoError(t, err)
	assert.True(t, innerCalled, "non-decode commands must reach the inner dispatcher")
	assert.JSONEq(t, `{"inner":"show ping 1.1.1.1"}`, out.Output)
}

// TestWithBGPDecodeNilInner verifies that withBGPDecode with a nil inner
// dispatcher still decodes in-process, and returns the friendly
// unavailable error for non-decode commands.
//
// VALIDATES: web-only mode (nil dispatcher) still supports decode.
// PREVENTS: nil-pointer panic when inner is nil.
func TestWithBGPDecodeNilInner(t *testing.T) {
	dispatch := withBGPDecode(nil)

	out, err := dispatch.JSON(context.Background(), wodCaller, "show bgp decode FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001304")
	defer out.TransportComplete()
	if bgpDecodeLinked {
		require.NoError(t, err)
		assert.Equal(t, "keepalive", decodedMessageType(t, out.Output))
	} else {
		// No decoder and no inner dispatcher: the same friendly
		// daemon-required error every other unservable command gets, not a
		// nil-pointer panic.
		require.Error(t, err)
		assert.Contains(t, err.Error(), "running daemon")
	}

	_, err = dispatch.JSON(context.Background(), wodCaller, "show ping 1.1.1.1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "running daemon")
}
