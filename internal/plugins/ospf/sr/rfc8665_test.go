// VALIDATES: RFC 8665 (OSPF Extensions for Segment Routing) at the shared SR control-plane
// seam: the SID/Label Range and SR Local Block TLV codec (Range Size > 0, exactly one
// SID/Label sub-TLV), the SRGB index-to-label arithmetic in advertised range order, the
// configured-range non-overlap validation, the Prefix-SID / Adj-SID / LAN-Adj-SID flag
// encoding (reserved bits, V/L pairing), and the NP/M/E outgoing-label truth table.
// PREVENTS: a range TLV accepted with size 0 or two SID/Label sub-TLVs, an index mapped
// without honoring the advertised range order, a reserved flag bit leaking onto the wire or
// changing a decode, an invalid V/L advertisement being installed, and a wrong PHP /
// Explicit-NULL decision at the penultimate hop.
package sr

import (
	"encoding/binary"
	"testing"
)

// srgbRange is a readable helper for a SID/Label range literal.
func srgbRange(base, size uint32) LabelRange { return LabelRange{Base: base, Size: size} }

// ---- SID/Label Range (SRGB, §3.2) and SR Local Block (SRLB, §3.3) ----

// RFC requirement: RFC8665-3.2-1 positive -- a SID/Label Range TLV with Range Size 100 is
// accepted and decodes back to the advertised base and size (DecodeRangeValue, codec.go:288).
// RFC requirement: RFC8665-3.3-1 positive -- the SRLB TLV shares the same value codec, so an
// SRLB range with a non-zero Range Size is accepted the same way (srBuildSRLB feeds
// EncodeRangeValue, sr.go:182).
// RFC requirement: RFC8665-3.2-2 positive -- the encoded range value carries exactly one
// SID/Label sub-TLV (type 1) holding the first label of the range (EncodeRangeValue,
// codec.go:271).
// RFC requirement: RFC8665-3.3-2 positive -- the SRLB value is built by the same encoder, so
// it too always carries the SID/Label sub-TLV.
func TestRFC8665RangeTLVRoundTrip(t *testing.T) {
	v := EncodeRangeValue(srgbRange(16000, 100))
	// Range Size occupies the first 3 octets, then a reserved octet, then the sub-TLVs.
	if got := read24(v, 0); got != 100 {
		t.Fatalf("Range Size = %d, want 100", got)
	}
	if v[3] != 0 {
		t.Fatalf("Reserved octet = %#x, want 0", v[3])
	}
	if typ := binary.BigEndian.Uint16(v[4:]); typ != 1 {
		t.Fatalf("first sub-TLV type = %d, want 1 (SID/Label)", typ)
	}
	isLabel, base, err := decodeSIDLabelSubTLV(v[4:])
	if err != nil || !isLabel || base != 16000 {
		t.Fatalf("SID/Label sub-TLV = label:%v base:%d err:%v", isLabel, base, err)
	}
	r, err := DecodeRangeValue(v)
	if err != nil || r.Base != 16000 || r.Size != 100 {
		t.Fatalf("DecodeRangeValue = %+v, %v", r, err)
	}
}

// RFC requirement: RFC8665-3.2-1 negative -- a SID/Label Range TLV advertising Range Size 0 is
// rejected as malformed, so no zero-sized range enters the SRGB (DecodeRangeValue,
// codec.go:293-295); the configuration path rejects the same shape (validRange,
// config.go:55-57).
// RFC requirement: RFC8665-3.3-1 negative -- the SRLB TLV is decoded by the same function, so
// an SRLB range with Range Size 0 is rejected identically, and a configured SRLB range with
// size 0 fails validation (SRConfig.Validate, config.go:111-115).
func TestRFC8665RangeSizeZeroRejected(t *testing.T) {
	v := EncodeRangeValue(srgbRange(16000, 100))
	put24(v, 0)
	if _, err := DecodeRangeValue(v); err == nil {
		t.Fatalf("Range Size 0 must be rejected")
	}
	cfg := SRConfig{Enabled: true, SRGB: []LabelRange{srgbRange(16000, 0)}}
	if err := cfg.Validate(nil); err == nil {
		t.Fatalf("configured SRGB Range Size 0 must be rejected")
	}
	cfgLB := SRConfig{
		Enabled: true,
		SRGB:    []LabelRange{srgbRange(16000, 100)},
		SRLB:    []LabelRange{srgbRange(40000, 0)},
	}
	if err := cfgLB.Validate(nil); err == nil {
		t.Fatalf("configured SRLB Range Size 0 must be rejected")
	}
}

