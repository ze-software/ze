// RFC: rfc/short/draft-ietf-bess-mup-safi.md -- gated requirement tests for the MUP SAFI
//
// Tests here bind draft-ietf-bess-mup-safi compliance-checklist ids to the MUP
// plugin behavior that produces them. Only what ze actually implements of the
// draft is bound: the BGP-MUP NLRI codec and its family scope. The MUP PE and
// MUP Controller procedures (routing-instance route targets, prefix-SID locator
// checks, session-state conversion) have no producer in ze and are annotated as
// gaps in the summary rather than tested here.

package mup

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"
)

// mpReachAFISAFI returns the (AFI, SAFI) of the MP_REACH_NLRI attribute in a
// packed UPDATE message, walking the path attributes rather than pattern
// matching bytes.
func mpReachAFISAFI(t *testing.T, update []byte) (uint16, uint8) {
	t.Helper()

	const (
		headerLen        = 19
		attrFlagExtLen   = 0x10
		attrCodeMPReach  = 14
		minAttrHeaderLen = 3
	)

	if len(update) < headerLen+4 {
		t.Fatalf("update too short: %d bytes", len(update))
	}
	pos := headerLen
	withdrawnLen := int(binary.BigEndian.Uint16(update[pos:]))
	pos += 2 + withdrawnLen
	if pos+2 > len(update) {
		t.Fatalf("update truncated before path attribute length")
	}
	attrLen := int(binary.BigEndian.Uint16(update[pos:]))
	pos += 2
	end := pos + attrLen
	if end > len(update) {
		t.Fatalf("path attribute length %d exceeds update size %d", attrLen, len(update))
	}

	for pos+minAttrHeaderLen <= end {
		flags := update[pos]
		code := update[pos+1]
		var valLen, hdrLen int
		if flags&attrFlagExtLen != 0 {
			valLen = int(binary.BigEndian.Uint16(update[pos+2:]))
			hdrLen = 4
		} else {
			valLen = int(update[pos+2])
			hdrLen = 3
		}
		val := update[pos+hdrLen : pos+hdrLen+valLen]
		if code == attrCodeMPReach {
			if len(val) < 3 {
				t.Fatalf("MP_REACH_NLRI value too short: %d bytes", len(val))
			}
			return binary.BigEndian.Uint16(val[0:2]), val[2]
		}
		pos += hdrLen + valLen
	}
	t.Fatal("no MP_REACH_NLRI attribute in update")
	return 0, 0
}

// TestRFCMUPFamiliesCoverBothAFIs verifies ze offers BGP-MUP under both the IPv4
// and IPv6 AFIs, so a PE and a MUP Controller can exchange BGP-MUP NLRIs for
// either AFI over one session.
//
// VALIDATES: ipv4/mup is (AFI 1, SAFI 85) and ipv6/mup is (AFI 2, SAFI 85), both
// families are registered by the plugin, and MUP NLRI decodes under both.
// PREVENTS: MUP shipping as an IPv4-only family, which would leave the IPv6 half
// of the MUP session with no NLRI codec.
func TestRFCMUPFamiliesCoverBothAFIs(t *testing.T) {
	t.Parallel()

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3-1 positive -- BGP-MUP is offered under both AFI 1 and AFI 2 with SAFI 85, so one session can exchange MUP NLRI for either AFI (Section 3.3)
	if IPv4MUP.AFI != AFIIPv4 || IPv4MUP.SAFI != SAFIMUP {
		t.Errorf("ipv4/mup = AFI %d SAFI %d, want AFI %d SAFI %d", IPv4MUP.AFI, IPv4MUP.SAFI, AFIIPv4, SAFIMUP)
	}
	if IPv6MUP.AFI != AFIIPv6 || IPv6MUP.SAFI != SAFIMUP {
		t.Errorf("ipv6/mup = AFI %d SAFI %d, want AFI %d SAFI %d", IPv6MUP.AFI, IPv6MUP.SAFI, AFIIPv6, SAFIMUP)
	}
	if SAFIMUP != 85 {
		t.Errorf("SAFI MUP = %d, want 85", SAFIMUP)
	}

	// The same ISD NLRI must decode under either MUP family.
	const isdHex = "0100010c0000fde9000000640a000001"
	for _, fam := range []string{"ipv4/mup", "ipv6/mup"} {
		got, err := DecodeNLRIHex(fam, isdHex)
		if err != nil {
			t.Fatalf("DecodeNLRIHex(%s) returned error: %v", fam, err)
		}
		m, ok := got.(map[string]any)
		if !ok {
			t.Fatalf("DecodeNLRIHex(%s) returned %T, want map", fam, got)
		}
		if m["route-type"] != int(MUPISD) {
			t.Errorf("DecodeNLRIHex(%s) route-type = %v, want %d", fam, m["route-type"], MUPISD)
		}
	}

	// Both AFIs encode a MUP NLRI from the same command shape.
	for _, fam := range []string{"ipv4/mup", "ipv6/mup"} {
		addr := "10.0.0.0/24"
		if strings.HasPrefix(fam, "ipv6/") {
			addr = "2001:db8::/64"
		}
		if _, err := EncodeNLRIHex(fam, []string{"route-type", "mup-isd", "rd", "100:100", "prefix", addr}); err != nil {
			t.Errorf("EncodeNLRIHex(%s) returned error: %v", fam, err)
		}
	}
}

