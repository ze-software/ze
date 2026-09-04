// Design: docs/architecture/core-design.md -- policy filter wire-level dirty tracking tests
package reactor

import (
	"encoding/binary"
	"fmt"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestExtractLegacyNLRIOverride covers the per-prefix modify path helper: the
// engine, not the filter plugin, re-encodes the accepted prefix subset, so the
// plugin side of a per-prefix deny stays pure text. The helper compares the "nlri
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
			var mods filterapi.ModAccumulator
			textDeltaToModOps(parseFilterAttrs(tt.original), parseFilterAttrs(tt.modified), &mods)
			assert.Equal(t, tt.wantOps, mods.Len())
			if tt.wantOps == 1 {
				ops := mods.Ops()
				assert.Equal(t, byte(tt.wantCode), ops[0].Code)
				assert.Equal(t, filterapi.AttrModSet, ops[0].Action)
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

	var mods filterapi.ModAccumulator
	textDeltaToModOps(parseFilterAttrs(originalText), parseFilterAttrs(modifiedText), &mods)
	require.Equal(t, 1, mods.Len(), "should have one op for local-pref")

	// Register a generic handler for LOCAL_PREF (code 5).
	handlers := map[uint8]filterapi.AttrModHandler{
		byte(attribute.AttrLocalPref): genericAttrSetHandler(0x40, byte(attribute.AttrLocalPref)),
	}

	result, _, _ := buildModifiedPayload(payload, &mods, handlers, nil, nil)
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

		var mods filterapi.ModAccumulator
		textDeltaToModOps(parseFilterAttrs(original), parseFilterAttrs(modified), &mods)
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
	code := byte(attribute.AttrLocalPref)

	// Set op: new value = 300.
	newVal := make([]byte, 4)
	binary.BigEndian.PutUint32(newVal, 300)
	ops := []filterapi.AttrOp{{
		Code:   byte(attribute.AttrLocalPref),
		Action: filterapi.AttrModSet,
		Buf:    newVal,
	}}

	t.Run("replace existing attribute", func(t *testing.T) {
		// Source: LOCAL_PREF=100.
		oldVal := make([]byte, 4)
		binary.BigEndian.PutUint32(oldVal, 100)
		src := makeAttr(0x40, byte(attribute.AttrLocalPref), oldVal)

		out, ok := planHandlerBytes(handler, code, src, ops)
		require.True(t, ok, "handler planned an emitted attribute")
		require.Len(t, out, 7, "header(3) + value(4)")
		assert.Equal(t, byte(0x40), out[0], "flags")
		assert.Equal(t, byte(attribute.AttrLocalPref), out[1], "code")
		assert.Equal(t, byte(4), out[2], "length")
		assert.Equal(t, uint32(300), binary.BigEndian.Uint32(out[3:7]))
	})

	t.Run("add new attribute (no source)", func(t *testing.T) {
		out, ok := planHandlerBytes(handler, code, nil, ops)
		require.True(t, ok, "handler planned an emitted attribute")
		require.Len(t, out, 7)
		assert.Equal(t, byte(0x40), out[0])
		assert.Equal(t, byte(attribute.AttrLocalPref), out[1])
		assert.Equal(t, uint32(300), binary.BigEndian.Uint32(out[3:7]))
	})

	t.Run("no set op copies source", func(t *testing.T) {
		oldVal := make([]byte, 4)
		binary.BigEndian.PutUint32(oldVal, 100)
		src := makeAttr(0x40, byte(attribute.AttrLocalPref), oldVal)

		noOps := []filterapi.AttrOp{{
			Code:   byte(attribute.AttrLocalPref),
			Action: filterapi.AttrModAdd, // Not Set.
			Buf:    newVal,
		}}
		out, ok := planHandlerBytes(handler, code, src, noOps)
		require.True(t, ok, "handler planned an emitted attribute")
		require.Len(t, out, len(src))
		assert.Equal(t, src, out, "should copy source unchanged")
	})
}

// TestOriginatorIDHandler verifies ORIGINATOR_ID set-if-absent semantics (RFC 4456).
//
// VALIDATES: AC-1 (ORIGINATOR_ID added on reflection), AC-5 (existing ORIGINATOR_ID preserved).
// PREVENTS: Overwriting existing ORIGINATOR_ID or failing to set when absent.
func TestOriginatorIDHandler(t *testing.T) {
	handler := originatorIDHandler()
	code := byte(attribute.AttrOriginatorID)

	t.Run("set when absent", func(t *testing.T) {
		ops := []filterapi.AttrOp{{
			Code:   byte(attribute.AttrOriginatorID),
			Action: filterapi.AttrModSet,
			Buf:    []byte{10, 0, 0, 1}, // 10.0.0.1
		}}
		out, ok := planHandlerBytes(handler, code, nil, ops)
		require.True(t, ok, "handler planned an emitted attribute")
		require.Len(t, out, 7) // flags(1) + code(1) + len(1) + value(4)
		assert.Equal(t, byte(0x80), out[0], "flags: optional non-transitive")
		assert.Equal(t, byte(attribute.AttrOriginatorID), out[1])
		assert.Equal(t, byte(4), out[2])
		assert.Equal(t, []byte{10, 0, 0, 1}, out[3:7])
	})

	t.Run("preserve existing", func(t *testing.T) {
		// Source has ORIGINATOR_ID = 192.168.1.1
		src := []byte{0x80, byte(attribute.AttrOriginatorID), 4, 192, 168, 1, 1}
		ops := []filterapi.AttrOp{{
			Code:   byte(attribute.AttrOriginatorID),
			Action: filterapi.AttrModSet,
			Buf:    []byte{10, 0, 0, 1}, // Would set to 10.0.0.1, but should be ignored
		}}
		out, ok := planHandlerBytes(handler, code, src, ops)
		require.True(t, ok, "handler planned an emitted attribute")
		require.Len(t, out, 7)
		assert.Equal(t, src, out, "existing ORIGINATOR_ID preserved")
	})

	t.Run("no ops copies source", func(t *testing.T) {
		src := []byte{0x80, byte(attribute.AttrOriginatorID), 4, 1, 2, 3, 4}
		out, ok := planHandlerBytes(handler, code, src, nil)
		require.True(t, ok, "handler planned an emitted attribute")
		require.Len(t, out, 7)
		assert.Equal(t, src, out)
	})
}

// TestClusterListHandler verifies CLUSTER_LIST prepend semantics (RFC 4456).
//
// VALIDATES: AC-1 (CLUSTER_LIST prepended on reflection).
// PREVENTS: Cluster-id appended instead of prepended, or existing list lost.
func TestClusterListHandler(t *testing.T) {
	handler := clusterListHandler()
	code := byte(attribute.AttrClusterList)

	t.Run("prepend to empty", func(t *testing.T) {
		ops := []filterapi.AttrOp{{
			Code:   byte(attribute.AttrClusterList),
			Action: filterapi.AttrModPrepend,
			Buf:    []byte{1, 1, 1, 1}, // cluster-id 1.1.1.1
		}}
		out, ok := planHandlerBytes(handler, code, nil, ops)
		require.True(t, ok, "handler planned an emitted attribute")
		require.Len(t, out, 7) // flags(1) + code(1) + len(1) + value(4)
		assert.Equal(t, byte(0x80), out[0])
		assert.Equal(t, byte(attribute.AttrClusterList), out[1])
		assert.Equal(t, byte(4), out[2])
		assert.Equal(t, []byte{1, 1, 1, 1}, out[3:7])
	})

	t.Run("prepend to existing", func(t *testing.T) {
		// Source: CLUSTER_LIST = [2.2.2.2]
		src := []byte{0x80, byte(attribute.AttrClusterList), 4, 2, 2, 2, 2}
		ops := []filterapi.AttrOp{{
			Code:   byte(attribute.AttrClusterList),
			Action: filterapi.AttrModPrepend,
			Buf:    []byte{1, 1, 1, 1}, // prepend 1.1.1.1
		}}
		out, ok := planHandlerBytes(handler, code, src, ops)
		require.True(t, ok, "handler planned an emitted attribute")
		require.Len(t, out, 11) // flags(1) + code(1) + len(1) + 4 + 4
		assert.Equal(t, byte(0x80), out[0])
		assert.Equal(t, byte(attribute.AttrClusterList), out[1])
		assert.Equal(t, byte(8), out[2])               // 4 + 4 = 8 bytes
		assert.Equal(t, []byte{1, 1, 1, 1}, out[3:7])  // prepended first
		assert.Equal(t, []byte{2, 2, 2, 2}, out[7:11]) // existing second
	})

	t.Run("no ops copies source", func(t *testing.T) {
		src := []byte{0x80, byte(attribute.AttrClusterList), 4, 3, 3, 3, 3}
		out, ok := planHandlerBytes(handler, code, src, nil)
		require.True(t, ok, "handler planned an emitted attribute")
		require.Len(t, out, 7)
		assert.Equal(t, src, out)
	})
}

// TestGenericAttrSetHandler_Suppress verifies AttrModSuppress removes the attribute.
//
// VALIDATES: Send-community control (suppress community types from outbound UPDATEs).
// PREVENTS: Suppressed attributes still present in wire output.
func TestGenericAttrSetHandler_Suppress(t *testing.T) {
	handler := genericAttrSetHandler(0xC0, 8) // COMMUNITIES

	// A suppressed attribute is a DROP, never a refusal: the attribute leaves the
	// UPDATE while the route is still forwarded. A refusal would suppress the whole
	// route, so the outcome kind is asserted here rather than only "no bytes".
	t.Run("suppress removes attribute", func(t *testing.T) {
		src := []byte{0xC0, 8, 4, 0xFF, 0xFE, 0x00, 0x64} // community 65534:100
		ops := []filterapi.AttrOp{{
			Code:   8,
			Action: filterapi.AttrModSuppress,
		}}
		res := planHandler(handler, 8, src, ops)
		assert.Empty(t, res.out, "suppress should write nothing")
		assert.True(t, res.dropped, "suppress drops the attribute, it does not refuse the route")
	})

	t.Run("suppress wins over set", func(t *testing.T) {
		src := []byte{0xC0, 8, 4, 0xFF, 0xFE, 0x00, 0x64}
		ops := []filterapi.AttrOp{
			{Code: 8, Action: filterapi.AttrModSet, Buf: []byte{0x00, 0x01, 0x00, 0x02}},
			{Code: 8, Action: filterapi.AttrModSuppress}, // last wins
		}
		res := planHandler(handler, 8, src, ops)
		assert.Empty(t, res.out, "suppress after set should suppress")
		assert.True(t, res.dropped, "suppress drops the attribute, it does not refuse the route")
	})

	t.Run("set wins over suppress when last", func(t *testing.T) {
		src := []byte{0xC0, 8, 4, 0xFF, 0xFE, 0x00, 0x64}
		ops := []filterapi.AttrOp{
			{Code: 8, Action: filterapi.AttrModSuppress},
			{Code: 8, Action: filterapi.AttrModSet, Buf: []byte{0x00, 0x01, 0x00, 0x02}}, // last wins
		}
		out, ok := planHandlerBytes(handler, 8, src, ops)
		require.True(t, ok, "handler planned an emitted attribute")
		assert.Len(t, out, 7, "set after suppress should write attribute")
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
			var mods filterapi.ModAccumulator
			ps := &PeerSettings{SendCommunity: tt.send}
			applySendCommunityFilter(ps, &mods)

			hasSuppressFor := func(code uint8) bool {
				for _, op := range mods.Ops() {
					if op.Code == code && op.Action == filterapi.AttrModSuppress {
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
		var mods filterapi.ModAccumulator
		ExtractASPathPrependOps(parseFilterAttrs("origin igp as-path-prepend 3 nlri ipv4/unicast add 10.0.0.0/24"), 65000, &mods)
		require.Equal(t, 1, mods.Len())
		op := mods.Ops()[0]
		assert.Equal(t, byte(attribute.AttrASPath), op.Code)
		assert.Equal(t, filterapi.AttrModPrepend, op.Action)
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
		var mods filterapi.ModAccumulator
		ExtractASPathPrependOps(parseFilterAttrs("origin igp local-preference 200"), 65000, &mods)
		assert.Equal(t, 0, mods.Len())
	})

	t.Run("prepend_1", func(t *testing.T) {
		var mods filterapi.ModAccumulator
		ExtractASPathPrependOps(parseFilterAttrs("as-path-prepend 1"), 65001, &mods)
		require.Equal(t, 1, mods.Len())
		op := mods.Ops()[0]
		require.Len(t, op.Buf, 6) // type(1) + count(1) + 1*ASN(4)
		assert.Equal(t, uint32(65001), binary.BigEndian.Uint32(op.Buf[2:6]))
	})

	t.Run("invalid_count_zero", func(t *testing.T) {
		var mods filterapi.ModAccumulator
		ExtractASPathPrependOps(parseFilterAttrs("as-path-prepend 0"), 65000, &mods)
		assert.Equal(t, 0, mods.Len())
	})

	t.Run("invalid_count_over_32", func(t *testing.T) {
		var mods filterapi.ModAccumulator
		ExtractASPathPrependOps(parseFilterAttrs("as-path-prepend 33"), 65000, &mods)
		assert.Equal(t, 0, mods.Len())
	})
}

// TestAspathHandler verifies AS_PATH handler supports both Set and Prepend.
//
// VALIDATES: Prepend inserts new segment before existing AS_PATH.
// PREVENTS: Prepend clobbering existing path or wrong segment format.
func TestAspathHandler(t *testing.T) {
	handler := aspathHandler()
	code := byte(attribute.AttrASPath)

	t.Run("prepend_to_existing", func(t *testing.T) {
		// Source: AS_PATH = AS_SEQUENCE [65002]
		srcVal := []byte{byte(attribute.ASSequence), 1, 0, 0, 0xFD, 0xEA} // 65002
		src := makeAttr(0x40, byte(attribute.AttrASPath), srcVal)

		// Prepend: AS_SEQUENCE [65000]
		prependVal := []byte{byte(attribute.ASSequence), 1, 0, 0, 0xFD, 0xE8} // 65000
		ops := []filterapi.AttrOp{{
			Code:   byte(attribute.AttrASPath),
			Action: filterapi.AttrModPrepend,
			Buf:    prependVal,
		}}

		out, ok := planHandlerBytes(handler, code, src, ops)
		require.True(t, ok, "handler planned an emitted attribute")
		// Header(3) + prepend(6) + existing(6) = 15
		require.Len(t, out, 15)
		assert.Equal(t, byte(0x40), out[0])
		assert.Equal(t, byte(attribute.AttrASPath), out[1])
		assert.Equal(t, byte(12), out[2]) // value length
		// Prepended segment first, then existing.
		assert.Equal(t, prependVal, out[3:9])
		assert.Equal(t, srcVal, out[9:15])
	})

	t.Run("prepend_to_empty", func(t *testing.T) {
		prependVal := []byte{byte(attribute.ASSequence), 1, 0, 0, 0xFD, 0xE8}
		ops := []filterapi.AttrOp{{
			Code:   byte(attribute.AttrASPath),
			Action: filterapi.AttrModPrepend,
			Buf:    prependVal,
		}}

		out, ok := planHandlerBytes(handler, code, nil, ops)
		require.True(t, ok, "handler planned an emitted attribute")
		require.Len(t, out, 9) // Header(3) + prepend(6)
		assert.Equal(t, byte(6), out[2])
		assert.Equal(t, prependVal, out[3:9])
	})

	t.Run("set_delegates_to_generic", func(t *testing.T) {
		newVal := []byte{byte(attribute.ASSequence), 1, 0, 0, 0xFD, 0xE9}
		ops := []filterapi.AttrOp{{
			Code:   byte(attribute.AttrASPath),
			Action: filterapi.AttrModSet,
			Buf:    newVal,
		}}

		out, ok := planHandlerBytes(handler, code, nil, ops)
		require.True(t, ok, "handler planned an emitted attribute")
		require.Len(t, out, 9)
		assert.Equal(t, newVal, out[3:9])
	})

	t.Run("set_and_prepend", func(t *testing.T) {
		setVal := []byte{byte(attribute.ASSequence), 1, 0, 0, 0xFD, 0xEA}     // 65002
		prependVal := []byte{byte(attribute.ASSequence), 1, 0, 0, 0xFD, 0xE8} // 65000
		ops := []filterapi.AttrOp{
			{Code: byte(attribute.AttrASPath), Action: filterapi.AttrModSet, Buf: setVal},
			{Code: byte(attribute.AttrASPath), Action: filterapi.AttrModPrepend, Buf: prependVal},
		}

		out, ok := planHandlerBytes(handler, code, nil, ops)
		require.True(t, ok, "handler planned an emitted attribute")
		require.Len(t, out, 15)
		assert.Equal(t, byte(12), out[2])
		assert.Equal(t, prependVal, out[3:9])
		assert.Equal(t, setVal, out[9:15])
	})
}

// TestExtractRemovePrivateASOps verifies the policy directive emits segment-preserving AS_PATH ops.
//
// VALIDATES: AC-5/AC-12 -- private ASNs are stripped while segment types are preserved.
// PREVENTS: text-level AS_PATH flattening from clobbering AS_SET or AS_SEQUENCE structure.
func TestExtractRemovePrivateASOps(t *testing.T) {
	// AS_SEQUENCE [64496 64512], AS_SET [64497 65534]
	asPathVal := []byte{
		byte(attribute.ASSequence), 2,
		0, 0, 0xFB, 0xF0,
		0, 0, 0xFC, 0x00,
		byte(attribute.ASSet), 2,
		0, 0, 0xFB, 0xF1,
		0, 0, 0xFF, 0xFE,
	}
	attrs := makeAttr(0x40, byte(attribute.AttrASPath), asPathVal)
	attrsWire := attribute.NewAttributesWire(attrs, 0)
	var mods filterapi.ModAccumulator

	ExtractRemovePrivateASOps(parseFilterAttrs("remove-private strip"), attrsWire, true, 65001, &mods)
	require.Equal(t, 1, mods.Len())
	op := mods.Ops()[0]
	require.Equal(t, byte(attribute.AttrASPath), op.Code)
	require.Equal(t, filterapi.AttrModSet, op.Action)
	want := []byte{
		byte(attribute.ASSequence), 1,
		0, 0, 0xFB, 0xF0,
		byte(attribute.ASSet), 1,
		0, 0, 0xFB, 0xF1,
	}
	assert.Equal(t, want, op.Buf)
}

// VALIDATES: AC-11 -- replace-with peer-as replaces Private Use ASNs with the peer ASN.
// PREVENTS: replace mode accidentally stripping ASNs or using local AS.
func TestExtractRemovePrivateASOpsReplacePeerAS(t *testing.T) {
	asPathVal := []byte{
		byte(attribute.ASSequence), 2,
		0, 0, 0xFB, 0xF0,
		0, 0, 0xFC, 0x00,
	}
	attrs := makeAttr(0x40, byte(attribute.AttrASPath), asPathVal)
	attrsWire := attribute.NewAttributesWire(attrs, 0)
	var mods filterapi.ModAccumulator

	ExtractRemovePrivateASOps(parseFilterAttrs("remove-private peer-as"), attrsWire, true, 65001, &mods)
	require.Equal(t, 1, mods.Len())
	op := mods.Ops()[0]
	want := []byte{
		byte(attribute.ASSequence), 2,
		0, 0, 0xFB, 0xF0,
		0, 0, 0xFD, 0xE9,
	}
	assert.Equal(t, want, op.Buf)
}

// VALIDATES: AC-15/AC-16 -- AS4_PATH is rewritten and suppressed when empty.
// PREVENTS: RFC 6996 AS4_PATH removal requirement being skipped.
func TestExtractRemovePrivateASOpsAS4Path(t *testing.T) {
	as4Val := []byte{
		byte(attribute.ASSequence), 1,
		0xFA, 0x56, 0xEA, 0x00, // 4200000000
	}
	attrs := makeAttr(0xC0, byte(attribute.AttrAS4Path), as4Val)
	attrsWire := attribute.NewAttributesWire(attrs, 0)
	var mods filterapi.ModAccumulator

	ExtractRemovePrivateASOps(parseFilterAttrs("remove-private strip"), attrsWire, true, 65001, &mods)
	require.Equal(t, 1, mods.Len())
	op := mods.Ops()[0]
	assert.Equal(t, byte(attribute.AttrAS4Path), op.Code)
	assert.Equal(t, filterapi.AttrModSuppress, op.Action)
}

// VALIDATES: AC-14 -- AS_PATH stays present with an empty value when every ASN is stripped.
// PREVENTS: treating mandatory AS_PATH like optional AS4_PATH suppression.
func TestExtractRemovePrivateASOpsEmptyASPath(t *testing.T) {
	asPathVal := []byte{
		byte(attribute.ASSequence), 2,
		0, 0, 0xFC, 0x00,
		0, 0, 0xFF, 0xFE,
	}
	attrs := makeAttr(0x40, byte(attribute.AttrASPath), asPathVal)
	attrsWire := attribute.NewAttributesWire(attrs, 0)
	var mods filterapi.ModAccumulator

	ExtractRemovePrivateASOps(parseFilterAttrs("remove-private strip"), attrsWire, true, 65001, &mods)
	require.Equal(t, 1, mods.Len())
	op := mods.Ops()[0]
	assert.Equal(t, byte(attribute.AttrASPath), op.Code)
	assert.Equal(t, filterapi.AttrModSet, op.Action)
	assert.Empty(t, op.Buf)
}

// VALIDATES: AC-19 -- no private ASN means no modification op is emitted.
// PREVENTS: allocating a modified payload on the no-op fast path.
func TestExtractRemovePrivateASOpsNoPrivateASN(t *testing.T) {
	asPathVal := []byte{
		byte(attribute.ASSequence), 2,
		0, 0, 0xFB, 0xF0,
		0, 0, 0xFB, 0xF1,
	}
	attrs := makeAttr(0x40, byte(attribute.AttrASPath), asPathVal)
	attrsWire := attribute.NewAttributesWire(attrs, 0)
	var mods filterapi.ModAccumulator

	ExtractRemovePrivateASOps(parseFilterAttrs("remove-private strip"), attrsWire, true, 65001, &mods)
	assert.Equal(t, 0, mods.Len())
}

// VALIDATES: AC-21 -- malformed AS_PATH is not rewritten into guessed bytes.
// PREVENTS: policy rewrite corrupting a malformed route attribute.
func TestExtractRemovePrivateASOpsMalformedASPath(t *testing.T) {
	asPathVal := []byte{byte(attribute.ASSequence), 1, 0, 0}
	attrs := makeAttr(0x40, byte(attribute.AttrASPath), asPathVal)
	attrsWire := attribute.NewAttributesWire(attrs, 0)
	var mods filterapi.ModAccumulator

	ExtractRemovePrivateASOps(parseFilterAttrs("remove-private strip"), attrsWire, true, 65001, &mods)
	assert.Equal(t, 0, mods.Len())
}

// VALIDATES: AC-17 -- export remove-private-AS rewrite feeds EBGP prepend.
// PREVENTS: EBGP rewrite using the original cached wire and reintroducing private ASNs.
func TestExportRemovePrivateASBeforeEBGPPrepend(t *testing.T) {
	origin := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0x00})
	asPathVal := []byte{
		byte(attribute.ASSequence), 3,
		0, 0, 0xFB, 0xF0, // 64496
		0, 0, 0xFC, 0x00, // 64512
		0, 0, 0xFB, 0xF1, // 64497
	}
	asPath := makeAttr(0x40, byte(attribute.AttrASPath), asPathVal)
	nextHop := makeAttr(0x40, byte(attribute.AttrNextHop), []byte{1, 1, 1, 1})
	attrs := append(append(append([]byte{}, origin...), asPath...), nextHop...)
	payload := buildModTestPayload(attrs, []byte{24, 10, 0, 0})
	attrsWire := attribute.NewAttributesWire(attrs, 0)

	var mods filterapi.ModAccumulator
	ExtractRemovePrivateASOps(parseFilterAttrs("remove-private strip"), attrsWire, true, 65002, &mods)
	modified, _, _ := buildModifiedPayload(payload, &mods, attrModHandlersWithDefaults(), nil, nil)
	require.NotNil(t, modified)

	dst := make([]byte, len(modified)+64)
	n, err := wireu.RewriteASPath(dst, modified, 65000, true, true)
	require.NoError(t, err)
	finalPath, err := attribute.ParseASPath(payloadASPathValue(t, dst[:n]), true)
	require.NoError(t, err)
	require.Len(t, finalPath.Segments, 1)
	assert.Equal(t, []uint32{65000, 64496, 64497}, finalPath.Segments[0].ASNs)
}

func payloadASPathValue(t testing.TB, payload []byte) []byte {
	t.Helper()
	require.GreaterOrEqual(t, len(payload), 4)
	wdLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrLenOff := 2 + wdLen
	require.GreaterOrEqual(t, len(payload), attrLenOff+2)
	attrLen := int(binary.BigEndian.Uint16(payload[attrLenOff : attrLenOff+2]))
	off := attrLenOff + 2
	end := off + attrLen
	require.GreaterOrEqual(t, len(payload), end)
	for off < end {
		require.GreaterOrEqual(t, end, off+3)
		code := payload[off+1]
		hdrLen := 3
		valueLen := int(payload[off+2])
		if payload[off]&0x10 != 0 {
			require.GreaterOrEqual(t, end, off+4)
			hdrLen = 4
			valueLen = int(binary.BigEndian.Uint16(payload[off+2 : off+4]))
		}
		valueStart := off + hdrLen
		valueEnd := valueStart + valueLen
		require.GreaterOrEqual(t, end, valueEnd)
		if code == byte(attribute.AttrASPath) {
			return payload[valueStart:valueEnd]
		}
		off = valueEnd
	}
	t.Fatalf("AS_PATH not found")
	return nil
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

// VALIDATES: AC-7 -- community-add directive emits AttrModAdd.
func TestCommunityAddDeltaToModOps(t *testing.T) {
	original := "origin igp community 65000:100 as-path 65001"
	modified := "origin igp community 65000:100 community-add 65000:200 as-path 65001"

	var mods filterapi.ModAccumulator
	textDeltaToModOps(parseFilterAttrs(original), parseFilterAttrs(modified), &mods)

	ops := mods.Ops()
	found := false
	for _, op := range ops {
		if op.Code == byte(attribute.AttrCommunity) && op.Action == filterapi.AttrModAdd {
			found = true
			if len(op.Buf) != 4 {
				t.Errorf("expected 4-byte community value, got %d", len(op.Buf))
			}
		}
	}
	if !found {
		t.Error("expected AttrModAdd op for COMMUNITY, not found")
	}
}

// VALIDATES: AC-8 -- community-remove directive emits AttrModRemove.
func TestCommunityRemoveDeltaToModOps(t *testing.T) {
	original := "origin igp community 65000:100 65000:200 as-path 65001"
	modified := "origin igp community 65000:100 65000:200 community-remove 65000:100 as-path 65001"

	var mods filterapi.ModAccumulator
	textDeltaToModOps(parseFilterAttrs(original), parseFilterAttrs(modified), &mods)

	ops := mods.Ops()
	found := false
	for _, op := range ops {
		if op.Code == byte(attribute.AttrCommunity) && op.Action == filterapi.AttrModRemove {
			found = true
			if len(op.Buf) != 4 {
				t.Errorf("expected 4-byte community value, got %d", len(op.Buf))
			}
		}
	}
	if !found {
		t.Error("expected AttrModRemove op for COMMUNITY, not found")
	}
}

// VALIDATES: AC-7 via large-community -- large-community-add emits AttrModAdd.
func TestLargeCommunityAddDeltaToModOps(t *testing.T) {
	original := "origin igp as-path 65001"
	modified := "origin igp large-community-add 65000:100:200 as-path 65001"

	var mods filterapi.ModAccumulator
	textDeltaToModOps(parseFilterAttrs(original), parseFilterAttrs(modified), &mods)

	ops := mods.Ops()
	found := false
	for _, op := range ops {
		if op.Code == byte(attribute.AttrLargeCommunity) && op.Action == filterapi.AttrModAdd {
			found = true
			if len(op.Buf) != 12 {
				t.Errorf("expected 12-byte large community value, got %d", len(op.Buf))
			}
		}
	}
	if !found {
		t.Error("expected AttrModAdd op for LARGE_COMMUNITY, not found")
	}
}

// VALIDATES: community-remove with multiple values emits one op per value.
// PREVENTS: multi-value remove silently doing nothing (removeValues rejects mismatched sizes).
func TestCommunityRemoveMultiValue(t *testing.T) {
	original := "origin igp community 65000:100 65000:200 65000:300 as-path 65001"
	modified := "origin igp community 65000:100 65000:200 65000:300 community-remove 65000:100 65000:200 as-path 65001"

	var mods filterapi.ModAccumulator
	textDeltaToModOps(parseFilterAttrs(original), parseFilterAttrs(modified), &mods)

	removeCount := 0
	for _, op := range mods.Ops() {
		if op.Code == byte(attribute.AttrCommunity) && op.Action == filterapi.AttrModRemove {
			if len(op.Buf) != 4 {
				t.Errorf("remove op should be exactly 4 bytes, got %d", len(op.Buf))
			}
			removeCount++
		}
	}
	if removeCount != 2 {
		t.Errorf("expected 2 AttrModRemove ops (one per value), got %d", removeCount)
	}
}

// VALIDATES: community directives don't interfere with regular community Set path.
func TestCommunityDirectiveNoSetInterference(t *testing.T) {
	original := "origin igp community 65000:100 as-path 65001"
	modified := "origin igp community 65000:100 community-add 65000:200 as-path 65001"

	var mods filterapi.ModAccumulator
	textDeltaToModOps(parseFilterAttrs(original), parseFilterAttrs(modified), &mods)

	for _, op := range mods.Ops() {
		if op.Code == byte(attribute.AttrCommunity) && op.Action == filterapi.AttrModSet {
			t.Error("unexpected AttrModSet for COMMUNITY (should only have AttrModAdd)")
		}
	}
}

// runFilterDeltaExtractors mirrors the three-call sequence shared by the
// egress (reactor_api_forward.go), ingress (reactor_notify.go), and dry-run
// (policy_dryrun.go) call sites. TestFilterDeltaParseOnceEquivalence pins the
// ops this sequence produces so the parse-once refactor can prove its output
// unchanged (spec filter-delta-parse-once AC-1).
func runFilterDeltaExtractors(original, modified string, attrsWire *attribute.AttributesWire, asn4 bool, peerAS, localAS uint32) []filterapi.AttrOp {
	var mods filterapi.ModAccumulator
	origAttrs := parseFilterAttrs(original)
	modAttrs := parseFilterAttrs(modified)
	textDeltaToModOps(origAttrs, modAttrs, &mods)
	ExtractRemovePrivateASOps(modAttrs, attrsWire, asn4, peerAS, &mods)
	ExtractASPathPrependOps(modAttrs, localAS, &mods)
	return mods.Ops()
}

// goldenOps renders ops as "code action hex(buf)" strings, sorted. Ops from
// textDeltaToModOps follow Go map iteration order, which was never
// deterministic, so equivalence is a sorted-multiset comparison.
func goldenOps(ops []filterapi.AttrOp) []string {
	out := make([]string, 0, len(ops))
	for _, op := range ops {
		out = append(out, fmt.Sprintf("%d %d %x", op.Code, op.Action, op.Buf))
	}
	slices.Sort(out)
	return out
}

// filterDeltaPrivateASFixture returns an AttributesWire whose AS_PATH mixes
// private and public ASNs: AS_SEQUENCE [64496 64512], AS_SET [64497 65534].
func filterDeltaPrivateASFixture() *attribute.AttributesWire {
	asPathVal := []byte{
		byte(attribute.ASSequence), 2,
		0, 0, 0xFB, 0xF0, // 64496 (public)
		0, 0, 0xFC, 0x00, // 64512 (private)
		byte(attribute.ASSet), 2,
		0, 0, 0xFB, 0xF1, // 64497 (public)
		0, 0, 0xFF, 0xFE, // 65534 (private)
	}
	return attribute.NewAttributesWire(makeAttr(0x40, byte(attribute.AttrASPath), asPathVal), 0)
}

// filterDeltaAS4PathFixture returns an AttributesWire with a public-only
// AS_PATH plus an AS4_PATH holding only private ASNs, so remove-private
// strip suppresses AS4_PATH entirely (RFC 6793 + RFC 6996).
func filterDeltaAS4PathFixture() *attribute.AttributesWire {
	asPath := makeAttr(0x40, byte(attribute.AttrASPath), []byte{
		byte(attribute.ASSequence), 1,
		0, 0, 0xFB, 0xF0, // 64496 (public)
	})
	as4Path := makeAttr(0xC0, byte(attribute.AttrAS4Path), []byte{
		byte(attribute.ASSequence), 2,
		0, 0, 0xFC, 0x00, // 64512 (private)
		0, 0, 0xFF, 0xFE, // 65534 (private)
	})
	packed := make([]byte, 0, len(asPath)+len(as4Path))
	packed = append(packed, asPath...)
	packed = append(packed, as4Path...)
	return attribute.NewAttributesWire(packed, 0)
}

// TestFilterDeltaParseOnceEquivalence is the golden gate for the parse-once
// refactor: the corpus covers set, add, remove, nlri-only change, prepend,
// remove-private (strip, peer-as, AS4_PATH suppress), community directives,
// and the combined call-site case. The golden strings were captured from the
// pre-refactor implementation; the refactored path must reproduce them.
//
// VALIDATES: AC-1 -- ModAccumulator ops are identical before/after refactor.
// PREVENTS: the parse-once refactor silently changing wire-level filter ops.
func TestFilterDeltaParseOnceEquivalence(t *testing.T) {
	attrsPrivate := filterDeltaPrivateASFixture()
	attrsAS4 := filterDeltaAS4PathFixture()

	tests := []struct {
		name     string
		original string
		modified string
		attrs    *attribute.AttributesWire
		asn4     bool
		peerAS   uint32
		localAS  uint32
		want     []string
	}{
		{
			name:     "no-op identical text",
			original: "origin igp local-preference 100",
			modified: "origin igp local-preference 100",
			want:     []string{},
		},
		{
			name:     "set origin",
			original: "origin igp",
			modified: "origin egp",
			want:     []string{"1 0 01"},
		},
		{
			name:     "add med",
			original: "origin igp",
			modified: "origin igp med 50",
			want:     []string{"4 0 00000032"},
		},
		{
			name:     "remove community",
			original: "origin igp community 65000:1",
			modified: "origin igp",
			want:     []string{"8 0 "},
		},
		{
			name:     "nlri-only change emits no attr ops",
			original: "origin igp nlri ipv4/unicast add 10.0.0.0/24 10.1.0.0/24",
			modified: "origin igp nlri ipv4/unicast add 10.0.0.0/24",
			want:     []string{},
		},
		{
			name:     "as-path prepend",
			original: "origin igp",
			modified: "origin igp as-path-prepend 3",
			localAS:  65000,
			want:     []string{"2 3 02030000fde80000fde80000fde8"},
		},
		{
			name:     "remove-private strip",
			original: "origin igp",
			modified: "origin igp remove-private strip",
			attrs:    attrsPrivate,
			asn4:     true,
			peerAS:   65001,
			want:     []string{"2 0 02010000fbf001010000fbf1"},
		},
		{
			name:     "remove-private peer-as",
			original: "origin igp",
			modified: "origin igp remove-private peer-as",
			attrs:    attrsPrivate,
			asn4:     true,
			peerAS:   65001,
			want:     []string{"2 0 02020000fbf00000fde901020000fbf10000fde9"},
		},
		{
			name:     "remove-private suppresses all-private AS4_PATH",
			original: "origin igp",
			modified: "origin igp remove-private strip",
			attrs:    attrsAS4,
			asn4:     true,
			peerAS:   65001,
			want:     []string{"17 4 "},
		},
		{
			name:     "community-add directive",
			original: "origin igp",
			modified: "origin igp community-add 65000:1 65000:2",
			want:     []string{"8 1 fde80001fde80002"},
		},
		{
			name:     "community-remove directive splits per value",
			original: "origin igp",
			modified: "origin igp community-remove 65000:1 65000:2",
			want:     []string{"8 2 fde80001", "8 2 fde80002"},
		},
		{
			name:     "combined set+remove+prepend+remove-private",
			original: "origin igp community 65000:99",
			modified: "origin egp as-path-prepend 2 remove-private strip",
			attrs:    attrsPrivate,
			asn4:     true,
			peerAS:   65001,
			localAS:  64999,
			want:     []string{"1 0 01", "2 0 02010000fbf001010000fbf1", "2 3 02020000fde70000fde7", "8 0 "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := goldenOps(runFilterDeltaExtractors(tt.original, tt.modified, tt.attrs, tt.asn4, tt.peerAS, tt.localAS))
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestFilterDeltaParseCallCount proves the call-site sequence parses the
// original filter text exactly once and the modified text exactly once.
//
// VALIDATES: AC-2/AC-3 -- no redundant parseFilterAttrs calls on the modify path.
// PREVENTS: extractors silently re-parsing text the call site already parsed.
func TestFilterDeltaParseCallCount(t *testing.T) {
	attrsPrivate := filterDeltaPrivateASFixture()
	// Retry guards against a stray background goroutine (another test's
	// reactor) bumping the global counter inside the measurement window.
	// A real regression adds parses on every attempt, so 3 misses = fail.
	var got uint64
	for range 3 {
		before := parseFilterAttrsCalls.Load()
		runFilterDeltaExtractors("origin igp", "origin egp as-path-prepend 2 remove-private strip", attrsPrivate, true, 65001, 65000)
		got = parseFilterAttrsCalls.Load() - before
		if got == 2 {
			return
		}
	}
	require.Equal(t, uint64(2), got, "modify path must parse original once and modified once, got %d parses", got)
}

// BenchmarkFilterModifyEgress measures the text-delta-to-wire-ops sequence as
// invoked per modified UPDATE on the egress hot path (AC-6 evidence).
func BenchmarkFilterModifyEgress(b *testing.B) {
	attrsWire := filterDeltaPrivateASFixture()
	original := "origin igp local-preference 100 community 65001:100 65001:200 nlri ipv4/unicast add 10.0.0.0/24"
	modified := "origin igp local-preference 200 community 65001:100 65001:200 as-path-prepend 2 remove-private strip nlri ipv4/unicast add 10.0.0.0/24"
	b.ReportAllocs()
	for b.Loop() {
		var mods filterapi.ModAccumulator
		origAttrs := parseFilterAttrs(original)
		modAttrs := parseFilterAttrs(modified)
		textDeltaToModOps(origAttrs, modAttrs, &mods)
		ExtractRemovePrivateASOps(modAttrs, attrsWire, true, 65001, &mods)
		ExtractASPathPrependOps(modAttrs, 65000, &mods)
	}
}

// TestMEDRemoveNeedsAMetricToRemove covers the gate the two call sites of
// ExtractMEDRemoveOps ask first: a configured removal on a route that carries no
// metric changes no byte, so it must not force the payload rebuild. Its sibling
// applyFactsMED (forward_med.go) refuses the same cost in the same words, and
// the route-server fast path is what both are protecting.
//
// THE GATE READS THE SUBJECT ALONE, and this is where both of the ways a metric
// can be there are pinned. appendSingleAttr (filter_format.go) renders `med`
// whenever the wire carries MULTI_EXIT_DISC, and applyFilterDelta
// (filter_chain.go) merges every filter's delta INTO the current subject, so a
// metric the peer sent and a metric an earlier filter set are both in the one
// text. The wire was a second reading of the first of those, and it is gone.
//
// The route that arrived WITHOUT a metric is the case that keeps the gate: it
// must still answer false, or `modify { del { med } }` forces a payload rebuild
// for a byte that is not there.
func TestMEDRemoveNeedsAMetricToRemove(t *testing.T) {
	s := newMEDSource([]byte{0x00, 0x00, 0x00, 0x64})
	bare := buildModTestPayload(slices.Concat(s.origin, s.community), s.nlri)
	noAttrs := parseFilterAttrs("origin igp")

	// The text a filter is actually handed for the metric-carrying route. It
	// names the metric, which is what lets one reading serve the whole gate.
	text := string(AppendUpdateForFilter(nil, medAttrsWire(t, s.payload), wireu.NewWireUpdate(s.payload, 0), nil))
	bareText := string(AppendUpdateForFilter(nil, medAttrsWire(t, bare), wireu.NewWireUpdate(bare, 0), nil))
	require.Contains(t, text, "med 100",
		"the subject names MULTI_EXIT_DISC, so the gate needs no second reading of the wire")
	require.NotContains(t, bareText, "med ",
		"the bare fixture carries no metric, so nothing names one")

	require.True(t, medRemoveHasWork(parseFilterAttrs(text)),
		"the metric the peer sent is a metric to remove")
	require.False(t, medRemoveHasWork(parseFilterAttrs(bareText)),
		"a route that arrived with no metric gives the removal nothing to do")
	require.False(t, medRemoveHasWork(noAttrs),
		"and neither does a subject naming no metric at all")
	assert.True(t, medRemoveHasWork(parseFilterAttrs("origin igp med 50 med-remove")),
		"a metric an earlier filter in the chain set is a metric to remove")

	var mods filterapi.ModAccumulator
	ExtractMEDRemoveOps(parseFilterAttrs(applyFilterDelta(text, "med-remove")), &mods)
	require.Equal(t, 1, mods.Len(), "the directive records one suppression")

	var none filterapi.ModAccumulator
	ExtractMEDRemoveOps(parseFilterAttrs(text), &none)
	assert.Zero(t, none.Len(), "no directive, no removal")

	result, _, fail := buildModifiedPayload(bare, &none, attrModHandlersWithDefaults(), nil, nil)
	assert.Equal(t, modifyFailureNone, fail)
	assert.Nil(t, result, "no operation means the route stays on the zero-copy path")
}

// TestMEDRemoveObeysTheChainOrder covers the two opposite instructions about
// attribute 4 meeting in one ordered chain. The merged filter text holds one
// slot per attribute and cannot carry which filter came last, so
// filterAttrs.merge cancels a removal when a LATER filter sets the metric. The
// reverse order needs no such clearing: filterapi.LastSetOrSuppress is last-wins
// and ExtractMEDRemoveOps records its Suppress after textDeltaToModOps has
// recorded the Set.
func TestMEDRemoveObeysTheChainOrder(t *testing.T) {
	s := newMEDSource([]byte{0x00, 0x00, 0x00, 0x64})
	updateText := "origin igp med 100 community 65000:100"

	ops := func(t *testing.T, first, second string) []byte {
		t.Helper()
		modAttrs := parseFilterAttrs(applyFilterDelta(applyFilterDelta(updateText, first), second))
		var mods filterapi.ModAccumulator
		textDeltaToModOps(parseFilterAttrs(updateText), modAttrs, &mods)
		ExtractMEDRemoveOps(modAttrs, &mods)
		result, _, fail := buildModifiedPayload(s.payload, &mods, attrModHandlersWithDefaults(), nil, nil)
		require.Equal(t, modifyFailureNone, fail)
		require.NotNil(t, result)
		return result
	}

	t.Run("a_later_set_wins", func(t *testing.T) {
		want := buildModTestPayload(
			slices.Concat(s.origin, makeAttr(0x80, 4, []byte{0x00, 0x00, 0x00, 0xC8}), s.community), s.nlri)
		assert.Equal(t, want, ops(t, "med-remove", "med 200"),
			"the operator's second filter set the metric, so it reaches the wire")
	})

	t.Run("a_later_removal_wins", func(t *testing.T) {
		assert.NotContains(t, rebuiltAttrs(t, ops(t, "med 200", "med-remove")), byte(attribute.AttrMED),
			"the operator's second filter removed the metric, so nothing reaches the wire")
	})

	// The same chain on a route that arrived with NO metric, driven through the
	// gate the production call sites ask. The only metric in play is the one the
	// first filter added, so a gate that read the wire alone would skip the
	// removal and leave the Set standing.
	t.Run("a_later_removal_wins_on_a_metric_less_route", func(t *testing.T) {
		bare := buildModTestPayload(slices.Concat(s.origin, s.community), s.nlri)
		bareText := string(AppendUpdateForFilter(nil, medAttrsWire(t, bare), wireu.NewWireUpdate(bare, 0), nil))

		modAttrs := parseFilterAttrs(applyFilterDelta(applyFilterDelta(bareText, "med 200"), "med-remove"))
		var mods filterapi.ModAccumulator
		textDeltaToModOps(parseFilterAttrs(bareText), modAttrs, &mods)
		require.True(t, medRemoveHasWork(modAttrs),
			"the metric the chain added is the work the removal has to do")
		ExtractMEDRemoveOps(modAttrs, &mods)

		result, _, fail := buildModifiedPayload(bare, &mods, attrModHandlersWithDefaults(), nil, nil)
		require.Equal(t, modifyFailureNone, fail)
		require.NotNil(t, result)
		assert.NotContains(t, rebuiltAttrs(t, result), byte(attribute.AttrMED),
			"the metric the chain added is the metric the chain removed, so none reaches the wire")
	})
}

// TestTextDeltaRecordsNothingForAnUnchangedSubject runs the subject the product
// renders through a chain that changes nothing about it, twice: once with every
// filter answering accept, and once with a filter that SETS an attribute to the
// value the route already carries.
//
// VALIDATES: AC-6 and AC-7 -- the chain output text equals its input, and
// textDeltaToModOps records no operation for an attribute whose before and
// after values are equal.
// PREVENTS: a rebuild of the UPDATE body for a route no filter asked to change,
// and the removal arm of textDeltaToModOps stripping an attribute nobody asked
// to strip (assumption A-2). Both conditions filter_ordered.go reads before it
// calls buildModifiedPayload are asserted here: the text comparison and the
// operation count. The five names below reached no filter until the renderer
// was repaired, so their slots were empty in both maps and neither arm could
// fire on them; now that the subject carries them, a round trip that reshaped
// the text would make every route on every session look modified.
func TestTextDeltaRecordsNothingForAnUnchangedSubject(t *testing.T) {
	subject := filterSubjectFixture(t)

	// assertNoWork asserts the two conditions the import site reads before it
	// rebuilds a payload (filter_ordered.go): the chain returned the text it was
	// given, and the delta holds no wire operation and no NLRI override.
	assertNoWork := func(t *testing.T, text string) {
		t.Helper()
		assert.Equal(t, subject, text, "a chain that changes nothing must return the text it was handed")

		var mods filterapi.ModAccumulator
		textDeltaToModOps(parseFilterAttrs(subject), parseFilterAttrs(text), &mods)
		assert.Equal(t, 0, mods.Len(), "no attribute value changed, so no wire operation is owed: %v",
			goldenOps(mods.Ops()))
		assert.Nil(t, extractLegacyNLRIOverride(subject, text), "the NLRI block is unchanged too")
	}

	t.Run("every filter accepts", func(t *testing.T) {
		res := PolicyFilterChain(frefs("a:accept", "b:accept"), "import", "10.0.0.1", 65000, subject,
			func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
				return PolicyResponse{Action: PolicyAccept}
			})
		require.Equal(t, PolicyAccept, res.Action)
		assertNoWork(t, res.Text)
	})

	// One row for each attribute the renderer dropped until this spec, set to
	// the value attrsFixtureWire puts on the wire. AC-7 names local-preference;
	// the other four reach the same arm of textDeltaToModOps and were equally
	// unreachable before, so each one is a row rather than a claim.
	for _, delta := range []string{
		"origin incomplete",
		"med 100",
		"local-preference 150",
		"atomic-aggregate",
		"cluster-list 1.1.1.1 2.2.2.2",
	} {
		t.Run("a filter sets "+delta, func(t *testing.T) {
			res := PolicyFilterChain(frefs("a:set"), "import", "10.0.0.1", 65000, subject,
				func(_, _, _, _ string, _ uint32, _ string) PolicyResponse {
					return PolicyResponse{Action: PolicyModify, Delta: delta}
				})
			require.Equal(t, PolicyAccept, res.Action)
			assertNoWork(t, res.Text)
		})
	}
}