// RFC requirement: RFC8665-3.2-2 negative -- a SID/Label Range TLV carrying no SID/Label
// sub-TLV has no first label and is rejected, so the range never enters the SRGB
// (DecodeRangeValue count check, codec.go:317-319).
// RFC requirement: RFC8665-3.3-2 negative -- the SRLB TLV runs through the same decoder, so an
// SRLB range without its SID/Label sub-TLV is rejected too.
func TestRFC8665RangeWithoutSIDLabelSubTLVRejected(t *testing.T) {
	other := writeSubTLV(99, []byte{0, 0, 0, 0})
	v := make([]byte, 4+len(other))
	put24(v, 100)
	copy(v[4:], other)
	if _, err := DecodeRangeValue(v); err == nil {
		t.Fatalf("a range TLV with no SID/Label sub-TLV must be rejected")
	}
}

// RFC requirement: RFC8665-3.2-3 positive -- a SID/Label Range TLV carrying exactly one
// SID/Label sub-TLV is accepted and its base label used (DecodeRangeValue, codec.go:296-319).
// RFC requirement: RFC8665-3.3-3 positive -- the SRLB TLV is accepted on the same
// exactly-one-sub-TLV rule.
func TestRFC8665RangeWithSingleSIDLabelAccepted(t *testing.T) {
	r, err := DecodeRangeValue(EncodeRangeValue(srgbRange(40000, 10)))
	if err != nil || r.Base != 40000 || r.Size != 10 {
		t.Fatalf("single SID/Label sub-TLV range = %+v, %v", r, err)
	}
}

// RFC requirement: RFC8665-3.2-3 negative -- a SID/Label Range TLV carrying two SID/Label
// sub-TLVs is ignored in full rather than resolved to one of them (DecodeRangeValue rejects on
// count != 1, codec.go:317-319).
// RFC requirement: RFC8665-3.3-3 negative -- an SRLB TLV with two SID/Label sub-TLVs is ignored
// by the same check; the reception path counts it and skips the range (srDecodeRemoteCapabilities,
// sr.go:343-348).
func TestRFC8665RangeWithTwoSIDLabelSubTLVsIgnored(t *testing.T) {
	sub := encodeSIDLabelSubTLV(true, 16000)
	v := make([]byte, 4+2*len(sub))
	put24(v, 100)
	copy(v[4:], sub)
	copy(v[4+len(sub):], sub)
	if _, err := DecodeRangeValue(v); err == nil {
		t.Fatalf("a range TLV with two SID/Label sub-TLVs must be ignored")
	}
}

// RFC requirement: RFC8665-3.2-6 positive -- a SID index is mapped to a label by walking the
// ranges in ADVERTISED order and accumulating the prior range sizes, not by sorting them
// (SRGB.Label, srgb.go:93-105): with ranges advertised 20000..20004 then 16000..16004, index 0
// is 20000 and index 5 is 16000.
func TestRFC8665SRGBIndexUsesAdvertisedOrder(t *testing.T) {
	g := NewSRGB([]LabelRange{srgbRange(20000, 5), srgbRange(16000, 5)})
	cases := []struct {
		index uint32
		label uint32
	}{{0, 20000}, {4, 20004}, {5, 16000}, {9, 16004}}
	for _, c := range cases {
		label, ok := g.Label(c.index)
		if !ok || label != c.label {
			t.Fatalf("Label(%d) = %d,%v want %d", c.index, label, ok, c.label)
		}
	}
}