// TestRFCMUPRejectsNonMUPFamily verifies the MUP codec refuses to operate under
// any family other than the two BGP-MUP families.
//
// VALIDATES: familyToAFI, and through it DecodeNLRIHex, rejects a family that is
// not (AFI 1, SAFI 85) or (AFI 2, SAFI 85).
// PREVENTS: MUP NLRI bytes being decoded under an unrelated SAFI, which would
// put MUP session state on a family the draft never scopes it to.
func TestRFCMUPRejectsNonMUPFamily(t *testing.T) {
	t.Parallel()

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3-1 negative -- the MUP codec rejects any family outside (AFI 1|2, SAFI 85), so BGP-MUP NLRI exchange is scoped to the two MUP families (Section 3.3)
	for _, fam := range []string{"ipv4/unicast", "ipv6/unicast", "l2vpn/evpn", "ipv4/flow", "mup", ""} {
		if _, err := DecodeNLRIHex(fam, "0100010c0000fde9000000640a000001"); err == nil {
			t.Errorf("DecodeNLRIHex(%q) accepted a non-MUP family", fam)
		}
	}
}

// TestRFCMUPUnknownRouteTypeIsNotRejected verifies a BGP-MUP NLRI carrying a
// route type ze does not know is parsed without error and, critically, that the
// declared Length is used to find the next NLRI in the same attribute.
//
// VALIDATES: ParseMUP accepts an unknown route type under the 3gpp-5g
// architecture type and returns the remaining bytes so the following NLRI is
// still reachable.
// PREVENTS: an unknown or future MUP route type turning into a parse failure
// that discards the well-known NLRIs alongside it.
func TestRFCMUPUnknownRouteTypeIsNotRejected(t *testing.T) {
	t.Parallel()

	// Two concatenated NLRIs: an unknown route type 0x0063 followed by an ISD.
	unknown := []byte{0x01, 0x00, 0x63, 0x04, 0xde, 0xad, 0xbe, 0xef}
	isdBytes, err := hex.DecodeString("0100010c0000fde9000000640a000001")
	if err != nil {
		t.Fatalf("fixture decode failed: %v", err)
	}
	data := append(append([]byte{}, unknown...), isdBytes...)

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.1-1 positive -- an unrecognized route type under the 3gpp-5g architecture type is consumed without error and the following NLRI is still parsed (Section 3.1)
	first, rest, err := ParseMUP(AFIIPv4, data)
	if err != nil {
		t.Fatalf("ParseMUP rejected an unknown route type: %v", err)
	}
	if first.RouteType() != MUPRouteType(0x63) {
		t.Errorf("route type = %d, want 99", first.RouteType())
	}
	if first.ArchType() != MUPArch3GPP5G {
		t.Errorf("arch type = %d, want %d", first.ArchType(), MUPArch3GPP5G)
	}
	if len(rest) != len(isdBytes) {
		t.Fatalf("remaining = %d bytes, want %d (the unknown NLRI must be skipped by its Length)", len(rest), len(isdBytes))
	}

	second, tail, err := ParseMUP(AFIIPv4, rest)
	if err != nil {
		t.Fatalf("ParseMUP of the following ISD returned error: %v", err)
	}
	if second.RouteType() != MUPISD {
		t.Errorf("second NLRI route type = %d, want %d", second.RouteType(), MUPISD)
	}
	if len(tail) != 0 {
		t.Errorf("tail = %d bytes, want 0", len(tail))
	}
}

