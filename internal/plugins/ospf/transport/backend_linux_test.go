//go:build linux

// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- Linux backend resolver and drop regressions

package transport

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC2328-8.1-1 positive -- resolving an active OSPF interface supplies
// its assigned IPv4 address to the raw socket configuration as the packet source.
func TestResolveOSPFInterfaceUsesIfaceResolverOSName(t *testing.T) {
	oldBinding := resolveIfaceBinding
	oldAddresses := resolveIfaceAddresses
	oldEnsure := ensureIfaceBackend
	t.Cleanup(func() {
		resolveIfaceBinding = oldBinding
		resolveIfaceAddresses = oldAddresses
		ensureIfaceBackend = oldEnsure
	})

	// resolveOSPFInterface loads the iface backend before resolving
	// (backend_linux.go:47). No backend is registered in a unit-test binary, so
	// without this seam the real iface.EnsureBackend fails with
	// `unknown backend "netlink" (registered: [])` and the resolver stubs below
	// are never reached.
	ensureIfaceBackend = func() error { return nil }

	resolveIfaceBinding = func(name string) (iface.Binding, error) {
		if name != "uplink" {
			t.Fatalf("resolve name = %q, want uplink", name)
		}
		return iface.Binding{Ifindex: 77, OsName: "eth-kernel"}, nil
	}
	resolveIfaceAddresses = func(name string) ([]iface.AddrInfo, error) {
		if name != "uplink" {
			t.Fatalf("addresses name = %q, want uplink", name)
		}
		return []iface.AddrInfo{{Address: "192.0.2.10", Family: "ipv4"}}, nil
	}

	got, err := resolveOSPFInterface("uplink")
	if err != nil {
		t.Fatalf("resolveOSPFInterface: %v", err)
	}
	if got.osName != "eth-kernel" || got.ifindex != 77 || got.local != [4]byte{192, 0, 2, 10} {
		t.Fatalf("resolved = %+v, want os-name eth-kernel ifindex 77 local 192.0.2.10", got)
	}
}

func TestLinuxInterfaceCountsMalformedReceiveDrop(t *testing.T) {
	var reasons []string
	li := &linuxInterface{
		ifindex:    44,
		recvCh:     make(chan RawPacket, 1),
		stop:       make(chan struct{}),
		recordDrop: dropRecorder(func(reason string) { reasons = append(reasons, reason) }),
	}

	if li.deliverDatagram([]byte{0x45}) {
		t.Fatalf("short IPv4 datagram delivered")
	}
	if len(reasons) != 1 || reasons[0] != dropMalformedIPv4 {
		t.Fatalf("drop reasons = %v, want [%s]", reasons, dropMalformedIPv4)
	}
	select {
	case got := <-li.recvCh:
		t.Fatalf("malformed datagram delivered: %+v", got)
	default:
	}

	packet := []byte{
		0x45, 0, 0, 24, 0, 0, 0, 0, 1, Protocol, 0, 0,
		192, 0, 2, 1, 224, 0, 0, 5,
		0xde, 0xad, 0xbe, 0xef,
	}
	binary.BigEndian.PutUint16(packet[10:12], types.InternetChecksumPair(packet[:20], nil))
	if !li.deliverDatagram(packet) {
		t.Fatalf("valid IPv4 datagram not delivered")
	}
	select {
	case got := <-li.recvCh:
		if got.IfIndex != 44 || got.Src != netip.MustParseAddr("192.0.2.1") || !bytes.Equal(got.Payload, []byte{0xde, 0xad, 0xbe, 0xef}) {
			t.Fatalf("received = %+v", got)
		}
	default:
		t.Fatalf("valid datagram missing from receive channel")
	}
	if len(reasons) != 1 {
		t.Fatalf("valid datagram changed drop reasons: %v", reasons)
	}
}

