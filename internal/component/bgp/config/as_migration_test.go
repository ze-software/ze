package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/reactor"
	"github.com/ze-software/ze/internal/component/config"
)

// TestPeersFromConfigTree_ASMigrationPerNeighborGroup drives the RFC 7705 Section 4.2
// leaf through group resolution, which is where the section's configurability sentence is
// satisfied or is not.
//
// RFC 7705 Section 4.2: "The mechanism introduced in this section MUST be configurable on
// a per-neighbor or per-neighbor-group basis to allow for maximum flexibility." Both
// halves are asserted in ONE tree, because a group setting that reaches every peer
// regardless of what the peer says satisfies the group half and breaks the neighbor half.
//
// The group states the migration AS beside the local AS. One peer inherits both. The
// second peer states a migration AS of its own and MUST end up with the one it named
// rather than the group's. The third peer is outside the group and MUST carry neither, so
// a group leaf cannot leak to a peer that never inherited it.
//
// VALIDATES: AC-8, RFC7705-4.2-1, and AC-7's configured half: each migrating peer resolves
// as iBGP whichever of the two ASNs its remote-as names.
// No `RFC requirement:` tag is carried: the summary is parked at rfc/pending/rfc7705.md,
// so the id is unknown to `./le rfc check` (internal/le/rfc/check_core.go) until enrolment.
// PREVENTS: the leaf being peer-only, a group value overwriting a peer's own choice, and a
// group value reaching a peer outside the group.
func TestPeersFromConfigTree_ASMigrationPerNeighborGroup(t *testing.T) {
	// The retained ASN and the ASN being retired, from the RFC 5398 documentation range.
	const retainedAS = "64500"
	const legacyAS = "64510"
	const otherLegacyAS = "64520"

	tree := config.NewTree()
	bgp := buildBGPBlock()

	group := config.NewTree()
	groupSession := config.NewTree()
	groupASN := config.NewTree()
	groupASN.Set("local", retainedAS)
	groupASN.Set("migration", legacyAS)
	groupSession.SetContainer("asn", groupASN)
	group.SetContainer("session", groupSession)

	// Inherits the group's local AS and migration AS. Its remote-as is the retained ASN,
	// so this is the peer that has already been renumbered.
	group.AddListEntry("peer", "inherits", buildMinimalPeer("10.0.0.1", retainedAS, "auto"))

	// States a migration AS of its own, and a remote-as equal to it: this is the peer
	// still running under a legacy ASN, and a different legacy ASN from the group's.
	overrides := buildMinimalPeer("10.0.0.2", otherLegacyAS, "auto")
	overrideASN := config.NewTree()
	overrideASN.Set("remote", otherLegacyAS)
	overrideASN.Set("migration", otherLegacyAS)
	overrideSession := config.NewTree()
	overrideSession.SetContainer("asn", overrideASN)
	overrides.SetContainer("session", overrideSession)
	group.AddListEntry("peer", "overrides", overrides)

	bgp.AddListEntry("group", "migrating", group)
	bgp.AddListEntry("peer", "outside", buildMinimalPeer("10.0.0.3", "65003", "auto"))
	tree.SetContainer("bgp", bgp)

	peers, err := PeersFromConfigTree(tree)
	require.NoError(t, err)
	require.Len(t, peers, 3)

	byAddr := make(map[string]*reactor.PeerSettings, len(peers))
	for _, ps := range peers {
		byAddr[ps.Address.String()] = ps
	}

	inherits := byAddr["10.0.0.1"]
	require.NotNil(t, inherits, "the inheriting peer must be present")
	assert.Equal(t, uint32(64500), inherits.LocalAS, "the group's local-as reaches the peer")
	assert.Equal(t, uint32(64510), inherits.MigrationAS, "the group's migration AS reaches the peer")
	assert.True(t, inherits.IsIBGP(),
		"a peer whose remote-as is the retained ASN is internal, as it was before the mechanism")

	overridden := byAddr["10.0.0.2"]
	require.NotNil(t, overridden, "the overriding peer must be present")
	assert.Equal(t, uint32(64500), overridden.LocalAS, "the group's local-as still reaches this peer")
	assert.Equal(t, uint32(64520), overridden.MigrationAS,
		"a peer that states migration REPLACES the group's value; taking the group's would put this peer on an ASN nobody configured for it")
	assert.True(t, overridden.IsIBGP(),
		"RFC 7705 Section 4.2: a session under the locally configured ASN is native iBGP, which the equality LocalAS == PeerAS alone calls external")

	outside := byAddr["10.0.0.3"]
	require.NotNil(t, outside, "the peer outside the group must be present")
	assert.Equal(t, uint32(65000), outside.LocalAS, "no group, so the globally configured AS number stands")
	assert.Zero(t, outside.MigrationAS, "a group leaf must not reach a peer outside the group")
	assert.True(t, outside.IsEBGP(), "a peer with no migration configuration is unchanged")
}

// TestPeersFromConfigTree_ASMigrationRefusesAThirdRemoteAS holds the config-load refusal.
//
// VALIDATES: a remote-as that is neither the local AS nor the migration AS stops the load,
// naming all three numbers. RFC 7705 Section 4.2 is about iBGP throughout, so a third AS
// describes an external peer and the mechanism cannot mean what the section says on it.
// PREVENTS: the config being accepted and the error surfacing later as an OPEN this
// speaker refuses, with no line of configuration for an operator to look at.
func TestPeersFromConfigTree_ASMigrationRefusesAThirdRemoteAS(t *testing.T) {
	tree := config.NewTree()
	bgp := buildBGPBlock()

	peer := buildMinimalPeer("10.0.0.1", "64496", "auto")
	asn := config.NewTree()
	asn.Set("local", "64500")
	asn.Set("remote", "64496")
	asn.Set("migration", "64510")
	session := config.NewTree()
	session.SetContainer("asn", asn)
	peer.SetContainer("session", session)

	bgp.AddListEntry("peer", "third-as", peer)
	tree.SetContainer("bgp", bgp)

	_, err := PeersFromConfigTree(tree)

	require.Error(t, err, "a remote-as outside the migrating pair must stop the load")
	assert.ErrorIs(t, err, reactor.ErrMigrationASNotPeerAS)
}