// TestRFCMUPAnnounceUsesRouteAFIWithMUPSAFI verifies an announced MUP route is
// carried in MP_REACH_NLRI under the AFI of the route and SAFI 85.
//
// VALIDATES: EncodeRoute emits AFI 1 for ipv4/mup and AFI 2 for ipv6/mup, both
// with SAFI 85, for the Type 1 ST route type the controller advertises.
// PREVENTS: MUP routes being announced under a mismatched AFI or a non-MUP SAFI,
// which peers scoped to (AFI, 85) would never accept.
func TestRFCMUPAnnounceUsesRouteAFIWithMUPSAFI(t *testing.T) {
	t.Parallel()

	cases := []struct {
		family string
		cmd    string
		afi    uint16
	}{
		{"ipv4/mup", "mup-t1st 192.168.0.2/32 rd 100:100 teid 12345 qfi 9 endpoint 10.0.0.1 next-hop 10.0.0.2", 1},
		{"ipv6/mup", "mup-t1st 2001:db8:1:1::2/128 rd 100:100 teid 12345 qfi 9 endpoint 2001::1 next-hop 2001::2", 2},
	}

	// RFC requirement: DRAFT-IETF-BESS-MUP-SAFI-3.3.7-2 positive -- an advertised Type 1 ST route is announced with the AFI of the route and the BGP-MUP SAFI 85 (Section 3.3.7)
	for _, tc := range cases {
		update, nlriBytes, err := EncodeRoute(tc.cmd, tc.family, 65000, true, true, false)
		if err != nil {
			t.Fatalf("EncodeRoute(%s) returned error: %v", tc.family, err)
		}
		if len(nlriBytes) == 0 {
			t.Fatalf("EncodeRoute(%s) produced no NLRI", tc.family)
		}
		afi, safi := mpReachAFISAFI(t, update)
		if afi != tc.afi {
			t.Errorf("%s: MP_REACH AFI = %d, want %d", tc.family, afi, tc.afi)
		}
		if safi != uint8(SAFIMUP) {
			t.Errorf("%s: MP_REACH SAFI = %d, want %d", tc.family, safi, SAFIMUP)
		}
	}
}

// TestRFCMUPWithdrawalNLRIKeepsMUPFamilyAndBytes verifies a MUP NLRI reports the
// MUP family and re-encodes to its exact received bytes. The family-generic
// withdrawal encoder (internal/component/bgp/reactor/peer_rib_routes.go:170)
// would need both properties to build an MP_UNREACH_NLRI withdrawal of a Type 1
// ST or Type 2 ST route, but no ze path hands it a SAFI 85 NLRI: its callers
// read from the PeerOpWithdraw queue, and neither withdrawal entry point parses
// SAFI 85 (internal/component/bgp/plugins/cmd/update/update_text_nlri.go:375-403,
// internal/component/bgp/plugins/cmd/announce/announce.go:257).
//
// VALIDATES: MUP.Family() carries the route AFI with SAFI 85 and MUP.WriteTo
// reproduces the parsed wire bytes for both session-transformed route types.
// PREVENTS: a withdrawal being emitted under the wrong family, or with NLRI
// bytes that do not match the advertised route, so the peer never removes it.
func TestRFCMUPWithdrawalNLRIKeepsMUPFamilyAndBytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		family string
		args   []string
		afi    AFI
		rt     MUPRouteType
	}{
		{
			name:   "t1st ipv4",
			family: "ipv4/mup",
			args:   strings.Fields("route-type mup-t1st rd 100:100 prefix 192.168.0.2/32 teid 12345 qfi 9 endpoint 10.0.0.1"),
			afi:    AFIIPv4,
			rt:     MUPT1ST,
		},
		{
			name:   "t2st ipv4",
			family: "ipv4/mup",
			args:   strings.Fields("route-type mup-t2st rd 100:100 address 10.0.0.1 teid 12345/32"),
			afi:    AFIIPv4,
			rt:     MUPT2ST,
		},
		{
			name:   "t1st ipv6",
			family: "ipv6/mup",
			args:   strings.Fields("route-type mup-t1st rd 100:100 prefix 2001:db8:1:1::2/128 teid 12345 qfi 9 endpoint 2001::1"),
			afi:    AFIIPv6,
			rt:     MUPT1ST,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			nlriHex, err := EncodeNLRIHex(tc.family, tc.args)
			if err != nil {
				t.Fatalf("EncodeNLRIHex returned error: %v", err)
			}
			wire, err := hex.DecodeString(nlriHex)
			if err != nil {
				t.Fatalf("hex decode failed: %v", err)
			}

			// NOT an RFC coverage claim for DRAFT-IETF-BESS-MUP-SAFI-3.3.8-1 or
			// -3.3.11-1: no withdrawal is emitted here, and no ze code path can hand a
			// SAFI 85 NLRI to the MP_UNREACH encoder at all (see the {gap} annotations
			// on both requirements). This pins the codec property a withdrawal would
			// depend on if the emission path existed.
			parsed, rest, err := ParseMUP(tc.afi, wire)
			if err != nil {
				t.Fatalf("ParseMUP returned error: %v", err)
			}
			if len(rest) != 0 {
				t.Errorf("rest = %d bytes, want 0", len(rest))
			}
			if parsed.RouteType() != tc.rt {
				t.Errorf("route type = %d, want %d", parsed.RouteType(), tc.rt)
			}
			if fam := parsed.Family(); fam.AFI != tc.afi || fam.SAFI != SAFIMUP {
				t.Errorf("family = AFI %d SAFI %d, want AFI %d SAFI %d", fam.AFI, fam.SAFI, tc.afi, SAFIMUP)
			}
			buf := make([]byte, parsed.Len())
			n := parsed.WriteTo(buf, 0)
			if n != len(wire) || !bytes.Equal(buf[:n], wire) {
				t.Errorf("re-encoded NLRI = %x, want %x", buf[:n], wire)
			}
		})
	}
}
