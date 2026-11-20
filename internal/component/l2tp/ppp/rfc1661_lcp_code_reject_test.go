// Design: docs/research/l2tpv2-implementation-guide.md -- 22.1 codeToEvent, RXJ+ and RXJ-
// Related: session_run.go -- lcpCodeRejectEvent, protocolRejectEvent, stopRejectedCode, the echo ticker arm of run
// Related: ncp.go -- ncpCodeToEvent, ncpCodeRejectEvent
// Related: lcp_lifecycle_test.go -- runLifecycleSession; ncp_restart_lifetime_test.go -- the IPCP lifecycle helpers
// Related: rfc/short/rfc1661.md -- RFC1661-4.3-3
//
// Goal: prove, through the session's real run loop, that a received
// Code-Reject or Protocol-Reject raises the RXJ+ or RXJ- event RFC 1661
// Section 4.3 names for what it rejects. RXJ+ of a Code Ze sends stops Ze
// sending that Code and changes nothing else; RXJ+ of a Code Ze never sends
// changes nothing; RXJ- ends the session (LCP) or stops the NCP that received
// it.
//
// Method: runLifecycleSession runs the session against a net.Pipe peer under
// synctest, so every timer is exact. The peer brings the session up with IPCP
// alone, then sends the reject. Everything is read where it crosses the
// session boundary: frames on the pipe and events on the transport channel.
// No session field is read from the test goroutine.

package ppp

import (
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// codeRejectEchoInterval is the echo interval of the echo tests: shorter than
// the five-second read bound of readLifecyclePacket, so one interval's
// Echo-Request is read inside it.
const codeRejectEchoInterval = 4 * time.Second

// codeRejectOf is the Data of a Code-Reject of pkt: the Rejected-Packet, a
// copy of pkt from its Code on (RFC 1661 Section 5.6).
func codeRejectOf(pkt LCPPacket) []byte {
	data := make([]byte, lcpHeaderLen, lcpHeaderLen+len(pkt.Data))
	data[0] = pkt.Code
	data[1] = pkt.Identifier
	binary.BigEndian.PutUint16(data[2:4], uint16(lcpHeaderLen+len(pkt.Data)))
	return append(data, pkt.Data...)
}

// runCodeRejectSession starts session with IPCP alone and the given echo
// interval, brings it up through LCP, authentication and IPCP, and returns
// the peer end of the channel.
func runCodeRejectSession(t *testing.T, echoInterval time.Duration) (net.Conn, <-chan Event) {
	t.Helper()
	session, _, events := newRFC1661Session(LCPStateInitial)
	authEvents, ips := prepareIPCPLifecycle(session)
	session.echoInterval = echoInterval
	peer := runLifecycleSession(t, session)
	request := negotiateIPCPLifecycle(t, peer, authEvents, ips)
	openLifecycleIPCP(t, peer, request, events)
	return peer, events
}

// expectSessionStaysUp requires that no EventSessionDown has reached the
// transport. Every other event is consumed and passed over.
func expectSessionStaysUp(t *testing.T, events <-chan Event) {
	t.Helper()
	synctest.Wait()
	for len(events) > 0 {
		if down, ok := (<-events).(EventSessionDown); ok {
			t.Fatalf("session ended: %+v", down)
		}
	}
}

// expectFatalRejectEnds reads the LCP Terminate-Request RXJ- in Opened sends
// (tld, irc, str), acknowledges it, and requires the session to end with NAS
// Error and a reason naming what was rejected.
func expectFatalRejectEnds(t *testing.T, peer net.Conn, events <-chan Event, rejected string) {
	t.Helper()
	terminate := readLifecyclePacket(t, peer, ProtoLCP, LCPTerminateRequest)
	writeLifecyclePacket(t, peer, ProtoLCP, LCPTerminateAck, terminate.Identifier, nil)
	down := awaitLifecycleDown(t, events)
	if down.Cause != l2tpevents.TerminateCauseNASError {
		t.Fatalf("session ended with cause %v (%q), want NAS Error", down.Cause, down.Reason)
	}
	if !strings.Contains(down.Reason, rejected) {
		t.Fatalf("session-down reason %q does not name %q", down.Reason, rejected)
	}
}

// TestCodeRejectedEchoRequestStopsEchoProbes proves that a peer's Code-Reject
// of Ze's Echo-Request stops the echo probes, and that the session outlives
// the replies that then never come.
//
// RFC requirement: RFC1661-4.3-3 positive -- after the peer Code-Rejects the
// first Echo-Request in LCP Opened, Ze sends no frame for six echo intervals
// (past the three-failure echo limit, which would otherwise end the session),
// still answers an Echo-Request with an Echo-Reply, and emits no
// EventSessionDown.
//
// MUTATION: make stopRejectedCode (session_run.go) leave echoRequestRejected
// false: the next Echo-Request arrives one interval later.
func TestCodeRejectedEchoRequestStopsEchoProbes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		peer, events := runCodeRejectSession(t, codeRejectEchoInterval)
		echo := readLifecyclePacket(t, peer, ProtoLCP, LCPEchoRequest)
		writeLifecyclePacket(t, peer, ProtoLCP, LCPCodeReject, 0x61, codeRejectOf(echo))

		expectNoLifecycleFrame(t, peer, 6*codeRejectEchoInterval)
		exchangeLifecycleEcho(t, peer)
		expectSessionStaysUp(t, events)
	})
}

