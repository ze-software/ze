// Design: docs/research/l2tpv2-ze-integration.md -- the no-NCP bound
// Related: ncp_bound.go -- the bound these tests drive
// Related: ncp.go -- runNCPPhase, the session-up rule
// Related: ncp_restart_lifetime_test.go -- the IPv4 lifecycle helpers reused here
//
// Goal: prove, through the session's real run loop, the no-NCP bound and the
// session-up rule (spec-ppp-ncp-independent-lifetime, AC-1, AC-6, AC-12). One
// NCP closing never ends the link (RFC 1661 Section 3.7 Implementation Note:
// "Conversely, the fact that one NCP has Closed is not sufficient reason to
// cause the termination of the PPP link, even if that NCP was the only NCP
// currently in the Opened state."). A link with no enabled NCP Opened is held
// for one bound, armed at the start of the NCP phase or when the last Opened
// NCP leaves Opened and never reset while none is Opened; its expiry is the
// idle-period timer the same section names ("the expiration of an
// idle-period timer"). In the NCP phase the expiry fails the session with NAS
// Error and no LCP Terminate-Request; after session-up it closes LCP with a
// Terminate-Request and the session ends with Idle Timeout. EventSessionUp is
// emitted only when an enabled NCP is Opened as the NCP phase settles.
//
// Method: a net.Pipe stands for the channel and synctest makes every timer
// exact, so each bound is asserted to the instant it was armed from. The
// tests observe only frames and events, never session state, because the
// session goroutine owns that state.

package ppp

import (
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"testing/synctest"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// idleBoundConfig is the given Restart time and a Max-Configure of three, so an
// unanswered NCP retransmits after one and two Restart times and reaches
// Stopped by TO- after three, within a bound of a few seconds.
func idleBoundConfig(t *testing.T, restart time.Duration) LCPConfig {
	t.Helper()
	config, err := NewLCPConfig(restart, 2, 3, 5)
	if err != nil {
		t.Fatal(err)
	}
	return config
}

// prepareDualStackLifecycle enables both NCPs on session and gives it the
// authentication and address queues the peer side answers. It MUST be called
// before the session runs.
func prepareDualStackLifecycle(session *pppSession) (<-chan AuthEvent, <-chan IPEvent) {
	authEvents, ips := prepareIPCPLifecycle(session)
	session.disableIPv6CP = false
	return authEvents, ips
}

// negotiateDualStackLifecycle opens LCP, accepts authentication, answers both
// address requests, and returns Ze's first IPCP and IPv6CP Configure-Requests
// and the options of a peer IPv6CP Configure-Request Ze acknowledges.
func negotiateDualStackLifecycle(t *testing.T, peer net.Conn, authEvents <-chan AuthEvent, ips <-chan IPEvent) (
	request4, request6 LCPPacket, peerOptions6 []byte,
) {
	t.Helper()
	request4 = negotiateIPCPLifecycle(t, peer, authEvents, ips)
	select {
	case event := <-ips:
		request, ok := event.(EventIPRequest)
		if !ok || request.Family != AddressFamilyIPv6 {
			t.Fatalf("second address request = %+v, want IPv6", event)
		}
		answerIPRequestForTest(t, request, ipResponseMsg{accept: true, family: AddressFamilyIPv6})
	case <-time.After(5 * time.Second):
		t.Fatal("missing IPv6 address request")
	}
	request6 = readLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureRequest)
	options, err := parseIPv6CPOptions(request6.Data)
	if err != nil {
		t.Fatal(err)
	}
	peerID := options.InterfaceID
	peerID[0] = 2
	peerID[7] ^= 0xff
	return request4, request6, append([]byte{IPv6CPOptInterfaceID, 10}, peerID[:]...)
}

// openLifecycleIPv6CP acknowledges a peer IPv6CP Configure-Request, then Ze's
// own, which takes IPv6CP to Opened.
func openLifecycleIPv6CP(t *testing.T, peer net.Conn, request6 LCPPacket, peerOptions6 []byte) {
	t.Helper()
	writeLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureRequest, 0x52, peerOptions6)
	readLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureAck)
	writeLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureAck, request6.Identifier, request6.Data)
}

