// Design: docs/research/l2tpv2-ze-integration.md -- IPv6 carrier cancellation and ownership

package kernel

import (
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/ipv6"
)

type carrierTestSocket struct {
	mu              sync.Mutex
	readStarted     chan struct{}
	readInterrupted chan struct{}
	startOnce       sync.Once
	interruptOnce   sync.Once
	closes          int
	closed          bool
	closeErr        error
	writes          int
	hopLimit        int
	peer            net.Addr
}

func (s *carrierTestSocket) read() (int, error) {
	s.startOnce.Do(func() { close(s.readStarted) })
	<-s.readInterrupted
	return 0, context.DeadlineExceeded
}

func (s *carrierTestSocket) SetReadDeadline(deadline time.Time) error {
	if !deadline.IsZero() {
		s.interruptOnce.Do(func() { close(s.readInterrupted) })
	}
	return nil
}

func (s *carrierTestSocket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closes++
	s.closed = true
	return s.closeErr
}

type carrierTestPacket struct {
	payload []byte
	control *ipv6.ControlMessage
	peer    net.Addr
}

type carrierTestRA struct {
	*carrierTestSocket
	packets []carrierTestPacket
	reads   int
}

func (s *carrierTestRA) ReadFrom(payload []byte) (int, *ipv6.ControlMessage, net.Addr, error) {
	if len(s.packets) > 0 {
		packet := s.packets[0]
		s.packets = s.packets[1:]
		s.reads++
		return copy(payload, packet.payload), packet.control, packet.peer, nil
	}
	n, err := s.read()
	return n, nil, nil, err
}

func (s *carrierTestRA) WriteTo(payload []byte, control *ipv6.ControlMessage, peer net.Addr) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, net.ErrClosed
	}
	s.writes++
	s.hopLimit = control.HopLimit
	s.peer = peer
	return len(payload), nil
}

type carrierTestDHCP struct {
	*carrierTestSocket
	packet *carrierTestPacket
}

func (s *carrierTestDHCP) ReadFrom(payload []byte) (int, net.Addr, error) {
	if s.packet != nil {
		return copy(payload, s.packet.payload), s.packet.peer, nil
	}
	n, err := s.read()
	return n, nil, err
}

func (s *carrierTestDHCP) WriteTo(payload []byte, peer net.Addr) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, net.ErrClosed
	}
	s.writes++
	s.peer = peer
	return len(payload), nil
}

func newCarrierTestSocket() *carrierTestSocket {
	return &carrierTestSocket{readStarted: make(chan struct{}), readInterrupted: make(chan struct{})}
}

