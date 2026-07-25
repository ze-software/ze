package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/core/selector"

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

// matchPeers is a test helper: parses a selector string and resolves matching peers.
func matchPeers(adapter *reactorAPIAdapter, s string) []*Peer {
	sel := selector.ParseDefault(s)
	adapter.r.mu.RLock()
	defer adapter.r.mu.RUnlock()
	return adapter.getMatchingPeersSel(sel)
}

// TestPeerSelectorByName verifies that a peer can be resolved by its Name field.
//
// VALIDATES: getMatchingPeersSel returns the peer whose settings.Name matches the selector.
// PREVENTS: Name-based peer selection silently returning empty results.
func TestPeerSelectorByName(t *testing.T) {
	adapter := setupSelectorReactor()

	peers := matchPeers(adapter, "upstream")
	require.Len(t, peers, 1, "should match exactly one peer by name")
	assert.Equal(t, "upstream", peers[0].settings.Name)
	assert.Equal(t, mustParseAddr("10.0.0.1"), peers[0].settings.Address)
}

// TestPeerSelectorByIP verifies that a peer can be resolved by bare IP address.
//
// VALIDATES: getMatchingPeersSel returns the peer whose Address matches a bare IP selector.
// PREVENTS: Bare IP selectors failing when peers have names configured.
func TestPeerSelectorByIP(t *testing.T) {
	adapter := setupSelectorReactor()

	peers := matchPeers(adapter, "10.0.0.1")
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

	byName := matchPeers(adapter, "downstream")
	require.Len(t, byName, 1)

	byIP := matchPeers(adapter, "10.0.0.2")
	require.Len(t, byIP, 1)

	assert.Equal(t, byName[0], byIP[0], "name and IP should resolve to the same peer object")
	assert.Equal(t, "downstream", byName[0].settings.Name)
	assert.Equal(t, mustParseAddr("10.0.0.2"), byIP[0].settings.Address)
}

// TestPeerSelectorWildcard verifies that "*" matches all peers.
//
// VALIDATES: getMatchingPeersSel with KindAll returns every peer in the reactor.
// PREVENTS: Wildcard selector missing peers or returning empty.
func TestPeerSelectorWildcard(t *testing.T) {
	adapter := setupSelectorReactor()

	peers := matchPeers(adapter, "*")
	assert.Len(t, peers, 3, "wildcard should match all 3 peers")
}

// TestPeerSelectorGlob verifies that glob patterns match by IP octets.
//
// VALIDATES: getMatchingPeersSel with KindGlob matches peers in that subnet.
// PREVENTS: Glob patterns failing to match or matching too broadly.
func TestPeerSelectorGlob(t *testing.T) {
	adapter := setupSelectorReactor()

	// "10.0.0.*" should match 10.0.0.1 and 10.0.0.2, but not 10.0.1.1
	peers := matchPeers(adapter, "10.0.0.*")
	assert.Len(t, peers, 2, "glob 10.0.0.* should match 2 peers")

	addrs := make(map[string]bool)
	for _, p := range peers {
		addrs[p.settings.Address.String()] = true
	}
	assert.True(t, addrs["10.0.0.1"], "should include 10.0.0.1")
	assert.True(t, addrs["10.0.0.2"], "should include 10.0.0.2")
	assert.False(t, addrs["10.0.1.1"], "should NOT include 10.0.1.1")
}

// TestPeerSelectorExclusion verifies that "!name" returns all peers except the named one.
//
// VALIDATES: getMatchingPeersSel with exclude flag excludes the upstream peer.
// PREVENTS: Exclusion selector including the excluded peer or returning empty.
func TestPeerSelectorExclusion(t *testing.T) {
	adapter := setupSelectorReactor()

	peers := matchPeers(adapter, "!upstream")
	assert.Len(t, peers, 2, "exclusion should return all peers except upstream")

	for _, p := range peers {
		assert.NotEqual(t, "upstream", p.settings.Name, "upstream should be excluded")
	}
}

// TestPeerSelectorNoMatch verifies that an unknown selector returns empty.
//
// VALIDATES: getMatchingPeersSel with a non-matching selector returns nil/empty.
// PREVENTS: Unknown selectors matching random peers.
func TestPeerSelectorNoMatch(t *testing.T) {
	adapter := setupSelectorReactor()

	peers := matchPeers(adapter, "nonexistent")
	assert.Empty(t, peers, "unknown selector should return empty")
}

// TestPeerSelectorByASN verifies that a peer can be resolved by ASN selector.
//
// VALIDATES: AC-1 -- unique ASN selects exactly one peer.
// PREVENTS: ASN selectors silently returning empty or wrong peers.
func TestPeerSelectorByASN(t *testing.T) {
	adapter := setupSelectorReactor()

	peers := matchPeers(adapter, "as65001")
	require.Len(t, peers, 1, "should match exactly one peer by ASN")
	assert.Equal(t, "upstream", peers[0].settings.Name)
	assert.Equal(t, uint32(65001), peers[0].settings.PeerAS)
}

