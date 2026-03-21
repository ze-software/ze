package ls

import (
	"encoding/binary"
	"math"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Framework Tests ---

// TestTLVRegistration verifies all Phase 1 TLV decoders are registered.
//
// VALIDATES: init() registration wires all TLV codes to decoders.
// PREVENTS: Missing registration causing silent decode failure.
func TestTLVRegistration(t *testing.T) {
	phase1Codes := []uint16{
		// Node
		TLVNodeFlagBits, TLVOpaqueNodeAttr, TLVNodeName,
		TLVISISAreaID, TLVIPv4RouterIDLocal, TLVIPv6RouterIDLocal,
		// Link
		TLVIPv4RouterIDRemote, TLVIPv6RouterIDRemote,
		TLVAdminGroup, TLVMaxLinkBandwidth, TLVMaxReservableBW,
		TLVUnreservedBW, TLVTEDefaultMetric, TLVIGPMetric,
		TLVSRLG, TLVOpaqueLinkAttr, TLVLinkName,
		// Prefix
		TLVIGPFlags, TLVPrefixMetric, TLVOpaquePrefixAttr,
	}

	for _, code := range phase1Codes {
		decoder := LookupLsAttrTLVDecoder(code)
		assert.NotNilf(t, decoder, "no decoder registered for TLV code %d", code)
	}

	// At least 20 for Phase 1
	assert.GreaterOrEqual(t, RegisteredLsAttrTLVCount(), 20)
}

// TestTLVIterator verifies the iterator yields correct entries.
//
// VALIDATES: IterateAttrTLVs parses Type(2)+Length(2)+Value(N) sequences.
// PREVENTS: Off-by-one in TLV header parsing.
func TestTLVIterator(t *testing.T) {
	// Build wire: TLV 1026 (Node Name) "test" + TLV 1095 (IGP Metric) 1-byte value
	data := make([]byte, 4+4+4+1)
	binary.BigEndian.PutUint16(data[0:], TLVNodeName)
	binary.BigEndian.PutUint16(data[2:], 4)
	copy(data[4:8], "test")
	binary.BigEndian.PutUint16(data[8:], TLVIGPMetric)
	binary.BigEndian.PutUint16(data[10:], 1)
	data[12] = 10

	var entries []AttrTLVEntry
	err := IterateAttrTLVs(data, func(e AttrTLVEntry) bool {
		entries = append(entries, e)
		return true
	})
	require.NoError(t, err)
	require.Len(t, entries, 2)

	assert.Equal(t, TLVNodeName, entries[0].Type)
	assert.Equal(t, []byte("test"), entries[0].Value)
	assert.Equal(t, TLVIGPMetric, entries[1].Type)
	assert.Equal(t, []byte{10}, entries[1].Value)
}

// TestTLVIteratorTruncated verifies error on truncated TLV data.
//
// VALIDATES: Iterator returns error when TLV length exceeds available data.
// PREVENTS: Out-of-bounds read on malformed input.
func TestTLVIteratorTruncated(t *testing.T) {
	// TLV header says 10 bytes but only 2 available
	data := make([]byte, 6)
	binary.BigEndian.PutUint16(data[0:], 9999)
	binary.BigEndian.PutUint16(data[2:], 10)
	data[4] = 0xFF
	data[5] = 0xFF

	err := IterateAttrTLVs(data, func(e AttrTLVEntry) bool {
		t.Fatal("should not yield entries on truncated data")
		return true
	})
	assert.ErrorIs(t, err, ErrBGPLSTruncated)
}

// TestTLVIteratorEmpty verifies no error on empty input.
//
// VALIDATES: Empty data produces no entries and no error.
// PREVENTS: Panic on nil/empty input.
func TestTLVIteratorEmpty(t *testing.T) {
	var count int
	err := IterateAttrTLVs(nil, func(e AttrTLVEntry) bool {
		count++
		return true
	})
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}

// TestTLVIteratorStopEarly verifies callback can stop iteration.
//
// VALIDATES: Returning false from callback stops iteration.
// PREVENTS: Iterator ignoring stop signal.
func TestTLVIteratorStopEarly(t *testing.T) {
	// Two TLVs: first one should be yielded, second should not
	data := make([]byte, 4+1+4+1)
	binary.BigEndian.PutUint16(data[0:], TLVNodeFlagBits)
	binary.BigEndian.PutUint16(data[2:], 1)
	data[4] = 0xFF
	binary.BigEndian.PutUint16(data[5:], TLVNodeFlagBits)
	binary.BigEndian.PutUint16(data[7:], 1)
	data[9] = 0x00

	var count int
	err := IterateAttrTLVs(data, func(e AttrTLVEntry) bool {
		count++
		return false // stop after first
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

// TestUnknownTLVDecode verifies unknown TLV codes return ErrUnknownAttrTLV.
//
// VALIDATES: DecodeAttrTLV returns sentinel error for unregistered codes.
// PREVENTS: Nil error on unknown TLV silently dropping data.
func TestUnknownTLVDecode(t *testing.T) {
	entry := AttrTLVEntry{Type: 65535, Value: []byte{0x01}}
	_, err := DecodeAttrTLV(entry)
	assert.ErrorIs(t, err, ErrUnknownAttrTLV)
}

// --- Round-trip tests for each TLV type ---

// tlvRoundTrip is a generic helper that encodes a TLV via WriteTo, then decodes via
// the iterator + registry, and returns the decoded TLV cast to the expected type.
func tlvRoundTrip[T LsAttrTLV](t *testing.T, original T) T {
	t.Helper()
	buf := make([]byte, original.Len())
	n := original.WriteTo(buf, 0)
	require.Equal(t, original.Len(), n, "WriteTo returned wrong length")

	var decoded LsAttrTLV
	err := IterateAttrTLVs(buf, func(e AttrTLVEntry) bool {
		var decErr error
		decoded, decErr = DecodeAttrTLV(e)
		require.NoError(t, decErr)
		return false
	})
	require.NoError(t, err)
	require.NotNil(t, decoded)
	result, ok := decoded.(T)
	require.True(t, ok, "decoded TLV has wrong type: got %T", decoded)
	return result
}

// TestNodeFlagBitsRoundTrip tests TLV 1024 encode/decode.
//
// VALIDATES: Node Flag Bits round-trip with non-zero flags.
// PREVENTS: Flag bit ordering errors.
func TestNodeFlagBitsRoundTrip(t *testing.T) {
	original := &LsNodeFlagBits{Flags: 0xEC} // O=1, T=1, E=1, B=0, R=1, V=1, RSV=0
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, original.Flags, decoded.Flags)

	j := decoded.ToJSON()
	flags, ok := j["node-flags"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1, flags["O"])
	assert.Equal(t, 1, flags["T"])
	assert.Equal(t, 1, flags["E"])
	assert.Equal(t, 0, flags["B"])
	assert.Equal(t, 1, flags["R"])
	assert.Equal(t, 1, flags["V"])
}

// TestOpaqueNodeAttrRoundTrip tests TLV 1025 encode/decode.
//
// VALIDATES: Opaque node attribute preserves arbitrary bytes.
// PREVENTS: Data corruption in opaque copy.
func TestOpaqueNodeAttrRoundTrip(t *testing.T) {
	original := &LsOpaqueNodeAttr{Data: []byte{0xDE, 0xAD, 0xBE, 0xEF}}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, original.Data, decoded.Data)
}

// TestNodeNameRoundTrip tests TLV 1026 encode/decode.
//
// VALIDATES: Node Name string round-trip.
// PREVENTS: String encoding issues.
func TestNodeNameRoundTrip(t *testing.T) {
	original := &LsNodeName{Name: "router1.example.com"}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, original.Name, decoded.Name)

	j := decoded.ToJSON()
	assert.Equal(t, "router1.example.com", j["node-name"])
}

// TestISISAreaIDRoundTrip tests TLV 1027 encode/decode.
//
// VALIDATES: IS-IS Area ID byte round-trip and hex JSON output.
// PREVENTS: Area ID data loss.
func TestISISAreaIDRoundTrip(t *testing.T) {
	original := &LsISISAreaID{AreaID: []byte{0x49, 0x00, 0x01}}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, original.AreaID, decoded.AreaID)

	j := decoded.ToJSON()
	assert.Equal(t, "0x490001", j["area-id"])
}

