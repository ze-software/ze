// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- LCP Protocol-Reject for unsupported protocols.
// Related: session.go -- clientSupportsProtocol, sendProtocolReject, runIPCP.
// Related: network_phase.go -- keepaliveLoop.

package pppoeclient

import (
	"bytes"
	"encoding/binary"
	"testing"
	"testing/synctest"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	pppkernel "github.com/ze-software/ze/internal/component/l2tp/ppp/kernel"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// rawPPPFrame builds a PPP frame with protocol and raw payload bytes.
func rawPPPFrame(proto uint16, payload []byte) readFrame {
	var buf [ppp.MaxFrameBufLen]byte
	off := ppp.WriteFrame(buf[:], 0, proto, payload)
	return readFrame{data: bytes.Clone(buf[:off])}
}

// TestClientNegotiateIPCPProtocolReject verifies that while LCP is Opened
// during IPCP negotiation, unsupported protocols draw an LCP Protocol-Reject
// carrying a monotonically advancing Identifier, the 2-octet Rejected-Protocol,
// and Rejected-Information truncated to peerMRU, while supported protocols do not.
//
// VALIDATES: RFC 1661 Section 5.7 during negotiateIPCP for unknown protocols
// (CCP, 0x8099) and disabled IPv6CP (0x8057), and RFC 1661 Section 3.6 silent
// discard for supported IPv4 (0x0021).
// PREVENTS: silent drop of unsupported protocols in negotiateIPCP, which left
// dual-stack peers retransmitting IPv6CP until timeout.
func TestClientNegotiateIPCPProtocolReject(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const peerMRU = 100
		w := &frameLog{}
		frames := make(chan readFrame)
		stop := make(chan struct{})
		ended := make(chan struct{})
		go func() {
			defer close(ended)
			var buf [ppp.MaxFrameBufLen]byte
			_, _ = negotiateIPCP(w, testFrameReader(t, frames, nil), buf[:], 1, peerMRU, nil,
				new(uint8), stop, slogutil.DiscardLogger(), new(uint8), nil)
		}()
		synctest.Wait()

		// Frame 0 is the initial IPCP Configure-Request.
		if got := len(w.snapshot()); got != 1 {
			t.Fatalf("initial frames written = %d, want 1", got)
		}

		// 1. CCP (0x80fd) draws LCP Protocol-Reject with advancing Identifier.
		ccpPayload := []byte{0x01, 0x11, 0x00, 0x04}
		frames <- rawPPPFrame(0x80fd, ccpPayload)
		synctest.Wait()

		written := w.snapshot()
		if len(written) != 2 {
			t.Fatalf("after CCP: %d frames written, want 2", len(written))
		}
		proto, rej1 := recordedNCP(t, w, 1)
		if proto != ppp.ProtoLCP || rej1.Code != ppp.LCPProtocolReject {
			t.Fatalf("CCP reply = proto %#x code %d, want LCP Protocol-Reject", proto, rej1.Code)
		}
		if len(rej1.Data) < 2 || binary.BigEndian.Uint16(rej1.Data[:2]) != 0x80fd {
			t.Fatalf("CCP Rejected-Protocol = % x, want 80 fd", rej1.Data)
		}
		if !bytes.Equal(rej1.Data[2:], ccpPayload) {
			t.Fatalf("CCP Rejected-Information = % x, want % x", rej1.Data[2:], ccpPayload)
		}

		// 2. IPv6CP (0x8057) when ipv6 is nil draws LCP Protocol-Reject.
		v6Frame := exchangeFrame(ppp.ProtoIPv6CP, ppp.LCPConfigureRequest, 41,
			ipv6cpOption(ppp.IPv6CPInterfaceID{0, 0, 0, 0, 0, 0, 0, 1}))
		frames <- readFrame{data: v6Frame}
		synctest.Wait()

		written = w.snapshot()
		if len(written) != 3 {
			t.Fatalf("after disabled IPv6CP: %d frames written, want 3", len(written))
		}
		proto, rej2 := recordedNCP(t, w, 2)
		if proto != ppp.ProtoLCP || rej2.Code != ppp.LCPProtocolReject {
			t.Fatalf("IPv6CP reply = proto %#x code %d, want LCP Protocol-Reject", proto, rej2.Code)
		}
		if rej2.Identifier == rej1.Identifier {
			t.Fatalf("IPv6CP Protocol-Reject Identifier %d repeated previous %d", rej2.Identifier, rej1.Identifier)
		}
		if len(rej2.Data) < 2 || binary.BigEndian.Uint16(rej2.Data[:2]) != ppp.ProtoIPv6CP {
			t.Fatalf("IPv6CP Rejected-Protocol = % x, want 80 57", rej2.Data)
		}
		if !bytes.Equal(rej2.Data[2:], v6Frame[2:]) {
			t.Fatalf("IPv6CP Rejected-Information = % x, want % x", rej2.Data[2:], v6Frame[2:])
		}

		// 3. Oversized non-LCP unknown protocol (0x8099) is truncated to peerMRU.
		bigPayload := bytes.Repeat([]byte{0xde}, 200)
		frames <- rawPPPFrame(0x8099, bigPayload)
		synctest.Wait()

		written = w.snapshot()
		if len(written) != 4 {
			t.Fatalf("after oversized unknown protocol: %d frames written, want 4", len(written))
		}
		if got := len(written[3]) - ppp.ProtoFieldLen; got != peerMRU {
			t.Fatalf("Protocol-Reject Information field = %d octets, want peerMRU %d", got, peerMRU)
		}
		proto, rej3 := recordedNCP(t, w, 3)
		if proto != ppp.ProtoLCP || rej3.Code != ppp.LCPProtocolReject {
			t.Fatalf("0x8099 reply = proto %#x code %d, want LCP Protocol-Reject", proto, rej3.Code)
		}
		if rej3.Identifier == rej2.Identifier {
			t.Fatalf("0x8099 Protocol-Reject Identifier %d repeated previous %d", rej3.Identifier, rej2.Identifier)
		}
		if binary.BigEndian.Uint16(rej3.Data[:2]) != 0x8099 {
			t.Fatalf("0x8099 Rejected-Protocol = % x, want 80 99", rej3.Data[:2])
		}
		wantInfoLen := peerMRU - 4 - 2
		if !bytes.Equal(rej3.Data[2:], bigPayload[:wantInfoLen]) {
			t.Fatalf("0x8099 Rejected-Information len = %d, want %d bytes of payload", len(rej3.Data[2:]), wantInfoLen)
		}

		// 4. Supported IPv4 (0x0021) while IPCP is not Opened is silently discarded.
		frames <- rawPPPFrame(ppp.ProtoIPv4, []byte{0x45, 0x00, 0x00, 0x14})
		synctest.Wait()
		if got := len(w.snapshot()); got != 4 {
			t.Fatalf("supported IPv4 drew a reply (%d total frames, want 4)", got)
		}

		close(stop)
		<-ended
	})
}