// terminateLifecycleNCP sends a peer Terminate-Request to the NCP of proto and
// requires Ze's Terminate-Ack with its Identifier.
func terminateLifecycleNCP(t *testing.T, peer net.Conn, proto uint16) {
	t.Helper()
	writeLifecyclePacket(t, peer, proto, LCPTerminateRequest, 0x71, nil)
	if ack := readLifecyclePacket(t, peer, proto, LCPTerminateAck); ack.Identifier != 0x71 {
		t.Fatalf("Terminate-Ack Identifier = %#02x, want the request's 0x71", ack.Identifier)
	}
}

// reopenStoppedIPCP sends a peer IPCP Configure-Request to a Stopped IPCP,
// reads Ze's Configure-Request and Configure-Ack (RFC 1661 Section 4.1, RCR+
// in Stopped: irc, scr, sca), and acknowledges Ze's request, which takes IPCP
// to Opened.
func reopenStoppedIPCP(t *testing.T, peer net.Conn) {
	t.Helper()
	peerAddress := restartLifetimePeer.As4()
	writeLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest, 0x5a,
		append([]byte{IPCPOptIPAddress, 6}, peerAddress[:]...))
	fresh := readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest)
	if ack := readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureAck); ack.Identifier != 0x5a {
		t.Fatalf("Configure-Ack Identifier = %#02x, want the request's 0x5a", ack.Identifier)
	}
	writeLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureAck, fresh.Identifier, fresh.Data)
}

// exchangeLifecycleEcho proves LCP is Opened: an Echo-Request carrying the
// peer's Magic-Number (zero, since the peer negotiated none) draws an
// Echo-Reply with its Identifier.
func exchangeLifecycleEcho(t *testing.T, peer net.Conn) {
	t.Helper()
	writeLifecyclePacket(t, peer, ProtoLCP, LCPEchoRequest, 0x23, []byte{0, 0, 0, 0})
	if reply := readLifecyclePacket(t, peer, ProtoLCP, LCPEchoReply); reply.Identifier != 0x23 {
		t.Fatalf("Echo-Reply Identifier = %#02x, want the request's 0x23", reply.Identifier)
	}
}

// expectNoSessionEvent requires that no EventSessionUp and no EventSessionDown
// has reached the transport. Every other event is consumed and passed over.
func expectNoSessionEvent(t *testing.T, events <-chan Event) {
	t.Helper()
	synctest.Wait()
	for len(events) > 0 {
		switch event := (<-events).(type) {
		case EventSessionUp:
			t.Fatalf("EventSessionUp with no NCP Opened: %+v", event)
		case EventSessionDown:
			t.Fatalf("session ended inside the no-NCP bound: %+v", event)
		}
	}
}

// expectChannelEndsSilently requires that Ze sends no frame once the session
// has ended: the channel closes (EOF) or stays silent, and an LCP
// Terminate-Request would arrive as a frame.
func expectChannelEndsSilently(t *testing.T, peer net.Conn) {
	t.Helper()
	if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		// net.Pipe answers io.ErrClosedPipe once teardown closed the pipe:
		// the channel ended with no frame.
		if errors.Is(err, io.ErrClosedPipe) {
			return
		}
		t.Fatal(err)
	}
	buf := make([]byte, MaxFrameBufLen)
	n, err := peer.Read(buf)
	if err == nil {
		proto, payload, _, perr := ParseFrame(buf[:n])
		if perr != nil {
			t.Fatalf("Ze sent a malformed frame at session end: %v", perr)
		}
		t.Fatalf("Ze sent protocol %#04x (% x) at session end, want no LCP Terminate-Request", proto, payload)
	}
	// Session teardown may already have closed the peer's own end of the
	// fixture pipe (io.ErrClosedPipe, as the first package run showed): that
	// too means no frame was sent.
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, os.ErrDeadlineExceeded) {
		return
	}
	t.Fatalf("peer read: %v", err)
}

// awaitNetworkEventsUntilUp returns the assignment and release events that
// precede EventSessionUp, failing on a session-down and on a deadline.
func awaitNetworkEventsUntilUp(t *testing.T, events <-chan Event) []Event {
	t.Helper()
	var network []Event
	deadline := time.After(10 * time.Second)
	for {
		select {
		case event := <-events:
			switch event.(type) {
			case EventSessionIPAssigned, EventSessionIPReleased:
				network = append(network, event)
			case EventSessionUp:
				return network
			case EventSessionDown:
				t.Fatalf("session went down instead of up: %+v", event)
			}
		case <-deadline:
			t.Fatal("session never came up")
		}
	}
}