// TestIPv4RouterIDLocalRoundTrip tests TLV 1028 encode/decode.
//
// VALIDATES: IPv4 local router ID round-trip.
// PREVENTS: Address byte ordering error.
func TestIPv4RouterIDLocalRoundTrip(t *testing.T) {
	addr := netip.MustParseAddr("10.0.0.1")
	original := &LsIPv4RouterIDLocal{Addr: addr}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, addr, decoded.Addr)

	j := decoded.ToJSON()
	ids, ok := j["local-router-ids"].([]string)
	require.True(t, ok)
	assert.Equal(t, "10.0.0.1", ids[0])
}

// TestIPv6RouterIDLocalRoundTrip tests TLV 1029 encode/decode.
//
// VALIDATES: IPv6 local router ID round-trip.
// PREVENTS: 16-byte address truncation.
func TestIPv6RouterIDLocalRoundTrip(t *testing.T) {
	addr := netip.MustParseAddr("2001:db8::1")
	original := &LsIPv6RouterIDLocal{Addr: addr}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, addr, decoded.Addr)
}

// TestIPv4RouterIDRemoteRoundTrip tests TLV 1030 encode/decode.
//
// VALIDATES: IPv4 remote router ID round-trip.
// PREVENTS: Local/Remote ID confusion.
func TestIPv4RouterIDRemoteRoundTrip(t *testing.T) {
	addr := netip.MustParseAddr("172.16.0.1")
	original := &LsIPv4RouterIDRemote{Addr: addr}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, addr, decoded.Addr)

	j := decoded.ToJSON()
	ids, ok := j["remote-router-ids"].([]string)
	require.True(t, ok)
	assert.Equal(t, "172.16.0.1", ids[0])
}

// TestIPv6RouterIDRemoteRoundTrip tests TLV 1031 encode/decode.
//
// VALIDATES: IPv6 remote router ID round-trip.
// PREVENTS: 16-byte address truncation.
func TestIPv6RouterIDRemoteRoundTrip(t *testing.T) {
	addr := netip.MustParseAddr("fd00::2")
	original := &LsIPv6RouterIDRemote{Addr: addr}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, addr, decoded.Addr)
}

