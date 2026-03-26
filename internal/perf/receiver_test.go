package perf

import (
	"encoding/binary"
	"net/netip"
	"testing"
)

// buildOriginAttr returns a well-known mandatory ORIGIN attribute (IGP).
// flags=0x40 (transitive), type=1, len=1, value=0 (IGP).
func buildOriginAttr() []byte {
	return []byte{0x40, 0x01, 0x01, 0x00}
}

// buildNextHopAttr returns a NEXT_HOP attribute with the given IPv4 address.
// flags=0x40 (transitive), type=3, len=4, value=4 bytes.
func buildNextHopAttr(a, b, c, d byte) []byte {
	return []byte{0x40, 0x03, 0x04, a, b, c, d}
}

// buildASPathAttr returns an empty AS_PATH attribute.
// flags=0x40 (transitive), type=2, len=0.
func buildASPathAttr() []byte {
	return []byte{0x40, 0x02, 0x00}
}

// buildMPReachIPv4 returns an MP_REACH_NLRI attribute for IPv4/unicast with
// the given next-hop and NLRI bytes.
// flags=0xC0 (optional+transitive), type=14.
func buildMPReachIPv4(nhIP [4]byte, nlri []byte) []byte {
	// AFI=1 (2 bytes) + SAFI=1 (1 byte) + NH_len=4 (1 byte) + NH (4 bytes) + reserved (1 byte) + NLRI
	valueLen := 2 + 1 + 1 + 4 + 1 + len(nlri)
	var attr []byte
	if valueLen > 255 {
		// Extended length.
		attr = make([]byte, 4+valueLen)
		attr[0] = 0xD0 // optional + transitive + extended-length
		attr[1] = 14
		binary.BigEndian.PutUint16(attr[2:4], uint16(valueLen))
		copy(attr[4:], []byte{0x00, 0x01, 0x01, 0x04})
		copy(attr[8:12], nhIP[:])
		attr[12] = 0x00 // reserved
		copy(attr[13:], nlri)
	} else {
		attr = make([]byte, 3+valueLen)
		attr[0] = 0xC0 // optional + transitive
		attr[1] = 14
		attr[2] = byte(valueLen)
		copy(attr[3:], []byte{0x00, 0x01, 0x01, 0x04})
		copy(attr[7:11], nhIP[:])
		attr[11] = 0x00 // reserved
		copy(attr[12:], nlri)
	}
	return attr
}

// buildMPReachIPv6 returns an MP_REACH_NLRI attribute for IPv6/unicast.
func buildMPReachIPv6(nhIP [16]byte, nlri []byte) []byte {
	// AFI=2 (2 bytes) + SAFI=1 (1 byte) + NH_len=16 (1 byte) + NH (16 bytes) + reserved (1 byte) + NLRI
	valueLen := 2 + 1 + 1 + 16 + 1 + len(nlri)
	var attr []byte
	if valueLen > 255 {
		attr = make([]byte, 4+valueLen)
		attr[0] = 0xD0
		attr[1] = 14
		binary.BigEndian.PutUint16(attr[2:4], uint16(valueLen))
		copy(attr[4:], []byte{0x00, 0x02, 0x01, 0x10})
		copy(attr[8:24], nhIP[:])
		attr[24] = 0x00
		copy(attr[25:], nlri)
	} else {
		attr = make([]byte, 3+valueLen)
		attr[0] = 0xC0
		attr[1] = 14
		attr[2] = byte(valueLen)
		copy(attr[3:], []byte{0x00, 0x02, 0x01, 0x10})
		copy(attr[7:23], nhIP[:])
		attr[23] = 0x00
		copy(attr[24:], nlri)
	}
	return attr
}

// buildUpdateBody constructs a minimal UPDATE body (no header) with zero
// withdrawn routes, the given attributes, and inline NLRI.
func buildUpdateBody(attrs, inlineNLRI []byte) []byte {
	body := make([]byte, 2+2+len(attrs)+len(inlineNLRI))
	// Withdrawn routes length = 0.
	binary.BigEndian.PutUint16(body[0:2], 0)
	binary.BigEndian.PutUint16(body[2:4], uint16(len(attrs)))
	copy(body[4:], attrs)
	copy(body[4+len(attrs):], inlineNLRI)
	return body
}

