// VALIDATES: Tunnel Encapsulation attribute (code 23, RFC 9012) parse and encode.
// PREVENTS: Wrong TLV/sub-TLV framing, loss of unknown tunnel types on forwarding.

package attribute

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTunnelEncapCode(t *testing.T) {
	assert.Equal(t, AttributeCode(23), AttrTunnelEncap)
}

func TestTunnelEncapParseEmpty(t *testing.T) {
	te, err := ParseTunnelEncap(nil)
	require.NoError(t, err)
	assert.Empty(t, te.TLVs)
}

func TestTunnelEncapParseSingleTLV(t *testing.T) {
	// Tunnel Type 15 (SR Policy), length 8, value = 8 bytes of zeros
	data := make([]byte, 12)
	binary.BigEndian.PutUint16(data[0:2], 15) // tunnel type
	binary.BigEndian.PutUint16(data[2:4], 8)  // length
	// data[4:12] = zeros (sub-TLV content)

	te, err := ParseTunnelEncap(data)
	require.NoError(t, err)
	require.Len(t, te.TLVs, 1)
	assert.Equal(t, uint16(15), te.TLVs[0].TunnelType)
	assert.Len(t, te.TLVs[0].Value, 8)
}

func TestTunnelEncapParseMultipleTLVs(t *testing.T) {
	// Two TLVs: type 15 (4 bytes value) + type 2 (2 bytes value)
	data2 := make([]byte, 14) // (4 hdr + 4 val) + (4 hdr + 2 val)
	binary.BigEndian.PutUint16(data2[0:2], 15)
	binary.BigEndian.PutUint16(data2[2:4], 4)
	binary.BigEndian.PutUint16(data2[8:10], 2)
	binary.BigEndian.PutUint16(data2[10:12], 2)

	te, err := ParseTunnelEncap(data2)
	require.NoError(t, err)
	require.Len(t, te.TLVs, 2)
	assert.Equal(t, uint16(15), te.TLVs[0].TunnelType)
	assert.Equal(t, uint16(2), te.TLVs[1].TunnelType)
}

func TestTunnelEncapParseTruncatedHeader(t *testing.T) {
	_, err := ParseTunnelEncap([]byte{0x00, 0x0F, 0x00}) // only 3 bytes, need 4
	assert.Error(t, err)
}

func TestTunnelEncapParseTruncatedValue(t *testing.T) {
	data := make([]byte, 6)
	binary.BigEndian.PutUint16(data[0:2], 15)
	binary.BigEndian.PutUint16(data[2:4], 10) // claims 10 bytes but only 2 available
	_, err := ParseTunnelEncap(data)
	assert.Error(t, err)
}

func TestTunnelEncapRoundTrip(t *testing.T) {
	// Build a TLV with type 15, some value bytes
	value := []byte{0x0C, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x64}
	tlv := TunnelTLV{TunnelType: 15, Value: value}
	te := &TunnelEncap{TLVs: []TunnelTLV{tlv}}

	buf := make([]byte, te.Len())
	n := te.WriteTo(buf, 0)
	assert.Equal(t, te.Len(), n)

	parsed, err := ParseTunnelEncap(buf)
	require.NoError(t, err)
	require.Len(t, parsed.TLVs, 1)
	assert.Equal(t, uint16(15), parsed.TLVs[0].TunnelType)
	assert.Equal(t, value, parsed.TLVs[0].Value)
}

func TestTunnelEncapFlags(t *testing.T) {
	te := &TunnelEncap{}
	assert.Equal(t, AttrTunnelEncap, te.Code())
	assert.Equal(t, FlagOptional|FlagTransitive, te.Flags())
}

func TestTunnelEncapPreservesUnknownTypes(t *testing.T) {
	// Unknown tunnel type 99 with 3 bytes of data
	data := make([]byte, 7)
	binary.BigEndian.PutUint16(data[0:2], 99)
	binary.BigEndian.PutUint16(data[2:4], 3)
	data[4] = 0xAA
	data[5] = 0xBB
	data[6] = 0xCC

	te, err := ParseTunnelEncap(data)
	require.NoError(t, err)
	require.Len(t, te.TLVs, 1)
	assert.Equal(t, uint16(99), te.TLVs[0].TunnelType)
	assert.Equal(t, []byte{0xAA, 0xBB, 0xCC}, te.TLVs[0].Value)

	// Round-trip preserves unknown type
	buf := make([]byte, te.Len())
	te.WriteTo(buf, 0)
	assert.Equal(t, data, buf)
}