// TestNCPClosedDoesNotEndSessionWhileOtherOpen proves AC-1: in the NCP phase
// IPv6CP reaches Opened before IPCP and the peer terminates it; IPCP then
// reaches Opened. Ze answers Terminate-Ack, IPv6CP reaches Stopped one Restart
// time after the request, and only then does the phase settle, with IPv4
// alone Opened: the network events are the IPv6 assignment, its release and
// the IPv4 assignment, then EventSessionUp. IPv6CP leaving Opened armed the
// no-NCP bound (one second) and IPCP reaching Opened canceled it, so nothing
// ends the session or sends LCP anything for twice the bound after that.
// Green today: the family-level independence landed
// (TestNCPClosingOneFamilyPreservesOtherFamily).
//
// MUTATION: make trackNoNCPBound (ncp_bound.go) return before
// cancelNoNCPBound when an NCP reaches Opened: the bound armed by the IPv6CP
// Terminate-Request expires in the NCP phase and the session fails before it
// comes up.
func TestNCPClosedDoesNotEndSessionWhileOtherOpen(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		session, _, events := newRFC1661Session(LCPStateInitial)
		const bound = time.Second
		session.ipTimeout = bound
		authEvents, ips := prepareDualStackLifecycle(session)
		peer := runLifecycleSession(t, session)
		request4, request6, peerOptions6 := negotiateDualStackLifecycle(t, peer, authEvents, ips)

		openLifecycleIPv6CP(t, peer, request6, peerOptions6)
		terminateLifecycleNCP(t, peer, ProtoIPv6CP)
		terminatedAt := time.Now()
		writeLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureAck, request4.Identifier, request4.Data)
		peerAddress := restartLifetimePeer.As4()
		writeLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest, 0x51,
			append([]byte{IPCPOptIPAddress, 6}, peerAddress[:]...))
		readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureAck)

		network := awaitNetworkEventsUntilUp(t, events)
		if waited, restart := time.Since(terminatedAt), testLCPConfig().RestartTimer(); waited != restart {
			t.Fatalf("session came up %s after the IPv6CP Terminate-Request, want one Restart time %s", waited, restart)
		}
		// At session-up IPv4 alone is Opened: the IPv4 assignment was
		// published, and the last IPv6 event, if any, is its release.
		ipv4Assigned := false
		var lastIPv6 Event
		for _, event := range network {
			switch event := event.(type) {
			case EventSessionIPAssigned:
				if event.Family == AddressFamilyIPv6 {
					lastIPv6 = event
					continue
				}
				if event.Peer != restartLifetimePeer {
					t.Fatalf("IPv4 assignment of %s, want %s", event.Peer, restartLifetimePeer)
				}
				ipv4Assigned = true
			case EventSessionIPReleased:
				if event.Family == AddressFamilyIPv6 {
					lastIPv6 = event
				}
			}
		}
		if !ipv4Assigned {
			t.Fatalf("network events before EventSessionUp = %+v, want the IPv4 assignment", network)
		}
		if _, stillAssigned := lastIPv6.(EventSessionIPAssigned); stillAssigned {
			t.Fatalf("IPv6 still assigned at EventSessionUp after its Terminate-Request: %+v", network)
		}

		expectNoLifecycleFrame(t, peer, 2*bound)
		expectNoSessionEvent(t, events)
	})
}

