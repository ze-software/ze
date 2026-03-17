package rpki

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteResetQuery verifies Reset Query PDU wire format.
//
// VALIDATES: Reset Query is 8 bytes with version=1, type=2, zero session, length=8.
// PREVENTS: Malformed Reset Query causing cache rejection.
func TestWriteResetQuery(t *testing.T) {
	buf := make([]byte, 16)
	n := writeResetQuery(buf, 0)
	assert.Equal(t, 8, n)
	assert.Equal(t, rtrVersion, buf[0])
	assert.Equal(t, pduResetQuery, buf[1])
	assert.Equal(t, uint16(0), binary.BigEndian.Uint16(buf[2:4]))
	assert.Equal(t, uint32(8), binary.BigEndian.Uint32(buf[4:8]))
}

// TestWriteSerialQuery verifies Serial Query PDU wire format.
//
// VALIDATES: Serial Query is 12 bytes with correct session ID and serial.
// PREVENTS: Wrong session/serial causing cache to reset instead of incremental.
func TestWriteSerialQuery(t *testing.T) {
	buf := make([]byte, 16)
	n := writeSerialQuery(buf, 0, 0x1234, 0xABCD0001)
	assert.Equal(t, 12, n)
	assert.Equal(t, rtrVersion, buf[0])
	assert.Equal(t, pduSerialQuery, buf[1])
	assert.Equal(t, uint16(0x1234), binary.BigEndian.Uint16(buf[2:4]))
	assert.Equal(t, uint32(12), binary.BigEndian.Uint32(buf[4:8]))
	assert.Equal(t, uint32(0xABCD0001), binary.BigEndian.Uint32(buf[8:12]))
}

// TestParseIPv4Prefix verifies IPv4 Prefix PDU parsing.
//
// VALIDATES: Correct extraction of flags, prefix, max-length, ASN.
// PREVENTS: Wrong prefix length or ASN from RTR data.
func TestParseIPv4Prefix(t *testing.T) {
	// Build a valid IPv4 Prefix PDU: 10.0.0.0/8, maxLen=24, ASN=65001, announce
	buf := make([]byte, 20)
	buf[0] = rtrVersion
	buf[1] = pduIPv4Prefix
	binary.BigEndian.PutUint32(buf[4:8], 20)
	buf[8] = 1   // flags: announce
	buf[9] = 8   // prefix length
	buf[10] = 24 // max length
	buf[11] = 0  // zero
	buf[12] = 10 // 10.0.0.0
	buf[13] = 0
	buf[14] = 0
	buf[15] = 0
	binary.BigEndian.PutUint32(buf[16:20], 65001) // ASN

	vrp, announce, err := parseIPv4Prefix(buf)
	require.NoError(t, err)
	assert.True(t, announce)
	assert.Equal(t, uint32(65001), vrp.ASN)
	assert.Equal(t, uint8(24), vrp.MaxLength)
	assert.Equal(t, "10.0.0.0/8", vrp.Prefix.String())
}

// TestParseIPv4PrefixWithdraw verifies withdraw flag parsing.
//
// VALIDATES: flags=0 means withdraw (not announce).
// PREVENTS: Treating withdrawals as announcements.
func TestParseIPv4PrefixWithdraw(t *testing.T) {
	buf := make([]byte, 20)
	buf[0] = rtrVersion
	buf[1] = pduIPv4Prefix
	binary.BigEndian.PutUint32(buf[4:8], 20)
	buf[8] = 0    // flags: withdraw
	buf[9] = 24   // prefix length
	buf[10] = 24  // max length
	buf[12] = 192 // 192.168.1.0
	buf[13] = 168
	buf[14] = 1
	buf[15] = 0
	binary.BigEndian.PutUint32(buf[16:20], 65002)

	vrp, announce, err := parseIPv4Prefix(buf)
	require.NoError(t, err)
	assert.False(t, announce)
	assert.Equal(t, uint32(65002), vrp.ASN)
}

// TestParseIPv4PrefixInvalidLength verifies boundary validation.
//
// VALIDATES: Prefix length > 32 rejected.
// PREVENTS: Invalid prefix length causing panics.
func TestParseIPv4PrefixInvalidLength(t *testing.T) {
	buf := make([]byte, 20)
	buf[0] = rtrVersion
	buf[1] = pduIPv4Prefix
	binary.BigEndian.PutUint32(buf[4:8], 20)
	buf[9] = 33 // invalid prefix length
	buf[10] = 33

	_, _, err := parseIPv4Prefix(buf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "> 32")
}

