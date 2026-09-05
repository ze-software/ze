//go:build linux

// Design: docs/architecture/mpls/mpls-kernel.md -- MPLS-in-use gating for the kernel capability
package doctor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
)

// kernelFIB adds the `fib { kernel { } }` block every in-use tree needs. The
// backend gate comes FIRST in the predicate (AC-7), so a tree without it is
// never in use whatever else it configures.
func kernelFIB(tree *config.Tree) *config.Tree {
	tree.GetOrCreateContainer("fib").GetOrCreateContainer("kernel")
	return tree
}

// VALIDATES: F15 and AC-7 -- the MPLS kernel capability is asked about only when
// MPLS forwarding is actually configured (a labeled BGP family, LDP, RSVP-TE or
// an interface MPLS block) on the KERNEL FIB backend, never for a plain
// BGP-over-kernel config and never for another backend.
// PREVENTS: over-reporting (R-3). This predicate now decides whether ze starts,
// so a config it wrongly reports as using MPLS is a router that will not boot.
func TestMPLSInUse(t *testing.T) {
	assert.False(t, kernelcap.MPLSInUse(config.NewTree()), "empty config uses no MPLS")
	assert.False(t, kernelcap.MPLSInUse(nil), "a nil tree uses no MPLS")

	plain := kernelFIB(config.NewTree())
	plain.GetOrCreateContainer("bgp")
	assert.False(t, kernelcap.MPLSInUse(plain), "plain BGP (no labeled family) uses no MPLS")

	ldp := kernelFIB(config.NewTree())
	ldp.GetOrCreateContainer("ldp")
	assert.True(t, kernelcap.MPLSInUse(ldp), "LDP needs MPLS")

	rsvp := kernelFIB(config.NewTree())
	rsvp.GetOrCreateContainer("rsvp-te")
	assert.True(t, kernelcap.MPLSInUse(rsvp), "RSVP-TE needs MPLS")

	// `family` is a LIST keyed by the family name -- `family ipv4/mpls-label { }`
	// parses to a list ENTRY, never to a container called "family". These two
	// cases used GetOrCreateContainer("family").GetOrCreateContainer(<fam>), which
	// mirrored the production bug instead of the parser: the predicate read
	// GetContainer("family"), so both this test and the code agreed on a shape no
	// config ever has. The test passed, the predicate was dead on every real
	// config, and test/plugin/mpls-doctor.ci -- the .ci that would have caught it
	// -- is Linux-only and had never run. Build it the way the parser does.
	labeled := kernelFIB(config.NewTree())
	bgp := labeled.GetOrCreateContainer("bgp")
	peer := config.NewTree()
	labeledFam := config.NewTree()
	peer.GetOrCreateContainer("session").AddListEntry("family", "ipv4/mpls-label", labeledFam)
	bgp.AddListEntry("peer", "p1", peer)
	assert.True(t, kernelcap.MPLSInUse(labeled), "a BGP peer with a labeled family needs MPLS")

	grouped := kernelFIB(config.NewTree())
	gbgp := grouped.GetOrCreateContainer("bgp")
	grp := config.NewTree()
	gpeer := config.NewTree()
	vpnFam := config.NewTree()
	gpeer.GetOrCreateContainer("session").AddListEntry("family", "ipv4/mpls-vpn", vpnFam)
	grp.AddListEntry("peer", "p1", gpeer)
	gbgp.AddListEntry("group", "g1", grp)
	assert.True(t, kernelcap.MPLSInUse(grouped), "a labeled family on a group peer needs MPLS")

	// A family declared ONCE on the group and on none of its peers. This is the
	// idiomatic shape (`list group { uses peer-fields; }` in ze-bgp-conf.yang;
	// ResolveBGPTree merges it into every member), used by 26 configs in this
	// repo -- and the predicate could not see it: it only ever looked at the
	// group's PEERS, so a whole peer-group negotiating labeled unicast produced
	// no MPLS report at all.
	groupOnly := kernelFIB(config.NewTree())
	gobgp := groupOnly.GetOrCreateContainer("bgp")
	ggrp := config.NewTree()
	ggrp.GetOrCreateContainer("session").AddListEntry("family", "ipv4/mpls-label", config.NewTree())
	ggrp.AddListEntry("peer", "p1", config.NewTree())
	gobgp.AddListEntry("group", "g1", ggrp)
	assert.True(t, kernelcap.MPLSInUse(groupOnly), "a labeled family on the GROUP needs MPLS")

	// A peer whose only family is unlabeled must NOT count. Without this the two
	// assertions above would still pass if the peer walk returned true for any
	// peer carrying any family at all.
	unlabeled := kernelFIB(config.NewTree())
	ubgp := unlabeled.GetOrCreateContainer("bgp")
	upeer := config.NewTree()
	upeer.GetOrCreateContainer("session").AddListEntry("family", "ipv4/unicast", config.NewTree())
	ubgp.AddListEntry("peer", "p1", upeer)
	assert.False(t, kernelcap.MPLSInUse(unlabeled), "a plain unicast peer needs no MPLS")

	iface := kernelFIB(config.NewTree())
	ifEntry := config.NewTree()
	ifEntry.GetOrCreateContainer("mpls")
	iface.AddListEntry("interface", "eth0", ifEntry)
	assert.True(t, kernelcap.MPLSInUse(iface), "an interface with MPLS enabled needs MPLS")
}
