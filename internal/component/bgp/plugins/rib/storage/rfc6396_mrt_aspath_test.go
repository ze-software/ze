// RFC: rfc/short/rfc6396.md -- TABLE_DUMP_V2 RIB entries carry 4-byte AS numbers
//
// Drives ParseAttributes (attrparse.go), which is where an MRT TABLE_DUMP_V2 RIB
// entry's AS_PATH actually acquires its 4-byte encoding. The MRT side copies:
// rib_mrt.go reads the interned entry.ASPath into the record's attribute blob,
// and internal/mrt/encode.go WriteRIBEntry writes those bytes verbatim. So the
// obligation is met here, on ingest, and nowhere downstream.

package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pool "github.com/ze-software/ze/internal/component/bgp/plugins/rib/pool"
)

// TestRFC6396RIBEntryASPathStoredFourByte proves that an AS_PATH received in
// 2-byte encoding, from a session that did not negotiate 4-byte AS support, is
// stored in the RIB in 4-byte encoding. That stored value is what a
// TABLE_DUMP_V2 RIB entry carries.
//
// RFC 6396 Section 4.3.4: "All AS numbers in the AS_PATH attribute MUST be
// encoded as 4-byte AS numbers."
//
// METHOD: feed ParseAttributes a two-octet AS_SEQUENCE [65001, 65002] with
// asn4=false, which is the only case where the received encoding and the
// required encoding differ, and read back the interned AS_PATH. The segment
// header stays (type 2, count 2) and each AS widens from two octets to four.
//
// VALIDATES: the 2-byte to 4-byte widening that the RIB dump then copies.
// PREVENTS: a TABLE_DUMP_V2 file whose RIB entries carry 2-byte AS_PATHs, which
// every reader parses as 4-byte and so mis-decodes into wrong AS numbers.
//
// The MRT-side test for this clause, TestDumpV2RIBEntryASPathIs4Byte
// (internal/plugins/mrt/dump_test.go), hands the dump visitor an AS_PATH it
// built 4-byte itself and asserts it comes back 4-byte, so it measures the
// encoder's transparency rather than this widening. Replacing
// `return expandASPath2to4(aspathValue)` in canonicalizeASPath with
// `return aspathValue` leaves that test green; it turns this one red
// (mutation-tested, 2026-08-30).
//
// RFC requirement: RFC6396-4.3.4-1 positive -- every AS number reaching a
// TABLE_DUMP_V2 RIB entry's AS_PATH is 4-byte encoded, because the RIB widens a
// 2-byte AS_PATH on ingest and the dump path copies the stored bytes verbatim.
func TestRFC6396RIBEntryASPathStoredFourByte(t *testing.T) {
	// AS_PATH from a session with no 4-byte AS capability: flags 0x40, type 2,
	// length 6, then AS_SEQUENCE (type 2) of 2 ASes in two octets each.
	wireTwoByteASPath := []byte{
		0x40, 0x02, 0x06,
		0x02, 0x02, 0xFD, 0xE9, 0xFD, 0xEA,
	}
	raw := concat(wireOriginIGP, wireTwoByteASPath)

	entry, err := ParseAttributes(raw, false)
	require.NoError(t, err)
	defer entry.Release()

	require.True(t, entry.HasASPath())
	got, err := pool.ASPath.Get(entry.ASPath)
	require.NoError(t, err)

	assert.Equal(t,
		[]byte{0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0xFD, 0xEA},
		got,
		"a 2-byte AS_PATH must be stored 4-byte encoded, so the MRT RIB entry that copies it is 4-byte")
}

// TestRFC6396RIBEntryASPathFourByteSessionUnchanged is the counterpart: a
// session that DID negotiate 4-byte AS support already sends the required
// encoding, so the widening must not run twice.
//
// VALIDATES: the widening is conditional on the received encoding, not applied
// unconditionally.
// PREVENTS: a doubly widened AS_PATH, which would put an 8-octet AS in a field
// every MRT reader parses as 4.
//
// RFC requirement: RFC6396-4.3.4-1 negative -- the 4-byte requirement is met by
// widening only what arrived narrow: an AS_PATH already in 4-byte encoding is
// stored unchanged rather than widened again.
func TestRFC6396RIBEntryASPathFourByteSessionUnchanged(t *testing.T) {
	// The same path in four octets per AS: flags 0x40, type 2, length 10.
	wireFourByteASPath := []byte{
		0x40, 0x02, 0x0A,
		0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0xFD, 0xEA,
	}
	raw := concat(wireOriginIGP, wireFourByteASPath)

	entry, err := ParseAttributes(raw, true)
	require.NoError(t, err)
	defer entry.Release()

	require.True(t, entry.HasASPath())
	got, err := pool.ASPath.Get(entry.ASPath)
	require.NoError(t, err)

	assert.Equal(t,
		[]byte{0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0x00, 0x00, 0xFD, 0xEA},
		got,
		"an AS_PATH already 4-byte encoded is stored as received")
}
