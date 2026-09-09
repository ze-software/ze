// Design: docs/architecture/wire/attributes.md -- UPDATE payload fixtures for the AS-path family tests
//
// Shared payload builders and one AS_PATH reader, used by aspath_transcode_test.go,
// rfc6793_as4_test.go and tombstone_test.go.
//
// This file held the tests for RewriteASPath and RewriteASPathDual until
// 2026-09-09, when Thomas retired the whole-payload rewrite and its
// draft-mangin-idr-attr-tombstone-00 Section 5.3 support. Every behavioral test
// moved to aspath_slot_test.go, against ASPathEdit.Record, which is the rail an
// EBGP prepend really takes; test/weakened/49b0956f.md maps each one. The
// builders stayed because three other files read them, and a test file is not
// deleted without the owner.

package wireu

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"

	"github.com/stretchr/testify/require"
)

// buildPayload constructs an UPDATE payload from parts.
// UPDATE body: wdLen(2) + withdrawn(wdLen) + attrLen(2) + attrs(attrLen) + nlri.
func buildPayload(withdrawn, attrs, nlri []byte) []byte {
	payload := make([]byte, 2+len(withdrawn)+2+len(attrs)+len(nlri))
	binary.BigEndian.PutUint16(payload[0:2], uint16(len(withdrawn))) //nolint:gosec // test data
	copy(payload[2:], withdrawn)
	off := 2 + len(withdrawn)
	binary.BigEndian.PutUint16(payload[off:off+2], uint16(len(attrs))) //nolint:gosec // test data
	copy(payload[off+2:], attrs)
	copy(payload[off+2+len(attrs):], nlri)
	return payload
}

// buildASPathAttr constructs an AS_PATH attribute with given segments using ASN4 encoding.
// Each segment is (type, []ASN). Returns the complete attribute (header + value).
func buildASPathAttr(segments []attribute.ASPathSegment, asn4 bool) []byte { //nolint:unparam // asn4 is always true in current tests but parameter needed for correctness
	path := &attribute.ASPath{Segments: segments}
	valueLen := path.LenWithASN4(asn4)
	// Header: flags(1) + code(1) + length(1 or 2)
	hdrLen := 3
	if valueLen > 255 {
		hdrLen = 4
	}
	buf := make([]byte, hdrLen+valueLen)
	attribute.WriteHeaderTo(buf, 0, attribute.FlagTransitive, attribute.AttrASPath, uint16(valueLen)) //nolint:gosec // test data
	path.WriteToWithASN4(buf, hdrLen, asn4)
	return buf
}

// buildOriginAttr constructs a simple ORIGIN attribute (value=0 IGP).
func buildOriginAttr() []byte {
	// Flags=0x40 (transitive), Code=1 (ORIGIN), Len=1, Value=0 (IGP)
	return []byte{0x40, 0x01, 0x01, 0x00}
}

// concatAttrs concatenates attribute byte slices into a single attrs section.
func concatAttrs(parts ...[]byte) []byte {
	size := 0
	for _, p := range parts {
		size += len(p)
	}
	buf := make([]byte, 0, size)
	for _, p := range parts {
		buf = append(buf, p...)
	}
	return buf
}

// parseASPathFromPayload extracts and parses the AS_PATH from a rewritten payload.
func parseASPathFromPayload(t *testing.T, payload []byte) *attribute.ASPath {
	t.Helper()
	require.True(t, len(payload) >= 4, "payload too short")

	wdLen := int(binary.BigEndian.Uint16(payload[0:2]))
	attrLenOff := 2 + wdLen
	require.True(t, len(payload) >= attrLenOff+2, "payload too short for attrLen")

	attrLen := int(binary.BigEndian.Uint16(payload[attrLenOff : attrLenOff+2]))
	attrsStart := attrLenOff + 2
	require.True(t, len(payload) >= attrsStart+attrLen, "payload too short for attrs")

	// Scan attrs to find AS_PATH
	off := attrsStart
	for off < attrsStart+attrLen {
		flags, code, length, hl, err := attribute.ParseHeader(payload[off:])
		require.NoError(t, err, "parse attr header")
		_ = flags
		if code == attribute.AttrASPath {
			value := payload[off+hl : off+hl+int(length)]
			path, err := attribute.ParseASPath(value, false)
			require.NoError(t, err, "parse AS_PATH value")
			return path
		}
		off += hl + int(length)
	}
	t.Fatal("AS_PATH not found in payload")
	return nil
}
