package reactor

import (
	"net"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/test/sim"
)

// openBodyWithBFDStrict is a valid OPEN from AS 65002 whose optional parameters
// carry the BFD Strict-Mode capability, code 74 with length 0
// (draft-ietf-idr-bgp-bfd-strict-mode Section 5). holdTime is the peer's Hold
// Time field, because Section 8.5.5 branches on the NEGOTIATED value being zero.
func openBodyWithBFDStrict(holdTime uint16) []byte {
	return []byte{
		0x04,       // Version 4
		0xFD, 0xEA, // MyAS = 65002
		byte(holdTime >> 8), byte(holdTime), // Hold Time
		0x01, 0x02, 0x03, 0x02, // BGP Identifier = 1.2.3.2
		0x04,       // OptParamLen = 4
		0x02, 0x02, // Optional Parameter: Capabilities, length 2
		byte(capability.CodeBFDStrictMode), 0x00, // capability 74, length 0
	}
}

// newStrictSession builds a session for a peer configured with BFD strict mode,
// already in OpenSent with a live pipe, and returns the client end so a case can
// see what reaches the wire. bfdUp decides what the peer's bfd.SessionState
// reader answers.
func newStrictSession(t *testing.T, holdTime time.Duration, bfdState api.State, bfdLive bool) (*Session, net.Conn) {
	t.Helper()
	session, client, _ := newStrictSessionOn(t, holdTime, 0, bfdState, bfdLive, nil)
	return session, client
}

// bfdTestState is the BFD session a strict-mode case presents to the session
// under test: the state its reader answers, and when that state was entered.
// Both are what production reads out of the peer's own bfdClient
// (peer_bfd.go, bfdSessionState), so a case can drive either.
type bfdTestState struct {
	state   *atomic.Int32
	entered *atomic.Int64
}

// Store sets the BFD session state the reader answers.
func (b *bfdTestState) Store(state int32) { b.state.Store(state) }

// enteredAt rewinds when the BFD session entered its state, so a case can
// present a hold-down interval that is already part or wholly served.
func (b *bfdTestState) enteredAt(when time.Time) { b.entered.Store(when.UnixNano()) }

// newStrictSessionOn is newStrictSession with the two knobs the hold-down and
// hold-timer cases need: the Section 10 hold-down interval, and a clock a case
// can drive. A nil clock leaves the session on the real one. The third return
// lets a case mutate what the bfd.SessionState reader answers mid-test, which is
// what a BFD session coming Up looks like from the session's side.
func newStrictSessionOn(
	t *testing.T,
	holdTime time.Duration,
	holdDown uint32,
	bfdState api.State,
	bfdLive bool,
	clk clock.Clock,
) (*Session, net.Conn, *bfdTestState) {
	t.Helper()

	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = holdTime
	settings.BFD = &BFDSettings{Enabled: true, Strict: true, HoldTime: 5, HoldDown: holdDown}
	settings.Capabilities = []capability.Capability{&capability.BFDStrictMode{}}

	// The BFD session is treated as having entered its state now, so a
	// hold-down interval measured from it is owed in full. A case that wants
	// an interval already served rewinds it with enteredAt.
	bfd := &bfdTestState{state: &atomic.Int32{}, entered: &atomic.Int64{}}
	bfd.state.Store(int32(bfdState))
	// From the SAME clock the session measures with. Seeding this from the wall
	// clock while the session reads an injected one puts the entry time a hair
	// in the future, so the hold-down remainder exceeds the whole interval and
	// no amount of fc.Add fires it.
	entered := time.Now()
	if clk != nil {
		entered = clk.Now()
	}
	bfd.entered.Store(entered.UnixNano())

	session := NewSession(settings)
	if clk != nil {
		session.SetClock(clk)
	}
	session.setBFDStateReader(func() (api.State, time.Time, bool) {
		return api.State(bfd.state.Load()), time.Unix(0, bfd.entered.Load()), bfdLive //nolint:gosec // the test writes only api.State values
	})
	require.NoError(t, session.Start())

	client, server := net.Pipe()
	_ = acceptWithReader(t, session, server, client)
	require.Equal(t, fsm.StateOpenSent, session.State())

	t.Cleanup(func() {
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	})
	return session, client, bfd
}

// drain reads and discards everything the session writes, so a handler that
// writes to the pipe never blocks. It returns a channel carrying every message
// the session sent, so a case can assert on the bytes instead of on nothing.
func drainMessages(t *testing.T, client net.Conn) <-chan []byte {
	t.Helper()
	out := make(chan []byte, 8)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := client.Read(buf)
			if err != nil {
				close(out)
				return
			}
			out <- append([]byte(nil), buf[:n]...)
		}
	}()
	return out
}

// newEstablishedSessionForPeer drives a real Session for p's own settings all
// the way to Established over a pipe, and publishes it as the peer's live
// session. It is what lets a BFD subscriber test assert on the wire: the
// subscriber delivers its FSM event to p.session, so a peer with no session has
// nowhere to deliver and nothing to assert.
func newEstablishedSessionForPeer(t *testing.T, p *Peer) (*Session, <-chan []byte) {
	t.Helper()

	session := NewSession(p.settings)
	session.setBFDStateReader(p.bfdSessionState)
	require.NoError(t, session.Start())

	client, server := net.Pipe()
	_ = acceptWithReader(t, session, server, client)

	// ONE reader for the pipe, returned to the caller: a second drain goroutine
	// would race this one for each message and the caller would see half of them.
	messages := drainMessages(t, client)
	require.NoError(t, session.handleOpen(validOpenBody()))
	require.NoError(t, session.handleKeepalive())
	require.Equal(t, fsm.StateEstablished, session.State())
	<-messages // the KEEPALIVE handleOpen sent, so a case reads only what follows

	p.mu.Lock()
	p.session = session
	p.mu.Unlock()

	t.Cleanup(func() {
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	})
	return session, messages
}

