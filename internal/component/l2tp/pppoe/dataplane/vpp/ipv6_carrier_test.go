// Design: docs/architecture/l2tp/bng-5-pppoe.md -- VPP IPv6 control wire boundary
// VALIDATES: independent IPv6/ICMPv6/UDP vectors, bounded queues and borrowed ownership.
package vpp

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"io"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// Literal checksums cover the pseudoheader and odd/even transport lengths;
// these inputs never pass through the production encoder before admission.
const ipv6RSVector = "00576000000000083afffe800000000000000000000000000002ff02000000000000000000000000000285007d3500000000"
const ipv6DHCPVector = "005760000000000c1101fe800000000000000000000000000002ff02000000000000000000000001000202220223000cfd0701000001"

type ipv6Endpoint struct {
	frames [][]byte
	writes [][]byte
	closed bool
}

func (e *ipv6Endpoint) Read(buf []byte) (int, error) {
	if len(e.frames) == 0 {
		return 0, io.EOF
	}
	frame := e.frames[0]
	e.frames = e.frames[1:]
	return copy(buf, frame), nil
}
func (e *ipv6Endpoint) Write(buf []byte) (int, error) {
	if e.closed {
		return 0, io.ErrClosedPipe
	}
	e.writes = append(e.writes, bytes.Clone(buf))
	return len(buf), nil
}
func (e *ipv6Endpoint) SetReadDeadline(time.Time) error { return nil }
func (e *ipv6Endpoint) Close() error                    { e.closed = true; return nil }

func ipv6Vector(t *testing.T, wire string) []byte {
	t.Helper()
	frame, err := hex.DecodeString(wire)
	if err != nil {
		t.Fatal(err)
	}
	return frame
}

func ipv6Fixture(t *testing.T) (*controlSocket, *ipv6Carrier, *ipv6Endpoint) {
	t.Helper()
	endpoint := &ipv6Endpoint{}
	socket := newControlSocket(endpoint)
	carrier, err := socket.openIPv6Carrier(netip.MustParseAddr("fe80::1"), netip.MustParseAddr("fe80::2"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := socket.Close(); err != nil {
			t.Error(err)
		}
	})
	return socket, carrier, endpoint
}

// One existing PPP reader must dispatch network controls and keep the next
// LCP frame intact; consumers receive only admitted transport payloads.
func TestVPPIPv6CarrierDemultiplexesLiteralWire(t *testing.T) {
	socket, carrier, endpoint := ipv6Fixture(t)
	lcp := []byte{0xc0, 0x21, 9, 1, 0, 4}
	endpoint.frames = [][]byte{ipv6Vector(t, ipv6RSVector), ipv6Vector(t, ipv6DHCPVector), lcp}
	var frame [ppp.MaxFrameBufLen]byte
	n, err := socket.Read(frame[:])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(frame[:n], lcp) {
		t.Fatalf("control frame %x", frame[:n])
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := carrier.ReadRS(ctx); err != nil {
		t.Fatal(err)
	}
	var payload [16]byte
	n, peer, err := carrier.ReadDHCPv6(ctx, payload[:])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(payload[:n], []byte{1, 0, 0, 1}) {
		t.Fatalf("DHCPv6 %x", payload[:n])
	}
	if peer != netip.MustParseAddrPort("[fe80::2]:546") {
		t.Fatalf("peer %v", peer)
	}
}

// Corrupt envelopes must never signal either worker. The next control frame
// proves a discarded datagram does not stop PPP's sole reader.
func TestVPPIPv6CarrierRejectsInvalidWire(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"version", func(b []byte) []byte { b[2] = 0x40; return b }},
		{"declared-length", func(b []byte) []byte { b[7]++; return b }},
		{"checksum", func(b []byte) []byte { b[len(b)-1]++; return b }},
		{"hop-limit", func(b []byte) []byte { b[9] = 254; return b }},
		{"non-peer-source", func(b []byte) []byte { b[25] = 3; return b }},
		{"wrong-destination", func(b []byte) []byte { b[41] = 3; return b }},
		{"extension-header", func(b []byte) []byte { b[8] = 0; return b }},
		{"short-header", func(b []byte) []byte { return b[:39] }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			socket, carrier, endpoint := ipv6Fixture(t)
			endpoint.frames = [][]byte{test.mutate(ipv6Vector(t, ipv6RSVector)), {0xc0, 0x21}}
			var frame [ppp.MaxFrameBufLen]byte
			if _, err := socket.Read(frame[:]); err != nil {
				t.Fatal(err)
			}
			if len(carrier.rs) != 0 {
				t.Fatal("invalid RS queued")
			}
			if socket.ipv6DiscardCount() != 1 {
				t.Fatal("missing discard reason")
			}
		})
	}
}

