package reactor

import (
	"context"
	"log/slog"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/test/sim"
)

// TestAPISyncNoProcesses verifies no wait when no processes.
//
// VALIDATES: WaitForAPIReady returns immediately with zero processes.
// PREVENTS: Unnecessary delay when no API processes configured.
func TestAPISyncNoProcesses(t *testing.T) {
	r := New(&Config{})
	// Don't call SetAPIProcessCount - defaults to 0

	done := make(chan struct{})
	go func() {
		r.WaitForAPIReady()
		close(done)
	}()

	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "WaitForAPIReady should return immediately with zero processes")
}

// TestAPISyncSingleProcess verifies waiting for one process.
//
// VALIDATES: WaitForAPIReady waits for single ready signal.
// PREVENTS: Proceeding before process is ready.
func TestAPISyncSingleProcess(t *testing.T) {
	r := New(&Config{})
	r.SetAPIProcessCount(1)
	r.apiTimeout = 2 * time.Second

	done := make(chan struct{})

	// WaitForAPIReady should NOT return before we signal.
	go func() {
		r.WaitForAPIReady()
		close(done)
	}()

	// Verify it does NOT complete before the signal.
	require.Never(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 50*time.Millisecond, time.Millisecond, "WaitForAPIReady should block until signal")

	// Now signal ready.
	r.SignalAPIReady()

	// Verify it completes after the signal.
	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "WaitForAPIReady should return after signal")
}

// TestAPISyncMultipleProcesses verifies waiting for all processes.
//
// VALIDATES: WaitForAPIReady waits for all N ready signals.
// PREVENTS: Proceeding before all processes are ready.
func TestAPISyncMultipleProcesses(t *testing.T) {
	r := New(&Config{})
	r.SetAPIProcessCount(3)
	r.apiTimeout = 2 * time.Second

	done := make(chan struct{})
	go func() {
		r.WaitForAPIReady()
		close(done)
	}()

	// After 1 signal, should still be blocked.
	r.SignalAPIReady()
	require.Never(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 50*time.Millisecond, time.Millisecond, "WaitForAPIReady should block after 1 of 3 signals")

	// After 2 signals, should still be blocked.
	r.SignalAPIReady()
	require.Never(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 50*time.Millisecond, time.Millisecond, "WaitForAPIReady should block after 2 of 3 signals")

	// After 3rd signal, should unblock.
	r.SignalAPIReady()
	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "WaitForAPIReady should return after all 3 signals")
}

// TestAPISyncTimeout verifies timeout when process doesn't respond.
//
// VALIDATES: WaitForAPIReady times out and proceeds.
// PREVENTS: Hanging forever when process is stuck.
func TestAPISyncTimeout(t *testing.T) {
	r := New(&Config{})
	r.SetAPIProcessCount(2)
	r.apiTimeout = 100 * time.Millisecond

	// Only send 1 ready, expect 2 -- must timeout.
	r.SignalAPIReady()

	done := make(chan struct{})
	go func() {
		r.WaitForAPIReady()
		close(done)
	}()

	// Should NOT return immediately (still waiting for 2nd signal).
	require.Never(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 50*time.Millisecond, time.Millisecond, "WaitForAPIReady should not return before timeout")

	// Should eventually return after the 100ms timeout.
	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "WaitForAPIReady should return after timeout")
}

// TestAPISyncImmediateReady verifies quick return when already ready.
//
// VALIDATES: WaitForAPIReady returns immediately if already ready.
// PREVENTS: Unnecessary delay on repeated calls.
func TestAPISyncImmediateReady(t *testing.T) {
	r := New(&Config{})
	r.SetAPIProcessCount(1)

	// Signal before waiting.
	r.SignalAPIReady()

	done := make(chan struct{})
	go func() {
		r.WaitForAPIReady()
		close(done)
	}()

	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "WaitForAPIReady should return immediately when already ready")
}

// TestAPISyncMultipleCalls verifies idempotency.
//
// VALIDATES: Multiple WaitForAPIReady calls don't block.
// PREVENTS: Deadlock on repeated calls.
func TestAPISyncMultipleCalls(t *testing.T) {
	r := New(&Config{})
	r.SetAPIProcessCount(1)
	r.SignalAPIReady()

	// First call.
	r.WaitForAPIReady()

	// Second call should return immediately.
	done := make(chan struct{})
	go func() {
		r.WaitForAPIReady()
		close(done)
	}()

	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "second WaitForAPIReady call should return immediately")
}

