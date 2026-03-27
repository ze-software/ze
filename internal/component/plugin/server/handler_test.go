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

	RegisterStreamingHandler("event monitor", handlerA)
	RegisterStreamingHandler("bgp monitor", handlerB)

	h, args := GetStreamingHandlerForCommand("event monitor peer 10.0.0.1")
	require.NotNil(t, h, "should match 'event monitor' prefix")
	require.Equal(t, []string{"peer", "10.0.0.1"}, args)

	h, args = GetStreamingHandlerForCommand("bgp monitor")
	require.NotNil(t, h, "should match 'bgp monitor' prefix")
	require.Nil(t, args, "no args after prefix")

	h, _ = GetStreamingHandlerForCommand("unknown command")
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
	RegisterStreamingHandler("event", func(_ context.Context, _ *Server, _ io.Writer, _ string, _ []string) error {
		matched = "event"
		return nil
	})
	RegisterStreamingHandler("event monitor", func(_ context.Context, _ *Server, _ io.Writer, _ string, _ []string) error {
		matched = "event monitor"
		return nil
	})

	h, args := GetStreamingHandlerForCommand("event monitor include update")
	require.NotNil(t, h, "should match 'event monitor' prefix")
	_ = h(context.Background(), nil, nil, "", nil)
	require.Equal(t, "event monitor", matched, "longest prefix should win")
	require.Equal(t, []string{"include", "update"}, args)

	h, args = GetStreamingHandlerForCommand("event list")
	require.NotNil(t, h, "should match 'event' prefix")
	_ = h(context.Background(), nil, nil, "", nil)
	require.Equal(t, "event", matched, "should match shorter prefix")
	require.Equal(t, []string{"list"}, args)
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
	RegisterStreamingHandler("event monitor", handler)

	require.True(t, IsStreamingCommand("event monitor"))
	require.True(t, IsStreamingCommand("event monitor peer 10.0.0.1"))
	require.True(t, IsStreamingCommand("EVENT MONITOR"), "should be case-insensitive")
	require.False(t, IsStreamingCommand("bgp peer list"))
	require.False(t, IsStreamingCommand("eventmonitor"), "no space should not match")
}
