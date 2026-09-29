//go:build integration && linux

// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- QEMU proof of the RFC Section 7.2 transmit fields on the wire.
// Related: transport_integration_linux_test.go -- the veth lab and capture helpers these tests reuse.

package transport

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

// The expected wire values are literals from the RFCs, never the package
// constants the code under test writes, so a drifted constant is caught.
var (
	wireVMACv4    = []byte{0x00, 0x00, 0x5e, 0x00, 0x01, testVRID} // RFC 5798/9568 Section 7.3 IPv4 Virtual Router MAC
	wireVMACv6    = []byte{0x00, 0x00, 0x5e, 0x00, 0x02, testVRID} // RFC 5798/9568 Section 7.3 IPv6 Virtual Router MAC
	wireGroupV4   = netip.MustParseAddr("224.0.0.18")
	wireGroupV6   = netip.MustParseAddr("ff02::12")
	secondaryV4   = "192.0.2.5/24" // lower than the primary, added after it: the kernel marks it secondary
	wireProtoVRRP = byte(112)
)

// TestTxAdvertV4OnWire proves what a transmitted IPv4 advertisement carries,
// for VRRPv2 and VRRPv3, read from a frame captured on the peer veth.
//
// Method: the parent holds its primary 192.0.2.251/24 and then a secondary
// 192.0.2.5/24 in the same subnet. For version 2 and version 3 the transport
// encodes and sends one advertisement; the captured frame (matched on the
// ethertype and the VRRP version/type octet only) must carry the source MAC
// 00-00-5e-00-01-0a, IP protocol 112, destination 224.0.0.18 and source
// 192.0.2.251, never the secondary 192.0.2.5.
//
// RFC requirement: RFC3768-7.2-2 positive -- a transmitted VRRPv2 advertisement leaves with source MAC 00-00-5e-00-01-{VRID}, read from the captured frame (openV4 backend_linux.go).
// RFC requirement: RFC5798-7.2-2 positive -- a transmitted VRRPv3 IPv4 advertisement leaves with source MAC 00-00-5e-00-01-{VRID}, read from the captured frame (openV4 backend_linux.go).
// RFC requirement: RFC9568-7.2-2 positive -- a transmitted VRRPv3 IPv4 advertisement leaves with source MAC 00-00-5e-00-01-{VRID}, read from the captured frame (openV4 backend_linux.go).
// RFC requirement: RFC3768-7.2-3 positive -- with a primary 192.0.2.251/24 and a lower secondary 192.0.2.5/24 on the interface, a VRRPv2 advertisement leaves with source 192.0.2.251 (resolveParentPrimaryV4 transport.go).
// RFC requirement: RFC5798-7.2-3 positive -- with a primary 192.0.2.251/24 and a lower secondary 192.0.2.5/24 on the interface, a VRRPv3 IPv4 advertisement leaves with source 192.0.2.251, the interface primary, never the secondary (resolveParentPrimaryV4 transport.go).
// RFC requirement: RFC9568-7.2-3 positive -- with a primary 192.0.2.251/24 and a lower secondary 192.0.2.5/24 on the interface, a VRRPv3 IPv4 advertisement leaves with source 192.0.2.251, the interface's primary IPv4 address, never the secondary (resolveParentPrimaryV4 transport.go).
// RFC requirement: RFC3768-7.2-4 positive -- a captured VRRPv2 advertisement carries IP protocol 112 and destination 224.0.0.18, compared with literals (buildIPv4Header transport.go).
// RFC requirement: RFC5798-7.2-4 positive -- a captured VRRPv3 IPv4 advertisement carries IP protocol 112 and destination 224.0.0.18, compared with literals (buildIPv4Header transport.go).
// RFC requirement: RFC9568-7.2-4 positive -- a captured VRRPv3 IPv4 advertisement carries IP protocol 112 and destination 224.0.0.18, compared with literals (buildIPv4Header transport.go).
func TestTxAdvertV4OnWire(t *testing.T) {
	lab := setupLab(t, packet.V4)
	parentLink, err := netlink.LinkByName(lab.parent)
	if err != nil {
		t.Fatalf("LinkByName(%s): %v", lab.parent, err)
	}
	secondary, err := netlink.ParseAddr(secondaryV4)
	if err != nil {
		t.Fatalf("ParseAddr: %v", err)
	}
	if err := netlink.AddrAdd(parentLink, secondary); err != nil {
		t.Fatalf("AddrAdd secondary: %v", err)
	}
	tr, key := openTransportInstance(t, lab, packet.V4)

	for _, version := range []uint8{packet.VersionV2, packet.VersionV3} {
		params := AdvertParams{Version: version, Priority: 100, AdverIntervalMS: 1000, VIPs: []netip.Addr{netip.MustParseAddr("192.0.2.1")}}
		if err := tr.UpdateAdvert(key, params); err != nil {
			t.Fatalf("v%d UpdateAdvert: %v", version, err)
		}
		if err := tr.SendAdvert(key); err != nil {
			t.Fatalf("v%d SendAdvert: %v", version, err)
		}
		frame := captureMatch(t, lab.captureFD, func(f []byte) bool {
			return len(f) >= 42 && f[12] == 0x08 && f[13] == 0x00 && f[14]>>4 == 4 &&
				f[14+int(f[14]&0x0f)*4] == version<<4|1
		})
		if !bytes.Equal(frame[6:12], wireVMACv4) {
			t.Errorf("v%d source MAC = % x, want % x", version, frame[6:12], wireVMACv4)
		}
		ip := frame[14:]
		if ip[9] != wireProtoVRRP {
			t.Errorf("v%d IP protocol = %d, want 112", version, ip[9])
		}
		if dst := netip.AddrFrom4([4]byte(ip[16:20])); dst != wireGroupV4 {
			t.Errorf("v%d destination = %v, want 224.0.0.18", version, dst)
		}
		if src := netip.AddrFrom4([4]byte(ip[12:16])); src != netip.MustParseAddr(parentV4) {
			t.Errorf("v%d source = %v, want the primary %s", version, src, parentV4)
		}
	}
}