// TestClientKeepaliveLoopProtocolReject verifies that in the connected network
// phase, keepaliveLoop answers unsupported protocols (unknown protocols and
// IPv6CP when disabled) with an LCP Protocol-Reject continuing the session's
// lcpIdentifier sequence and bounded by peerMRU, while never Protocol-Rejecting
// supported protocols.
//
// VALIDATES: RFC 1661 Section 5.7 in keepaliveLoop and RFC 1661 Section 3.6
// silent discard for supported protocols.
// PREVENTS: silent drop of unsupported protocols after IPCP opens.
func TestClientKeepaliveLoopProtocolReject(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const peerMRU = 80
		w := newConnectedExchange()
		s, _ := w.session(t)
		s.current.peerMRU = peerMRU
		s.lcpIdentifier = 10
		done := make(chan struct{})
		stop := make(chan struct{})
		defer close(stop)
		go keepaliveLoop(s, done, stop, slogutil.DiscardLogger())
		synctest.Wait()

		// 1. CCP (0x80fd) draws Protocol-Reject with Identifier 11.
		ccpPayload := []byte{0x01, 0x05, 0x00, 0x04}
		w.frames <- rawPPPFrame(0x80fd, ccpPayload)
		rej1 := w.take(t, ppp.ProtoLCP, ppp.LCPProtocolReject)
		if rej1[3] != 11 {
			t.Fatalf("first Protocol-Reject Identifier = %d, want 11", rej1[3])
		}
		if binary.BigEndian.Uint16(rej1[6:8]) != 0x80fd {
			t.Fatalf("first Rejected-Protocol = % x, want 80 fd", rej1[6:8])
		}
		if !bytes.Equal(rej1[8:], ccpPayload) {
			t.Fatalf("first Rejected-Information = % x, want % x", rej1[8:], ccpPayload)
		}

		// 2. IPv6CP (0x8057) when ipv6 is nil draws Protocol-Reject with Identifier 12.
		w.send(ppp.ProtoIPv6CP, ppp.LCPConfigureRequest, 7, ipv6cpOption(ppp.IPv6CPInterfaceID{0, 0, 0, 0, 0, 0, 0, 1}))
		rej2 := w.take(t, ppp.ProtoLCP, ppp.LCPProtocolReject)
		if rej2[3] != 12 {
			t.Fatalf("second Protocol-Reject Identifier = %d, want 12", rej2[3])
		}
		if binary.BigEndian.Uint16(rej2[6:8]) != ppp.ProtoIPv6CP {
			t.Fatalf("second Rejected-Protocol = % x, want 80 57", rej2[6:8])
		}

		// 3. Oversized unknown protocol (0x002b) is truncated to peerMRU.
		bigPayload := bytes.Repeat([]byte{0xab}, 150)
		w.frames <- rawPPPFrame(0x002b, bigPayload)
		rej3 := w.take(t, ppp.ProtoLCP, ppp.LCPProtocolReject)
		if rej3[3] != 13 {
			t.Fatalf("third Protocol-Reject Identifier = %d, want 13", rej3[3])
		}
		if got := len(rej3) - ppp.ProtoFieldLen; got != peerMRU {
			t.Fatalf("Protocol-Reject Information field = %d octets, want peerMRU %d", got, peerMRU)
		}
		if binary.BigEndian.Uint16(rej3[6:8]) != 0x002b {
			t.Fatalf("third Rejected-Protocol = % x, want 00 2b", rej3[6:8])
		}
		wantInfoLen := peerMRU - 4 - 2
		if !bytes.Equal(rej3[8:], bigPayload[:wantInfoLen]) {
			t.Fatalf("third Rejected-Information len = %d, want %d", len(rej3[8:]), wantInfoLen)
		}

		// 4. Supported IPv4 (0x0021) and PAP (0xc023) draw no Protocol-Reject.
		w.frames <- rawPPPFrame(ppp.ProtoIPv4, []byte{0x45, 0x00, 0x00, 0x14})
		w.frames <- rawPPPFrame(ppp.ProtoPAP, []byte{0x02, 0x01, 0x00, 0x05, 0x00})
		w.quiet(t)

		if workerEnded(done) {
			t.Fatal("unsupported protocols ended the connected session")
		}
	})
}

