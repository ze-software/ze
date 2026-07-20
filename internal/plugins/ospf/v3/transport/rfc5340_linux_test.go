//go:build linux

// VALIDATES: RFC 5340 §A.1 / §4.2.2 -- the OSPFv3 transport runs directly over IPv6 with
// Next Header 89, so every datagram it sends carries protocol 89 and the only datagrams the
// kernel ever delivers to it are protocol-89 ones.
// PREVENTS: the OSPFv3 socket being opened on a different protocol number (which would both
// emit an unrecognizable Next Header and silently receive traffic that is not OSPF).

package transport

import "testing"

// RFC requirement: RFC5340-4.2.2-2 positive -- the Next Header field of the encapsulating IPv6
// header specifies the OSPF protocol (89): the transport opens its raw socket on the "ip6:89"
// network (listenNetwork, backend_linux.go:28), which is what stamps Next Header 89 on every
// packet sent and is the kernel's demux key for every packet received, matching the OSPF
// protocol number the package declares (Protocol, multicast.go:11).
func TestRFC5340TransportUsesOSPFProtocolNumber(t *testing.T) {
	if Protocol != 89 {
		t.Fatalf("OSPF protocol number = %d, want 89 (RFC 5340 §A.1)", Protocol)
	}
	if listenNetwork != "ip6:89" {
		t.Fatalf("OSPFv3 listen network = %q, want %q: the socket protocol IS the Next Header value",
			listenNetwork, "ip6:89")
	}
}
