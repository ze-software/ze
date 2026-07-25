// Design: plan/spec-mpls-1-kernel.md -- Loc-RIB label pass-through (F1) test
package sysrib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
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
			NextHop: netip.MustParseAddr("10.0.0.2"),
			Labels:  []uint32{1000, 2000},
		},
	}

	batch := changeToBatch(c)
	require.NotNil(t, batch)
	require.Len(t, batch.Changes, 1)
	assert.Equal(t, []uint32{1000, 2000}, batch.Changes[0].Labels,
		"labeled Loc-RIB change must carry its label stack into the BestChange entry")
}

// VALIDATES: F1 -- a withdraw carries no labels (the prefix is being removed).
func TestChangeToBatchRemoveHasNoLabels(t *testing.T) {
	batch := changeToBatch(locrib.Change{
		Family: family.IPv4Unicast,
		Prefix: netip.MustParsePrefix("10.1.0.0/24"),
		Kind:   locrib.ChangeRemove,
	})
	require.NotNil(t, batch)
	require.Len(t, batch.Changes, 1)
	assert.Empty(t, batch.Changes[0].Labels)
}
