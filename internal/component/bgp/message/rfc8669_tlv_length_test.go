// Design: docs/architecture/wire/attributes.md -- BGP Prefix-SID receive actions per family
// Overview: rfc7606.go -- validatePrefixSIDForRoutes, the post-walk Prefix-SID verdict

package message

import (
	"encoding/hex"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// VALIDATES: the receive walk judges the Label-Index and Originator SRGB TLV
// lengths, the presence of a Label-Index, and an empty attribute by the family
// of the routes (RFC 8669 Sections 3.1, 3.2, 4.1 and 6).
// PREVENTS: a malformed labeled-unicast Prefix-SID being relayed, and an
// unrecognized type 1 or 3 TLV off labeled unicast being discarded.
//
// The UPDATE fixtures below are built from hex so each octet a test turns on is
// visible. Every one carries ORIGIN and AS_PATH, one MP_REACH_NLRI naming the
// family, and attribute 40, in the order the test asks for.
const (
	psidOrigin = "40010100"
	psidASPath = "4002060201" + "0000fde9" // AS_SEQUENCE [65001], 4-octet AS

	// MP_REACH_NLRI value: AFI, SAFI, next hop, reserved, one NLRI.
	psidReachIPv4LU   = "000104" + "04c0000263" + "00" + "30000641c63364"                                   // 198.51.100.0/24 label 100
	psidReachIPv6LU   = "000204" + "1020010db8000000000000000000000001" + "00" + "5800064120010db800010000" // 2001:db8:1::/64 label 100
	psidReachIPv4VPN  = "000180" + "0c0000000000000000c0000263" + "00" + "70000641" + "00010000fde900000001" + "c63364"
	psidReachIPv6Uni  = "000201" + "1020010db8000000000000000000000001" + "00" + "3020010db80001"
	psidLabelIndex7   = "010007" + "00" + "0000" + "00000064"
	psidSRGB8         = "030008" + "0000" + "003e80" + "0003e8"
	psidSRGB14        = "03000e" + "0000" + "003e80" + "0003e8" + "004e20" + "0003e8"
	psidSIDInfoValue  = "00" + "20010db8000100010000000000000000" + "00" + "0048" + "00" // 21 octets
	psidL3ServiceTLV  = "050019" + "00" + "010015" + psidSIDInfoValue
	psidUnknownTLV    = "c80003aabbcc"
	psidPrefixSIDCode = "c028"
)

// psidTLV returns a TLV of type tlvType whose value is valueLen zero octets.
func psidTLV(tlvType byte, valueLen int) string {
	value := make([]byte, valueLen)
	return hex.EncodeToString([]byte{tlvType, byte(valueLen >> 8), byte(valueLen)}) + hex.EncodeToString(value)
}

// psidAttr wraps a hex value as attribute 40 (optional transitive).
func psidAttr(t *testing.T, value string) string {
	t.Helper()
	n := len(value) / 2
	require.Less(t, n, 256, "fixture needs the extended-length header")
	return psidPrefixSIDCode + hex.EncodeToString([]byte{byte(n)}) + value
}

// psidReach wraps a hex value as MP_REACH_NLRI (optional).
func psidReach(reach string) string {
	return "800e" + hex.EncodeToString([]byte{byte(len(reach) / 2)}) + reach
}

// psidValidate runs the receive walk over ORIGIN, AS_PATH, then MP_REACH_NLRI
// for reach and attribute 40 holding value, with attribute 40 first when
// prefixSIDFirst. An empty reach sends no MP_REACH_NLRI and no NLRI.
func psidValidate(t *testing.T, reach, value string, prefixSIDFirst bool) *RFC7606ValidationResult {
	t.Helper()
	attrs := psidOrigin + psidASPath
	sid := psidAttr(t, value)
	mp := ""
	if reach != "" {
		mp = psidReach(reach)
	}
	if prefixSIDFirst {
		attrs += sid + mp
	} else {
		attrs += mp + sid
	}
	raw, err := hex.DecodeString(attrs)
	require.NoError(t, err)
	return ValidateUpdateRFC7606AddPath(raw, false, false, true, func(uint16, uint8) bool { return false })
}

// psidRequireDiscard asserts attribute-discard of exactly code 40.
func psidRequireDiscard(t *testing.T, r *RFC7606ValidationResult) {
	t.Helper()
	require.Equal(t, RFC7606ActionAttributeDiscard, r.Action, "description %q", r.Description)
	require.Equal(t, attrCodePrefixSID, r.AttrCode)
	require.Len(t, r.DiscardEntries, 1)
	require.Equal(t, attrCodePrefixSID, r.DiscardEntries[0].Code)
}

var psidLabeledUnicast = map[string]string{"ipv4/labeled-unicast": psidReachIPv4LU, "ipv6/labeled-unicast": psidReachIPv6LU}

// TestRFC8669LabelIndexLengthMalformedOnLabeledUnicast feeds a Label-Index TLV
// one octet short and one octet long through the receive walk on both labeled
// unicast families, with attribute 40 before and after MP_REACH_NLRI.
//
// RFC requirement: RFC8669-6-1 negative -- on IPv4 and IPv6 labeled unicast a Label-Index TLV whose length is 6 or 8 makes the Prefix-SID malformed, and the receive walk answers attribute-discard of code 40 whichever of attribute 40 and MP_REACH_NLRI comes first.
func TestRFC8669LabelIndexLengthMalformedOnLabeledUnicast(t *testing.T) {
	for family, reach := range psidLabeledUnicast {
		for _, length := range []int{6, 8} {
			for _, first := range []bool{true, false} {
				t.Run(family+"/"+strconv.Itoa(length)+"/"+strconv.FormatBool(first), func(t *testing.T) {
					psidRequireDiscard(t, psidValidate(t, reach, psidTLV(1, length), first))
				})
			}
		}
	}
}

// TestRFC8669SRGBLengthMalformedOnLabeledUnicast feeds Originator SRGB TLVs of
// lengths 2 (no range), 3 and 9 beside a valid Label-Index on labeled unicast.
//
// RFC requirement: RFC8669-6-1 negative -- on labeled unicast an Originator SRGB TLV whose length is 2, 3 or 9 (not 2 + a non-zero multiple of 6) makes the Prefix-SID malformed, and the receive walk answers attribute-discard of code 40.
func TestRFC8669SRGBLengthMalformedOnLabeledUnicast(t *testing.T) {
	for family, reach := range psidLabeledUnicast {
		for _, length := range []int{2, 3, 9} {
			t.Run(family+"/"+strconv.Itoa(length), func(t *testing.T) {
				psidRequireDiscard(t, psidValidate(t, reach, psidLabelIndex7+psidTLV(3, length), false))
			})
		}
	}
}

// TestRFC8669ConformingTLVLengthsAcceptedOnLabeledUnicast is the positive half:
// the lengths the RFC fixes are accepted on labeled unicast.
//
// RFC requirement: RFC8669-6-1 positive -- on labeled unicast a Label-Index TLV of length 7, alone or with an Originator SRGB TLV of length 8 or 14, is not malformed: the receive walk answers no action.
// RFC requirement: RFC8669-4.1-1 positive -- a labeled-unicast Prefix-SID that contains the Label-Index TLV is accepted by the receive walk (no action).
func TestRFC8669ConformingTLVLengthsAcceptedOnLabeledUnicast(t *testing.T) {
	for family, reach := range psidLabeledUnicast {
		for name, value := range map[string]string{
			"label-index":         psidLabelIndex7,
			"label-index+srgb-8":  psidLabelIndex7 + psidSRGB8,
			"label-index+srgb-14": psidLabelIndex7 + psidSRGB14,
		} {
			t.Run(family+"/"+name, func(t *testing.T) {
				r := psidValidate(t, reach, value, true)
				require.Equal(t, RFC7606ActionNone, r.Action, "description %q", r.Description)
			})
		}
	}
}

var psidOffLabeledUnicast = map[string]string{"ipv4/vpn": psidReachIPv4VPN, "ipv6/unicast": psidReachIPv6Uni}

// TestRFC8669LabelIndexAnyLengthIgnoredOffLabeledUnicast feeds a 6-octet
// Label-Index on IPv4 VPN and IPv6 unicast routes.
//
// RFC requirement: RFC8669-3.1-2 positive -- off labeled unicast (IPv4 VPN, IPv6 unicast) a Label-Index TLV of length 6 is ignored: the receive walk answers no action, so the attribute is kept.
func TestRFC8669LabelIndexAnyLengthIgnoredOffLabeledUnicast(t *testing.T) {
	for family, reach := range psidOffLabeledUnicast {
		t.Run(family, func(t *testing.T) {
			r := psidValidate(t, reach, psidTLV(1, 6)+psidL3ServiceTLV, true)
			require.Equal(t, RFC7606ActionNone, r.Action, "description %q", r.Description)
		})
	}
}

// TestRFC8669SRGBAnyLengthIgnoredOffLabeledUnicast feeds a 3-octet Originator
// SRGB on an IPv4 VPN route.
//
// RFC requirement: RFC8669-3.2-4 positive -- off labeled unicast (IPv4 VPN) an Originator SRGB TLV of length 3 is ignored: the receive walk answers no action, so the attribute is kept.
func TestRFC8669SRGBAnyLengthIgnoredOffLabeledUnicast(t *testing.T) {
	r := psidValidate(t, psidReachIPv4VPN, psidTLV(3, 3)+psidL3ServiceTLV, false)
	require.Equal(t, RFC7606ActionNone, r.Action, "description %q", r.Description)
}

// TestRFC8669NoLabelIndexInvalidOnLabeledUnicast feeds a Prefix-SID holding
// only an Originator SRGB TLV.
//
// RFC requirement: RFC8669-4.1-1 negative -- a labeled-unicast Prefix-SID that does not contain the Label-Index TLV is not accepted: the receive walk answers attribute-discard of code 40.
// RFC requirement: RFC8669-4.1-2 negative -- on IPv4 and IPv6 labeled unicast a Prefix-SID with no Label-Index TLV is invalid, and the receive walk answers attribute-discard of code 40.
// RFC requirement: RFC8669-4.1-2 positive -- the same attribute on an IPv4 VPN route is kept (no action), since the Label-Index TLV is not required there.
func TestRFC8669NoLabelIndexInvalidOnLabeledUnicast(t *testing.T) {
	for family, reach := range psidLabeledUnicast {
		t.Run(family, func(t *testing.T) {
			psidRequireDiscard(t, psidValidate(t, reach, psidSRGB8, true))
		})
	}
	r := psidValidate(t, psidReachIPv4VPN, psidSRGB8, true)
	require.Equal(t, RFC7606ActionNone, r.Action, "description %q", r.Description)
}

// TestRFC8669EmptyAttributeMalformed feeds attribute 40 with no value.
//
// RFC requirement: RFC8669-6-1 negative -- an empty Prefix-SID attribute fails the minimum attribute length, and the receive walk answers attribute-discard of code 40 on labeled unicast and on IPv4 VPN.
func TestRFC8669EmptyAttributeMalformed(t *testing.T) {
	for family, reach := range map[string]string{"ipv4/labeled-unicast": psidReachIPv4LU, "ipv4/vpn": psidReachIPv4VPN} {
		t.Run(family, func(t *testing.T) {
			psidRequireDiscard(t, psidValidate(t, reach, "", false))
		})
	}
}
