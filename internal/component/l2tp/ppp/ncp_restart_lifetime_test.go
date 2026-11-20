// Design: docs/research/l2tpv2-ze-integration.md -- independent NCP Restart timers
// Related: ncp_restart.go -- the per-family Restart timer and counter these tests drive
// Related: ncp_termination_timer_test.go -- the dual-stack termination grace
//
// Goal: prove, through the session's real run loop, that an NCP owns the RFC
// 1661 Restart timer and counter: a Configure-Request the peer never answers
// is retransmitted each Restart time until Max-Configure is spent, then TO-
// takes the NCP to Stopped; a Terminate-Request the peer sends to an Opened
// NCP leaves it in Stopping for exactly one Restart time, then TO- takes it to
// Stopped. Both behaviors exist in the tree (ncp_restart.go); these tests are
// proofs of landed behavior, so each names the break that turns it red.
//
// Method: a net.Pipe stands for the channel, synctest makes the Restart
// timers exact, and the peer side plays LCP, authentication and the IPv4
// address answer before it stays silent or terminates IPCP. IPv6CP stays
// disabled so one family's timer is the only NCP timer that runs.

package ppp

import (
	"bytes"
	"errors"
	"net"
	"net/netip"
	"os"
	"testing"
	"testing/synctest"
	"time"
)

var (
	restartLifetimeLocal = netip.MustParseAddr("192.0.2.1")
	restartLifetimePeer  = netip.MustParseAddr("192.0.2.2")
)

// prepareIPCPLifecycle enables IPCP alone on session and gives it the
// authentication and address queues the peer side answers. It MUST be called
// before the session runs.
func prepareIPCPLifecycle(session *pppSession) (<-chan AuthEvent, <-chan IPEvent) {
	session.disableIPCP = false
	session.echoInterval = time.Hour
	authEvents := make(chan AuthEvent, 16)
	session.authEventsOut = authEvents
	ips := make(chan IPEvent, 2)
	session.ipEventsOut = ips
	return authEvents, ips
}

// negotiateIPCPLifecycle opens LCP, accepts authentication and answers the
// IPv4 address request, and returns Ze's first IPCP Configure-Request.
func negotiateIPCPLifecycle(t *testing.T, peer net.Conn, authEvents <-chan AuthEvent, ips <-chan IPEvent) LCPPacket {
	t.Helper()
	openLifecycleLCP(t, peer)
	answerAuthRequestForTest(t, awaitAuthRequest(t, authEvents), authResponseMsg{accept: true})
	select {
	case event := <-ips:
		request, ok := event.(EventIPRequest)
		if !ok {
			t.Fatalf("address event %T, want EventIPRequest", event)
		}
		if request.Family != AddressFamilyIPv4 {
			t.Fatalf("address request for %s, want IPv4", request.Family)
		}
		answerIPRequestForTest(t, request, ipResponseMsg{
			accept: true, family: AddressFamilyIPv4,
			local: restartLifetimeLocal, peer: restartLifetimePeer,
		})
	case <-time.After(5 * time.Second):
		t.Fatal("missing IPv4 address request")
	}
	return readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest)
}

// openLifecycleIPCP acknowledges Ze's IPCP Configure-Request, sends the peer's
// own for the assigned address, and waits for the session to come up.
func openLifecycleIPCP(t *testing.T, peer net.Conn, request LCPPacket, events <-chan Event) {
	t.Helper()
	writeLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureAck, request.Identifier, request.Data)
	peerAddress := restartLifetimePeer.As4()
	writeLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest, 0x51,
		append([]byte{IPCPOptIPAddress, 6}, peerAddress[:]...))
	readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureAck)
	awaitLifecycleUp(t, events)
}