// TestSessionBFDStrictWithholdsKeepalive is draft
// draft-ietf-idr-bgp-bfd-strict-mode Section 8.5.5 on the wire: "DOES NOT send a
// KEEPALIVE message, and DOES NOT start the KeepaliveTimer ... stays in OpenSent
// state (OpenSentBfdUpPending)".
//
// VALIDATES: With strict mode negotiated and the BFD session Down, handleOpen
// leaves the FSM in OpenSent with the pending sub-state set, starts no keepalive
// timer, and puts no KEEPALIVE on the wire.
//
// PREVENTS: The gate being wired only in the FSM while the session still sends
// the KEEPALIVE, which would tell the peer to establish a session ze is holding.
//
// RFC requirement: DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-1 positive -- with
// strict-mode enabled for this peer the BGP FSM IS enabled: it left Idle, took
// the TCP connection, sent its OPEN, processed the peer's, and sits in OpenSent.
// Only the last advance waits for BFD (internal/component/bgp/reactor/
// session_bfd_strict.go, advanceAfterOpen).
// RFC requirement: DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-2 positive -- the BFD
// session being down does not prevent BGP from establishing a CONNECTION with
// the remote speaker: the connection is up and the OPEN exchange completed while
// bfd.SessionState is Down, which is the deadlock Section 10 forbids.
func TestSessionBFDStrictWithholdsKeepalive(t *testing.T) {
	session, client := newStrictSession(t, 90*time.Second, api.StateDown, true)
	messages := drainMessages(t, client)

	require.NoError(t, session.handleOpen(openBodyWithBFDStrict(90)))

	require.True(t, session.Negotiated().BFDStrictMode, "both sides advertised code 74")
	require.Equal(t, fsm.StateOpenSent, session.State())
	require.Equal(t, fsm.SubStateOpenSentBfdUpPending, session.fsm.BfdSubState())
	require.False(t, session.timers.IsKeepaliveTimerRunning())

	select {
	case msg := <-messages:
		t.Fatalf("no message is owed while BFD is down, got % x", msg)
	case <-time.After(100 * time.Millisecond):
	}
}

// TestSessionBFDStrictSendsKeepaliveOnBFDUp is the release half of Section
// 8.5.1: "sends a KEEPALIVE message ... and changes its state to OpenConfirm".
//
// VALIDATES: A BfdUp event delivered to a held session puts one KEEPALIVE on the
// wire and moves the FSM to OpenConfirm.
//
// PREVENTS: A session that advances its state without telling the peer, which
// would sit in OpenConfirm until its hold timer expired.
//
// RFC requirement: DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-1 negative -- the FSM
// is not merely enabled, it RUNS to completion: the wait it was holding ends the
// moment BFD reports Up, so no state ze can reach under strict mode is one it
// cannot leave (internal/component/bgp/reactor/session_bfd_strict.go,
// handleBFDEvent).
func TestSessionBFDStrictSendsKeepaliveOnBFDUp(t *testing.T) {
	session, client := newStrictSession(t, 90*time.Second, api.StateDown, true)
	messages := drainMessages(t, client)

	require.NoError(t, session.handleOpen(openBodyWithBFDStrict(90)))
	require.Equal(t, fsm.SubStateOpenSentBfdUpPending, session.fsm.BfdSubState())

	require.NoError(t, session.handleBFDEvent(fsm.EventBfdUp))
	require.Equal(t, fsm.StateOpenConfirm, session.State())

	select {
	case msg := <-messages:
		require.Equal(t, byte(msgtype.TypeKEEPALIVE), msg[18], "the withheld KEEPALIVE is what goes out")
	case <-time.After(time.Second):
		t.Fatal("no KEEPALIVE reached the wire after BFD came up")
	}
}

// TestSessionBFDStrictConfirmedPathReachesEstablished is Section 8.5.6 followed
// by Section 8.5.1's second branch: the peer's KEEPALIVE arrives first, so the
// next BfdUp goes straight to Established.
//
// VALIDATES: handleKeepalive in the pending sub-state confirms rather than
// erroring, and the following BfdUp sends the KEEPALIVE, starts the keepalive
// timer and reaches Established.
//
// PREVENTS: A session stuck in OpenSent forever because the peer, having already
// confirmed, will send nothing more.
func TestSessionBFDStrictConfirmedPathReachesEstablished(t *testing.T) {
	session, client := newStrictSession(t, 90*time.Second, api.StateDown, true)
	messages := drainMessages(t, client)

	require.NoError(t, session.handleOpen(openBodyWithBFDStrict(90)))
	require.NoError(t, session.handleKeepalive())
	require.Equal(t, fsm.StateOpenSent, session.State())
	require.Equal(t, fsm.SubStateOpenSentConfirmedBfdUpPending, session.fsm.BfdSubState())

	require.NoError(t, session.handleBFDEvent(fsm.EventBfdUp))
	require.Equal(t, fsm.StateEstablished, session.State())
	require.True(t, session.timers.IsKeepaliveTimerRunning(),
		"Section 8.5.1 sets a KeepaliveTimer, and no handleKeepalive runs on this path")

	select {
	case msg := <-messages:
		require.Equal(t, byte(msgtype.TypeKEEPALIVE), msg[18])
	case <-time.After(time.Second):
		t.Fatal("no KEEPALIVE reached the wire on the confirmed path")
	}
}

