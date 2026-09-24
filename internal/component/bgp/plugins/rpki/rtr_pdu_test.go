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
	// RFC requirement: RFC6810-5-1 positive -- the Reset Query has no Session ID; that field is
	// "unspecified content" for this PDU and writeResetQuery emits it as zero on transmission
	// (buf[2:4] == 0), while the version/type/length fields still carry their real values.
	// RFC requirement: RFC8210-5-1 positive -- v1 keeps the same rule: the reserved header bytes of
	// a Reset Query are emitted as zero on transmission.
	buf := make([]byte, 16)
	n := writeResetQuery(buf, 0, rtrVersionMax)
	assert.Equal(t, 8, n)
	assert.Equal(t, rtrVersionMax, buf[0])
	assert.Equal(t, pduResetQuery, buf[1])
	assert.Equal(t, uint16(0), binary.BigEndian.Uint16(buf[2:4]))
	assert.Equal(t, uint32(8), binary.BigEndian.Uint32(buf[4:8]))
}

// TestWriteSerialQuery verifies Serial Query PDU wire format.
//
// VALIDATES: Serial Query is 12 bytes with correct session ID and serial.
// PREVENTS: Wrong session/serial causing cache to reset instead of incremental.
func TestWriteSerialQuery(t *testing.T) {
	// RFC requirement: RFC6810-5-1 negative -- the same header bytes 2-3 are SPECIFIED content in a
	// Serial Query (the Session ID), so they are NOT forced to zero: writeSerialQuery emits 0x1234
	// there. This pins the "unspecified fields MUST be zero" rule to unspecified fields only, not a
	// blanket zeroing of the header.
	// RFC requirement: RFC8210-5-1 negative -- header bytes 2-3 of a Serial Query are the Session ID,
	// not a reserved field, so they are NOT zeroed: a writer that zeroed every non-version header byte
	// would break the Session ID and is rejected by this assertion.
	buf := make([]byte, 16)
	n := writeSerialQuery(buf, 0, rtrVersionMax, 0x1234, 0xABCD0001)
	assert.Equal(t, 12, n)
	assert.Equal(t, rtrVersionMax, buf[0])
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
	// RFC requirement: RFC6810-5.1-1 positive -- a Prefix PDU whose Max Length (24) is >= its Prefix
	// Length (8) is accepted by parsePrefixPDU and yields a usable VRP.
	// RFC requirement: RFC8210-5.1-2 positive -- the v1 Prefix PDU carries the same Max Length rule and
	// the same 20-byte layout, so a conformant PDU (maxLen 24 >= prefixLen 8) is accepted.
	// Build a valid IPv4 Prefix PDU: 10.0.0.0/8, maxLen=24, ASN=65001, announce
	buf := make([]byte, 20)
	buf[0] = rtrVersionMin
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
	buf[0] = rtrVersionMin
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
	buf[0] = rtrVersionMin
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
	// RFC requirement: RFC6810-5.1-1 negative -- a Prefix PDU whose Max Length (16) is LESS than its
	// Prefix Length (24) is rejected by parsePrefixPDU, so the impossible VRP never enters the cache.
	// RFC requirement: RFC8210-5.1-2 negative -- the same rejection holds for a v1 Prefix PDU: Max
	// Length below Prefix Length is refused rather than stored.
	buf := make([]byte, 20)
	buf[0] = rtrVersionMin
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
	buf[0] = rtrVersionMin
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
	buf[0] = rtrVersionMin
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
	// RFC requirement: RFC8210-12-1 negative -- No Data Available is a non-fatal condition, unlike the fatal version error.
	assert.False(t, isFatalError(errNoDataAvail))
	assert.True(t, isFatalError(errUnsupportedVersion))
}

// aspaPDU encodes the 8210bis-27 Section 5.12 layout independently of the parser.
func aspaPDU(customer uint32, providers ...uint32) []byte {
	buf := make([]byte, 12+4*len(providers))
	buf[0], buf[1], buf[2] = 2, 11, 1
	binary.BigEndian.PutUint32(buf[4:8], uint32(len(buf))) //nolint:gosec // bounded fixture
	binary.BigEndian.PutUint32(buf[8:12], customer)
	for i, provider := range providers {
		binary.BigEndian.PutUint32(buf[12+4*i:], provider)
	}
	return buf
}

// TestParseASPAPDU checks the actual RTR v2 offsets and complete provider list.
func TestParseASPAPDU(t *testing.T) {
	// RFC requirement: DRAFT-IETF-SIDROPS-8210BIS-5.12-1 positive -- an ASPA announcement containing three providers is accepted.
	// RFC requirement: DRAFT-IETF-SIDROPS-8210BIS-5.12-3 positive -- distinct ascending providers are accepted.
	rec, announce, err := parseASPAPDU(aspaPDU(64500, 100, 200, 300))
	require.NoError(t, err)
	assert.True(t, announce)
	assert.Equal(t, uint32(64500), rec.CustomerAS)
	assert.Equal(t, []uint32{100, 200, 300}, rec.Providers)
}

// TestParseASPAPDUWithdraw checks a withdrawal contains only the customer AS.
func TestParseASPAPDUWithdraw(t *testing.T) {
	buf := aspaPDU(64501)
	buf[2] = 0
	rec, announce, err := parseASPAPDU(buf)
	require.NoError(t, err)
	assert.False(t, announce)
	assert.Equal(t, uint32(64501), rec.CustomerAS)
	assert.Empty(t, rec.Providers)
}

