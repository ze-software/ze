// Design: docs/architecture/wire/pppoe.md -- Access Concentrator PPP start
// RFC: rfc/short/rfc2516.md
//
// RFC 2516 Section 7: "The Maximum-Receive-Unit (MRU) option MUST NOT be
// negotiated to a larger size than 1492."
//
// VALIDATES: the Access Concentrator side of the ceiling, over the real path:
// handlePADR admits a subscriber and starts the production PPP driver with the
// StartSession the server builds, and the LCP that driver runs on the PPP
// channel is read from the subscriber's end of a socket pair. The AC asks for
// an MRU of at most 1492, Acks a client MRU of 1492, and Naks a client MRU of
// 1500 with 1492 in its place.
// PREVENTS: server.go starting PPP with a MaxMRU above PPPoEMaxMTU, or none,
// which lets the driver accept the PPP default of 1500 on a PPPoE link.
//go:build linux

package pppoe

import (
	"encoding/binary"
	"log/slog"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

type mruCeilingBackend struct{ ppp.IfaceBackend }

// acPPPChannel admits one subscriber through handlePADR, with the production
// PPP driver running, and answers the subscriber's end of the PPP channel.
func acPPPChannel(t *testing.T) int {
	t.Helper()
	driver := ppp.NewProductionDriver(slog.Default(), mruCeilingBackend{})
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	// Cleanups run last-registered first: the socket pairs below close their
	// test ends before Stop, so the session's blocking channel read sees EOF
	// and Stop can join it.
	t.Cleanup(driver.Stop)
	channel := socketPair(t)
	unit := socketPair(t)

	key := CookieKey{}
	hwAddr := [EthALen]byte{0x02, 0, 0, 0, 0, 0x01}
	srcMAC := [EthALen]byte{0x02, 0, 0, 0, 0, 0x02}
	s := &InterfaceServer{
		ifName:            "eth0",
		hwAddr:            hwAddr,
		sessions:          newSessionTable("eth0", 8),
		cookieKey:         key,
		cookieTimeout:     30 * time.Second,
		maxSessionsPerMAC: 8,
		logger:            slog.Default(),
		pppDriver:         driver,
		sendFrameFn:       func([]byte) {},
		pppoeCreateFn: func(string, uint16, [EthALen]byte) (int, error) {
			return unix.Open("/dev/null", unix.O_RDWR|unix.O_CLOEXEC, 0)
		},
		devPPPSetupFn: func(int) (int, int, int, error) {
			return channel[0], unit[0], 1, nil
		},
	}
	s.handlePADR(&Packet{
		Code:   CodePADR,
		SrcMAC: srcMAC,
		Tags: []Tag{
			{Type: TagACCookie, Value: GenerateCookie(key, hwAddr[:], srcMAC[:], nil)},
			{Type: TagServiceName},
		},
	})
	return channel[1]
}

// socketPair answers a SOCK_SEQPACKET pair, so each PPP frame is one read.
// The driver owns and closes the first end once handed over; the test closes
// the second.
func socketPair(t *testing.T) [2]int {
	t.Helper()
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_SEQPACKET|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unix.Close(fds[1]) })
	return [2]int{fds[0], fds[1]}
}

// readLCP answers the next LCP packet with the given code the AC writes on
// fd, skipping any other frame, within five seconds.
func readLCP(t *testing.T, fd int, code uint8) ppp.LCPPacket {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	buf := make([]byte, 2048)
	for time.Now().Before(deadline) {
		pfd := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		if n, err := unix.Poll(pfd, 100); err != nil || n == 0 {
			continue
		}
		n, err := unix.Read(fd, buf)
		if err != nil {
			t.Fatalf("read PPP channel: %v", err)
		}
		proto, payload, _, err := ppp.ParseFrame(buf[:n])
		if err != nil || proto != ppp.ProtoLCP {
			continue
		}
		pkt, err := ppp.ParseLCPPacket(payload)
		if err != nil {
			t.Fatalf("AC wrote a malformed LCP packet: %v", err)
		}
		if pkt.Code == code {
			return pkt
		}
	}
	t.Fatalf("the AC wrote no LCP code %d within five seconds", code)
	return ppp.LCPPacket{}
}

// mruOf answers the MRU option in an LCP option list, and whether it is there.
func mruOf(t *testing.T, data []byte) (uint16, bool) {
	t.Helper()
	opts, err := ppp.ParseLCPOptions(data)
	if err != nil {
		t.Fatalf("LCP options: %v", err)
	}
	for _, opt := range opts {
		if opt.Type == ppp.LCPOptMRU && len(opt.Data) == 2 {
			return binary.BigEndian.Uint16(opt.Data), true
		}
	}
	return 0, false
}

// sendClientConfigureRequest writes a client Configure-Request carrying only
// an MRU option of mru.
func sendClientConfigureRequest(t *testing.T, fd int, id uint8, mru uint16) {
	t.Helper()
	opts := []byte{ppp.LCPOptMRU, 4, byte(mru >> 8), byte(mru)}
	lcp := make([]byte, 4+len(opts))
	n := ppp.WriteLCPPacket(lcp, 0, ppp.LCPConfigureRequest, id, opts)
	frame := make([]byte, 2+n)
	m := ppp.WriteFrame(frame, 0, ppp.ProtoLCP, lcp[:n])
	if _, err := unix.Write(fd, frame[:m]); err != nil {
		t.Fatalf("write client Configure-Request: %v", err)
	}
}

// RFC requirement: RFC2516-x-9 positive -- the Ze Access Concentrator, started
// by handlePADR, asks in its own LCP Configure-Request for an MRU no larger
// than 1492, and Configure-Acks a client Configure-Request naming MRU 1492.
func TestRFC2516AccessConcentratorNegotiatesMRUAtThePPPoECeiling(t *testing.T) {
	client := acPPPChannel(t)

	request := readLCP(t, client, ppp.LCPConfigureRequest)
	mru, ok := mruOf(t, request.Data)
	if !ok {
		t.Fatal("AC Configure-Request carries no MRU, so the 1500-octet default applies")
	}
	if mru > PPPoEMaxMTU {
		t.Fatalf("AC Configure-Request asks for MRU %d, above %d", mru, PPPoEMaxMTU)
	}

	sendClientConfigureRequest(t, client, 0x21, 1492)
	ack := readLCP(t, client, ppp.LCPConfigureAck)
	if ack.Identifier != 0x21 {
		t.Fatalf("Configure-Ack identifier %#x, want 0x21", ack.Identifier)
	}
	if got, ok := mruOf(t, ack.Data); !ok || got != 1492 {
		t.Fatalf("Configure-Ack MRU %d (present %t), want 1492", got, ok)
	}
}

// RFC requirement: RFC2516-x-9 negative -- a client Configure-Request naming
// MRU 1500 is never Acked by the Ze Access Concentrator: the answer is a
// Configure-Nak whose MRU suggestion is 1492.
func TestRFC2516AccessConcentratorRefusesAnMRUAboveThePPPoECeiling(t *testing.T) {
	client := acPPPChannel(t)
	readLCP(t, client, ppp.LCPConfigureRequest)

	sendClientConfigureRequest(t, client, 0x22, 1500)
	nak := readLCP(t, client, ppp.LCPConfigureNak)
	if nak.Identifier != 0x22 {
		t.Fatalf("Configure-Nak identifier %#x, want 0x22", nak.Identifier)
	}
	if got, ok := mruOf(t, nak.Data); !ok || got != PPPoEMaxMTU {
		t.Fatalf("Configure-Nak MRU %d (present %t), want %d", got, ok, PPPoEMaxMTU)
	}
}
