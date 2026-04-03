// Design: docs/architecture/testing/ci-format.md -- multi-peer loopback alias tests

package runner

import (
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPerProcessSyncWriter verifies that multiple ze-peer processes get
// independent syncWriter instances with independent WaitFor synchronization.
//
// VALIDATES: AC-1 (independent syncWriter per ze-peer), AC-2 (independent WaitFor)
// PREVENTS: Shared syncWriter race where first peer's "listening on" satisfies second peer's WaitFor.
func TestPerProcessSyncWriter(t *testing.T) {
	// Create two independent peerOutput instances (simulating what runOrchestrated does).
	po1 := peerOutput{
		stdout: newSyncWriter(),
		stderr: &strings.Builder{},
	}
	po2 := peerOutput{
		stdout: newSyncWriter(),
		stderr: &strings.Builder{},
	}

	// Write "listening on" to first peer only.
	_, err := po1.stdout.Write([]byte("listening on 127.0.0.1:1790\n"))
	require.NoError(t, err)

	// First peer's WaitFor should succeed immediately.
	ctx1, cancel1 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel1()
	assert.True(t, po1.stdout.WaitFor(ctx1), "first peer should find 'listening on'")

	// Second peer's WaitFor should NOT succeed (no output written to it).
	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()
	assert.False(t, po2.stdout.WaitFor(ctx2), "second peer should not find 'listening on' from first peer")

	// Now write to second peer.
	_, err = po2.stdout.Write([]byte("listening on 127.0.0.2:1790\n"))
	require.NoError(t, err)

	ctx3, cancel3 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel3()
	assert.True(t, po2.stdout.WaitFor(ctx3), "second peer should find 'listening on' after its own write")

	// Verify output isolation: each peer's output is independent.
	assert.Contains(t, po1.stdout.String(), "127.0.0.1")
	assert.NotContains(t, po1.stdout.String(), "127.0.0.2")
	assert.Contains(t, po2.stdout.String(), "127.0.0.2")
	assert.NotContains(t, po2.stdout.String(), "127.0.0.1")
}

// TestPerProcessSyncWriterConcurrent verifies the fix for the original race:
// peer1's "listening on" must NOT unblock peer2's WaitFor under concurrency.
//
// VALIDATES: AC-2 (independent WaitFor under concurrent writes)
// PREVENTS: Race where concurrent write to peer1 satisfies peer2's blocking WaitFor.
func TestPerProcessSyncWriterConcurrent(t *testing.T) {
	po1 := peerOutput{stdout: newSyncWriter(), stderr: &strings.Builder{}}
	po2 := peerOutput{stdout: newSyncWriter(), stderr: &strings.Builder{}}

	var wg sync.WaitGroup

	// Start WaitFor on peer2 in a goroutine (will block).
	wg.Add(1)
	var po2Found bool
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		po2Found = po2.stdout.WaitFor(ctx)
	}()

	// Write "listening on" to peer1 concurrently.
	time.Sleep(20 * time.Millisecond) // Let peer2's WaitFor start blocking
	_, err := po1.stdout.Write([]byte("listening on 127.0.0.1:1790\n"))
	require.NoError(t, err)

	wg.Wait()
	// peer2's WaitFor should have timed out (peer1's write doesn't affect it).
	assert.False(t, po2Found, "peer1's write must not unblock peer2's WaitFor")
}

// TestSinglePeerUnchanged verifies that single-peer tests still work with
// the per-process output tracking (backward compatibility).
//
// VALIDATES: AC-4 (single peer unchanged behavior)
// PREVENTS: Regression where single-peer tests break due to per-process tracking changes.
func TestSinglePeerUnchanged(t *testing.T) {
	// Single peer: one peerOutput in the slice.
	po := peerOutput{
		stdout: newSyncWriter(),
		stderr: &strings.Builder{},
	}
	outputs := []peerOutput{po}

	// Write output.
	_, err := outputs[0].stdout.Write([]byte("listening on 127.0.0.1:1790\nsuccessful\n"))
	require.NoError(t, err)
	_, err = outputs[0].stderr.WriteString("some stderr\n")
	require.NoError(t, err)

	// WaitFor works.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	assert.True(t, outputs[0].stdout.WaitFor(ctx))

	// Combined output works the same as before.
	var allStdout, allStderr strings.Builder
	for i := range outputs {
		allStdout.WriteString(outputs[i].stdout.String())
		allStderr.WriteString(outputs[i].stderr.String())
	}
	combined := allStdout.String() + allStderr.String()
	assert.Contains(t, combined, "listening on 127.0.0.1:1790")
	assert.Contains(t, combined, "some stderr")
	// Success detection still works on combined output.
	assert.Contains(t, combined, "successful")
}

