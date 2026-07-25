// VALIDATES: Tunnel Encapsulation attribute (code 23) decode produces human-readable output with tunnel type, preference, sub-TLV count.
// PREVENTS: TunnelEncap/SubTLVs/Preference unreachable from cross-package caller.

package decode

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

func TestDecodeTunnelEncapSingleTLV(t *testing.T) {
	// Tunnel Type 15 (SR Policy) with Preference sub-TLV (type 12, value 200).
	// Sub-TLV per RFC 9830 Section 2.4.1: type=12, length=6,
	// value = flags(1)+reserved(1)+preference(4).
	subTLV := make([]byte, 8)
	subTLV[0] = attribute.SubTLVPreference
	subTLV[1] = 6
	binary.BigEndian.PutUint32(subTLV[4:8], 200)

	data := make([]byte, 4+len(subTLV))
	binary.BigEndian.PutUint16(data[0:2], 15) // tunnel type
	binary.BigEndian.PutUint16(data[2:4], uint16(len(subTLV)))
	copy(data[4:], subTLV)

	result := decodeTunnelEncap(data)
	assert.Contains(t, result, "TT=15")
	assert.Contains(t, result, "pref=200")
	assert.Contains(t, result, "stlvs=1")
}

func TestDecodeTunnelEncapMultipleTLVs(t *testing.T) {
	// Two TLVs: type 15 (empty) + type 2 (empty).
	data := make([]byte, 8)
	binary.BigEndian.PutUint16(data[0:2], 15)
	binary.BigEndian.PutUint16(data[2:4], 0)
	binary.BigEndian.PutUint16(data[4:6], 2)
	binary.BigEndian.PutUint16(data[6:8], 0)

	result := decodeTunnelEncap(data)
	assert.Contains(t, result, "TT=15")
	assert.Contains(t, result, "TT=2")
}

func TestDecodeTunnelEncapEmpty(t *testing.T) {
	result := decodeTunnelEncap(nil)
	assert.Equal(t, "EMPTY", result)
}

func TestDecodeTunnelEncapMalformed(t *testing.T) {
	result := decodeTunnelEncap([]byte{0x00, 0x0F, 0x00})
	assert.Contains(t, result, "MALFORMED:")
}

func TestDecodeTunnelEncapNoPreference(t *testing.T) {
	// Tunnel Type 15 with Priority sub-TLV only (no Preference).
	subTLV := []byte{attribute.SubTLVPriority, 2, 0x00, 0x05}

	data := make([]byte, 4+len(subTLV))
	binary.BigEndian.PutUint16(data[0:2], 15)
	binary.BigEndian.PutUint16(data[2:4], uint16(len(subTLV)))
	copy(data[4:], subTLV)

	result := decodeTunnelEncap(data)
	assert.Contains(t, result, "TT=15")
	assert.NotContains(t, result, "pref=")
	assert.Contains(t, result, "stlvs=1")
}