// TestSessionBFDStrictEstablishesWhenPeerDoesNotAdvertise is Section 6's other
// half, and Section 1's reason for it: "always using 'strict-mode' would
// preclude BGP operation in an environment where not all routers support BFD
// strict-mode".
//
// VALIDATES: With BFD down and strict configured locally, a peer OPEN carrying
// no capability 74 establishes on the unmodified RFC 4271 rail: KEEPALIVE sent,
// state OpenConfirm, no sub-state.
//
// PREVENTS: The local configuration alone gating the session, which is the
// design this spec replaced and which would black-hole every peering against a
// speaker that does not implement the draft.
//
// RFC requirement: DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-10-2 negative -- where
// strict mode is NOT negotiated the wait does not bind at all, so a BFD session
// that is Down prevents nothing: the session advances to OpenConfirm with the
// KEEPALIVE on the wire. This is the case the deadlock rule exists to keep
// reachable (internal/component/bgp/reactor/session_bfd_strict.go,
// bfdStrictHolds).
func TestSessionBFDStrictEstablishesWhenPeerDoesNotAdvertise(t *testing.T) {
	session, client := newStrictSession(t, 90*time.Second, api.StateDown, true)
	messages := drainMessages(t, client)

	require.NoError(t, session.handleOpen(validOpenBody()))
	require.False(t, session.Negotiated().BFDStrictMode)
	require.Equal(t, fsm.StateOpenConfirm, session.State())
	require.Equal(t, fsm.SubStateNone, session.fsm.BfdSubState())

	select {
	case msg := <-messages:
		require.Equal(t, byte(msgtype.TypeKEEPALIVE), msg[18])
	case <-time.After(time.Second):
		t.Fatal("a peer that does not run strict mode must not be held")
	}
}

// TestSessionBFDStrictProceedsWhenBFDIsAlreadyUp reads the third clause of
// Section 8.5.5: the wait applies only while "bfd.SessionState is neither Up nor
// AdminDown".
//
// VALIDATES: A BFD session already Up, and one AdminDown, both let the OPEN rail
// send the KEEPALIVE and advance.
//
// PREVENTS: A strict peer waiting for an Up event that already happened before
// the OPEN arrived, which Section 7's early start makes the common case.
func TestSessionBFDStrictProceedsWhenBFDIsAlreadyUp(t *testing.T) {
	for _, state := range []api.State{api.StateUp, api.StateAdminDown} {
		t.Run(state.String(), func(t *testing.T) {
			session, client := newStrictSession(t, 90*time.Second, state, true)
			drainMessages(t, client)

			require.NoError(t, session.handleOpen(openBodyWithBFDStrict(90)))
			require.True(t, session.Negotiated().BFDStrictMode)
			require.Equal(t, fsm.StateOpenConfirm, session.State())
		})
	}
}

// TestSessionBFDStrictHoldsWhenNoBFDSessionExists is the fail-closed case: the
// operator asked for a gate and no gate is running.
//
// VALIDATES: A strict session whose bfd.SessionState reader answers "no session"
// holds in OpenSent.
//
// PREVENTS: A missing BFD plugin turning a strict peer into an ordinary one,
// which delivers the opposite of what was configured and does it silently
// (ai/rules/principles.md).
func TestSessionBFDStrictHoldsWhenNoBFDSessionExists(t *testing.T) {
	session, client := newStrictSession(t, 90*time.Second, api.StateAdminDown, false)
	drainMessages(t, client)

	require.NoError(t, session.handleOpen(openBodyWithBFDStrict(90)))
	require.Equal(t, fsm.StateOpenSent, session.State())
	require.Equal(t, fsm.SubStateOpenSentBfdUpPending, session.fsm.BfdSubState())
}

// TestSessionBFDStrictArmsBfdHoldTimerOnZeroHoldTime is the last clause of
// Section 8.5.5: "if the HoldTimer negotiated value is zero, starts the
// BfdHoldTimer with the value BfdHoldTime".
//
// VALIDATES: A negotiated hold time of zero arms the BfdHoldTimer at the
// configured BfdHoldTime, and a non-zero one does not arm it at all.
//
// PREVENTS: A session with no hold timer waiting for BFD with nothing to bound
// it, and the mirror mistake of arming a second timer where the BGP hold timer
// already bounds the wait.
func TestSessionBFDStrictArmsBfdHoldTimerOnZeroHoldTime(t *testing.T) {
	zero, zeroClient := newStrictSession(t, 0, api.StateDown, true)
	drainMessages(t, zeroClient)
	require.NoError(t, zero.handleOpen(openBodyWithBFDStrict(0)))
	require.True(t, zero.timers.IsBfdHoldTimerRunning())
	require.Equal(t, 5*time.Second, zero.timers.BfdHoldTime(), "the peer's hold-time leaf, in seconds")

	nonZero, nonZeroClient := newStrictSession(t, 90*time.Second, api.StateDown, true)
	drainMessages(t, nonZeroClient)
	require.NoError(t, nonZero.handleOpen(openBodyWithBFDStrict(90)))
	require.False(t, nonZero.timers.IsBfdHoldTimerRunning(),
		"a non-zero negotiated hold time already bounds the wait")
}

