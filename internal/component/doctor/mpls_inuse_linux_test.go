//go:build linux

// Design: docs/architecture/mpls/mpls-kernel.md -- MPLS-in-use gating for the doctor check (F15)
package doctor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/config"
)

// VALIDATES: F15 -- the MPLS-module warning only fires when MPLS forwarding is
// actually configured (labeled BGP family, LDP, RSVP-TE, or iface MPLS), not for
// any plain BGP-over-kernel config.
func TestMPLSInUse(t *testing.T) {
	assert.False(t, mplsInUse(config.NewTree()), "empty config uses no MPLS")

	plain := config.NewTree()
	plain.GetOrCreateContainer("bgp")
	assert.False(t, mplsInUse(plain), "plain BGP (no labeled family) uses no MPLS")

	ldp := config.NewTree()
	ldp.GetOrCreateContainer("ldp")
	assert.True(t, mplsInUse(ldp), "LDP needs MPLS")

	rsvp := config.NewTree()
	rsvp.GetOrCreateContainer("rsvp-te")
	assert.True(t, mplsInUse(rsvp), "RSVP-TE needs MPLS")

	// `family` is a LIST keyed by the family name -- `family ipv4/mpls-label { }`
	// parses to a list ENTRY, never to a container called "family". These two
	// cases used GetOrCreateContainer("family").GetOrCreateContainer(<fam>), which
	// mirrored the production bug instead of the parser: containerPeersLabeled
	// read GetContainer("family"), so both this test and the code agreed on a
	// shape no config ever has. The test passed, mplsInUse was dead on every real
	// config, and test/plugin/mpls-doctor.ci -- the .ci that would have caught it
	// -- is Linux-only and had never run. Build it the way the parser does.
	labeled := config.NewTree()
	bgp := labeled.GetOrCreateContainer("bgp")
	peer := config.NewTree()
	labeledFam := config.NewTree()
	peer.GetOrCreateContainer("session").AddListEntry("family", "ipv4/mpls-label", labeledFam)
	bgp.AddListEntry("peer", "p1", peer)
	assert.True(t, mplsInUse(labeled), "a BGP peer with a labeled family needs MPLS")

	grouped := config.NewTree()
	gbgp := grouped.GetOrCreateContainer("bgp")
	grp := config.NewTree()
	gpeer := config.NewTree()
	vpnFam := config.NewTree()
	gpeer.GetOrCreateContainer("session").AddListEntry("family", "ipv4/mpls-vpn", vpnFam)
	grp.AddListEntry("peer", "p1", gpeer)
	gbgp.AddListEntry("group", "g1", grp)
	assert.True(t, mplsInUse(grouped), "a labeled family on a group peer needs MPLS")

	// A family declared ONCE on the group and on none of its peers. This is the
	// idiomatic shape (`list group { uses peer-fields; }` in ze-bgp-conf.yang;
	// ResolveBGPTree merges it into every member), used by 26 configs in this
	// repo -- and mplsInUse could not see it: it only ever looked at the group's
	// PEERS, so a whole peer-group negotiating labeled unicast produced no MPLS
	// module warning at all.
	groupOnly := config.NewTree()
	gobgp := groupOnly.GetOrCreateContainer("bgp")
	ggrp := config.NewTree()
	ggrp.GetOrCreateContainer("session").AddListEntry("family", "ipv4/mpls-label", config.NewTree())
	ggrp.AddListEntry("peer", "p1", config.NewTree())
	gobgp.AddListEntry("group", "g1", ggrp)
	assert.True(t, mplsInUse(groupOnly), "a labeled family on the GROUP needs MPLS")

	// A peer whose only family is unlabeled must NOT count. Without this the two
	// assertions above would still pass if containerPeersLabeled returned true
	// for any peer carrying any family at all.
	unlabeled := config.NewTree()
	ubgp := unlabeled.GetOrCreateContainer("bgp")
	upeer := config.NewTree()
	upeer.GetOrCreateContainer("session").AddListEntry("family", "ipv4/unicast", config.NewTree())
	ubgp.AddListEntry("peer", "p1", upeer)
	assert.False(t, mplsInUse(unlabeled), "a plain unicast peer needs no MPLS")

	iface := config.NewTree()
	ifEntry := config.NewTree()
	ifEntry.GetOrCreateContainer("mpls")
	iface.AddListEntry("interface", "eth0", ifEntry)
	assert.True(t, mplsInUse(iface), "an interface with MPLS enabled needs MPLS")
}
