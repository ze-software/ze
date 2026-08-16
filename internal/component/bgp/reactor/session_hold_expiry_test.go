// Design: docs/architecture/core-design.md — BGP session hold-timer lifecycle
// Related: session.go — the OnHoldTimerExpires callback under test
//
// rfc-test-change-approved: 2026-08-03 -- Thomas ruled for full RFC 4271
// Section 8.2.2 Event 10 conformance and ordered the hold-timer grace removed:
// a hold expiry now always tears the session down, with no reprieve. He
// accepted the stated cost, that a CPU-congested daemon will drop sessions it
// used to keep. This file was session_hold_grace_test.go and asserted the
// opposite; both of its tests are inverted here rather than deleted
// (ai/rules/testing.md). The ruling supersedes spec Q-1 (2026-07-17),
// which settled only the grace DURATION and predates the 2026-07-27 void date
// in ai/rules/rfc-compliance.md.
// every assertion this file lost ("hold timer must still be armed
// after a graced expiry", "first expiry should be graced") asserted the REMOVED
// grace branch. Each is replaced by its opposite below.

package reactor

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/test/sim"
)

// newHoldExpirySession builds a real Session (so the production
// OnHoldTimerExpires closure installed by NewSession is the code under test)
// with its timers driven by a fake clock.
func newHoldExpirySession(t *testing.T, hold time.Duration) (*Session, *sim.FakeClock) {
	t.Helper()

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x0a000001)
	settings.ReceiveHoldTime = hold

	s := NewSession(settings)
	fc := sim.NewFakeClock(time.Now())
	s.timers.SetClock(fc)
	s.timers.SetHoldTime(hold)
	t.Cleanup(s.timers.StopAll)

	return s, fc
}

// newEstablishedFakeClockSession drives a real Session all the way to
// Established over a net.Pipe, with a FakeClock behind its timers.
//
// The traffic is what this helper exists for. The removed grace branch keyed on
// "did the read loop see a message since the timer was armed", so a test that
// fires an expiry on a session that never read anything would pass with the
// grace fully restored -- the restored branch would take its teardown arm and
// look identical. Reaching Established means an OPEN and a KEEPALIVE were read
// through the production read path immediately before the expiry, which is the
// exact condition the grace used to fire on.
func newEstablishedFakeClockSession(t *testing.T, hold time.Duration) (*Session, *sim.FakeClock) {
	t.Helper()

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = hold
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
	}

	s := NewSession(settings)
	fc := sim.NewFakeClock(time.Now())
	s.SetClock(fc)

	require.NoError(t, s.Start())

	server, client := net.Pipe()
	// The drain must be running before anything is written: net.Pipe is
	// unbuffered, and the timer callbacks fired by fc.Add run synchronously on
	// the goroutine calling it, so an unread write would deadlock the test
	// inside the code under test instead of asserting on it.
	startDrain(t, client)

	t.Cleanup(func() {
		s.timers.StopAll()
		s.stopSendHoldTimer()
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	})

	require.NoError(t, s.Accept(server))

	peerOpen := &message.Open{
		Version: 4, MyAS: 65002, HoldTime: uint16(hold / time.Second), BGPIdentifier: 0x02020302,
		OptionalParams: []byte{
			2, 12,
			65, 4, 0, 0, 0xFD, 0xEA,
			1, 4, 0, 1, 0, 1,
		},
	}
	go func() {
		client.Write(message.PackTo(peerOpen, nil)) //nolint:errcheck // test goroutine
	}()
	require.NoError(t, s.ReadAndProcess())

	go func() {
		client.Write(message.PackTo(message.NewKeepalive(), nil)) //nolint:errcheck // test goroutine
	}()
	require.NoError(t, s.ReadAndProcess())
	require.Equal(t, fsm.StateEstablished, s.State())

	// The hold timer must be running for the clock advance below to cross a real
	// expiry rather than silently doing nothing.
	require.True(t, s.timers.IsHoldTimerRunning(), "precondition: hold timer armed once Established")

	return s, fc
}

// TestHoldExpiryTearsDownOnTheFirstFireAfterTraffic is the reactor-level
// statement of RFC 4271 Section 8.2.2 Event 10: HoldTimer_Expires runs the
// action list. There is no branch that keeps the session, and in particular no
// branch that keeps it because the daemon was busy rather than the peer.
//
// VALIDATES: RFC 4271 Section 8.2.2 Event 10 -- the local system tears the
// session down on the expiry, having just told the peer why.
//
// PREVENTS: the reprieve returning. Ze used to grant one bounded grace window
// when the read loop had seen traffic since the timer was armed, and tore down
// only on the NEXT expiry -- so a silent peer survived up to two hold times and
// the Event 10 action list ran late. This test reaches Established through the
// real read path, so the traffic condition the old grace branch keyed on holds
// at the moment the timer fires: restoring the grace turns it red rather than
// passing on the teardown arm.
func TestHoldExpiryTearsDownOnTheFirstFireAfterTraffic(t *testing.T) {
	const hold = 3 * time.Second

	s, fc := newEstablishedFakeClockSession(t, hold)

	fc.Add(hold)

	require.False(t, s.timers.IsHoldTimerRunning(),
		"RFC 4271 Section 8.2.2 Event 10: the FIRST hold expiry tears the session "+
			"down, so the hold timer must be left disarmed. An armed timer here "+
			"means a reprieve was granted and dead-peer detection now costs two "+
			"hold times instead of one")

	select {
	case err := <-s.errChan:
		require.ErrorIs(t, err, ErrHoldTimerExpired,
			"the first expiry must signal hold-timer expiry")
	default:
		t.Fatal("RFC 4271 Section 8.2.2 Event 10: the first hold expiry must tear " +
			"the session down, even though the read loop saw traffic moments before")
	}
}

// TestHoldExpiryGrantsNoSecondWindow is the other half of the contract, stated
// over time rather than over one fire: the whole dead-peer detection budget is
// ONE hold time.
//
// VALIDATES: RFC 4271 Section 8.2.2 Event 10 -- one expiry, one teardown. The
// session does not get a second hold time to be silent in.
//
// PREVENTS: a grace re-arm of ANY size. The window advanced through here is ten
// hold times, so a reprieve clamped to the negotiated hold time (what ze used
// to grant) fires a second expiry inside it, which both the armed-timer check
// and the second errChan signal catch.
func TestHoldExpiryGrantsNoSecondWindow(t *testing.T) {
	const hold = 90 * time.Second

	s, fc := newHoldExpirySession(t, hold)

	s.timers.StartHoldTimer()
	require.True(t, s.timers.IsHoldTimerRunning(), "precondition: hold timer armed")

	fc.Add(hold)
	require.False(t, s.timers.IsHoldTimerRunning(),
		"the first expiry is final: nothing may re-arm the hold timer")

	select {
	case err := <-s.errChan:
		require.ErrorIs(t, err, ErrHoldTimerExpired,
			"the first expiry must signal hold-timer expiry")
	default:
		t.Fatal("the first hold expiry must tear the session down")
	}

	// Ten further hold times. A reprieve of any size would expire inside this
	// window and put a second signal on errChan.
	fc.Add(10 * hold)
	require.False(t, s.timers.IsHoldTimerRunning())
	select {
	case err := <-s.errChan:
		t.Fatalf("a second hold expiry fired (%v): the timer was re-armed after "+
			"the teardown expiry, which is the reprieve RFC 4271 Section 8.2.2 "+
			"Event 10 does not allow", err)
	default:
	}
}