// TestCodeRejectOfUnsentCodeKeepsEchoProbes proves that a Code-Reject of a
// Code Ze never sends is an RXJ+ that stops nothing: the echo probes go on.
//
// RFC requirement: RFC1661-4.3-3 negative -- after the peer answers the first
// Echo-Request and then Code-Rejects Code 0x99, which Ze never sends, the next
// Echo-Request still arrives one echo interval after the first, and the
// session does not end.
//
// MUTATION: make handleLCPPacket (session_run.go) set echoRequestRejected for
// every RXJ+ Code-Reject in place of calling stopRejectedCode: no second
// Echo-Request arrives.
func TestCodeRejectOfUnsentCodeKeepsEchoProbes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		peer, events := runCodeRejectSession(t, codeRejectEchoInterval)
		first := readLifecyclePacket(t, peer, ProtoLCP, LCPEchoRequest)
		sent := time.Now()
		// The peer negotiated no Magic-Number, so its Echo-Reply carries zero
		// (RFC 1661 Section 6.4).
		writeLifecyclePacket(t, peer, ProtoLCP, LCPEchoReply, first.Identifier, []byte{0, 0, 0, 0})
		writeLifecyclePacket(t, peer, ProtoLCP, LCPCodeReject, 0x62, []byte{0x99, 0x01, 0, 4})

		next := readLifecyclePacket(t, peer, ProtoLCP, LCPEchoRequest)
		if waited := time.Since(sent); waited != codeRejectEchoInterval {
			t.Fatalf("next Echo-Request came %s after the first, want the echo interval %s", waited, codeRejectEchoInterval)
		}
		if next.Identifier == first.Identifier {
			t.Fatal("next Echo-Request reused the first one's Identifier")
		}
		expectSessionStaysUp(t, events)
	})
}

// TestCodeRejectedEchoReplyLeavesEchoRequestUnanswered proves that a peer's
// Code-Reject of Ze's Echo-Reply stops Ze answering its Echo-Requests.
//
// RFC requirement: RFC1661-4.3-3 positive -- after the peer Code-Rejects the
// Echo-Reply Ze sent to its Echo-Request in LCP Opened, the peer's next
// Echo-Request draws no frame for two seconds, and the session does not end.
//
// MUTATION: make stopRejectedCode (session_run.go) leave echoReplyRejected
// false: the second Echo-Request draws an Echo-Reply.
func TestCodeRejectedEchoReplyLeavesEchoRequestUnanswered(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		peer, events := runCodeRejectSession(t, time.Hour)
		writeLifecyclePacket(t, peer, ProtoLCP, LCPEchoRequest, 0x24, []byte{0, 0, 0, 0})
		reply := readLifecyclePacket(t, peer, ProtoLCP, LCPEchoReply)
		writeLifecyclePacket(t, peer, ProtoLCP, LCPCodeReject, 0x63, codeRejectOf(reply))

		writeLifecyclePacket(t, peer, ProtoLCP, LCPEchoRequest, 0x25, []byte{0, 0, 0, 0})
		expectNoLifecycleFrame(t, peer, 2*time.Second)
		expectSessionStaysUp(t, events)
	})
}

