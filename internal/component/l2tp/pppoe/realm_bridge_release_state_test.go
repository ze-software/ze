// Design: docs/architecture/l2tp/followup-l2tp-call.md -- actual admission release guards.
package pppoe

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/core/callsink"
	"github.com/ze-software/ze/internal/core/report"
)

// Only the native release/descriptor outcome is a model. Admission, CHAP/LCP,
// auth request, reader quiescence, worker release, Attempt status, admission
// selection, preparation and End all run their actual production producers.
type realmReleaseOwnerModel struct {
	*fixtureCurrentWire
	mu      sync.Mutex
	pending bool
	closed  bool
	failure error
	closes  int
}

func (*realmReleaseOwnerModel) BindNativeReleaseSeal(ppp.NativeReleaseSeal) error {
	return errors.New("model seal is already bound")
}
func (owner *realmReleaseOwnerModel) NativeReleaseBound() bool {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return !owner.closed
}
func (*realmReleaseOwnerModel) NativeReleaseBirthTag() string { return "realm-preparation/original" }
func (owner *realmReleaseOwnerModel) NativeReleasePending() bool {
	owner.mu.Lock()
	defer owner.mu.Unlock()
	return owner.closed && owner.pending
}
func (owner *realmReleaseOwnerModel) Close() error {
	cause := owner.fixtureCurrentWire.Close()
	owner.mu.Lock()
	defer owner.mu.Unlock()
	owner.closed = true
	owner.closes++
	return errors.Join(cause, owner.failure)
}

// This fixture negotiates the same real LCP and CHAP exchange as
// fixtureCurrentRealmRequest, retaining its owner to control release state.
func newRealmReleaseRequest(t *testing.T, fixture *coreRetentionFixture) (*Session, *realmReleaseOwnerModel, ppp.EventAuthRequest) {
	t.Helper()
	server := fixture.server
	session := &Session{SID: 5, State: StateSession, AccessIfIndex: server.ifIndex, MAC: []byte{2, 0, 0, 0, 0, 5}}
	require.NoError(t, server.sessions.Add(session))
	old := fixtureRelayOpened(t, 42)
	wire := &fixtureCurrentWire{old: old, input: make(chan []byte, 32), output: make(chan []byte, 32), done: make(chan struct{})}
	owner := &realmReleaseOwnerModel{fixtureCurrentWire: wire}
	opened, err := dataplane.NewOpenedSession(wire, owner, fixtureCurrentTransport{old})
	require.NoError(t, err)
	require.True(t, fixtureCurrentOpened(t, server, session, opened))
	budget, err := ppp.NewNativeReleaseBudget(1)
	require.NoError(t, err)
	require.NoError(t, fixture.driver.ConfigureNativeReleaseBudget(budget))
	require.NoError(t, fixture.driver.Start())
	fixture.started = true
	t.Cleanup(func() {
		owner.mu.Lock()
		owner.pending, owner.failure = false, nil
		owner.mu.Unlock()
		fixture.driver.Stop()
		require.NoError(t, fixture.driver.RetryPendingNativeRelease(context.Background()))
		require.NoError(t, opened.CloseTransport())
	})
	require.NotNil(t, server.sessions.takePPPLease(session))
	lcp, err := ppp.NewLCPConfig(3*time.Second, 2, 10, 5)
	require.NoError(t, err)
	fixture.driver.SessionsIn() <- ppp.StartSession{LCP: lcp, SubscriberID: session.SubscriberID, Attempt: session.Attempt, TunnelID: uint16(server.ifIndex), SessionID: session.SID, Frames: wire, Dataplane: owner, LNSMode: true, PPPoE: true, MaxMRU: 1492, AuthMethod: ppp.AuthMethodCHAPMD5, AuthRequired: true, AuthFallbackOrder: []ppp.AuthMethod{ppp.AuthMethodCHAPMD5}, DisableIPCP: true, DisableIPv6CP: true, AuthTimeout: time.Minute, IPTimeout: time.Minute, CaptureProxyLCP: true}
	protocol := &fixtureCurrentPPP{driver: fixture.driver, wire: wire, session: session, method: ppp.AuthMethodCHAPMD5, packets: make(map[uint16][]ppp.LCPPacket)}
	local := protocol.read(t, ppp.ProtoLCP, ppp.LCPConfigureRequest)
	protocol.send(t, ppp.ProtoLCP, ppp.LCPConfigureAck, local.Identifier, local.Data)
	initial := []byte{1, 4, 5, 220}
	protocol.send(t, ppp.ProtoLCP, ppp.LCPConfigureRequest, 0x31, initial)
	nak := protocol.read(t, ppp.ProtoLCP, ppp.LCPConfigureNak)
	require.Equal(t, uint8(0x31), nak.Identifier)
	require.Equal(t, []byte{1, 4, 5, 212}, nak.Data)
	protocol.send(t, ppp.ProtoLCP, ppp.LCPConfigureRequest, 0x32, nak.Data)
	ack := protocol.read(t, ppp.ProtoLCP, ppp.LCPConfigureAck)
	require.Equal(t, uint8(0x32), ack.Identifier)
	require.Equal(t, nak.Data, ack.Data)
	up, ok := protocol.next(t).(ppp.EventLCPUp)
	require.True(t, ok)
	fixture.sub.handlePPPEvent(up)
	protocol.control = up.Control
	request := protocol.request(t, "cpe@example.net")
	require.NotNil(t, request.ProxyLCP)
	require.True(t, bytes.Equal(initial, request.ProxyLCP.InitialReceived))
	require.Equal(t, nak.Data, request.ProxyLCP.LastReceived)
	require.Equal(t, local.Data, request.ProxyLCP.LastSent)
	require.NoError(t, request.Control.ReserveNativeRelease())
	return session, owner, request
}