// Goal: cancel either receive lane without closing the final-RA write path.
// Method: block the actual carrier read on a fake deadline-aware socket, then
// cancel and require both the context error and a successful RA write.
func TestIPv6CarrierCancellationPreservesFinalRA(t *testing.T) {
	for _, lane := range []string{"rs", "dhcpv6"} {
		t.Run(lane, func(t *testing.T) {
			ra := &carrierTestRA{carrierTestSocket: newCarrierTestSocket()}
			dhcp := &carrierTestDHCP{carrierTestSocket: newCarrierTestSocket()}
			carrier := newIPv6Carrier(ra, dhcp, "ppp7", 7)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			if lane == "rs" {
				go func() { done <- carrier.ReadRS(ctx) }()
				<-ra.readStarted
			} else {
				go func() { var payload [64]byte; _, _, err := carrier.ReadDHCPv6(ctx, payload[:]); done <- err }()
				<-dhcp.readStarted
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("read error = %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("cancel did not release carrier read")
			}
			if err := carrier.WriteRA([]byte{134, 0, 0, 0}, 255); err != nil {
				t.Fatal(err)
			}
			if ra.hopLimit != 255 {
				t.Fatalf("hop limit = %d", ra.hopLimit)
			}
			if ra.closes != 0 {
				t.Fatalf("RA closed before final advertisement: %d", ra.closes)
			}
			if err := carrier.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Goal: one carrier releases both owned sockets exactly once, preserving both
// cleanup failures. It owns no PPP endpoint and cannot close one.
func TestIPv6CarrierCloseOncePreservesFailures(t *testing.T) {
	ra := &carrierTestRA{carrierTestSocket: newCarrierTestSocket()}
	dhcp := &carrierTestDHCP{carrierTestSocket: newCarrierTestSocket()}
	ra.closeErr = io.ErrClosedPipe
	dhcp.closeErr = io.ErrUnexpectedEOF
	carrier := newIPv6Carrier(ra, dhcp, "ppp7", 7)
	for range 2 {
		err := carrier.Close()
		if !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("lost RA close failure: %v", err)
		}
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("lost DHCP close failure: %v", err)
		}
	}
	if ra.closes != 1 {
		t.Fatalf("RA closes = %d", ra.closes)
	}
	if dhcp.closes != 1 {
		t.Fatalf("DHCP closes = %d", dhcp.closes)
	}
}

// Goal: preserve DHCPv6's scoped unicast reply address at the carrier boundary.
func TestIPv6CarrierDHCPReplyAddress(t *testing.T) {
	ra := &carrierTestRA{carrierTestSocket: newCarrierTestSocket()}
	dhcp := &carrierTestDHCP{carrierTestSocket: newCarrierTestSocket()}
	carrier := newIPv6Carrier(ra, dhcp, "ppp7", 7)
	peer := netip.MustParseAddrPort("[fe80::2%ppp7]:546")
	n, err := carrier.WriteDHCPv6([]byte{1, 2, 3, 4}, peer)
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Fatalf("written = %d", n)
	}
	address, ok := dhcp.peer.(*net.UDPAddr)
	if !ok {
		t.Fatalf("peer type = %T", dhcp.peer)
	}
	if address.AddrPort() != peer {
		t.Fatalf("reply peer = %v", address)
	}
}

// Goal: the session carrier never sends a scoped DHCPv6 reply on a different
// interface, even if a caller supplies a syntactically valid foreign zone.
func TestIPv6CarrierRejectsCrossInterfaceDHCPReply(t *testing.T) {
	ra := &carrierTestRA{carrierTestSocket: newCarrierTestSocket()}
	dhcp := &carrierTestDHCP{carrierTestSocket: newCarrierTestSocket()}
	carrier := newIPv6Carrier(ra, dhcp, "ppp7", 7)
	if _, err := carrier.WriteDHCPv6([]byte{1, 2, 3, 4}, netip.MustParseAddrPort("[fe80::2%ppp8]:546")); err == nil {
		t.Fatal("foreign interface accepted for session reply")
	}
	if dhcp.writes != 0 {
		t.Fatal("foreign reply reached socket")
	}
}

// Goal: actual receive addresses stay IPv6 and session-scoped; a missing
// link-local zone is supplied by the carrier's known interface binding.
func TestIPv6CarrierDHCPPeerAdmission(t *testing.T) {
	for _, tc := range []struct {
		name string
		peer net.Addr
		want netip.AddrPort
	}{
		{"scoped", net.UDPAddrFromAddrPort(netip.MustParseAddrPort("[fe80::2%ppp7]:546")), netip.MustParseAddrPort("[fe80::2%ppp7]:546")},
		{"binding supplies zone", net.UDPAddrFromAddrPort(netip.MustParseAddrPort("[fe80::2]:546")), netip.MustParseAddrPort("[fe80::2%ppp7]:546")},
		{"global", net.UDPAddrFromAddrPort(netip.MustParseAddrPort("[2001:db8::2]:546")), netip.MustParseAddrPort("[2001:db8::2]:546")},
		{"foreign zone", net.UDPAddrFromAddrPort(netip.MustParseAddrPort("[fe80::2%ppp8]:546")), netip.AddrPort{}},
		{"IPv4", net.UDPAddrFromAddrPort(netip.MustParseAddrPort("192.0.2.2:546")), netip.AddrPort{}},
		{"not UDP", &net.IPAddr{IP: net.ParseIP("2001:db8::2")}, netip.AddrPort{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ra := &carrierTestRA{carrierTestSocket: newCarrierTestSocket()}
			dhcp := &carrierTestDHCP{carrierTestSocket: newCarrierTestSocket(), packet: &carrierTestPacket{payload: []byte{1, 2, 3, 4}, peer: tc.peer}}
			carrier := newIPv6Carrier(ra, dhcp, "ppp7", 7)
			var payload [64]byte
			n, peer, err := carrier.ReadDHCPv6(t.Context(), payload[:])
			if !tc.want.IsValid() {
				if err == nil {
					t.Fatal("invalid receive peer accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if n != 4 {
				t.Fatalf("payload length = %d", n)
			}
			if peer != tc.want {
				t.Fatalf("peer = %v, want %v", peer, tc.want)
			}
		})
	}
}

// Goal: only a valid solicitation on this carrier's actual interface reaches
// the scheduler. Each rejected input is followed by a valid packet, so a
// receiver that discards everything cannot satisfy the test.
func TestIPv6CarrierSolicitationAdmission(t *testing.T) {
	valid := carrierTestPacket{
		payload: []byte{133, 0, 0, 0, 0, 0, 0, 0},
		control: &ipv6.ControlMessage{IfIndex: 7, HopLimit: 255},
		peer:    &net.IPAddr{IP: net.ParseIP("2001:db8::2")},
	}
	for _, tc := range []struct {
		name   string
		packet carrierTestPacket
	}{
		{"missing metadata", carrierTestPacket{valid.payload, nil, valid.peer}},
		{"wrong interface", carrierTestPacket{valid.payload, &ipv6.ControlMessage{IfIndex: 8, HopLimit: 255}, valid.peer}},
		{"wrong hop limit", carrierTestPacket{valid.payload, &ipv6.ControlMessage{IfIndex: 7, HopLimit: 254}, valid.peer}},
		{"wrong source zone", carrierTestPacket{valid.payload, valid.control, &net.IPAddr{IP: net.ParseIP("fe80::2"), Zone: "ppp8"}}},
		{"not solicitation", carrierTestPacket{[]byte{134, 0, 0, 0, 0, 0, 0, 0}, valid.control, valid.peer}},
		{"short body", carrierTestPacket{valid.payload[:7], valid.control, valid.peer}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ra := &carrierTestRA{carrierTestSocket: newCarrierTestSocket(), packets: []carrierTestPacket{tc.packet, valid}}
			dhcp := &carrierTestDHCP{carrierTestSocket: newCarrierTestSocket()}
			carrier := newIPv6Carrier(ra, dhcp, "ppp7", 7)
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			if err := carrier.ReadRS(ctx); err != nil {
				t.Fatal(err)
			}
			if ra.reads != 2 {
				t.Fatalf("read count = %d, invalid packet reached scheduler", ra.reads)
			}
		})
	}
}

// Goal: construction transfers ownership only on success and unwinds the RA
// socket if DHCPv6 cannot be acquired, preserving the primary and close errors.
func TestIPv6CarrierAcquisitionUnwindsPartialSockets(t *testing.T) {
	t.Run("RA refusal", func(t *testing.T) {
		dhcpCalls := 0
		carrier, err := acquireIPv6Carrier(t.Context(), "ppp7", 7,
			func(context.Context) (raSocket, error) { return nil, io.ErrClosedPipe },
			func(context.Context) (dhcpSocket, error) { dhcpCalls++; return nil, io.ErrUnexpectedEOF })
		if carrier != nil {
			t.Fatal("failed acquisition returned carrier")
		}
		if !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("setup error = %v", err)
		}
		if dhcpCalls != 0 {
			t.Fatal("DHCP opened after RA refusal")
		}
	})
	t.Run("DHCP refusal", func(t *testing.T) {
		ra := &carrierTestRA{carrierTestSocket: newCarrierTestSocket()}
		ra.closeErr = io.ErrUnexpectedEOF
		carrier, err := acquireIPv6Carrier(t.Context(), "ppp7", 7,
			func(context.Context) (raSocket, error) { return ra, nil },
			func(context.Context) (dhcpSocket, error) { return nil, io.ErrClosedPipe })
		if carrier == nil {
			t.Fatal("failed RA close lost its exact partial cleanup owner")
		}
		if err := carrier.ReadRS(t.Context()); err == nil {
			t.Fatal("partial cleanup owner was usable as an acquired carrier")
		}
		if !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("lost setup error: %v", err)
		}
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("lost RA close error: %v", err)
		}
		if ra.closes != 1 {
			t.Fatalf("RA close count = %d", ra.closes)
		}
	})
	t.Run("success", func(t *testing.T) {
		ra := &carrierTestRA{carrierTestSocket: newCarrierTestSocket()}
		dhcp := &carrierTestDHCP{carrierTestSocket: newCarrierTestSocket()}
		carrier, err := acquireIPv6Carrier(t.Context(), "ppp7", 7,
			func(context.Context) (raSocket, error) { return ra, nil },
			func(context.Context) (dhcpSocket, error) { return dhcp, nil })
		if err != nil {
			t.Fatal(err)
		}
		if carrier == nil {
			t.Fatal("successful acquisition returned no carrier")
		}
		if ra.closes != 0 {
			t.Fatal("successful acquisition closed RA socket")
		}
		if dhcp.closes != 0 {
			t.Fatal("successful acquisition closed DHCP socket")
		}
		if err := carrier.Close(); err != nil {
			t.Fatal(err)
		}
	})
}
