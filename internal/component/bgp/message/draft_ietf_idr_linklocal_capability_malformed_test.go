package message

import (
	"encoding/hex"
	"testing"
)

// Next Hop halves, 16 octets each, written out so a reader checks the bytes the
// validator is given rather than a parser's idea of them.
const (
	llnhGlobal      = "20010DB8000900000000000000000009" // 2001:db8:9::9
	llnhLinkLocal   = "FE800000000000000000000000000009" // fe80::9
	llnhUnspecified = "00000000000000000000000000000000" // ::
	llnhMulticast   = "FF020000000000000000000000000001" // ff02::1
)

// llnhUpdateAttrs answers the path attributes of an IPv6 unicast UPDATE whose
// MP_REACH_NLRI carries nextHop (hex) under the Next Hop Length nextHopLen (hex
// octet) and announces 2001:db8:5701::/48:
//
//	ORIGIN IGP:           40 01 01 00
//	AS_PATH [65001]:      40 02 06 02 01 0000FDE9   (4-octet ASNs)
//	MP_REACH_NLRI:        80 0E <len> 0002 01 <nhLen> <next hop> 00 30 20010DB85701
func llnhUpdateAttrs(t *testing.T, nextHopLen, nextHop string) []byte {
	t.Helper()
	nextHopBytes, err := hex.DecodeString(nextHop)
	if err != nil {
		t.Fatalf("next hop hex %q: %v", nextHop, err)
	}
	// AFI(2) + SAFI(1) + Next Hop Length(1) + next hop + Reserved(1) + NLRI(7).
	valueOctets := 2 + 1 + 1 + len(nextHopBytes) + 1 + 7
	attrs := "40010100" + "4002060201" + "0000FDE9" +
		"800E" + hex.EncodeToString([]byte{byte(valueOctets)}) +
		"000201" + nextHopLen + nextHop + "00" + "3020010DB85701"
	raw, err := hex.DecodeString(attrs)
	if err != nil {
		t.Fatalf("attribute hex %q: %v", attrs, err)
	}
	return raw
}

// VALIDATES: on a session that negotiated the Link-Local Next Hop capability, an
// UPDATE whose 32-octet Next Hop field is not a Global (or ::) followed by a
// Link-Local, nor the two-Link-Local form Section 6 tells receivers to use, is
// treat-as-withdraw, and the verdict names the draft and MP_REACH_NLRI.
//
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1 negative -- "If the Next
// Hop field is malformed, the implementation MUST handle the malformed UPDATE
// message using the approach of "treat-as- withdraw", as described in section 7.3
// of [RFC7606]" (Section 6). validateLinkLocalNextHopPair (rfc7606_linklocal.go),
// called from the attribute walk when the session negotiated capability 77, finds
// each malformed field below and records treat-as-withdraw.
//
// PREVENTS: a capability-77 session installing a route whose next hop puts the
// Link-Local first, carries no Link-Local at all, or opens with an address that
// is not a Global unicast one.
func TestLinkLocalCapabilityMalformedThirtyTwoOctet(t *testing.T) {
	cases := []struct {
		name    string
		nextHop string
	}{
		{"link-local then global", llnhLinkLocal + llnhGlobal},
		{"global then global", llnhGlobal + llnhGlobal},
		{"global then unspecified", llnhGlobal + llnhUnspecified},
		{"multicast then link-local", llnhMulticast + llnhLinkLocal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			attrs := llnhUpdateAttrs(t, "20", tc.nextHop)
			result := ValidateUpdateRFC7606AddPath(attrs, false, false, true, nil, true)
			if result.Action != RFC7606ActionTreatAsWithdraw {
				t.Fatalf("action = %v (%q), want treat-as-withdraw", result.Action, result.Description)
			}
			if result.AttrCode != attrCodeMPReachNLRI {
				t.Fatalf("attribute code = %d, want MP_REACH_NLRI (%d)", result.AttrCode, attrCodeMPReachNLRI)
			}
			const want = "draft-ietf-idr-linklocal-capability Section 6: 32-octet Next Hop is not a Global followed by a Link-Local"
			if result.Description != want {
				t.Fatalf("description = %q, want %q", result.Description, want)
			}
		})
	}
}