// TestPeerSelectorByASNMultiple verifies that shared ASN selects all matching peers.
//
// VALIDATES: AC-2 -- multiple peers with same ASN are all returned.
// PREVENTS: ASN selector only returning first match instead of all.
func TestPeerSelectorByASNMultiple(t *testing.T) {
	r := New(&Config{})

	// Two peers with same ASN (iBGP mesh)
	s1 := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65000, 0x01010101)
	s1.Name = "ibgp-a"
	r.peers[s1.PeerKey()] = NewPeer(s1)

	s2 := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65000, 0x02020202)
	s2.Name = "ibgp-b"
	r.peers[s2.PeerKey()] = NewPeer(s2)

	// One peer with different ASN
	s3 := NewPeerSettings(mustParseAddr("10.0.1.1"), 65000, 65001, 0x03030303)
	s3.Name = "ebgp"
	r.peers[s3.PeerKey()] = NewPeer(s3)

	adapter := &reactorAPIAdapter{r: r}

	peers := matchPeers(adapter, "as65000")
	assert.Len(t, peers, 2, "should match both iBGP peers with same ASN")

	names := make(map[string]bool)
	for _, p := range peers {
		names[p.settings.Name] = true
	}
	assert.True(t, names["ibgp-a"], "should include ibgp-a")
	assert.True(t, names["ibgp-b"], "should include ibgp-b")
	assert.False(t, names["ebgp"], "should NOT include ebgp")
}

// TestPeerSelectorByASNNoMatch verifies that unknown ASN returns empty.
//
// VALIDATES: AC-3 -- non-existent ASN returns empty result.
// PREVENTS: ASN selector matching wrong peers when no peer has that ASN.
func TestPeerSelectorByASNNoMatch(t *testing.T) {
	adapter := setupSelectorReactor()

	peers := matchPeers(adapter, "as99999")
	assert.Empty(t, peers, "unknown ASN should return empty")
}

// TestPeerSelectorASNExclusion verifies that "!as<N>" excludes ASN-matched peers.
//
// VALIDATES: AC-4 -- exclusion with ASN selector returns all peers except matching.
// PREVENTS: Exclusion prefix not working with ASN selectors.
func TestPeerSelectorASNExclusion(t *testing.T) {
	adapter := setupSelectorReactor()

	peers := matchPeers(adapter, "!as65001")
	assert.Len(t, peers, 2, "exclusion should return all peers except AS 65001")

	for _, p := range peers {
		assert.NotEqual(t, uint32(65001), p.settings.PeerAS, "AS 65001 should be excluded")
	}
}

// TestPeerSelectorASNNameCollision verifies that a peer named "as65001" is
// resolved by name (not ASN) because name matching has priority over ASN.
//
// VALIDATES: Name match runs before ASN match in ParseDefault -> matchPositive.
// PREVENTS: Peer with ASN-like name being resolved as ASN selector.
func TestPeerSelectorASNNameCollision(t *testing.T) {
	r := New(&Config{})

	// Peer named "as65001" with PeerAS=65002 (different from the name's number)
	s1 := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65002, 0x01010101)
	s1.Name = "as65001"
	r.peers[s1.PeerKey()] = NewPeer(s1)

	// Another peer with PeerAS=65001 (matches the ASN in the selector)
	s2 := NewPeerSettings(mustParseAddr("10.0.0.2"), 65000, 65001, 0x02020202)
	s2.Name = "other"
	r.peers[s2.PeerKey()] = NewPeer(s2)

	adapter := &reactorAPIAdapter{r: r}

	// "as65001" should match by name (peer 10.0.0.1), not by ASN (peer 10.0.0.2)
	peers := matchPeers(adapter, "as65001")
	require.Len(t, peers, 1, "should match exactly one peer by name, not ASN")
	assert.Equal(t, "as65001", peers[0].settings.Name)
	assert.Equal(t, uint32(65002), peers[0].settings.PeerAS, "should be the named peer, not the ASN-matched one")
}

func TestPeerSelectorNamePriority(t *testing.T) {
	r := New(&Config{})

	// Create a peer with a name that could also look like a pattern
	s1 := NewPeerSettings(mustParseAddr("10.0.0.1"), 65000, 65001, 0x01010101)
	s1.Name = "router-a"
	r.peers[s1.PeerKey()] = NewPeer(s1)

	adapter := &reactorAPIAdapter{r: r}

	peers := matchPeers(adapter, "router-a")
	require.Len(t, peers, 1)
	assert.Equal(t, "router-a", peers[0].settings.Name)
}
