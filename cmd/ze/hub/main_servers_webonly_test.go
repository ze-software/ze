package hub

import (
	"strings"
	"testing"

	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

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
	_, err := dispatch("show ping 1.1.1.1", "alice", "127.0.0.1")
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
	if _, nsErr := dispatch("show event namespaces", "alice", "127.0.0.1"); nsErr != nil {
		t.Fatalf("show event namespaces should succeed in web-only mode: %v", nsErr)
	}
}
