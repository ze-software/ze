// VALIDATES: spec-ospf-ext-2 RFC 3630 TE LSA body codec -- Router Address TLV and Link
// TLV with sub-TLVs 1-9 round-trip; every TLV is 4-octet aligned with padding excluded
// from Length; bandwidth is IEEE-754 single-precision bytes/sec stored as float64; the
// eight Unreserved values are ordered priority 0 first; the admin group is LSB=group 0;
// a malformed/truncated body never panics.
// PREVENTS: a padding slip that corrupts following TLVs, reading bandwidth as an integer
// or bits/sec, reversing the Unreserved order, or a decoder panic on untrusted input.
package packet

import (
	"math"
	"testing"
)

func TestTERouterAddressTLVRoundTrip(t *testing.T) {
	src := TELSA{IsRouterAddress: true, RouterAddress: [4]byte{192, 0, 2, 1}}
	body := src.Encode()
	// RFC 3630 sec 2.4.1: Router Address TLV is type 1, length 4 -> 8 octets total, aligned.
	if len(body) != 8 {
		t.Fatalf("router-address body len = %d, want 8", len(body))
	}
	if len(body)%4 != 0 {
		t.Fatalf("router-address body not 4-octet aligned: %d", len(body))
	}
	got, err := DecodeTELSA(body)
	if err != nil {
		t.Fatalf("DecodeTELSA: %v", err)
	}
	if !got.IsRouterAddress || got.IsLink || got.RouterAddress != src.RouterAddress {
		t.Fatalf("decoded = %+v, want router-address %v", got, src.RouterAddress)
	}
}

func fullTELink() TELink {
	return TELink{
		HasLinkType:      true,
		LinkType:         TELinkTypePointToPoint,
		HasLinkID:        true,
		LinkID:           [4]byte{2, 2, 2, 2},
		LocalIPs:         [][4]byte{{10, 0, 0, 1}},
		RemoteIPs:        [][4]byte{{10, 0, 0, 2}},
		HasTEMetric:      true,
		TEMetric:         100,
		HasMaxBandwidth:  true,
		MaxBandwidth:     1.25e9,
		HasMaxReservable: true,
		MaxReservable:    1.0e9,
		HasUnreserved:    true,
		Unreserved:       [8]float64{8e8, 7e8, 6e8, 5e8, 4e8, 3e8, 2e8, 1e8},
		HasAdminGroup:    true,
		AdminGroup:       0x80000001,
	}
}

func TestTELinkTLVRoundTrip(t *testing.T) {
	src := TELSA{IsLink: true, Link: fullTELink()}
	body := src.Encode()
	if len(body)%4 != 0 {
		t.Fatalf("link body not 4-octet aligned: %d", len(body))
	}
	got, err := DecodeTELSA(body)
	if err != nil {
		t.Fatalf("DecodeTELSA: %v", err)
	}
	if !got.IsLink {
		t.Fatalf("decoded not a link: %+v", got)
	}
	l := got.Link
	if !l.HasLinkType || l.LinkType != TELinkTypePointToPoint {
		t.Fatalf("link type = %v/%d, want p2p", l.HasLinkType, l.LinkType)
	}
	if !l.HasLinkID || l.LinkID != [4]byte{2, 2, 2, 2} {
		t.Fatalf("link id = %v", l.LinkID)
	}
	if len(l.LocalIPs) != 1 || l.LocalIPs[0] != [4]byte{10, 0, 0, 1} {
		t.Fatalf("local ips = %v", l.LocalIPs)
	}
	if len(l.RemoteIPs) != 1 || l.RemoteIPs[0] != [4]byte{10, 0, 0, 2} {
		t.Fatalf("remote ips = %v", l.RemoteIPs)
	}
	if !l.HasTEMetric || l.TEMetric != 100 {
		t.Fatalf("te metric = %v/%d", l.HasTEMetric, l.TEMetric)
	}
	if !l.HasAdminGroup || l.AdminGroup != 0x80000001 {
		t.Fatalf("admin group = %v/%#x", l.HasAdminGroup, l.AdminGroup)
	}
}