func TestPrefixExtractionInline(t *testing.T) {
	// VALIDATES: "Extract IPv4 prefixes from inline NLRI field"
	// PREVENTS: Missing inline NLRI parsing after path attributes

	// Build minimal path attributes: ORIGIN + NEXT_HOP + AS_PATH.
	var attrs []byte
	attrs = append(attrs, buildOriginAttr()...)
	attrs = append(attrs, buildNextHopAttr(1, 1, 1, 1)...)
	attrs = append(attrs, buildASPathAttr()...)

	// Inline NLRI: 10.0.0.0/24 = prefix_len=24, 3 bytes of address.
	inlineNLRI := []byte{24, 10, 0, 0}

	body := buildUpdateBody(attrs, inlineNLRI)

	prefixes := ExtractPrefixes(body)

	if len(prefixes) != 1 {
		t.Fatalf("expected 1 prefix, got %d", len(prefixes))
	}

	want := netip.MustParsePrefix("10.0.0.0/24")
	if prefixes[0] != want {
		t.Errorf("got %v, want %v", prefixes[0], want)
	}
}

func TestPrefixExtractionMP(t *testing.T) {
	// VALIDATES: "Extract prefixes from MP_REACH_NLRI attribute"
	// PREVENTS: Skipping MP_REACH_NLRI during attribute walk

	// NLRI: 10.0.0.0/24 = [24, 10, 0, 0]
	nlri := []byte{24, 10, 0, 0}
	mpReach := buildMPReachIPv4([4]byte{1, 1, 1, 1}, nlri)

	body := buildUpdateBody(mpReach, nil)

	prefixes := ExtractPrefixes(body)

	if len(prefixes) != 1 {
		t.Fatalf("expected 1 prefix, got %d", len(prefixes))
	}

	want := netip.MustParsePrefix("10.0.0.0/24")
	if prefixes[0] != want {
		t.Errorf("got %v, want %v", prefixes[0], want)
	}
}

func TestPrefixExtractionBothInlineAndMP(t *testing.T) {
	// VALIDATES: "Extract prefixes from both inline NLRI and MP_REACH_NLRI"
	// PREVENTS: Only extracting from one source when both are present

	// MP_REACH_NLRI with 10.1.0.0/24.
	mpNLRI := []byte{24, 10, 1, 0}
	mpReach := buildMPReachIPv4([4]byte{1, 1, 1, 1}, mpNLRI)

	// Also add ORIGIN + NEXT_HOP + AS_PATH for inline NLRI validity.
	var attrs []byte
	attrs = append(attrs, buildOriginAttr()...)
	attrs = append(attrs, buildNextHopAttr(1, 1, 1, 1)...)
	attrs = append(attrs, buildASPathAttr()...)
	attrs = append(attrs, mpReach...)

	// Inline NLRI with 10.0.0.0/24.
	inlineNLRI := []byte{24, 10, 0, 0}

	body := buildUpdateBody(attrs, inlineNLRI)

	prefixes := ExtractPrefixes(body)

	if len(prefixes) != 2 {
		t.Fatalf("expected 2 prefixes, got %d", len(prefixes))
	}

	// MP_REACH comes first (found during attribute walk), inline NLRI second.
	wantMP := netip.MustParsePrefix("10.1.0.0/24")
	wantInline := netip.MustParsePrefix("10.0.0.0/24")

	if prefixes[0] != wantMP {
		t.Errorf("prefix[0]: got %v, want %v", prefixes[0], wantMP)
	}
	if prefixes[1] != wantInline {
		t.Errorf("prefix[1]: got %v, want %v", prefixes[1], wantInline)
	}
}

func TestPrefixExtractionIPv6MP(t *testing.T) {
	// VALIDATES: "Extract IPv6 prefixes from MP_REACH_NLRI"
	// PREVENTS: Only handling AFI=1 in MP_REACH_NLRI parsing

	// 2001:db8:1::/48 encoded as NLRI: prefix_len=48, 6 bytes of address.
	nlri := []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}

	// Next-hop: 2001:db8::1.
	var nh [16]byte
	nh[0] = 0x20
	nh[1] = 0x01
	nh[2] = 0x0d
	nh[3] = 0xb8
	nh[15] = 0x01

	mpReach := buildMPReachIPv6(nh, nlri)

	body := buildUpdateBody(mpReach, nil)

	prefixes := ExtractPrefixes(body)

	if len(prefixes) != 1 {
		t.Fatalf("expected 1 prefix, got %d", len(prefixes))
	}

	want := netip.MustParsePrefix("2001:db8:1::/48")
	if prefixes[0] != want {
		t.Errorf("got %v, want %v", prefixes[0], want)
	}
}

