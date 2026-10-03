// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- PPPoE client session teardown
// Related: dialer.go -- tryReadPADO, tryReadPADS and sendPADT, which pick and
//   address the session the Host terminates; superviseNetworkPhase, which ends
//   the session when its network phase does
// Related: network_phase.go -- keepaliveLoop, which answers the Access
//   Concentrator's LCP Terminate-Request
// Related: rfc2516_padt_lifetime_test.go -- the PADT a Host receives
// RFC: rfc/short/rfc2516.md -- Sections 5.5 and 7
//
// VALIDATES: the Host half of two RFC 2516 obligations the Access Concentrator
// half of which the pppoe package proves. Section 5.5: the PADT the Host sends
// is addressed to a unicast Ethernet address, carries CODE 0xa7, and names the
// session it terminates. Section 7: when LCP terminates, the Host stops using
// the PPPoE session.
// METHOD: the discovery seams hand the Host an Access Concentrator's PADO and
// PADS, and the PADT the Host then sends is read as raw octets. The network
// phase runs over a hostChannel, which keeps each PPP frame the Host wrote.
// PREVENTS: a Host PADT sent to a broadcast or multicast address, or naming a
// session ID another Access Concentrator offered; a Host that keeps writing PPP
// on a session whose LCP the Access Concentrator terminated.

package pppoeclient

import (
	"encoding/binary"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
)

// hostDiscoveryUniq is the Host-Uniq the Host's PADI carried.
var hostDiscoveryUniq = [4]byte{9, 8, 7, 6}

// acOffer builds the PADO the Access Concentrator at acMAC answers the Host's
// PADI with.
func acOffer(t *testing.T, acMAC [pppoe.EthALen]byte) []byte {
	t.Helper()
	padi := pppoe.Packet{Code: pppoe.CodePADI, SrcMAC: testHostMAC, Tags: []pppoe.Tag{
		{Type: pppoe.TagServiceName}, {Type: pppoe.TagHostUniq, Value: hostDiscoveryUniq[:]},
	}}
	var buf [pppoe.EthMaxLen]byte
	pado := pppoe.BuildPADO(buf[:], acMAC, &padi, "ac", nil, nil)
	if pado == nil {
		t.Fatal("BuildPADO returned nil")
	}
	return append([]byte(nil), pado...)
}

// acConfirmation builds the PADS the Access Concentrator at acMAC confirms sid
// with.
func acConfirmation(t *testing.T, acMAC [pppoe.EthALen]byte, sid uint16) []byte {
	t.Helper()
	padr := pppoe.Packet{Code: pppoe.CodePADR, SrcMAC: testHostMAC, Tags: []pppoe.Tag{{Type: pppoe.TagServiceName}}}
	var buf [pppoe.EthMaxLen]byte
	pads := pppoe.BuildPADS(buf[:], acMAC, &padr, "ac", sid)
	if pads == nil {
		t.Fatal("BuildPADS returned nil")
	}
	return append([]byte(nil), pads...)
}

// hostDiscovers runs the Host's discovery reads over pado and then pads, and
// answers the Access Concentrator address and session ID the Host holds, as
// Dial derives them.
func hostDiscovers(t *testing.T, pado []byte, pads ...[]byte) ([pppoe.EthALen]byte, uint16) {
	t.Helper()
	serveFrame(t, pado)
	offer, ok := tryReadPADO(0, 1, hostDiscoveryUniq, "")
	if !ok {
		t.Fatal("the Host refused the Access Concentrator's PADO")
	}
	var acMAC [pppoe.EthALen]byte
	copy(acMAC[:], offer.SrcMAC[:])
	for _, frame := range pads {
		serveFrame(t, frame)
		sid, err := tryReadPADS(0, 1, acMAC)
		if err != nil {
			t.Fatalf("tryReadPADS: %v", err)
		}
		if sid != 0 {
			return acMAC, sid
		}
	}
	t.Fatal("the Host accepted no PADS")
	return acMAC, 0
}

// capturePADT runs sendPADT for the session and answers the one frame it put
// on the discovery socket.
func capturePADT(t *testing.T, acMAC [pppoe.EthALen]byte, sid uint16) []byte {
	t.Helper()
	previous := sendDiscoveryFrame
	t.Cleanup(func() { sendDiscoveryFrame = previous })
	var frames [][]byte
	sendDiscoveryFrame = func(_ int, _ int, frame []byte) error {
		frames = append(frames, append([]byte(nil), frame...))
		return nil
	}
	sendPADT(-1, 1, testHostMAC, acMAC, sid, nil)
	if len(frames) != 1 {
		t.Fatalf("sendPADT put %d frames on the discovery socket, want 1", len(frames))
	}
	return frames[0]
}