// TestSessionBFDStrictDownClosesWithBFDDownSubcode is draft Section 8.5.2 and
// Section 9: the session closes with "the Cease Code (6) and the 'BFD Down'
// Subcode (10)".
//
// VALIDATES: A BfdDown event on a held session puts a NOTIFICATION carrying
// Cease and subcode 10 on the wire and leaves the FSM in Idle.
//
// PREVENTS: A silent close. Section 9 gives the reason: the subcode "informs the
// operator that interaction with BFD is the root cause of the BGP session being
// unable to move to the Established state".
func TestSessionBFDStrictDownClosesWithBFDDownSubcode(t *testing.T) {
	session, client := newStrictSession(t, 90*time.Second, api.StateDown, true)
	messages := drainMessages(t, client)

	require.NoError(t, session.handleOpen(openBodyWithBFDStrict(90)))
	require.NoError(t, session.handleBFDEvent(fsm.EventBfdDown))
	require.Equal(t, fsm.StateIdle, session.State())

	select {
	case msg := <-messages:
		require.Equal(t, byte(msgtype.TypeNOTIFICATION), msg[18])
		require.Equal(t, byte(message.NotifyCease), msg[19])
		require.Equal(t, message.NotifyCeaseBFDDown, msg[20])
	case <-time.After(time.Second):
		t.Fatal("no NOTIFICATION reached the wire")
	}
}

// TestSessionBFDStrictConfigChangedUsesConfigSubcode is draft Section 8.5.4,
// the one clause of this feature that does NOT use subcode 10.
//
// VALIDATES: raiseBFDStrictConfigChanged sends Cease with Other Configuration
// Change (6) and drops the session to Idle.
//
// PREVENTS: Reporting an operator's configuration change to the peer as a
// forwarding-path failure.
//
// RFC requirement: DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-4-1 positive -- with
// BfdEnabled TRUE, raiseBFDStrictConfigChanged raises Event 35, which the
// OpenSent handler answers with Cease / Other Configuration Change and a drop to
// Idle; the event is permitted to occur exactly in the case the MUST NOT does
// not forbid (internal/component/bgp/reactor/session_bfd_strict.go,
// raiseBFDStrictConfigChanged).
func TestSessionBFDStrictConfigChangedUsesConfigSubcode(t *testing.T) {
	session, client := newStrictSession(t, 90*time.Second, api.StateDown, true)
	messages := drainMessages(t, client)

	require.NoError(t, session.handleOpen(openBodyWithBFDStrict(90)))
	session.raiseBFDStrictConfigChanged()
	require.Equal(t, fsm.StateIdle, session.State())

	select {
	case msg := <-messages:
		require.Equal(t, byte(msgtype.TypeNOTIFICATION), msg[18])
		require.Equal(t, byte(message.NotifyCease), msg[19])
		require.Equal(t, message.NotifyCeaseOtherConfigChange, msg[20])
	case <-time.After(time.Second):
		t.Fatal("no NOTIFICATION reached the wire")
	}
}

// TestBFDStrictConfigChangedComparison covers the predicate the reload path uses
// to decide whether draft Event 35 is owed.
//
// VALIDATES: A change to strict, hold-time or enabled counts; gaining or losing
// the whole bfd block counts; an unrelated change does not; and a nil side
// answers false rather than guessing.
//
// PREVENTS: The event firing on every reload of every peer, which would put a
// NOTIFICATION on sessions the change never touched.
func TestBFDStrictConfigChangedComparison(t *testing.T) {
	base := func() *PeerSettings {
		s := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 1)
		s.BFD = &BFDSettings{Enabled: true, Strict: true, HoldTime: 30}
		return s
	}

	same := base()
	require.False(t, bfdStrictConfigChanged(base(), same))

	strictOff := base()
	strictOff.BFD.Strict = false
	require.True(t, bfdStrictConfigChanged(base(), strictOff))

	holdChanged := base()
	holdChanged.BFD.HoldTime = 31
	require.True(t, bfdStrictConfigChanged(base(), holdChanged))

	disabled := base()
	disabled.BFD.Enabled = false
	require.True(t, bfdStrictConfigChanged(base(), disabled))

	noBFD := base()
	noBFD.BFD = nil
	require.True(t, bfdStrictConfigChanged(base(), noBFD))
	require.True(t, bfdStrictConfigChanged(noBFD, base()))

	unrelated := base()
	unrelated.BFD.MinTTL = 250
	require.False(t, bfdStrictConfigChanged(base(), unrelated))

	require.False(t, bfdStrictConfigChanged(nil, base()))
	require.False(t, bfdStrictConfigChanged(base(), nil))
}

