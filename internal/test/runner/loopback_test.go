// Design: docs/architecture/testing/ci-format.md -- multi-peer loopback alias tests

package runner

import (
	"context"
	"net"
	"strings"
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
	_, err := outputs[0].stdout.Write([]byte("listening on 127.0.0.1:1790\n"))
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
}

// TestEnsureLoopbackAlias verifies that ensureLoopbackAlias succeeds for
// 127.0.0.1 (always available) on all platforms.
//
// VALIDATES: AC-5 (loopback alias -- basic case)
// PREVENTS: ensureLoopbackAlias failing for the default loopback address.
func TestEnsureLoopbackAlias(t *testing.T) {
	// 127.0.0.1 is always available on all platforms.
	err := ensureLoopbackAlias(net.ParseIP("127.0.0.1"))
	assert.NoError(t, err)
}