// TestParseIPv4PrefixMaxLenLessThanPrefixLen verifies maxLen >= prefixLen.
//
// VALIDATES: maxLength < prefixLength rejected.
// PREVENTS: Impossible VRP from entering cache.
func TestParseIPv4PrefixMaxLenLessThanPrefixLen(t *testing.T) {
	buf := make([]byte, 20)
	buf[0] = rtrVersion
	buf[1] = pduIPv4Prefix
	binary.BigEndian.PutUint32(buf[4:8], 20)
	buf[9] = 24  // prefix length
	buf[10] = 16 // max length < prefix length

	_, _, err := parseIPv4Prefix(buf)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid")
}

// TestParseIPv6Prefix verifies IPv6 Prefix PDU parsing.
//
// VALIDATES: Correct extraction of IPv6 prefix, max-length, ASN.
// PREVENTS: IPv6 address parsing errors.
func TestParseIPv6Prefix(t *testing.T) {
	buf := make([]byte, 32)
	buf[0] = rtrVersion
	buf[1] = pduIPv6Prefix
	binary.BigEndian.PutUint32(buf[4:8], 32)
	buf[8] = 1   // announce
	buf[9] = 48  // /48
	buf[10] = 48 // maxLen /48
	// 2001:db8:: at offset 12
	buf[12] = 0x20
	buf[13] = 0x01
	buf[14] = 0x0d
	buf[15] = 0xb8
	binary.BigEndian.PutUint32(buf[28:32], 65003)

	vrp, announce, err := parseIPv6Prefix(buf)
	require.NoError(t, err)
	assert.True(t, announce)
	assert.Equal(t, uint32(65003), vrp.ASN)
	assert.Equal(t, uint8(48), vrp.MaxLength)
	assert.Equal(t, vrp.Prefix.IP.String(), "2001:db8::")
}

// TestParseEndOfData verifies End of Data PDU parsing with timing parameters.
//
// VALIDATES: All 5 fields extracted correctly from 24-byte PDU.
// PREVENTS: Timing parameters being ignored or misread.
func TestParseEndOfData(t *testing.T) {
	buf := make([]byte, 24)
	buf[0] = rtrVersion
	buf[1] = pduEndOfData
	binary.BigEndian.PutUint16(buf[2:4], 0x5678) // session ID
	binary.BigEndian.PutUint32(buf[4:8], 24)     // length
	binary.BigEndian.PutUint32(buf[8:12], 42)    // serial
	binary.BigEndian.PutUint32(buf[12:16], 3600) // refresh
	binary.BigEndian.PutUint32(buf[16:20], 600)  // retry
	binary.BigEndian.PutUint32(buf[20:24], 7200) // expire

	params, err := parseEndOfData(buf)
	require.NoError(t, err)
	assert.Equal(t, uint16(0x5678), params.SessionID)
	assert.Equal(t, uint32(42), params.SerialNumber)
	assert.Equal(t, uint32(3600), params.RefreshInterval)
	assert.Equal(t, uint32(600), params.RetryInterval)
	assert.Equal(t, uint32(7200), params.ExpireInterval)
}

// TestParseHeaderTooShort verifies header rejects short buffers.
//
// VALIDATES: Buffer < 8 bytes returns error.
// PREVENTS: Panic on truncated PDU.
func TestParseHeaderTooShort(t *testing.T) {
	_, err := parseHeader(make([]byte, 4))
	assert.Error(t, err)
}

// TestIsFatalError verifies error code classification.
//
// VALIDATES: Only "No Data Available" (code 2) is non-fatal.
// PREVENTS: Fatal errors being silently ignored.
func TestIsFatalError(t *testing.T) {
	assert.True(t, isFatalError(0))  // Corrupt Data
	assert.True(t, isFatalError(1))  // Internal Error
	assert.False(t, isFatalError(2)) // No Data Available
	assert.True(t, isFatalError(3))  // Invalid Request
	assert.True(t, isFatalError(8))  // Unexpected Version
}