// RFC requirement: RFC2328-8.1-1 negative -- an interface without an assigned IPv4
// source, including an unspecified or multicast value, cannot open an OSPF socket.
func TestOSPFRefusesInterfaceWithoutIPv4Source(t *testing.T) {
	oldAddresses := resolveIfaceAddresses
	t.Cleanup(func() { resolveIfaceAddresses = oldAddresses })
	for _, address := range []string{"", "0.0.0.0", "224.0.0.5", "255.255.255.255", "2001:db8::1"} {
		t.Run(address, func(t *testing.T) {
			resolveIfaceAddresses = func(string) ([]iface.AddrInfo, error) {
				return []iface.AddrInfo{{Address: address}}, nil
			}
			if _, err := interfaceIPv4("eth0"); err == nil {
				t.Fatalf("interface with address %q supplied an OSPF source", address)
			}
		})
	}
}

// ipv4OSPFDatagram supplies the kernel-delivered header and payload to the live receive gate.
func ipv4OSPFDatagram(dst netip.Addr) []byte {
	data := []byte{
		0x45, 0, 0, 24, 0, 0, 0, 0, 1, Protocol, 0, 0,
		192, 0, 2, 2, 0, 0, 0, 0, 0xde, 0xad, 0xbe, 0xef,
	}
	addr := dst.As4()
	copy(data[16:20], addr[:])
	binary.BigEndian.PutUint16(data[10:12], types.InternetChecksumPair(data[:20], nil))
	return data
}

// RFC requirement: RFC2328-8.2-1 positive -- the Linux receive path delivers protocol-89
// packets with a valid IP checksum addressed to its interface or either OSPF multicast group.
func TestOSPFReceiveAcceptsValidIPv4Envelope(t *testing.T) {
	local := netip.MustParseAddr("192.0.2.1")
	li := &linuxInterface{ifindex: 44, local: local.As4(), recvCh: make(chan RawPacket, 1), stop: make(chan struct{})}
	for _, dst := range []netip.Addr{local, AllSPFRouters, AllDRouters} {
		data := ipv4OSPFDatagram(dst)
		if !li.deliverDatagram(data) {
			t.Fatalf("valid IPv4 packet to %s rejected", dst)
		}
		select {
		case got := <-li.recvCh:
			if got.Src != netip.MustParseAddr("192.0.2.2") || got.Dst != dst || !bytes.Equal(got.Payload, data[20:]) {
				t.Fatalf("delivered packet = %+v", got)
			}
		default:
			t.Fatal("accepted packet never reached the router receive channel")
		}
	}
}

// RFC requirement: RFC2328-8.2-1 negative -- an invalid IP checksum, foreign protocol
// or foreign destination is rejected before a packet reaches the router receive channel.
func TestOSPFReceiveRejectsInvalidIPv4Envelope(t *testing.T) {
	local := netip.MustParseAddr("192.0.2.1")
	for _, which := range []string{"checksum", "protocol", "destination", "version", "length"} {
		t.Run(which, func(t *testing.T) {
			data := ipv4OSPFDatagram(local)
			switch which {
			case "checksum":
				data[8]++
			case "protocol":
				data[9] = 17
			case "destination":
				data[19] = 9
			case "version":
				data[0] = 0x65
			case "length":
				data[3] = 19
			}
			if which != "checksum" {
				clear(data[10:12])
				binary.BigEndian.PutUint16(data[10:12], types.InternetChecksumPair(data[:20], nil))
			}
			drops := 0
			li := &linuxInterface{local: local.As4(), recvCh: make(chan RawPacket, 1), stop: make(chan struct{}),
				recordDrop: func(string) { drops++ }}
			if li.deliverDatagram(data) {
				t.Fatal("invalid IPv4 packet accepted")
			}
			if drops != 1 {
				t.Fatalf("drop count = %d, want 1", drops)
			}
			select {
			case got := <-li.recvCh:
				t.Fatalf("rejected IPv4 packet delivered: %+v", got)
			default:
			}
		})
	}
}
