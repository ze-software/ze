// Design: docs/architecture/l2tp/subscriber-session-model.md -- original PPP construction and unused cleanup
//go:build linux

package l2tp

import (
	"errors"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
)

// TestKernelPPPFactoryRollbackRetainsOriginalFailure drives the real worker's
// transfer into a constructor refusal. Its exact rollback failure must remain
// observable after logical removal and a successful native transport sweep.
func TestKernelPPPFactoryRollbackRetainsOriginalFailure(t *testing.T) {
	rig := newPPPConstructorReceiptRig(t)
	failure := errors.New("original constructor descriptor close remains unknown")
	rig.reactor.newPPPSession = func(pppkernel.SessionConfig) (ppp.FrameChannel, ppp.SessionHandle, error) {
		return nil, nil, pppkernel.RecordSetupCleanup(errors.New("constructor refused"), failure)
	}
	rig.reactor.handleKernelSuccess(rig.success)
	require.Nil(t, rig.tunnel.lookupSession(rig.session.localSID), "the refused logical row must leave the active map")
	rig.reactor.kernelWorker.teardownAll()
	require.ErrorIs(t, rig.reactor.cleanupCapturedOriginal(t.Context(), rig.tunnel, rig.session), failure)
	rig.reactor.tunnelsMu.Lock()
	rig.reactor.pruneRetiredSessionsLocked()
	retained := rig.reactor.retiredSessions[rig.session]
	rig.reactor.tunnelsMu.Unlock()
	require.Same(t, rig.tunnel, retained, "uncertain constructor cleanup must retain its original admission")
}

// TestKernelPPPConstructionKeepsRemovedOriginalPending removes the row at the
// real factory boundary, before any PPP attempt exists. A factory still owning
// the transferred descriptors must prevent pruning that removed original.
func TestKernelPPPConstructionKeepsRemovedOriginalPending(t *testing.T) {
	rig := newPPPConstructorReceiptRig(t)
	rig.reactor.newPPPSession = func(pppkernel.SessionConfig) (ppp.FrameChannel, ppp.SessionHandle, error) {
		rig.reactor.retireSessions()
		rig.reactor.kernelWorker.teardownAll()
		require.ErrorIs(t, rig.reactor.cleanupCapturedOriginal(t.Context(), rig.tunnel, rig.session), ErrSessionCleanupPending,
			"construction has not yet reported its own physical cleanup outcome")
		rig.reactor.tunnelsMu.Lock()
		rig.reactor.pruneRetiredSessionsLocked()
		retained := rig.reactor.retiredSessions[rig.session]
		rig.reactor.tunnelsMu.Unlock()
		require.Same(t, rig.tunnel, retained)
		return nil, nil, errors.New("construction refused after confirming its own rollback")
	}
	rig.reactor.handleKernelSuccess(rig.success)
	require.NoError(t, rig.reactor.cleanupCapturedOriginal(t.Context(), rig.tunnel, rig.session),
		"confirmed constructor rollback must permit retirement of the useful original root")
}

// TestKernelPPPRemovedDuringFactoryRetainsUnusedCloseFailure removes the exact
// row before the factory returns a usable owner. Its failed unused Close must
// remain owned even though no PPP attempt was ever published on that row.
func TestKernelPPPRemovedDuringFactoryRetainsUnusedCloseFailure(t *testing.T) {
	rig := newPPPConstructorReceiptRig(t)
	failure := errors.New("removed original unused PPP close remains unknown")
	var owner *unusedPPPReceiptOwner
	rig.reactor.newPPPSession = func(cfg pppkernel.SessionConfig) (ppp.FrameChannel, ppp.SessionHandle, error) {
		frames, lease, err := dispatchProtocolSessionFixture(cfg)
		if err != nil {
			return nil, nil, err
		}
		owner = &unusedPPPReceiptOwner{SessionHandle: lease, failure: failure}
		rig.reactor.retireSessions()
		return frames, owner, nil
	}
	rig.reactor.handleKernelSuccess(rig.success)
	require.NotNil(t, owner)
	require.Equal(t, 1, owner.closes)
	require.Equal(t, ppp.SessionAttempt{}, rig.session.pppAttempt,
		"removal before the factory result cannot publish a late handoff")
	rig.reactor.kernelWorker.teardownAll()
	for range 2 {
		require.ErrorIs(t, rig.reactor.cleanupCapturedOriginal(t.Context(), rig.tunnel, rig.session), failure)
	}
	rig.reactor.tunnelsMu.Lock()
	rig.reactor.pruneRetiredSessionsLocked()
	retained := rig.reactor.retiredSessions[rig.session]
	rig.reactor.tunnelsMu.Unlock()
	require.Same(t, rig.tunnel, retained)
	require.Equal(t, 1, owner.closes, "observing unknown cleanup cannot retry a consumed descriptor")
}

