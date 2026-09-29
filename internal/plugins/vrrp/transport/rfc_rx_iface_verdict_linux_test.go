//go:build integration && linux

// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- QEMU proof that receive is scoped to the interface the VRID is configured on.
// Related: transport_integration_linux_test.go -- the veth lab and the IP_HDRINCL helpers these tests reuse.

package transport

import (
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

const otherVRID = 20

// addOtherInterface adds a second veth pair beside the lab's, with an IPv4
// address and a macvlan, in the lab's namespace. It returns the parent, peer
// and macvlan names.
func addOtherInterface(t *testing.T, lab vrrpLab) (parent, peer, macvlan string) {
	t.Helper()
	parent = strings.Replace(lab.parent, "zevrpp", "zevrpb", 1)
	peer = strings.Replace(lab.parent, "zevrpp", "zevrpc", 1)
	macvlan = strings.Replace(lab.parent, "zevrpp", "zevrpn", 1)
	if err := netlink.LinkAdd(&netlink.Veth{Name: parent, PeerName: peer}); err != nil {
		t.Fatalf("add second veth: %v", err)
	}
	parentLink, err := netlink.LinkByName(parent)
	if err != nil {
		t.Fatalf("LinkByName(%s): %v", parent, err)
	}
	peerLink, err := netlink.LinkByName(peer)
	if err != nil {
		t.Fatalf("LinkByName(%s): %v", peer, err)
	}
	mustUp(t, parentLink)
	mustUp(t, peerLink)
	addr, err := netlink.ParseAddr("198.51.100.251/24")
	if err != nil {
		t.Fatalf("ParseAddr: %v", err)
	}
	if err := netlink.AddrAdd(parentLink, addr); err != nil {
		t.Fatalf("AddrAdd second parent: %v", err)
	}
	vmac := packet.VirtualMAC(packet.V4, otherVRID)
	mv := &netlink.Macvlan{
		Name: macvlan, ParentIndex: parentLink.Attrs().Index, HardwareAddr: net.HardwareAddr(vmac[:]),
		Mode: netlink.MACVLAN_MODE_BRIDGE,
	}
	if err := netlink.LinkAdd(mv); err != nil {
		t.Fatalf("add second macvlan: %v", err)
	}
	mvLink, err := netlink.LinkByName(macvlan)
	if err != nil {
		t.Fatalf("LinkByName(%s): %v", macvlan, err)
	}
	mustUp(t, mvLink)
	return parent, peer, macvlan
}

// peerSender returns a function that sends one VRID testVRID advertisement of
// the given version, TTL 255, from source over a raw socket bound to peer.
func peerSender(t *testing.T, peer, source string, version uint8) func() {
	t.Helper()
	var datagram [64]byte
	adv := packet.Advertisement{Version: version, Family: packet.V4, VRID: testVRID, Priority: 100, AdverIntervalMS: 1000, VIPs: []netip.Addr{netip.MustParseAddr("192.0.2.1")}}
	n := adv.WriteTo(datagram[:], ipv4HeaderLen)
	src := netip.MustParseAddr(source)
	packet.FillChecksum(datagram[:], ipv4HeaderLen, n, src, packet.MulticastV4)
	hdr := buildIPv4Header(datagram[:], src.As4(), packet.MulticastV4.As4())

	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_RAW, int(packet.ProtoNumber))
	if err != nil {
		t.Fatalf("peer tx socket: %v", err)
	}
	t.Cleanup(func() { unix.Close(fd) }) //nolint:errcheck // best-effort cleanup
	if err := unix.SetsockoptString(fd, unix.SOL_SOCKET, unix.SO_BINDTODEVICE, peer); err != nil {
		t.Fatalf("bind peer tx: %v", err)
	}
	if err := unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_HDRINCL, 1); err != nil {
		t.Fatalf("peer IP_HDRINCL: %v", err)
	}
	return func() {
		if err := unix.Sendto(fd, datagram[:hdr+n], 0, &unix.SockaddrInet4{Addr: packet.MulticastV4.As4()}); err != nil {
			t.Fatalf("peer sendto: %v", err)
		}
	}
}