// RFC requirement: RFC8665-3.2-6 negative -- an index at or beyond the total advertised range
// size maps to no label at all, so nothing is installed from it (SRGB.Label returns false,
// srgb.go:104); a reversed advertised order yields different labels, proving the mapping is
// order-dependent rather than range-sorted.
func TestRFC8665SRGBIndexOutOfRangeAndOrderSensitivity(t *testing.T) {
	g := NewSRGB([]LabelRange{srgbRange(20000, 5), srgbRange(16000, 5)})
	if _, ok := g.Label(10); ok {
		t.Fatalf("index 10 is beyond the total size 10 and must map to no label")
	}
	reversed := NewSRGB([]LabelRange{srgbRange(16000, 5), srgbRange(20000, 5)})
	fwd, _ := g.Label(0)
	rev, _ := reversed.Label(0)
	if fwd == rev {
		t.Fatalf("index 0 must follow the advertised order: %d vs %d", fwd, rev)
	}
}

// RFC requirement: RFC8665-3.2-7 positive -- two non-overlapping SRGB ranges validate, so a
// conformant multi-range advertisement is originated (noSelfOverlap, config.go:71-81 via
// SRConfig.Validate, config.go:116).
// RFC requirement: RFC8665-3.3-4 positive -- two non-overlapping SRLB ranges validate the same
// way (config.go:119).
func TestRFC8665NonOverlappingRangesAccepted(t *testing.T) {
	cfg := SRConfig{
		Enabled: true,
		SRGB:    []LabelRange{srgbRange(16000, 100), srgbRange(20000, 100)},
		SRLB:    []LabelRange{srgbRange(40000, 10), srgbRange(41000, 10)},
	}
	if err := cfg.Validate(nil); err != nil {
		t.Fatalf("non-overlapping ranges must validate: %v", err)
	}
}

// RFC requirement: RFC8665-3.2-7 negative -- two overlapping SRGB ranges are rejected before
// origination, so this router never advertises overlapping ranges (noSelfOverlap,
// config.go:71-81).
// RFC requirement: RFC8665-3.3-4 negative -- two overlapping SRLB ranges are rejected by the
// same check (config.go:119), and an SRLB range overlapping the SRGB is rejected as well
// (noCrossOverlap, config.go:122).
func TestRFC8665OverlappingRangesRejected(t *testing.T) {
	srgb := SRConfig{
		Enabled: true,
		SRGB:    []LabelRange{srgbRange(16000, 100), srgbRange(16050, 100)},
	}
	if err := srgb.Validate(nil); err == nil {
		t.Fatalf("overlapping SRGB ranges must be rejected")
	}
	srlb := SRConfig{
		Enabled: true,
		SRGB:    []LabelRange{srgbRange(16000, 100)},
		SRLB:    []LabelRange{srgbRange(40000, 10), srgbRange(40005, 10)},
	}
	if err := srlb.Validate(nil); err == nil {
		t.Fatalf("overlapping SRLB ranges must be rejected")
	}
	cross := SRConfig{
		Enabled: true,
		SRGB:    []LabelRange{srgbRange(16000, 100)},
		SRLB:    []LabelRange{srgbRange(16050, 10)},
	}
	if err := cross.Validate(nil); err == nil {
		t.Fatalf("an SRLB range overlapping the SRGB must be rejected")
	}
}

// ---- Prefix-SID sub-TLV (§5) ----

// RFC requirement: RFC8665-5-1 positive -- the Prefix-SID Flags octet is assembled from the
// five defined bits only, so bit 0 and bits 6-7 are always zero on transmission
// (SIDFlags.toByte, codec.go:87-105, written at codec.go:354).
func TestRFC8665PrefixSIDReservedFlagBitsZeroOnSend(t *testing.T) {
	const reservedMask = 0x83 // bit 0 (0x80) and bits 6-7 (0x02, 0x01)
	all := SIDFlags{NP: true, M: true, E: true, V: true, L: true}
	v := EncodePrefixSIDValue(PrefixSID{Flags: all, Label: 16001})
	if v[0]&reservedMask != 0 {
		t.Fatalf("reserved Prefix-SID flag bits set on send: %#x", v[0])
	}
	if v[1] != 0 {
		t.Fatalf("Prefix-SID Reserved octet = %#x, want 0", v[1])
	}
}