// A carrier closes its own reads, while cancellation preserves writes for the
// final RA; its borrowed endpoint remains owned by the session socket.
func TestVPPIPv6CarrierCancellationAndFinalRA(t *testing.T) {
	_, carrier, endpoint := ipv6Fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := carrier.ReadRS(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("read: %v", err)
	}
	ra := ipv6Vector(t, "86000000400007080000000000000000")
	if err := carrier.WriteRA(ra, 255); err != nil {
		t.Fatal(err)
	}
	expected := ipv6Vector(t, "00576000000000103afffe800000000000000000000000000001ff02000000000000000000000000000186003527400007080000000000000000")
	if !bytes.Equal(endpoint.writes[0], expected) {
		t.Fatalf("RA %x, want %x", endpoint.writes[0], expected)
	}
	if err := carrier.Close(); err != nil {
		t.Fatal(err)
	}
	if err := carrier.Close(); err != nil {
		t.Fatal(err)
	}
	if endpoint.closed {
		t.Fatal("carrier closed borrowed endpoint")
	}
	if err := carrier.ReadRS(context.Background()); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("closed read: %v", err)
	}
	if err := carrier.WriteRA(ra, 255); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("closed write: %v", err)
	}
}

// The common service publishes IPv6CP's pair before opening, so opening uses the
// pair recorded by AddAddressP2P, never an Ethernet-derived identifier.
func TestVPPIPv6CarrierRequiresNegotiatedPair(t *testing.T) {
	s := &session{socket: newControlSocket(&ipv6Endpoint{}), addresses: make(map[netip.Prefix]sessionRoute)}
	if _, err := s.OpenIPv6Carrier(context.Background()); err == nil {
		t.Fatal("missing pair accepted")
	}
	s.addresses[netip.MustParsePrefix("169.254.0.1/16")] = sessionRoute{destination: netip.MustParsePrefix("169.254.0.2/32")}
	if _, err := s.OpenIPv6Carrier(context.Background()); err == nil {
		t.Fatal("IPv4 link-local pair accepted")
	}
	s.addresses[netip.MustParsePrefix("fe80::9/64")] = sessionRoute{destination: netip.MustParsePrefix("fe80::a/128")}
	s.routes = map[sessionRoute]struct{}{s.addresses[netip.MustParsePrefix("fe80::9/64")]: {}}
	carrier, err := s.OpenIPv6Carrier(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.OpenIPv6Carrier(context.Background()); err == nil {
		t.Fatal("duplicate carrier accepted")
	}
	if err := carrier.Close(); err != nil {
		t.Fatal(err)
	}
}

// The response vector independently fixes UDP ports, source, target, length
// and the pseudoheader checksum, including the padded final odd octet.
func TestVPPIPv6CarrierWritesLiteralDHCPv6(t *testing.T) {
	_, carrier, endpoint := ipv6Fixture(t)
	payload := []byte{7, 0, 0, 1, 255}
	n, err := carrier.WriteDHCPv6(payload, netip.MustParseAddrPort("[fe80::2]:546"))
	if err != nil {
		t.Fatal(err)
	}
	if n != len(payload) {
		t.Fatalf("write count %d", n)
	}
	expected := ipv6Vector(t, "005760000000000d1101fe800000000000000000000000000001fe80000000000000000000000000000202230222000df88807000001ff")
	if !bytes.Equal(endpoint.writes[0], expected) {
		t.Fatalf("DHCPv6 %x, want %x", endpoint.writes[0], expected)
	}
	if _, err := carrier.WriteDHCPv6(payload, netip.MustParseAddrPort("[ff02::1]:546")); err == nil {
		t.Fatal("multicast reply target accepted")
	}
	if err := carrier.WriteRA(make([]byte, 16), 64); err == nil {
		t.Fatal("invalid RA accepted")
	}
}

// UDP admission enforces each boundary before queueing; corruption cannot
// turn a truncated or foreign datagram into a DHCPv6 request.
func TestVPPIPv6CarrierRejectsInvalidDHCPv6(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"source-port", func(b []byte) []byte { b[43] = 35; return b }},
		{"target-port", func(b []byte) []byte { b[45] = 34; return b }},
		{"udp-length", func(b []byte) []byte { b[47]--; return b }},
		{"zero-checksum", func(b []byte) []byte { b[48], b[49] = 0, 0; return b }},
		{"checksum", func(b []byte) []byte { b[53]++; return b }},
		{"hop-limit", func(b []byte) []byte { b[9] = 0; return b }},
		{"short", func(b []byte) []byte { return b[:49] }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			socket, carrier, endpoint := ipv6Fixture(t)
			endpoint.frames = [][]byte{test.mutate(ipv6Vector(t, ipv6DHCPVector)), {0xc0, 0x21}}
			var frame [ppp.MaxFrameBufLen]byte
			if _, err := socket.Read(frame[:]); err != nil {
				t.Fatal(err)
			}
			if len(carrier.dhcp) != 0 {
				t.Fatal("invalid UDP queued")
			}
			if socket.ipv6DiscardCount() != 1 {
				t.Fatal("missing discard reason")
			}
		})
	}
}

