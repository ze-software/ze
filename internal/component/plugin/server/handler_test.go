package server

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
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

	handlerA := func(_ context.Context, _ *Server, _ io.Writer, _ string, _ []string) error { return nil }
	handlerB := func(_ context.Context, _ *Server, _ io.Writer, _ string, _ []string) error { return nil }

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
	RegisterStreamingHandler("monitor", func(_ context.Context, _ *Server, _ io.Writer, _ string, _ []string) error {
		matched = "monitor"
		return nil
	})
	RegisterStreamingHandler("monitor event", func(_ context.Context, _ *Server, _ io.Writer, _ string, _ []string) error {
		matched = "monitor event"
		return nil
	})

	h, args := streamingLookup(t, "monitor event include update")
	require.NotNil(t, h, "should match 'monitor event' prefix")
	_ = h(context.Background(), nil, nil, "", nil)
	require.Equal(t, "monitor event", matched, "longest prefix should win")
	require.Equal(t, []string{"include", "update"}, args)

	h, args = streamingLookup(t, "monitor something")
	require.NotNil(t, h, "should match 'monitor' prefix")
	_ = h(context.Background(), nil, nil, "", nil)
	require.Equal(t, "monitor", matched, "should match shorter prefix")
	require.Equal(t, []string{"something"}, args)
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

	handler := func(_ context.Context, _ *Server, _ io.Writer, _ string, _ []string) error { return nil }
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