// TestLastNCPFinishedIdlesThenClosesLCP proves AC-6 (a) after session-up:
// IPv4 is the only NCP; the peer terminates IPCP, which reaches Stopped one
// Restart time later. LCP stays Opened (Echo is answered) and no
// EventSessionDown is sent until the no-NCP bound, measured from the
// Terminate-Request, expires; then Ze sends an LCP Terminate-Request, and the
// peer's Terminate-Ack ends the session with Idle Timeout (RFC 2866 Section
// 5.10 value 4, "Idle timer expired."). Red today: nothing bounds a session
// with no NCP after session-up, so no Terminate-Request is ever sent.
//
// MUTATION: delete the noNCPBoundChannel case from the run loop
// (session_run.go): no LCP Terminate-Request arrives.
// MUTATION: delete the noNCPBoundExpired branch of applyTransition
// (session_run.go): the session ends with User Request.
func TestLastNCPFinishedIdlesThenClosesLCP(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		session, _, events := newRFC1661Session(LCPStateInitial)
		const bound = 10 * time.Second
		session.ipTimeout = bound
		authEvents, ips := prepareIPCPLifecycle(session)
		peer := runLifecycleSession(t, session)
		request := negotiateIPCPLifecycle(t, peer, authEvents, ips)
		openLifecycleIPCP(t, peer, request, events)

		terminateLifecycleNCP(t, peer, ProtoIPCP)
		leftOpened := time.Now()
		// IPCP is Stopped after one Restart time (3 s); LCP stays Opened.
		time.Sleep(bound - time.Second)
		exchangeLifecycleEcho(t, peer)
		expectNoSessionEvent(t, events)

		terminate := readLifecyclePacket(t, peer, ProtoLCP, LCPTerminateRequest)
		if waited := time.Since(leftOpened); waited != bound {
			t.Fatalf("LCP Terminate-Request came %s after IPCP left Opened, want the no-NCP bound %s", waited, bound)
		}
		writeLifecyclePacket(t, peer, ProtoLCP, LCPTerminateAck, terminate.Identifier, nil)
		down := awaitLifecycleDown(t, events)
		if down.Cause != l2tpevents.TerminateCauseIdleTimeout {
			t.Fatalf("session ended with cause %v (%q), want Idle Timeout", down.Cause, down.Reason)
		}
	})
}

// TestNoNCPBoundEndsPostUpNakLoop proves AC-6 (a) for the post-up Nak loop:
// after session-up the peer's IPCP Configure-Request takes IPCP out of Opened
// (RCR+ in Opened: tld, scr, sca), and the peer answers every Configure-Request
// Ze sends with a valid Configure-Nak, so irc keeps IPCP negotiating. The Naks
// do not reset the bound: at exactly one bound after the Configure-Request Ze
// sends an LCP Terminate-Request, and the session ends with Idle Timeout. Red
// today: nothing bounds a post-up renegotiation, so IPCP's Restart timer
// retransmits a Configure-Request where the Terminate-Request is expected.
//
// MUTATION: make trackNoNCPBound (ncp_bound.go) arm the bound on every edge
// that leaves no NCP Opened, not only on leaving Opened: each Nak's edge
// re-arms it and the Terminate-Request comes late.
func TestNoNCPBoundEndsPostUpNakLoop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		session, _, events := newRFC1661Session(LCPStateInitial)
		const bound = time.Second
		session.ipTimeout = bound
		authEvents, ips := prepareIPCPLifecycle(session)
		peer := runLifecycleSession(t, session)
		first := negotiateIPCPLifecycle(t, peer, authEvents, ips)
		openLifecycleIPCP(t, peer, first, events)

		peerAddress := restartLifetimePeer.As4()
		writeLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest, 0x5b,
			append([]byte{IPCPOptIPAddress, 6}, peerAddress[:]...))
		leftOpened := time.Now()
		request := readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest)
		readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureAck)
		for range 9 {
			time.Sleep(100 * time.Millisecond)
			writeLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureNak, request.Identifier, request.Data)
			request = readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest)
		}

		terminate := readLifecyclePacket(t, peer, ProtoLCP, LCPTerminateRequest)
		if waited := time.Since(leftOpened); waited != bound {
			t.Fatalf("LCP Terminate-Request came %s after IPCP left Opened, want the no-NCP bound %s", waited, bound)
		}
		writeLifecyclePacket(t, peer, ProtoLCP, LCPTerminateAck, terminate.Identifier, nil)
		down := awaitLifecycleDown(t, events)
		if down.Cause != l2tpevents.TerminateCauseIdleTimeout {
			t.Fatalf("session ended with cause %v (%q), want Idle Timeout", down.Cause, down.Reason)
		}
	})
}