// Actual releaseRelayedSession must distinguish protocol retirement from
// physical release, and physical release from a clean Close. Each failure
// prevents the fresh-attachment continuation, while End still reaches exactly
// the original admission. The healthy control prevents unconditional refusal.
func TestRealmPreparationChecksActualRetiredReleaseState(t *testing.T) {
	for _, mode := range []string{"native pending", "released with descriptor error", "healthy"} {
		t.Run(mode, func(t *testing.T) {
			report.ResetForTest()
			t.Cleanup(report.ResetForTest)
			fixture := newCoreRetentionFixtureStart(t, 3, false)
			session, owner, request := newRealmReleaseRequest(t, fixture)
			cause := errors.New("original native descriptor close cause")
			owner.mu.Lock()
			owner.pending = mode == "native pending"
			if mode != "healthy" {
				owner.failure = cause
			}
			owner.mu.Unlock()
			callsink.Unregister()
			t.Cleanup(callsink.Unregister)
			sink := &fakeRealmSink{canceled: make(chan struct{}, 1)}
			callsink.Register(sink)
			relay := newRealmRelay(fixture.sub.logger, fixture.driver, fixture.sub.realmSubscriber, fixture.sub.endRelayedSession, fixture.sub.releaseRelayedSession)
			t.Cleanup(relay.stopAll)
			require.True(t, relay.decide(&request))
			sink.mu.Lock()
			handoff := sink.handoff.(*realmHandoff)
			sink.mu.Unlock()
			freshAttachments := 0
			prepareCapture := func() error {
				if err := handoff.PrepareBridge(); err != nil {
					return err
				}
				freshAttachments++
				return nil
			}
			first := prepareCapture()
			status, err := session.Attempt.CleanupStatus()
			require.NoError(t, err)
			require.True(t, status.Admitted)
			require.True(t, status.Finished)
			require.True(t, status.LeaseAcquired)
			require.Equal(t, mode != "native pending", status.Released)
			if mode == "healthy" {
				require.NoError(t, first)
				require.NoError(t, status.Err)
				require.Equal(t, 1, freshAttachments)
			} else {
				require.Error(t, first)
				require.ErrorIs(t, status.Err, cause)
				if mode == "native pending" {
					require.EqualError(t, first, "pppoe: realm PPP native release remains unconfirmed")
				} else {
					require.ErrorIs(t, first, cause)
				}
				require.Equal(t, 0, freshAttachments)
				require.ErrorIs(t, prepareCapture(), first)
				require.Equal(t, 0, freshAttachments)
			}
			require.True(t, session.pppRelayReleased, "retired protocol ownership must be returned even when native attachment is refused")
			owner.mu.Lock()
			closes := owner.closes
			owner.mu.Unlock()
			closesExpected := 1
			if mode == "native pending" {
				// Run retirement retries the same pending owner once before
				// ReleaseSession returns; a confirmed first Close needs no retry.
				closesExpected = 2
			}
			require.Equal(t, closesExpected, closes)
			handoff.Ended("the original prepared call ended")
			select {
			case <-handoff.done:
			case <-time.After(time.Second):
				t.Fatal("actual prepared handoff did not retire")
			}
			require.Nil(t, fixture.server.sessions.Lookup(session.SID))
			select {
			case <-session.closeDone:
			default:
				t.Fatal("End did not complete the captured admission")
			}
			require.Equal(t, int32(1), fixture.frames.Load(), "original admission sends PADT once")
			fixture.sub.endRelayedSession(request)
			require.Equal(t, int32(1), fixture.frames.Load())
		})
	}
}