// RFC requirement: RFC8665-5-1 negative -- a received Prefix-SID whose reserved flag bits are
// set is not rejected and decodes to exactly the same advertisement as one with them clear:
// the reserved bits are ignored, never interpreted (sidFlagsFromByte reads only the five
// defined masks, codec.go:107-109).
func TestRFC8665PrefixSIDReservedFlagBitsIgnoredOnReceive(t *testing.T) {
	clean := EncodePrefixSIDValue(PrefixSID{Flags: SIDFlags{NP: true}, Index: 9})
	dirty := make([]byte, len(clean))
	copy(dirty, clean)
	dirty[0] |= 0x83
	dirty[1] = 0xFF // Reserved octet: MUST be ignored on reception too
	want, err := DecodePrefixSIDValue(clean)
	if err != nil {
		t.Fatalf("clean Prefix-SID decode: %v", err)
	}
	got, err := DecodePrefixSIDValue(dirty)
	if err != nil {
		t.Fatalf("a Prefix-SID with reserved bits set must still decode: %v", err)
	}
	if got != want {
		t.Fatalf("reserved bits changed the decode: %+v vs %+v", got, want)
	}
}

// RFC requirement: RFC8665-5-5 positive -- the two legal V/L combinations decode: V=0/L=0 is a
// 4-octet index and V=1/L=1 a 3-octet local label in the 20 rightmost bits
// (DecodePrefixSIDValue, codec.go:363-386; DecodeAdjSIDValue, codec.go:407; DecodeLANAdjSIDValue,
// codec.go:451).
func TestRFC8665ValidVLCombinationsDecode(t *testing.T) {
	idx := EncodePrefixSIDValue(PrefixSID{Index: 9})
	if len(idx) != 8 {
		t.Fatalf("index-form Prefix-SID value length = %d, want 8", len(idx))
	}
	p, err := DecodePrefixSIDValue(idx)
	if err != nil || p.IsLabel || p.Index != 9 {
		t.Fatalf("index-form Prefix-SID = %+v, %v", p, err)
	}
	lbl := EncodePrefixSIDValue(PrefixSID{Flags: SIDFlags{V: true, L: true}, Label: 16001})
	if len(lbl) != 7 {
		t.Fatalf("label-form Prefix-SID value length = %d, want 7", len(lbl))
	}
	p, err = DecodePrefixSIDValue(lbl)
	if err != nil || !p.IsLabel || p.Label != 16001 {
		t.Fatalf("label-form Prefix-SID = %+v, %v", p, err)
	}
	a, err := DecodeAdjSIDValue(EncodeAdjSIDValue(AdjSID{Flags: AdjSIDFlags{V: true, L: true}, Label: 40001}))
	if err != nil || !a.IsLabel || a.Label != 40001 {
		t.Fatalf("label-form Adj-SID = %+v, %v", a, err)
	}
	lan := EncodeLANAdjSIDValue(AdjSID{Flags: AdjSIDFlags{V: true, L: true}, Label: 40002, NeighborID: [4]byte{10, 0, 0, 2}})
	l, err := DecodeLANAdjSIDValue(lan)
	if err != nil || !l.IsLabel || l.Label != 40002 || l.NeighborID != [4]byte{10, 0, 0, 2} {
		t.Fatalf("label-form LAN-Adj-SID = %+v, %v", l, err)
	}
}

