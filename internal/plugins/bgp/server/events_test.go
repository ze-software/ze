package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/plugin"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/format"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/message"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
)

// newTestProcWithConn creates a Process with a working ConnB and delivery goroutine for testing.
// Returns the process and a plugin-side connection for the mock responder.
func newTestProcWithConn(t *testing.T, name string) (*plugin.Process, *plugin.PluginConn) {
	t.Helper()

	engineSide, pluginSide := net.Pipe()

	proc := plugin.NewProcess(plugin.PluginConfig{Name: name})
	engineConn := plugin.NewPluginConn(engineSide, engineSide)
	proc.SetConnB(engineConn)
	proc.StartDelivery(t.Context())

	t.Cleanup(func() {
		proc.Stop()
		if err := pluginSide.Close(); err != nil {
			t.Logf("close plugin side: %v", err) // Best-effort cleanup in test
		}
	})

	pluginConn := plugin.NewPluginConn(pluginSide, pluginSide)
	return proc, pluginConn
}

// mockPluginResponder reads RPCs and responds OK after a delay.
// Exits when context is canceled or connection closes.
func mockPluginResponder(ctx context.Context, pluginConn *plugin.PluginConn, delay time.Duration) {
	for {
		req, err := pluginConn.ReadRequest(ctx)
		if err != nil {
			return
		}
		if delay > 0 {
			time.Sleep(delay)
		}
		if err := pluginConn.SendResult(ctx, req.ID, nil); err != nil {
			return
		}
	}
}

// newTestServer creates a minimal plugin.Server with context set for testing.
func newTestServer(t *testing.T) *plugin.Server {
	t.Helper()

	srv := plugin.NewServer(&plugin.ServerConfig{}, nil)
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

		srv.Subscriptions().Add(proc, &plugin.Subscription{
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
	srv.Subscriptions().Add(proc1, &plugin.Subscription{
		Namespace: plugin.NamespaceBGP,
		EventType: plugin.EventKeepalive,
		Direction: plugin.DirectionBoth,
	})
	go mockPluginResponder(t.Context(), pluginConn1, 0)

	// Plugin 2: conn closed immediately — delivery fails
	proc2, pluginConn2 := newTestProcWithConn(t, "broken")
	proc2.SetCacheConsumer(true)
	srv.Subscriptions().Add(proc2, &plugin.Subscription{
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
	srv.Subscriptions().Add(proc3, &plugin.Subscription{
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

		srv.Subscriptions().Add(proc, &plugin.Subscription{
			Namespace: plugin.NamespaceBGP,
			EventType: plugin.EventKeepalive,
			Direction: plugin.DirectionBoth,
		})

		captureCh := captures[i]
		go func(conn *plugin.PluginConn, ch chan received) {
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
