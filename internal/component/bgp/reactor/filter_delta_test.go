// Design: docs/architecture/core-design.md -- policy filter wire-level dirty tracking tests
package reactor

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

// TestExtractLegacyNLRIOverride covers the per-prefix modify path helper for
// cmd-4 phase 2 (plan/learned/552). The helper compares the "nlri
// ipv4/unicast add ..." block in two filter-text strings and returns
// wire-encoded NLRI bytes for the modified prefix list, or nil when no
// IPv4-unicast rewrite is needed.
//
// VALIDATES: per-prefix partition path emits the correct wire NLRI bytes so
//
//	buildModifiedPayload can splice them into the payload tail.
//
// PREVENTS:  regression where a filter plugin returns action=modify with a
//
//	smaller prefix list but the engine still forwards the original
//	full prefix list on the wire.
func TestExtractLegacyNLRIOverride(t *testing.T) {
	tests := []struct {
		name     string
		original string
		modified string
		want     []byte // nil means "override should be nil"; [] means "empty override"
	}{
		{
			name:     "unchanged nlri returns nil override",
			original: "origin igp nlri ipv4/unicast add 10.0.0.0/24",
			modified: "origin igp nlri ipv4/unicast add 10.0.0.0/24",
			want:     nil,
		},
		{
			name:     "subset rewrite encodes accepted prefixes",
			original: "origin igp nlri ipv4/unicast add 10.0.0.0/24 10.0.1.0/24 192.168.1.0/24",
			modified: "origin igp nlri ipv4/unicast add 10.0.0.0/24 192.168.1.0/24",
			want:     []byte{0x18, 0x0A, 0x00, 0x00, 0x18, 0xC0, 0xA8, 0x01},
		},
		{
			name:     "all denied yields empty non-nil override",
			original: "origin igp nlri ipv4/unicast add 10.0.0.0/24 10.0.1.0/24",
			modified: "origin igp",
			want:     []byte{},
		},
		{
			name:     "zero-length prefix encodes length byte only",
			original: "origin igp nlri ipv4/unicast add 10.0.0.0/24",
			modified: "origin igp nlri ipv4/unicast add 0.0.0.0/0",
			want:     []byte{0x00},
		},
		{
			name:     "sub-byte prefix length rounds up",
			original: "origin igp nlri ipv4/unicast add 10.0.0.0/24",
			modified: "origin igp nlri ipv4/unicast add 10.0.0.0/20",
			want:     []byte{0x14, 0x0A, 0x00, 0x00},
		},
		{
			name:     "ipv6 unicast rewrite is ignored (MP_REACH out of scope)",
			original: "origin igp nlri ipv6/unicast add 2001:db8::/32 2001:db8:1::/48",
			modified: "origin igp nlri ipv6/unicast add 2001:db8::/32",
			want:     nil,
		},
		{
			name:     "mixed families: only ipv4 unicast subset triggers override",
			original: "origin igp nlri ipv4/unicast add 10.0.0.0/24 10.0.1.0/24 nlri ipv6/unicast add 2001:db8::/32",
			modified: "origin igp nlri ipv4/unicast add 10.0.0.0/24 nlri ipv6/unicast add 2001:db8::/32",
			want:     []byte{0x18, 0x0A, 0x00, 0x00},
		},
		{
			name:     "no nlri field in either returns nil",
			original: "origin igp",
			modified: "origin igp local-preference 200",
			want:     nil,
		},
		{
			name:     "malformed prefix token returns nil fail-closed",
			original: "origin igp nlri ipv4/unicast add 10.0.0.0/24",
			modified: "origin igp nlri ipv4/unicast add not-a-prefix",
			want:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractLegacyNLRIOverride(tt.original, tt.modified)
			if tt.want == nil {
				assert.Nil(t, got, "expected nil override")
				return
			}
			if len(tt.want) == 0 {
				assert.NotNil(t, got, "expected empty non-nil override")
				assert.Equal(t, 0, len(got))
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestTextDeltaToModOps verifies that text delta comparison produces correct AttrModSet ops.
//
// VALIDATES: Wire-level dirty tracking -- text delta converted to ModAccumulator ops.
// PREVENTS: Modified text silently discarded without updating wire bytes.
func TestTextDeltaToModOps(t *testing.T) {
	tests := []struct {
		name       string
		original   string
		modified   string
		wantOps    int
		wantCode   attribute.AttributeCode
		wantNilBuf bool // true for removal ops (Buf is nil)
	}{
		{
			name:     "no change produces no ops",
			original: "origin igp local-preference 100",
			modified: "origin igp local-preference 100",
			wantOps:  0,
		},
		{
			name:     "local-pref changed",
			original: "origin igp local-preference 100",
			modified: "origin igp local-preference 200",
			wantOps:  1,
			wantCode: attribute.AttrLocalPref,
		},
		{
			name:     "community added",
			original: "origin igp",
			modified: "origin igp community 65000:100",
			wantOps:  1,
			wantCode: attribute.AttrCommunity,
		},
		{
			name:     "med changed",
			original: "origin igp med 100",
			modified: "origin igp med 200",
			wantOps:  1,
			wantCode: attribute.AttrMED,
		},
		{
			name:     "origin changed",
			original: "origin igp",
			modified: "origin egp",
			wantOps:  1,
			wantCode: attribute.AttrOrigin,
		},
		{
			name:     "multiple attributes changed",
			original: "origin igp local-preference 100 med 50",
			modified: "origin igp local-preference 200 med 100",
			wantOps:  2,
		},
		{
			name:     "nlri changes ignored",
			original: "origin igp nlri ipv4/unicast add 10.0.0.0/24",
			modified: "origin igp nlri ipv4/unicast add 10.0.1.0/24",
			wantOps:  0,
		},
		{
			name:     "as-path changes skipped",
			original: "origin igp as-path 65001 65002",
			modified: "origin igp as-path 65001",
			wantOps:  0,
		},
		{
			name:       "attribute removed produces op",
			original:   "origin igp community 65000:100",
			modified:   "origin igp",
			wantOps:    1,
			wantCode:   attribute.AttrCommunity,
			wantNilBuf: true,
		},
		{
			name:     "empty delta",
			original: "",
			modified: "",
			wantOps:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mods registry.ModAccumulator
			textDeltaToModOps(tt.original, tt.modified, &mods)
			assert.Equal(t, tt.wantOps, mods.Len())
			if tt.wantOps == 1 {
				ops := mods.Ops()
				assert.Equal(t, byte(tt.wantCode), ops[0].Code)
				assert.Equal(t, registry.AttrModSet, ops[0].Action)
				if tt.wantNilBuf {
					assert.Nil(t, ops[0].Buf, "removal op should have nil Buf")
				} else {
					assert.NotEmpty(t, ops[0].Buf, "wire value bytes must not be empty")
				}
			}
		})
	}
}

// TestEncodeAttrValueLocalPref verifies local-preference encoding to 4-byte big-endian.
//
// VALIDATES: Text "200" encodes to wire bytes [0,0,0,200].
// PREVENTS: Encoding producing wrong byte order or length.
func TestEncodeAttrValueLocalPref(t *testing.T) {
	buf, err := encodeAttrValue("local-preference", "200")
	require.NoError(t, err)
	require.Len(t, buf, 4)
	val := binary.BigEndian.Uint32(buf)
	assert.Equal(t, uint32(200), val)
}

// TestEncodeAttrValueOrigin verifies origin encoding.
//
// VALIDATES: "igp"->0, "egp"->1, "incomplete"->2.
// PREVENTS: Wrong origin byte value.
func TestEncodeAttrValueOrigin(t *testing.T) {
	tests := []struct {
		text string
		want byte
	}{
		{"igp", 0},
		{"egp", 1},
		{"incomplete", 2},
	}
	for _, tt := range tests {
		buf, err := encodeAttrValue("origin", tt.text)
		require.NoError(t, err)
		require.Len(t, buf, 1)
		assert.Equal(t, tt.want, buf[0], "origin=%s", tt.text)
	}
}

// TestEncodeAttrValueMED verifies MED encoding to 4-byte big-endian.
//
// VALIDATES: Text "500" encodes correctly.
// PREVENTS: MED encoding error.
func TestEncodeAttrValueMED(t *testing.T) {
	buf, err := encodeAttrValue("med", "500")
	require.NoError(t, err)
	require.Len(t, buf, 4)
	assert.Equal(t, uint32(500), binary.BigEndian.Uint32(buf))
}

// TestEncodeAttrValueNextHop verifies next-hop encoding to 4 IPv4 bytes.
//
// VALIDATES: "10.0.0.1" encodes to [10,0,0,1].
// PREVENTS: Next-hop encoding error.
func TestEncodeAttrValueNextHop(t *testing.T) {
	buf, err := encodeAttrValue("next-hop", "10.0.0.1")
	require.NoError(t, err)
	require.Len(t, buf, 4)
	assert.Equal(t, []byte{10, 0, 0, 1}, buf)
}

// TestEncodeAttrValueASPath verifies AS_PATH encoding to wire segment format.
//
// VALIDATES: "65001 65002" encodes to AS_SEQUENCE segment with two 4-byte ASNs.
// PREVENTS: AS_PATH encoding error.
func TestEncodeAttrValueASPath(t *testing.T) {
	buf, err := encodeAttrValue("as-path", "65001 65002")
	require.NoError(t, err)
	// type(1) + count(1) + 2*ASN(4) = 10 bytes
	require.Len(t, buf, 10)
	assert.Equal(t, byte(attribute.ASSequence), buf[0])
	assert.Equal(t, byte(2), buf[1])
	assert.Equal(t, uint32(65001), binary.BigEndian.Uint32(buf[2:6]))
	assert.Equal(t, uint32(65002), binary.BigEndian.Uint32(buf[6:10]))
}

// TestEncodeAttrValueCommunity verifies community encoding.
//
// VALIDATES: "65000:100 65000:200" encodes to two 4-byte values.
// PREVENTS: Community encoding error.
func TestEncodeAttrValueCommunity(t *testing.T) {
	buf, err := encodeAttrValue("community", "65000:100 65000:200")
	require.NoError(t, err)
	require.Len(t, buf, 8) // Two communities, 4 bytes each.
	comm1 := binary.BigEndian.Uint32(buf[0:4])
	comm2 := binary.BigEndian.Uint32(buf[4:8])
	assert.Equal(t, uint32(65000)<<16|uint32(100), comm1)
	assert.Equal(t, uint32(65000)<<16|uint32(200), comm2)
}

// TestDirtyTracking verifies the full round-trip: text modify -> ModAccumulator ->
// buildModifiedPayload -> wire bytes contain modified attribute.
//
// VALIDATES: Wire-level dirty tracking end-to-end.
// PREVENTS: Text delta accepted but wire bytes unchanged.
func TestDirtyTracking(t *testing.T) {
	// Build source UPDATE: ORIGIN=IGP + LOCAL_PREF=100 + NLRI.
	origin := makeAttr(0x40, 1, []byte{0x00}) // ORIGIN=IGP
	lpOld := make([]byte, 4)
	binary.BigEndian.PutUint32(lpOld, 100)
	localPref := makeAttr(0x40, 5, lpOld) // LOCAL_PREF=100
	var attrs []byte
	attrs = append(attrs, origin...)
	attrs = append(attrs, localPref...)
	nlri := []byte{24, 10, 0, 0} // 10.0.0.0/24
	payload := buildModTestPayload(attrs, nlri)

	// Simulate policy filter changing local-pref from 100 to 200.
	originalText := "origin igp local-preference 100"
	modifiedText := "origin igp local-preference 200"

	var mods registry.ModAccumulator
	textDeltaToModOps(originalText, modifiedText, &mods)
	require.Equal(t, 1, mods.Len(), "should have one op for local-pref")

	// Register a generic handler for LOCAL_PREF (code 5).
	handlers := map[uint8]registry.AttrModHandler{
		byte(attribute.AttrLocalPref): genericAttrSetHandler(0x40, byte(attribute.AttrLocalPref)),
	}

	result, _ := buildModifiedPayload(payload, &mods, handlers, nil, nil)
	require.NotNil(t, result, "buildModifiedPayload should produce output")

	// Parse result to find LOCAL_PREF value.
	wdLen := int(binary.BigEndian.Uint16(result[0:2]))
	attrOff := 2 + wdLen + 2
	attrLen := int(binary.BigEndian.Uint16(result[2+wdLen : 2+wdLen+2]))
	attrEnd := attrOff + attrLen

	// Walk attributes to find LOCAL_PREF (code 5).
	found := false
	off := attrOff
	for off < attrEnd {
		code := result[off+1]
		var hdrLen int
		var aLen uint16
		if result[off]&0x10 != 0 {
			aLen = binary.BigEndian.Uint16(result[off+2 : off+4])
			hdrLen = 4
		} else {
			aLen = uint16(result[off+2])
			hdrLen = 3
		}
		if code == byte(attribute.AttrLocalPref) {
			found = true
			valStart := off + hdrLen
			require.Equal(t, 4, int(aLen), "LOCAL_PREF value must be 4 bytes")
			newLP := binary.BigEndian.Uint32(result[valStart : valStart+4])
			assert.Equal(t, uint32(200), newLP, "LOCAL_PREF should be 200 after modification")
		}
		off += hdrLen + int(aLen)
	}
	assert.True(t, found, "LOCAL_PREF attribute should be present in result")

	// Verify NLRI preserved.
	nlriStart := attrOff + attrLen
	assert.Equal(t, nlri, result[nlriStart:], "NLRI must be preserved")
}

// TestFilterModifyOnlyDeclared verifies that undeclared attribute modifications are rejected
// and only declared attributes reach the wire modification path.
//
// VALIDATES: AC-13 -- Filter modifying undeclared attribute is rejected before wire encoding.
// PREVENTS: Plugin modifying attributes it didn't declare, bypassing validation.
func TestFilterModifyOnlyDeclared(t *testing.T) {
	// AC-13 validation happens in policyFilterFunc (filter_chain.go:251-257).
	// This test verifies the validateModifyDelta + textDeltaToModOps interaction:
	// only delta that passes validation reaches textDeltaToModOps.

	// Delta modifying declared attribute: should produce ops.
	t.Run("declared attribute produces ops", func(t *testing.T) {
		original := "origin igp local-preference 100"
		modified := "origin igp local-preference 200"
		declared := []string{"local-preference"}

		violation := validateModifyDelta("local-preference 200", declared)
		assert.Empty(t, violation, "declared attribute should pass validation")

		var mods registry.ModAccumulator
		textDeltaToModOps(original, modified, &mods)
		assert.Equal(t, 1, mods.Len(), "declared attribute change should produce op")
	})

	// Delta modifying undeclared attribute: rejected by validateModifyDelta.
	t.Run("undeclared attribute rejected before wire encoding", func(t *testing.T) {
		declared := []string{"local-preference"}
		violation := validateModifyDelta("community 65000:1", declared)
		assert.Equal(t, "community", violation, "undeclared community should be rejected")
		// textDeltaToModOps would NOT be called in production because
		// policyFilterFunc returns PolicyReject on violation.
	})
}

// TestGenericAttrSetHandler verifies the generic handler writes correct wire bytes.
//
// VALIDATES: Generic handler produces valid attribute (header + value) from AttrModSet op.
// PREVENTS: Handler producing malformed wire bytes.
func TestGenericAttrSetHandler(t *testing.T) {
	handler := genericAttrSetHandler(0x40, byte(attribute.AttrLocalPref))

	// Set op: new value = 300.
	newVal := make([]byte, 4)
	binary.BigEndian.PutUint32(newVal, 300)
	ops := []registry.AttrOp{{
		Code:   byte(attribute.AttrLocalPref),
		Action: registry.AttrModSet,
		Buf:    newVal,
	}}

	buf := make([]byte, 64)

	t.Run("replace existing attribute", func(t *testing.T) {
		// Source: LOCAL_PREF=100.
		oldVal := make([]byte, 4)
		binary.BigEndian.PutUint32(oldVal, 100)
		src := makeAttr(0x40, byte(attribute.AttrLocalPref), oldVal)

		newOff := handler(src, ops, buf, 0)
		require.Equal(t, 7, newOff, "header(3) + value(4)")
		assert.Equal(t, byte(0x40), buf[0], "flags")
		assert.Equal(t, byte(attribute.AttrLocalPref), buf[1], "code")
		assert.Equal(t, byte(4), buf[2], "length")
		assert.Equal(t, uint32(300), binary.BigEndian.Uint32(buf[3:7]))
	})

	t.Run("add new attribute (no source)", func(t *testing.T) {
		newOff := handler(nil, ops, buf, 0)
		require.Equal(t, 7, newOff)
		assert.Equal(t, byte(0x40), buf[0])
		assert.Equal(t, byte(attribute.AttrLocalPref), buf[1])
		assert.Equal(t, uint32(300), binary.BigEndian.Uint32(buf[3:7]))
	})

	t.Run("no set op copies source", func(t *testing.T) {
		oldVal := make([]byte, 4)
		binary.BigEndian.PutUint32(oldVal, 100)
		src := makeAttr(0x40, byte(attribute.AttrLocalPref), oldVal)

		noOps := []registry.AttrOp{{
			Code:   byte(attribute.AttrLocalPref),
			Action: registry.AttrModAdd, // Not Set.
			Buf:    newVal,
		}}
		newOff := handler(src, noOps, buf, 0)
		require.Equal(t, len(src), newOff)
		assert.Equal(t, src, buf[:newOff], "should copy source unchanged")
	})
}

// TestOriginatorIDHandler verifies ORIGINATOR_ID set-if-absent semantics (RFC 4456).
//
// VALIDATES: AC-1 (ORIGINATOR_ID added on reflection), AC-5 (existing ORIGINATOR_ID preserved).
// PREVENTS: Overwriting existing ORIGINATOR_ID or failing to set when absent.
func TestOriginatorIDHandler(t *testing.T) {
	handler := originatorIDHandler()
	buf := make([]byte, 64)

	t.Run("set when absent", func(t *testing.T) {
		ops := []registry.AttrOp{{
			Code:   byte(attribute.AttrOriginatorID),
			Action: registry.AttrModSet,
			Buf:    []byte{10, 0, 0, 1}, // 10.0.0.1
		}}
		off := handler(nil, ops, buf, 0)
		require.Equal(t, 7, off) // flags(1) + code(1) + len(1) + value(4)
		assert.Equal(t, byte(0x80), buf[0], "flags: optional non-transitive")
		assert.Equal(t, byte(attribute.AttrOriginatorID), buf[1])
		assert.Equal(t, byte(4), buf[2])
		assert.Equal(t, []byte{10, 0, 0, 1}, buf[3:7])
	})

	t.Run("preserve existing", func(t *testing.T) {
		// Source has ORIGINATOR_ID = 192.168.1.1
		src := []byte{0x80, byte(attribute.AttrOriginatorID), 4, 192, 168, 1, 1}
		ops := []registry.AttrOp{{
			Code:   byte(attribute.AttrOriginatorID),
			Action: registry.AttrModSet,
			Buf:    []byte{10, 0, 0, 1}, // Would set to 10.0.0.1, but should be ignored
		}}
		off := handler(src, ops, buf, 0)
		require.Equal(t, 7, off)
		assert.Equal(t, src, buf[:7], "existing ORIGINATOR_ID preserved")
	})

	t.Run("no ops copies source", func(t *testing.T) {
		src := []byte{0x80, byte(attribute.AttrOriginatorID), 4, 1, 2, 3, 4}
		off := handler(src, nil, buf, 0)
		require.Equal(t, 7, off)
		assert.Equal(t, src, buf[:7])
	})
}

// TestClusterListHandler verifies CLUSTER_LIST prepend semantics (RFC 4456).
//
// VALIDATES: AC-1 (CLUSTER_LIST prepended on reflection).
// PREVENTS: Cluster-id appended instead of prepended, or existing list lost.
func TestClusterListHandler(t *testing.T) {
	handler := clusterListHandler()
	buf := make([]byte, 128)

	t.Run("prepend to empty", func(t *testing.T) {
		ops := []registry.AttrOp{{
			Code:   byte(attribute.AttrClusterList),
			Action: registry.AttrModPrepend,
			Buf:    []byte{1, 1, 1, 1}, // cluster-id 1.1.1.1
		}}
		off := handler(nil, ops, buf, 0)
		require.Equal(t, 7, off) // flags(1) + code(1) + len(1) + value(4)
		assert.Equal(t, byte(0x80), buf[0])
		assert.Equal(t, byte(attribute.AttrClusterList), buf[1])
		assert.Equal(t, byte(4), buf[2])
		assert.Equal(t, []byte{1, 1, 1, 1}, buf[3:7])
	})

	t.Run("prepend to existing", func(t *testing.T) {
		// Source: CLUSTER_LIST = [2.2.2.2]
		src := []byte{0x80, byte(attribute.AttrClusterList), 4, 2, 2, 2, 2}
		ops := []registry.AttrOp{{
			Code:   byte(attribute.AttrClusterList),
			Action: registry.AttrModPrepend,
			Buf:    []byte{1, 1, 1, 1}, // prepend 1.1.1.1
		}}
		off := handler(src, ops, buf, 0)
		require.Equal(t, 11, off) // flags(1) + code(1) + len(1) + 4 + 4
		assert.Equal(t, byte(0x80), buf[0])
		assert.Equal(t, byte(attribute.AttrClusterList), buf[1])
		assert.Equal(t, byte(8), buf[2])               // 4 + 4 = 8 bytes
		assert.Equal(t, []byte{1, 1, 1, 1}, buf[3:7])  // prepended first
		assert.Equal(t, []byte{2, 2, 2, 2}, buf[7:11]) // existing second
	})

	t.Run("no ops copies source", func(t *testing.T) {
		src := []byte{0x80, byte(attribute.AttrClusterList), 4, 3, 3, 3, 3}
		off := handler(src, nil, buf, 0)
		require.Equal(t, 7, off)
		assert.Equal(t, src, buf[:7])
	})
}

// TestGenericAttrSetHandler_Suppress verifies AttrModSuppress removes the attribute.
//
// VALIDATES: Send-community control (suppress community types from outbound UPDATEs).
// PREVENTS: Suppressed attributes still present in wire output.
func TestGenericAttrSetHandler_Suppress(t *testing.T) {
	handler := genericAttrSetHandler(0xC0, 8) // COMMUNITIES
	buf := make([]byte, 64)

	t.Run("suppress removes attribute", func(t *testing.T) {
		src := []byte{0xC0, 8, 4, 0xFF, 0xFE, 0x00, 0x64} // community 65534:100
		ops := []registry.AttrOp{{
			Code:   8,
			Action: registry.AttrModSuppress,
		}}
		off := handler(src, ops, buf, 0)
		assert.Equal(t, 0, off, "suppress should write nothing")
	})

	t.Run("suppress wins over set", func(t *testing.T) {
		src := []byte{0xC0, 8, 4, 0xFF, 0xFE, 0x00, 0x64}
		ops := []registry.AttrOp{
			{Code: 8, Action: registry.AttrModSet, Buf: []byte{0x00, 0x01, 0x00, 0x02}},
			{Code: 8, Action: registry.AttrModSuppress}, // last wins
		}
		off := handler(src, ops, buf, 0)
		assert.Equal(t, 0, off, "suppress after set should suppress")
	})

	t.Run("set wins over suppress when last", func(t *testing.T) {
		src := []byte{0xC0, 8, 4, 0xFF, 0xFE, 0x00, 0x64}
		ops := []registry.AttrOp{
			{Code: 8, Action: registry.AttrModSuppress},
			{Code: 8, Action: registry.AttrModSet, Buf: []byte{0x00, 0x01, 0x00, 0x02}}, // last wins
		}
		off := handler(src, ops, buf, 0)
		assert.Equal(t, 7, off, "set after suppress should write attribute")
	})
}

// TestApplySendCommunityFilter verifies send-community control logic.
//
// VALIDATES: AC-1 (standard only), AC-2 (none), AC-3 (all default), AC-4 (standard+large).
// PREVENTS: Wrong community types suppressed or kept.
func TestApplySendCommunityFilter(t *testing.T) {
	tests := []struct {
		name           string
		send           []string
		wantSuppress8  bool // standard communities
		wantSuppress16 bool // extended communities
		wantSuppress32 bool // large communities
	}{
		{"nil (default all)", nil, false, false, false},
		{"empty (default all)", []string{}, false, false, false},
		{"all", []string{"all"}, false, false, false},
		{"none", []string{"none"}, true, true, true},
		{"standard only", []string{"standard"}, false, true, true},
		{"large only", []string{"large"}, true, true, false},
		{"standard+large", []string{"standard", "large"}, false, true, false},
		{"standard+extended+large", []string{"standard", "extended", "large"}, false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mods registry.ModAccumulator
			ps := &PeerSettings{SendCommunity: tt.send}
			applySendCommunityFilter(ps, &mods)

			hasSuppressFor := func(code uint8) bool {
				for _, op := range mods.Ops() {
					if op.Code == code && op.Action == registry.AttrModSuppress {
						return true
					}
				}
				return false
			}

			assert.Equal(t, tt.wantSuppress8, hasSuppressFor(8), "standard community suppress")
			assert.Equal(t, tt.wantSuppress16, hasSuppressFor(16), "extended community suppress")
			assert.Equal(t, tt.wantSuppress32, hasSuppressFor(32), "large community suppress")
		})
	}
}

// TestExtractASPathPrependOps verifies AS-path prepend extraction from modified text.
//
// VALIDATES: AC-5 -- as-path-prepend N produces AttrModPrepend op with N copies of localAS.
// PREVENTS: Wrong ASN prepended, wrong count, or no op when expected.
func TestExtractASPathPrependOps(t *testing.T) {
	t.Run("prepend_3", func(t *testing.T) {
		var mods registry.ModAccumulator
		ExtractASPathPrependOps("origin igp as-path-prepend 3 nlri ipv4/unicast add 10.0.0.0/24", 65000, &mods)
		require.Equal(t, 1, mods.Len())
		op := mods.Ops()[0]
		assert.Equal(t, byte(attribute.AttrASPath), op.Code)
		assert.Equal(t, registry.AttrModPrepend, op.Action)
		// Wire: type(1) + count(1) + 3*ASN(4) = 14 bytes
		require.Len(t, op.Buf, 14)
		assert.Equal(t, byte(attribute.ASSequence), op.Buf[0])
		assert.Equal(t, byte(3), op.Buf[1])
		for i := range 3 {
			asn := binary.BigEndian.Uint32(op.Buf[2+i*4:])
			assert.Equal(t, uint32(65000), asn, "ASN at position %d", i)
		}
	})

	t.Run("no_prepend", func(t *testing.T) {
		var mods registry.ModAccumulator
		ExtractASPathPrependOps("origin igp local-preference 200", 65000, &mods)
		assert.Equal(t, 0, mods.Len())
	})

	t.Run("prepend_1", func(t *testing.T) {
		var mods registry.ModAccumulator
		ExtractASPathPrependOps("as-path-prepend 1", 65001, &mods)
		require.Equal(t, 1, mods.Len())
		op := mods.Ops()[0]
		require.Len(t, op.Buf, 6) // type(1) + count(1) + 1*ASN(4)
		assert.Equal(t, uint32(65001), binary.BigEndian.Uint32(op.Buf[2:6]))
	})

	t.Run("invalid_count_zero", func(t *testing.T) {
		var mods registry.ModAccumulator
		ExtractASPathPrependOps("as-path-prepend 0", 65000, &mods)
		assert.Equal(t, 0, mods.Len())
	})

	t.Run("invalid_count_over_32", func(t *testing.T) {
		var mods registry.ModAccumulator
		ExtractASPathPrependOps("as-path-prepend 33", 65000, &mods)
		assert.Equal(t, 0, mods.Len())
	})
}

// TestAspathHandler verifies AS_PATH handler supports both Set and Prepend.
//
// VALIDATES: Prepend inserts new segment before existing AS_PATH.
// PREVENTS: Prepend clobbering existing path or wrong segment format.
func TestAspathHandler(t *testing.T) {
	handler := aspathHandler()
	buf := make([]byte, 128)

	t.Run("prepend_to_existing", func(t *testing.T) {
		// Source: AS_PATH = AS_SEQUENCE [65002]
		srcVal := []byte{byte(attribute.ASSequence), 1, 0, 0, 0xFD, 0xEA} // 65002
		src := makeAttr(0x40, byte(attribute.AttrASPath), srcVal)

		// Prepend: AS_SEQUENCE [65000]
		prependVal := []byte{byte(attribute.ASSequence), 1, 0, 0, 0xFD, 0xE8} // 65000
		ops := []registry.AttrOp{{
			Code:   byte(attribute.AttrASPath),
			Action: registry.AttrModPrepend,
			Buf:    prependVal,
		}}

		off := handler(src, ops, buf, 0)
		// Header(3) + prepend(6) + existing(6) = 15
		require.Equal(t, 15, off)
		assert.Equal(t, byte(0x40), buf[0])
		assert.Equal(t, byte(attribute.AttrASPath), buf[1])
		assert.Equal(t, byte(12), buf[2]) // value length
		// Prepended segment first, then existing.
		assert.Equal(t, prependVal, buf[3:9])
		assert.Equal(t, srcVal, buf[9:15])
	})

	t.Run("prepend_to_empty", func(t *testing.T) {
		prependVal := []byte{byte(attribute.ASSequence), 1, 0, 0, 0xFD, 0xE8}
		ops := []registry.AttrOp{{
			Code:   byte(attribute.AttrASPath),
			Action: registry.AttrModPrepend,
			Buf:    prependVal,
		}}

		off := handler(nil, ops, buf, 0)
		require.Equal(t, 9, off) // Header(3) + prepend(6)
		assert.Equal(t, byte(6), buf[2])
		assert.Equal(t, prependVal, buf[3:9])
	})

	t.Run("set_delegates_to_generic", func(t *testing.T) {
		newVal := []byte{byte(attribute.ASSequence), 1, 0, 0, 0xFD, 0xE9}
		ops := []registry.AttrOp{{
			Code:   byte(attribute.AttrASPath),
			Action: registry.AttrModSet,
			Buf:    newVal,
		}}

		off := handler(nil, ops, buf, 0)
		require.Equal(t, 9, off)
		assert.Equal(t, newVal, buf[3:9])
	})
}

// TestRewriteASPathOverride verifies AS-override replaces peer ASN with local ASN.
//
// VALIDATES: AC-12 (as-override replaces peer ASN in AS_PATH).
// PREVENTS: Wrong ASN replaced, or no replacement when needed.
func TestRewriteASPathOverride(t *testing.T) {
	t.Run("replaces peer ASN", func(t *testing.T) {
		// AS_SEQUENCE: type=2, len=3, ASNs: 65001, 65002, 65001
		data := []byte{
			2, 3, // type=AS_SEQUENCE, length=3
			0, 0, 0xFD, 0xE9, // 65001
			0, 0, 0xFD, 0xEA, // 65002
			0, 0, 0xFD, 0xE9, // 65001
		}
		result := rewriteASPathOverride(data, 65001, 65000, true)
		require.NotNil(t, result)
		// Both 65001 occurrences replaced with 65000.
		assert.Equal(t, byte(0xFD), result[4])
		assert.Equal(t, byte(0xE8), result[5]) // 65000
		assert.Equal(t, byte(0xFD), result[8])
		assert.Equal(t, byte(0xEA), result[9]) // 65002 unchanged
		assert.Equal(t, byte(0xFD), result[12])
		assert.Equal(t, byte(0xE8), result[13]) // 65000
	})

	t.Run("no match returns nil", func(t *testing.T) {
		data := []byte{
			2, 1,
			0, 0, 0xFD, 0xEA, // 65002 only
		}
		result := rewriteASPathOverride(data, 65001, 65000, true)
		assert.Nil(t, result, "no match should return nil")
	})

	t.Run("empty data", func(t *testing.T) {
		result := rewriteASPathOverride(nil, 65001, 65000, true)
		assert.Nil(t, result)
	})
}
