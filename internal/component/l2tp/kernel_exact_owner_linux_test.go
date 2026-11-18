// Design: docs/research/l2tpv2-ze-integration.md -- exact queued kernel cleanup ownership

//go:build linux

package l2tp

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// TestKernelQueuedTeardownCannotDeleteSameLabelSuccessor captures the real
// removal event, completes its original cleanup, then delivers that same event
// after a new real worker setup reused both labels. The late event must neither
// close the successor's descriptors nor delete its native session.
func TestKernelQueuedTeardownCannotDeleteSameLabelSuccessor(t *testing.T) {
	worker, fake, setup := workerCleanupBaseline(t)
	original := setup.session
	original.state = L2TPSessionEstablished
	tunnel := &L2TPTunnel{localTID: setup.localTID, sessions: map[uint16]*L2TPSession{setup.localSID: original}}
	worker.setupSession(setup)
	require.Len(t, worker.sessions, 1)
	tunnel.removeSession(setup.localSID, l2tpevents.TerminateCauseAdminReset)
	require.Len(t, tunnel.pendingKernelTeardowns, 1)
	removed := tunnel.pendingKernelTeardowns[0]
	worker.teardownSession(removed)
	require.Empty(t, worker.sessions)
	require.Len(t, fake.sessionGone, 1)
	require.Len(t, fake.closes, 3)

	replacement := &L2TPSession{localSID: setup.localSID, state: L2TPSessionEstablished}
	setup.session = replacement
	worker.setupSession(setup)
	key := sessionKey{setup.localTID, setup.localSID}
	owned := worker.sessions[key]
	require.NotNil(t, owned)
	t.Cleanup(worker.teardownAll)

	worker.teardownSession(removed)
	require.Same(t, owned, worker.sessions[key], "the queued event belongs to the removed row, not its replacement")
	require.Len(t, fake.sessionGone, 1, "a late original event must not issue a delete for its successor")
	require.Len(t, fake.closes, 3, "a late original event must not close reused descriptor numbers")
}

// An asynchronous failure must carry the same original row as setup and
// cleanup; ordinary nil-relay calls cannot use labels as failure authority.
func TestKernelLateSetupFailureCannotRemoveSameLabelSuccessor(t *testing.T) {
	captureBirthSIDDraw(t)
	reactor := newReactorForSnapshot(t)
	reactor.listener = newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), reactor.logger)
	require.NoError(t, reactor.listener.Start(t.Context()))
	t.Cleanup(func() { require.NoError(t, reactor.listener.Stop()) })
	insertEstablishedTunnel(t, reactor, 100, 200, netip.MustParseAddrPort("192.0.2.1:1701"))
	tunnel := reactor.tunnelsByLocalID[100]
	captureBirthAdmit(t, reactor, tunnel, 70)
	original := tunnel.sessions[7]
	original.state = L2TPSessionEstablished
	fake := &fakeKernelOps{failPPPSetup: errors.New("original setup refused")}
	failures := make(chan kernelSetupFailed, 1)
	worker := newKernelWorker(fake.ops(), failures, nil, discardLogger())
	t.Cleanup(worker.teardownAll)
	event := setupEvent(100, 7)
	event.session = original
	worker.setupSession(event)
	failure := <-failures
	tunnel.removeSession(7, l2tpevents.TerminateCauseAdminReset)
	acknowledgeCapturedControlForTest(t, tunnel, reactor.params.Clock())
	captureBirthAdmit(t, reactor, tunnel, 71)
	replacement := tunnel.sessions[7]
	reactor.handleKernelError(failure)
	require.Same(t, replacement, tunnel.sessions[7], "the original failure cannot tear down its same-label successor")
}

// TestKernelSetupCannotReplaceUnretiredNativeOwner makes the existing kernel
// seam accept every create. The worker must still preserve its original owner
// until cleanup confirms release, then admit useful setup under those labels.
func TestKernelSetupCannotReplaceUnretiredNativeOwner(t *testing.T) {
	worker, fake, setup := workerCleanupBaseline(t)
	worker.setupSession(setup)
	key := sessionKey{setup.localTID, setup.localSID}
	original := worker.sessions[key]
	require.NotNil(t, original)
	t.Cleanup(worker.teardownAll)

	replacement := setup
	replacement.session = &L2TPSession{localSID: setup.localSID}
	worker.setupSession(replacement)
	require.Same(t, original, worker.sessions[key], "a reusable tuple cannot replace an unretired native owner")
	require.Len(t, fake.pppSetups, 1, "the replacement cannot acquire descriptors before the original releases them")

	worker.teardownAll()
	require.Empty(t, worker.sessions)
	worker.setupSession(replacement)
	require.NotNil(t, worker.sessions[key], "confirmed cleanup must permit useful setup of the replacement")
	require.NotSame(t, original, worker.sessions[key])
	require.Len(t, fake.pppSetups, 2)
}

// TestKernelCleanupBeforeSetupCannotResurrectRemovedSession delivers the real
// removal before its old queued setup. No native object may be created for
// that removed row; a fresh same-label row must still acquire useful resources.
func TestKernelCleanupBeforeSetupCannotResurrectRemovedSession(t *testing.T) {
	worker, fake, setup := workerCleanupBaseline(t)
	t.Cleanup(worker.teardownAll)
	original := setup.session
	original.state = L2TPSessionEstablished
	tunnel := &L2TPTunnel{localTID: setup.localTID, sessions: map[uint16]*L2TPSession{setup.localSID: original}}
	tunnel.removeSession(setup.localSID, l2tpevents.TerminateCauseAdminReset)
	require.Len(t, tunnel.pendingKernelTeardowns, 1)
	worker.teardownSession(tunnel.pendingKernelTeardowns[0])
	require.Empty(t, worker.sessions)

	worker.setupSession(setup)
	require.Empty(t, worker.sessions, "late setup must not resurrect the removed original row")
	require.Empty(t, fake.tunnelCreated, "the removed row must not create a native tunnel")
	require.Empty(t, fake.sessionAdded, "the removed row must not create a native session")
	require.Empty(t, fake.pppSetups, "the removed row must not acquire PPP descriptors")

	fresh := setup
	fresh.session = &L2TPSession{localSID: setup.localSID, state: L2TPSessionEstablished}
	tunnel.sessions[setup.localSID] = fresh.session
	worker.setupSession(fresh)
	require.Len(t, worker.sessions, 1, "removing an original must not prohibit its useful same-label successor")
	require.Len(t, fake.tunnelCreated, 1)
	require.Len(t, fake.sessionAdded, 1)
	require.Len(t, fake.pppSetups, 1)
}
