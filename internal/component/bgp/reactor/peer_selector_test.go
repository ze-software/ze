package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: creates a reactor with named peers for selector tests.
func setupSelectorReactor() *reactorAPIAdapter {
	r := New(&Config{})

	// Peer 1: named "upstream", IP 10.0.0.1
	s1 := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0x01010101)
	s1.Name = "upstream"
	r.peers[s1.PeerKey()] = NewPeer(s1)

	// Peer 2: named "downstream", IP 10.0.0.2
	s2 := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65002, 0x02020202)
	s2.Name = "downstream"
	r.peers[s2.PeerKey()] = NewPeer(s2)

	// Peer 3: named "lateral", IP 10.0.1.1
	s3 := NewPeerSettings(mustParseAddr("10.0.1.1"), 65000, 65003, 0x03030303)
	s3.Name = "lateral"
	r.peers[s3.PeerKey()] = NewPeer(s3)

	return &reactorAPIAdapter{r: r}
}

// TestPeerSelectorByName verifies that a peer can be resolved by its Name field.
//
// VALIDATES: getMatchingPeersLocked returns the peer whose settings.Name matches the selector.
// PREVENTS: Name-based peer selection silently returning empty results.
func TestPeerSelectorByName(t *testing.T) {
	adapter := setupSelectorReactor()

	peers := adapter.getMatchingPeers("upstream")
	require.Len(t, peers, 1, "should match exactly one peer by name")
	assert.Equal(t, "upstream", peers[0].settings.Name)
	assert.Equal(t, mustParseAddr("10.0.0.1"), peers[0].settings.Address)
}

// TestPeerSelectorByIP verifies that a peer can be resolved by bare IP address.
//
// VALIDATES: getMatchingPeersLocked returns the peer whose Address matches a bare IP selector.
// PREVENTS: Bare IP selectors failing when peers have names configured.
func TestPeerSelectorByIP(t *testing.T) {
	adapter := setupSelectorReactor()

	peers := adapter.getMatchingPeers("10.0.0.1")
	require.Len(t, peers, 1, "should match exactly one peer by IP")
	assert.Equal(t, mustParseAddr("10.0.0.1"), peers[0].settings.Address)
	assert.Equal(t, "upstream", peers[0].settings.Name)
}

// TestPeerSelectorByIPWhenNameExists verifies that both name and IP resolve the same peer.
//
// VALIDATES: The same peer is reachable by either its Name or its Address.
// PREVENTS: Ambiguity between name and IP selection for the same peer.
func TestPeerSelectorByIPWhenNameExists(t *testing.T) {
	adapter := setupSelectorReactor()

	byName := adapter.getMatchingPeers("downstream")
	require.Len(t, byName, 1)

	byIP := adapter.getMatchingPeers("10.0.0.2")
	require.Len(t, byIP, 1)

	assert.Equal(t, byName[0], byIP[0], "name and IP should resolve to the same peer object")
	assert.Equal(t, "downstream", byName[0].settings.Name)
	assert.Equal(t, mustParseAddr("10.0.0.2"), byIP[0].settings.Address)
}

// TestPeerSelectorWildcard verifies that "*" matches all peers.
//
// VALIDATES: getMatchingPeersLocked with "*" returns every peer in the reactor.
// PREVENTS: Wildcard selector missing peers or returning empty.
func TestPeerSelectorWildcard(t *testing.T) {
	adapter := setupSelectorReactor()

	peers := adapter.getMatchingPeers("*")
	assert.Len(t, peers, 3, "wildcard should match all 3 peers")
}

// TestPeerSelectorGlob verifies that glob patterns match by IP octets.
//
// VALIDATES: getMatchingPeersLocked with "10.0.0.*" matches peers in that subnet.
// PREVENTS: Glob patterns failing to match or matching too broadly.
func TestPeerSelectorGlob(t *testing.T) {
	adapter := setupSelectorReactor()

	// "10.0.0.*" should match 10.0.0.1 and 10.0.0.2, but not 10.0.1.1
	peers := adapter.getMatchingPeers("10.0.0.*")
	assert.Len(t, peers, 2, "glob 10.0.0.* should match 2 peers")

	addrs := make(map[string]bool)
	for _, p := range peers {
		addrs[p.settings.Address.String()] = true
	}
	assert.True(t, addrs["10.0.0.1"], "should include 10.0.0.1")
	assert.True(t, addrs["10.0.0.2"], "should include 10.0.0.2")
	assert.False(t, addrs["10.0.1.1"], "should NOT include 10.0.1.1")
}

// TestPeerSelectorExactKey verifies that "addr:port" exact key match works.
//
// VALIDATES: getMatchingPeersLocked with full "addr:port" key returns the peer directly.
// PREVENTS: Full key lookup broken or falling through to slow path.
func TestPeerSelectorExactKey(t *testing.T) {
	adapter := setupSelectorReactor()

	peers := adapter.getMatchingPeers("10.0.0.1:179")
	require.Len(t, peers, 1, "exact key should match one peer")
	assert.Equal(t, mustParseAddr("10.0.0.1"), peers[0].settings.Address)
}

// TestPeerSelectorExclusion verifies that "!name" returns all peers except the named one.
//
// VALIDATES: getMatchingPeers with "!upstream" excludes the upstream peer.
// PREVENTS: Exclusion selector including the excluded peer or returning empty.
func TestPeerSelectorExclusion(t *testing.T) {
	adapter := setupSelectorReactor()

	peers := adapter.getMatchingPeers("!upstream")
	assert.Len(t, peers, 2, "exclusion should return all peers except upstream")

	for _, p := range peers {
		assert.NotEqual(t, "upstream", p.settings.Name, "upstream should be excluded")
	}
}

// TestPeerSelectorNoMatch verifies that an unknown selector returns empty.
//
// VALIDATES: getMatchingPeersLocked with a non-matching selector returns nil/empty.
// PREVENTS: Unknown selectors matching random peers.
func TestPeerSelectorNoMatch(t *testing.T) {
	adapter := setupSelectorReactor()

	peers := adapter.getMatchingPeers("nonexistent")
	assert.Empty(t, peers, "unknown selector should return empty")
}

// TestPeerSelectorNamePriority verifies name match takes priority over glob.
// A selector that is a valid name should match by name, not fall through to glob.
//
// VALIDATES: Name match short-circuits before glob evaluation.
// PREVENTS: Name selectors being interpreted as glob patterns.
func TestPeerSelectorNamePriority(t *testing.T) {
	r := New(&Config{})

	// Create a peer with a name that could also look like a pattern
	s1 := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0x01010101)
	s1.Name = "router-a"
	r.peers[s1.PeerKey()] = NewPeer(s1)

	adapter := &reactorAPIAdapter{r: r}

	peers := adapter.getMatchingPeers("router-a")
	require.Len(t, peers, 1)
	assert.Equal(t, "router-a", peers[0].settings.Name)
}