// assertHostPADT checks the raw PADT octets: DESTINATION_ADDR is acMAC and
// unicast, CODE is 0xa7 and SESSION_ID is sid.
func assertHostPADT(t *testing.T, frame []byte, acMAC [pppoe.EthALen]byte, sid uint16) {
	t.Helper()
	if len(frame) < pppoe.EthHdrLen+pppoe.PPPoEHdrLen {
		t.Fatalf("PADT is %d octets", len(frame))
	}
	var destination [pppoe.EthALen]byte
	copy(destination[:], frame[:pppoe.EthALen])
	if destination != acMAC {
		t.Fatalf("PADT DESTINATION_ADDR % x, want the Access Concentrator % x", destination, acMAC)
	}
	if destination[0]&0x01 != 0 {
		t.Fatalf("PADT DESTINATION_ADDR % x is a group address", destination)
	}
	if code := frame[pppoe.EthHdrLen+1]; code != 0xa7 {
		t.Fatalf("PADT CODE %#x, want 0xa7", code)
	}
	if got := binary.BigEndian.Uint16(frame[pppoe.EthHdrLen+2:]); got != sid {
		t.Fatalf("PADT SESSION_ID %#04x, want the session's %#04x", got, sid)
	}
}

// TestRFC2516HostPADTNamesItsAccessConcentratorAndSession discovers one session
// and terminates it.
//
// RFC 2516 Section 5.5: "The DESTINATION_ADDR field is a unicast Ethernet
// address, the CODE field is set to 0xa7 and the SESSION_ID MUST be set to
// indicate which session is to be terminated."
//
// RFC requirement: RFC2516-5.5-3 positive -- the PADT the Host sends for a
// session it discovered is addressed to the unicast MAC of the Access
// Concentrator that offered it, carries CODE 0xa7 and the SESSION_ID that
// Access Concentrator's PADS assigned (dialer.go tryReadPADO, tryReadPADS,
// sendPADT).
func TestRFC2516HostPADTNamesItsAccessConcentratorAndSession(t *testing.T) {
	acMAC, sid := hostDiscovers(t, acOffer(t, testACMAC), acConfirmation(t, testACMAC, 0x2345))
	if acMAC != testACMAC || sid != 0x2345 {
		t.Fatalf("the Host holds AC % x session %#04x, want % x session 0x2345", acMAC, sid, testACMAC)
	}
	assertHostPADT(t, capturePADT(t, acMAC, sid), testACMAC, 0x2345)
}

// TestRFC2516HostPADTNeverAddressesAGroupOrAForeignSession offers the Host the
// inputs that would make its PADT violate Section 5.5: a PADO from a broadcast
// or multicast source, and a PADS from another Access Concentrator.
//
// RFC 2516 Section 5.5: "The DESTINATION_ADDR field is a unicast Ethernet
// address, the CODE field is set to 0xa7 and the SESSION_ID MUST be set to
// indicate which session is to be terminated."
//
// RFC requirement: RFC2516-5.5-3 negative -- a PADO whose source is the
// broadcast or a multicast address is never an offer, so no session and no PADT
// can be addressed to it; a PADS from another Access Concentrator does not give
// the Host its SESSION_ID, and the PADT names the session its own Access
// Concentrator assigned (dialer.go tryReadPADO, tryReadPADS, sendPADT).
func TestRFC2516HostPADTNeverAddressesAGroupOrAForeignSession(t *testing.T) {
	for name, source := range map[string][pppoe.EthALen]byte{
		"broadcast": {0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		"multicast": {0x01, 0x00, 0x5e, 0x00, 0x00, 0x01},
	} {
		pado := acOffer(t, testACMAC)
		copy(pado[pppoe.EthALen:2*pppoe.EthALen], source[:])
		serveFrame(t, pado)
		if offer, ok := tryReadPADO(0, 1, hostDiscoveryUniq, ""); ok {
			t.Fatalf("a PADO from the %s address % x was taken as an offer: %+v", name, source, offer)
		}
	}

	otherAC := [pppoe.EthALen]byte{0x02, 0x99, 0x88, 0x77, 0x66, 0x55}
	acMAC, sid := hostDiscovers(t, acOffer(t, testACMAC),
		acConfirmation(t, otherAC, 0x3456),
		acConfirmation(t, testACMAC, 0x2345),
	)
	if sid != 0x2345 {
		t.Fatalf("the Host took SESSION_ID %#04x, want its own Access Concentrator's 0x2345", sid)
	}
	assertHostPADT(t, capturePADT(t, acMAC, sid), testACMAC, 0x2345)
}

// hostChannel is a PPP channel that keeps each frame the Host wrote whole, and
// counts its closes. Like padtChannel it keeps accepting writes after Close, so
// only the session's own guard can refuse them.
type hostChannel struct {
	frameLog
	closes int
}

func (c *hostChannel) Close() error {
	c.closes++
	return nil
}

func (c *hostChannel) Read([]byte) (int, error) { return 0, io.EOF }

// networkPhase runs superviseNetworkPhase over a hostChannel and answers the
// link, its frames channel and the channel the Host wrote to.
func networkPhase(t *testing.T) (*sessionLink, chan<- readFrame, *hostChannel, <-chan struct{}) {
	t.Helper()
	channel := &hostChannel{}
	link := &sessionLink{channel: channel, stopped: make(chan struct{}), closeTransport: func() {}}
	frames := make(chan readFrame)
	done := make(chan struct{})
	go superviseNetworkPhase(link, &sessionResult{frames: frames, magic: 0x0badcafe}, done, slog.Default())
	t.Cleanup(func() { _ = link.Close() }) //nolint:errcheck // test cleanup
	return link, frames, channel, done
}

// hostLCPWrites answers the LCP Code and Identifier of each frame the Host
// wrote, and fails on a frame that is not LCP.
func hostLCPWrites(t *testing.T, channel *hostChannel) [][2]uint8 {
	t.Helper()
	packets := channel.lcpPackets(t)
	if frames := len(channel.snapshot()); frames != len(packets) {
		t.Fatalf("the Host wrote %d frames, %d of them LCP", frames, len(packets))
	}
	out := make([][2]uint8, 0, len(packets))
	for _, pkt := range packets {
		out = append(out, [2]uint8{pkt.Code, pkt.Identifier})
	}
	return out
}

// waitClosed fails unless ch closes within five seconds.
func waitClosed(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("%s did not happen", what)
	}
}

