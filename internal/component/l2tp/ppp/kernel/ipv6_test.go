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
	controlIfIndex  int
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
	s.controlIfIndex = control.IfIndex
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
			carrier := newIPv6Carrier(ra, dhcp, 7)
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
	carrier := newIPv6Carrier(ra, dhcp, 7)
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

// Goal: a DHCPv6 reply reaches the socket unzoned, so the kernel's binding to
// the original unit's device, not a name lookup, selects where it goes.
func TestIPv6CarrierDHCPReplyAddress(t *testing.T) {
	for _, address := range []string{"[fe80::2]:546", "[2001:db8::2]:546"} {
		t.Run(address, func(t *testing.T) {
			ra := &carrierTestRA{carrierTestSocket: newCarrierTestSocket()}
			dhcp := &carrierTestDHCP{carrierTestSocket: newCarrierTestSocket()}
			carrier := newIPv6Carrier(ra, dhcp, 7)
			peer := netip.MustParseAddrPort(address)
			n, err := carrier.WriteDHCPv6([]byte{1, 2, 3, 4}, peer)
			if err != nil {
				t.Fatal(err)
			}
			if n != 4 {
				t.Fatalf("written = %d", n)
			}
			written, ok := dhcp.peer.(*net.UDPAddr)
			if !ok {
				t.Fatalf("peer type = %T", dhcp.peer)
			}
			if written.Zone != "" {
				t.Fatalf("reply peer carries zone %q", written.Zone)
			}
			if written.AddrPort() != peer {
				t.Fatalf("reply peer = %v", written)
			}
		})
	}
}

// Goal: the session carrier never resolves a reply's interface by name. Any
// zone is refused before the socket, the carrier's own former name and a
// numeric zone included, because the name may now belong to another device.
func TestIPv6CarrierRejectsZonedDHCPReply(t *testing.T) {
	for _, zone := range []string{"ppp7", "ppp8", "7"} {
		t.Run(zone, func(t *testing.T) {
			ra := &carrierTestRA{carrierTestSocket: newCarrierTestSocket()}
			dhcp := &carrierTestDHCP{carrierTestSocket: newCarrierTestSocket()}
			carrier := newIPv6Carrier(ra, dhcp, 7)
			peer := netip.AddrPortFrom(netip.MustParseAddr("fe80::2").WithZone(zone), 546)
			if _, err := carrier.WriteDHCPv6([]byte{1, 2, 3, 4}, peer); err == nil {
				t.Fatal("zoned peer accepted for session reply")
			}
			if dhcp.writes != 0 {
				t.Fatal("zoned reply reached socket")
			}
		})
	}
}

