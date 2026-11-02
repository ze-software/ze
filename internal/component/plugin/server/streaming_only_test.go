package server

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
)

// withStreamingRegistry gives the test an empty streaming registry and puts
// the package's back when it ends.
func withStreamingRegistry(t *testing.T) {
	t.Helper()
	streamingHandlersMu.Lock()
	saved := streamingHandlers
	streamingHandlers = make(map[string]streamingRegistration)
	streamingHandlersMu.Unlock()
	t.Cleanup(func() {
		streamingHandlersMu.Lock()
		streamingHandlers = saved
		streamingHandlersMu.Unlock()
	})
}

// TestStreamingOnlyCommandRefusesOneShot proves the dispatcher every one-shot
// surface uses refuses a command whose streaming registration declared
// OneShotStreamingOnly, naming the terminal command that streams it, and
// never runs its RPC handler, while a OneShotSample command still runs its
// handler. Method: two streaming registrations, one of each declaration, each
// with a builtin of the same path whose handler marks that it ran.
func TestStreamingOnlyCommandRefusesOneShot(t *testing.T) {
	withStreamingRegistry(t)
	noStream := func(context.Context, *Server, io.Writer, string, command.ValidatedArgs) error { return nil }
	RegisterStreamingHandler("monitor feed", OneShotStreamingOnly, noStream)
	RegisterStreamingHandler("monitor rate", OneShotSample, noStream)

	d := NewDispatcher()
	var feedRan, rateRan bool
	if err := d.Register("monitor feed", handlerMarking(&feedRan), "feed"); err != nil {
		t.Fatalf("register monitor feed: %v", err)
	}
	if err := d.Register("monitor rate", handlerMarking(&rateRan), "rate"); err != nil {
		t.Fatalf("register monitor rate: %v", err)
	}

	resp, err := d.Dispatch(&CommandContext{}, "monitor feed")
	refusal, ok := errors.AsType[*StreamingOnlyError](err)
	if !ok {
		t.Fatalf("monitor feed one-shot: got error %v, want a StreamingOnlyError", err)
	}
	if refusal.Path != "monitor feed" {
		t.Errorf("refusal path %q, want monitor feed", refusal.Path)
	}
	if !strings.Contains(err.Error(), "'ze monitor feed'") {
		t.Errorf("refusal %q does not name the command that streams it", err)
	}
	if resp == nil || resp.Error != err.Error() {
		t.Errorf("response %+v does not carry the refusal", resp)
	}
	if feedRan {
		t.Error("the streaming-only command's RPC handler ran: its reply would carry no data")
	}

	if _, err := d.Dispatch(&CommandContext{}, "monitor rate"); err != nil {
		t.Fatalf("monitor rate one-shot: %v", err)
	}
	if !rateRan {
		t.Error("a one-shot sample command did not run its handler")
	}
}

// TestStreamingOnlyRPCNeverAnswers proves the shared RPC handler of a
// streaming-only command answers an error, so a route that skipped the
// refusal still cannot hand back a reply shaped like success.
func TestStreamingOnlyRPCNeverAnswers(t *testing.T) {
	resp, err := StreamingOnlyRPC(&CommandContext{}, command.ValidatedArgs{})
	if !errors.Is(err, errStreamingOnlyReached) {
		t.Fatalf("got error %v, want errStreamingOnlyReached", err)
	}
	if resp != nil {
		t.Errorf("got response %+v, want none", resp)
	}
}
