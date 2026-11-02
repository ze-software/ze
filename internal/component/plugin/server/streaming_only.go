// Design: docs/architecture/api/commands.md — one-shot answer of a streaming command
// Related: handler.go — streaming registrations and their declared one-shot answer

package server

import (
	"errors"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	plugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// StreamingOnlyError is the refusal a route that answers once (MCP, REST,
// gRPC, the web admin console, a plugin's dispatch-command, a wire call) gets
// for a command whose streaming registration declared OneShotStreamingOnly.
// Its stream is a sequence of events, so no single reply holds its data, and
// a reply shaped like success would carry nothing.
type StreamingOnlyError struct {
	// Path is the streaming command's registered prefix.
	Path string
	// Input is the command as it was typed, which the fix repeats.
	Input string
}

func (e *StreamingOnlyError) Error() string {
	var tb textbuf.Buffer
	tb.Quoted(e.Path).Str(" shows events as they happen, and this interface answers only once, so it has nothing to return")
	tb.Str("; run it in a terminal: 'ze ").Str(e.Input).Str("', or type it in an SSH session to the router")
	return tb.String()
}

// streamingOnlyRefusal answers the refusal the command at path owes on a
// one-shot route when its streaming registration declared
// OneShotStreamingOnly, and nil for every other command. The registration is
// the only declaration: no command is named here.
func streamingOnlyRefusal(path, input string) error {
	oneShot, ok := StreamingOneShot(path)
	if !ok {
		return nil
	}
	if oneShot != OneShotStreamingOnly {
		return nil
	}
	input = strings.TrimSpace(input)
	if input == "" {
		input = path
	}
	return &StreamingOnlyError{Path: path, Input: input}
}

// errStreamingOnlyReached reports a one-shot route that ran the RPC handler
// of a streaming-only command without asking streamingOnlyRefusal first.
var errStreamingOnlyReached = errors.New("BUG: a streaming-only command reached its one-shot handler; the route must refuse it first (streamingOnlyRefusal)")

// StreamingOnlyRPC is the RPC handler of a command whose streaming
// registration declared OneShotStreamingOnly. Every one-shot route refuses
// such a command before a handler runs (Dispatcher.Dispatch, wrapHandler), so
// this answers only when a route skipped that refusal, and it answers an
// error, never a reply.
func StreamingOnlyRPC(_ *CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {
	return nil, errStreamingOnlyReached
}
