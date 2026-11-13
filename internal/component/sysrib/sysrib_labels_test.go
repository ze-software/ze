// Design: docs/architecture/mpls/mpls-kernel.md -- Loc-RIB label pass-through (F1) test
package sysrib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// VALIDATES: F1 -- a labeled Loc-RIB best change is converted into a BestChange
// entry that retains the MPLS label stack, so BGP labeled-unicast (SAFI 4)
// programs a kernel MPLS push entry rather than a plain IP route.
func TestChangeToBatchCarriesLabels(t *testing.T) {
	c := locrib.Change{
		Family: family.IPv4Unicast,
		Prefix: netip.MustParsePrefix("10.1.0.0/24"),
		Kind:   locrib.ChangeAdd,
		Best: locrib.Path{
			Source:  redistevents.RegisterProtocol("static"),
			NextHop: netip.MustParseAddr("10.0.0.2"),
			Labels:  []uint32{1000, 2000},
		},
	}

	fixture := newCandidateForwardingFixture(t, fixtureRouteOverride)
	fixture.rib.Insert(c.Family, c.Prefix, c.Best)
	batches := fixture.forwardSelected(map[prefixKey]struct{}{{family: c.Family, prefix: c.Prefix}: {}})
	changes := batches[c.Family]
	require.Len(t, changes, 1)
	assert.Equal(t, []uint32{1000, 2000}, changes[0].Labels,
		"labeled Loc-RIB change must carry its label stack into the BestChange entry")
}

// VALIDATES: F1 -- a withdraw carries no labels (the prefix is being removed).
func TestChangeToBatchRemoveHasNoLabels(t *testing.T) {
	fixture := newCandidateForwardingFixture(t, fixtureRouteOverride)
	prefix := netip.MustParsePrefix("10.1.0.0/24")
	_, initial := fixture.offerCandidates(makePayload("static", family.IPv4Unicast, []incomingChange{{
		Action: routeaction.Add, Prefix: prefix, NextHop: netip.MustParseAddr("10.0.0.2"), Labels: []uint32{1000, 2000},
	}}))
	require.Len(t, initial, 1)
	assert.Equal(t, []uint32{1000, 2000}, initial[0].Labels)
	_, changes := fixture.offerCandidates(makePayload("static", family.IPv4Unicast, []incomingChange{{
		Action: routeaction.Withdraw, Prefix: prefix,
	}}))
	require.Len(t, changes, 1)
	assert.Equal(t, routeaction.Withdraw, changes[0].Action)
	assert.Empty(t, changes[0].Labels)
}
