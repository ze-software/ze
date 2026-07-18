// VALIDATES: RFC 5701 -- the IPv6 Address Specific Extended Community attribute
// (code 25) is optional-transitive, each community is exactly 20 octets, and the
// attribute length must be a multiple of 20 octets.
// PREVENTS: mis-flagging the attribute (so an unrecognized receiver resets the
// session instead of passing it through), a wrong per-community stride (the
// 8-octet RFC 4360 stride), or accepting a truncated community list.
package attribute

import (
	"errors"
	"testing"
)

// RFC requirement: RFC5701-2-1 positive -- the IPv6 Address Specific Extended
// Community attribute is optional AND transitive (RFC 5701 sec 2): Flags()
// (internal/core/bgp/attribute/community.go:483) returns FlagOptional |
// FlagTransitive, so a speaker that does not recognize code 25 passes it through
// transitively rather than treating it as a well-known-mandatory attribute error.
// RFC requirement: RFC5701-2-1 negative -- the attribute is NOT the optional
// NON-transitive form (0x80) and NOT a well-known attribute (Optional cleared),
// and the Partial bit is not set on origination: the encoded flag byte is exactly
// 0xC0, never 0x80 / 0x00 / a partial-set variant.
func TestRFC5701IPv6ExtCommunityFlags(t *testing.T) {
	t.Parallel()
	ecs := IPv6ExtendedCommunities{IPv6ExtendedCommunity{}}
	f := ecs.Flags()

	if !f.IsOptional() {
		t.Errorf("Flags()=%#x: Optional bit not set; an unrecognized receiver would reject code 25 as a mandatory-attribute error", byte(f))
	}
	if !f.IsTransitive() {
		t.Errorf("Flags()=%#x: Transitive bit not set; the community would not propagate through non-supporting speakers", byte(f))
	}
	// negative: reject the wrong encodings the MUST forbids.
	if f == FlagOptional {
		t.Errorf("Flags()=%#x is optional NON-transitive (0x80); RFC 5701 sec 2 requires transitive", byte(f))
	}
	if f.IsPartial() {
		t.Errorf("Flags()=%#x sets the Partial bit on origination; RFC 5701 does not", byte(f))
	}
	if f != FlagOptional|FlagTransitive {
		t.Errorf("Flags()=%#x, want exactly Optional|Transitive (0xC0)", byte(f))
	}
}

// RFC requirement: RFC5701-2-2 positive -- each IPv6 Address Specific extended
// community is encoded as exactly 20 octets (RFC 5701 sec 2): the type is a
// [20]byte (community.go:469), Len() reports 20*count, and WriteTo lays each
// community down on a 20-octet stride (community.go:486-494).
// RFC requirement: RFC5701-2-2 negative -- the per-community stride is 20 octets,
// NOT the 8-octet stride of a (non-IPv6) RFC 4360 Extended Community: two
// communities occupy 40 octets and the second begins at offset 20, never 8 or 16.
func TestRFC5701IPv6ExtCommunityTwentyOctets(t *testing.T) {
	t.Parallel()
	var a IPv6ExtendedCommunity
	var b IPv6ExtendedCommunity
	for i := range b {
		b[i] = 0xBB
	}
	ecs := IPv6ExtendedCommunities{a, b}

	if got := ecs.Len(); got != 40 {
		t.Fatalf("Len()=%d for 2 communities, want 40 (2*20)", got)
	}
	buf := make([]byte, 64)
	n := ecs.WriteTo(buf, 0)
	if n != 40 {
		t.Fatalf("WriteTo wrote %d octets for 2 communities, want 40 (2*20)", n)
	}
	// negative: the SECOND community must start at offset 20 (20-octet stride),
	// not the 8-octet stride of an RFC 4360 extended community.
	if buf[8] == 0xBB || buf[16] == 0xBB {
		t.Fatalf("second community appears before offset 20 (stride != 20): buf[8]=%#x buf[16]=%#x", buf[8], buf[16])
	}
	if buf[20] != 0xBB {
		t.Fatalf("second community does not start at offset 20; buf[20]=%#x, want 0xBB", buf[20])
	}
}

// RFC requirement: RFC5701-2-3 positive -- the attribute length must be a
// multiple of 20 octets (RFC 5701 sec 2): ParseIPv6ExtendedCommunities
// (community.go:514) accepts a 40-octet buffer as exactly 2 communities.
// RFC requirement: RFC5701-2-3 negative -- a length that is not a multiple of 20
// (19, 21, an 8-octet RFC 4360 extended community, 39, 41) is rejected with
// ErrInvalidLength (community.go:515-516) rather than silently truncated.
func TestRFC5701IPv6ExtCommunityLengthMultipleOf20(t *testing.T) {
	t.Parallel()
	ecs, err := ParseIPv6ExtendedCommunities(make([]byte, 40))
	if err != nil {
		t.Fatalf("valid 40-octet (2*20) buffer rejected: %v", err)
	}
	if len(ecs) != 2 {
		t.Fatalf("40-octet buffer parsed into %d communities, want 2", len(ecs))
	}

	for _, n := range []int{19, 21, 8, 39, 41} {
		_, err := ParseIPv6ExtendedCommunities(make([]byte, n))
		if !errors.Is(err, ErrInvalidLength) {
			t.Errorf("length %d (not a multiple of 20): err = %v, want ErrInvalidLength", n, err)
		}
	}
}
