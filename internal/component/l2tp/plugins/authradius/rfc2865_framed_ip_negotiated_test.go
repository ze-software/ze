// Related: extract.go -- extractAuthMetadata handles Framed-IP-Address special values
// RFC: rfc/short/rfc2865.md -- Section 5.8
//
// VALIDATES: RFC 2865 Section 5.8 Framed-IP-Address 0xFFFFFFFF sets
// AuthMetadata.FramedIPNegotiated while leaving FramedIP unset, and neither
// 0xFFFFFFFE (NAS selects) nor a static unicast address sets it.
// PREVENTS: 0xFFFFFFFF being silently dropped as limited broadcast and falling
// through to pool assignment as if it were 0xFFFFFFFE.

package l2tpauthradius

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/radius"
)

// TestRFC2865FramedIPUserSelectsMarksNegotiated verifies that Framed-IP-Address
// 0xFFFFFFFF marks the session as peer-negotiated without pinning a literal IP.
//
// VALIDATES: extractAuthMetadata sets FramedIPNegotiated=true and leaves
// FramedIP unset for 0xFFFFFFFF, both alone and alongside other attributes.
// PREVENTS: isValidSubscriberIP discarding 0xFFFFFFFF and causing the pool to
// allocate an address instead of letting IPCP negotiate with the peer.
//
// RFC requirement: RFC2865-5.8-1 positive -- an Access-Accept carrying
// Framed-IP-Address 0xFFFFFFFF sets AuthMetadata.FramedIPNegotiated to true
// while keeping AuthMetadata.FramedIP unset (extract.go extractAuthMetadata).
func TestRFC2865FramedIPUserSelectsMarksNegotiated(t *testing.T) {
	alone := &radius.Packet{
		Code: radius.CodeAccessAccept,
		Attrs: []radius.Attr{
			{Type: radius.AttrFramedIPAddress, Value: []byte{0xff, 0xff, 0xff, 0xff}},
		},
	}
	meta := extractAuthMetadata(alone, "alice")
	if meta == nil {
		t.Fatal("extractAuthMetadata(0xFFFFFFFF) = nil, want non-nil metadata with FramedIPNegotiated=true")
	}
	if !meta.FramedIPNegotiated {
		t.Error("FramedIPNegotiated = false, want true for 0xFFFFFFFF")
	}
	if meta.FramedIP.IsValid() {
		t.Errorf("FramedIP = %v, want unset for 0xFFFFFFFF", meta.FramedIP)
	}

	withPool := &radius.Packet{
		Code: radius.CodeAccessAccept,
		Attrs: []radius.Attr{
			{Type: radius.AttrFramedIPAddress, Value: []byte{0xff, 0xff, 0xff, 0xff}},
			{Type: radius.AttrFramedPool, Value: []byte("gold")},
		},
	}
	metaWithPool := extractAuthMetadata(withPool, "alice")
	if metaWithPool == nil {
		t.Fatal("extractAuthMetadata(0xFFFFFFFF + Framed-Pool) = nil, want metadata")
	}
	if !metaWithPool.FramedIPNegotiated {
		t.Error("FramedIPNegotiated = false, want true when paired with Framed-Pool")
	}
	if metaWithPool.FramedIP.IsValid() {
		t.Errorf("FramedIP = %v, want unset when paired with Framed-Pool", metaWithPool.FramedIP)
	}
	if metaWithPool.FramedPool != "gold" {
		t.Errorf("FramedPool = %q, want gold", metaWithPool.FramedPool)
	}
}

// TestRFC2865FramedIPNASSelectsDoesNotMarkNegotiated verifies that 0xFFFFFFFE
// and static unicast Framed-IP-Address values never enable peer IP selection.
//
// VALIDATES: extractAuthMetadata leaves FramedIPNegotiated=false for 0xFFFFFFFE
// (NAS/pool selects) and for a static unicast IPv4 address.
// PREVENTS: mistakenly allowing peer-selected addresses when RADIUS asked the
// NAS to assign from its pool or pinned a specific IP.
//
// RFC requirement: RFC2865-5.8-1 negative -- an Access-Accept carrying
// Framed-IP-Address 0xFFFFFFFE or a specific unicast IPv4 address leaves
// AuthMetadata.FramedIPNegotiated false (extract.go extractAuthMetadata).
func TestRFC2865FramedIPNASSelectsDoesNotMarkNegotiated(t *testing.T) {
	nasSelects := &radius.Packet{
		Code: radius.CodeAccessAccept,
		Attrs: []radius.Attr{
			{Type: radius.AttrFramedIPAddress, Value: []byte{0xff, 0xff, 0xff, 0xfe}},
			{Type: radius.AttrFramedPool, Value: []byte("subscribers")},
		},
	}
	metaNAS := extractAuthMetadata(nasSelects, "bob")
	if metaNAS == nil {
		t.Fatal("extractAuthMetadata(0xFFFFFFFE + Framed-Pool) = nil, want metadata")
	}
	if metaNAS.FramedIPNegotiated {
		t.Error("0xFFFFFFFE set FramedIPNegotiated=true, want false (NAS selects)")
	}
	if metaNAS.FramedIP.IsValid() {
		t.Errorf("0xFFFFFFFE pinned FramedIP = %v, want unset", metaNAS.FramedIP)
	}

	pinned := &radius.Packet{
		Code: radius.CodeAccessAccept,
		Attrs: []radius.Attr{
			{Type: radius.AttrFramedIPAddress, Value: []byte{198, 51, 100, 10}},
		},
	}
	metaPinned := extractAuthMetadata(pinned, "carol")
	if metaPinned == nil {
		t.Fatal("extractAuthMetadata(198.51.100.10) = nil, want metadata")
	}
	if metaPinned.FramedIPNegotiated {
		t.Error("static Framed-IP-Address set FramedIPNegotiated=true, want false")
	}
	if want := netip.MustParseAddr("198.51.100.10"); metaPinned.FramedIP != want {
		t.Errorf("FramedIP = %v, want %v", metaPinned.FramedIP, want)
	}
}