func TestTELinkTLVAlignment(t *testing.T) {
	// Link Type is a 1-octet value: Length=1 but the sub-TLV occupies 8 octets (4 header
	// + 1 value + 3 pad), pad excluded from Length (RFC 3630 sec 2.3.2). A single Local
	// IP is a 4-octet value (no pad). Both must round-trip and keep the body 4-aligned.
	for _, tc := range []struct {
		name string
		link TELink
	}{
		{"one-octet-linktype", TELink{HasLinkType: true, LinkType: TELinkTypeMultiAccess}},
		{"four-octet-linkid", TELink{HasLinkID: true, LinkID: [4]byte{9, 9, 9, 9}}},
		{"multi-4n-localips", TELink{LocalIPs: [][4]byte{{1, 1, 1, 1}, {2, 2, 2, 2}, {3, 3, 3, 3}}}},
	} {
		body := TELSA{IsLink: true, Link: tc.link}.Encode()
		if len(body)%4 != 0 {
			t.Fatalf("%s: body len %d not 4-octet aligned", tc.name, len(body))
		}
		got, err := DecodeTELSA(body)
		if err != nil || !got.IsLink {
			t.Fatalf("%s: decode err=%v isLink=%v", tc.name, err, got.IsLink)
		}
	}
	// Explicitly assert the on-wire Length of a 1-octet sub-TLV is 1 (not 4).
	body := TELSA{IsLink: true, Link: TELink{HasLinkType: true, LinkType: 1}}.Encode()
	// body = [Link TLV hdr type=2 len=8][sub hdr type=1 len=1][01][pad pad pad]
	subLen := int(body[6])<<8 | int(body[7]) // sub-TLV Length field
	if subLen != 1 {
		t.Fatalf("Link Type sub-TLV Length = %d, want 1 (pad excluded)", subLen)
	}
}

func TestTEBandwidthIEEERoundTrip(t *testing.T) {
	for _, want := range []float64{0, 1.25e9, 1e6, 12500000, 9.4e9} {
		src := TELSA{IsLink: true, Link: TELink{HasMaxBandwidth: true, MaxBandwidth: want}}
		got, err := DecodeTELSA(src.Encode())
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		// The wire is IEEE-754 single precision, so the decoded value equals the value
		// rounded to float32 and widened back to float64.
		if got.Link.MaxBandwidth != float64(float32(want)) {
			t.Fatalf("bandwidth %g round-trip = %g, want %g", want, got.Link.MaxBandwidth, float64(float32(want)))
		}
	}
}

func TestTEUnreservedBandwidthOrder(t *testing.T) {
	var vals [8]float64
	for i := range vals {
		vals[i] = float64((i + 1) * 100000000) // 8 distinct, increasing values by priority
	}
	src := TELSA{IsLink: true, Link: TELink{HasUnreserved: true, Unreserved: vals}}
	got, err := DecodeTELSA(src.Encode())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.Link.HasUnreserved {
		t.Fatalf("unreserved not decoded")
	}
	for i := range vals {
		if got.Link.Unreserved[i] != float64(float32(vals[i])) {
			t.Fatalf("unreserved[%d] = %g, want %g (priority order not preserved)", i, got.Link.Unreserved[i], float64(float32(vals[i])))
		}
	}
}

func TestTEAdminGroupBitNumbering(t *testing.T) {
	// RFC 3630 sec 2.5.9: LSB = group 0, MSB = group 31.
	mask := uint32(1)<<0 | uint32(1)<<31
	if !TEAdminGroupHasGroup(mask, 0) || !TEAdminGroupHasGroup(mask, 31) {
		t.Fatalf("group 0 and 31 must be set in %#x", mask)
	}
	if TEAdminGroupHasGroup(mask, 1) || TEAdminGroupHasGroup(mask, 30) {
		t.Fatalf("group 1 and 30 must be clear in %#x", mask)
	}
	got, err := DecodeTELSA(TELSA{IsLink: true, Link: TELink{HasAdminGroup: true, AdminGroup: mask}}.Encode())
	if err != nil || got.Link.AdminGroup != mask {
		t.Fatalf("admin group round-trip = %#x err=%v, want %#x", got.Link.AdminGroup, err, mask)
	}
}

