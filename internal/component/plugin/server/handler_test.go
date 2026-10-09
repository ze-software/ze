package server

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/command"
)

// VALIDATES: RegisterStreamingHandler stores handlers by prefix.
// PREVENTS: Singleton streaming handler only supporting one streaming command.
func TestStreamingHandlerRegistry(t *testing.T) {
	// Reset registry state for test isolation.
	streamingHandlersMu.Lock()
	saved := streamingHandlers
	streamingHandlers = make(map[string]StreamingHandler)
	streamingHandlersMu.Unlock()
	defer func() {
		streamingHandlersMu.Lock()
		streamingHandlers = saved
		streamingHandlersMu.Unlock()
	}()

	handlerA := func(_ context.Context, _ *Server, _ io.Writer, _ string, _ command.ValidatedArgs) error { return nil }
	handlerB := func(_ context.Context, _ *Server, _ io.Writer, _ string, _ command.ValidatedArgs) error { return nil }

	RegisterStreamingHandler("monitor event", handlerA)
	RegisterStreamingHandler("monitor bgp", handlerB)

	h, args := streamingLookup(t, "monitor event peer 10.0.0.1")
	require.NotNil(t, h, "should match 'monitor event' prefix")
	require.Equal(t, []string{"peer", "10.0.0.1"}, args)

	h, args = streamingLookup(t, "monitor bgp")
	require.NotNil(t, h, "should match 'monitor bgp' prefix")
	require.Nil(t, args, "no args after prefix")

	h, _ = streamingLookup(t, "unknown command")
	require.Nil(t, h, "should return nil for unregistered prefix")
}

// VALIDATES: GetStreamingHandlerForCommand picks the longest matching prefix.
// PREVENTS: Short prefix stealing commands meant for a longer prefix.
func TestStreamingHandlerPrefixMatch(t *testing.T) {
	streamingHandlersMu.Lock()
	saved := streamingHandlers
	streamingHandlers = make(map[string]StreamingHandler)
	streamingHandlersMu.Unlock()
	defer func() {
		streamingHandlersMu.Lock()
		streamingHandlers = saved
		streamingHandlersMu.Unlock()
	}()

	var matched string
	RegisterStreamingHandler("monitor", func(_ context.Context, _ *Server, _ io.Writer, _ string, _ command.ValidatedArgs) error {
		matched = "monitor"
		return nil
	})
	RegisterStreamingHandler("monitor event", func(_ context.Context, _ *Server, _ io.Writer, _ string, _ command.ValidatedArgs) error {
		matched = "monitor event"
		return nil
	})

	h, validated, err := GetStreamingHandlerForCommand("monitor event include update")
	require.NoError(t, err)
	require.NotNil(t, h, "should match 'monitor event' prefix")
	require.NoError(t, h(context.Background(), nil, nil, "", validated))
	require.Equal(t, "monitor event", matched, "longest prefix should win")
	require.Equal(t, []string{"include", "update"}, validated.Tokens())

	h, validated, err = GetStreamingHandlerForCommand("monitor something")
	require.NoError(t, err)
	require.NotNil(t, h, "should match 'monitor' prefix")
	require.NoError(t, h(context.Background(), nil, nil, "", validated))
	require.Equal(t, "monitor", matched, "should match shorter prefix")
	require.Equal(t, []string{"something"}, validated.Tokens())
}

// VALIDATES: IsStreamingCommand checks all registered prefixes.
// PREVENTS: Hardcoded prefix check missing newly registered streaming commands.
func TestIsStreamingCommand(t *testing.T) {
	streamingHandlersMu.Lock()
	saved := streamingHandlers
	streamingHandlers = make(map[string]StreamingHandler)
	streamingHandlersMu.Unlock()
	defer func() {
		streamingHandlersMu.Lock()
		streamingHandlers = saved
		streamingHandlersMu.Unlock()
	}()

	handler := func(_ context.Context, _ *Server, _ io.Writer, _ string, _ command.ValidatedArgs) error { return nil }
	RegisterStreamingHandler("monitor event", handler)

	require.True(t, IsStreamingCommand("monitor event"))
	require.True(t, IsStreamingCommand("monitor event peer 10.0.0.1"))
	require.True(t, IsStreamingCommand("MONITOR EVENT"), "should be case-insensitive")
	require.False(t, IsStreamingCommand("bgp peer list"))
	require.False(t, IsStreamingCommand("monitorvent"), "no space should not match")
}

// streamingLookup is GetStreamingHandlerForCommand for a test that expects the
// arguments to be accepted: it fails the test on a refusal and answers the
// judged tokens.
func streamingLookup(t *testing.T, input string) (StreamingHandler, []string) {
	t.Helper()
	handler, validated, err := GetStreamingHandlerForCommand(input)
	if err != nil {
		t.Fatalf("GetStreamingHandlerForCommand(%q): %v", input, err)
	}
	return handler, validated.Tokens()
}

// VALIDATES: AC-3 for the author-facing handler types. A registered handler
// receives exactly the value ValidateArgs returned: the judged tokens and the
// leaf a lone positional token filled, which a raw token slice cannot carry.
// PREVENTS: a handler type declared with, or invoked with, raw tokens.
// METHOD: one subtest per handler type, each registering at a command the
// model declares a leaf for and invoking through its real lookup.
func TestHandlerTypesTakeValidatedArguments(t *testing.T) {
	t.Run("StreamingHandler", func(t *testing.T) {
		streamingHandlersMu.Lock()
		saved := streamingHandlers
		streamingHandlers = make(map[string]StreamingHandler)
		streamingHandlersMu.Unlock()
		t.Cleanup(func() {
			streamingHandlersMu.Lock()
			streamingHandlers = saved
			streamingHandlersMu.Unlock()
		})
		var received command.ValidatedArgs
		RegisterStreamingHandler(modelPath, func(_ context.Context, _ *Server, _ io.Writer, _ string, args command.ValidatedArgs) error {
			received = args
			return nil
		})

		handler, validated, err := GetStreamingHandlerForCommand(modelPath + " " + nameAtBound)
		require.NoError(t, err)
		require.NotNil(t, handler)
		require.NoError(t, handler(context.Background(), nil, io.Discard, "", validated))
		require.Equal(t, []string{nameAtBound}, received.Tokens())
		name, found := received.Positional("name")
		require.True(t, found, "the lone positional token did not reach the handler bound to its leaf")
		require.Equal(t, nameAtBound, name)
	})
}