func TestPrefixExtractionMultipleInline(t *testing.T) {
	// VALIDATES: "Extract multiple IPv4 prefixes from inline NLRI"
	// PREVENTS: Stopping after first prefix in inline NLRI

	var attrs []byte
	attrs = append(attrs, buildOriginAttr()...)
	attrs = append(attrs, buildNextHopAttr(1, 1, 1, 1)...)
	attrs = append(attrs, buildASPathAttr()...)

	// Two inline prefixes: 10.0.0.0/24 and 192.0.2.0/24.
	inlineNLRI := []byte{
		24, 10, 0, 0,
		24, 192, 0, 2,
	}

	body := buildUpdateBody(attrs, inlineNLRI)

	prefixes := ExtractPrefixes(body)

	if len(prefixes) != 2 {
		t.Fatalf("expected 2 prefixes, got %d", len(prefixes))
	}

	want0 := netip.MustParsePrefix("10.0.0.0/24")
	want1 := netip.MustParsePrefix("192.0.2.0/24")

	if prefixes[0] != want0 {
		t.Errorf("prefix[0]: got %v, want %v", prefixes[0], want0)
	}
	if prefixes[1] != want1 {
		t.Errorf("prefix[1]: got %v, want %v", prefixes[1], want1)
	}
}

func TestPrefixExtractionEmptyUpdate(t *testing.T) {
	// VALIDATES: "Handle UPDATE with no NLRI gracefully"
	// PREVENTS: Panic on UPDATE with only withdrawn routes or EOR

	var attrs []byte
	attrs = append(attrs, buildOriginAttr()...)
	attrs = append(attrs, buildNextHopAttr(1, 1, 1, 1)...)
	attrs = append(attrs, buildASPathAttr()...)

	body := buildUpdateBody(attrs, nil)

	prefixes := ExtractPrefixes(body)

	if len(prefixes) != 0 {
		t.Errorf("expected 0 prefixes, got %d", len(prefixes))
	}
}

func TestPrefixExtractionShortBody(t *testing.T) {
	// VALIDATES: "Handle truncated UPDATE body without panic"
	// PREVENTS: Out-of-bounds access on malformed input

	tests := []struct {
		name string
		body []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"one byte", []byte{0x00}},
		{"three bytes", []byte{0x00, 0x00, 0x00}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefixes := ExtractPrefixes(tt.body)
			if len(prefixes) != 0 {
				t.Errorf("expected 0 prefixes, got %d", len(prefixes))
			}
		})
	}
}

func TestPrefixExtractionSkipsWithdrawn(t *testing.T) {
	// VALIDATES: "Withdrawn routes are skipped, not returned"
	// PREVENTS: Returning withdrawn prefixes as announced

	// Withdrawn: 10.99.0.0/24 = [24, 10, 99, 0] (4 bytes).
	withdrawn := []byte{24, 10, 99, 0}

	var attrs []byte
	attrs = append(attrs, buildOriginAttr()...)
	attrs = append(attrs, buildNextHopAttr(1, 1, 1, 1)...)
	attrs = append(attrs, buildASPathAttr()...)

	// Inline NLRI: 10.0.0.0/24.
	inlineNLRI := []byte{24, 10, 0, 0}

	// Build body manually with withdrawn section.
	body := make([]byte, 2+len(withdrawn)+2+len(attrs)+len(inlineNLRI))
	binary.BigEndian.PutUint16(body[0:2], uint16(len(withdrawn)))
	copy(body[2:], withdrawn)
	off := 2 + len(withdrawn)
	binary.BigEndian.PutUint16(body[off:off+2], uint16(len(attrs)))
	off += 2
	copy(body[off:], attrs)
	off += len(attrs)
	copy(body[off:], inlineNLRI)

	prefixes := ExtractPrefixes(body)

	if len(prefixes) != 1 {
		t.Fatalf("expected 1 prefix, got %d", len(prefixes))
	}

	want := netip.MustParsePrefix("10.0.0.0/24")
	if prefixes[0] != want {
		t.Errorf("got %v, want %v", prefixes[0], want)
	}

	// Verify the withdrawn prefix is NOT in the result.
	withdrawn0 := netip.MustParsePrefix("10.99.0.0/24")
	for _, p := range prefixes {
		if p == withdrawn0 {
			t.Error("withdrawn prefix 10.99.0.0/24 should not be in result")
		}
	}
}