// collectRx sends with send every 100 ms for the window and returns every
// item the transport delivered meanwhile.
func collectRx(tr *Transport, send func(), window time.Duration) []RxItem {
	var items []RxItem
	deadline := time.After(window)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	send()
	for {
		select {
		case item := <-tr.Receive():
			items = append(items, item)
		case <-tick.C:
			send()
		case <-deadline:
			return items
		}
	}
}

// TestRxAdvertScopedToReceivingInterface proves that an advertisement is
// checked against the VRIDs configured on the interface it arrived on.
//
// Method: VRID 10 is configured on interface A and VRID 20 on interface B, in
// one namespace, both joined to 224.0.0.18. For VRRPv2 and VRRPv3 a VRID 10
// advertisement is sent from B's peer: every delivered copy must belong to B's
// instance, none to A's, at least one must arrive, and decoding it against B's
// configuration (VRID 20 only) must refuse it with ErrUnknownVRID. The same
// advertisement sent from A's peer must reach A's instance and decode against
// A's configuration.
//
// RFC requirement: RFC9568-7.1-4 positive -- a VRRPv3 advert for VRID 10 arriving on the interface where VRID 10 is configured is delivered to that interface's instance and decodes (openV4 backend_linux.go, Decode packet/validate.go).
// RFC requirement: RFC9568-7.1-4 negative -- the same VRRPv3 advert arriving on another interface, where only VRID 20 is configured, never reaches the VRID 10 instance and is refused there with ErrUnknownVRID (openV4 backend_linux.go, Decode packet/validate.go).
// RFC requirement: RFC3768-7.1-4 positive -- a VRRPv2 advert for VRID 10 arriving on the interface where VRID 10 is configured is delivered to that interface's instance and decodes (openV4 backend_linux.go, Decode packet/validate.go).
// RFC requirement: RFC3768-7.1-4 negative -- the same VRRPv2 advert arriving on another interface, where only VRID 20 is configured, never reaches the VRID 10 instance and is refused there with ErrUnknownVRID (openV4 backend_linux.go, Decode packet/validate.go).
func TestRxAdvertScopedToReceivingInterface(t *testing.T) {
	lab := setupLab(t, packet.V4)
	otherParent, otherPeer, otherMacvlan := addOtherInterface(t, lab)
	tr, keyA := openTransportInstance(t, lab, packet.V4)
	keyB, err := tr.OpenInstance(InstanceSpec{
		Family: packet.V4, VRID: otherVRID, Parent: otherParent,
		MacvlanDevice: otherMacvlan, VirtualMAC: packet.VirtualMAC(packet.V4, otherVRID),
	})
	if err != nil {
		t.Fatalf("OpenInstance on the second interface: %v", err)
	}

	for _, version := range []uint8{packet.VersionV2, packet.VersionV3} {
		configured := func(vrid uint8) packet.Lookup {
			return func(got uint8) (packet.Local, bool) {
				if got != vrid {
					return packet.Local{}, false
				}
				return packet.Local{Version: version, AdverIntervalMS: 1000}, true
			}
		}

		var atB int
		for _, item := range collectRx(tr, peerSender(t, otherPeer, "198.51.100.2", version), 1500*time.Millisecond) {
			if item.Key == keyA {
				t.Fatalf("v%d: VRID 10 advert received on the other interface reached the VRID 10 instance", version)
			}
			if item.Key != keyB {
				continue
			}
			atB++
			if _, derr := packet.Decode(item.Payload, item.Meta, configured(otherVRID)); !errors.Is(derr, packet.ErrUnknownVRID) {
				t.Fatalf("v%d: VRID 10 advert at the VRID 20 interface decoded with %v, want ErrUnknownVRID", version, derr)
			}
		}
		if atB == 0 {
			t.Fatalf("v%d: the advert sent on the other interface never arrived there, so its absence at A proves nothing", version)
		}

		var atA int
		for _, item := range collectRx(tr, peerSender(t, lab.peer, "192.0.2.2", version), 1500*time.Millisecond) {
			if item.Key != keyA {
				continue
			}
			atA++
			if _, derr := packet.Decode(item.Payload, item.Meta, configured(testVRID)); derr != nil {
				t.Fatalf("v%d: VRID 10 advert at the VRID 10 interface: %v, want decoded", version, derr)
			}
		}
		if atA == 0 {
			t.Fatalf("v%d: VRID 10 advert sent on the VRID 10 interface never reached its instance", version)
		}
	}
}
