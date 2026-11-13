// Design: docs/architecture/api/process-protocol.md -- authenticated instance identity.
package server

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type identityStartupSink struct{ *recordingSink }

func (sink *identityStartupSink) pluginName() string { return "forwarding-west" }

func TestStartupAcknowledgesConfiguredInstanceName(t *testing.T) {
	engine, plugin := newDriverPipe(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	sink := &identityStartupSink{&recordingSink{pc: engine}}
	ended := make(chan struct{})
	go func() { defer close(ended); runAbandonedHandshake(ctx, sink) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-ended:
		case <-time.After(5 * time.Second):
			t.Error("identity startup did not join")
		}
	})
	answer, err := plugin.CallRPC(ctx, rpc.MethodDeclareRegistration, &rpc.DeclareRegistrationInput{ProtocolVersion: rpc.ProtocolVersion})
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		PluginName string `json:"plugin-name"`
	}
	if err := json.Unmarshal(answer, &out); err != nil {
		t.Fatal(err)
	}
	if out.PluginName != "forwarding-west" {
		t.Fatalf("engine instance = %q, want configured alias forwarding-west", out.PluginName)
	}
}