// TestTxAdvertV6OnWire proves what a transmitted IPv6 advertisement carries,
// read from a frame captured on the peer veth.
//
// Method: the transport sends one VRRPv3 IPv6 advertisement from the macvlan.
// The captured frame (matched on the ethertype, IP version and the VRRP
// version/type octet only) must carry the source MAC 00-00-5e-00-02-0a, next
// header 112, destination ff02::12, and as source the macvlan's own
// link-local address, which must be a link-local unicast address.
//
// RFC requirement: RFC5798-7.2-2 positive -- a transmitted VRRPv3 IPv6 advertisement leaves with source MAC 00-00-5e-00-02-{VRID}, read from the captured frame (openV6 backend_linux.go).
// RFC requirement: RFC9568-7.2-2 positive -- a transmitted VRRPv3 IPv6 advertisement leaves with source MAC 00-00-5e-00-02-{VRID}, read from the captured frame (openV6 backend_linux.go).
// RFC requirement: RFC5798-7.2-3 positive -- a captured IPv6 advertisement's source is the sending macvlan's link-local address (macvlanLinkLocal backend_linux.go, SendAdvert backend_linux.go).
// RFC requirement: RFC9568-7.2-3 positive -- a captured IPv6 advertisement's source is the sending macvlan's link-local address (macvlanLinkLocal backend_linux.go, SendAdvert backend_linux.go).
// RFC requirement: RFC5798-7.2-4 positive -- a captured IPv6 advertisement carries next header 112 and destination ff02::12, compared with literals (SendAdvert backend_linux.go).
// RFC requirement: RFC9568-7.2-4 positive -- a captured IPv6 advertisement carries next header 112 and destination ff02::12, compared with literals (SendAdvert backend_linux.go).
func TestTxAdvertV6OnWire(t *testing.T) {
	lab := setupLab(t, packet.V6)
	tr, key := openTransportInstance(t, lab, packet.V6)
	ll := waitLinkLocal(t, lab.macvlan)
	if err := tr.UpdateAdvert(key, AdvertParams{Version: packet.VersionV3, Priority: 100, AdverIntervalMS: 1000, VIPs: []netip.Addr{ll}}); err != nil {
		t.Fatalf("UpdateAdvert: %v", err)
	}
	if err := tr.SendAdvert(key); err != nil {
		t.Fatalf("SendAdvert: %v", err)
	}
	frame := captureMatch(t, lab.captureFD, func(f []byte) bool {
		return len(f) >= 62 && f[12] == 0x86 && f[13] == 0xdd && f[14]>>4 == 6 && f[54] == 0x31
	})
	if !bytes.Equal(frame[6:12], wireVMACv6) {
		t.Errorf("source MAC = % x, want % x", frame[6:12], wireVMACv6)
	}
	ip := frame[14:]
	if ip[6] != wireProtoVRRP {
		t.Errorf("next header = %d, want 112", ip[6])
	}
	if dst := netip.AddrFrom16([16]byte(ip[24:40])); dst != wireGroupV6 {
		t.Errorf("destination = %v, want ff02::12", dst)
	}
	src := netip.AddrFrom16([16]byte(ip[8:24]))
	if !src.IsLinkLocalUnicast() {
		t.Errorf("source = %v, want a link-local address", src)
	}
	if src != ll {
		t.Errorf("source = %v, want the macvlan link-local %v", src, ll)
	}
}
