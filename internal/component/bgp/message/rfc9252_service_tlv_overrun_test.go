// Design: docs/architecture/wire/attributes.md -- BGP Prefix-SID receive actions per family
// Overview: rfc7606.go -- validatePrefixSIDForRoutes, the post-walk Prefix-SID verdict
// Related: rfc8669_tlv_length_test.go -- the fixtures these tests share

package message

import (
	"encoding/hex"
	"maps"
	"testing"

	"github.com/stretchr/testify/require"
)

// VALIDATES: a malformed SRv6 Service TLV, including one whose length disagrees
// with attribute 40, is treat-as-withdraw (RFC 9252 Section 7), and is escalated
// to session reset when the UPDATE has no NLRI (RFC 7606 Section 5.2).
// PREVENTS: an SRv6 route installed without its Service SID after an
// attribute-discard RFC 7606 Section 2 forbids.

// psidOverrun returns a TLV header of type tlvType declaring 40 octets, followed
// by only 4, so the TLV runs past the end of the attribute.
func psidOverrun(tlvType byte) string {
	return hex.EncodeToString([]byte{tlvType, 0, 40}) + "00000000"
}

// TestRFC9252ServiceTLVOverrunTreatAsWithdraw feeds SRv6 L3 and L2 Service TLVs
// whose length runs past the attribute, and, as a paired control, a Label-Index
// and an unknown TLV that do the same.
//
// RFC requirement: RFC9252-3.4-1 negative -- an SRv6 L3 (type 5) or L2 (type 6) Service TLV whose length is inconsistent with the length of attribute 40 is a malformed Service TLV, and the receive walk answers exactly treat-as-withdraw for code 40 on IPv4 VPN and IPv6 unicast, while a type 1 or unknown TLV that overruns stays attribute-discard.
func TestRFC9252ServiceTLVOverrunTreatAsWithdraw(t *testing.T) {
	for family, reach := range psidOffLabeledUnicast {
		for _, tlvType := range []byte{5, 6} {
			t.Run(family+"/service-"+string('0'+tlvType), func(t *testing.T) {
				r := psidValidate(t, reach, psidOverrun(tlvType), false)
				require.Equal(t, RFC7606ActionTreatAsWithdraw, r.Action, "description %q", r.Description)
				require.Equal(t, attrCodePrefixSID, r.AttrCode)
				require.Contains(t, r.Description, "RFC 9252 Section 7")
			})
		}
		for _, tlvType := range []byte{1, 200} {
			t.Run(family+"/control", func(t *testing.T) {
				r := psidValidate(t, reach, psidOverrun(tlvType), false)
				psidRequireDiscard(t, r)
				require.Equal(t, "RFC 8669 Section 6: Prefix-SID TLV length exceeds attribute bounds", r.Description)
			})
		}
	}
}

// TestRFC9252ZeroLengthServiceTLVTreatAsWithdraw feeds SRv6 L3 and L2 Service
// TLVs of length 0 on every family, and, as the boundary control, the same TLVs
// of length 1 (the RESERVED octet alone), which are well-formed. On labeled
// unicast a conforming Label-Index rides along, so RFC 8669 Section 4.1 has
// nothing to refuse and only the Service TLV decides the action.
// RFC 9252 Section 7: "An SRv6 Service TLV is considered malformed in the
// following cases: * The TLV Length is less than 1."
//
// RFC requirement: RFC9252-3.4-1 negative -- an SRv6 L3 (type 5) or L2 (type 6) Service TLV whose length is 0 is a malformed Service TLV, and the receive walk answers exactly treat-as-withdraw for code 40 on IPv4 VPN, IPv6 unicast and IPv4/IPv6 labeled unicast, while the same TLV of length 1 answers no action.
func TestRFC9252ZeroLengthServiceTLVTreatAsWithdraw(t *testing.T) {
	families := maps.Clone(psidOffLabeledUnicast)
	maps.Copy(families, psidLabeledUnicast)
	for family, reach := range families {
		labelIndex := ""
		if _, labeled := psidLabeledUnicast[family]; labeled {
			labelIndex = psidLabelIndex7
		}
		for _, tlvType := range []byte{5, 6} {
			name := family + "/service-" + string('0'+tlvType)
			t.Run(name+"/length-0", func(t *testing.T) {
				r := psidValidate(t, reach, labelIndex+psidTLV(tlvType, 0), false)
				require.Equal(t, RFC7606ActionTreatAsWithdraw, r.Action, "description %q", r.Description)
				require.Equal(t, attrCodePrefixSID, r.AttrCode)
				require.Contains(t, r.Description, "RFC 9252 Section 7")
			})
			t.Run(name+"/length-1", func(t *testing.T) {
				r := psidValidate(t, reach, labelIndex+psidTLV(tlvType, 1), false)
				require.Equal(t, RFC7606ActionNone, r.Action, "description %q", r.Description)
			})
		}
	}
}

