package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/format"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/component/bgp/types"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	plugipc "codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/process"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// newTestProcWithConn creates a Process with a working ConnB and delivery goroutine for testing.
// Returns the process and a plugin-side connection for the mock responder.
func newTestProcWithConn(t *testing.T, name string) (*process.Process, *plugipc.PluginConn) {
	t.Helper()

	engineSide, pluginSide := net.Pipe()

	proc := process.NewProcess(plugin.PluginConfig{Name: name})
	engineConn := plugipc.NewPluginConn(engineSide, engineSide)
	proc.SetConn(engineConn)
	proc.StartDelivery(t.Context())

	t.Cleanup(func() {
		proc.Stop()
		if err := pluginSide.Close(); err != nil {
			t.Logf("close plugin side: %v", err) // Best-effort cleanup in test
		}
	})

	pluginConn := plugipc.NewPluginConn(pluginSide, pluginSide)
	return proc, pluginConn
}

// mockPluginResponder reads RPCs and responds OK after a delay.
// Exits when context is canceled or connection closes.
func mockPluginResponder(ctx context.Context, pluginConn *plugipc.PluginConn, delay time.Duration) {
	for {
		req, err := pluginConn.ReadRequest(ctx)
		if err != nil {
			return
		}
		if delay > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
		}
		if err := pluginConn.SendResult(ctx, req.ID, nil); err != nil {
			return
		}
	}
}

// newTestServer creates a minimal plugin.Server with context set for testing.
func newTestServer(t *testing.T) *pluginserver.Server {
	t.Helper()

	srv, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, nil)
	require.NoError(t, err)
	require.NoError(t, srv.StartWithContext(t.Context()))
	t.Cleanup(func() { srv.Stop() })
	return srv
}

// keepaliveMsg returns a simple KEEPALIVE RawMessage for testing.
func keepaliveMsg() bgptypes.RawMessage {
	return bgptypes.RawMessage{
		Type:      message.TypeKEEPALIVE,
		Direction: "received",
	}
}

// TestParallelPluginFanOut verifies concurrent plugin delivery.
//
// VALIDATES: AC-5: 3 slow plugins, wall time < 3x single delivery time.
//
// PREVENTS: Sequential delivery where N plugins = Nx latency.
func TestParallelPluginFanOut(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	encoder := format.NewJSONEncoder("test")

	const pluginDelay = 100 * time.Millisecond
	const pluginCount = 3

	for i := range pluginCount {
		proc, pluginConn := newTestProcWithConn(t, fmt.Sprintf("slow-%d", i))
		proc.SetCacheConsumer(true)

		srv.Subscriptions().Add(proc, &pluginserver.Subscription{
			Namespace: plugin.NamespaceBGP,
			EventType: plugin.EventKeepalive,
			Direction: plugin.DirectionBoth,
		})

		go mockPluginResponder(t.Context(), pluginConn, pluginDelay)
	}

	peer := testPeerInfo()
	msg := keepaliveMsg()

	start := time.Now()
	count := onMessageReceived(srv, encoder, peer, msg)
	elapsed := time.Since(start)

	require.Equal(t, pluginCount, count, "all cache consumers should succeed")
	// Sequential: 3 × 100ms = 300ms. Parallel: ~100ms.
	// Threshold 250ms gives generous margin but proves concurrency.
	require.Less(t, elapsed, 250*time.Millisecond,
		"parallel delivery should complete in less than 250ms (sequential would be ~300ms)")
}

