// Related: register.go — the six families that had no splitter
// Related: flowspec.go, vpls.go, bgpls.go, cidr.go — the splitters under test
//
// VALIDATES: the families ze negotiates and decodes are carved into individual
// NLRIs, so they reach the RIB instead of being dropped at its door.
// PREVENTS: silent route loss. Every RIB entry point returns early on
// nlrisplit.Supported, so an unregistered family is accepted on the wire,
// decoded, and discarded with one Debug line and no red test.
package nlrisplit

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// assertSplit checks that data carves into exactly want, by value.
//
// is what TestTheNewSplittersCarryThePathIdentifier drives directly, and keeping
// the parameter here states that every family's walk reads it.
//
//nolint:unparam // addPath is half the Splitter contract: a caller passing true
func assertSplit(t *testing.T, fam family.Family, data []byte, addPath bool, want [][]byte) {
	t.Helper()

	got, err := Split(fam, data, addPath)
	if err != nil {
		t.Fatalf("%s: split returned %v", fam, err)
	}
	if len(got) != len(want) {
		t.Fatalf("%s: split gave %d NLRI(s), want %d: %v", fam, len(got), len(want), got)
	}
	for i := range want {
		if !bytes.Equal(got[i], want[i]) {
			t.Errorf("%s: NLRI %d is % x, want % x", fam, i, got[i], want[i])
		}
	}
}

// TestEverySilentlyDroppedFamilyNowSplits is the whole point of the six
// registrations. Before them each family below answered false to
// nlrisplit.Supported, which every RIB entry point reads as "return".
func TestEverySilentlyDroppedFamilyNowSplits(t *testing.T) {
	for _, fam := range []family.Family{
		{AFI: family.AFIIPv4, SAFI: family.SAFIVPN},
		{AFI: family.AFIIPv6, SAFI: family.SAFIVPN},
		{AFI: family.AFIIPv4, SAFI: family.SAFIRTC},
		{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec},
		{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec},
		{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpecVPN},
		{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpecVPN},
		{AFI: family.AFIL2VPN, SAFI: family.SAFIVPLS},
		{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState},
		{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkStateVPN},
	} {
		if !Supported(fam) {
			t.Errorf("%s has no splitter, so every route in it is decoded and then "+
				"dropped at the RIB door", fam)
		}
	}
}

// TestVPNSplitsOnALengthThatCountsLabelAndRD is the case the CIDR bound
// rejected: RFC 4364 Section 4.3.4 counts the label stack and the Route
// Distinguisher in the one length octet, so the value exceeds 128 for every
// real VPN route.
func TestVPNSplitsOnALengthThatCountsLabelAndRD(t *testing.T) {
	// 24 label bits + 64 RD bits + 24 prefix bits = 112.
	first := []byte{
		112,
		0x00, 0x01, 0x01, // label 16, bottom of stack
		0x00, 0x00, 0xfd, 0xe9, 0x00, 0x00, 0x00, 0x01, // RD 65001:1
		10, 0, 0, // 10.0.0.0/24
	}
	// 24 + 64 + 16 = 104: a /16, to prove the walk reads the length rather than
	// assuming a fixed body.
	second := []byte{
		104,
		0x00, 0x02, 0x01,
		0x00, 0x00, 0xfd, 0xe9, 0x00, 0x00, 0x00, 0x02,
		172, 16,
	}

	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}
	assertSplit(t, fam, concat(first, second), false, [][]byte{first, second})
}

// TestVPNLengthAbove128IsNotRejected is the discrimination case against the
// bound splitCIDR applies. Sharing that bound would fail every VPN route.
func TestVPNLengthAbove128IsNotRejected(t *testing.T) {
	nlri := append([]byte{200}, make([]byte, 25)...)

	if _, err := splitVPN(nlri, false); err != nil {
		t.Fatalf("a 200-bit VPN length was rejected: %v. The octet counts label, "+
			"Route Distinguisher and prefix bits together, so it passes 128 routinely", err)
	}
	if _, err := splitCIDR(nlri, false); err == nil {
		t.Fatal("splitCIDR accepted a 200-bit prefix length; the CIDR bound is what " +
			"makes the two walks different, so it must still refuse this")
	}
}

// TestRTCSplitsIncludingTheDefaultRoute covers RFC 4684 Section 4. The
// zero-length NLRI is the RTC default route, which subscribes to everything, so
// it must frame as a one-octet NLRI rather than read as an empty section.
func TestRTCSplitsIncludingTheDefaultRoute(t *testing.T) {
	defaultRoute := []byte{0}
	full := []byte{
		96,
		0x00, 0x00, 0xfd, 0xe9, // origin AS 65001
		0x00, 0x02, 0xfd, 0xe9, 0x00, 0x00, 0x00, 0x64, // route target
	}

	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIRTC}
	assertSplit(t, fam, concat(defaultRoute, full), false, [][]byte{defaultRoute, full})
}

// TestFlowSpecReadsTheExtendedLength is the case a one-octet read gets wrong.
// RFC 8955 Section 4: a length below 240 is one octet, and 240 or above is two
// octets whose high nibble is 0xf.
func TestFlowSpecReadsTheExtendedLength(t *testing.T) {
	short := append([]byte{5}, bytes.Repeat([]byte{0xaa}, 5)...)
	long := append([]byte{0xf1, 0x00}, bytes.Repeat([]byte{0xbb}, 256)...)

	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	assertSplit(t, fam, concat(short, long, short), false, [][]byte{short, long, short})
}