// expectNoLifecycleFrame requires that Ze sends nothing on the channel for
// window. The read deadline runs on the synctest clock.
func expectNoLifecycleFrame(t *testing.T, peer net.Conn, window time.Duration) {
	t.Helper()
	if err := peer.SetReadDeadline(time.Now().Add(window)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, MaxFrameBufLen)
	n, err := peer.Read(buf)
	if err == nil {
		proto, payload, _, perr := ParseFrame(buf[:n])
		if perr != nil {
			t.Fatalf("Ze sent a malformed frame within %s: %v", window, perr)
		}
		t.Fatalf("Ze sent protocol %#04x (% x) within %s, want silence", proto, payload, window)
	}
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("peer read: %v", err)
	}
}

// TestNCPRestartTimerRetransmitsConfigureRequest proves AC-7: with a Restart
// time of 100 ms and a Max-Configure of 3 under an ipTimeout of 30 s, a peer
// that never answers Ze's IPCP Configure-Request receives it exactly three
// times, each exactly one Restart time after the one before, with the same
// options; the third timeout is TO- and IPCP reaches Stopped, long before
// ipTimeout, after which Ze sends no further Configure-Request.
//
// The Identifier is Ze's choice, not an RFC requirement: RFC 1661 Section 5.1
// lets a retransmission keep or change it ("For retransmissions, the
// Identifier MAY remain unchanged."), and nextNCPIdentifier gives every
// Configure-Request the next value of the family's counter, so the test pins
// that each retransmission carries the previous Identifier plus one.
//
// MUTATION: delete the armNCPRestartTimer call in spendNCPRestartCount
// (ncp_restart.go): no timer runs after the first transmission, so the second
// Configure-Request never arrives and the read fails.
// MUTATION: make handleNCPRestartTimeout (ncp_restart.go) raise TO+ whatever
// the counter holds: a fourth Configure-Request arrives and IPCP never reaches
// Stopped.
func TestNCPRestartTimerRetransmitsConfigureRequest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		session, _, _ := newRFC1661Session(LCPStateInitial)
		const maxConfigure = 3
		restart := 100 * time.Millisecond
		config, err := NewLCPConfig(restart, 2, maxConfigure, 5)
		if err != nil {
			t.Fatal(err)
		}
		session.lcpConfig = config
		const ipTimeout = 30 * time.Second
		session.ipTimeout = ipTimeout
		authEvents, ips := prepareIPCPLifecycle(session)
		peer := runLifecycleSession(t, session)

		first := negotiateIPCPLifecycle(t, peer, authEvents, ips)
		firstSentAt := time.Now()
		previous, sentAt := first, firstSentAt
		for transmission := 2; transmission <= maxConfigure; transmission++ {
			retry := readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest)
			if waited := time.Since(sentAt); waited != restart {
				t.Fatalf("transmission %d came %s after the one before, want the Restart time %s",
					transmission, waited, restart)
			}
			if retry.Identifier != previous.Identifier+1 {
				t.Fatalf("transmission %d Identifier = %d, want Ze's next value %d",
					transmission, retry.Identifier, previous.Identifier+1)
			}
			if !bytes.Equal(retry.Data, first.Data) {
				t.Fatalf("transmission %d options % x differ from the first request's % x",
					transmission, retry.Data, first.Data)
			}
			previous, sentAt = retry, time.Now()
		}

		// The counter is spent: the last Restart expiry is TO-, so nothing
		// more is sent.
		expectNoLifecycleFrame(t, peer, 10*restart)
		if elapsed := time.Since(firstSentAt); elapsed >= ipTimeout {
			t.Fatalf("the silence check ended %s after the first request, not before ipTimeout %s", elapsed, ipTimeout)
		}
		// Stopped is observed on the wire, never by reading session state
		// across goroutines: RFC 1661 Section 4.1 answers RCR+ in Stopped
		// with irc, scr and sca, so a peer Configure-Request draws a fresh
		// Configure-Request before the Configure-Ack. In Req-Sent the same
		// event runs sca alone, so the Configure-Request proves TO- reached
		// Stopped.
		expectStoppedNCPAnswersConfigureRequest(t, peer, previous, first)
	})
}

