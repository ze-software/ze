// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- advertisement transmit path
// Related: transport.go -- encodeLocked, the encoder under test
package transport

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
)

// VALIDATES: a transmitted IPv6 advertisement lists its addresses in the order
// the group gave them, so the link-local the config puts first is first on the
// wire.
//
// TestSendAdvertIPv6LeadsWithLinkLocal checks the address order of a sent IPv6
// advertisement. The group config refuses an IPv6 group whose first virtual
// address is not link-local (validateGroup groups.go), so the order the
// transport keeps is what puts the link-local first on the wire. netip order
// sorts 2001:db8::1 ahead of fe80::1, so a transport that sorted or reordered
// the list would send the global address first.
//
// Method: an IPv6 instance is given fe80::1 then 2001:db8::1 and sends one
// advertisement. The frame is the VRRP message (the kernel adds the IPv6
// header), so the first address sits right after the 8-octet fixed header.
//
// RFC requirement: RFC5798-5.2.9-1 positive -- a transmitted IPv6 advertisement for a group whose first address is the link-local fe80::1 carries fe80::1 as its first IPvX address and the global address after it, read from the sent frame (encodeLocked transport.go).
func TestSendAdvertIPv6LeadsWithLinkLocal(t *testing.T) {
	fb := &fakeBackend{}
	tr := New(fb)
	key, err := tr.OpenInstance(v6Spec())
	if err != nil {
		t.Fatalf("OpenInstance: %v", err)
	}
	linkLocal := netip.MustParseAddr("fe80::1")
	global := netip.MustParseAddr("2001:db8::1")
	if err := tr.UpdateAdvert(key, AdvertParams{
		Version:         packet.VersionV3,
		Priority:        100,
		AdverIntervalMS: 1000,
		VIPs:            []netip.Addr{linkLocal, global},
	}); err != nil {
		t.Fatalf("UpdateAdvert: %v", err)
	}
	if err := tr.SendAdvert(key); err != nil {
		t.Fatalf("SendAdvert: %v", err)
	}

	frame := fb.last().lastAdvert()
	if len(frame) != packet.HeaderLen+2*16 {
		t.Fatalf("sent frame is %d octets, want %d (header and two IPv6 addresses)", len(frame), packet.HeaderLen+2*16)
	}
	first, _ := netip.AddrFromSlice(frame[packet.HeaderLen : packet.HeaderLen+16])
	second, _ := netip.AddrFromSlice(frame[packet.HeaderLen+16 : packet.HeaderLen+32])
	if first != linkLocal {
		t.Fatalf("first address on the wire = %v, want the link-local %v", first, linkLocal)
	}
	if second != global {
		t.Fatalf("second address on the wire = %v, want %v", second, global)
	}
}
