// VALIDATES: discovery remains active during PPP, and only a PADT for the
// established interface, MAC pair and session stops further PPP writes.
// RFC: rfc/short/rfc2516.md
package pppoeclient

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
)

// padtChannel deliberately keeps accepting writes after Close: a closed raw
// descriptor could be reused, so the session must guard subsequent writes.
type padtChannel struct {
	bytes.Buffer
	closes int
}

func (c *padtChannel) Close() error {
	c.closes++
	return nil
}

// RFC requirement: RFC2516-5.5-4 positive -- a matching received PADT closes the client transport, publishes Done and refuses a later PPP Terminate-Ack.
// RFC requirement: RFC2516-5.5-4 negative -- wrong interface, source, destination, session, code and malformed discovery frames leave PPP usable before the matching PADT.
// MUTATION: remove link.Close from watchPADT's matching-PADT branch to leave PPP writable.
func TestRFC2516ClientPADTStopsOnlyMatchingSession(t *testing.T) {
	channel := &padtChannel{}
	transportCloses := 0
	link := &sessionLink{channel: channel, stopped: make(chan struct{}), closeTransport: func() { transportCloses++ }}
	var buf [pppoe.EthMaxLen]byte
	valid := append([]byte(nil), pppoe.BuildPADT(buf[:], testACMAC, testHostMAC, 0x1234, "ze")...)
	if valid == nil {
		t.Fatal("BuildPADT failed")
	}
	wrongSource := append([]byte(nil), valid...)
	wrongSource[pppoe.EthALen+5] ^= 1
	wrongDestination := append([]byte(nil), valid...)
	wrongDestination[5] ^= 1
	wrongSession := append([]byte(nil), valid...)
	wrongSession[pppoe.EthHdrLen+3] ^= 1
	wrongCode := append([]byte(nil), valid...)
	wrongCode[pppoe.EthHdrLen+1] = pppoe.CodePADS
	frames := []struct {
		frame   []byte
		ifindex int
	}{
		{valid, 2},
		{wrongSource, 1},
		{wrongDestination, 1},
		{wrongSession, 1},
		{wrongCode, 1},
		{valid[:pppoe.EthHdrLen], 1},
		{valid, 1},
	}
	previous := readDiscoveryFrame
	defer func() { readDiscoveryFrame = previous }()
	reads := 0
	readDiscoveryFrame = func(_ int, dst []byte) (int, int, error) {
		if reads >= len(frames) {
			t.Fatal("PADT monitor kept reading after the matching PADT")
		}
		if _, err := link.Write([]byte{byte(reads)}); err != nil {
			t.Fatalf("PPP stopped before frame %d: %v", reads, err)
		}
		frame := frames[reads]
		reads++
		return copy(dst, frame.frame), frame.ifindex, nil
	}
	done := make(chan struct{})
	watchPADT(0, 1, testHostMAC, testACMAC, 0x1234, link, make(chan struct{}), done, slog.Default())
	if reads != len(frames) || channel.closes != 1 || transportCloses != 1 {
		t.Fatalf("reads=%d channel closes=%d transport closes=%d", reads, channel.closes, transportCloses)
	}
	select {
	case <-link.stopped:
	default:
		t.Fatal("matching PADT did not publish session Done")
	}
	before := channel.Len()
	var response [ppp.MaxFrameBufLen]byte
	sendTerminateAck(link, response[:], ppp.LCPPacket{Identifier: 9})
	if channel.Len() != before {
		t.Fatal("client transmitted PPP termination after receiving PADT")
	}
	if n, err := link.Write([]byte("PPP")); n != 0 || !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("write after PADT = %d, %v; want closed transport", n, err)
	}
	if err := link.Close(); err != nil {
		t.Fatal(err)
	}
	if channel.closes != 1 || transportCloses != 1 {
		t.Fatal("cleanup closed an already released descriptor")
	}
}

// VALIDATES: the owner's stop signal closes PPP before Done is published and refuses subsequent PPP writes, including a Terminate-Ack.
// MUTATION: permit sessionLink.Write after closed to send PPP through a released descriptor.
func TestRFC2516ClientLocalStopDisablesPPP(t *testing.T) {
	channel := &padtChannel{}
	transportClosed := false
	link := &sessionLink{channel: channel, stopped: make(chan struct{}), closeTransport: func() { transportClosed = true }}
	if _, err := link.Write([]byte("before shutdown")); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	close(stop)
	done := make(chan struct{})
	watchPADT(0, 1, testHostMAC, testACMAC, 0x1234, link, stop, done, slog.Default())
	<-link.stopped
	if channel.closes != 1 || !transportClosed {
		t.Fatal("Done was published before PPP descriptors closed")
	}
	before := channel.Len()
	var response [ppp.MaxFrameBufLen]byte
	sendTerminateAck(link, response[:], ppp.LCPPacket{Identifier: 10})
	if channel.Len() != before {
		t.Fatal("client transmitted PPP termination after local shutdown")
	}
}

// RFC requirement: RFC2516-5.5-4 positive -- the client closes PPP before publishing an outbound PADT and refuses PPP termination writes at that send boundary.
// MUTATION: remove sendPADT's link.Close call to leave PPP live when PADT is emitted.
func TestRFC2516ClientSentPADTStopsPPPBeforeSend(t *testing.T) {
	channel := &padtChannel{}
	transportClosed := false
	link := &sessionLink{channel: channel, stopped: make(chan struct{}), closeTransport: func() { transportClosed = true }}
	previous := sendDiscoveryFrame
	defer func() { sendDiscoveryFrame = previous }()
	sent := 0
	sendDiscoveryFrame = func(_ int, ifindex int, frame []byte) error {
		sent++
		packet, err := pppoe.ParseDiscovery(frame)
		if err != nil {
			t.Fatal(err)
		}
		if ifindex != 1 || packet.Code != pppoe.CodePADT || packet.SID != 0x1234 {
			t.Fatalf("outbound PADT = %+v on interface %d", packet, ifindex)
		}
		if packet.SrcMAC != testHostMAC || packet.DstMAC != testACMAC {
			t.Fatalf("outbound PADT has wrong MAC pair: %+v", packet)
		}
		if channel.closes != 1 || !transportClosed {
			t.Fatal("PADT reached the discovery socket before PPP closed")
		}
		var response [ppp.MaxFrameBufLen]byte
		sendTerminateAck(link, response[:], ppp.LCPPacket{Identifier: 11})
		if channel.Len() != 0 {
			t.Fatal("PPP termination was transmitted after outbound PADT")
		}
		return nil
	}
	sendPADT(-1, 1, testHostMAC, testACMAC, 0x1234, link)
	if sent != 1 {
		t.Fatalf("sent %d PADTs, want one", sent)
	}
}
