// VALIDATES: the RFC 7999 Section 3.3 exemption reads the communities the
// SESSION agreed to, not one constant.
// PREVENTS: a peer that runs RTBH on its own community (65001:666, the common
// case) setting blackhole-exempt and getting nothing, because the exemption was
// looking for 65535:666 on a session that never uses it.

package rpki

import (
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
)

const testPeer = "198.51.100.1"

// agreementPlugin builds a plugin whose one peer agreed to the listed
// communities and asked for the exemption.
func agreementPlugin(communities ...attribute.Community) *rPKIPlugin {
	rp := &rPKIPlugin{}
	peers := map[configjson.PeerConfigKey]peerActionSet{
		{ID: testPeer}: {
			BlackholeExempt:      true,
			BlackholeCommunities: communities,
		},
	}
	rp.perPeerActions.Store(&peers)
	return rp
}

// communityAttrs wraps a COMMUNITIES attribute carrying one 4-octet value.
func communityAttrs(value [4]byte) *attribute.AttributesWire {
	raw := []byte{0xC0, 0x08, 0x04, value[0], value[1], value[2], value[3]}
	return attribute.NewAttributesWire(raw, bgpctx.APIContextID)
}

var (
	wellKnownOnWire = [4]byte{0xFF, 0xFF, 0x02, 0x9A} // 65535:666
	ownOnWire       = [4]byte{0xFD, 0xE9, 0x02, 0x9A} // 65001:666
)

// The case the constant could not express. A session that agreed to its own
// community, and a route carrying it, is a legitimate blackhole announcement.
func TestCarriesAgreedBlackholeReadsTheSessionsOwnCommunity(t *testing.T) {
	rp := agreementPlugin(attribute.Community(65001<<16 | 666))

	if !rp.carriesAgreedBlackhole(testPeer, "", communityAttrs(ownOnWire)) {
		t.Error("a session that agreed to 65001:666 got no exemption for a route carrying it")
	}
}

// The other half. A session that agreed to its own community has not agreed to
// the well-known one, so a route carrying 65535:666 is not its blackhole.
func TestCarriesAgreedBlackholeIgnoresACommunityTheSessionDidNotName(t *testing.T) {
	rp := agreementPlugin(attribute.Community(65001<<16 | 666))

	if rp.carriesAgreedBlackhole(testPeer, "", communityAttrs(wellKnownOnWire)) {
		t.Error("the exemption fired on a community this session never agreed to")
	}
}

// The well-known value still works, under either spelling of the agreement.
func TestCarriesAgreedBlackholeReadsTheWellKnownCommunity(t *testing.T) {
	rp := agreementPlugin(attribute.CommunityBlackhole)

	if !rp.carriesAgreedBlackhole(testPeer, "", communityAttrs(wellKnownOnWire)) {
		t.Error("a session that agreed to BLACKHOLE got no exemption for a route carrying it")
	}
}

// A session that named no community agreed to nothing, so RFC 7999 Section 3.3's
// agreement condition fails and there is no legitimate blackhole to protect.
func TestCarriesAgreedBlackholeIsClosedWithoutAnAgreement(t *testing.T) {
	rp := agreementPlugin()

	if rp.carriesAgreedBlackhole(testPeer, "", communityAttrs(wellKnownOnWire)) {
		t.Error("the exemption fired on a session that agreed to no community at all")
	}
}

// A peer with no per-peer entry is every peer in a deployment that does not use
// the feature.
func TestCarriesAgreedBlackholeIsClosedForAnUnlistedPeer(t *testing.T) {
	rp := agreementPlugin(attribute.CommunityBlackhole)

	if rp.carriesAgreedBlackhole("203.0.113.9", "", communityAttrs(wellKnownOnWire)) {
		t.Error("the exemption reached a peer with no per-peer policy")
	}
}

// A session a listen-range group accepted appears nowhere in the config document,
// so RFC 7999 Section 3.3's "that particular BGP session" is stated on the group.
// Its members read the agreement under the group's name, and a session in no
// group reads nothing from it.
func TestCarriesAgreedBlackholeResolvesThroughTheGroup(t *testing.T) {
	rp := &rPKIPlugin{}
	m := map[configjson.PeerConfigKey]peerActionSet{
		configjson.GroupKey("ix"): {
			BlackholeExempt:      true,
			BlackholeCommunities: []attribute.Community{attribute.Community(65001<<16 | 666)},
		},
	}
	rp.perPeerActions.Store(&m)

	if !rp.carriesAgreedBlackhole("192.0.2.50", "ix", communityAttrs(ownOnWire)) {
		t.Error("a member of the group got no exemption for the community the group agreed to")
	}
	if rp.carriesAgreedBlackhole("192.0.2.50", "", communityAttrs(ownOnWire)) {
		t.Error("the exemption fired for a session that belongs to no group")
	}
}

// Nil attributes, and a plugin that has seen no config, both answer false. The
// exemption stays closed on input it cannot read.
func TestCarriesAgreedBlackholeFailsClosed(t *testing.T) {
	if agreementPlugin(attribute.CommunityBlackhole).carriesAgreedBlackhole(testPeer, "", nil) {
		t.Error("nil attributes produced an exemption")
	}
	if (&rPKIPlugin{}).carriesAgreedBlackhole(testPeer, "", communityAttrs(wellKnownOnWire)) {
		t.Error("a plugin with no per-peer config produced an exemption")
	}
}

// The agreement travels from the config subtree into the resolved per-peer set,
// so the two leaves an operator writes on one session end up in one record.
func TestParsePeerActionsCarriesTheAgreement(t *testing.T) {
	// The bgp-level rpki container is what makes the plugin read per-peer
	// overrides at all: without a cache server nothing is ever Invalid, so there
	// is nothing for the exemption to act on.
	cfg, err := parseRPKIConfig(`{"bgp":{
		"rpki":{"action":{"invalid":"reject"}},
		"peer":{"p1":{
		"connection":{"remote":{"ip":"198.51.100.1"}},
		"rpki":{"blackhole-exempt":"true"},
		"blackhole":{"communities":["65001:666"]}}}}}`)
	if err != nil {
		t.Fatalf("parseRPKIConfig: %v", err)
	}

	set, ok := cfg.PeerActions[configjson.PeerConfigKey{ID: testPeer}]
	if !ok {
		t.Fatalf("peer absent from the resolved action map: %v", cfg.PeerActions)
	}
	if !set.BlackholeExempt {
		t.Error("blackhole-exempt did not survive the walk")
	}
	if len(set.BlackholeCommunities) != 1 || set.BlackholeCommunities[0] != attribute.Community(65001<<16|666) {
		t.Errorf("BlackholeCommunities = %v, want [65001:666]", set.BlackholeCommunities)
	}
}

// A blackhole block the parser cannot act on refuses the config rather than
// resolving to a silently empty agreement.
func TestParseRPKIConfigRefusesAnUnreadableAgreement(t *testing.T) {
	_, err := parseRPKIConfig(`{"bgp":{
		"rpki":{"action":{"invalid":"reject"}},
		"peer":{"p1":{
		"connection":{"remote":{"ip":"198.51.100.1"}},
		"rpki":{"blackhole-exempt":"true"},
		"blackhole":{"communities":["not-a-community"]}}}}}`)
	if err == nil {
		t.Error("an unparseable blackhole community was accepted: the operator would read an agreement that exempts nothing")
	}
}
