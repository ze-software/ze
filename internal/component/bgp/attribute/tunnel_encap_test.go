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