// RFC requirement: RFC8665-5-5 negative -- a SID advertisement with V set and L clear (or the
// reverse) is an invalid combination and is ignored, in the Prefix-SID, the Adj-SID and the
// LAN-Adj-SID alike (validVL, codec.go:113 and codec.go:139, enforced at codec.go:368, 412, 456).
func TestRFC8665InvalidVLCombinationIgnored(t *testing.T) {
	for _, flags := range []byte{flagV, flagL} {
		v := EncodePrefixSIDValue(PrefixSID{Index: 9})
		v[0] = flags
		if _, err := DecodePrefixSIDValue(v); err == nil {
			t.Fatalf("Prefix-SID with flags %#x must be ignored", flags)
		}
	}
	for _, flags := range []byte{adjFlagV, adjFlagL} {
		v := EncodeAdjSIDValue(AdjSID{Index: 9})
		v[0] = flags
		if _, err := DecodeAdjSIDValue(v); err == nil {
			t.Fatalf("Adj-SID with flags %#x must be ignored", flags)
		}
		lan := EncodeLANAdjSIDValue(AdjSID{Index: 9})
		lan[0] = flags
		if _, err := DecodeLANAdjSIDValue(lan); err == nil {
			t.Fatalf("LAN-Adj-SID with flags %#x must be ignored", flags)
		}
	}
}

// ---- Outgoing-label truth table (§5) ----

// RFC requirement: RFC8665-5-2 positive -- with the NP-Flag set and the E-Flag clear the
// penultimate hop keeps the Prefix-SID label rather than popping it (OutgoingActionFor,
// install.go:50-61 returns ActionKeep; OutgoingLabel, install.go:67-76 imposes the label).
// RFC requirement: RFC8665-5-11 positive -- the same NP=1/E=0 case keeps the Prefix-SID on top
// of the stack.
// RFC requirement: RFC8665-5-3 negative -- with the E-Flag clear the label is NOT replaced by
// the IPv4 Explicit NULL label 0.
// RFC requirement: RFC8665-5-12 negative -- NP set with E clear does not produce an Explicit
// NULL replacement.
// RFC requirement: RFC8665-5-10 negative -- with the NP-Flag set the upstream neighbor does not
// PHP-pop the Prefix-SID.
func TestRFC8665NoPHPKeepsPrefixSIDLabel(t *testing.T) {
	action := OutgoingActionFor(SIDFlags{NP: true})
	if action != ActionKeep {
		t.Fatalf("NP=1/E=0 action = %v, want ActionKeep", action)
	}
	label, imposed := OutgoingLabel(16009, action, ExplicitNullV4)
	if !imposed || label != 16009 {
		t.Fatalf("NP=1/E=0 label = %d,%v want 16009 imposed", label, imposed)
	}
}

// RFC requirement: RFC8665-5-10 positive -- with the NP-Flag clear the upstream neighbor pops
// the Prefix-SID (PHP) and the received E-Flag is ignored (OutgoingActionFor tests NP before E,
// install.go:53-58); OutgoingLabel reports that no label is imposed (install.go:71-72).
// RFC requirement: RFC8665-5-2 negative -- the no-pop obligation binds only when NP is set: with
// NP clear the label IS popped, so the decision is driven by the advertised flag rather than
// applied unconditionally.
func TestRFC8665PHPPopsWhenNoPHPFlagClear(t *testing.T) {
	action := OutgoingActionFor(SIDFlags{E: true})
	if action != ActionPHP {
		t.Fatalf("NP=0 action = %v, want ActionPHP (E ignored)", action)
	}
	if _, imposed := OutgoingLabel(16009, action, ExplicitNullV4); imposed {
		t.Fatalf("PHP must impose no label")
	}
}

// RFC requirement: RFC8665-5-3 positive -- with the E-Flag set the upstream neighbor replaces
// the Prefix-SID with the Explicit NULL label, which is 0 for IPv4 (ExplicitNullV4,
// install.go:14; OutgoingLabel ActionExplicitNull branch, install.go:69-70).
// RFC requirement: RFC8665-5-12 positive -- NP and E both set yields the Explicit NULL
// replacement (OutgoingActionFor, install.go:56-58).
// RFC requirement: RFC8665-5-11 negative -- NP set with E set does NOT keep the Prefix-SID on
// top of the stack, so the keep rule is conditional on E being clear.
func TestRFC8665ExplicitNullReplacesPrefixSID(t *testing.T) {
	action := OutgoingActionFor(SIDFlags{NP: true, E: true})
	if action != ActionExplicitNull {
		t.Fatalf("NP=1/E=1 action = %v, want ActionExplicitNull", action)
	}
	label, imposed := OutgoingLabel(16009, action, ExplicitNullV4)
	if !imposed || label != ExplicitNullV4 {
		t.Fatalf("NP=1/E=1 label = %d,%v want the IPv4 Explicit NULL 0", label, imposed)
	}
}

