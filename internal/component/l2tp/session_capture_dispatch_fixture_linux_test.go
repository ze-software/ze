// Design: docs/architecture/l2tp/subscriber-session-model.md -- real actor command fixture
//go:build linux

package l2tp

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// SessionCaptureDispatchFixtureForTest exposes an actual ICRQ admission and
// worker setup to the external command test. It creates no shipped constructor
// and no successful fake CaptureSession implementation.
func SessionCaptureDispatchFixtureForTest(t *testing.T) (*Subsystem, SessionSelector, func() SessionSelector, func() (int, int, int)) {
	t.Helper()
	return sessionCaptureDispatchFixtureForObservation(t, nil, l2tpevents.ObservationGeneration{})
}

// Observation inputs configure this unpublished raw root before its listener,
// tunnel or first ICRQ can publish a session. Nil explicitly disables CQM.
func sessionCaptureDispatchFixtureForObservation(t *testing.T, observer *Observer, generation l2tpevents.ObservationGeneration) (*Subsystem, SessionSelector, func() SessionSelector, func() (int, int, int)) {
	t.Helper()
	if observer != nil {
		require.True(t, generation.IsValid())
	}
	captureBirthSIDDraw(t)
	reactor := newReactorForSnapshot(t)
	reactor.params.Generation = generation
	reactor.observer = observer
	reactor.listener = newUDPListener(netip.MustParseAddrPort("127.0.0.1:0"), reactor.logger)
	require.NoError(t, reactor.listener.Start(t.Context()))
	t.Cleanup(func() { require.NoError(t, reactor.listener.Stop()) })
	insertEstablishedTunnel(t, reactor, 100, 200, netip.MustParseAddrPort("127.0.0.1:1701"))
	tunnel := reactor.tunnelsByLocalID[100]
	worker, fake, event := workerCleanupBaseline(t)
	t.Cleanup(worker.teardownAll)
	reactor.kernelWorker = worker
	serial := uint16(69)
	admit := func() SessionSelector {
		serial++
		acknowledgeCapturedControlForTest(t, tunnel, reactor.params.Clock())
		captureBirthAdmit(t, reactor, tunnel, serial)
		session := tunnel.sessions[7]
		session.state = L2TPSessionEstablished
		event.localTID, event.localSID, event.session = 100, 7, session
		worker.setupSession(event)
		return SessionSelector{SID: 7, TID: 100, Birth: session.Birth()}
	}
	selector := admit()
	subsystem := NewSubsystem(&Parameters{})
	subsystem.started = true
	subsystem.observer = observer
	subsystem.reactors = []*l2tpReactor{reactor}
	observe := func() (int, int, int) {
		worker.mu.Lock()
		defer worker.mu.Unlock()
		fake.mu.Lock()
		defer fake.mu.Unlock()
		return len(worker.sessions), len(fake.sessionGone), len(fake.closes)
	}
	return subsystem, selector, admit, observe
}

// The fixture is a synthetic peer as well as a direct ICRQ producer. Receive
// every outstanding control sequence through a real ZLB before its next call,
// allowing the initial one-message congestion window to advance normally.
func acknowledgeCapturedControlForTest(t *testing.T, tunnel *L2TPTunnel, now time.Time) {
	t.Helper()
	for range MaxSendQueueDepth + 1 {
		if tunnel.engine.Outstanding() == 0 {
			if len(tunnel.engine.sendQueue) == 0 {
				return
			}
		}
		tunnel.engine.OnReceive(MessageHeader{IsControl: true, Nr: tunnel.engine.nextSendSeq}, nil, now)
	}
	require.Zero(t, tunnel.engine.Outstanding(), "peer must acknowledge the actual control sequences")
	require.Empty(t, tunnel.engine.sendQueue)
}