// TestAPISyncConcurrent verifies thread safety.
//
// VALIDATES: Concurrent SignalAPIReady calls are safe.
// PREVENTS: Race conditions in API sync signaling.
func TestAPISyncConcurrent(t *testing.T) {
	r := New(&Config{})
	r.SetAPIProcessCount(10)
	r.apiTimeout = 2 * time.Second

	// Use a WaitGroup to start all goroutines simultaneously.
	var ready sync.WaitGroup
	ready.Add(10)

	var started sync.WaitGroup
	started.Add(10)

	// Spawn 10 goroutines signaling concurrently.
	for range 10 {
		go func() {
			started.Done()
			ready.Wait() // All goroutines start signaling at the same time.
			r.SignalAPIReady()
		}()
	}

	// Wait for all goroutines to be ready, then release them.
	started.Wait()
	ready.Add(-10) // Release all.

	done := make(chan struct{})
	var readyCount int32
	go func() {
		r.WaitForAPIReady()
		readyCount = r.readyCount.Load()
		close(done)
	}()

	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 2*time.Second, time.Millisecond, "WaitForAPIReady should return after all concurrent signals")

	// Verify all signals were received.
	require.Equal(t, int32(10), readyCount, "all 10 ready signals should be received")
}

// --- SignalPeerAPIReady: routing a per-peer ready signal ---
//
// These tests drive SignalPeerAPIReady, the entry point the "peer <addr> plugin
// session ready" command lands on (bgp/plugins/cmd/peer/session.go:20), and
// assert the peer-visible effect: waitForAPISync (peer.go:423) is released.
// They do not test findPeerByAddr directly -- the bug was never in the helper,
// it was that the caller never reached it (ai/rules/fail-closed-guards.md,
// "Test corollary").

// warnRecorder is a slog.Handler that records the "peer" attribute of Warn+
// records. It lets a test assert that a miss is LOUD without pinning the exact
// log wording, which would be brittle.
type warnRecorder struct {
	mu    sync.Mutex
	peers []string
	msgs  []string
}

func (h *warnRecorder) Enabled(_ context.Context, l slog.Level) bool { return l >= slog.LevelWarn }

//nolint:gocritic // hugeParam: slog.Handler's interface takes slog.Record by value.
func (h *warnRecorder) Handle(_ context.Context, rec slog.Record) error {
	h.mu.Lock()
	h.msgs = append(h.msgs, rec.Message)
	h.mu.Unlock()
	rec.Attrs(func(a slog.Attr) bool {
		if a.Key == "peer" {
			h.mu.Lock()
			h.peers = append(h.peers, a.Value.String())
			h.mu.Unlock()
		}
		return true
	})
	return nil
}

func (h *warnRecorder) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *warnRecorder) WithGroup(string) slog.Handler      { return h }

func (h *warnRecorder) warnedPeers() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.peers...)
}

// messages returns the recorded Warn+ log messages (a copy). Used by tests that
// assert a miss spoke but carry no "peer" attribute (e.g. SetPluginServerAny).
func (h *warnRecorder) messages() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.msgs...)
}

// armAPISyncPeer adds a peer on the given port, arms its per-session API sync for
// one ready signal, and starts waitForAPISync in the background. The returned
// channel closes ONLY when the ready signal actually reaches this peer.
//
// The peer runs on a sim.FakeClock, whose After() never fires (sim.go:102). That
// removes the 2s timeout escape hatch from waitForAPISync: the wait can end for
// exactly one reason, the signal arriving. Without this the test would pass on a
// timeout and assert nothing.
func armAPISyncPeer(t *testing.T, r *Reactor, addr netip.Addr, port uint16) <-chan struct{} {
	t.Helper()

	settings := NewPeerSettings(addr, 65000, 65001, 0x01020304)
	settings.Port = port
	require.NoError(t, r.AddPeer(settings))

	r.mu.RLock()
	peer := r.peers[settings.PeerKey()]
	r.mu.RUnlock()
	require.NotNil(t, peer, "peer must be stored under its own address:port key")

	peer.SetClock(sim.NewFakeClock(time.Now()))
	peer.ResetAPISync(1)

	synced := make(chan struct{})
	go func() {
		peer.waitForAPISync(2 * time.Second)
		close(synced)
	}()
	// Release the waiter so the goroutine never outlives the test when the
	// signal legitimately does not arrive (unknown-peer case).
	t.Cleanup(peer.SignalAPIReady)

	return synced
}

