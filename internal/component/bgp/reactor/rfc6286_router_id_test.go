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
