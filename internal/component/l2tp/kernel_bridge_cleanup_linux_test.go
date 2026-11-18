// Design: docs/architecture/l2tp/followup-l2tp-call.md -- exact bridge retry custody.
//go:build linux

package l2tp

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/core/report"
)

// This private lifecycle port owns no public Created channel or native grant.
type cleanupBridgeModel struct {
	pending bool
	failure error
	closes  int
}

func (*cleanupBridgeModel) Bridge() error             { return nil }
func (*cleanupBridgeModel) Unbridge() error           { return nil }
func (bridge *cleanupBridgeModel) ClosePending() bool { return bridge.pending }
func (bridge *cleanupBridgeModel) Close() error       { bridge.closes++; return bridge.failure }

// An uncertain native unbridge keeps the SAME handle and original transport
// through worker cleanup. A settled retry closes
// transport, deletes only that session and leaves its sibling/tunnel alive.
func TestWorkerBridgeCleanupRetainsOriginalUntilSettled(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	original, peer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, peer.Close()) })
	want := errors.New("exact unbridge is uncertain")
	bridge := &cleanupBridgeModel{pending: true, failure: want}
	owners := &kernelPPPOwners{bridge: bridge}
	key := sessionKey{tunnelID: 501, sessionID: 502}
	siblingKey := sessionKey{tunnelID: 501, sessionID: 503}
	tunnel := &kernelTunnelState{localTID: 501, connFD: -1, sessionCount: 2}
	owned := &pppSessionFDs{ppp: owners, transport: original, pppoxFD: 771, chanFD: 772, unitFD: -1, tunnel: tunnel, bridged: true}
	sibling := &pppSessionFDs{pppoxFD: -1, chanFD: -1, unitFD: -1, tunnel: tunnel}
	deletes := 0
	worker := newKernelWorker(kernelOps{
		closePPP: func(owners *kernelPPPOwners, transport *os.File, _ [3]int) [3]error {
			return closeOriginalKernelPPP(owners, transport, (*os.File).Close)
		},
		sessionDelete: func(tid, sid uint16) error {
			require.Equal(t, key.tunnelID, tid)
			require.Equal(t, key.sessionID, sid)
			deletes++
			return nil
		},
	}, nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	worker.tunnels[501] = tunnel
	worker.sessions[key], worker.sessions[siblingKey] = owned, sibling
	t.Cleanup(func() {
		// Only the task's still-owned original is eligible for fallback close.
		if owned.transport != nil {
			require.NoError(t, owned.transport.Close())
			owned.transport = nil
		}
	})
	worker.mu.Lock()
	worker.teardownSessionFDsLocked(key, owned)
	worker.mu.Unlock()
	require.Same(t, owners, owned.ppp)
	require.Same(t, bridge, owned.ppp.bridge)
	require.Same(t, original, owned.transport)
	require.ErrorIs(t, owned.bridgeErr, want)
	require.NoError(t, owned.closeErr)
	require.Equal(t, 0, deletes)
	require.Len(t, report.Warnings(), 1)
	_, err = original.Stat()
	require.NoError(t, err)
	bridge.pending, bridge.failure = false, nil
	worker.mu.Lock()
	worker.teardownSessionFDsLocked(key, owned)
	worker.mu.Unlock()
	require.Equal(t, 2, bridge.closes)
	require.Equal(t, 1, deletes)
	require.Nil(t, worker.sessions[key])
	require.Same(t, sibling, worker.sessions[siblingKey])
	require.Same(t, tunnel, worker.tunnels[501])
	require.Equal(t, 1, tunnel.sessionCount)
	require.Nil(t, owned.bridgeErr)
	require.Empty(t, report.Warnings(), "only settled exact bridge warning may clear")
	_, err = original.Stat()
	require.ErrorIs(t, err, os.ErrClosed)
}

// Stop's exact teardown traversal must retain a pending handle and transport,
// then retry that same handle before removing its session and tunnel.
func TestWorkerStopTraversalRetriesSameBridge(t *testing.T) {
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
	worker, _, event := workerCleanupBaseline(t)
	worker.setupSession(event)
	key := sessionKey{event.localTID, event.localSID}
	owned := worker.sessions[key]
	require.NotNil(t, owned)
	bridge := &cleanupBridgeModel{pending: true, failure: errors.New("native unbridge pending")}
	owned.ppp = &kernelPPPOwners{bridge: bridge}
	owned.unitFD, owned.chanFD, owned.pppoxFD = -1, -1, -1
	worker.ops.closePPP = func(owners *kernelPPPOwners, transport *os.File, _ [3]int) [3]error {
		return closeOriginalKernelPPP(owners, transport, (*os.File).Close)
	}
	worker.teardownAll()
	require.Same(t, owned, worker.sessions[key])
	require.Same(t, bridge, owned.ppp.bridge)
	require.Len(t, worker.tunnels, 1)
	bridge.pending, bridge.failure = false, nil
	worker.teardownAll()
	require.Equal(t, 2, bridge.closes)
	require.Empty(t, worker.sessions)
	require.Empty(t, worker.tunnels)
}

// Terminal bridge descriptor failure remains recorded while independent
// transport cleanup still runs. It cannot become an uncertain native retry.
func TestBridgeTerminalCloseFailureStillClosesTransport(t *testing.T) {
	original, peer, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, peer.Close()) })
	want := errors.New("original channel close failed")
	owners := &kernelPPPOwners{bridge: &cleanupBridgeModel{failure: want}}
	failures := closeOriginalKernelPPP(owners, original, (*os.File).Close)
	require.ErrorIs(t, failures[1], want)
	require.NoError(t, failures[2])
	require.False(t, kernelPPPBridgePending(owners))
	_, err = original.Stat()
	require.ErrorIs(t, err, os.ErrClosed)
}