// TestAdminGroupRoundTrip tests TLV 1088 encode/decode.
//
// VALIDATES: Admin Group 4-byte mask round-trip.
// PREVENTS: Bit mask endianness error.
func TestAdminGroupRoundTrip(t *testing.T) {
	original := &LsAdminGroup{Mask: 0xDEADBEEF}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, uint32(0xDEADBEEF), decoded.Mask)
}

// TestMaxLinkBandwidthRoundTrip tests TLV 1089 encode/decode.
//
// VALIDATES: IEEE float32 bandwidth round-trip.
// PREVENTS: Float encoding error.
func TestMaxLinkBandwidthRoundTrip(t *testing.T) {
	original := &LsMaxLinkBandwidth{Bandwidth: 1.25e9} // 10 Gbps in bytes/sec
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, float32(1.25e9), decoded.Bandwidth)

	j := decoded.ToJSON()
	assert.InDelta(t, 1.25e9, j["max-link-bandwidth"], 1.0)
}

// TestMaxReservableBWRoundTrip tests TLV 1090 encode/decode.
//
// VALIDATES: Max reservable bandwidth float32 round-trip.
// PREVENTS: Wrong TLV code for reservable vs max bandwidth.
func TestMaxReservableBWRoundTrip(t *testing.T) {
	original := &LsMaxReservableBW{Bandwidth: 5e8}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, float32(5e8), decoded.Bandwidth)
}

// TestUnreservedBWRoundTrip tests TLV 1091 encode/decode.
//
// VALIDATES: 8 priority levels of unreserved bandwidth round-trip.
// PREVENTS: Priority level ordering or count error.
func TestUnreservedBWRoundTrip(t *testing.T) {
	bw := [8]float32{1e9, 9e8, 8e8, 7e8, 6e8, 5e8, 4e8, 3e8}
	original := &LsUnreservedBW{Bandwidth: bw}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, bw, decoded.Bandwidth)

	j := decoded.ToJSON()
	bws, ok := j["unreserved-bandwidth"].([]float64)
	require.True(t, ok)
	assert.Len(t, bws, 8)
	assert.InDelta(t, 1e9, bws[0], 1.0)
}

// TestTEDefaultMetricRoundTrip tests TLV 1092 encode/decode.
//
// VALIDATES: TE default metric uint32 round-trip.
// PREVENTS: Metric value truncation.
func TestTEDefaultMetricRoundTrip(t *testing.T) {
	original := &LsTEDefaultMetric{Metric: 12345}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, uint32(12345), decoded.Metric)
}

// TestIGPMetricVariableLengths tests TLV 1095 with all encoding lengths.
//
// VALIDATES: IGP metric decodes correctly for 1/2/3 byte encodings per RFC 7752.
// PREVENTS: Wrong mask for IS-IS small metric (6 bits), wrong byte count.
func TestIGPMetricVariableLengths(t *testing.T) {
	tests := []struct {
		name    string
		metric  uint32
		wireLen int
	}{
		{"1-byte IS-IS small (6 bits)", 0x3F, 1},
		{"2-byte OSPF", 1000, 2},
		{"3-byte IS-IS wide", 0x0ABCDE, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := &LsIGPMetric{Metric: tt.metric, wireLen: tt.wireLen}
			decoded := tlvRoundTrip(t, original)
			assert.Equal(t, tt.metric, decoded.Metric)
			assert.Equal(t, tt.wireLen, decoded.wireLen)
		})
	}
}

// TestSRLGRoundTrip tests TLV 1096 encode/decode.
//
// VALIDATES: SRLG array of uint32 values round-trip.
// PREVENTS: Array element ordering or count error.
func TestSRLGRoundTrip(t *testing.T) {
	original := &LsSRLG{Values: []uint32{100, 200, 300}}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, original.Values, decoded.Values)
}

// TestOpaqueLinkAttrRoundTrip tests TLV 1097 encode/decode.
//
// VALIDATES: Opaque link attribute data round-trip.
// PREVENTS: Data corruption.
func TestOpaqueLinkAttrRoundTrip(t *testing.T) {
	original := &LsOpaqueLinkAttr{Data: []byte{0xCA, 0xFE}}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, original.Data, decoded.Data)
}

// TestLinkNameRoundTrip tests TLV 1098 encode/decode.
//
// VALIDATES: Link name string round-trip.
// PREVENTS: String encoding issues.
func TestLinkNameRoundTrip(t *testing.T) {
	original := &LsLinkName{Name: "ge-0/0/1"}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, "ge-0/0/1", decoded.Name)
}

// TestIGPFlagsRoundTrip tests TLV 1152 encode/decode.
//
// VALIDATES: IGP Flags byte round-trip with non-zero flags.
// PREVENTS: Flag bit position errors.
func TestIGPFlagsRoundTrip(t *testing.T) {
	original := &LsIGPFlags{Flags: 0xE0} // D=1, N=1, L=1
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, uint8(0xE0), decoded.Flags)

	j := decoded.ToJSON()
	flags, ok := j["igp-flags"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1, flags["D"])
	assert.Equal(t, 1, flags["N"])
	assert.Equal(t, 1, flags["L"])
}

