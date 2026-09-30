// Design: docs/architecture/wire/nlri-bgpls.md -- BGP-LS SR attribute decode.
// RFC: rfc/short/rfc9085.md
// Related: attr_node.go -- decodeSRCapabilities, decodeSRLocalBlock
//
// VALIDATES: a received SR Capabilities TLV 1034 or SR Local Block TLV 1036
// whose Flags and Reserved octets are all ones decodes to the range it
// carries: neither octet is checked, rejected, or read as range data, as RFC
// 9085 Sections 2.1.2 and 2.1.4 require ("ignored on receipt"). The SID TLVs
// with a Reserved field set to all ones decode (1099, 1158) or are kept whole
// (1100, 1159, which this decoder carries undecoded).
// PREVENTS: a collector refusing, or misaligning, a speaker's TLV because the
// speaker sets a flag or reserved bit this implementation does not know.
package ls

import "testing"

// rfc9085ReceivedRange is a Flags ff, Reserved ff header followed by one range
// of 1000 labels whose SID/Label sub-TLV 1161 of length 3 carries label 16000.
var rfc9085ReceivedRange = []byte{0xff, 0xff, 0x00, 0x03, 0xe8, 0x04, 0x89, 0x00, 0x03, 0x00, 0x3e, 0x80}

// rfc9085CheckRange fails unless ranges holds the one range above.
func rfc9085CheckRange(t *testing.T, ranges []LsSrLabelRange) {
	t.Helper()
	if len(ranges) != 1 {
		t.Fatalf("ranges = %+v, want one", ranges)
	}
	if ranges[0].Range != 1000 {
		t.Fatalf("range size = %d, want 1000", ranges[0].Range)
	}
	if ranges[0].FirstSID != 16000 {
		t.Fatalf("first label = %d, want 16000", ranges[0].FirstSID)
	}
}

// TestRFC9085CapabilitiesReceiptIgnoresFlagsAndReserved decodes TLV 1034 and
// TLV 1036 with every Flags and Reserved bit set.
//
// RFC requirement: RFC9085-2.1.2-1 positive -- a received TLV 1034 with Flags ff is not rejected and its range decodes intact: the flags are ignored on receipt (§2.1.2).
// RFC requirement: RFC9085-2.1.2-2 positive -- a received TLV 1034 with Reserved ff is not rejected and its range decodes intact: the Reserved octet is ignored on receipt (§2.1.2).
// RFC requirement: RFC9085-2.1.4-1 positive -- a received TLV 1036 with Flags ff is not rejected and its range decodes intact: the flags are ignored on receipt (§2.1.4).
// RFC requirement: RFC9085-2.1.4-2 positive -- a received TLV 1036 with Reserved ff is not rejected and its range decodes intact: the Reserved octet is ignored on receipt (§2.1.4).
func TestRFC9085CapabilitiesReceiptIgnoresFlagsAndReserved(t *testing.T) {
	decoded, err := decodeSRCapabilities(rfc9085ReceivedRange)
	if err != nil {
		t.Fatalf("TLV 1034 refused: %v", err)
	}
	capabilities, ok := decoded.(*LsSRCapabilities)
	if !ok {
		t.Fatalf("TLV 1034 decoded as %T", decoded)
	}
	rfc9085CheckRange(t, capabilities.Ranges)

	decoded, err = decodeSRLocalBlock(rfc9085ReceivedRange)
	if err != nil {
		t.Fatalf("TLV 1036 refused: %v", err)
	}
	block, ok := decoded.(*lsSRLocalBlock)
	if !ok {
		t.Fatalf("TLV 1036 decoded as %T", decoded)
	}
	rfc9085CheckRange(t, block.Ranges)
}

