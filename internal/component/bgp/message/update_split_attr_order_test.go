package message

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// Attribute type-code ordering across a split.
//
// The splitter used to copy every base attribute into scratch and then write the
// per-chunk MP attribute AFTER all of them, so an UPDATE whose attributes arrived
// in ascending order (RFC 4271 Section 5) came back out with MP_REACH_NLRI (14)
// behind EXTENDED_COMMUNITIES (16). The same announce below the message-size limit
// took the Split fast path and was emitted untouched, so one route encoded two
// different ways depending only on how many NLRIs it carried.

// splitAttrCodes returns the attribute type codes of a packed block, in wire order.
func splitAttrCodes(t *testing.T, b []byte) []int {
	t.Helper()
	var out []int
	iter := attribute.NewAttrIterator(b)
	for {
		code, _, _, ok := iter.Next()
		if !ok {
			break
		}
		out = append(out, int(code))
	}
	require.Equal(t, len(b), iter.Offset(), "attribute block did not walk cleanly to its end")
	return out
}

// orderedMPUpdate builds an UPDATE whose attributes are in ascending type-code
// order and whose MP_REACH_NLRI is large enough to force a split: ORIGIN (1),
// AS_PATH (2), LOCAL_PREF (5), MP_REACH_NLRI (14), EXTENDED_COMMUNITIES (16).
func orderedMPUpdate(t *testing.T, nlriLen int) *Update {
	t.Helper()
	buf := make([]byte, wireExtendedMax)
	off := 0
	off += attribute.WriteAttrTo(attribute.Origin(0), buf, off)
	off += attribute.WriteAttrTo(&attribute.ASPath{}, buf, off)
	off += attribute.WriteAttrTo(attribute.LocalPref(100), buf, off)

	// IPv6 unicast NLRI: repeated /32 prefixes, 5 bytes each on the wire.
	routes := make([]byte, 0, nlriLen)
	for i := 0; len(routes)+5 <= nlriLen; i++ {
		routes = append(routes, 32, 0x20, 0x01, byte(i>>8), byte(i)) //nolint:gosec // G115: test fixture
	}
	mp := attribute.NewMPReachNLRI(2, 1, []netip.Addr{netip.MustParseAddr("2001:db8::1")}, routes)
	off += attribute.WriteAttrTo(mp, buf, off)

	var ec attribute.ExtendedCommunity
	copy(ec[:], []byte{0x80, 0x06, 0x00, 0x00, 0x46, 0x16, 0x00, 0x00})
	off += attribute.WriteAttrTo(attribute.ExtendedCommunities{ec}, buf, off)

	u := &Update{PathAttributes: buf[:off]}
	require.Equal(t, []int{1, 2, 5, 14, 16}, splitAttrCodes(t, u.PathAttributes),
		"fixture must start in ascending type-code order")
	return u
}

const wireExtendedMax = 65535

// TestSplitMP_PreservesAscendingAttributeOrder pins the ordering of every emitted
// chunk.
//
// RFC requirement: RFC4271-5-7 positive -- "The sender of an UPDATE message SHOULD
// order path attributes within the UPDATE message in ascending order of attribute
// type" (RFC 4271 Section 5), across a message split.
// VALIDATES: each split chunk carries MP_REACH_NLRI (14) between LOCAL_PREF (5)
// and EXTENDED_COMMUNITIES (16), and every base attribute survives the split.
// PREVENTS: regressing to appending the MP attribute after the whole base block,
// which emitted 1,2,5,16,14 and made a split announce encode differently from the
// same announce that fitted in one message.
func TestSplitMP_PreservesAscendingAttributeOrder(t *testing.T) {
	u := orderedMPUpdate(t, 5000)

	s := GetSplitter()
	defer PutSplitter(s)

	chunks := 0
	err := s.Split(u, 4096, false, func(c *Update) error {
		chunks++
		assert.Equal(t, []int{1, 2, 5, 14, 16}, splitAttrCodes(t, c.PathAttributes),
			"chunk %d attribute order", chunks)
		return nil
	})
	require.NoError(t, err)
	require.Greater(t, chunks, 1, "fixture must actually split")
}

// TestSplitMP_ChunkMatchesUnsplitEncoding is the invariant behind the ordering:
// splitting must not change HOW a route is encoded, only how much of it goes in
// each message. A one-NLRI announce takes Split's fast path and is emitted
// untouched; a many-NLRI announce of the same shape must produce the same
// attribute sequence.
//
// VALIDATES: the attribute codes of a split chunk equal those of the same UPDATE
// emitted unsplit.
// PREVENTS: a size-dependent encoding, which no hex-pinned test can express and no
// reviewer would think to look for.
func TestSplitMP_ChunkMatchesUnsplitEncoding(t *testing.T) {
	s := GetSplitter()
	defer PutSplitter(s)

	var unsplit []int
	small := orderedMPUpdate(t, 5)
	require.NoError(t, s.Split(small, 4096, false, func(c *Update) error {
		unsplit = splitAttrCodes(t, c.PathAttributes)
		return nil
	}))

	large := orderedMPUpdate(t, 5000)
	seen := 0
	require.NoError(t, s.Split(large, 4096, false, func(c *Update) error {
		seen++
		assert.Equal(t, unsplit, splitAttrCodes(t, c.PathAttributes),
			"split chunk %d must encode its attributes exactly like the unsplit UPDATE", seen)
		return nil
	}))
	require.Greater(t, seen, 1, "fixture must actually split")
}

// TestSplitMP_HighAttrsIdenticalInEveryChunk guards the stash: the higher-coded
// base attributes are copied into every chunk from one parked copy, so a chunk
// that clobbered the stash would show up as a corrupted or missing attribute in a
// LATER chunk, not the first.
//
// VALIDATES: the EXTENDED_COMMUNITIES value is byte-identical in every chunk.
// PREVENTS: the per-chunk MP region growing over the stash it reads from.
func TestSplitMP_HighAttrsIdenticalInEveryChunk(t *testing.T) {
	u := orderedMPUpdate(t, 20000)

	s := GetSplitter()
	defer PutSplitter(s)

	want := []byte{0x80, 0x06, 0x00, 0x00, 0x46, 0x16, 0x00, 0x00}
	chunks := 0
	err := s.Split(u, 4096, false, func(c *Update) error {
		chunks++
		iter := attribute.NewAttrIterator(c.PathAttributes)
		found := false
		for {
			code, _, value, ok := iter.Next()
			if !ok {
				break
			}
			if code == attribute.AttrExtCommunity {
				found = true
				assert.Equal(t, want, value, "chunk %d extended community", chunks)
			}
		}
		assert.True(t, found, "chunk %d lost its extended community", chunks)
		return nil
	})
	require.NoError(t, err)
	require.Greater(t, chunks, 4, "fixture must produce several chunks")
}
