package message

import (
	"fmt"
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// RFC 7606 Section 5.1: "The MP_REACH_NLRI or MP_UNREACH_NLRI attribute (if
// present) SHALL be encoded as the very first path attribute in an UPDATE
// message."
//
// Every message-package builder writes its attributes through
// attribute.OrderAttributes, which places the MP attribute first and the rest
// by ascending type code. This test asks each builder that emits MP_REACH_NLRI,
// with attributes of lower type code present, which attribute leads.
// BuildGroupedUnicast builds IPv4 unicast only and carries no MP attribute.

// VALIDATES: BuildUnicast (IPv6), BuildVPN and BuildLabeledUnicast, each given ORIGIN, AS_PATH and MED (codes below 14) and
// LARGE_COMMUNITY (above it), emit MP_REACH_NLRI as the first path attribute,
// and the attributes after it ascend by type code.
// PREVENTS: MP_REACH_NLRI sorting into its type-code position behind ORIGIN and
// AS_PATH, the order Ze sent before RFC7606-5.1-1 was met.
// RFC requirement: RFC7606-5.1-1 positive -- an UPDATE built by BuildUnicast (IPv6), BuildVPN or BuildLabeledUnicast with ORIGIN, AS_PATH, MED and LARGE_COMMUNITY carries MP_REACH_NLRI (type 14) as its first path attribute.
func TestRFC7606Section51MPAttributeEncodedFirst(t *testing.T) {
	unicast := UnicastParams{
		Prefix:           netip.MustParsePrefix("2001:db8:1::/48"),
		NextHop:          netip.MustParseAddr("2001:db8::1"),
		Origin:           attribute.OriginIGP,
		ASPath:           []uint32{65002},
		MED:              10,
		HasMED:           true,
		LargeCommunities: [][3]uint32{{65001, 1, 2}},
	}
	blocks := map[string][]byte{}

	built := mustBuildUnicast(t, NewUpdateBuilder(65001, false, true, false), &unicast)
	blocks["BuildUnicast"] = append([]byte(nil), built.PathAttributes...)

	vpn := NewUpdateBuilder(65001, false, true, false).BuildVPN(&VPNParams{
		Prefix:           netip.MustParsePrefix("10.0.0.0/24"),
		NextHop:          netip.MustParseAddr("192.168.1.1"),
		Origin:           attribute.OriginIGP,
		ASPath:           []uint32{65002},
		Labels:           []uint32{100},
		RDBytes:          [8]byte{0, 1, 0, 0, 0, 100, 0, 100},
		MED:              50,
		HasMED:           true,
		LargeCommunities: [][3]uint32{{65001, 1, 2}},
	})
	if vpn == nil {
		t.Fatal("BuildVPN returned nil")
		return
	}
	blocks["BuildVPN"] = vpn.PathAttributes

	labeled := NewUpdateBuilder(65001, false, true, false).BuildLabeledUnicast(&LabeledUnicastParams{
		Prefix:           netip.MustParsePrefix("2001:db8:2::/48"),
		NextHop:          netip.MustParseAddr("2001:db8::1"),
		Origin:           attribute.OriginIGP,
		ASPath:           []uint32{65002},
		MED:              10,
		HasMED:           true,
		LargeCommunities: [][3]uint32{{65001, 1, 2}},
		Labels:           []uint32{100},
	})
	if labeled == nil {
		t.Fatal("BuildLabeledUnicast returned nil")
		return
	}
	blocks["BuildLabeledUnicast"] = labeled.PathAttributes

	for _, builder := range []string{"BuildUnicast", "BuildVPN", "BuildLabeledUnicast"} {
		codes, err := section51AttributeCodes(blocks[builder])
		if err != nil {
			t.Fatalf("%s: path attribute block: %v", builder, err)
			return
		}
		if !slices.Contains(codes, attribute.AttrOrigin) {
			t.Fatalf("%s: no ORIGIN among %v, so nothing of a lower code competes for first place", builder, codes)
			return
		}
		// RFC 7606 Section 5.1
		if codes[0] != attribute.AttrMPReachNLRI {
			t.Errorf("%s: first path attribute is type %d, want MP_REACH_NLRI (14); order %v",
				builder, codes[0], codes)
			continue
		}
		// RFC 4271 Section 5: the rest ascend by type code.
		for i := 2; i < len(codes); i++ {
			if codes[i-1] >= codes[i] {
				t.Errorf("%s: attributes after MP_REACH_NLRI do not ascend: %v", builder, codes)
				break
			}
		}
	}
}

// section51AttributeCodes answers the type code of every path attribute in
// block, in wire order.
//
// RFC 4271 Section 4.3: each attribute is a flags octet, a type code octet,
// then a length of one octet, or two when the Extended Length bit of the
// flags is set, then the value.
func section51AttributeCodes(block []byte) ([]attribute.AttributeCode, error) {
	var codes []attribute.AttributeCode
	for off := 0; off < len(block); {
		if off+3 > len(block) {
			return nil, fmt.Errorf("attribute at offset %d runs past the %d-octet block", off, len(block))
		}
		flags := attribute.AttributeFlags(block[off])
		code := attribute.AttributeCode(block[off+1])
		headerLen := 3
		valueLen := int(block[off+2])
		if flags.IsExtLength() {
			if off+4 > len(block) {
				return nil, fmt.Errorf("attribute at offset %d runs past the %d-octet block", off, len(block))
			}
			headerLen = 4
			valueLen = int(block[off+2])<<8 | int(block[off+3])
		}
		end := off + headerLen + valueLen
		if end > len(block) {
			return nil, fmt.Errorf("attribute at offset %d runs past the %d-octet block", off, len(block))
		}
		codes = append(codes, code)
		off = end
	}
	if len(codes) == 0 {
		return nil, fmt.Errorf("the path attribute block is empty")
	}
	return codes, nil
}
