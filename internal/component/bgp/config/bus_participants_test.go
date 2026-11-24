// Design: docs/architecture/api/architecture.md -- tests for BGP secondary /bgp/ip/<address> participant resolution
// Related: bus_participants.go -- resolveBGPSecondaryIPPath

package bgpconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

func buildBGPParticipantsTree() *zeconfig.Tree {
	root := zeconfig.NewTree()
	bgp := zeconfig.NewTree()

	// Standalone peer edge1 at 192.0.2.1.
	edge1 := zeconfig.NewTree()
	edge1Conn := zeconfig.NewTree()
	edge1Rem := zeconfig.NewTree()
	edge1Rem.Set("ip", "192.0.2.1")
	edge1Conn.SetContainer("remote", edge1Rem)
	edge1.SetContainer("connection", edge1Conn)
	bgp.AddListEntry("peer", "edge1", edge1)

	// Static group transit with peer t1 at 192.0.2.2.
	transit := zeconfig.NewTree()
	t1 := zeconfig.NewTree()
	t1Conn := zeconfig.NewTree()
	t1Rem := zeconfig.NewTree()
	t1Rem.Set("ip", "192.0.2.2")
	t1Conn.SetContainer("remote", t1Rem)
	t1.SetContainer("connection", t1Conn)
	transit.AddListEntry("peer", "t1", t1)
	bgp.AddListEntry("group", "transit", transit)

	// Dynamic group clients accepting 198.51.100.0/24.
	clients := zeconfig.NewTree()
	clientsConn := zeconfig.NewTree()
	clientsRem := zeconfig.NewTree()
	clientsRem.Set("ip", "dynamic")
	clientsRem.SetSlice("range", []string{"198.51.100.0/24"})
	clientsConn.SetContainer("remote", clientsRem)
	clients.SetContainer("connection", clientsConn)
	bgp.AddListEntry("group", "clients", clients)

	root.SetContainer("bgp", bgp)
	return root
}

// TestResolveBGPSecondaryIPPath verifies M-3 and M-18 rules for /bgp/ip/<addr>.
//
// VALIDATES: Configured static standalone and grouped peers resolve with
// dormant=false; an unconfigured address inside a dynamic group range resolves
// to /bgp/group/<g> with dormant=true; an address outside all peers and ranges
// is refused naming configured IPs and dynamic ranges.
// PREVENTS: Unconfigured out-of-range /bgp/ip/ addresses silently accepted or
// valid dynamic-group addresses wrongly refused.
func TestResolveBGPSecondaryIPPath(t *testing.T) {
	tree := buildBGPParticipantsTree()

	// Standalone static peer.
	paths, dormant, _, err := resolveBGPSecondaryIPPath(tree, "/bgp/ip/192.0.2.1")
	require.NoError(t, err)
	assert.False(t, dormant)
	assert.Equal(t, []string{"/bgp/peer/edge1"}, paths)

	// Grouped static peer.
	paths, dormant, _, err = resolveBGPSecondaryIPPath(tree, "/bgp/ip/192.0.2.2")
	require.NoError(t, err)
	assert.False(t, dormant)
	assert.Equal(t, []string{"/bgp/group/transit/peer/t1"}, paths)

	// Unconfigured address inside dynamic group range -> dormant=true (M-18).
	paths, dormant, _, err = resolveBGPSecondaryIPPath(tree, "/bgp/ip/198.51.100.7")
	require.NoError(t, err)
	assert.True(t, dormant)
	assert.Equal(t, []string{"/bgp/group/clients"}, paths)

	// Unconfigured address outside every peer and dynamic range -> refused (M-18).
	_, _, validDesc, err := resolveBGPSecondaryIPPath(tree, "/bgp/ip/203.0.113.99")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "192.0.2.1")
	assert.Contains(t, err.Error(), "192.0.2.2")
	assert.Contains(t, err.Error(), "198.51.100.0/24")
	assert.Contains(t, validDesc, "198.51.100.0/24")

	// Malformed IP or CIDR suffix refused.
	_, _, _, err = resolveBGPSecondaryIPPath(tree, "/bgp/ip/not-an-ip")
	require.Error(t, err)
	_, _, _, err = resolveBGPSecondaryIPPath(tree, "/bgp/ip/198.51.100.0/24")
	require.Error(t, err)
}