// TestLastNCPIdleCancelledByReopen proves AC-6 (b): after session-up the peer
// terminates the only NCP, IPCP, which reaches Stopped; within the no-NCP
// bound the peer's Configure-Request reopens it to Opened, Ze reinstalls the
// IPv4 family (EventSessionIPAssigned for the held peer address), and for
// twice the bound after that Ze sends nothing and the session stays up. Green
// today, since after session-up no bound exists to fire.
//
// MUTATION: make trackNoNCPBound (ncp_bound.go) return before
// cancelNoNCPBound when an NCP reaches Opened: the bound armed by the
// Terminate-Request expires after the reopen and Ze sends an LCP
// Terminate-Request inside the silence window.
func TestLastNCPIdleCancelledByReopen(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		session, _, events := newRFC1661Session(LCPStateInitial)
		const bound = 5 * time.Second
		session.ipTimeout = bound
		authEvents, ips := prepareIPCPLifecycle(session)
		peer := runLifecycleSession(t, session)
		request := negotiateIPCPLifecycle(t, peer, authEvents, ips)
		openLifecycleIPCP(t, peer, request, events)

		terminateLifecycleNCP(t, peer, ProtoIPCP)
		time.Sleep(testLCPConfig().RestartTimer() + time.Millisecond)
		reopenStoppedIPCP(t, peer)
		// The Terminate-Request's release, then the reopen's reassignment.
		if released, ok := awaitNetworkEvent(t, events).(EventSessionIPReleased); !ok || released.Family != AddressFamilyIPv4 {
			t.Fatalf("first network event = %+v, want the IPv4 release", released)
		}
		assigned, ok := awaitNetworkEvent(t, events).(EventSessionIPAssigned)
		if !ok || assigned.Family != AddressFamilyIPv4 || assigned.Peer != restartLifetimePeer {
			t.Fatalf("second network event = %+v, want the IPv4 reassignment of %s", assigned, restartLifetimePeer)
		}

		expectNoLifecycleFrame(t, peer, 2*bound)
		expectNoSessionEvent(t, events)
	})
}

// TestAllNCPsFinishedBeforeUpEndsWithNASError proves AC-12 for an NCP phase
// whose only enabled NCP finishes without opening: IPCP's Configure-Request
// goes unanswered until Max-Configure is spent and TO- takes it to Stopped.
// No EventSessionUp follows, LCP answers Echo, and at exactly one no-NCP
// bound from the start of the NCP phase (IPCP finishing does not stop or
// reset it) the session ends with NAS Error and sends no LCP
// Terminate-Request. Red today: the settled phase returns and afterLCPOpen
// emits EventSessionUp with no NCP Opened.
//
// MUTATION: make runNCPPhase (ncp.go) return true on ncpsComplete alone: the
// session comes up when IPCP reaches Stopped.
func TestAllNCPsFinishedBeforeUpEndsWithNASError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		session, _, events := newRFC1661Session(LCPStateInitial)
		session.lcpConfig = idleBoundConfig(t, 100*time.Millisecond)
		const bound = 2 * time.Second
		session.ipTimeout = bound
		authEvents, ips := prepareIPCPLifecycle(session)
		peer := runLifecycleSession(t, session)
		negotiateIPCPLifecycle(t, peer, authEvents, ips)
		phaseStart := time.Now()
		readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest)
		readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest)

		time.Sleep(time.Second)
		exchangeLifecycleEcho(t, peer)
		expectNoSessionEvent(t, events)

		down := awaitLifecycleDown(t, events)
		if waited := time.Since(phaseStart); waited != bound {
			t.Fatalf("session ended %s after the NCP phase started, want the no-NCP bound %s", waited, bound)
		}
		if down.Cause != l2tpevents.TerminateCauseNASError {
			t.Fatalf("session ended with cause %v (%q), want NAS Error", down.Cause, down.Reason)
		}
		expectChannelEndsSilently(t, peer)
	})
}