// RFC requirement: RFC8665-5-13 positive -- when the M-Flag is set the NP- and E-Flags are
// ignored on reception: NP clear plus E set would otherwise mean PHP, yet the M branch returns
// ActionKeep first (OutgoingActionFor, install.go:51-52).
func TestRFC8665MappingServerFlagIgnoresNPAndE(t *testing.T) {
	action := OutgoingActionFor(SIDFlags{M: true, E: true})
	if action != ActionKeep {
		t.Fatalf("M=1 action = %v, want ActionKeep (NP and E ignored)", action)
	}
	label, imposed := OutgoingLabel(16009, action, ExplicitNullV4)
	if !imposed || label != 16009 {
		t.Fatalf("M=1 label = %d,%v want 16009 imposed", label, imposed)
	}
}

// RFC requirement: RFC8665-5-13 negative -- with the M-Flag clear the very same NP/E pair is
// honored instead of ignored, so the ignore rule is conditional on M rather than blanket.
func TestRFC8665MappingServerFlagClearHonorsNPAndE(t *testing.T) {
	if action := OutgoingActionFor(SIDFlags{E: true}); action != ActionPHP {
		t.Fatalf("M=0/NP=0 action = %v, want ActionPHP", action)
	}
	if action := OutgoingActionFor(SIDFlags{NP: true, E: true}); action != ActionExplicitNull {
		t.Fatalf("M=0/NP=1/E=1 action = %v, want ActionExplicitNull", action)
	}
}

// ---- Adj-SID sub-TLV (§6.1) ----

// RFC requirement: RFC8665-6.1-1 positive -- the Adj-SID Flags octet is assembled from the five
// defined bits (B/V/L/G/P) only, so reserved bits 5-7 are zero on transmission
// (AdjSIDFlags.toByte, codec.go:115-133, written at codec.go:399 and codec.go:437).
func TestRFC8665AdjSIDReservedFlagBitsZeroOnSend(t *testing.T) {
	const reservedMask = 0x07 // bits 5-7
	all := AdjSIDFlags{B: true, V: true, L: true, G: true, P: true}
	v := EncodeAdjSIDValue(AdjSID{Flags: all, Label: 40001})
	if v[0]&reservedMask != 0 {
		t.Fatalf("reserved Adj-SID flag bits set on send: %#x", v[0])
	}
	if v[1] != 0 {
		t.Fatalf("Adj-SID Reserved octet = %#x, want 0", v[1])
	}
	lan := EncodeLANAdjSIDValue(AdjSID{Flags: all, Label: 40002})
	if lan[0]&reservedMask != 0 || lan[1] != 0 {
		t.Fatalf("reserved LAN-Adj-SID flag bits set on send: %#x %#x", lan[0], lan[1])
	}
}

// RFC requirement: RFC8665-6.1-1 negative -- a received Adj-SID with reserved bits 5-7 set is
// not rejected and decodes identically to one with them clear: the reserved bits are ignored
// (adjFlagsFromByte reads only the five defined masks, codec.go:135-137).
func TestRFC8665AdjSIDReservedFlagBitsIgnoredOnReceive(t *testing.T) {
	clean := EncodeAdjSIDValue(AdjSID{Flags: AdjSIDFlags{V: true, L: true}, Label: 40001})
	dirty := make([]byte, len(clean))
	copy(dirty, clean)
	dirty[0] |= 0x07
	dirty[1] = 0xFF
	want, err := DecodeAdjSIDValue(clean)
	if err != nil {
		t.Fatalf("clean Adj-SID decode: %v", err)
	}
	got, err := DecodeAdjSIDValue(dirty)
	if err != nil {
		t.Fatalf("an Adj-SID with reserved bits set must still decode: %v", err)
	}
	if got != want {
		t.Fatalf("reserved bits changed the decode: %+v vs %+v", got, want)
	}
}

