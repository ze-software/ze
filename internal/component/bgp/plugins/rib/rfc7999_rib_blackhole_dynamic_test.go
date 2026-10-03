// VALIDATES: the RIB learns which peer group a session belongs to, on both
// event rails, and the RFC 7999 honoring decision resolves a listen-range
// group's rule through it.
// PREVENTS: an IXP route server's `blackhole` block reaching none of its
// members. A session created from a dynamic group has no address in the
// operator's document, so the group name is the only identity the document and
// the session share -- and a decision that never receives it silently forwards
// traffic the operator asked to discard.

package rib

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/rib/routetype"
)

// The rail the default in-process deployment uses. The decision cases in
// rib_blackhole_wiring_test.go plant the group on the session; this one
// RECEIVES it, so the identity comes from the event the reactor really sends
// (bgp/server's getStructuredEvent fills PeerGroup from PeerInfo.GroupName).
//
// RFC requirement: RFC7999-3.3-1 positive -- the announced prefix is covered by
// an equal or shorter prefix the neighboring network is authorized to
// advertise, through the authorization the session's group stated.
// RFC requirement: RFC7999-3.3-2 positive -- the receiving party agreed to
// honor BLACKHOLE on that particular BGP session, through the community the
// session's group stated.
func TestBlackholeGroupIdentityArrivesOnAStructuredEvent(t *testing.T) {
	member := netip.MustParseAddr("192.0.2.9")
	r := newTestRIBManagerWithBus(newTestEventBus())
	loc := locrib.NewRIB()
	r.SetLocRIB(loc)
	rules := map[configjson.PeerConfigKey]blackholeConfig{
		configjson.GroupKey("ix"): {
			communities: agreedBlackhole,
			authorized:  []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")},
		},
	}
	r.blackholeCfg.Store(&rules)
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

	// One UPDATE announcing 10.0.0.1/32 tagged 65535:666: withdrawn length 0,
	// 21 octets of attributes (ORIGIN, empty AS_PATH, NEXT_HOP, COMMUNITIES),
	// then the NLRI.
	feedReceivedFromGroup(r, member, "ix", ctxID, []byte{
		0x00, 0x00, // Withdrawn Routes length 0
		0x00, 0x15, // Total Path Attribute Length 21
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
		0xC0, 0x08, 0x04, 0xFF, 0xFF, 0x02, 0x9A, // COMMUNITIES = 65535:666
		0x20, 0x0a, 0x00, 0x00, 0x01, // NLRI 10.0.0.1/32
	})

	assert.Equal(t, "ix", r.peerGroupName(member),
		"the group on the received event was not recorded for the session")
	assert.Equal(t, routetype.Blackhole,
		blackholeLocRIBType(t, loc, netip.MustParsePrefix("10.0.0.1/32")),
		"the group on the received event did not reach the honoring decision")
}

// The same identity on the JSON rail, which a RIB running as its own process
// reads instead. Both rails carry the group, so the deployment shape does not
// decide whether an IXP member's agreement is honored.
func TestBlackholeGroupIdentityArrivesOnAJSONEvent(t *testing.T) {
	member := netip.MustParseAddr("192.0.2.9")
	r := newTestRIBManagerWithBus(newTestEventBus())
	peerJSON, err := json.Marshal(map[string]any{
		"name":   "dyn-" + member.String(),
		"group":  "ix",
		"remote": map[string]any{"address": member.String(), "as": 65001},
	})
	require.NoError(t, err)

	r.peerMu.Lock()
	r.updatePeerMetadata(&Event{Peer: peerJSON}, member)
	r.peerMu.Unlock()

	assert.Equal(t, "ix", r.peerGroupName(member),
		"the group on a JSON event was not recorded for the session")
}

// A session that belongs to no group records none, so the honoring decision
// gets an empty group rather than a stale one. The map lookup for an empty
// group is what configjson.LookupPeerConfig skips.
func TestBlackholeGroupIdentityIsEmptyForAStandalonePeer(t *testing.T) {
	peer := netip.MustParseAddr("192.0.2.1")
	r := newTestRIBManagerWithBus(newTestEventBus())
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))

	feedReceived(r, peer, ctxID, []byte{
		0x00, 0x00, // Withdrawn Routes length 0
		0x00, 0x0e, // Total Path Attribute Length 14
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
		0x20, 0x0a, 0x00, 0x00, 0x01, // NLRI 10.0.0.1/32
	})

	assert.Empty(t, r.peerGroupName(peer))
}