func TestTEBodyMalformedNoPanic(t *testing.T) {
	// Each of these must return an error (or a clean decode) without panicking (AC-18/R-8).
	cases := [][]byte{
		{},                       // empty body: no top-level TLV
		{0x00},                   // truncated header
		{0x00, 0x02, 0x00, 0xff}, // Link TLV claims 255 bytes, none present
		{0x00, 0x01, 0x00, 0x02, 0x01, 0x02, 0x00, 0x00},             // Router Address value too short (2 < 4)
		{0x00, 0x02, 0x00, 0x08, 0x00, 0x08, 0x00, 0x20, 0, 0, 0, 0}, // Unreserved claims 32, truncated
		{0xff, 0xff, 0x00, 0x00},                                     // unknown top-level TLV, empty value
	}
	for i, body := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("case %d panicked: %v", i, r)
				}
			}()
			// The assertion is "no panic"; a returned error for malformed input is expected.
			if _, err := DecodeTELSA(body); err != nil {
				return
			}
		}()
	}
	// A well-formed but semantically-empty Link TLV must decode without panic or error.
	if _, err := DecodeTELSA(TELSA{IsLink: true, Link: TELink{}}.Encode()); err != nil {
		t.Fatalf("empty link decode: %v", err)
	}
}

func TestTELSABoundaries(t *testing.T) {
	// TE Metric (sub-TLV 5) is a full uint32: min 0 and max 4294967295 round-trip.
	for _, m := range []uint32{0, 4294967295} {
		got, err := DecodeTELSA(TELSA{IsLink: true, Link: TELink{HasTEMetric: true, TEMetric: m}}.Encode())
		if err != nil || got.Link.TEMetric != m {
			t.Fatalf("te-metric %d round-trip = %d err=%v", m, got.Link.TEMetric, err)
		}
	}
	// Administrative Group mask (sub-TLV 9) is a full 32-bit mask: min 0 and max round-trip.
	for _, ag := range []uint32{0, 0xFFFFFFFF} {
		got, err := DecodeTELSA(TELSA{IsLink: true, Link: TELink{HasAdminGroup: true, AdminGroup: ag}}.Encode())
		if err != nil || got.Link.AdminGroup != ag {
			t.Fatalf("admin-group %#x round-trip = %#x err=%v", ag, got.Link.AdminGroup, err)
		}
	}
	// Local Interface IP (sub-TLV 3) is 4N: a length that is not a multiple of 4 is malformed.
	badLocal := linkTLVWithRawSubForTest(TESubLocalInterfaceIP, make([]byte, 6))
	if _, err := DecodeTELSA(badLocal); err == nil {
		t.Fatalf("expected a length error for a 6-octet Local Interface IP sub-TLV")
	}
	// Unreserved Bandwidth (sub-TLV 8) is exactly 32 octets: 28 (< 8 floats) is malformed.
	badUnres := linkTLVWithRawSubForTest(TESubUnreservedBW, make([]byte, 28))
	if _, err := DecodeTELSA(badUnres); err == nil {
		t.Fatalf("expected a length error for a 28-octet Unreserved Bandwidth sub-TLV")
	}
	// Exactly 32 octets (8 floats) decodes.
	okUnres := linkTLVWithRawSubForTest(TESubUnreservedBW, make([]byte, 32))
	if _, err := DecodeTELSA(okUnres); err != nil {
		t.Fatalf("32-octet Unreserved Bandwidth sub-TLV rejected: %v", err)
	}
}

// TestTEFloatHelperMatchesStdlib guards the IEEE-754 helper against math.Float32bits.
func TestTEFloatHelperMatchesStdlib(t *testing.T) {
	b := teFloat32Bytes(1.25e9)
	if teReadFloat32(b) != float64(math.Float32frombits(math.Float32bits(1.25e9))) {
		t.Fatalf("IEEE float helper disagrees with stdlib")
	}
}