// TestClientIPv6EnabledDoesNotProtocolRejectIPv6 verifies that when IPv6CP is
// enabled on the session, neither IPv6CP (0x8057) nor IPv6 (0x0057) draws an
// LCP Protocol-Reject.
//
// VALIDATES: clientSupportsProtocol admits IPv6CP and IPv6 when ipv6 != nil.
// PREVENTS: falsely Protocol-Rejecting IPv6 traffic on a dual-stack link.
func TestClientIPv6EnabledDoesNotProtocolRejectIPv6(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var v6 []pppkernel.NPMode
		w, s, done := ipcpIndependence(t, true, &v6)
		stop := make(chan struct{})
		defer close(stop)
		go keepaliveLoop(s, done, stop, slogutil.DiscardLogger())
		synctest.Wait()

		// IPv6 (0x0057) reaching userspace is silently discarded, not rejected.
		w.frames <- rawPPPFrame(ppp.ProtoIPv6, []byte{0x60, 0x00, 0x00, 0x00})
		w.quiet(t)

		// IPv6CP (0x8057) is handled by the IPv6CP automaton, not Protocol-Rejected.
		w.send(ppp.ProtoIPv6CP, ppp.LCPTerminateRequest, 19, nil)
		ack := w.take(t, ppp.ProtoIPv6CP, ppp.LCPTerminateAck)
		if ack[3] != 19 {
			t.Fatalf("IPv6CP Terminate-Ack Identifier = %d, want 19", ack[3])
		}
		w.quiet(t)
	})
}
