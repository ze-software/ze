package message

import (
	"fmt"
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/test/rfcgap"
)

// RFC 7606 Section 5.1: "The MP_REACH_NLRI or MP_UNREACH_NLRI attribute (if
// present) SHALL be encoded as the very first path attribute in an UPDATE
// message."
//
// Ze does not meet this on purpose: BuildUnicast writes the attributes through
// attribute.OrderAttributes, which puts MP_UNREACH_NLRI first and every other
// attribute in type-code order, so ORIGIN (1) and AS_PATH (2) precede
// MP_REACH_NLRI (14). The decision
// and its reasoning are in docs/architecture/wire/mp-nlri-ordering.md, and
// rfc/short/rfc7606.md keeps RFC7606-5.1-1 annotated {gap}. The test below
// demonstrates that gap: it asserts the RFC-correct order, so it passes while
// the gap stands and fails the day the builder encodes the MP attribute first.

// VALIDATES: an IPv6 announcement built by BuildUnicast, the builder the static
// route path uses, carries its MP_REACH_NLRI as some path attribute, and the
// body asserts that attribute is the first one.
// PREVENTS: the RFC7606-5.1-1 gap staying published after the order changes.
// RFC requirement: RFC7606-5.1-1 gap -- the first path attribute of an IPv6 unicast UPDATE built by BuildUnicast with ORIGIN, AS_PATH and MED is MP_REACH_NLRI (type 14) or MP_UNREACH_NLRI (type 15).
func TestRFC7606Section51MPAttributeEncodedFirst(t *testing.T) {
	ub := NewUpdateBuilder(65001, false, true, false)
	params := UnicastParams{
		Prefix:  netip.MustParsePrefix("2001:db8:1::/48"),
		NextHop: netip.MustParseAddr("2001:db8::1"),
		Origin:  attribute.OriginIGP,
		ASPath:  []uint32{65002},
		MED:     10,
	}
	update := ub.BuildUnicast(&params)
	if update == nil {
		t.Fatal("BuildUnicast returned nil")
		return
	}

	// The walk runs outside the body: a malformed block or a missing
	// MP_REACH_NLRI is a defect of its own and MUST NOT read as the gap standing.
	codes, err := section51AttributeCodes(update.PathAttributes)
	if err != nil {
		t.Fatalf("path attribute block: %v", err)
		return
	}
	if !slices.Contains(codes, attribute.AttrMPReachNLRI) {
		t.Fatalf("no MP_REACH_NLRI among path attribute codes %v", codes)
		return
	}

	// RFC 7606 Section 5.1
	rfcgap.Demonstrate(t, "RFC7606-5.1-1", func(tb testing.TB) {
		first := codes[0]
		if first != attribute.AttrMPReachNLRI && first != attribute.AttrMPUnreachNLRI {
			tb.Errorf("first path attribute is type %d, want MP_REACH_NLRI (14) or MP_UNREACH_NLRI (15); order %v",
				first, codes)
		}
	})
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
