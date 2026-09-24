//go:build linux

// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- destination capture and response source selection
// Related: udp.go -- Run and SendFrom
package transport

import (
	"bytes"
	"errors"
	"log/slog"
	"net"
	"testing"
	"time"
)

// TestUDPTransportWildcardResponseSource sends requests to two loopback
// destinations on one wildcard socket. Each delivered packet retains its own
// destination, bound port and NAT-T role, and its response leaves from that
// endpoint rather than the kernel's default loopback source.
func TestUDPTransportWildcardResponseSource(t *testing.T) {
	for _, tc := range []struct {
		name string
		open func(string, *slog.Logger) (*UDPTransport, error)
		natt bool
	}{
		{name: "ike", open: NewUDPTransport},
		{name: "natt", open: NewNATTTransport, natt: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr, err := tc.open("0.0.0.0:0", slog.Default())
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			done := make(chan struct{})
			go func() {
				defer close(done)
				tr.Run()
			}()
			t.Cleanup(func() {
				if err := tr.Close(); err != nil {
					t.Errorf("Close: %v", err)
				}
				select {
				case <-done:
				case <-time.After(prtArrive):
					t.Error("Run did not stop after Close")
				}
			})
			bound, ok := tr.LocalAddr().(*net.UDPAddr)
			if !ok {
				t.Fatal("LocalAddr is not *net.UDPAddr")
			}
			peer, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
			if err != nil {
				t.Fatalf("ListenUDP: %v", err)
			}
			t.Cleanup(func() { _ = peer.Close() })
			peerAddr, ok := peer.LocalAddr().(*net.UDPAddr)
			if !ok {
				t.Fatal("peer address is not *net.UDPAddr")
			}
			destinations := []net.IP{net.IPv4(127, 0, 0, 2), net.IPv4(127, 0, 0, 3)}
			for i, ip := range destinations {
				request := prtIKEDatagram(byte(i + 1))
				if _, err := peer.WriteToUDP(request, &net.UDPAddr{IP: ip, Port: bound.Port}); err != nil {
					t.Fatalf("request to %s: %v", ip, err)
				}
			}
			var packets [2]Packet
			for i := range packets {
				select {
				case packets[i] = <-tr.Recv():
				case <-time.After(prtArrive):
					t.Fatalf("request %d did not arrive", i)
				}
			}
			for i, pkt := range packets {
				if pkt.LocalAddr == nil {
					t.Fatal("packet has no destination")
				}
				if !pkt.LocalAddr.IP.Equal(destinations[i]) {
					t.Fatalf("destination = %v, want %v", pkt.LocalAddr, destinations[i])
				}
				if pkt.LocalAddr.Port != bound.Port {
					t.Fatalf("destination port = %d, want %d", pkt.LocalAddr.Port, bound.Port)
				}
				if pkt.NATT != tc.natt {
					t.Fatalf("NATT = %v, want %v", pkt.NATT, tc.natt)
				}
				if pkt.RemoteAddr == nil {
					t.Fatal("packet has no source")
				}
				if !pkt.RemoteAddr.IP.Equal(peerAddr.IP) || pkt.RemoteAddr.Port != peerAddr.Port || pkt.RemoteAddr.Zone != peerAddr.Zone {
					t.Fatalf("remote = %v, want %v", pkt.RemoteAddr, peerAddr)
				}
				if !bytes.Equal(pkt.Data, prtIKEDatagram(byte(i+1))) {
					t.Fatalf("request %d changed in transit: %x", i, pkt.Data)
				}
				if err := tr.SendFrom(pkt.Data, pkt.LocalAddr, pkt.RemoteAddr); err != nil {
					t.Fatalf("SendFrom: %v", err)
				}
				if err := peer.SetReadDeadline(time.Now().Add(prtArrive)); err != nil {
					t.Fatalf("SetReadDeadline: %v", err)
				}
				var response [MaxMsgSize]byte
				n, source, err := peer.ReadFromUDP(response[:])
				if err != nil {
					t.Fatalf("read response: %v", err)
				}
				if !source.IP.Equal(pkt.LocalAddr.IP) || source.Port != pkt.LocalAddr.Port || source.Zone != pkt.LocalAddr.Zone {
					t.Fatalf("response source = %v, want %v", source, pkt.LocalAddr)
				}
				if !bytes.Equal(response[:n], pkt.Data) {
					t.Fatalf("response = %x, want %x", response[:n], pkt.Data)
				}
			}

			// Per-datagram selection must not change a later route-selected send.
			want := prtIKEDatagram(0x7f)
			if err := tr.SendFrom(want, nil, peerAddr); err != nil {
				t.Fatalf("route-selected SendFrom: %v", err)
			}
			if err := peer.SetReadDeadline(time.Now().Add(prtArrive)); err != nil {
				t.Fatalf("SetReadDeadline: %v", err)
			}
			var response [MaxMsgSize]byte
			n, source, err := peer.ReadFromUDP(response[:])
			if err != nil {
				t.Fatalf("read route-selected response: %v", err)
			}
			if !source.IP.Equal(peerAddr.IP) {
				t.Fatalf("route-selected source = %v, want %v", source.IP, peerAddr.IP)
			}
			if source.Port != bound.Port {
				t.Fatalf("route-selected port = %d, want %d", source.Port, bound.Port)
			}
			if !bytes.Equal(response[:n], want) {
				t.Fatalf("route-selected response = %x, want %x", response[:n], want)
			}
		})
	}
}