// TestSessionBFDStrictConfigChangedWithBFDDisabled is the MUST NOT of draft
// Section 4, Event 35: "If BfdEnabled is FALSE, this event MUST NOT occur. When
// BFD has been disabled, the local system will trigger a BfdAdminDown event
// instead."
//
// VALIDATES: With the peer's bfd block disabled, the config-change path raises
// BfdAdminDown and NOT Event 35, so the session sends no NOTIFICATION at all and
// stays where it was. Event 35's own clause would have sent Cease / Other
// Configuration Change and dropped to Idle, so the two outcomes are
// distinguishable on the wire.
//
// PREVENTS: The one producer of Event 35 raising it for a peer that has BFD
// switched off, which is the only way ze could violate this clause.
//
// RFC requirement: DRAFT-IETF-IDR-BGP-BFD-STRICT-MODE-4-1 negative -- with
// BfdEnabled FALSE the event does not occur: raiseBFDStrictConfigChanged
// substitutes BfdAdminDown, which draft Section 8.5.1 answers with nothing while
// no sub-state is pending, so no Cease / Other Configuration Change reaches the
// wire and the state does not move
// (internal/component/bgp/reactor/session_bfd_strict.go,
// raiseBFDStrictConfigChanged).
func TestSessionBFDStrictConfigChangedWithBFDDisabled(t *testing.T) {
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Connection = ConnectionPassive
	settings.ReceiveHoldTime = 90 * time.Second
	settings.BFD = &BFDSettings{Enabled: false, Strict: true}

	session := NewSession(settings)
	require.NoError(t, session.Start())

	client, server := net.Pipe()
	_ = acceptWithReader(t, session, server, client)
	t.Cleanup(func() {
		client.Close() //nolint:errcheck // test cleanup
		server.Close() //nolint:errcheck // test cleanup
	})
	messages := drainMessages(t, client)

	session.raiseBFDStrictConfigChanged()

	require.Equal(t, fsm.StateOpenSent, session.State(),
		"a BfdAdminDown outside a pending sub-state changes nothing (draft Section 8.5.1)")
	select {
	case msg := <-messages:
		t.Fatalf("Event 35 MUST NOT occur with BfdEnabled FALSE, yet a message went out: % x", msg)
	case <-time.After(300 * time.Millisecond):
	}
}

// establishFromOpenConfirm drives a strict session from OpenSent to the point
// where only the Section 10 hold-down can hold it: the OPEN is processed, BFD is
// up, and the peer's KEEPALIVE has arrived in OpenConfirm.
func establishFromOpenConfirm(t *testing.T, session *Session, bfd *bfdTestState, holdTime uint16) {
	t.Helper()
	require.NoError(t, session.handleOpen(openBodyWithBFDStrict(holdTime)))
	if session.fsm.BfdSubState() != fsm.SubStateNone {
		bfd.Store(int32(api.StateUp))
		require.NoError(t, session.handleBFDEvent(fsm.EventBfdUp))
	}
	require.Equal(t, fsm.StateOpenConfirm, session.State(), "the wait for BFD is over; only the hold-down is left")
	require.NoError(t, session.handleKeepalive())
}

// TestSessionBFDStrictHoldDownDelaysEstablishment is the positive half of the
// BFD hold-down interval of draft-ietf-idr-bgp-bfd-strict-mode Section 10: "the
// BGP state machine is permitted to transition to the Established state from the
// OpenConfirm state after the locally configured BFD hold-down interval is
// observed. That is, the BFD session has been Up for the desired amount of time."
//
// VALIDATES: With a 400 ms hold-down, the KEEPALIVE that would take the session
// from OpenConfirm to Established is withheld, the interval is armed, and only
// when it elapses does the session establish.
//
// PREVENTS: A hold-down leaf that parses and is then ignored, which would
// publish a damping feature that damps nothing.
func TestSessionBFDStrictHoldDownDelaysEstablishment(t *testing.T) {
	fc := sim.NewFakeClock(time.Now())
	session, client, bfd := newStrictSessionOn(t, 90*time.Second, 400, api.StateDown, true, fc)
	drainMessages(t, client)

	establishFromOpenConfirm(t, session, bfd, 90)

	require.True(t, session.timers.IsBfdHoldDownTimerRunning(), "the transition armed the interval")
	require.Equal(t, fsm.StateOpenConfirm, session.State(), "the interval has not been observed yet")

	fc.Add(400 * time.Millisecond)

	require.Eventually(t, func() bool { return session.State() == fsm.StateEstablished },
		2*time.Second, 10*time.Millisecond,
		"the interval is observed, so the transition is now permitted")
}

// TestSessionBFDStrictHoldDownNotBypassedWhenBFDIsAlreadyUp is the case the
// interval exists for, and the one an OPEN-rail gate silently skipped.
//
// draft-ietf-idr-bgp-bfd-strict-mode Section 7 has ze open the BFD session
// before the FSM starts and keep it open across a teardown, so on a
// connect-retry after a BFD-driven flap the session is Up again before the new
// OPEN arrives. A gate on the OPEN rail sees "already Up", holds nothing, and
// the damping never runs on the only path that needed it.
//
// VALIDATES: With BFD Up at the moment the OPEN is processed, the session still
// serves the interval before it establishes.
//
// PREVENTS: A flapping link carrying a BGP session, which is what
// docs/guide/bfd.md promises it cannot.
func TestSessionBFDStrictHoldDownNotBypassedWhenBFDIsAlreadyUp(t *testing.T) {
	fc := sim.NewFakeClock(time.Now())
	session, client, bfd := newStrictSessionOn(t, 90*time.Second, 400, api.StateUp, true, fc)
	drainMessages(t, client)

	require.NoError(t, session.handleOpen(openBodyWithBFDStrict(90)))
	require.Equal(t, fsm.SubStateNone, session.fsm.BfdSubState(),
		"BFD was already Up, so Section 8.5.5 withholds nothing")
	require.Equal(t, fsm.StateOpenConfirm, session.State())
	_ = bfd

	require.NoError(t, session.handleKeepalive())
	require.True(t, session.timers.IsBfdHoldDownTimerRunning(), "the interval is still owed")
	require.Equal(t, fsm.StateOpenConfirm, session.State())

	fc.Add(400 * time.Millisecond)
	require.Eventually(t, func() bool { return session.State() == fsm.StateEstablished },
		2*time.Second, 10*time.Millisecond)
}