// TestFlowSpecOneOctetReadWouldMisframe pins why the nibble test matters. Read
// as one octet, the extended length 0xf100 says 241, so the next NLRI starts in
// the middle of this one and every following boundary is wrong.
func TestFlowSpecOneOctetReadWouldMisframe(t *testing.T) {
	long := append([]byte{0xf1, 0x00}, bytes.Repeat([]byte{0xbb}, 256)...)

	got, err := SplitFlowSpec(long, false)
	if err != nil {
		t.Fatalf("split returned %v", err)
	}
	if len(got) != 1 || len(got[0]) != len(long) {
		t.Fatalf("the extended length framed %d NLRI(s) of %d octets, want one of %d",
			len(got), len(got[0]), len(long))
	}
}

// TestVPLSReadsATwoOctetLength covers RFC 4761 Section 3.2.2. A one-octet read
// takes the high half of the length as the whole of it.
func TestVPLSReadsATwoOctetLength(t *testing.T) {
	body := []byte{
		0x00, 0x00, 0xfd, 0xe9, 0x00, 0x00, 0x00, 0x01, // RD
		0x00, 0x01, // VE-ID
		0x00, 0x01, // VE Block Offset
		0x00, 0x0a, // VE Block Size
		0x00, 0x01, 0x01, // Label Base
	}
	one := append([]byte{0x00, byte(len(body))}, body...)

	fam := family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIVPLS}
	assertSplit(t, fam, concat(one, one), false, [][]byte{one, one})
}

// TestBGPLSFramesByTotalLengthWhateverTheType covers RFC 9552 Section 5.1. A
// Propagator must carry an NLRI type it does not implement, so the walk reads
// the length alone.
func TestBGPLSFramesByTotalLengthWhateverTheType(t *testing.T) {
	node := append([]byte{0x00, 0x01, 0x00, 0x06}, bytes.Repeat([]byte{0x11}, 6)...)
	unknown := append([]byte{0x00, 0x63, 0x00, 0x04}, bytes.Repeat([]byte{0x22}, 4)...)

	for _, fam := range []family.Family{
		{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkState},
		{AFI: family.AFIBGPLS, SAFI: family.SAFIBGPLinkStateVPN},
	} {
		assertSplit(t, fam, concat(node, unknown), false, [][]byte{node, unknown})
	}
}

// TestTheNewSplittersCarryThePathIdentifier pins the ADD-PATH contract the
// Splitter type states: the 4-byte path identifier is part of the returned
// slice, because downstream consumers key on the exact wire bytes.
func TestTheNewSplittersCarryThePathIdentifier(t *testing.T) {
	pathID := []byte{0x00, 0x00, 0x00, 0x07}

	cases := map[string]struct {
		split Splitter
		body  []byte
	}{
		"flowspec": {SplitFlowSpec, append([]byte{3}, 0xaa, 0xbb, 0xcc)},
		"vpls":     {SplitVPLS, append([]byte{0x00, 0x03}, 0xaa, 0xbb, 0xcc)},
		"bgp-ls":   {SplitBGPLS, append([]byte{0x00, 0x01, 0x00, 0x03}, 0xaa, 0xbb, 0xcc)},
		"vpn":      {splitVPN, append([]byte{24}, 0xaa, 0xbb, 0xcc)},
	}

	for name, tc := range cases {
		want := concat(pathID, tc.body)
		got, err := tc.split(want, true)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if len(got) != 1 || !bytes.Equal(got[0], want) {
			t.Errorf("%s: split gave %v, want one NLRI of % x", name, got, want)
		}
	}
}

// TestAZeroLengthNLRIIsMalformedRatherThanAnInfiniteWalk is the guard on each
// length-framed splitter. A zero length advances the cursor by the header
// alone, so without this the walk either spins or fabricates NLRIs.
func TestAZeroLengthNLRIIsMalformedRatherThanAnInfiniteWalk(t *testing.T) {
	cases := map[string]struct {
		split Splitter
		data  []byte
	}{
		"flowspec": {SplitFlowSpec, []byte{0, 0, 0, 0}},
		"vpls":     {SplitVPLS, []byte{0x00, 0x00, 0x00, 0x00}},
		"bgp-ls":   {SplitBGPLS, []byte{0x00, 0x01, 0x00, 0x00}},
	}

	for name, tc := range cases {
		if _, err := tc.split(tc.data, false); err == nil {
			t.Errorf("%s accepted a zero-length NLRI instead of calling it malformed", name)
		}
	}
}

// TestATruncatedNLRIReturnsWhatWasParsed pins the Splitter contract's partial
// result: the caller decides what to do with the routes that did parse.
func TestATruncatedNLRIReturnsWhatWasParsed(t *testing.T) {
	good := append([]byte{0x00, 0x01, 0x00, 0x02}, 0xaa, 0xbb)
	truncated := []byte{0x00, 0x01, 0x00, 0x40, 0xcc}

	got, err := SplitBGPLS(concat(good, truncated), false)
	if err == nil {
		t.Fatal("a BGP-LS NLRI whose length runs past the section was accepted")
	}
	if len(got) != 1 || !bytes.Equal(got[0], good) {
		t.Fatalf("the NLRI parsed before the corruption was not returned: %v", got)
	}
}