// TestPrefixMetricRoundTrip tests TLV 1155 encode/decode.
//
// VALIDATES: Prefix metric uint32 round-trip.
// PREVENTS: Metric value confusion with IGP metric.
func TestPrefixMetricRoundTrip(t *testing.T) {
	original := &LsPrefixMetric{Metric: 42}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, uint32(42), decoded.Metric)
}

// TestOpaquePrefixAttrRoundTrip tests TLV 1157 encode/decode.
//
// VALIDATES: Opaque prefix attribute data round-trip.
// PREVENTS: Data corruption.
func TestOpaquePrefixAttrRoundTrip(t *testing.T) {
	original := &LsOpaquePrefixAttr{Data: []byte{0xAB, 0xCD}}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, original.Data, decoded.Data)
}

// --- Boundary tests ---

// TestIGPMetricBoundary tests boundary values for IGP metric encoding.
//
// VALIDATES: Last valid values for each encoding width.
// PREVENTS: Off-by-one in encoding width selection.
func TestIGPMetricBoundary(t *testing.T) {
	tests := []struct {
		name    string
		metric  uint32
		wantLen int // value length (without TLV header)
	}{
		{"zero", 0, 1},
		{"max 6-bit (0x3F)", 0x3F, 1},
		{"first 2-byte (0x40)", 0x40, 2},
		{"max 2-byte (0xFFFF)", 0xFFFF, 2},
		{"first 3-byte (0x10000)", 0x10000, 3},
		{"max 3-byte (0xFFFFFF)", 0xFFFFFF, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &LsIGPMetric{Metric: tt.metric}
			assert.Equal(t, 4+tt.wantLen, m.Len(), "unexpected Len()")

			decoded := tlvRoundTrip(t, m)
			assert.Equal(t, tt.metric, decoded.Metric)
		})
	}
}

// TestBandwidthMaxFloat32 tests maximum IEEE float32 value.
//
// VALIDATES: Max float32 bandwidth encodes and decodes correctly.
// PREVENTS: Float overflow in encoding.
func TestBandwidthMaxFloat32(t *testing.T) {
	original := &LsMaxLinkBandwidth{Bandwidth: math.MaxFloat32}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, float32(math.MaxFloat32), decoded.Bandwidth)
}

// TestDecodeAllAttrTLVs verifies full decode of multiple TLVs.
//
// VALIDATES: DecodeAllAttrTLVs returns all recognized TLVs.
// PREVENTS: Missing TLVs in batch decode.
func TestDecodeAllAttrTLVs(t *testing.T) {
	// Build wire: Node Name "r1" + IGP Metric 10 + unknown TLV 9999
	data := make([]byte, 4+2+4+1+4+2)
	off := 0
	binary.BigEndian.PutUint16(data[off:], TLVNodeName)
	binary.BigEndian.PutUint16(data[off+2:], 2)
	copy(data[off+4:], "r1")
	off += 6
	binary.BigEndian.PutUint16(data[off:], TLVIGPMetric)
	binary.BigEndian.PutUint16(data[off+2:], 1)
	data[off+4] = 10
	off += 5
	binary.BigEndian.PutUint16(data[off:], 9999)
	binary.BigEndian.PutUint16(data[off+2:], 2)
	data[off+4] = 0xFF
	data[off+5] = 0xFF

	tlvs, err := DecodeAllAttrTLVs(data)
	require.NoError(t, err)
	// Unknown TLV 9999 is not in the result (only recognized ones)
	assert.Len(t, tlvs, 2)
	assert.Equal(t, TLVNodeName, tlvs[0].Code())
	assert.Equal(t, TLVIGPMetric, tlvs[1].Code())
}

// TestAttrTLVsToJSON verifies JSON output with known and unknown TLVs.
//
// VALIDATES: AttrTLVsToJSON produces correct JSON for known TLVs and generic hex for unknown.
// PREVENTS: Missing JSON keys, wrong generic key format.
func TestAttrTLVsToJSON(t *testing.T) {
	// Build wire: Node Name "test" + unknown TLV 8888
	data := make([]byte, 4+4+4+1)
	binary.BigEndian.PutUint16(data[0:], TLVNodeName)
	binary.BigEndian.PutUint16(data[2:], 4)
	copy(data[4:8], "test")
	binary.BigEndian.PutUint16(data[8:], 8888)
	binary.BigEndian.PutUint16(data[10:], 1)
	data[12] = 0xAB

	result := AttrTLVsToJSON(data)
	assert.Equal(t, "test", result["node-name"])
	assert.Equal(t, []string{"0xAB"}, result["generic-lsid-8888"])
}