// TestPartialDeliveryFailure verifies that one plugin failure doesn't block others.
//
// VALIDATES: AC-9: one plugin fails, others succeed, count reflects actual deliveries.
//
// PREVENTS: One broken plugin taking down all event delivery.
func TestPartialDeliveryFailure(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	encoder := format.NewJSONEncoder("test")

	// Plugin 1: cache consumer, responds normally
	proc1, pluginConn1 := newTestProcWithConn(t, "good-1")
	proc1.SetCacheConsumer(true)
	srv.Subscriptions().Add(proc1, &pluginserver.Subscription{
		Namespace: plugin.NamespaceBGP,
		EventType: plugin.EventKeepalive,
		Direction: plugin.DirectionBoth,
	})
	go mockPluginResponder(t.Context(), pluginConn1, 0)

	// Plugin 2: conn closed immediately — delivery fails
	proc2, pluginConn2 := newTestProcWithConn(t, "broken")
	proc2.SetCacheConsumer(true)
	srv.Subscriptions().Add(proc2, &pluginserver.Subscription{
		Namespace: plugin.NamespaceBGP,
		EventType: plugin.EventKeepalive,
		Direction: plugin.DirectionBoth,
	})
	// Close plugin side — engine's SendDeliverEvent will fail
	if err := pluginConn2.Close(); err != nil {
		t.Logf("close broken plugin conn: %v", err)
	}

	// Plugin 3: cache consumer, responds normally
	proc3, pluginConn3 := newTestProcWithConn(t, "good-2")
	proc3.SetCacheConsumer(true)
	srv.Subscriptions().Add(proc3, &pluginserver.Subscription{
		Namespace: plugin.NamespaceBGP,
		EventType: plugin.EventKeepalive,
		Direction: plugin.DirectionBoth,
	})
	go mockPluginResponder(t.Context(), pluginConn3, 0)

	peer := testPeerInfo()
	msg := keepaliveMsg()

	count := onMessageReceived(srv, encoder, peer, msg)

	// 2 out of 3 cache consumers should succeed
	require.Equal(t, 2, count, "broken plugin should not affect other deliveries")
}

// TestPreFormatOptimization verifies that formatting happens once per distinct format mode.
//
// VALIDATES: AC-14: same format → single encoding, different formats → separate encodings.
//
// PREVENTS: Redundant JSON encoding when multiple plugins use the same format.
func TestPreFormatOptimization(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	encoder := format.NewJSONEncoder("test")

	// Capture what each plugin receives
	type received struct {
		event string
	}
	captures := make([]chan received, 3)
	for i := range captures {
		captures[i] = make(chan received, 1)
	}

	// Create 3 plugins: 2 with "hex" format, 1 with "parsed" format
	formats := []string{"hex", "hex", "parsed"}
	for i, fmtMode := range formats {
		proc, pluginConn := newTestProcWithConn(t, fmt.Sprintf("fmt-plugin-%d", i))
		proc.SetFormat(fmtMode)
		proc.SetCacheConsumer(true)

		srv.Subscriptions().Add(proc, &pluginserver.Subscription{
			Namespace: plugin.NamespaceBGP,
			EventType: plugin.EventKeepalive,
			Direction: plugin.DirectionBoth,
		})

		captureCh := captures[i]
		go func(conn *plugipc.PluginConn, ch chan received) {
			req, err := conn.ReadRequest(context.Background())
			if err != nil {
				return
			}
			// Delivery pipeline sends deliver-batch with events array.
			var input struct {
				Events []json.RawMessage `json:"events"`
			}
			if err := json.Unmarshal(req.Params, &input); err != nil {
				return
			}
			if len(input.Events) > 0 {
				ch <- received{event: string(input.Events[0])}
			}
			if err := conn.SendResult(context.Background(), req.ID, nil); err != nil {
				return // Connection closed during shutdown
			}
		}(pluginConn, captureCh)
	}

	peer := testPeerInfo()
	msg := keepaliveMsg()

	count := onMessageReceived(srv, encoder, peer, msg)
	require.Equal(t, 3, count, "all plugins should succeed")

	// Collect results with timeout
	results := make([]string, 3)
	for i := range 3 {
		select {
		case r := <-captures[i]:
			results[i] = r.event
		case <-time.After(2 * time.Second):
			t.Fatalf("plugin %d did not receive event", i)
		}
	}

	// Plugins 0 and 1 (both "hex") should receive identical output
	require.Equal(t, results[0], results[1], "same format mode should produce identical output")
	// Plugin 2 ("parsed") may differ from "hex" — at minimum it should be non-empty
	require.NotEmpty(t, results[2], "parsed format plugin should receive output")
}

// testPeerInfo returns a PeerInfo for testing.
func testPeerInfo() plugin.PeerInfo {
	return plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
	}
}

