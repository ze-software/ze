// Design: docs/guide/l2tp.md -- RFC 1661 link phases driven through run()
// Related: lcp_lifecycle_test.go -- the run() harness these tests reuse

package ppp

import (
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"os"
	"testing"
	"testing/synctest"
	"time"
)

// startPhaseSession runs a session from Initial with IPCP enabled over the
// lifecycle harness. auth is the configured method, and echo the keepalive
// interval; ipcp enables IPCP. The returned channels carry the session's
// auth and address requests.
func startPhaseSession(t *testing.T, auth AuthMethod, ipcp bool, echo time.Duration) (*pppSession, net.Conn, chan AuthEvent, chan IPEvent) {
	t.Helper()
	return startFamilySession(t, auth, ipcp, false, echo)
}

// startFamilySession is startPhaseSession with IPv6CP selectable as well.
func startFamilySession(t *testing.T, auth AuthMethod, ipcp, ipv6cp bool, echo time.Duration) (*pppSession, net.Conn, chan AuthEvent, chan IPEvent) {
	t.Helper()
	s, _, _ := newRFC1661Session(LCPStateInitial)
	s.disableIPv6CP = !ipv6cp
	s.configuredAuthMethod = auth
	s.authTimeout = 20 * time.Second
	s.ipTimeout = 20 * time.Second
	s.echoInterval = echo
	s.disableIPCP = !ipcp
	authEvents := make(chan AuthEvent, 16)
	ipEvents := make(chan IPEvent, 4)
	s.authEventsOut = authEvents
	s.ipEventsOut = ipEvents
	return s, runLifecycleSession(t, s), authEvents, ipEvents
}

// TestRFC1661RunSendsLCPBeforeAnyOtherProtocol starts a real session
// goroutine with IPCP enabled and no authentication, and reads the first
// frame it writes.
//
// VALIDATES: RFC 1661 Section 3.1, LCP is sent first: the first frame run()
// writes is an LCP Configure-Request, although an NCP is enabled.
// PREVENTS: run() writing an NCP or authentication frame before LCP.
//
// RFC requirement: RFC1661-3.1-1 positive -- run() started from Initial with IPCP enabled and auth none writes an LCP (0xc021) Configure-Request as its first frame.
func TestRFC1661RunSendsLCPBeforeAnyOtherProtocol(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, peer, _, _ := startPhaseSession(t, AuthMethodNone, true, time.Hour)
		readLifecyclePacket(t, peer, ProtoLCP, LCPConfigureRequest)
	})
}

// TestRFC1661RunSendsNCPAfterLCPOpens drives LCP to Opened with no
// authentication, accepts the no-auth decision, and reads what follows.
//
// VALIDATES: RFC 1661 Section 3.1, after LCP, PPP sends NCP packets: an
// enabled IPCP puts its Configure-Request (0x8021) on the wire.
// PREVENTS: a runNCPPhase that negotiates no enabled network protocol.
//
// RFC requirement: RFC1661-3.1-2 positive -- after LCP Opened and the accepted no-auth decision, run() requests an IPv4 address and writes an IPCP (0x8021) Configure-Request.
func TestRFC1661RunSendsNCPAfterLCPOpens(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, peer, authEvents, ipEvents := startPhaseSession(t, AuthMethodNone, true, time.Hour)
		openLifecycleLCP(t, peer)
		awaitAuthRequest(t, authEvents)
		s.authRespCh <- authResponseMsg{accept: true}
		pendingLifecycleNCP(t, s, peer, ipEvents, AddressFamilyIPv4)
	})
}

// TestRFC1661NoNetworkPhaseBeforeAuthenticationCompletes holds a CHAP
// exchange open with IPCP enabled, then completes it.
//
// VALIDATES: RFC 1661 Section 3.5, the Network-Layer Protocol phase does not
// start while authentication is pending, and does start once it completes.
// PREVENTS: runNCPPhase moved ahead of, or run beside, runAuthPhase.
//
// RFC requirement: RFC1661-3.5-3 negative -- with IPCP enabled and the CHAP Challenge unanswered, run() emits no address request (the first NCP step) while authentication is pending, and none after a rejecting decision.
// RFC requirement: RFC1661-3.5-3 positive -- the same session, once the CHAP Response is accepted, requests an IPv4 address and writes an IPCP Configure-Request.
func TestRFC1661NoNetworkPhaseBeforeAuthenticationCompletes(t *testing.T) {
	for _, accept := range []bool{true, false} {
		name := "rejected"
		if accept {
			name = "accepted"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, peer, authEvents, ipEvents := startPhaseSession(t, AuthMethodCHAPMD5, true, time.Hour)
				openLifecycleLCP(t, peer)
				challenge := readLifecyclePacket(t, peer, ProtoCHAP, CHAPCodeChallenge)
				time.Sleep(time.Second)
				synctest.Wait()
				if len(ipEvents) != 0 {
					t.Fatal("address requested while authentication is pending")
				}
				answerLifecycleChallenge(t, s, peer, authEvents, AuthMethodCHAPMD5, challenge, accept)
				if accept {
					pendingLifecycleNCP(t, s, peer, ipEvents, AddressFamilyIPv4)
					return
				}
				synctest.Wait()
				if len(ipEvents) != 0 {
					t.Fatal("address requested after authentication was rejected")
				}
			})
		})
	}
}