// TestUDPTransportSendFromRejectsInvalidLocal sends invalid source selections
// before a valid barrier datagram. The peer must receive only the barrier, so an
// error cannot hide a send from a substituted source address or port.
func TestUDPTransportSendFromRejectsInvalidLocal(t *testing.T) {
	tr, err := NewUDPTransport("0.0.0.0:0", slog.Default())
	if err != nil {
		t.Fatalf("NewUDPTransport: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })
	bound, ok := tr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("LocalAddr is not *net.UDPAddr")
	}
	peer, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	t.Cleanup(func() { _ = peer.Close() })
	remote, ok := peer.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("peer address is not *net.UDPAddr")
	}
	wrongPort := bound.Port - 1
	for _, tc := range []struct {
		name  string
		local net.UDPAddr
	}{
		{name: "different-port", local: net.UDPAddr{IP: remote.IP, Port: wrongPort}},
		{name: "zero-port", local: net.UDPAddr{IP: remote.IP}},
		{name: "missing-ip", local: net.UDPAddr{Port: bound.Port}},
		{name: "wildcard", local: net.UDPAddr{IP: net.IPv4zero, Port: bound.Port}},
		{name: "ipv6", local: net.UDPAddr{IP: net.IPv6loopback, Port: bound.Port}},
		{name: "multicast", local: net.UDPAddr{IP: net.IPv4(224, 0, 0, 1), Port: bound.Port}},
		{name: "broadcast", local: net.UDPAddr{IP: net.IPv4bcast, Port: bound.Port}},
		{name: "zone", local: net.UDPAddr{IP: remote.IP, Port: bound.Port, Zone: "lo"}},
		{name: "nonlocal", local: net.UDPAddr{IP: net.IPv4(203, 0, 113, 73), Port: bound.Port}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tr.SendFrom(prtIKEDatagram(0xad), &tc.local, remote); !errors.Is(err, ErrSendFailed) {
				t.Fatalf("SendFrom(%v) = %v, want ErrSendFailed", tc.local, err)
			}
		})
	}
	want := prtIKEDatagram(0x6a)
	local := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 2), Port: bound.Port}
	if err := tr.SendFrom(want, local, remote); err != nil {
		t.Fatalf("valid SendFrom after refusals: %v", err)
	}
	if err := peer.SetReadDeadline(time.Now().Add(prtArrive)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	var response [MaxMsgSize]byte
	n, source, err := peer.ReadFromUDP(response[:])
	if err != nil {
		t.Fatalf("read valid datagram: %v", err)
	}
	if !bytes.Equal(response[:n], want) {
		t.Fatalf("invalid source selection sent a datagram: %x", response[:n])
	}
	if !source.IP.Equal(local.IP) || source.Port != local.Port || source.Zone != local.Zone {
		t.Fatalf("valid datagram source = %v, want %v", source, local)
	}
}

// TestNATKeepaliveExplicitSource observes a keepalive from the configured SA
// endpoint on a wildcard NAT-T socket, rather than the route-selected source.
func TestNATKeepaliveExplicitSource(t *testing.T) {
	tr, err := NewNATTTransport("0.0.0.0:0", slog.Default())
	if err != nil {
		t.Fatalf("NewNATTTransport: %v", err)
	}
	t.Cleanup(func() { _ = tr.Close() })
	bound, ok := tr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("LocalAddr is not *net.UDPAddr")
	}
	peer, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	t.Cleanup(func() { _ = peer.Close() })
	remote, ok := peer.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("peer address is not *net.UDPAddr")
	}
	local := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 2), Port: bound.Port}
	ka := NewKeepalive(tr, local, remote, 50*time.Millisecond, slog.Default())
	go ka.Run()
	t.Cleanup(ka.Stop)
	if err := peer.SetReadDeadline(time.Now().Add(prtArrive)); err != nil {
		t.Fatalf("SetReadDeadline: %v", err)
	}
	var payload [16]byte
	n, source, err := peer.ReadFromUDP(payload[:])
	if err != nil {
		t.Fatalf("read keepalive: %v", err)
	}
	if n != 1 {
		t.Fatalf("keepalive length = %d, want one octet", n)
	}
	if payload[0] != 0xff {
		t.Fatalf("keepalive payload = %x, want ff", payload[:n])
	}
	if !source.IP.Equal(local.IP) || source.Port != local.Port || source.Zone != local.Zone {
		t.Fatalf("keepalive source = %v, want %v", source, local)
	}
}