// TestParseASPAPDUMalformed rejects truncated framing and an empty announcement.
func TestParseASPAPDUMalformed(t *testing.T) {
	// RFC requirement: DRAFT-IETF-SIDROPS-8210BIS-5.12-1 negative -- an announcement with no providers returns the provider-list error.
	_, _, err := parseASPAPDU(aspaPDU(64500))
	require.ErrorIs(t, err, errASPAProviderList)
	_, _, err = parseASPAPDU([]byte{2, 11})
	require.Error(t, err)
	buf := aspaPDU(64500, 100)
	binary.BigEndian.PutUint32(buf[4:8], uint32(len(buf)+4)) //nolint:gosec // bounded fixture
	_, _, err = parseASPAPDU(buf)
	require.Error(t, err)
}

// TestParseASPAPDUReservedIgnored verifies the zero field and reserved flags do
// not alter the announcement. There is no AFI field in the current ASPA PDU.
func TestParseASPAPDUReservedIgnored(t *testing.T) {
	buf := aspaPDU(64500, 100)
	buf[2], buf[3] = 0xff, 0xff
	rec, announce, err := parseASPAPDU(buf)
	require.NoError(t, err)
	assert.True(t, announce)
	assert.Equal(t, uint32(64500), rec.CustomerAS)
	assert.Equal(t, []uint32{100}, rec.Providers)
}

// TestParseASPAPDUSelfRef rejects a customer naming itself as provider.
func TestParseASPAPDUSelfRef(t *testing.T) {
	_, _, err := parseASPAPDU(aspaPDU(64500, 100, 64500))
	require.ErrorIs(t, err, errASPAProviderList)
}

// TestParseASPAPDUUnsorted rejects both inversions and duplicate providers.
func TestParseASPAPDUUnsorted(t *testing.T) {
	// RFC requirement: DRAFT-IETF-SIDROPS-8210BIS-5.12-3 negative -- unordered or duplicate provider lists are rejected.
	for _, providers := range [][]uint32{{300, 100}, {100, 100}} {
		_, _, err := parseASPAPDU(aspaPDU(64500, providers...))
		require.ErrorIs(t, err, errASPAProviderList)
	}
}

// TestParseASPAPDUReservedCustomerAS rejects a reserved customer ASN.
func TestParseASPAPDUReservedCustomerAS(t *testing.T) {
	_, _, err := parseASPAPDU(aspaPDU(0, 100))
	require.Error(t, err)
}

// TestParseASPAZeroProvider distinguishes an AS0 ASPA from a mixed provider list.
func TestParseASPAZeroProvider(t *testing.T) {
	rec, announce, err := parseASPAPDU(aspaPDU(64500, 0))
	require.NoError(t, err)
	assert.True(t, announce)
	assert.Equal(t, []uint32{0}, rec.Providers)
	_, _, err = parseASPAPDU(aspaPDU(64500, 0, 100))
	require.ErrorIs(t, err, errASPAProviderList)
	buf := aspaPDU(64500, 100)
	buf[2] = 0
	_, _, err = parseASPAPDU(buf)
	require.ErrorIs(t, err, errASPAProviderList)
}

// TestParsePrefixPDUFlagsHighBitsIgnored verifies that only bit 0 of the Prefix PDU Flags field is
// read, and that the remaining bits are ignored on receipt.
//
// VALIDATES: RFC 8210 Section 5.1 -- Flags bits other than bit 0 are zero on transmission and MUST be
// ignored on receipt. parsePrefixPDU computes announce as flags&1 (rtr_pdu.go:132), so the seven high
// bits change nothing about how the PDU is interpreted.
// PREVENTS: A cache setting a reserved Flags bit turning an announcement into a withdrawal (or the
// reverse), or the parser rejecting an otherwise valid PDU because of a reserved bit.
func TestParsePrefixPDUFlagsHighBitsIgnored(t *testing.T) {
	// build a 20-byte IPv4 Prefix PDU for 10.0.0.0/8, maxLen 24, AS 65001 with the given Flags byte.
	build := func(flags byte) []byte {
		buf := make([]byte, pduIPv4PrefixLen)
		buf[0] = rtrVersionMin
		buf[1] = pduIPv4Prefix
		binary.BigEndian.PutUint32(buf[4:8], pduIPv4PrefixLen)
		buf[8] = flags
		buf[9] = 8
		buf[10] = 24
		buf[12] = 10
		binary.BigEndian.PutUint32(buf[16:20], 65001)
		return buf
	}

	// RFC requirement: RFC8210-5.1-1 positive -- every reserved Flags bit is set (0xFF) alongside
	// bit 0; the PDU is still accepted and still read as an announcement, proving the high bits are
	// ignored on receipt rather than validated or mixed into the decision.
	vrp, announce, err := parseIPv4Prefix(build(0xFF))
	require.NoError(t, err, "reserved Flags bits must not make the PDU invalid")
	assert.True(t, announce, "bit 0 set means announce whatever the reserved bits carry")
	assert.Equal(t, "10.0.0.0/8", vrp.Prefix.String())
	assert.Equal(t, uint32(65001), vrp.ASN)

	// RFC requirement: RFC8210-5.1-1 negative -- the same reserved bits set but bit 0 CLEAR (0xFE)
	// must still be a withdrawal. A parser that looked at any bit other than bit 0 would read this
	// non-conformant-looking PDU as an announcement and wrongly install the VRP.
	vrp, announce, err = parseIPv4Prefix(build(0xFE))
	require.NoError(t, err)
	assert.False(t, announce, "bit 0 clear means withdraw despite every reserved bit being set")
	assert.Equal(t, "10.0.0.0/8", vrp.Prefix.String())
}