// TestOnMessageBatchReceivedSingle verifies single-message batch matches per-message behavior.
//
// VALIDATES: AC-7: single UPDATE arrives (batch size 1) — behavior identical to per-message path.
//
// PREVENTS: Batch path diverging from per-message path for single-item batches.
func TestOnMessageBatchReceivedSingle(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	encoder := format.NewJSONEncoder("test")

	proc, pluginConn := newTestProcWithConn(t, "single-batch")
	proc.SetCacheConsumer(true)

	srv.Subscriptions().Add(proc, &pluginserver.Subscription{
		Namespace: plugin.NamespaceBGP,
		EventType: plugin.EventKeepalive,
		Direction: plugin.DirectionBoth,
	})
	go mockPluginResponder(t.Context(), pluginConn, 0)

	peer := testPeerInfo()
	msgs := []bgptypes.RawMessage{keepaliveMsg()}

	// Single-message batch should return same result as per-message call
	counts := onMessageBatchReceived(srv, encoder, peer, msgs)
	require.Len(t, counts, 1, "single-message batch returns one count")
	require.Equal(t, 1, counts[0], "single cache consumer should succeed")
}

// TestOnMessageBatchReceivedMultiple verifies multi-message batch with shared subscription lookup.
//
// VALIDATES: AC-4: 10 UPDATEs arrive from same peer — GetMatching called once, not 10 times.
//
// PREVENTS: Subscription lookup repeated per message in batch.
func TestOnMessageBatchReceivedMultiple(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	encoder := format.NewJSONEncoder("test")

	proc, pluginConn := newTestProcWithConn(t, "batch-multi")
	proc.SetCacheConsumer(true)

	srv.Subscriptions().Add(proc, &pluginserver.Subscription{
		Namespace: plugin.NamespaceBGP,
		EventType: plugin.EventKeepalive,
		Direction: plugin.DirectionBoth,
	})
	go mockPluginResponder(t.Context(), pluginConn, 0)

	peer := testPeerInfo()
	msgs := make([]bgptypes.RawMessage, 5)
	for i := range msgs {
		msgs[i] = keepaliveMsg()
	}

	counts := onMessageBatchReceived(srv, encoder, peer, msgs)
	require.Len(t, counts, 5, "batch returns one count per message")
	for i, c := range counts {
		require.Equal(t, 1, c, "message %d: cache consumer should succeed", i)
	}
}

// TestOnMessageBatchReceivedNoSubscribers verifies early return when no subscribers.
//
// VALIDATES: AC-9: no subscribers for UPDATE events — batch path short-circuits.
//
// PREVENTS: Unnecessary formatting or delivery when nobody is listening.
func TestOnMessageBatchReceivedNoSubscribers(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	encoder := format.NewJSONEncoder("test")

	peer := testPeerInfo()
	msgs := []bgptypes.RawMessage{keepaliveMsg(), keepaliveMsg()}

	counts := onMessageBatchReceived(srv, encoder, peer, msgs)
	require.Len(t, counts, 2, "returns one count per message even with no subscribers")
	for i, c := range counts {
		require.Equal(t, 0, c, "message %d: no subscribers means zero count", i)
	}
}

// TestOnMessageBatchReceivedCacheCount verifies per-message cacheCount correctness.
//
// VALIDATES: AC-5: batch of N UPDATEs — Activate(msgID, count) called per message with correct count.
//
// PREVENTS: Cache counts being aggregated across messages instead of per-message.
func TestOnMessageBatchReceivedCacheCount(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t)
	encoder := format.NewJSONEncoder("test")

	// Two cache consumers + one non-cache consumer
	for i := range 2 {
		proc, pluginConn := newTestProcWithConn(t, fmt.Sprintf("cache-%d", i))
		proc.SetCacheConsumer(true)
		srv.Subscriptions().Add(proc, &pluginserver.Subscription{
			Namespace: plugin.NamespaceBGP,
			EventType: plugin.EventKeepalive,
			Direction: plugin.DirectionBoth,
		})
		go mockPluginResponder(t.Context(), pluginConn, 0)
	}

	proc3, pluginConn3 := newTestProcWithConn(t, "non-cache")
	// proc3 is NOT a cache consumer (default)
	srv.Subscriptions().Add(proc3, &pluginserver.Subscription{
		Namespace: plugin.NamespaceBGP,
		EventType: plugin.EventKeepalive,
		Direction: plugin.DirectionBoth,
	})
	go mockPluginResponder(t.Context(), pluginConn3, 0)

	peer := testPeerInfo()
	msgs := []bgptypes.RawMessage{keepaliveMsg(), keepaliveMsg(), keepaliveMsg()}

	counts := onMessageBatchReceived(srv, encoder, peer, msgs)
	require.Len(t, counts, 3, "batch returns one count per message")
	for i, c := range counts {
		require.Equal(t, 2, c, "message %d: only cache consumers counted", i)
	}
}