// Goal: actual receive addresses stay IPv6 and leave the carrier unzoned. The
// zone Go derives from the receive scope is a current interface name, never
// authority: the kernel delivers only the original device's traffic. Every
// admitted peer must also be a reply address WriteDHCPv6 accepts unchanged.
func TestIPv6CarrierDHCPPeerAdmission(t *testing.T) {
	for _, tc := range []struct {
		name string
		peer net.Addr
		want netip.AddrPort
	}{
		{"scope named by original name", net.UDPAddrFromAddrPort(netip.MustParseAddrPort("[fe80::2%ppp7]:546")), netip.MustParseAddrPort("[fe80::2]:546")},
		{"scope named after rename", net.UDPAddrFromAddrPort(netip.MustParseAddrPort("[fe80::2%ppp8]:546")), netip.MustParseAddrPort("[fe80::2]:546")},
		{"unscoped link-local", net.UDPAddrFromAddrPort(netip.MustParseAddrPort("[fe80::2]:546")), netip.MustParseAddrPort("[fe80::2]:546")},
		{"global", net.UDPAddrFromAddrPort(netip.MustParseAddrPort("[2001:db8::2]:546")), netip.MustParseAddrPort("[2001:db8::2]:546")},
		{"IPv4", net.UDPAddrFromAddrPort(netip.MustParseAddrPort("192.0.2.2:546")), netip.AddrPort{}},
		{"not UDP", &net.IPAddr{IP: net.ParseIP("2001:db8::2")}, netip.AddrPort{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ra := &carrierTestRA{carrierTestSocket: newCarrierTestSocket()}
			dhcp := &carrierTestDHCP{carrierTestSocket: newCarrierTestSocket(), packet: &carrierTestPacket{payload: []byte{1, 2, 3, 4}, peer: tc.peer}}
			carrier := newIPv6Carrier(ra, dhcp, 7)
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
			if _, err := carrier.WriteDHCPv6(payload[:n], peer); err != nil {
				t.Fatalf("received peer refused as reply address: %v", err)
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
		{"wrong interface under original name", carrierTestPacket{valid.payload, &ipv6.ControlMessage{IfIndex: 8, HopLimit: 255}, &net.IPAddr{IP: net.ParseIP("fe80::2"), Zone: "ppp7"}}},
		{"wrong hop limit", carrierTestPacket{valid.payload, &ipv6.ControlMessage{IfIndex: 7, HopLimit: 254}, valid.peer}},
		{"not solicitation", carrierTestPacket{[]byte{134, 0, 0, 0, 0, 0, 0, 0}, valid.control, valid.peer}},
		{"short body", carrierTestPacket{valid.payload[:7], valid.control, valid.peer}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ra := &carrierTestRA{carrierTestSocket: newCarrierTestSocket(), packets: []carrierTestPacket{tc.packet, valid}}
			dhcp := &carrierTestDHCP{carrierTestSocket: newCarrierTestSocket()}
			carrier := newIPv6Carrier(ra, dhcp, 7)
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

// Goal: a solicitation from the original device is admitted whatever name its
// source zone carries, because the kernel's numeric receive index decides and
// a renamed original keeps that index.
func TestIPv6CarrierSolicitationIgnoresZoneName(t *testing.T) {
	renamed := carrierTestPacket{
		payload: []byte{133, 0, 0, 0, 0, 0, 0, 0},
		control: &ipv6.ControlMessage{IfIndex: 7, HopLimit: 255},
		peer:    &net.IPAddr{IP: net.ParseIP("fe80::2"), Zone: "ppp8"},
	}
	ra := &carrierTestRA{carrierTestSocket: newCarrierTestSocket(), packets: []carrierTestPacket{renamed}}
	dhcp := &carrierTestDHCP{carrierTestSocket: newCarrierTestSocket()}
	carrier := newIPv6Carrier(ra, dhcp, 7)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	if err := carrier.ReadRS(ctx); err != nil {
		t.Fatal(err)
	}
	if ra.reads != 1 {
		t.Fatalf("read count = %d, original device's solicitation was discarded", ra.reads)
	}
}

// Goal: a Router Advertisement names neither a zone nor an interface index, so
// the socket's kernel binding to the original device selects where it goes.
func TestIPv6CarrierRAUsesKernelBinding(t *testing.T) {
	ra := &carrierTestRA{carrierTestSocket: newCarrierTestSocket()}
	dhcp := &carrierTestDHCP{carrierTestSocket: newCarrierTestSocket()}
	carrier := newIPv6Carrier(ra, dhcp, 7)
	if err := carrier.WriteRA([]byte{134, 0, 0, 0}, 255); err != nil {
		t.Fatal(err)
	}
	peer, ok := ra.peer.(*net.IPAddr)
	if !ok {
		t.Fatalf("peer type = %T", ra.peer)
	}
	if !peer.IP.Equal(net.IPv6linklocalallnodes) {
		t.Fatalf("advertisement destination = %v", peer.IP)
	}
	if peer.Zone != "" {
		t.Fatalf("advertisement carries zone %q", peer.Zone)
	}
	if ra.controlIfIndex != 0 {
		t.Fatalf("advertisement names interface index %d", ra.controlIfIndex)
	}
	if ra.hopLimit != 255 {
		t.Fatalf("hop limit = %d", ra.hopLimit)
	}
}

// Goal: construction transfers ownership only on success and unwinds the RA
// socket if DHCPv6 cannot be acquired, preserving the primary and close errors.
func TestIPv6CarrierAcquisitionUnwindsPartialSockets(t *testing.T) {
	t.Run("RA refusal", func(t *testing.T) {
		dhcpCalls := 0
		carrier, err := acquireIPv6Carrier(t.Context(), 7,
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
		carrier, err := acquireIPv6Carrier(t.Context(), 7,
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
		carrier, err := acquireIPv6Carrier(t.Context(), 9,
			func(context.Context) (raSocket, error) { return ra, nil },
			func(context.Context) (dhcpSocket, error) { return dhcp, nil })
		if err != nil {
			t.Fatal(err)
		}
		acquired, ok := carrier.(*ipv6Carrier)
		if !ok {
			t.Fatalf("successful acquisition returned %T", carrier)
		}
		if acquired.ifindex != 9 {
			t.Fatalf("acquired carrier filters index %d, want the captured 9", acquired.ifindex)
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