// TestRFC2516HostStopsUsingTheSessionWhenLCPTerminates has the Access
// Concentrator terminate LCP in the network phase.
//
// RFC 2516 Section 7: "When LCP terminates, the Host and Access concentrator
// MUST stop using that PPPoE session."
//
// RFC requirement: RFC2516-7-3 positive -- after the Access Concentrator's LCP
// Terminate-Request, the Host writes its Terminate-Ack, closes the session's PPP
// channel and transport, publishes Done, and every later PPP write is refused
// (network_phase.go keepaliveLoop, dialer.go superviseNetworkPhase).
func TestRFC2516HostStopsUsingTheSessionWhenLCPTerminates(t *testing.T) {
	link, frames, channel, done := networkPhase(t)
	frames <- serverFrame(ppp.LCPTerminateRequest, 5, nil)
	waitClosed(t, done, "the network phase ending")
	waitClosed(t, link.stopped, "Done")

	writes := hostLCPWrites(t, channel)
	if len(writes) != 1 || writes[0] != [2]uint8{ppp.LCPTerminateAck, 5} {
		t.Fatalf("the Host wrote %v, want one Terminate-Ack for Identifier 5", writes)
	}
	if channel.closes != 1 {
		t.Fatalf("the PPP channel closed %d times, want 1", channel.closes)
	}
	if _, err := link.Write(serverFrame(ppp.LCPEchoRequest, 6, nil).data); err == nil {
		t.Fatal("the Host could still write PPP on the session after LCP terminated")
	}
	var buf [ppp.MaxFrameBufLen]byte
	sendEchoReply(link, buf[:], ppp.LCPPacket{Identifier: 7}, 0x0badcafe)
	if got := hostLCPWrites(t, channel); len(got) != 1 {
		t.Fatalf("the Host wrote %v after LCP terminated", got[1:])
	}
}

// TestRFC2516HostKeepsTheSessionUntilLCPTerminates sends the network phase LCP
// traffic that does not terminate LCP, then the Terminate-Request.
//
// RFC 2516 Section 7: "When LCP terminates, the Host and Access concentrator
// MUST stop using that PPPoE session."
//
// RFC requirement: RFC2516-7-3 negative -- an Echo-Request and a stray
// Terminate-Ack leave the session in use: Done stays open and the Host answers
// the next Echo-Request; only the Terminate-Request that follows ends it
// (network_phase.go keepaliveLoop, dialer.go superviseNetworkPhase).
func TestRFC2516HostKeepsTheSessionUntilLCPTerminates(t *testing.T) {
	link, frames, channel, done := networkPhase(t)
	frames <- serverFrame(ppp.LCPEchoRequest, 7, []byte{0, 0, 0, 1})
	frames <- serverFrame(ppp.LCPTerminateAck, 8, nil)
	// The loop takes a frame only after it finished the previous one, so this
	// send returning means the stray Terminate-Ack was handled.
	frames <- serverFrame(ppp.LCPEchoRequest, 9, []byte{0, 0, 0, 1})
	select {
	case <-link.stopped:
		t.Fatal("the session stopped before LCP terminated")
	default:
	}

	frames <- serverFrame(ppp.LCPTerminateRequest, 10, nil)
	waitClosed(t, done, "the network phase ending")
	want := [][2]uint8{{ppp.LCPEchoReply, 7}, {ppp.LCPEchoReply, 9}, {ppp.LCPTerminateAck, 10}}
	got := hostLCPWrites(t, channel)
	if len(got) != len(want) {
		t.Fatalf("the Host wrote %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("the Host wrote %v, want %v", got, want)
		}
	}
	waitClosed(t, link.stopped, "Done after the Terminate-Request")
}