// expectStoppedNCPAnswersConfigureRequest sends IPCP a peer Configure-Request
// for the assigned address and requires the Stopped answer: Ze's own
// Configure-Request, with the next Identifier after last and the options of
// first, then the Configure-Ack of the peer's request.
func expectStoppedNCPAnswersConfigureRequest(t *testing.T, peer net.Conn, last, first LCPPacket) {
	t.Helper()
	peerAddress := restartLifetimePeer.As4()
	writeLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest, 0x5a,
		append([]byte{IPCPOptIPAddress, 6}, peerAddress[:]...))
	fresh := readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest)
	if fresh.Identifier != last.Identifier+1 {
		t.Fatalf("Configure-Request after Stopped has Identifier %d, want %d", fresh.Identifier, last.Identifier+1)
	}
	if !bytes.Equal(fresh.Data, first.Data) {
		t.Fatalf("Configure-Request after Stopped carries % x, want % x", fresh.Data, first.Data)
	}
	if ack := readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureAck); ack.Identifier != 0x5a {
		t.Fatalf("Configure-Ack Identifier = %#02x, want the request's 0x5a", ack.Identifier)
	}
}

// TestNCPRestartTimerCompletesStopping proves the AC-7 sibling for
// termination: after session-up the peer sends IPCP a Terminate-Request, Ze
// answers with a Terminate-Ack carrying its Identifier, and IPCP stays in
// Stopping, sending nothing, until exactly one Restart time has passed; then
// TO- takes it to Stopped, again with nothing sent. The RFC 1661 Section 4.1
// table takes Opened on RTR to Stopping with the actions tld, zrc and sta,
// and Section 4.4 says of zrc: "This action enables the FSA to pause before
// proceeding to the desired final state, allowing traffic to be processed by
// the peer." The zeroed counter makes the first expiry TO-. It overlaps TestNCPTerminationGracePreservesIndependentFamily,
// which proves the same grace beside an Opened IPv6CP; this one pins the
// timing of the single family.
//
// MUTATION: make the LCPActZRC case of performNCPAction (ncp.go) zero the
// counter without arming the family's Restart timer: IPCP stays in Stopping
// and the Stopped check fails.
// MUTATION: make applyNCPTransition (ncp_restart.go) stop the family's timer
// in every state: the same red.
func TestNCPRestartTimerCompletesStopping(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		session, _, events := newRFC1661Session(LCPStateInitial)
		session.ipTimeout = 30 * time.Second
		authEvents, ips := prepareIPCPLifecycle(session)
		peer := runLifecycleSession(t, session)
		request := negotiateIPCPLifecycle(t, peer, authEvents, ips)
		openLifecycleIPCP(t, peer, request, events)

		writeLifecyclePacket(t, peer, ProtoIPCP, LCPTerminateRequest, 0x71, nil)
		ack := readLifecyclePacket(t, peer, ProtoIPCP, LCPTerminateAck)
		if ack.Identifier != 0x71 {
			t.Fatalf("Terminate-Ack Identifier = %#02x, want the request's 0x71", ack.Identifier)
		}
		// The state is observed on the wire, never by reading session state
		// across goroutines. RFC 1661 Section 4.1 gives RCR in Stopping no
		// action, and RCR+ in Stopped irc, scr and sca: a peer
		// Configure-Request just before the Restart time draws nothing, and
		// one after it draws Ze's Configure-Request and Configure-Ack.
		restart := testLCPConfig().RestartTimer()
		expectNoLifecycleFrame(t, peer, restart-2*time.Millisecond)
		peerAddress := restartLifetimePeer.As4()
		writeLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureRequest, 0x59,
			append([]byte{IPCPOptIPAddress, 6}, peerAddress[:]...))
		expectNoLifecycleFrame(t, peer, time.Millisecond)
		time.Sleep(2 * time.Millisecond)
		expectStoppedNCPAnswersConfigureRequest(t, peer, request, request)
	})
}
