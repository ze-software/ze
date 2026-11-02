package server

import (
	"context"
	"io"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// TestStreamingPluginCommandDeclared proves a streaming registration carries
// the plugin command it forwards to, so a reader of the registry can name the
// plugin a stream depends on. Method: register one stream with a plugin
// command and one without, on an emptied registry, and read both back, then
// read a prefix nobody registered.
func TestStreamingPluginCommandDeclared(t *testing.T) {
	streamingHandlersMu.Lock()
	saved := streamingHandlers
	streamingHandlers = make(map[string]streamingRegistration)
	streamingHandlersMu.Unlock()
	t.Cleanup(func() {
		streamingHandlersMu.Lock()
		streamingHandlers = saved
		streamingHandlersMu.Unlock()
	})
	noStream := func(context.Context, *Server, io.Writer, string, command.ValidatedArgs) error { return nil }

	RegisterPluginStreamingHandler("monitor widget rate", OneShotSample, "show widget rate", noStream)
	RegisterStreamingHandler("monitor widget feed", OneShotStreamingOnly, noStream)

	if got, ok := StreamingPluginCommand("Monitor Widget Rate"); !ok || got != "show widget rate" {
		t.Errorf("forwarding stream: got %q, %v; want %q, true", got, ok, "show widget rate")
	}
	if got, ok := StreamingPluginCommand("monitor widget feed"); !ok || got != "" {
		t.Errorf("daemon-produced stream: got %q, %v; want \"\", true", got, ok)
	}
	if _, ok := StreamingPluginCommand("monitor nothing"); ok {
		t.Error("an unregistered prefix answered a registration")
	}
}

// TestRegisterPluginStreamingHandlerRefusesEmptyCommand proves a forwarding
// registration with no plugin command is a defect caught at init, never a
// stream that silently declares no dependency.
func TestRegisterPluginStreamingHandlerRefusesEmptyCommand(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("an empty plugin command was accepted")
		}
	}()
	RegisterPluginStreamingHandler("monitor widget rate", OneShotSample, " ",
		func(context.Context, *Server, io.Writer, string, command.ValidatedArgs) error { return nil })
}
