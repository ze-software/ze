package message

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Splitter.splitUpdateWithMP used to emit only `&Update{PathAttributes: ...}` chunks, with
// no WithdrawnRoutes and no NLRI. An oversized UPDATE carrying BOTH an MP attribute and the
// legacy IPv4 fields therefore lost the IPv4 half silently: no error, no log, the routes
// simply never reached the peer. Reached from peer_send.go (sendUpdateWithSplit) and
// reactor/forward_body.go.

// mixedMPUpdate returns an UPDATE carrying IPv4 withdrawn routes, IPv4 NLRI, ORIGIN, and an
// MP_REACH for IPv6 -- large enough that Split must chunk it.
func mixedMPUpdate(t *testing.T) *Update {
	t.Helper()

	var withdrawn, nlri []byte
	for i := range 40 {
		withdrawn = append(withdrawn, 0x18, 0x0a, byte(i), 0x00) // 10.i.0.0/24
		nlri = append(nlri, 0x18, 0xc0, 0x00, byte(i))           // 192.0.i.0/24
	}

	mpReachValue := []byte{
		0x00, 0x02, 0x01, 0x10, // AFI IPv6, SAFI unicast, next-hop length 16
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x00, 0x01,
		0x00, // reserved
	}
	for i := range 40 {
		mpReachValue = append(mpReachValue,
			0x40, 0x20, 0x01, 0x0d, 0xb8, 0x01, byte(i), 0x00, 0x00) // 2001:db8:1:i::/64
	}
	mpReach := append([]byte{0x90, 0x0e,
		byte(len(mpReachValue) >> 8), byte(len(mpReachValue))}, mpReachValue...)

	attrs := []byte{0x40, 0x01, 0x01, 0x00} // ORIGIN = IGP
	attrs = append(attrs, mpReach...)

	return &Update{WithdrawnRoutes: withdrawn, PathAttributes: attrs, NLRI: nlri}
}

// VALIDATES: splitting an UPDATE that mixes an MP attribute with legacy IPv4 fields
// preserves every field.
// PREVENTS: the silent data loss above. A dropped withdrawal leaves a stale route
// installed on the peer; a dropped NLRI leaves a route unreachable. Neither is visible
// anywhere: the split reports success.
func TestSplitUpdateWithMPPreservesIPv4Fields(t *testing.T) {
	u := mixedMPUpdate(t)

	var s Splitter
	var withdrawnOut, nlriOut []byte
	mpReachChunks := 0
	err := s.Split(u, 200, true, func(chunk *Update) error {
		withdrawnOut = append(withdrawnOut, chunk.WithdrawnRoutes...)
		nlriOut = append(nlriOut, chunk.NLRI...)
		if findAttr(chunk.PathAttributes, 14) {
			mpReachChunks++
		}
		return nil
	})
	require.NoError(t, err)

	assert.Equal(t, u.WithdrawnRoutes, withdrawnOut,
		"every withdrawn route must survive the split")
	assert.Equal(t, u.NLRI, nlriOut, "every announced route must survive the split")
	assert.Positive(t, mpReachChunks, "the MP_REACH must still be emitted")
}

// VALIDATES: each emitted chunk carries at most one NLRI-bearing field, and withdrawals
// come before announcements.
// PREVENTS: fixing the data loss by simply bolting the IPv4 fields onto an MP chunk, which
// would reintroduce the RFC 7606 Section 5.1 violation the wireu splitter was just changed
// to avoid, and could reorder a withdrawal after its announcement.
func TestSplitUpdateWithMPEmitsOneFieldPerChunk(t *testing.T) {
	u := mixedMPUpdate(t)

	var s Splitter
	seenAnnounce := false
	var order []string
	err := s.Split(u, 200, true, func(chunk *Update) error {
		var present []string
		if len(chunk.WithdrawnRoutes) > 0 {
			present = append(present, "withdrawn")
		}
		if len(chunk.NLRI) > 0 {
			present = append(present, "nlri")
		}
		if findAttr(chunk.PathAttributes, 14) {
			present = append(present, "mp-reach")
		}
		if findAttr(chunk.PathAttributes, 15) {
			present = append(present, "mp-unreach")
		}
		require.LessOrEqualf(t, len(present), 1,
			"chunk carries %v; RFC 7606 Section 5.1 allows at most one NLRI-bearing field",
			present)
		if len(present) == 1 {
			order = append(order, present[0])
			switch present[0] {
			case "withdrawn", "mp-unreach":
				assert.Falsef(t, seenAnnounce,
					"withdrawal %q emitted after an announcement; order was %v",
					present[0], order)
			case "nlri", "mp-reach":
				seenAnnounce = true
			}
		}
		return nil
	})
	require.NoError(t, err)
	require.Contains(t, order, "withdrawn")
	require.Contains(t, order, "nlri")
}

// findAttr reports whether pathAttrs contains an attribute of the given type code.
func findAttr(pathAttrs []byte, code byte) bool {
	for pos := 0; pos+2 <= len(pathAttrs); {
		flags, typeCode := pathAttrs[pos], pathAttrs[pos+1]
		pos += 2
		var l int
		if flags&0x10 != 0 {
			if pos+2 > len(pathAttrs) {
				return false
			}
			l = int(pathAttrs[pos])<<8 | int(pathAttrs[pos+1])
			pos += 2
		} else {
			if pos+1 > len(pathAttrs) {
				return false
			}
			l = int(pathAttrs[pos])
			pos++
		}
		if typeCode == code {
			return true
		}
		pos += l
	}
	return false
}
