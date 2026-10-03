// VALIDATES: a received route whose NLRI the Adj-RIB-In does not hold is placed in it, and
// the placement is keyed on NLRI identity: a second route with the same NLRI takes the
// first one's slot rather than a second one.
// PREVENTS: a store that drops a new prefix, and one that grows a duplicate entry for a
// prefix it already holds.

package adj_rib_in

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// rfc4271Announce builds an UPDATE announcing one IPv4 prefix with the three well-known
// mandatory attributes. nextHop is the last octet of NEXT_HOP 10.0.0.x, so two calls can
// carry the same NLRI with different attributes.
func rfc4271Announce(nextHop byte, nlri ...byte) []byte {
	body := []byte{
		0x00, 0x00, // Withdrawn Routes length 0
		0x00, 0x0e, // Total Path Attribute Length 14
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, nextHop, // NEXT_HOP = 10.0.0.<nextHop>
	}
	return append(body, nlri...)
}

// TestRFC4271AdjRIBInPlacesNewRoute feeds the Adj-RIB-In through handleReceivedStructured,
// the entry point a reactor UPDATE event takes, and reads the ribIn map directly.
//
// RFC requirement: RFC4271-9-4 positive — a route whose NLRI is not in the Adj-RIB-In is
// placed in it: 10.0.0.0/8 then 10.1.0.0/16 leave two entries, and the first is found
// under its own key with the attributes it arrived with.
// RFC requirement: RFC4271-9-4 negative — a route whose NLRI IS in the Adj-RIB-In is not
// placed beside it: 10.0.0.0/8 re-announced with another NEXT_HOP leaves two entries, and
// the key now holds the newer NEXT_HOP. The placement is about the NLRI, not the message.
func TestRFC4271AdjRIBInPlacesNewRoute(t *testing.T) {
	r := newTestManager(t)
	peer := netip.MustParseAddr("192.0.2.1")
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

	feed := func(body []byte) {
		wu := wireu.NewWireUpdate(body, ctxID)
		attrs, _ := wu.Attrs()
		r.handleReceivedStructured(&rpc.StructuredEvent{
			EventType:   rpc.EventKindUpdate,
			PeerAddress: peer.String(),
			RawMessage: &bgptypes.RawMessage{
				Type:       msgtype.TypeUPDATE,
				WireUpdate: wu,
				AttrsWire:  attrs,
			},
		})
	}

	slash8 := routeKeyFromStrings(family.IPv4Unicast, "10.0.0.0/8", 0)

	feed(rfc4271Announce(1, 0x08, 0x0a)) // 10.0.0.0/8 via 10.0.0.1
	require.Equal(t, 1, r.ribIn[peer].Len(), "9-4 positive: the first NLRI is placed")

	feed(rfc4271Announce(1, 0x10, 0x0a, 0x01)) // 10.1.0.0/16 via 10.0.0.1
	require.Equal(t, 2, r.ribIn[peer].Len(), "9-4 positive: a second, distinct NLRI is placed beside the first")
	first, ok := r.ribIn[peer].Get(slash8)
	require.True(t, ok, "9-4 positive: 10.0.0.0/8 is found under its own key")
	assert.Equal(t, "0a000001", first.NHopHex, "the stored route carries the NEXT_HOP it arrived with")

	feed(rfc4271Announce(2, 0x08, 0x0a)) // 10.0.0.0/8 again, via 10.0.0.2
	assert.Equal(t, 2, r.ribIn[peer].Len(), "9-4 negative: an identical NLRI adds no entry")
	replaced, ok := r.ribIn[peer].Get(slash8)
	require.True(t, ok)
	assert.Equal(t, "0a000002", replaced.NHopHex, "the identical NLRI took the existing slot")
}
