package reactor

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
)

// keepaliveObservation is how long a test watches the wire for a periodic
// KEEPALIVE. The sessions below configure a one-second keepalive interval,
// so a running KeepaliveTimer fires at least once inside it.
const keepaliveObservation = 1600 * time.Millisecond

// establishWithHold drives a passive session configured with localHold and a
// one-second keepalive interval to Established against a peer OPEN whose Hold
// Time is peerHold seconds. It returns the session, its connection, and the
// octet count written before the observation starts (ze's OPEN and its
// KEEPALIVE answer).
func establishWithHold(t *testing.T, localHold time.Duration, peerHold uint16) (*Session, *recordingConn, int) {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = localHold
	settings.KeepaliveTime = time.Second
	session := NewSession(settings)
	require.NoError(t, session.Start())
	conn := &recordingConn{}
	require.NoError(t, session.Accept(conn))
	t.Cleanup(func() {
		require.NoError(t, session.Stop())
		session.closeConn()
	})

	body := validOpenBody()
	body[3] = byte(peerHold >> 8)
	body[4] = byte(peerHold)
	require.NoError(t, session.handleOpen(body))
	require.NoError(t, session.handleKeepalive())
	require.Equal(t, fsm.StateEstablished, session.State())
	return session, conn, len(conn.written())
}

// countKeepalives returns how many KEEPALIVE messages the stream holds.
func countKeepalives(t *testing.T, stream []byte) int {
	t.Helper()
	count := 0
	for _, m := range splitMessages(t, stream) {
		if m[18] == byte(msgtype.TypeKEEPALIVE) {
			count++
		}
	}
	return count
}

// Goal: prove a session whose negotiated Hold Time is non-zero sends periodic
// KEEPALIVEs, so the observation window below can see one.
// Method: ze configured with hold 90 s and keepalive 1 s meets a peer offering
// hold 9 s; after Established the wire is watched for 1.6 s.
//
// VALIDATES: negotiated hold 9 s, KeepaliveTimer running, at least one
// periodic KEEPALIVE written.
// PREVENTS: the zero-hold test below passing because keepalives never leave
// at all.
//
// RFC requirement: RFC4271-4.4-2 positive -- through handleOpen and handleKeepalive, ze configured hold 90 s / keepalive 1 s against a peer OPEN with Hold Time 9 negotiates 9 s, runs its KeepaliveTimer, and writes at least one periodic KEEPALIVE within 1.6 s of Established.
func TestRFC4271NonZeroNegotiatedHoldSendsPeriodicKeepalives(t *testing.T) {
	s, conn, before := establishWithHold(t, 90*time.Second, 9)
	require.Equal(t, 9*time.Second, s.timers.HoldTime())
	require.True(t, s.timers.IsKeepaliveTimerRunning())

	time.Sleep(keepaliveObservation)
	require.GreaterOrEqual(t, countKeepalives(t, conn.written()[before:]), 1)
}

// Goal: prove no periodic KEEPALIVE leaves a session whose NEGOTIATED Hold
// Time is zero, whichever side offered the zero, even with a configured
// keepalive interval that would fire within the window.
// Method: ze configured hold 90 s against a peer offering 0, then ze
// configured hold 0 against a peer offering 90; each session is established
// through handleOpen and handleKeepalive and its wire is watched for 1.6 s.
//
// VALIDATES: the timers carry hold 0, the KeepaliveTimer is not running, and
// nothing is written after Established.
// PREVENTS: the configured hold time, or the configured keepalive interval,
// reaching the timers in place of the negotiated zero.
//
// RFC requirement: RFC4271-4.4-2 negative -- a negotiated Hold Time of zero sends no periodic KEEPALIVE: ze hold 90 s against peer hold 0, and ze hold 0 against peer hold 90, both with a configured 1 s keepalive, reach Established with the timers' hold 0, the KeepaliveTimer stopped, and zero octets written in the 1.6 s after Established.
func TestRFC4271ZeroNegotiatedHoldSendsNoPeriodicKeepalive(t *testing.T) {
	cases := []struct {
		name      string
		localHold time.Duration
		peerHold  uint16
	}{
		{"peer offers zero", 90 * time.Second, 0},
		{"ze offers zero", 0, 90},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, conn, before := establishWithHold(t, tc.localHold, tc.peerHold)
			require.Equal(t, time.Duration(0), s.timers.HoldTime())
			require.False(t, s.timers.IsKeepaliveTimerRunning())

			time.Sleep(keepaliveObservation)
			require.Empty(t, conn.written()[before:], "no periodic KEEPALIVE with a negotiated Hold Time of zero")
		})
	}
}