func TestTunnelTLVSubTLVsShortHeader(t *testing.T) {
	// Two sub-TLVs with short headers (type < 128):
	// type=12(Preference), length=8, value=8 bytes
	// type=15(Priority), length=2, value=2 bytes
	value := make([]byte, 14) // (2+8) + (2+2)
	value[0] = SubTLVPreference
	value[1] = 8
	binary.BigEndian.PutUint32(value[6:10], 200) // preference at offset 4 within value
	value[10] = SubTLVPriority
	value[11] = 2
	value[12] = 0x00
	value[13] = 0x05

	tlv := TunnelTLV{TunnelType: 15, Value: value}
	stlvs, err := tlv.SubTLVs()
	require.NoError(t, err)
	require.Len(t, stlvs, 2)
	assert.Equal(t, SubTLVPreference, stlvs[0].Type)
	assert.Len(t, stlvs[0].Value, 8)
	assert.Equal(t, SubTLVPriority, stlvs[1].Type)
	assert.Len(t, stlvs[1].Value, 2)
}

func TestTunnelTLVSubTLVsLongHeader(t *testing.T) {
	// Sub-TLV type 128 (Segment List) uses 2-byte length.
	// type=128, length=4 (2 bytes), value=4 bytes
	value := make([]byte, 7) // 1 + 2 + 4
	value[0] = SubTLVSegmentList
	binary.BigEndian.PutUint16(value[1:3], 4)
	value[3] = 0x01
	value[4] = 0x02
	value[5] = 0x03
	value[6] = 0x04

	tlv := TunnelTLV{TunnelType: 15, Value: value}
	stlvs, err := tlv.SubTLVs()
	require.NoError(t, err)
	require.Len(t, stlvs, 1)
	assert.Equal(t, SubTLVSegmentList, stlvs[0].Type)
	assert.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, stlvs[0].Value)
}

func TestTunnelTLVSubTLVsMixed(t *testing.T) {
	// Short header sub-TLV followed by long header sub-TLV.
	// type=12, len=8, val=8bytes | type=128, len=2, val=2bytes
	value := make([]byte, 15) // (2+8) + (3+2)
	value[0] = SubTLVPreference
	value[1] = 8
	// value[2:10] = preference sub-TLV value (zeros, flags+reserved+pref=0)
	value[10] = SubTLVSegmentList
	binary.BigEndian.PutUint16(value[11:13], 2)
	value[13] = 0xAA
	value[14] = 0xBB

	tlv := TunnelTLV{TunnelType: 15, Value: value}
	stlvs, err := tlv.SubTLVs()
	require.NoError(t, err)
	require.Len(t, stlvs, 2)
	assert.Equal(t, SubTLVPreference, stlvs[0].Type)
	assert.Equal(t, SubTLVSegmentList, stlvs[1].Type)
}

func TestTunnelTLVSubTLVsTruncated(t *testing.T) {
	// Short header says length=10 but only 3 bytes of value available.
	value := []byte{SubTLVPreference, 10, 0x00, 0x00, 0x00}
	tlv := TunnelTLV{TunnelType: 15, Value: value}
	_, err := tlv.SubTLVs()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "truncated")
}

func TestTunnelTLVSubTLVsEmpty(t *testing.T) {
	tlv := TunnelTLV{TunnelType: 15, Value: nil}
	stlvs, err := tlv.SubTLVs()
	assert.NoError(t, err)
	assert.Empty(t, stlvs)
}

func TestTunnelTLVPreference(t *testing.T) {
	// Preference sub-TLV (RFC 9830 Section 2.4.1): type=12, length=6,
	// value = flags(1) + reserved(1) + preference(4).
	value := make([]byte, 8) // 2 (header) + 6 (value)
	value[0] = SubTLVPreference
	value[1] = preferenceValueLen
	// value[2] = flags (0)
	// value[3] = reserved (0)
	binary.BigEndian.PutUint32(value[4:8], 200) // preference

	tlv := TunnelTLV{TunnelType: 15, Value: value}
	pref, ok := tlv.Preference()
	assert.True(t, ok)
	assert.Equal(t, uint32(200), pref)
}

func TestTunnelTLVPreferenceNotPresent(t *testing.T) {
	// Only a Priority sub-TLV, no Preference.
	value := []byte{SubTLVPriority, 2, 0x00, 0x05}
	tlv := TunnelTLV{TunnelType: 15, Value: value}
	pref, ok := tlv.Preference()
	assert.False(t, ok)
	assert.Equal(t, uint32(0), pref)
}

func TestTunnelTLVPreferenceMalformedValue(t *testing.T) {
	// Preference sub-TLV with value too short (4 bytes instead of the mandated 6).
	value := []byte{SubTLVPreference, 4, 0x00, 0x00, 0x00, 0x00}
	tlv := TunnelTLV{TunnelType: 15, Value: value}
	pref, ok := tlv.Preference()
	assert.False(t, ok)
	assert.Equal(t, uint32(0), pref)
}