// TestSessionBFDStrictHoldDownCountsTimeAlreadyServed reads the sentence
// literally: the interval measures how long "the BFD session has been Up", not
// how long ago this BGP connection noticed.
//
// VALIDATES: A BFD session that came Up well before the OPEN owes no wait, and
// one that came Up part-way through owes only the remainder.
//
// PREVENTS: Restarting a wait the link has already served on every connect
// retry, which would delay every reconnection by the whole interval.
func TestSessionBFDStrictHoldDownCountsTimeAlreadyServed(t *testing.T) {
	fc := sim.NewFakeClock(time.Now())
	session, client, bfd := newStrictSessionOn(t, 90*time.Second, 400, api.StateUp, true, fc)
	drainMessages(t, client)
	// The BFD session came Up a full second ago, which is more than the 400 ms
	// interval, so nothing is owed.
	bfd.enteredAt(time.Now().Add(-time.Second))

	establishFromOpenConfirm(t, session, bfd, 90)

	require.False(t, session.timers.IsBfdHoldDownTimerRunning(), "the interval was already served")
	require.Equal(t, fsm.StateEstablished, session.State())
}

// TestSessionBFDStrictHoldDownAbsentEstablishesOnFirstUp is the negative half:
// the interval binds only where it is configured.
//
// VALIDATES: With no hold-down leaf (zero), the OpenConfirm KEEPALIVE
// establishes at once and no hold-down timer is armed.
//
// PREVENTS: The hold-down defaulting to something non-zero, which would delay
// every strict peer in the field by an interval nobody configured. Section 10
// offers the interval as something that "may help reduce the frequency of BGP
// session flaps", never as something every session runs.
func TestSessionBFDStrictHoldDownAbsentEstablishesOnFirstUp(t *testing.T) {
	fc := sim.NewFakeClock(time.Now())
	session, client, bfd := newStrictSessionOn(t, 90*time.Second, 0, api.StateDown, true, fc)
	drainMessages(t, client)

	establishFromOpenConfirm(t, session, bfd, 90)

	require.False(t, session.timers.IsBfdHoldDownTimerRunning(), "a zero interval arms nothing")
	require.Equal(t, fsm.StateEstablished, session.State())
}

// TestSessionBFDStrictBfdHoldTimerTearsDownAZeroHoldTimeSession is FSM Event 34
// driven through the SESSION, which is the only path that can reach it in
// production.
//
// draft-ietf-idr-bgp-bfd-strict-mode Section 3, attribute 19 (BfdHoldTimer):
// "Hold timer used when the BGP HoldTime has been negotiated to zero to ensure
// the BGP session terminates if the associated BFD session does not enter the Up
// state." Section 8.5.3 is its action list: Cease (6) / BFD Down (10), drop the
// connection, increment the ConnectRetryCounter, go to Idle.
//
// VALIDATES: With a negotiated hold time of zero and a BFD session that never
// comes up, the BfdHoldTimer fires, a Cease / BFD Down NOTIFICATION reaches the
// wire, and the session goes to Idle.
//
// PREVENTS: The hang this feature shipped with for one review round. The timer
// was armed and its callback was never registered, so `fireBfdHold` read a nil
// callback and returned: the three Event 34 arms in the FSM were unreachable,
// and with the negotiated hold time zero RFC 4271's own HoldTimer arms nothing
// either, so the session waited in OpenSent forever. Arming and firing were each
// tested and their connection was not, which is why this case drives the session
// rather than a bare Timers.
func TestSessionBFDStrictBfdHoldTimerTearsDownAZeroHoldTimeSession(t *testing.T) {
	fc := sim.NewFakeClock(time.Now())
	session, client, _ := newStrictSessionOn(t, 0, 0, api.StateDown, true, fc)
	messages := drainMessages(t, client)

	// Hold time zero on both sides, so RFC 4271 Section 4.2 negotiates zero and
	// no HoldTimer runs. The BfdHoldTimer is the only bound left.
	require.NoError(t, session.handleOpen(openBodyWithBFDStrict(0)))
	require.Equal(t, fsm.SubStateOpenSentBfdUpPending, session.fsm.BfdSubState())
	require.True(t, session.timers.IsBfdHoldTimerRunning())
	require.Equal(t, 5*time.Second, session.timers.BfdHoldTime())

	fc.Add(5 * time.Second)

	require.Eventually(t, func() bool { return session.State() == fsm.StateIdle },
		2*time.Second, 10*time.Millisecond,
		"the BfdHoldTimer is the whole bound when the negotiated hold time is zero")

	var sawBFDDown bool
	for range 4 {
		select {
		case msg := <-messages:
			if len(msg) > 20 && msg[18] == byte(msgtype.TypeNOTIFICATION) &&
				msg[19] == byte(message.NotifyCease) && msg[20] == message.NotifyCeaseBFDDown {
				sawBFDDown = true
			}
		case <-time.After(500 * time.Millisecond):
		}
		if sawBFDDown {
			break
		}
	}
	require.True(t, sawBFDDown, "Section 8.5.3 sends Cease (6) with subcode BFD Down (10)")
}