// TestRFC9252ServiceTLVOverrunWithoutNLRIResetsSession sends the overrunning
// Service TLV in an UPDATE that carries no NLRI at all. RFC 7606 Section 5.2: an
// error whose action is stronger than attribute-discard in an UPDATE with no
// reachable NLRI MUST be a session reset.
func TestRFC9252ServiceTLVOverrunWithoutNLRIResetsSession(t *testing.T) {
	r := psidValidate(t, "", psidOverrun(5), false)
	require.Equal(t, RFC7606ActionSessionReset, r.Action, "description %q", r.Description)
	require.Equal(t, attrCodePrefixSID, r.AttrCode)
}

// TestRFC9252SIDInformationStrayOctetsTreatAsWithdraw puts 1 and 2 octets after
// the last Sub-Sub-TLV of a SID Information Sub-TLV whose length covers them.
//
// RFC requirement: RFC9252-3.4-1 negative -- a SID Information Sub-TLV whose last 1 or 2 octets frame no Sub-Sub-TLV holds a malformed Sub-Sub-TLV, so the SRv6 Service TLV is malformed and the receive walk answers treat-as-withdraw for code 40.
func TestRFC9252SIDInformationStrayOctetsTreatAsWithdraw(t *testing.T) {
	for _, stray := range []string{"ee", "eeee"} {
		t.Run(stray, func(t *testing.T) {
			info := psidSIDInfoValue + "010006" + "201810000000" + stray
			subTLV := "01" + hex.EncodeToString([]byte{0, byte(len(info) / 2)}) + info
			service := "00" + subTLV
			value := "05" + hex.EncodeToString([]byte{0, byte(len(service) / 2)}) + service
			r := psidValidate(t, psidReachIPv4VPN, value, true)
			require.Equal(t, RFC7606ActionTreatAsWithdraw, r.Action, "description %q", r.Description)
			require.Equal(t, attrCodePrefixSID, r.AttrCode)
		})
	}
}

// TestRFC7606PrefixSIDFramingErrorAfterServiceTLV pins the owner's Q6 (a)
// reading off labeled unicast: a framing error met after a valid SRv6 Service
// TLV is that Service TLV's length disagreeing with the attribute, so the
// routes are withdrawn; with no Service TLV the same errors stay
// attribute-discard.
//
// RFC requirement: RFC9252-3.4-1 negative -- on IPv4 VPN a valid L3 Service TLV followed by an unknown TLV overrunning attribute 40, or by 2 trailing octets, is a malformed Service TLV: treat-as-withdraw for code 40; with no Service TLV the same framing errors answer attribute-discard.
func TestRFC7606PrefixSIDFramingErrorAfterServiceTLV(t *testing.T) {
	for name, value := range map[string]string{
		"unknown TLV overrun": psidL3ServiceTLV + psidOverrun(200),
		"two trailing octets": psidL3ServiceTLV + "0000",
	} {
		t.Run(name, func(t *testing.T) {
			r := psidValidate(t, psidReachIPv4VPN, value, false)
			require.Equal(t, RFC7606ActionTreatAsWithdraw, r.Action, "description %q", r.Description)
			require.Equal(t, attrCodePrefixSID, r.AttrCode)
		})
	}
	for name, value := range map[string]string{
		"unknown TLV overrun, no service": psidUnknownTLV + psidOverrun(200),
		"two trailing octets, no service": psidUnknownTLV + "0000",
	} {
		t.Run(name, func(t *testing.T) {
			psidRequireDiscard(t, psidValidate(t, psidReachIPv4VPN, value, false))
		})
	}
}
