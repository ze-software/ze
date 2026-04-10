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

	result, _ := buildModifiedPayload(payload, &mods, handlers, nil)
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