// TestNCPOpenedThenStoppedBeforeUpEmitsNoSessionUp proves AC-12 for an NCP
// that opened and finished before the phase settled: IPv6CP reaches Opened
// and the peer terminates it half a second later; IPCP, never answered,
// reaches Stopped by Max-Configure at three seconds. No EventSessionUp is
// emitted, LCP answers Echo, and the session ends with NAS Error at exactly
// one no-NCP bound from IPv6CP leaving Opened, not from the phase start, with
// no LCP Terminate-Request. Red today: the settled phase returns and
// afterLCPOpen emits EventSessionUp.
//
// MUTATION: make runNCPPhase (ncp.go) return true on ncpsComplete alone: the
// session comes up at three seconds.
// MUTATION: delete the armNoNCPBound call from trackNoNCPBound
// (ncp_bound.go): the bound armed at the phase start was canceled by
// IPv6CP's Opened and nothing ends the session.
func TestNCPOpenedThenStoppedBeforeUpEmitsNoSessionUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		session, _, events := newRFC1661Session(LCPStateInitial)
		session.lcpConfig = idleBoundConfig(t, time.Second)
		const bound = 5 * time.Second
		session.ipTimeout = bound
		authEvents, ips := prepareDualStackLifecycle(session)
		peer := runLifecycleSession(t, session)
		_, request6, peerOptions6 := negotiateDualStackLifecycle(t, peer, authEvents, ips)

		openLifecycleIPv6CP(t, peer, request6, peerOptions6)
		time.Sleep(500 * time.Millisecond)
		terminateLifecycleNCP(t, peer, ProtoIPv6CP)
		leftOpened := time.Now()
		// IPCP retransmits at one and two seconds, then TO- at three.
		readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest)
		readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest)
		time.Sleep(2 * time.Second)
		exchangeLifecycleEcho(t, peer)
		expectNoSessionEvent(t, events)

		down := awaitLifecycleDown(t, events)
		if waited := time.Since(leftOpened); waited != bound {
			t.Fatalf("session ended %s after IPv6CP left Opened, want the no-NCP bound %s", waited, bound)
		}
		if down.Cause != l2tpevents.TerminateCauseNASError {
			t.Fatalf("session ended with cause %v (%q), want NAS Error", down.Cause, down.Reason)
		}
		expectChannelEndsSilently(t, peer)
	})
}

// TestNCPStoppingWhileOtherStoppedBeforeUpEndsAtNoNCPBound proves AC-12 for
// the ordering that hangs today: IPv6CP reaches Opened, IPCP retransmits
// unanswered, and at 2.5 s the peer terminates IPv6CP (Stopping), which arms
// the no-NCP bound; IPCP reaches Stopped by Max-Configure at 3 s while IPv6CP
// is still Stopping, and IPv6CP reaches Stopped at 3.5 s. No EventSessionUp
// follows, and the session ends with NAS Error at exactly one bound from the
// Terminate-Request, with no LCP Terminate-Request. Red today on the
// no-EventSessionUp check: the IPv6CP Restart timer's TO- settles the phase
// and afterLCPOpen emits EventSessionUp with no NCP Opened.
//
// MUTATION: make runNCPPhase (ncp.go) return true on ncpsComplete alone: the
// session comes up at 3.5 s.
func TestNCPStoppingWhileOtherStoppedBeforeUpEndsAtNoNCPBound(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		session, _, events := newRFC1661Session(LCPStateInitial)
		session.lcpConfig = idleBoundConfig(t, time.Second)
		const bound = 5 * time.Second
		session.ipTimeout = bound
		authEvents, ips := prepareDualStackLifecycle(session)
		peer := runLifecycleSession(t, session)
		_, request6, peerOptions6 := negotiateDualStackLifecycle(t, peer, authEvents, ips)

		openLifecycleIPv6CP(t, peer, request6, peerOptions6)
		readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest)
		readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest)
		time.Sleep(500 * time.Millisecond)
		terminateLifecycleNCP(t, peer, ProtoIPv6CP)
		leftOpened := time.Now()
		time.Sleep(1500 * time.Millisecond)
		exchangeLifecycleEcho(t, peer)
		expectNoSessionEvent(t, events)

		down := awaitLifecycleDown(t, events)
		if waited := time.Since(leftOpened); waited != bound {
			t.Fatalf("session ended %s after IPv6CP left Opened, want the no-NCP bound %s", waited, bound)
		}
		if down.Cause != l2tpevents.TerminateCauseNASError {
			t.Fatalf("session ended with cause %v (%q), want NAS Error", down.Cause, down.Reason)
		}
		expectChannelEndsSilently(t, peer)
	})
}