// TestSessionBFDStrictSecondOpenIsAnFSMErrorOnTheWire is draft
// Section 8.5.5's first branch, driven through handleOpen rather than through a
// bare FSM.
//
// VALIDATES: A second OPEN arriving while a pending sub-state is set answers
// NOTIFICATION with Finite State Machine Error (code 5) and drops to Idle, and
// the sub-state is not re-entered.
//
// PREVENTS: Two things the bare-FSM test could not see. The re-negotiation gate
// in handleOpen fires only in Established and OpenConfirm, and strict mode keeps
// the state at OpenSent, so a second OPEN re-ran negotiateWith on a live
// session -- the exact mid-session capability change that gate exists to stop.
// And EnterBfdUpPending succeeded again, DEMOTING an
// OpenSentConfirmedBfdUpPending sub-state back to pending, so the next BFD Up
// would have gone to OpenConfirm against a peer that had already confirmed.
func TestSessionBFDStrictSecondOpenIsAnFSMErrorOnTheWire(t *testing.T) {
	for _, tc := range []struct {
		name    string
		confirm bool
	}{{"OpenSentBfdUpPending", false}, {"OpenSentConfirmedBfdUpPending", true}} {
		t.Run(tc.name, func(t *testing.T) {
			session, client, _ := newStrictSessionOn(t, 90*time.Second, 0, api.StateDown, true, nil)
			messages := drainMessages(t, client)

			require.NoError(t, session.handleOpen(openBodyWithBFDStrict(90)))
			if tc.confirm {
				require.NoError(t, session.handleKeepalive())
				require.Equal(t, fsm.SubStateOpenSentConfirmedBfdUpPending, session.fsm.BfdSubState())
			}
			negotiated := session.Negotiated()

			err := session.handleOpen(openBodyWithBFDStrict(90))
			require.ErrorIs(t, err, ErrInvalidState)
			require.Equal(t, fsm.StateIdle, session.State())
			require.Same(t, negotiated, session.Negotiated(),
				"the second OPEN must not have re-run negotiation")

			select {
			case msg := <-messages:
				require.Equal(t, byte(msgtype.TypeNOTIFICATION), msg[18])
				require.Equal(t, byte(message.NotifyFSMError), msg[19])
			case <-time.After(time.Second):
				t.Fatal("no Finite State Machine Error NOTIFICATION reached the wire")
			}
		})
	}
}

// TestSessionBFDStrictHoldIsBoundedByTheBGPHoldTimer answers the question the
// whole design turns on: what ends the wait when BFD never comes up?
//
// The BfdHoldTimer of draft Section 3 attribute 19 is armed ONLY when the
// negotiated BGP hold time is zero -- the draft says so in that attribute's own
// definition, "Hold timer used when the BGP HoldTime has been negotiated to zero
// to ensure the BGP session terminates if the associated BFD session does not
// enter the Up state", and Section 8.5.5 arms it only in that branch. In every
// other case the bound is RFC 4271's own HoldTimer, which
// Session.advanceAfterOpen re-arms to the negotiated value on the way into the
// wait, exactly as the unmodified rail does.
//
// VALIDATES: A strict session waiting on a BFD that never comes up leaves
// OpenSent for Idle when the negotiated BGP hold timer expires, and the peer is
// told with NOTIFICATION code 4 (Hold Timer Expired).
//
// PREVENTS: A hang. Without this the feature would ship a session that waits
// forever for a BFD session the far end never brings up, holding a TCP
// connection and a peer slot with nothing to end it.
//
// RFC 4271 Section 8.2.2, OpenSent, Event 10: "sends a NOTIFICATION message with
// the error code Hold Timer Expired, sets the ConnectRetryTimer to zero,
// releases all BGP resources, drops the TCP connection, increments the
// ConnectRetryCounter, ... and changes its state to Idle".
func TestSessionBFDStrictHoldIsBoundedByTheBGPHoldTimer(t *testing.T) {
	// Seeded at the wall clock, not the epoch: the session computes its socket
	// write deadlines from this clock, and a deadline in 1970 is already past,
	// so the OPEN write fails before the case starts.
	fc := sim.NewFakeClock(time.Now())
	session, client, _ := newStrictSessionOn(t, 90*time.Second, 0, api.StateDown, true, fc)
	messages := drainMessages(t, client)

	// The peer's OPEN carries hold time 30, so the negotiated value is 30s: the
	// smaller of the two (RFC 4271 Section 4.2).
	require.NoError(t, session.handleOpen(openBodyWithBFDStrict(30)))
	require.Equal(t, fsm.SubStateOpenSentBfdUpPending, session.fsm.BfdSubState())
	require.True(t, session.timers.IsHoldTimerRunning(),
		"advanceAfterOpen re-armed the BGP hold timer to the negotiated value")
	require.False(t, session.timers.IsBfdHoldTimerRunning(),
		"a non-zero negotiated hold time means the BfdHoldTimer is not the bound")

	fc.Add(30 * time.Second)

	require.Eventually(t, func() bool { return session.State() == fsm.StateIdle },
		2*time.Second, 10*time.Millisecond,
		"the BGP hold timer is what ends a wait for a BFD session that never comes up")

	var sawHoldTimerExpired bool
	for range 4 {
		select {
		case msg := <-messages:
			if len(msg) > 19 && msg[18] == byte(msgtype.TypeNOTIFICATION) &&
				msg[19] == byte(message.NotifyHoldTimerExpired) {
				sawHoldTimerExpired = true
			}
		case <-time.After(500 * time.Millisecond):
		}
		if sawHoldTimerExpired {
			break
		}
	}
	require.True(t, sawHoldTimerExpired, "the peer is told why the session ended")
}