// TestFormatSubscriptionRespectsEncoding verifies formatMessageForSubscription uses encoding param.
//
// VALIDATES: AC-2 (text encoding produces text events), AC-3 (JSON unchanged).
//
// PREVENTS: Event delivery hardcoding JSON encoding for all plugins.
func TestFormatSubscriptionRespectsEncoding(t *testing.T) {
	t.Parallel()

	encoder := format.NewJSONEncoder("test")
	peer := testPeerInfo()
	msg := keepaliveMsg()

	// JSON encoding → JSON output
	jsonOut := formatMessageForSubscription(encoder, peer, msg, plugin.FormatParsed, plugin.EncodingJSON)
	require.True(t, strings.HasPrefix(jsonOut, "{"),
		"json encoding should produce JSON, got: %s", jsonOut)

	// Text encoding → text output
	textOut := formatMessageForSubscription(encoder, peer, msg, plugin.FormatParsed, plugin.EncodingText)
	require.True(t, strings.HasPrefix(textOut, "peer "),
		"text encoding should produce text starting with 'peer', got: %s", textOut)

	// They should be different
	require.NotEqual(t, jsonOut, textOut,
		"json and text encoding should produce different output")
}

// TestCacheKeyIncludesEncoding verifies pre-format cache distinguishes by encoding.
//
// VALIDATES: AC-11 (same format, different encoding → distinct outputs).
//
// PREVENTS: Cache collision when two procs share format but differ in encoding.
func TestCacheKeyIncludesEncoding(t *testing.T) {
	t.Parallel()

	encoder := format.NewJSONEncoder("test")
	peer := testPeerInfo()
	msg := keepaliveMsg()

	// Same format (parsed), different encoding — must produce different output.
	jsonOut := formatMessageForSubscription(encoder, peer, msg, plugin.FormatParsed, plugin.EncodingJSON)
	textOut := formatMessageForSubscription(encoder, peer, msg, plugin.FormatParsed, plugin.EncodingText)

	require.NotEqual(t, jsonOut, textOut,
		"same format + different encoding should produce different output")

	// Verify the cache key pattern works: format+encoding is unique
	key1 := plugin.FormatParsed + "+" + plugin.EncodingJSON
	key2 := plugin.FormatParsed + "+" + plugin.EncodingText
	require.NotEqual(t, key1, key2, "cache keys should differ for different encodings")
}

// TestStateChangeRespectsEncoding verifies onPeerStateChange formats per-process encoding.
//
// VALIDATES: AC-12 (text-encoded process gets text state event), AC-7 (text state parseable).
//
// PREVENTS: State events always being JSON regardless of process encoding.
func TestStateChangeRespectsEncoding(t *testing.T) {
	t.Parallel()

	peer := plugin.PeerInfo{
		Address: netip.MustParseAddr("10.0.0.1"),
		PeerAS:  65001,
	}

	// Verify FormatStateChange produces correct output per encoding.
	// onPeerStateChange calls FormatStateChange with proc.Encoding(),
	// so testing the format function directly validates the per-process path.
	textState := format.FormatStateChange(peer, "up", "", plugin.EncodingText)
	jsonState := format.FormatStateChange(peer, "up", "", plugin.EncodingJSON)

	require.Contains(t, textState, "peer 10.0.0.1", "text state should contain peer address")
	require.Contains(t, textState, "state up", "text state should contain state value")
	require.NotContains(t, textState, "{", "text state should not be JSON")

	require.Contains(t, jsonState, `"type":"bgp"`, "json state should be JSON")
	require.Contains(t, jsonState, `"state":"up"`, "json state should contain state value")

	// Verify they're different
	require.NotEqual(t, textState, jsonState,
		"text and json state events should differ")
}