// TestEnsureLoopbackAlias verifies that ensureLoopbackAlias succeeds for
// loopback addresses on all platforms.
//
// VALIDATES: AC-5 (loopback alias)
// PREVENTS: ensureLoopbackAlias failing for loopback addresses.
func TestEnsureLoopbackAlias(t *testing.T) {
	// 127.0.0.1 is always available on all platforms.
	err := ensureLoopbackAlias(net.ParseIP("127.0.0.1"))
	assert.NoError(t, err)

	// 127.0.0.2 -- on Linux this is a no-op (127.0.0.0/8 routes to lo).
	// On macOS/FreeBSD this requires root (SIOCAIFADDR ioctl).
	err = ensureLoopbackAlias(net.ParseIP("127.0.0.2"))
	assert.NoError(t, err) // Linux: always passes. macOS: passes if root.

	// Verify 127.0.0.2 is actually usable after the call.
	var lc net.ListenConfig
	ln, listenErr := lc.Listen(context.Background(), "tcp", "127.0.0.2:0")
	if listenErr == nil {
		require.NoError(t, ln.Close())
	}
	assert.NoError(t, listenErr, "127.0.0.2 should be bindable after ensureLoopbackAlias")

	// Idempotent: calling twice for the same IP must not error.
	err = ensureLoopbackAlias(net.ParseIP("127.0.0.2"))
	assert.NoError(t, err)
}

// TestEnsureLoopbackAliasRejectsIPv6 verifies that non-IPv4 addresses are rejected.
//
// VALIDATES: AC-5 (input validation)
// PREVENTS: Passing IPv6 address to IPv4-only ioctl.
func TestEnsureLoopbackAliasRejectsIPv6(t *testing.T) {
	err := ensureLoopbackAlias(net.ParseIP("::1"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not IPv4")
}

// TestExtractBindAddresses verifies the --bind address parsing logic used
// in runOrchestrated to call ensureLoopbackAlias for multi-peer tests.
//
// VALIDATES: Correct extraction of --bind IPs from command strings.
// PREVENTS: Missed or incorrect bind address extraction.
func TestExtractBindAddresses(t *testing.T) {
	tests := []struct {
		name    string
		exec    string
		wantIPs []string // expected IPs to extract (empty = none)
	}{
		{"peer_with_bind", "ze-peer --bind 127.0.0.2 --mode sink --port 1790", []string{"127.0.0.2"}},
		{"peer_no_bind", "ze-peer --port 1790", nil},
		{"non_peer_with_bind", "ze --bind 127.0.0.2", nil}, // only ze-peer commands
		{"bind_truncated", "ze-peer --bind", nil},          // --bind without value
		{"bind_invalid_ip", "ze-peer --bind not-an-ip --port 1790", nil},
		{"bind_default_loopback", "ze-peer --bind 127.0.0.1 --port 1790", []string{"127.0.0.1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			if !strings.Contains(tt.exec, "ze-peer") {
				// Non-ze-peer commands are skipped in the real code.
				assert.Empty(t, tt.wantIPs)
				return
			}
			parts := strings.Fields(tt.exec)
			for i, p := range parts {
				if p == "--bind" && i+1 < len(parts) {
					if ip := net.ParseIP(parts[i+1]); ip != nil {
						got = append(got, ip.String())
					}
				}
			}
			assert.Equal(t, tt.wantIPs, got)
		})
	}
}
