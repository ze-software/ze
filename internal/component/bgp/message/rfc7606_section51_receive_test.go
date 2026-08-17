package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func section51BaseAttrs(withNextHop bool) []byte {
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
	}
	if withNextHop {
		attrs = append(attrs, 0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01) // NEXT_HOP = 192.0.2.1
	}
	return attrs
}

func section51MPReach() []byte {
	return []byte{
		0x80, 0x0e, 0x1e, // MP_REACH_NLRI, value length 30
		0x00, 0x02, 0x01, 0x10, // IPv6 unicast, 16-byte next hop
		0x20, 0x01, 0x0d, 0xb8, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		0x00, // reserved
		0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01, 0x00, 0x00, // 2001:db8:1::/64
	}
}

func section51MPUnreach() []byte {
	return []byte{
		0x80, 0x0f, 0x0c, // MP_UNREACH_NLRI, value length 12
		0x00, 0x02, 0x01, // IPv6 unicast
		0x40, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x02, 0x00, 0x00, // 2001:db8:2::/64
	}
}

// VALIDATES: RFC 7606 Section 5.1 accepts MP_UNREACH_NLRI after other attributes.
// PREVENTS: a receive-side first-attribute restriction that applies the sender rule to an older peer.
// RFC requirement: RFC7606-5.1-3 positive -- MP_UNREACH_NLRI is accepted outside first position.
func TestRFC7606Section51AcceptsMPUnreachAfterOtherAttributes(t *testing.T) {
	attrs := append(section51BaseAttrs(false), section51MPUnreach()...)

	result := ValidateUpdateRFC7606(attrs, false, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// VALIDATES: RFC 7606 Section 5.1 accepts MP_REACH_NLRI and MP_UNREACH_NLRI together.
// PREVENTS: treating two distinct MP attributes as the duplicate-attribute error in Section 3.g.
// RFC requirement: RFC7606-5.1-3 positive -- distinct MP_REACH_NLRI and MP_UNREACH_NLRI attributes are accepted together.
func TestRFC7606Section51AcceptsReachAndUnreachTogether(t *testing.T) {
	attrs := append(section51BaseAttrs(false), section51MPReach()...)
	attrs = append(attrs, section51MPUnreach()...)

	result := ValidateUpdateRFC7606(attrs, false, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// VALIDATES: RFC 7606 Section 5.1 accepts MP_REACH_NLRI with the legacy NLRI field.
// PREVENTS: rejecting a mixed reachable-NLRI shape that an older speaker can send.
// RFC requirement: RFC7606-5.1-3 positive -- MP_REACH_NLRI is accepted with legacy NLRI.
func TestRFC7606Section51AcceptsMPReachWithLegacyNLRI(t *testing.T) {
	attrs := append(section51BaseAttrs(true), section51MPReach()...)

	result := ValidateUpdateRFC7606(attrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}

// VALIDATES: RFC 7606 Section 5.1 accepts MP_UNREACH_NLRI with the legacy NLRI field.
// PREVENTS: rejecting a mixed withdrawal-and-announcement shape that an older speaker can send.
// RFC requirement: RFC7606-5.1-3 positive -- MP_UNREACH_NLRI is accepted with legacy NLRI.
func TestRFC7606Section51AcceptsMPUnreachWithLegacyNLRI(t *testing.T) {
	attrs := append(section51BaseAttrs(true), section51MPUnreach()...)

	result := ValidateUpdateRFC7606(attrs, true, false, false)
	require.Equal(t, RFC7606ActionNone, result.Action)
}