// TestNoNCPBoundArmsWhenLastOpenedRenegotiates holds the ordering of
// TestNCPConvergenceResumesDuringTerminationGrace to exact timing: in the NCP
// phase IPCP is Stopping and IPv6CP Opened, and a wait longer than the bound
// passes with no EventSessionUp and no EventSessionDown, since the bound is
// not armed while IPv6CP is Opened. The peer's IPv6CP Configure-Request then
// takes IPv6CP out of Opened and the peer Naks every Configure-Request; the
// session ends with NAS Error at exactly one bound from that Configure-Request,
// so the bound armed fresh there and the Naks did not reset it. A guard, green
// today: runNCPPhase Resets its convergence timer for the Configure lifetime
// the request resumes, which ends the session at the same instant. It claims
// no red.
func TestNoNCPBoundArmsWhenLastOpenedRenegotiates(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		session, _, events := newRFC1661Session(LCPStateInitial)
		const bound = time.Second
		session.ipTimeout = bound
		authEvents, ips := prepareDualStackLifecycle(session)
		peer := runLifecycleSession(t, session)
		request4, request6, peerOptions6 := negotiateDualStackLifecycle(t, peer, authEvents, ips)

		peerAddress := restartLifetimePeer.As4()
		writeLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest, 0x51,
			append([]byte{IPCPOptIPAddress, 6}, peerAddress[:]...))
		readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureAck)
		writeLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureAck, request4.Identifier, request4.Data)
		terminateLifecycleNCP(t, peer, ProtoIPCP)
		openLifecycleIPv6CP(t, peer, request6, peerOptions6)
		time.Sleep(bound + 100*time.Millisecond)
		expectNoSessionEvent(t, events)

		writeLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureRequest, 0x72, peerOptions6)
		renegotiatedAt := time.Now()
		request := readLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureRequest)
		readLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureAck)
		for attempt := byte(1); attempt <= 9; attempt++ {
			time.Sleep(100 * time.Millisecond)
			suggested := [8]byte{2, 0, 0, 0, 0, 0, 0, 100 + attempt}
			writeLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureNak, request.Identifier,
				append([]byte{IPv6CPOptInterfaceID, 10}, suggested[:]...))
			request = readLifecyclePacket(t, peer, ProtoIPv6CP, LCPConfigureRequest)
		}

		down := awaitLifecycleDown(t, events)
		if waited := time.Since(renegotiatedAt); waited != bound {
			t.Fatalf("session ended %s after IPv6CP left Opened, want the no-NCP bound %s", waited, bound)
		}
		if down.Cause != l2tpevents.TerminateCauseNASError {
			t.Fatalf("session ended with cause %v (%q), want NAS Error", down.Cause, down.Reason)
		}
	})
}

// TestAllNCPsFinishedBeforeUpReopenBringsSessionUp proves the reopen half of
// AC-12: IPCP, the only enabled NCP, reaches Stopped by Max-Configure in the
// NCP phase and no EventSessionUp follows; within the no-NCP bound the peer's
// Configure-Request reopens it to Opened, and EventSessionUp follows before
// the bound. Red today: the session comes up when IPCP reaches Stopped, before
// any reopen.
//
// MUTATION: make runNCPPhase (ncp.go) return true on ncpsComplete alone: the
// EventSessionUp check before the reopen fails.
func TestAllNCPsFinishedBeforeUpReopenBringsSessionUp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		session, _, events := newRFC1661Session(LCPStateInitial)
		session.lcpConfig = idleBoundConfig(t, 100*time.Millisecond)
		const bound = 5 * time.Second
		session.ipTimeout = bound
		authEvents, ips := prepareIPCPLifecycle(session)
		peer := runLifecycleSession(t, session)
		negotiateIPCPLifecycle(t, peer, authEvents, ips)
		phaseStart := time.Now()
		readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest)
		readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest)
		time.Sleep(time.Second)
		expectNoSessionEvent(t, events)

		reopenStoppedIPCP(t, peer)
		awaitLifecycleUp(t, events)
		if waited := time.Since(phaseStart); waited >= bound {
			t.Fatalf("session came up %s after the NCP phase started, want within the bound %s", waited, bound)
		}
	})
}
