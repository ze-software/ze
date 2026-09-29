package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// rfc6996AS4Path encodes one AS4_PATH value: an AS_SEQUENCE of the given ASNs.
func rfc6996AS4Path(asns ...uint32) []byte {
	path := &attribute.AS4Path{Segments: []attribute.ASPathSegment{{Type: attribute.ASSequence, ASNs: asns}}}
	buf := make([]byte, path.Len())
	path.WriteTo(buf, 0)
	return buf
}

// rfc6996Strip runs the remove-private strip directive over an UPDATE whose
// only attribute is the given AS4_PATH, and returns the ops the reactor applies
// before the route is advertised.
func rfc6996Strip(t *testing.T, as4Path []byte) *filterapi.ModAccumulator {
	t.Helper()
	attrsWire := attribute.NewAttributesWire(makeAttr(0xC0, byte(attribute.AttrAS4Path), as4Path), 0)
	var mods filterapi.ModAccumulator
	ExtractRemovePrivateASOps(newTestScratch(t), parseFilterAttrs("remove-private strip"), attrsWire, true, 65001, &mods)
	return &mods
}

// RFC requirement: RFC6996-4-1 positive -- Private Use ASNs are removed from
// AS4_PATH before the route is advertised: an AS4_PATH of [64496 4200000000
// 64512 4294967294 64497] is rewritten to [64496 64497], both ends of the
// four-octet range and the two-octet range removed.
//
// VALIDATES: RFC 6996 Section 4 "(including AS4_PATH if utilizing a four-octet AS number space)".
// PREVENTS: the reactor rewriting AS_PATH alone and leaving Private Use ASNs in AS4_PATH.
func TestRFC6996StripRemovesPrivateUseASNsFromAS4Path(t *testing.T) {
	mods := rfc6996Strip(t, rfc6996AS4Path(64496, 4200000000, 64512, 4294967294, 64497))
	require.Equal(t, 1, mods.Len())
	op := mods.Ops()[0]
	assert.Equal(t, byte(attribute.AttrAS4Path), op.Code)
	assert.Equal(t, filterapi.AttrModSet, op.Action)
	assert.Equal(t, rfc6996AS4Path(64496, 64497), op.Buf)
}

// RFC requirement: RFC6996-4-1 negative -- an AS4_PATH that holds only ASNs
// outside the Private Use ranges (64511 65535 4199999999 4294967295) draws no
// rewrite: the reactor emits no op and the attribute is advertised unchanged.
//
// VALIDATES: the ASNs adjacent to each Private Use range survive the strip.
// PREVENTS: a widened range removing ASNs RFC 6996 does not reserve.
func TestRFC6996StripKeepsAS4PathASNsOutsideThePrivateUseRanges(t *testing.T) {
	mods := rfc6996Strip(t, rfc6996AS4Path(64511, 65535, 4199999999, 4294967295))
	assert.Equal(t, 0, mods.Len())
}