// TestMergeRouterIDs verifies that multiple router ID TLVs merge correctly.
//
// VALIDATES: Two IPv4 local router IDs merge into one array.
// PREVENTS: Second router ID overwriting first.
func TestMergeRouterIDs(t *testing.T) {
	addr1 := netip.MustParseAddr("10.0.0.1")
	addr2 := netip.MustParseAddr("10.0.0.2")

	tlv1 := &LsIPv4RouterIDLocal{Addr: addr1}
	tlv2 := &LsIPv4RouterIDLocal{Addr: addr2}

	// Build wire with both TLVs
	buf := make([]byte, tlv1.Len()+tlv2.Len())
	off := tlv1.WriteTo(buf, 0)
	tlv2.WriteTo(buf, off)

	result := AttrTLVsToJSON(buf)
	ids, ok := result["local-router-ids"].([]string)
	require.True(t, ok)
	assert.Len(t, ids, 2)
	assert.Equal(t, "10.0.0.1", ids[0])
	assert.Equal(t, "10.0.0.2", ids[1])
}

// --- Phase 2: SR-MPLS round-trip tests ---

// TestSRCapabilitiesRoundTrip tests TLV 1034 encode/decode.
//
// VALIDATES: SR Capabilities with nested SID/Label sub-TLVs round-trip.
// PREVENTS: Sub-TLV nesting, 3-byte range encoding, label range parse errors.
func TestSRCapabilitiesRoundTrip(t *testing.T) {
	original := &LsSRCapabilities{
		Flags: 0xC0, // I=1, V=1
		Ranges: []LsSrLabelRange{
			{Range: 8000, FirstSID: 16000, sidLen: 4},
			{Range: 1000, FirstSID: 100000, sidLen: 3},
		},
	}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, uint8(0xC0), decoded.Flags)
	require.Len(t, decoded.Ranges, 2)
	assert.Equal(t, uint32(8000), decoded.Ranges[0].Range)
	assert.Equal(t, uint32(16000), decoded.Ranges[0].FirstSID)
	assert.Equal(t, uint32(1000), decoded.Ranges[1].Range)
	// 3-byte label: 20-bit mask applied
	assert.Equal(t, uint32(100000)&0xFFFFF, decoded.Ranges[1].FirstSID)

	j := decoded.ToJSON()
	caps, ok := j["sr-capabilities"].(map[string]any)
	require.True(t, ok)
	flags, ok := caps["flags"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1, flags["I"])
	assert.Equal(t, 1, flags["V"])
}

// TestSRAlgorithmRoundTrip tests TLV 1035 encode/decode.
//
// VALIDATES: SR Algorithm byte array round-trip.
// PREVENTS: Algorithm ID ordering or loss.
func TestSRAlgorithmRoundTrip(t *testing.T) {
	original := &LsSRAlgorithm{Algorithms: []uint8{0, 1, 128}}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, []uint8{0, 1, 128}, decoded.Algorithms)

	j := decoded.ToJSON()
	algos, ok := j["sr-algorithms"].([]int)
	require.True(t, ok)
	assert.Equal(t, []int{0, 1, 128}, algos)
}

// TestSRLocalBlockRoundTrip tests TLV 1036 encode/decode.
//
// VALIDATES: SR Local Block with label ranges round-trip.
// PREVENTS: SRLB/SRGB confusion, flags always 0 per RFC 9085.
func TestSRLocalBlockRoundTrip(t *testing.T) {
	original := &LsSRLocalBlock{
		Flags:  0, // RFC 9085 Section 5: MUST be 0
		Ranges: []LsSrLabelRange{{Range: 1000, FirstSID: 15000, sidLen: 4}},
	}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, uint8(0), decoded.Flags)
	require.Len(t, decoded.Ranges, 1)
	assert.Equal(t, uint32(1000), decoded.Ranges[0].Range)
	assert.Equal(t, uint32(15000), decoded.Ranges[0].FirstSID)
}

// TestAdjacencySIDRoundTrip tests TLV 1099 encode/decode.
//
// VALIDATES: Adjacency SID with V/L flag combos (label vs index).
// PREVENTS: Wrong SID length based on V/L flags.
func TestAdjacencySIDRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		flags   uint8
		weight  uint8
		sid     uint32
		wireLen int
	}{
		{"V=0 L=0 index", 0x00, 10, 100000, 8},
		{"V=1 L=1 label", 0x30, 20, 16001, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := &LsAdjacencySID{
				Flags:   tt.flags,
				Weight:  tt.weight,
				SID:     tt.sid,
				wireLen: tt.wireLen,
			}
			decoded := tlvRoundTrip(t, original)
			assert.Equal(t, tt.flags, decoded.Flags)
			assert.Equal(t, tt.weight, decoded.Weight)
			if tt.wireLen == 7 {
				// 3-byte label: 20-bit mask
				assert.Equal(t, tt.sid&0xFFFFF, decoded.SID)
			} else {
				assert.Equal(t, tt.sid, decoded.SID)
			}
		})
	}
}

// TestPrefixSIDRoundTrip tests TLV 1158 encode/decode.
//
// VALIDATES: Prefix SID with flags, algorithm, and SID (label vs index).
// PREVENTS: Flags/algorithm byte position errors.
func TestPrefixSIDRoundTrip(t *testing.T) {
	original := &LsPrefixSID{
		Flags:     0x40, // N-flag set
		Algorithm: 1,
		SID:       24000,
		wireLen:   8,
	}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, uint8(0x40), decoded.Flags)
	assert.Equal(t, uint8(1), decoded.Algorithm)
	assert.Equal(t, uint32(24000), decoded.SID)

	j := decoded.ToJSON()
	ps, ok := j["prefix-sid"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1, ps["algorithm"])
}