// TestRFC1661KeepaliveEchoOnlyInOpened runs the keepalive ticker through
// Opened and then out of it.
//
// VALIDATES: RFC 1661 Section 5.8, ze's own Echo-Request goes out in
// Opened, and after a renegotiation leaves Opened the next frame, past
// several echo intervals, is the Restart-timer retransmission of the
// Configure-Request, not an Echo-Request.
// PREVENTS: a keepalive ticker left running outside Opened.
//
// RFC requirement: RFC1661-5.8-2 positive -- in Opened with a one-second echo interval, run() writes an LCP Echo-Request.
// RFC requirement: RFC1661-5.8-2 negative -- after the peer's Configure-Request takes the session out of Opened, the next frame written, three seconds on with a one-second echo interval, is the Configure-Request retransmission and no Echo-Request.
func TestRFC1661KeepaliveEchoOnlyInOpened(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, peer, authEvents, _ := startPhaseSession(t, AuthMethodNone, false, time.Second)
		openLifecycleLCP(t, peer)
		awaitAuthRequest(t, authEvents)
		s.authRespCh <- authResponseMsg{accept: true}
		readLifecyclePacket(t, peer, ProtoLCP, LCPEchoRequest)

		request := restartLifecycleLCP(t, peer)
		resent := readLifecyclePacket(t, peer, ProtoLCP, LCPConfigureRequest)
		if resent.Identifier != request.Identifier {
			t.Fatalf("retransmission identifier = %d, want %d", resent.Identifier, request.Identifier)
		}
	})
}

// awaitAuthRequest waits for the session's EventAuthRequest.
func awaitAuthRequest(t *testing.T, authEvents <-chan AuthEvent) {
	t.Helper()
	select {
	case ev := <-authEvents:
		if _, ok := ev.(EventAuthRequest); !ok {
			t.Fatalf("auth event %T, want EventAuthRequest", ev)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no EventAuthRequest")
	}
}

// TestNCPHeldUntilNetworkPhase holds a CHAP exchange open with one NCP
// enabled, for IPCP and for IPV6CP, then accepts the Response.
//
// VALIDATES: RFC 1332 Section 2.1 and RFC 5072 Section 3, ze neither
// requests an address, installs one, nor writes any NCP frame while PPP is
// still in the Authentication phase; once authentication completes, the
// family's Control Protocol Configure-Request goes on the wire.
// PREVENTS: IPCP or IPV6CP started beside, or ahead of, authentication.
//
// RFC requirement: RFC1332-2.1-1 negative -- with IPCP enabled and the CHAP Challenge unanswered, no address request, no address installed (fake backend P2P calls) and no frame on the wire for three seconds.
// RFC requirement: RFC5072-3-1 negative -- with IPV6CP enabled and the CHAP Challenge unanswered, no IPV6CP Configure-Request (no frame at all) is written for three seconds and no IPv6 address is requested.
// RFC requirement: RFC5072-3-1 positive -- the same IPV6CP session, once the CHAP Response is accepted, writes an IPV6CP (0x8057) Configure-Request.
func TestNCPHeldUntilNetworkPhase(t *testing.T) {
	for _, family := range []AddressFamily{AddressFamilyIPv4, AddressFamilyIPv6} {
		t.Run(family.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				s, peer, authEvents, ipEvents := startFamilySession(t, AuthMethodCHAPMD5,
					family == AddressFamilyIPv4, family == AddressFamilyIPv6, time.Hour)
				openLifecycleLCP(t, peer)
				challenge := readLifecyclePacket(t, peer, ProtoCHAP, CHAPCodeChallenge)
				assertNoLifecycleFrame(t, peer, 3*time.Second)
				if len(ipEvents) != 0 {
					t.Fatal("address requested while authentication is pending")
				}
				backend, ok := s.backend.(*fakeBackend)
				if !ok {
					t.Fatalf("session backend is %T, want *fakeBackend", s.backend)
				}
				if calls := backend.P2PCalls(); len(calls) != 0 {
					t.Fatalf("address installed while authentication is pending: %+v", calls)
				}
				answerLifecycleChallenge(t, s, peer, authEvents, AuthMethodCHAPMD5, challenge, true)
				pendingLifecycleNCP(t, s, peer, ipEvents, family)
			})
		})
	}
}