// ---- Malformed TLV handling (§9, §10) ----

// srTLVCase is one well-formed SR TLV / sub-TLV value with the decoder that owns it and the
// smallest length that is still a complete field layout. Everything shorter than content is a
// length inconsistent with the layout and MUST be rejected; the octets between content and the
// full value are 4-octet alignment padding, which the decoders tolerate on a trailing sub-TLV.
type srTLVCase struct {
	name    string
	value   []byte
	content int
	decode  func([]byte) error
}

// srWellFormedValues returns one well-formed value per SR TLV / sub-TLV codec, so the malformed
// tests can truncate each in turn.
func srWellFormedValues() []srTLVCase {
	return []srTLVCase{
		// 4 fixed octets + a 4-octet sub-TLV header + a 3-octet label, padded to 12.
		{"range", EncodeRangeValue(srgbRange(16000, 100)), 11, func(b []byte) error { _, err := DecodeRangeValue(b); return err }},
		// Preference octet then 3 Reserved octets, which are ignored on reception.
		{"srms", EncodeSRMSValue(200), 1, func(b []byte) error { _, err := DecodeSRMSValue(b); return err }},
		{"prefix-sid", EncodePrefixSIDValue(PrefixSID{Index: 9}), 8, func(b []byte) error { _, err := DecodePrefixSIDValue(b); return err }},
		{"adj-sid", EncodeAdjSIDValue(AdjSID{Index: 9}), 8, func(b []byte) error { _, err := DecodeAdjSIDValue(b); return err }},
		{"lan-adj-sid", EncodeLANAdjSIDValue(AdjSID{Index: 9}), 12, func(b []byte) error { _, err := DecodeLANAdjSIDValue(b); return err }},
		// 12 fixed octets (the sub-TLV region is optional) then one Prefix-SID sub-TLV.
		{"ext-prefix-range", EncodeExtPrefixRangeValueV4(32, [4]byte{10, 0, 0, 9}, 1, false, PrefixSID{Index: 9}), 12,
			func(b []byte) error { _, err := decodeExtPrefixRangeValueV4(b); return err }},
	}
}

// RFC requirement: RFC8665-9-1 positive -- every SR TLV and sub-TLV whose length is consistent
// with its field layout decodes without error, so a well-formed LSA is applied.
// RFC requirement: RFC8665-10-1 positive -- the same well-formed values are handled by the
// bound-checked decoders without panicking (subTLVIter, codec.go:187-208).
func TestRFC8665WellFormedTLVsDecode(t *testing.T) {
	for _, c := range srWellFormedValues() {
		if err := c.decode(c.value); err != nil {
			t.Fatalf("%s: well-formed value must decode: %v", c.name, err)
		}
	}
}

// RFC requirement: RFC8665-9-1 negative -- an SR TLV/sub-TLV truncated inside its field layout
// has an invalid length and is rejected with ErrMalformed, so the carrying LSA is treated as
// malformed and ignored rather than half-applied (length checks at codec.go:289, 336, 364,
// 408, 452, 498). The octets past `content` are 4-octet alignment padding, which the decoder
// deliberately tolerates on a trailing sub-TLV (subTLVIter, codec.go:202-206).
// RFC requirement: RFC8665-10-1 negative -- EVERY truncation, including the ones the codec
// legitimately tolerates, is bound-checked rather than indexing past the buffer: each decode
// returns instead of panicking, and this test fails on a panic, so a malformed TLV is not a
// crash vulnerability.
func TestRFC8665TruncatedTLVsRejectedWithoutPanic(t *testing.T) {
	for _, c := range srWellFormedValues() {
		for n := range len(c.value) {
			err := c.decode(c.value[:n])
			if n < c.content && err == nil {
				t.Fatalf("%s: truncation to %d octets must be rejected", c.name, n)
			}
		}
	}
}
