package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RFC requirement: RFC6286-2.1-1 negative -- a peer-level router-id of
// 0.0.0.1, the smallest non-zero BGP Identifier, is accepted at config parse
// and becomes the peer's identifier: RFC 6286 Section 2.1 refuses only zero.
//
// VALIDATES: parseRouterID refuses zero alone, so the non-zero clause of
// RFC 6286 Section 2.1 rejects nothing the RFC allows.
// PREVENTS: A non-zero check widened past zero refusing a legal identifier.
func TestRFC6286NonZeroRouterIDAccepted(t *testing.T) {
	tree := map[string]any{
		"connection": map[string]any{"remote": map[string]any{"ip": "10.0.0.1"}, "local": map[string]any{"ip": "auto"}},
		"session":    map[string]any{"asn": map[string]any{"remote": "65001"}, "router-id": "0.0.0.1"},
	}

	ps, err := parsePeerFromTree("peer1", tree, 65000, 0x0a000001)
	require.NoError(t, err)
	assert.Equal(t, uint32(1), ps.RouterID)
}

// TestRFC6286GlobalZeroRouterIDRejected parses a bgp tree whose global
// router-id leaf is 0.0.0.0 and whose one peer sets no override, then the
// same tree with the global leaf at 0.0.0.1.
//
// RFC 6286 Section 2.1: "The BGP Identifier is a 4-octet, unsigned, non-zero
// integer that should be unique within an AS."
//
// RFC requirement: RFC6286-2.1-1 positive -- a global bgp router-id of 0.0.0.0
// is rejected at config parse by PeersFromTree with the RFC 6286 Section 2.1
// error, so no peer inherits a zero BGP Identifier from the global leaf.
//
// RFC requirement: RFC6286-2.1-1 negative -- a global bgp router-id of
// 0.0.0.1 is accepted and every peer without an override inherits it as
// identifier 1: the global leaf refuses zero alone.
//
// VALIDATES: the global router-id leaf, which every peer without a session
// router-id override inherits, enforces the non-zero clause.
// PREVENTS: A zero global identifier reaching the OPEN of every peer.
func TestRFC6286GlobalZeroRouterIDRejected(t *testing.T) {
	tree := func(routerID string) map[string]any {
		return map[string]any{
			"router-id": routerID,
			"session":   map[string]any{"asn": map[string]any{"local": "65000"}},
			"peer": map[string]any{
				"peer1": map[string]any{
					"connection": map[string]any{"remote": map[string]any{"ip": "192.0.2.1"}, "local": map[string]any{"ip": "auto"}},
					"session":    map[string]any{"asn": map[string]any{"remote": "65001"}},
				},
			},
		}
	}

	_, err := PeersFromTree(tree("0.0.0.0"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RFC 6286 Section 2.1 requires a non-zero BGP Identifier")

	peers, err := PeersFromTree(tree("0.0.0.1"))
	require.NoError(t, err)
	require.Len(t, peers, 1)
	assert.Equal(t, uint32(1), peers[0].RouterID)
}