// assertNoLifecycleFrame fails when the session writes any frame within d.
func assertNoLifecycleFrame(t *testing.T, peer net.Conn, d time.Duration) {
	t.Helper()
	if err := peer.SetReadDeadline(time.Now().Add(d)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, MaxFrameBufLen)
	n, err := peer.Read(buf)
	if err == nil {
		t.Fatalf("frame written while authentication is pending: %x", buf[:n])
	}
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("read: %v", err)
	}
}

// TestIPCPOnePacketPerFrameThroughRun reads the IPCP Configure-Request a
// running session writes, then hands it one frame carrying a peer IPCP
// Configure-Request followed by a second IPCP packet.
//
// VALIDATES: RFC 1332 Section 2, exactly one IPCP packet fills the
// Information field of a Protocol-0x8021 frame: ze's own frame is one packet
// whose Length covers the whole Information field, and in a received frame
// the octets past the first packet's Length are padding, never a second
// packet.
// PREVENTS: a send path that packs two packets into one frame, and a receive
// path that parses a second IPCP packet out of the same frame.
//
// RFC requirement: RFC1332-2-1 positive -- the IPCP Configure-Request run() writes is a Protocol-0x8021 frame whose IPCP Length field equals the length of the Information field.
// RFC requirement: RFC1332-2-1 negative -- a received 0x8021 frame holding a Configure-Request followed by a Terminate-Request draws the Configure-Ack alone: no Terminate-Ack is written within one second.
func TestIPCPOnePacketPerFrameThroughRun(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s, peer, authEvents, ipEvents := startPhaseSession(t, AuthMethodNone, true, time.Hour)
		openLifecycleLCP(t, peer)
		awaitAuthRequest(t, authEvents)
		s.authRespCh <- authResponseMsg{accept: true}
		select {
		case <-ipEvents:
		case <-time.After(5 * time.Second):
			t.Fatal("NCP never requested an address")
		}
		s.ipRespCh <- ipResponseMsg{
			accept: true, family: AddressFamilyIPv4,
			local: netip.MustParseAddr("192.0.2.1"), peer: netip.MustParseAddr("192.0.2.2"),
		}

		if err := peer.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, MaxFrameBufLen)
		n, err := peer.Read(buf)
		if err != nil {
			t.Fatalf("reading the IPCP Configure-Request: %v", err)
		}
		proto, info, _, err := ParseFrame(buf[:n])
		if err != nil {
			t.Fatal(err)
		}
		if proto != ProtoIPCP {
			t.Fatalf("protocol = %#x, want 0x8021", proto)
		}
		if len(info) < 4 || info[0] != LCPConfigureRequest {
			t.Fatalf("Information field %x is no IPCP Configure-Request", info)
		}
		if got := int(binary.BigEndian.Uint16(info[2:4])); got != len(info) {
			t.Fatalf("IPCP Length = %d, Information field = %d octets", got, len(info))
		}

		frame := lcpFrame(ProtoIPCP, LCPConfigureRequest, 0x51, []byte{IPCPOptIPAddress, 6, 192, 0, 2, 2})
		frame = append(frame, LCPTerminateRequest, 0x52, 0x00, 0x04)
		if err := peer.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, err := peer.Write(frame); err != nil {
			t.Fatal(err)
		}
		if ack := readLifecyclePacket(t, peer, ProtoIPCP, LCPConfigureAck); ack.Identifier != 0x51 {
			t.Fatalf("Configure-Ack identifier = %#x, want 0x51", ack.Identifier)
		}
		if err := peer.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		n, err = peer.Read(buf)
		if err == nil {
			t.Fatalf("frame written after the Configure-Ack: %x (the padding was parsed as a packet)", buf[:n])
		}
		if !errors.Is(err, os.ErrDeadlineExceeded) {
			t.Fatalf("read: %v", err)
		}
	})
}