// TestKernelPPPUnusedAttemptConfirmsItsOwnClose stops the real pre-send path.
// Both successful and failed Close must finish that exact attempt; repeated
// observation cannot retry raw descriptors or invent success for the failure.
func TestKernelPPPUnusedAttemptConfirmsItsOwnClose(t *testing.T) {
	for _, outcome := range []string{"confirmed", "unconfirmed"} {
		t.Run(outcome, func(t *testing.T) {
			rig := newPPPConstructorReceiptRig(t)
			driver := newFakePPPDriver()
			driver.sessionsIn = make(chan ppp.StartSession)
			rig.reactor.pppDriver = driver
			stop := make(chan struct{})
			close(stop)
			rig.reactor.stop = stop
			var failure error
			if outcome == "unconfirmed" {
				failure = errors.New("unused original PPP lease close remains unknown")
			}
			var owner *unusedPPPReceiptOwner
			rig.reactor.newPPPSession = func(cfg pppkernel.SessionConfig) (ppp.FrameChannel, ppp.SessionHandle, error) {
				frames, lease, err := dispatchProtocolSessionFixture(cfg)
				if err != nil {
					return nil, nil, err
				}
				owner = &unusedPPPReceiptOwner{SessionHandle: lease, failure: failure}
				return frames, owner, nil
			}
			rig.reactor.handleKernelSuccess(rig.success)
			require.NotNil(t, owner)
			require.Equal(t, 1, owner.closes)
			attempt := rig.session.pppAttempt
			require.NotEqual(t, ppp.SessionAttempt{}, attempt)
			status, err := attempt.CleanupStatus()
			require.NoError(t, err)
			require.True(t, status.Finished, "the producer actually completed the unused lease's Close")
			require.False(t, status.Admitted)
			require.Equal(t, failure == nil, status.Released)
			if failure != nil {
				require.ErrorIs(t, status.Err, failure)
			} else {
				require.NoError(t, status.Err)
			}
			rig.reactor.retireSessions()
			rig.reactor.kernelWorker.teardownAll()
			for range 2 {
				err := rig.reactor.cleanupCapturedOriginal(t.Context(), rig.tunnel, rig.session)
				if failure != nil {
					require.ErrorIs(t, err, failure)
				} else {
					require.NoError(t, err)
				}
			}
			require.Equal(t, 1, owner.closes)
		})
	}
}

// TestKernelPPPStoppedGenerationRetainsFactoryFailure drives actual Stop and
// Start after a constructor rollback failure. Neither may discard the original
// generation or close its backend lease while physical release is uncertain.
func TestKernelPPPStoppedGenerationRetainsFactoryFailure(t *testing.T) {
	rig := newPPPConstructorReceiptRig(t)
	// Subsystem.Stop owns a started worker's stop channel and join. Start
	// the actual worker here rather than inventing its private lifecycle.
	rig.reactor.kernelWorker.Start()
	t.Cleanup(rig.reactor.kernelWorker.Stop)
	failure := errors.New("stopped original constructor close remains unknown")
	rig.reactor.newPPPSession = func(pppkernel.SessionConfig) (ppp.FrameChannel, ppp.SessionHandle, error) {
		return nil, nil, pppkernel.RecordSetupCleanup(errors.New("constructor refused"), failure)
	}
	rig.reactor.handleKernelSuccess(rig.success)
	backend := &kernelBackendProbe{}
	subsystem := NewSubsystem(&Parameters{})
	subsystem.started = true
	subsystem.reactors = []*l2tpReactor{rig.reactor}
	subsystem.kernelWorkers = []*kernelWorker{rig.reactor.kernelWorker}
	subsystem.kernelBackend = backend
	require.ErrorIs(t, subsystem.Stop(t.Context()), ErrSessionCleanupPending)
	require.Equal(t, 0, backend.closes)
	require.Equal(t, []*l2tpReactor{rig.reactor}, subsystem.retiredReactors)
	require.ErrorIs(t, subsystem.Start(t.Context(), nil, nil), failure)
	require.False(t, subsystem.started)
	require.Equal(t, 0, backend.closes)
	require.Equal(t, []*l2tpReactor{rig.reactor}, subsystem.retiredReactors)
}

type pppConstructorReceiptRig struct {
	reactor *l2tpReactor
	tunnel  *L2TPTunnel
	session *L2TPSession
	success kernelSetupSucceeded
}

// The existing worker's syscall seam supplies recorder descriptor tokens, not
// OS descriptors. This fixture starts no socket or worker goroutine.
func newPPPConstructorReceiptRig(t *testing.T) *pppConstructorReceiptRig {
	t.Helper()
	worker, _, event := workerCleanupBaseline(t)
	event.session.state = L2TPSessionEstablished
	successes := make(chan kernelSetupSucceeded, 1)
	worker.successCh = successes
	worker.setupSession(event)
	t.Cleanup(worker.teardownAll)
	var success kernelSetupSucceeded
	select {
	case success = <-successes:
	default:
		t.Fatal("real worker setup did not produce its own success receipt")
	}
	reactor := newReactorForSnapshot(t)
	reactor.listener = newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), reactor.logger)
	reactor.kernelWorker = worker
	reactor.pppDriver = newFakePPPDriver()
	insertEstablishedTunnel(t, reactor, event.localTID, event.remoteTID, event.peerAddr, event.session)
	tunnel := reactor.tunnelsByLocalID[event.localTID]
	return &pppConstructorReceiptRig{reactor: reactor, tunnel: tunnel, session: event.session, success: success}
}

// This dispatch-only owner caches its physical Close result exactly as the
// SessionHandle contract requires. It owns no native descriptor or goroutine.
type unusedPPPReceiptOwner struct {
	ppp.SessionHandle
	once    sync.Once
	closes  int
	failure error
	result  error
}

func (owner *unusedPPPReceiptOwner) Close() error {
	owner.once.Do(func() {
		owner.closes++
		owner.result = errors.Join(owner.SessionHandle.Close(), owner.failure)
	})
	return owner.result
}
