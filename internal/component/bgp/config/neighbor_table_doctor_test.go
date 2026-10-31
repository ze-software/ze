package bgpconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/network"
)

// replaceNeighborTableRead swaps the check's table read for read until the
// test ends.
func replaceNeighborTableRead(t *testing.T, read func() ([]network.Neighbor, error)) {
	t.Helper()
	saved := readNeighborTable
	readNeighborTable = read
	t.Cleanup(func() { readNeighborTable = saved })
}

// TestDoctorNeighbourTableUnreadable proves `ze doctor` reports a kernel
// neighbor table BGP cannot read, with the fix for the cause it found, before
// the daemon starts: RFC 2545 Section 3's third-party Link-Local comes only
// from that table, and without it every such next hop goes out as the global
// alone. A readable table and a config with no bgp block report nothing.
//
// MUTATION: return nil from checkBGPNeighborTable on a read error: the
// unreadable table is reported as ready.
func TestDoctorNeighbourTableUnreadable(t *testing.T) {
	tree := strictDoctorTree(t, strictPeerConfig)

	for _, tc := range []struct {
		name string
		err  error
		fix  string
	}{
		{"no-reader", network.ErrNeighborTableNotRegistered, "interface component"},
		{"no-backend", errors.New("no interface backend is loaded: not supported on this platform"), "netlink interface backend"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			replaceNeighborTableRead(t, func() ([]network.Neighbor, error) { return nil, tc.err })
			found := checkBGPNeighborTable(diagnosticContext(tree))
			require.Len(t, found, 1)
			assert.Equal(t, codeBGPNeighborTable, found[0].Code)
			assert.Equal(t, diagnostic.SeverityWarning, found[0].Severity)
			assert.True(t, strings.Contains(found[0].Message, tc.err.Error()), "the cause is named: %s", found[0].Message)
			assert.Contains(t, found[0].Message, tc.fix, "the fix for that cause is named")
		})
	}

	t.Run("readable", func(t *testing.T) {
		replaceNeighborTableRead(t, func() ([]network.Neighbor, error) { return []network.Neighbor{}, nil })
		assert.Empty(t, checkBGPNeighborTable(diagnosticContext(tree)))
	})

	t.Run("no-bgp", func(t *testing.T) {
		replaceNeighborTableRead(t, func() ([]network.Neighbor, error) { return nil, network.ErrNeighborTableNotRegistered })
		assert.Empty(t, checkBGPNeighborTable(diagnosticContext(config.NewTree())))
	})
}

// TestDoctorNeighbourTableCheckRegistered proves the check reaches `ze doctor`
// through the registry, and its code resolves for `ze explain`.
func TestDoctorNeighbourTableCheckRegistered(t *testing.T) {
	found := false
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
		if check.Name == neighborTableDoctorCheck.Name {
			found = true
		}
	}
	assert.True(t, found, "bgp-neighbor-table is registered for the post-config phase")
	assert.NotNil(t, diagnostic.Lookup(codeBGPNeighborTable), "the code resolves")
}