// VALIDATES: on a session that negotiated the Link-Local Next Hop capability, the
// three 32-octet forms Section 6 admits are accepted: Global then Link-Local (the
// RFC 2545 layout), :: then Link-Local, and two Link-Locals.
//
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1 positive -- the
// treat-as-withdraw is confined to a malformed field. A conforming pair is
// accepted, and so are the two forms the same section tells a receiver to use:
// "When the first IPv6 address is 0::0/0 and the second IPv6 address is an IPv6
// Link-Local address, the second address is used for forwarding", and
// "Receivers SHOULD use the second Link-Local IPv6 address for forwarding" for a
// field carrying two Link-Locals (Section 6).
//
// PREVENTS: a check that withdraws every 32-octet next hop, or the BIRD-style
// two-Link-Local field the draft says is observed in deployment.
func TestLinkLocalCapabilityWellFormedThirtyTwoOctet(t *testing.T) {
	cases := []struct {
		name    string
		nextHop string
	}{
		{"global then link-local", llnhGlobal + llnhLinkLocal},
		{"unspecified then link-local", llnhUnspecified + llnhLinkLocal},
		{"link-local then link-local", llnhLinkLocal + llnhLinkLocal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			attrs := llnhUpdateAttrs(t, "20", tc.nextHop)
			result := ValidateUpdateRFC7606AddPath(attrs, false, false, true, nil, true)
			if result.Action != RFC7606ActionNone {
				t.Fatalf("action = %v (%q), want none", result.Action, result.Description)
			}
		})
	}
}

// VALIDATES: the address check binds only a session that negotiated capability 77,
// as Section 6 opens: "These procedures apply only when the Link-Local Next Hop
// Capability has been negotiated for a BGP session." Without it, a 32-octet field
// is judged by its length alone, as before. A 16-octet field is not the subject of
// the check on either kind of session.
// PREVENTS: the draft's procedure leaking onto a plain RFC 2545 session.
func TestLinkLocalCapabilityNotNegotiatedJudgesLengthOnly(t *testing.T) {
	for _, nextHop := range []string{llnhLinkLocal + llnhGlobal, llnhGlobal + llnhGlobal} {
		attrs := llnhUpdateAttrs(t, "20", nextHop)
		result := ValidateUpdateRFC7606AddPath(attrs, false, false, true, nil, false)
		if result.Action != RFC7606ActionNone {
			t.Fatalf("next hop %s without capability 77: action = %v (%q), want none", nextHop, result.Action, result.Description)
		}
	}
	for _, negotiated := range []bool{false, true} {
		attrs := llnhUpdateAttrs(t, "10", llnhLinkLocal)
		result := ValidateUpdateRFC7606AddPath(attrs, false, false, true, nil, negotiated)
		if result.Action != RFC7606ActionNone {
			t.Fatalf("16-octet next hop, capability 77 %v: action = %v (%q), want none", negotiated, result.Action, result.Description)
		}
	}
}

// VALIDATES: a Next Hop Length that is not valid for IPv6 unicast still ends in a
// session reset on a capability-77 session (RFC 7606 Section 7.11), so the address
// check never turns a length fault into a treat-as-withdraw.
// PREVENTS: a 33-octet field being read as a malformed pair and withdrawn, which
// leaves the NLRI unlocatable.
func TestLinkLocalCapabilityWrongLengthStillResets(t *testing.T) {
	attrs := llnhUpdateAttrs(t, "21", llnhGlobal+llnhLinkLocal+"00")
	result := ValidateUpdateRFC7606AddPath(attrs, false, false, true, nil, true)
	if result.Action != RFC7606ActionSessionReset {
		t.Fatalf("action = %v (%q), want session reset", result.Action, result.Description)
	}
}
