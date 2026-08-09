//go:build linux

// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- Linux backend resolver and drop regressions

package transport

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
)

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