// TestSIDLabelRoundTrip tests TLV 1161 encode/decode.
//
// VALIDATES: SID/Label with 3-byte and 4-byte encodings.
// PREVENTS: Label 20-bit masking errors.
func TestSIDLabelRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		sid     uint32
		wireLen int
	}{
		{"4-byte index", 100000, 4},
		{"3-byte label", 16001, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := &LsSIDLabel{SID: tt.sid, wireLen: tt.wireLen}
			decoded := tlvRoundTrip(t, original)
			if tt.wireLen == 3 {
				assert.Equal(t, tt.sid&0xFFFFF, decoded.SID)
			} else {
				assert.Equal(t, tt.sid, decoded.SID)
			}
		})
	}
}

// TestSRPrefixFlagsRoundTrip tests TLV 1170 encode/decode.
//
// VALIDATES: SR Prefix Attribute Flags round-trip.
// PREVENTS: Flag bit position errors (X/R/N).
func TestSRPrefixFlagsRoundTrip(t *testing.T) {
	original := &LsSRPrefixFlags{Flags: 0xE0} // X=1, R=1, N=1
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, uint8(0xE0), decoded.Flags)

	j := decoded.ToJSON()
	flags, ok := j["sr-prefix-flags"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 1, flags["X"])
	assert.Equal(t, 1, flags["R"])
	assert.Equal(t, 1, flags["N"])
}

// TestSourceRouterIDRoundTrip tests TLV 1171 encode/decode.
//
// VALIDATES: Source Router ID with IPv4 and IPv6 variants.
// PREVENTS: Wrong length check (must be exactly 4 or 16).
func TestSourceRouterIDRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		id   []byte
		json string
	}{
		{"IPv4", []byte{10, 0, 0, 1}, "10.0.0.1"},
		{"IPv6", []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, "2001:db8::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := &LsSourceRouterID{ID: tt.id}
			decoded := tlvRoundTrip(t, original)
			assert.Equal(t, tt.id, decoded.ID)

			j := decoded.ToJSON()
			assert.Equal(t, tt.json, j["source-router-id"])
		})
	}
}

// TestPhase2Registration verifies all Phase 2 TLV decoders are registered.
//
// VALIDATES: Phase 2 init() registration adds 8 more TLV codes.
// PREVENTS: Missing Phase 2 registration.
func TestPhase2Registration(t *testing.T) {
	phase2Codes := []uint16{
		TLVSRCapabilities, TLVSRAlgorithm, TLVSRLocalBlock,
		TLVAdjacencySID,
		TLVPrefixSID, TLVSIDLabel, TLVSRPrefixFlags, TLVSourceRouterID,
	}
	for _, code := range phase2Codes {
		decoder := LookupLsAttrTLVDecoder(code)
		assert.NotNilf(t, decoder, "no decoder registered for TLV code %d", code)
	}
	// Phase 1 (20) + Phase 2 (8) = 28
	assert.GreaterOrEqual(t, RegisteredLsAttrTLVCount(), 28)
}

// --- Phase 3: EPE + Delay + SRv6 + Descriptor tests ---

// TestPeerNodeSIDRoundTrip tests TLV 1101 encode/decode.
//
// VALIDATES: Peer Node SID with 4-byte index round-trip.
// PREVENTS: Wrong TLV code in PeerSID dispatch.
func TestPeerNodeSIDRoundTrip(t *testing.T) {
	original := &LsPeerSID{TLVCode: TLVPeerNodeSID, Flags: 0xC0, Weight: 5, SID: 24000, wireLen: 8}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, TLVPeerNodeSID, decoded.TLVCode)
	assert.Equal(t, uint8(0xC0), decoded.Flags)
	assert.Equal(t, uint8(5), decoded.Weight)
	assert.Equal(t, uint32(24000), decoded.SID)

	j := decoded.ToJSON()
	_, ok := j["peer-node-sid"].(map[string]any)
	require.True(t, ok)
}

// TestPeerAdjSIDRoundTrip tests TLV 1102 encode/decode.
//
// VALIDATES: Peer Adjacency SID with 3-byte label.
// PREVENTS: Label 20-bit masking error in peer SID.
func TestPeerAdjSIDRoundTrip(t *testing.T) {
	original := &LsPeerSID{TLVCode: TLVPeerAdjSID, Flags: 0x80, Weight: 10, SID: 16001, wireLen: 7}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, TLVPeerAdjSID, decoded.TLVCode)
	assert.Equal(t, uint32(16001)&0xFFFFF, decoded.SID)
}

// TestSRv6EndXSIDRoundTrip tests TLV 1106 encode/decode.
//
// VALIDATES: SRv6 End.X SID with behavior, flags, SID, and SID structure sub-TLV.
// PREVENTS: Nested sub-TLV parse failure, SID byte ordering.
func TestSRv6EndXSIDRoundTrip(t *testing.T) {
	original := &LsSRv6EndXSID{
		TLVCode:          TLVSRv6EndXSID,
		EndpointBehavior: 42,
		Flags:            0xE0,
		Algorithm:        1,
		Weight:           10,
		SID:              [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		SIDStructure:     [4]uint8{32, 16, 16, 0},
		hasSIDStructure:  true,
	}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, uint16(42), decoded.EndpointBehavior)
	assert.Equal(t, uint8(0xE0), decoded.Flags)
	assert.Equal(t, uint8(1), decoded.Algorithm)
	assert.Equal(t, original.SID, decoded.SID)
	assert.True(t, decoded.hasSIDStructure)
	assert.Equal(t, [4]uint8{32, 16, 16, 0}, decoded.SIDStructure)
}