func chanClosed(ch <-chan struct{}) func() bool {
	return func() bool {
		select {
		case <-ch:
			return true
		default:
			return false
		}
	}
}

// TestSignalPeerAPIReadyNonDefaultPort verifies a peer configured on a
// non-standard BGP port receives its ready signal when the emitter sends a bare IP.
//
// VALIDATES: SignalPeerAPIReady routes "peer <bare-ip> plugin session ready" to a
// peer whose map key is <ip>:1179. parsePeerAddrToKey must assume DefaultBGPPort
// for a bare IP (reactor_peers.go:28), and PeerSettings.PeerKey uses the real port
// (peersettings.go:521), so the direct r.peers[key] read misses for every non-179
// peer. Emitters do send a bare IP: bgp-rib dispatches "request peer <addr> plugin
// session ready" with StructuredEvent.PeerAddress (rib.go:1079), which is
// peer.AddrStr() (bgp/server/events.go:83) = settings.Address.String() (peer.go:303).
// PREVENTS: a non-default-port peer never being signaled, burning the full 2s
// waitForAPISync timeout (peer_initial_sync.go:177) and emitting its EOR ~2.5s late.
func TestSignalPeerAPIReadyNonDefaultPort(t *testing.T) {
	r := New(&Config{})
	addr := netip.MustParseAddr("192.0.2.10")
	synced := armAPISyncPeer(t, r, addr, 1179)

	// Bare IP: exactly what the emitters put on the wire.
	r.SignalPeerAPIReady(addr.String())

	require.Eventually(t, chanClosed(synced), 2*time.Second, time.Millisecond,
		"a peer on a non-default port must receive its ready signal")
}

// TestSignalPeerAPIReadyDefaultPort verifies the default-port fast path still works.
//
// VALIDATES: the direct r.peers[key] hit continues to signal the peer, so the
// fallback added for non-standard ports did not disturb the common case.
// PREVENTS: a regression on the 179 path, which is every standard deployment.
func TestSignalPeerAPIReadyDefaultPort(t *testing.T) {
	r := New(&Config{})
	addr := netip.MustParseAddr("192.0.2.11")
	synced := armAPISyncPeer(t, r, addr, DefaultBGPPort)

	r.SignalPeerAPIReady(addr.String())

	require.Eventually(t, chanClosed(synced), 2*time.Second, time.Millisecond,
		"a peer on the default port must receive its ready signal")
}

// TestSignalPeerAPIReadyUnknownPeerWarns verifies a signal for a peer that does
// not exist reaches nobody AND is observable.
//
// VALIDATES: ai/rules/fail-closed-guards.md "or say something" -- the lookup miss
// logs at Warn naming the peer instead of degrading into a silent no-op. Nothing
// downstream can report it: handlePeerSessionReady still answers "peer ready
// acknowledged" (cmd/peer/session.go:22).
// PREVENTS: a typo'd or stale peer address silently dropping the signal, leaving
// only an unexplained 2.5s EOR delay as evidence.
func TestSignalPeerAPIReadyUnknownPeerWarns(t *testing.T) {
	rec := &warnRecorder{}
	old := slog.Default()
	slog.SetDefault(slog.New(rec))
	t.Cleanup(func() { slog.SetDefault(old) })

	r := New(&Config{})
	synced := armAPISyncPeer(t, r, netip.MustParseAddr("192.0.2.12"), 1179)

	r.SignalPeerAPIReady("198.51.100.99")

	require.Never(t, chanClosed(synced), 100*time.Millisecond, time.Millisecond,
		"a signal for an unknown peer must not release a different peer's API sync")
	require.Contains(t, rec.warnedPeers(), "198.51.100.99",
		"the miss must be logged at Warn naming the peer, not silently dropped")
}