// TestCodeRejectOfConfigureRequestEndsSession proves that a Code-Reject of
// Configure-Request is RXJ-: in LCP Opened Ze sends a Terminate-Request
// (tld, irc, str), and the Terminate-Ack ends the session with NAS Error.
// RFC 1661 Section 4.3: "The RXJ- event arises when the rejected value is
// catastrophic, such as a Code-Reject of Configure-Request, or a
// Protocol-Reject of LCP!" Red before lcpCodeRejectEvent: every Code-Reject
// was RXJ+, a no-op in Opened, so no Terminate-Request came.
//
// MUTATION: make lcpCodeRejectEvent (session_run.go) answer RXJ+ for a
// rejected Configure-Request: no Terminate-Request arrives.
func TestCodeRejectOfConfigureRequestEndsSession(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		peer, events := runCodeRejectSession(t, time.Hour)
		writeLifecyclePacket(t, peer, ProtoLCP, LCPCodeReject, 0x64,
			[]byte{LCPConfigureRequest, 0x01, 0, 4})
		expectFatalRejectEnds(t, peer, events, LCPCodeName(LCPConfigureRequest))
	})
}

// TestProtocolRejectOfLCPEndsSession proves that a Protocol-Reject naming LCP
// (0xc021) received in LCP Opened is RXJ-: Ze sends a Terminate-Request and
// the Terminate-Ack ends the session with NAS Error. Red before
// protocolRejectEvent: it was RXJ+, a no-op in Opened.
//
// MUTATION: make protocolRejectEvent (session_run.go) answer RXJ+ for
// ProtoLCP: no Terminate-Request arrives.
func TestProtocolRejectOfLCPEndsSession(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		peer, events := runCodeRejectSession(t, time.Hour)
		writeLifecyclePacket(t, peer, ProtoLCP, LCPProtocolReject, 0x65,
			[]byte{0xc0, 0x21, LCPConfigureRequest, 0x01, 0, 4})
		expectFatalRejectEnds(t, peer, events, "Protocol-Rejected LCP")
	})
}

// TestIPCPCodeRejectOfConfigureRequestStopsIPCP proves that a Code-Reject on
// IPCP of a Code IPCP uses is RXJ- for IPCP alone: IPCP in Opened sends an
// IPCP Terminate-Request (tld, irc, str), while LCP stays Opened and answers
// an Echo-Request, and the session does not end at once.
//
// MUTATION: make ncpCodeRejectEvent (ncp.go) answer RXJ+ for a rejected
// Configure-Request: IPCP stays Opened and sends nothing, so the read of the
// Terminate-Request reaches its deadline.
func TestIPCPCodeRejectOfConfigureRequestStopsIPCP(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		peer, events := runCodeRejectSession(t, time.Hour)
		writeLifecyclePacket(t, peer, ProtoIPCP, LCPCodeReject, 0x66,
			[]byte{LCPConfigureRequest, 0x01, 0, 4})
		readLifecyclePacket(t, peer, ProtoIPCP, LCPTerminateRequest)
		exchangeLifecycleEcho(t, peer)
		expectSessionStaysUp(t, events)
	})
}

// TestIPCPCodeRejectOfUnusedCodeKeepsIPCP proves that a Code-Reject on IPCP of
// a Code IPCP never sends is RXJ+, a no-op in Opened: no frame follows, and
// LCP still answers an Echo-Request.
//
// MUTATION: make ncpCodeRejectEvent (ncp.go) answer RXJ- for every rejected
// Code: IPCP sends a Terminate-Request where the Echo-Reply is read.
func TestIPCPCodeRejectOfUnusedCodeKeepsIPCP(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		peer, events := runCodeRejectSession(t, time.Hour)
		writeLifecyclePacket(t, peer, ProtoIPCP, LCPCodeReject, 0x67, []byte{12, 0x01, 0, 4})
		expectNoLifecycleFrame(t, peer, 2*time.Second)
		exchangeLifecycleEcho(t, peer)
		expectSessionStaysUp(t, events)
	})
}