// A full queue drops the newest event without blocking the PPP reader. A
// closed NCP discards network data without manufacturing a reject or metric.
func TestVPPIPv6CarrierBoundsQueuesAndClosedNCP(t *testing.T) {
	socket, carrier, endpoint := ipv6Fixture(t)
	for range ipv6ControlQueueDepth + 1 {
		endpoint.frames = append(endpoint.frames, ipv6Vector(t, ipv6RSVector))
	}
	endpoint.frames = append(endpoint.frames, []byte{0xc0, 0x21})
	var frame [ppp.MaxFrameBufLen]byte
	if _, err := socket.Read(frame[:]); err != nil {
		t.Fatal(err)
	}
	if len(carrier.rs) != ipv6ControlQueueDepth {
		t.Fatal("queue depth differs")
	}
	if socket.ipv6DiscardCount() != 1 {
		t.Fatal("full queue not counted")
	}
	if err := carrier.Close(); err != nil {
		t.Fatal(err)
	}
	endpoint.frames = [][]byte{ipv6Vector(t, ipv6RSVector), {0xc0, 0x21}}
	if _, err := socket.Read(frame[:]); err != nil {
		t.Fatal(err)
	}
	if socket.ipv6DiscardCount() != 1 {
		t.Fatal("closed NCP produced a discard metric")
	}
}

// Closing a carrier may overlap dispatch and both blocking readers; queues
// remain open while a separate broadcast wakes every outstanding receive.
func TestVPPIPv6CarrierCloseRacesDispatch(t *testing.T) {
	socket, carrier, _ := ipv6Fixture(t)
	packet := ipv6Vector(t, ipv6RSVector)[2:]
	var workers sync.WaitGroup
	workers.Add(3)
	go func() {
		defer workers.Done()
		for range 64 {
			socket.dispatchIPv6(packet)
		}
	}()
	go func() {
		defer workers.Done()
		var buf [16]byte
		_, _, err := carrier.ReadDHCPv6(context.Background(), buf[:])
		if !errors.Is(err, io.ErrClosedPipe) {
			t.Errorf("DHCPv6 read: %v", err)
		}
	}()
	go func() {
		defer workers.Done()
		for range 64 {
			if err := carrier.ReadRS(context.Background()); err != nil {
				if !errors.Is(err, io.ErrClosedPipe) {
					t.Errorf("RS read: %v", err)
				}
				return
			}
		}
	}()
	if err := carrier.Close(); err != nil {
		t.Fatal(err)
	}
	workers.Wait()
}

// Arbitrary full PPP IPv6 information fields must remain bounded and never
// panic; seeds exercise independent valid packets before mutations.
func FuzzVPPIPv6ControlAdmission(f *testing.F) {
	for _, wire := range []string{ipv6RSVector, ipv6DHCPVector} {
		packet, err := hex.DecodeString(wire)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(packet[2:])
	}
	f.Fuzz(func(t *testing.T, packet []byte) {
		socket, _, _ := ipv6Fixture(t)
		socket.dispatchIPv6(packet)
	})
}