// TestRFC9085SIDReceiptIgnoresReserved decodes an Adjacency SID TLV 1099 and a
// Prefix-SID TLV 1158 whose 2-octet Reserved field is ff ff, and checks the
// TLV is kept with its flags, weight or algorithm, and SID intact.
//
// RFC requirement: RFC9085-2.2.1-1 positive -- a received TLV 1099 with Reserved ff ff is not rejected and its flags, weight and 4-octet SID 7 decode intact: the Reserved field is ignored on receipt (§2.2.1).
// RFC requirement: RFC9085-2.3.1-1 positive -- a received TLV 1158 with Reserved ff ff is not rejected and its flags, algorithm and 4-octet SID 7 decode intact: the Reserved field is ignored on receipt (§2.3.1).
func TestRFC9085SIDReceiptIgnoresReserved(t *testing.T) {
	value := []byte{0x30, 0x01, 0xff, 0xff, 0x00, 0x00, 0x00, 0x07}
	decoded, err := decodeAdjacencySID(value)
	if err != nil {
		t.Fatalf("TLV 1099 refused: %v", err)
	}
	adjacency, ok := decoded.(*lsAdjacencySID)
	if !ok {
		t.Fatalf("TLV 1099 decoded as %T", decoded)
	}
	if adjacency.Flags != 0x30 || adjacency.Weight != 1 || adjacency.SID != 7 {
		t.Fatalf("TLV 1099 = %+v, want flags 30, weight 1, SID 7", adjacency)
	}

	decoded, err = decodePrefixSID(value)
	if err != nil {
		t.Fatalf("TLV 1158 refused: %v", err)
	}
	prefixSID, ok := decoded.(*lsPrefixSID)
	if !ok {
		t.Fatalf("TLV 1158 decoded as %T", decoded)
	}
	if prefixSID.Flags != 0x30 || prefixSID.Algorithm != 1 || prefixSID.SID != 7 {
		t.Fatalf("TLV 1158 = %+v, want flags 30, algorithm 1, SID 7", prefixSID)
	}
}

// TestRFC9085UndecodedSIDReceiptKeepsReserved hands the offline decoder a
// BGP-LS Attribute holding a LAN Adjacency SID TLV 1100 whose 2-octet Reserved
// field is ff ff and a Range TLV 1159 whose Reserved octet is ff. This decoder
// carries both TLVs undecoded, so the check is that each is kept whole, every
// octet as received, and that neither Reserved value refuses the TLV or the
// attribute.
//
// RFC requirement: RFC9085-2.2.2-1 positive -- a received TLV 1100 with Reserved ff ff is not refused: the offline decoder keeps it whole, Reserved included, beside the other TLVs (§2.2.2).
// RFC requirement: RFC9085-2.3.5-1 positive -- a received TLV 1159 with Reserved ff is not refused: the offline decoder keeps it whole, Reserved included, beside the other TLVs (§2.3.5).
func TestRFC9085UndecodedSIDReceiptKeepsReserved(t *testing.T) {
	data := []byte{
		// TLV 1100, length 12: Flags, Weight, Reserved ff ff, Neighbor ID 4.4.4.4, SID 7.
		0x04, 0x4c, 0x00, 0x0c, 0x30, 0x01, 0xff, 0xff, 4, 4, 4, 4, 0x00, 0x00, 0x00, 0x07,
		// TLV 1159, length 16: Flags, Reserved ff, Range Size 16, Prefix-SID TLV 1158 (index 100).
		0x04, 0x87, 0x00, 0x10, 0x80, 0xff, 0x00, 0x10, 0x04, 0x86, 0x00, 0x08,
		0x40, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x64,
		// TLV 1026 Node Name "r1", which must still decode after both.
		0x04, 0x02, 0x00, 0x02, 'r', '1',
	}
	result := AttrTLVsToJSON(data)
	want := map[string]string{
		"generic-lsid-1100": "0x3001FFFF0404040400000007",
		"generic-lsid-1159": "0x80FF0010048600084000000000000064",
	}
	for key, value := range want {
		kept, ok := result[key].([]string)
		if !ok || len(kept) != 1 || kept[0] != value {
			t.Fatalf("%s = %v, want [%s]", key, result[key], value)
		}
	}
	if result["node-name"] != "r1" {
		t.Fatalf("node-name = %v, want r1: a Reserved value refused the attribute", result["node-name"])
	}
}
