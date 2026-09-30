//go:build linux

// Design: docs/architecture/ike/ipsec-7-ikev2-engine.md -- liveness and path probes
// Related: probe.go -- handleSizeRefusal, the one ICMP error the engine acts on
// Related: ../transport/udp.go -- IP_RECVERR, the error-queue drain, deliverQueuedError
//
// VALIDATES: the routing-information clause of RFC 7296 Section 2.4, "an endpoint MUST
// NOT conclude that the other endpoint has failed based on any routing information
// (e.g., ICMP messages)". The IKE socket sets IP_RECVERR, so the kernel hands Ze every
// ICMP error for a datagram it sent. Real ICMP errors are produced on loopback (a
// datagram to a closed UDP port draws a Port Unreachable, no privilege needed) and the
// SA is checked afterwards: still Established, its path probe still outstanding, and
// still exchanging authenticated messages with the peer.
package engine

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/core/ikeprobe"
	"github.com/ze-software/ze/internal/core/probe"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// icmpClosedPort returns a loopback UDP address nothing listens on. A datagram sent
// there draws an ICMP Port Unreachable onto the sender's error queue.
func icmpClosedPort(t *testing.T) *net.UDPAddr {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("LocalAddr is not *net.UDPAddr")
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return addr
}

// icmpQueue sends one datagram to target and waits for the kernel's ICMP answer to
// reach the error queue of the SA's socket.
func icmpQueue(t *testing.T, tr *transport.UDPTransport, target *net.UDPAddr) {
	t.Helper()
	if err := tr.Send(make([]byte, 28), target); err != nil {
		t.Fatalf("send to %v: %v", target, err)
	}
	time.Sleep(50 * time.Millisecond) // sleep(kernel): loopback ICMP delivery runs in softirq, and reading the socket to wait for it would consume the error under test
}

// icmpNoRefusal fails the test when an ICMP error other than a size refusal reached the
// engine's refusal channel.
func icmpNoRefusal(t *testing.T, tr *transport.UDPTransport, what string) {
	t.Helper()
	select {
	case got := <-tr.Refusals():
		t.Fatalf("%s: an ICMP unreachable reached the engine as a size refusal: %+v", what, got)
	default:
	}
}

// RFC requirement: RFC7296-2.4-3 positive -- a real ICMP Port Unreachable queued on the
// SA's IKE socket (IP_RECVERR) leaves the SA Established: the SA's next request still
// reaches the peer, the ICMP error never reaches the engine, and the peer's
// authenticated answer is accepted (path probe settled as fits).
func TestRFC7296ICMPUnreachableDoesNotFailTheSA(t *testing.T) {
	ini, peer, ps, peerTr, myTr := dpdProbeLink(t)

	icmpQueue(t, myTr, icmpClosedPort(t))

	request := probeAsk(ps, ini, myTr, probeGridSize(t, ini, false))
	sent := rtxRecv(t, peerTr)
	if sent == nil {
		t.Fatal("after an ICMP Port Unreachable the SA's next request never reached the peer")
	}
	icmpNoRefusal(t, myTr, "after a Port Unreachable")
	if ini.State != StateEstablished {
		t.Fatalf("SA state %v after an ICMP Port Unreachable, want Established", ini.State)
	}

	probeDeliver(ps, ini, myTr, winInformationalAnswer(t, peer, parseMsg(t, sent).Header.MessageID))
	if result := probeReply(t, request, "after the peer answered"); result.Outcome != ikeprobe.OutcomeFits {
		t.Fatalf("probe outcome %v after the peer answered, want fits", result.Outcome)
	}
	if ini.State != StateEstablished {
		t.Fatalf("SA state %v after the peer answered, want Established", ini.State)
	}
}

// RFC requirement: RFC7296-2.4-3 negative -- routing information that says the peer is
// unreachable, arriving while Ze waits for that peer (the moment a failure verdict
// would be drawn), is refused as evidence: a real ICMP Port Unreachable from the peer's
// own address and port, drained by the next send, and a Fragmentation Needed for the
// peer routed to the engine leave the SA Established and the probe outstanding with no
// failure answer; the peer's later authenticated answer still settles it.
func TestRFC7296ICMPFromThePeerWhileWaitingFailsNothing(t *testing.T) {
	ini, peer, ps, peerTr, myTr := dpdProbeLink(t)
	peerAddr, ok := peerTr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("peer transport local address is not *net.UDPAddr")
	}

	request := probeAsk(ps, ini, myTr, probeGridSize(t, ini, false))
	sent := rtxRecv(t, peerTr)
	if sent == nil {
		t.Fatal("the probe never reached the peer")
	}

	// The peer's socket goes away, so the next datagram to it draws a Port Unreachable
	// from the peer's own address. The send after that meets the queued error and drains
	// it through the transport's error-queue reader.
	if err := peerTr.Close(); err != nil {
		t.Fatalf("close the peer transport: %v", err)
	}
	icmpQueue(t, myTr, peerAddr)
	if err := myTr.Send(make([]byte, 28), peerAddr); err != nil {
		t.Fatalf("a send after a Port Unreachable from the peer failed: %v", err)
	}
	icmpNoRefusal(t, myTr, "after a Port Unreachable from the peer")

	// A router's Fragmentation Needed for this peer, the ICMP error the engine reads.
	ps.handleSizeRefusal(ini, myTr, transport.SizeRefusal{
		Peer:     netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), uint16(peerAddr.Port)),
		Outcome:  probe.ErrQueueMTUReported,
		MTU:      1300,
		Offender: netip.MustParseAddr("192.0.2.1"),
	}, slogutil.DiscardLogger())

	if ini.State != StateEstablished {
		t.Fatalf("SA state %v after ICMP errors naming the peer, want Established", ini.State)
	}
	if ps.pendingProbe == nil {
		t.Fatal("ICMP errors naming the peer ended the outstanding probe")
	}
	probeUnanswered(t, request, "after ICMP errors naming the peer")

	probeDeliver(ps, ini, myTr, winInformationalAnswer(t, peer, parseMsg(t, sent).Header.MessageID))
	if result := probeReply(t, request, "after the peer answered"); result.Outcome == ikeprobe.OutcomeSAFailed {
		t.Fatal("the probe answered sa-failed although the peer answered")
	}
	if ini.State != StateEstablished {
		t.Fatalf("SA state %v after the peer answered, want Established", ini.State)
	}
}
