//go:build ze_web

package hub

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
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
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	t.Cleanup(server.Stop)
	ring := server.EventRing()
	dispatch := webOnlyDispatcher(server)

	// An absent command keeps the standalone interface's guidance.
	_, err = dispatch.JSON(context.Background(), wodCaller, "show standalone-unavailable")
	if err == nil {
		t.Fatal("expected an error for an unsupported operational command")
	}
	msg := err.Error()
	if strings.Contains(msg, "web-only mode") || strings.Contains(msg, "show standalone-unavailable") {
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