// TestSRv6LANEndXISISRoundTrip tests TLV 1107 with 6-byte neighbor ID.
//
// VALIDATES: IS-IS LAN End.X SID with neighbor ID round-trip.
// PREVENTS: Neighbor ID length mismatch.
func TestSRv6LANEndXISISRoundTrip(t *testing.T) {
	original := &LsSRv6EndXSID{
		TLVCode:          TLVSRv6LANEndXISIS,
		EndpointBehavior: 6,
		Flags:            0x40,
		Algorithm:        0,
		Weight:           5,
		NeighborID:       []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
		SID:              [16]byte{0xfd, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x02},
	}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, TLVSRv6LANEndXISIS, decoded.TLVCode)
	assert.Equal(t, []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06}, decoded.NeighborID)
	assert.Equal(t, original.SID, decoded.SID)
}

// TestPeerSetSIDRoundTrip tests TLV 1103 encode/decode.
//
// VALIDATES: Peer Set SID with 4-byte index round-trip.
// PREVENTS: Missing Peer Set SID variant.
func TestPeerSetSIDRoundTrip(t *testing.T) {
	original := &LsPeerSID{TLVCode: TLVPeerSetSID, Flags: 0x00, Weight: 1, SID: 50000, wireLen: 8}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, TLVPeerSetSID, decoded.TLVCode)
	assert.Equal(t, uint32(50000), decoded.SID)

	j := decoded.ToJSON()
	_, ok := j["peer-set-sid"].(map[string]any)
	require.True(t, ok)
}

// TestSRv6LANEndXOSPFRoundTrip tests TLV 1108 with 4-byte neighbor ID.
//
// VALIDATES: OSPFv3 LAN End.X SID with 4-byte neighbor ID round-trip.
// PREVENTS: OSPFv3 neighbor ID length mismatch.
func TestSRv6LANEndXOSPFRoundTrip(t *testing.T) {
	original := &LsSRv6EndXSID{
		TLVCode:          TLVSRv6LANEndXOSPF,
		EndpointBehavior: 9,
		Flags:            0x20,
		Algorithm:        0,
		Weight:           3,
		NeighborID:       []byte{0x0A, 0x00, 0x00, 0x01},
		SID:              [16]byte{0xfd, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x03},
	}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, TLVSRv6LANEndXOSPF, decoded.TLVCode)
	assert.Equal(t, []byte{0x0A, 0x00, 0x00, 0x01}, decoded.NeighborID)
	assert.Equal(t, original.SID, decoded.SID)
}

// TestUnidirectionalDelayRoundTrip tests TLV 1114 encode/decode.
//
// VALIDATES: 24-bit delay in microseconds with A flag.
// PREVENTS: A flag bit position error, 24-bit value extraction.
func TestUnidirectionalDelayRoundTrip(t *testing.T) {
	original := &LsUnidirectionalDelay{Anomalous: true, Delay: 500}
	decoded := tlvRoundTrip(t, original)
	assert.True(t, decoded.Anomalous)
	assert.Equal(t, uint32(500), decoded.Delay)
}

// TestMinMaxDelayRoundTrip tests TLV 1115 encode/decode.
//
// VALIDATES: Min/max delay with A flag round-trip.
// PREVENTS: Min/max value swap.
func TestMinMaxDelayRoundTrip(t *testing.T) {
	original := &LsMinMaxDelay{Anomalous: false, MinDelay: 100, MaxDelay: 5000}
	decoded := tlvRoundTrip(t, original)
	assert.False(t, decoded.Anomalous)
	assert.Equal(t, uint32(100), decoded.MinDelay)
	assert.Equal(t, uint32(5000), decoded.MaxDelay)
}

// TestDelayVariationRoundTrip tests TLV 1116 encode/decode.
//
// VALIDATES: Delay variation 24-bit value round-trip.
// PREVENTS: Reserved byte corruption.
func TestDelayVariationRoundTrip(t *testing.T) {
	original := &LsDelayVariation{Variation: 42}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, uint32(42), decoded.Variation)
}

// TestSRv6EndpointBehaviorRoundTrip tests TLV 1250 encode/decode.
//
// VALIDATES: SRv6 Endpoint Behavior fields round-trip.
// PREVENTS: Field byte ordering.
func TestSRv6EndpointBehaviorRoundTrip(t *testing.T) {
	original := &LsSRv6EndpointBehavior{EndpointBehavior: 48, Flags: 0x80, Algorithm: 1}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, uint16(48), decoded.EndpointBehavior)
	assert.Equal(t, uint8(0x80), decoded.Flags)
	assert.Equal(t, uint8(1), decoded.Algorithm)
}

