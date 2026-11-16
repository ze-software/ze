//go:build linux

// Design: docs/architecture/l2tp/followup-l2tp-call.md -- queued relay ownership
// Related: kernel_linux.go -- setupSession, teardownSession, reportError

package l2tp

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// An old queued teardown must preserve a new relay's worker resources. The
// current owner's request still closes all three descriptors in reverse order.
// All descriptor numbers are fake and all operations use fakeKernelOps.
func TestRelayKernelTeardownRequiresExactOwner(t *testing.T) {
	stale := &relayCall{handoff: &serviceLifetimeHandoff{}}
	current := &relayCall{handoff: &serviceLifetimeHandoff{}}
	sibling := &relayCall{handoff: &serviceLifetimeHandoff{}}
	target := &pppSessionFDs{pppoxFD: 10, chanFD: 11, unitFD: 12, relay: current}
	other := &pppSessionFDs{pppoxFD: 20, chanFD: 21, unitFD: 22, relay: sibling}
	fake := &fakeKernelOps{}
	worker := &kernelWorker{logger: discardLogger(), ops: fake.ops(), sessions: map[sessionKey]*pppSessionFDs{{100, 41}: target, {100, 51}: other}, tunnels: map[uint16]*kernelTunnelState{100: {localTID: 100, connFD: -1, sessionCount: 2}}}
	worker.teardownSession(kernelTeardownEvent{localTID: 100, localSID: 41, relay: stale})
	require.Same(t, target, worker.sessions[sessionKey{100, 41}])
	require.Empty(t, fake.closes)
	require.Empty(t, fake.sessionGone)
	worker.teardownSession(kernelTeardownEvent{localTID: 100, localSID: 41, relay: current})
	require.NotContains(t, worker.sessions, sessionKey{100, 41})
	require.Same(t, other, worker.sessions[sessionKey{100, 51}])
	require.Equal(t, []int{12, 11, 10}, fake.closes)
	require.Equal(t, []sessionKey{{100, 41}}, fake.sessionGone)
	require.Equal(t, 1, worker.tunnels[100].sessionCount)
}

// The real setup worker must retain the exact call on its async refusal, so
// the reactor can reject a late result after the same numeric pair is reused.
func TestRelayKernelFailureRetainsSetupOwner(t *testing.T) {
	failure := errors.New("controlled PPP setup failure")
	fake := &fakeKernelOps{failPPPSetup: failure}
	owner := &relayCall{handoff: &serviceLifetimeHandoff{}}
	errorsOut := make(chan kernelSetupFailed, 1)
	worker := &kernelWorker{logger: discardLogger(), ops: fake.ops(), sessions: make(map[sessionKey]*pppSessionFDs), tunnels: make(map[uint16]*kernelTunnelState), errCh: errorsOut, stop: make(chan struct{})}
	worker.setupSession(kernelSetupEvent{localTID: 100, localSID: 41, relay: owner})
	select {
	case event := <-errorsOut:
		require.Same(t, owner, event.relay)
		require.ErrorIs(t, event.err, failure)
		require.Equal(t, uint16(100), event.localTID)
		require.Equal(t, uint16(41), event.localSID)
	default:
		t.Fatal("setup failure was not reported")
	}
	require.Empty(t, worker.sessions)
	require.Empty(t, worker.tunnels)
}

// Successful setup stores the exact owner in both worker resources and its
// async receipt, rather than relying on a test to prepopulate that identity.
func TestRelayKernelSetupRetainsOwner(t *testing.T) {
	fake := &fakeKernelOps{}
	owner := &relayCall{handoff: &serviceLifetimeHandoff{}}
	success := make(chan kernelSetupSucceeded, 1)
	worker := &kernelWorker{logger: discardLogger(), ops: fake.ops(), sessions: make(map[sessionKey]*pppSessionFDs), tunnels: make(map[uint16]*kernelTunnelState), successCh: success, stop: make(chan struct{})}
	// RFC 2661 Section 10.1: attach resources to the accepted incoming call.
	worker.setupSession(kernelSetupEvent{localTID: 100, localSID: 41, relay: owner})
	require.Same(t, owner, worker.sessions[sessionKey{100, 41}].relay)
	select {
	case event := <-success:
		require.Same(t, owner, event.fds.relay)
	default:
		t.Fatal("successful setup did not publish its receipt")
	}
	worker.teardownSession(kernelTeardownEvent{localTID: 100, localSID: 41, relay: owner})
	require.Empty(t, worker.sessions)
	require.Empty(t, worker.tunnels)
}

// A service has no local proxy-PPP reader to quiesce. That permitted variant
// must still refuse once its call is canceled rather than bypassing lifetime.
func TestServiceRelayQuiesceRequiresLiveOwner(t *testing.T) {
	call := &relayCall{handoff: &serviceLifetimeHandoff{}}
	require.NoError(t, quiesceRelaySubscriber(call))
	call.Cancel()
	require.ErrorIs(t, quiesceRelaySubscriber(call), errRelayCallCancelled)
}

// The realm-only reader transition must refuse a malformed internal handoff;
// it must not silently treat that owner as a Service-Name call.
func TestRealmRelayQuiesceRefusesWrongHandoff(t *testing.T) {
	call := &relayCall{realm: &relayRealmDetails{name: "example.net"}, handoff: &serviceLifetimeHandoff{}}
	require.ErrorContains(t, quiesceRelaySubscriber(call), "no PPP handoff")
}
