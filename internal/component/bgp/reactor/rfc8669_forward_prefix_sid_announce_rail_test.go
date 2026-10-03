// Design: forward_prefix_sid.go -- prefixSIDAllowedTo, the one site every egress
// rail asks about RFC 8669 Section 8.
// Related: reactor_api_batch.go -- buildBatchAnnounceUpdate, the rail this file
// covers.
// Related: forward_prefix_sid_test.go -- the same boundary on the two forward
// rails and the two origination rails.
//
// This file was zzprobe_prefixsid_announce_test.go, where the requirement was
// stated as a deliberately RED probe before the fix existed.
package reactor

import (
	"encoding/hex"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// prefixSIDAnnounceWire is the Prefix-SID attribute the announce carries: flags
// 0xC0 (optional transitive), code 40 (0x28), length 10, and a Label-Index TLV
// (RFC 8669 Section 3.1: type 1, length 7, RESERVED 0, Flags 0, Label Index 100).
const prefixSIDAnnounceWire = "c0280a01000700000000000064"

// VALIDATES: RFC 8669 Section 8 on the API/readvertise ANNOUNCE rail --
// buildBatchAnnounceUpdate (reactor_api_batch.go) asks prefixSIDAllowedTo
// (forward_prefix_sid.go), so attribute 40 crosses an AS boundary only toward a
// neighbor the operator has declared to be inside ze's SR domain.
// PREVENTS: the boundary holding on four rails and leaking on the fifth. The two
// forward rails and the two origination rails asked prefixSIDAllowedTo from the
// day the leaf shipped; this rail copied the caller's attribute block verbatim
// and its only removal was LOCAL_PREF, so an announce carrying a Prefix-SID
// reached an external peer with no leaf set. It carries the per-peer API
// announce, the grouped announce, and the RFC 9494 stale readvertise
// (sendStaleReadvertise), which replays a stored received block.
//
// RFC requirement: RFC8669-8-1 negative -- "The propagation to other ASes MUST be
// explicitly configured." An API announce carrying a Prefix-SID toward an
// EXTERNAL peer that sets no propagate-srv6-prefix-sid emits no attribute 40.
// RFC requirement: RFC8669-8-1 positive -- the removal is confined to that
// destination: the same announce toward an external peer the operator DID
// configure, and toward an internal peer, carries the attribute byte for byte. So
// the drop is a decision about the session rather than an unconditional strip.
func TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain(t *testing.T) {
	// ORIGIN igp, AS_PATH [65001], then the Prefix-SID above.
	// No LOCAL_PREF: RFC 4271 Section 5.1.5 forbids one toward an external peer,
	// and the sibling strip would remove it, which would confuse the reading.
	packed, err := hex.DecodeString(strings.ReplaceAll(
		"400101 00 4002 06 02010000fde9 "+prefixSIDAnnounceWire, " ", ""))
	require.NoError(t, err)

	route := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.0/24"), 0)
	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}}}

	// build runs the announce rail toward one destination. isIBGP and
	// propagatePrefixSID are the two inputs prefixSIDAllowedTo reads, and nothing
	// else about the destination changes between the cases below.
	build := func(isIBGP, propagatePrefixSID bool) []byte {
		update, buildErr := adapter.buildBatchAnnounceUpdate(
			make([]byte, message.MaxMsgLen), make([]byte, message.MaxMsgLen),
			bgptypes.NLRIBatch{
				Family:  family.IPv4Unicast,
				NLRIs:   []nlri.NLRI{route},
				NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
				Wire:    attribute.NewAttributesWire(packed, bgpctx.APIContextID),
			},
			announceFacts{
				nextHop:            netip.MustParseAddr("10.0.0.1"),
				isIBGP:             isIBGP,
				asn4:               true,
				prepend:            localASOnly(65000),
				propagatePrefixSID: propagatePrefixSID,
			})
		require.NoError(t, buildErr)
		require.NotNil(t, update)
		return update.PathAttributes
	}

	t.Run("external-peer-with-no-leaf-is-sent-no-prefix-sid", func(t *testing.T) {
		attrs := build(false /*external*/, false /*leaf unset*/)

		_, _, _, found := attribute.AttrFind(attrs, attribute.AttrPrefixSID)
		assert.False(t, found,
			"RFC 8669 Section 8: \"The propagation to other ASes MUST be explicitly configured.\" "+
				"No leaf was set, so attribute 40 must not be in %x", attrs)

		// The absence is specific: every other attribute of the announce arrives,
		// so a whole dropped UPDATE fails this line rather than passing it.
		assert.Equal(t, []int{1, 2, 3}, attrCodes(t, attrs))
		assertAscending(t, attrCodes(t, attrs))
	})

	t.Run("external-peer-the-operator-configured-keeps-it", func(t *testing.T) {
		configured := build(false /*external*/, true /*propagate-srv6-prefix-sid*/)
		stripped := build(false /*external*/, false /*leaf unset*/)

		_, _, value, found := attribute.AttrFind(configured, attribute.AttrPrefixSID)
		require.True(t, found, "an explicitly configured neighbor must still receive attribute 40")
		assert.Equal(t, "01000700000000000064", hex.EncodeToString(value),
			"the Label-Index TLV crosses the boundary unchanged, not re-encoded")

		// Code 40 sorts last here, so the configured frame is the stripped frame
		// plus the attribute. One equality states both that the removal took only
		// attribute 40 and that it took all of it.
		assert.Equal(t, hex.EncodeToString(stripped)+prefixSIDAnnounceWire, hex.EncodeToString(configured),
			"the two destinations differ in the Prefix-SID and in nothing else")
	})

	t.Run("internal-peer-keeps-it-whatever-the-leaf-says", func(t *testing.T) {
		// Section 8 governs propagation "to other ASes". An internal peer shares
		// this AS, so the leaf does not reach the decision and both spellings of it
		// must produce the same frame.
		withLeaf := build(true /*internal*/, true)
		withoutLeaf := build(true /*internal*/, false)

		_, _, value, found := attribute.AttrFind(withoutLeaf, attribute.AttrPrefixSID)
		require.True(t, found, "an internal peer is inside the SR domain by construction")
		assert.Equal(t, "01000700000000000064", hex.EncodeToString(value))
		assert.Equal(t, hex.EncodeToString(withLeaf), hex.EncodeToString(withoutLeaf),
			"the leaf is an eBGP question; it must not change what an internal peer is sent")
	})
}