// VALIDATES: "CountPrefixes returns same count as len(ExtractPrefixes) for all UPDATE formats"
// PREVENTS: Divergence between cheap counter and full extractor.
func TestCountPrefixesMatchesExtract(t *testing.T) {
	tests := []struct {
		name string
		body []byte
	}{
		{"inline single", func() []byte {
			var attrs []byte
			attrs = append(attrs, buildOriginAttr()...)
			attrs = append(attrs, buildNextHopAttr(1, 1, 1, 1)...)
			attrs = append(attrs, buildASPathAttr()...)
			return buildUpdateBody(attrs, []byte{24, 10, 0, 0})
		}()},
		{"inline multiple", func() []byte {
			var attrs []byte
			attrs = append(attrs, buildOriginAttr()...)
			attrs = append(attrs, buildNextHopAttr(1, 1, 1, 1)...)
			attrs = append(attrs, buildASPathAttr()...)
			return buildUpdateBody(attrs, []byte{24, 10, 0, 0, 24, 192, 0, 2})
		}()},
		{"mp reach ipv4", buildUpdateBody(buildMPReachIPv4([4]byte{1, 1, 1, 1}, []byte{24, 10, 0, 0}), nil)},
		{"mp reach ipv6", func() []byte {
			var nh [16]byte
			nh[0] = 0x20
			nh[1] = 0x01
			nh[15] = 0x01
			return buildUpdateBody(buildMPReachIPv6(nh, []byte{48, 0x20, 0x01, 0x0d, 0xb8, 0x00, 0x01}), nil)
		}()},
		{"both inline and mp", func() []byte {
			var attrs []byte
			attrs = append(attrs, buildOriginAttr()...)
			attrs = append(attrs, buildNextHopAttr(1, 1, 1, 1)...)
			attrs = append(attrs, buildASPathAttr()...)
			attrs = append(attrs, buildMPReachIPv4([4]byte{1, 1, 1, 1}, []byte{24, 10, 1, 0})...)
			return buildUpdateBody(attrs, []byte{24, 10, 0, 0})
		}()},
		{"empty update", func() []byte {
			var attrs []byte
			attrs = append(attrs, buildOriginAttr()...)
			attrs = append(attrs, buildNextHopAttr(1, 1, 1, 1)...)
			attrs = append(attrs, buildASPathAttr()...)
			return buildUpdateBody(attrs, nil)
		}()},
		{"withdrawn present", func() []byte {
			withdrawn := []byte{24, 10, 99, 0} // 10.99.0.0/24 withdrawn
			var attrs []byte
			attrs = append(attrs, buildOriginAttr()...)
			attrs = append(attrs, buildNextHopAttr(1, 1, 1, 1)...)
			attrs = append(attrs, buildASPathAttr()...)
			inlineNLRI := []byte{24, 10, 0, 0}
			body := make([]byte, 2+len(withdrawn)+2+len(attrs)+len(inlineNLRI))
			binary.BigEndian.PutUint16(body[0:2], uint16(len(withdrawn)))
			copy(body[2:], withdrawn)
			off := 2 + len(withdrawn)
			binary.BigEndian.PutUint16(body[off:off+2], uint16(len(attrs)))
			copy(body[off+2:], attrs)
			copy(body[off+2+len(attrs):], inlineNLRI)
			return body
		}()},
		{"nil body", nil},
		{"short body", []byte{0x00}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			extracted := len(ExtractPrefixes(tt.body))
			counted := CountPrefixes(tt.body)
			if counted != extracted {
				t.Errorf("CountPrefixes=%d, len(ExtractPrefixes)=%d", counted, extracted)
			}
		})
	}
}

func TestPrefixExtractionExtendedLengthAttr(t *testing.T) {
	// VALIDATES: "Handle extended-length path attributes"
	// PREVENTS: Misparse when attribute has 2-byte length field

	// Build an MP_REACH_NLRI with extended-length flag (0x10).
	// AFI=1, SAFI=1, NH_len=4, NH=1.1.1.1, reserved=0, NLRI: 10.0.0.0/24
	nlriBytes := []byte{24, 10, 0, 0}
	valueLen := 2 + 1 + 1 + 4 + 1 + len(nlriBytes) // = 13
	attr := make([]byte, 4+valueLen)
	attr[0] = 0xD0 // optional + transitive + extended-length (0x80|0x40|0x10)
	attr[1] = 14
	binary.BigEndian.PutUint16(attr[2:4], uint16(valueLen))
	attr[4] = 0x00 // AFI high
	attr[5] = 0x01 // AFI low
	attr[6] = 0x01 // SAFI
	attr[7] = 0x04 // NH len
	attr[8] = 1    // NH
	attr[9] = 1
	attr[10] = 1
	attr[11] = 1
	attr[12] = 0x00 // reserved
	copy(attr[13:], nlriBytes)

	body := buildUpdateBody(attr, nil)

	prefixes := ExtractPrefixes(body)

	if len(prefixes) != 1 {
		t.Fatalf("expected 1 prefix, got %d", len(prefixes))
	}

	want := netip.MustParsePrefix("10.0.0.0/24")
	if prefixes[0] != want {
		t.Errorf("got %v, want %v", prefixes[0], want)
	}
}