// TestSRv6BGPPeerNodeSIDRoundTrip tests TLV 1251 encode/decode.
//
// VALIDATES: SRv6 BGP Peer Node SID with all fields round-trip.
// PREVENTS: PeerAS/PeerBGPID byte position errors.
func TestSRv6BGPPeerNodeSIDRoundTrip(t *testing.T) {
	original := &LsSRv6BGPPeerNodeSID{
		Flags:     0x40,
		Weight:    3,
		PeerAS:    65001,
		PeerBGPID: netip.MustParseAddr("10.0.0.1"),
	}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, uint8(0x40), decoded.Flags)
	assert.Equal(t, uint8(3), decoded.Weight)
	assert.Equal(t, uint32(65001), decoded.PeerAS)
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), decoded.PeerBGPID)
}

// TestSRv6SIDStructureAttrRoundTrip tests TLV 1252 as standalone attribute.
//
// VALIDATES: SRv6 SID Structure 4-byte fields round-trip.
// PREVENTS: Field ordering.
func TestSRv6SIDStructureAttrRoundTrip(t *testing.T) {
	original := &LsSRv6SIDStructureAttr{LocBlockLen: 40, LocNodeLen: 24, FuncLen: 16, ArgLen: 0}
	decoded := tlvRoundTrip(t, original)
	assert.Equal(t, uint8(40), decoded.LocBlockLen)
	assert.Equal(t, uint8(24), decoded.LocNodeLen)
	assert.Equal(t, uint8(16), decoded.FuncLen)
	assert.Equal(t, uint8(0), decoded.ArgLen)
}

// TestDescriptorBGPRouterID tests descriptor sub-TLV 516 round-trip.
//
// VALIDATES: BGP Router-ID in NodeDescriptor encodes and parses.
// PREVENTS: Missing TLV 516 in descriptor encoding/parsing.
func TestDescriptorBGPRouterID(t *testing.T) {
	nd := NodeDescriptor{
		ASN:         65001,
		BGPRouterID: 0x0A000001, // 10.0.0.1 as uint32
	}

	buf := make([]byte, nd.Len())
	nd.WriteTo(buf, 0)

	var parsed NodeDescriptor
	err := parseNodeDescriptorTLVs(buf, &parsed)
	require.NoError(t, err)
	assert.Equal(t, uint32(65001), parsed.ASN)
	assert.Equal(t, uint32(0x0A000001), parsed.BGPRouterID)
}

// TestDescriptorConfedMember tests descriptor sub-TLV 517 round-trip.
//
// VALIDATES: Confederation Member in NodeDescriptor encodes and parses.
// PREVENTS: Missing TLV 517 in descriptor encoding/parsing.
func TestDescriptorConfedMember(t *testing.T) {
	nd := NodeDescriptor{
		ASN:          65001,
		ConfedMember: 65100,
	}

	buf := make([]byte, nd.Len())
	nd.WriteTo(buf, 0)

	var parsed NodeDescriptor
	err := parseNodeDescriptorTLVs(buf, &parsed)
	require.NoError(t, err)
	assert.Equal(t, uint32(65001), parsed.ASN)
	assert.Equal(t, uint32(65100), parsed.ConfedMember)
}

// TestDescriptorSRv6SID tests descriptor sub-TLV 518 round-trip.
//
// VALIDATES: SRv6 SID in NodeDescriptor encodes and parses (16-byte IPv6 address).
// PREVENTS: Missing TLV 518 in node descriptor, matching GoBGP behavior.
func TestDescriptorSRv6SID(t *testing.T) {
	sid := []byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}
	nd := NodeDescriptor{
		ASN:      65001,
		SRv6SIDs: [][]byte{sid},
	}

	buf := make([]byte, nd.Len())
	nd.WriteTo(buf, 0)

	var parsed NodeDescriptor
	err := parseNodeDescriptorTLVs(buf, &parsed)
	require.NoError(t, err)
	assert.Equal(t, uint32(65001), parsed.ASN)
	require.Len(t, parsed.SRv6SIDs, 1)
	assert.Equal(t, sid, parsed.SRv6SIDs[0])
}

// TestPhase3Registration verifies all Phase 3 TLV decoders are registered.
//
// VALIDATES: Phase 3 registration adds 12 more TLV codes.
// PREVENTS: Missing Phase 3 registration.
func TestPhase3Registration(t *testing.T) {
	phase3Codes := []uint16{
		TLVPeerNodeSID, TLVPeerAdjSID, TLVPeerSetSID,
		TLVSRv6EndXSID, TLVSRv6LANEndXISIS, TLVSRv6LANEndXOSPF,
		TLVUnidirectionalDelay, TLVMinMaxDelay, TLVDelayVariation,
		TLVSRv6EndpointBehavior, TLVSRv6BGPPeerNodeSID, TLVSRv6SIDStructure,
	}
	for _, code := range phase3Codes {
		decoder := LookupLsAttrTLVDecoder(code)
		assert.NotNilf(t, decoder, "no decoder registered for TLV code %d", code)
	}
	// Phase 1 (20) + Phase 2 (8) + Phase 3 (12) = 40
	assert.GreaterOrEqual(t, RegisteredLsAttrTLVCount(), 40)
}
