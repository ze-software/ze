//go:build linux

// Design: plan/spec-mpls-1-kernel.md -- MPLS-in-use gating for the doctor check (F15)
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

	labeled := config.NewTree()
	bgp := labeled.GetOrCreateContainer("bgp")
	peer := config.NewTree()
	peer.GetOrCreateContainer("session").GetOrCreateContainer("family").GetOrCreateContainer("ipv4/mpls-label")
	bgp.AddListEntry("peer", "p1", peer)
	assert.True(t, mplsInUse(labeled), "a BGP peer with a labeled family needs MPLS")

	grouped := config.NewTree()
	gbgp := grouped.GetOrCreateContainer("bgp")
	grp := config.NewTree()
	gpeer := config.NewTree()
	gpeer.GetOrCreateContainer("session").GetOrCreateContainer("family").GetOrCreateContainer("ipv4/mpls-vpn")
	grp.AddListEntry("peer", "p1", gpeer)
	gbgp.AddListEntry("group", "g1", grp)
	assert.True(t, mplsInUse(grouped), "a labeled family on a group peer needs MPLS")

	iface := config.NewTree()
	ifEntry := config.NewTree()
	ifEntry.GetOrCreateContainer("mpls")
	iface.AddListEntry("interface", "eth0", ifEntry)
	assert.True(t, mplsInUse(iface), "an interface with MPLS enabled needs MPLS")
}