// TestSessionBFDStrictProceedsWhenBFDWasAlreadyUpBeforeTheSession is the
// round-2 blocker: a BFD session another client already brought Up.
//
// api.SessionHandle.Subscribe now delivers the current state as its first
// value, so the seed is the engine's own fact. Before that, startBFDClient
// stored Down unconditionally, EnsureSession on an existing key only bumped a
// refcount, and Subscribe replayed nothing -- so the peer read Down until the
// next TRANSITION, which for a stable link never comes.
//
// VALIDATES: With the reader answering Up and an entry time from before this
// connection, the OPEN rail withholds nothing and the session advances.
//
// PREVENTS: A peer that can never establish. It would hit the BGP hold timer,
// drop to Idle, retry, and repeat for the life of the process, and the
// configuration that reaches it -- a top-level `bfd { session ... }` entry to
// the same neighbor -- is supported and ordinary.
func TestSessionBFDStrictProceedsWhenBFDWasAlreadyUpBeforeTheSession(t *testing.T) {
	session, client, bfd := newStrictSessionOn(t, 90*time.Second, 0, api.StateUp, true, nil)
	messages := drainMessages(t, client)
	bfd.enteredAt(time.Now().Add(-time.Minute))

	require.NoError(t, session.handleOpen(openBodyWithBFDStrict(90)))
	require.True(t, session.Negotiated().BFDStrictMode)
	require.Equal(t, fsm.SubStateNone, session.fsm.BfdSubState(),
		"a session that is already Up has nothing to wait for")
	require.Equal(t, fsm.StateOpenConfirm, session.State())

	select {
	case msg := <-messages:
		require.Equal(t, byte(msgtype.TypeKEEPALIVE), msg[18])
	case <-time.After(time.Second):
		t.Fatal("the KEEPALIVE was withheld for a BFD session that was already up")
	}
}

// TestSessionBFDStrictHoldDownRestartsOnAFlapInsideTheInterval covers the case
// draft-ietf-idr-bgp-bfd-strict-mode Section 10 exists for: Up, Down, Up inside
// the interval.
//
// VALIDATES: The interval is measured from the LATEST entry into Up, so a
// session that dropped and came back owes the whole interval again rather than
// counting the time it spent down.
//
// PREVENTS: A flapping link establishing early by accumulating credit across
// its own outages, which would make the damping report success on exactly the
// link it exists to refuse. The producer is runBFDSubscriber's re-stamp of the
// entry time, which fires only on a state CHANGE.
func TestSessionBFDStrictHoldDownRestartsOnAFlapInsideTheInterval(t *testing.T) {
	fc := sim.NewFakeClock(time.Now())
	session, client, bfd := newStrictSessionOn(t, 90*time.Second, 400, api.StateUp, true, fc)
	drainMessages(t, client)

	// Up for 300ms of a 400ms interval, then a flap: the new Up is now.
	bfd.enteredAt(fc.Now().Add(-300 * time.Millisecond))
	bfd.Store(int32(api.StateDown))
	bfd.Store(int32(api.StateUp))
	bfd.enteredAt(fc.Now())

	require.NoError(t, session.handleOpen(openBodyWithBFDStrict(90)))
	require.NoError(t, session.handleKeepalive())
	require.True(t, session.timers.IsBfdHoldDownTimerRunning())
	require.Equal(t, fsm.StateOpenConfirm, session.State())

	// 300ms would have been enough had the earlier Up counted. It must not.
	fc.Add(300 * time.Millisecond)
	require.Equal(t, fsm.StateOpenConfirm, session.State(),
		"the interval restarted at the second Up, so the first 300ms buys nothing")

	fc.Add(100 * time.Millisecond)
	require.Eventually(t, func() bool { return session.State() == fsm.StateEstablished },
		2*time.Second, 10*time.Millisecond)
}

// TestBFDHoldDownStarvationIsRefused is the Section 10 timing requirement the
// help text states, enforced.
//
// VALIDATES: A hold-down at or above the peer's configured hold time is
// refused at config parse, and one below it is accepted; a zero hold time
// bounds nothing and is accepted.
//
// PREVENTS: A configuration that starves its own peer. While the interval runs
// the session sends nothing, so an interval as long as the hold time guarantees
// the far end times out, and Section 10 names that outcome exactly.
func TestBFDHoldDownStarvationIsRefused(t *testing.T) {
	bfd := func(holdDown uint32) *BFDSettings {
		return &BFDSettings{Enabled: true, Strict: true, HoldDown: holdDown}
	}
	require.Error(t, validateBFDHoldDown("peer1", bfd(90000), 90*time.Second))
	require.Error(t, validateBFDHoldDown("peer1", bfd(120000), 90*time.Second))
	require.NoError(t, validateBFDHoldDown("peer1", bfd(300), 90*time.Second))
	require.NoError(t, validateBFDHoldDown("peer1", bfd(90000), 0),
		"a hold time of zero never expires, so it bounds nothing")
	require.NoError(t, validateBFDHoldDown("peer1", &BFDSettings{Enabled: true, HoldDown: 90000}, 90*time.Second),
		"a peer that is not strict runs no hold-down")
}
