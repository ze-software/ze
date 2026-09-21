// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RFC 3209 Section 4.4.3 RRO proofs
// Related: rro.go -- prependRRO; engine.go -- recordRoute
package rsvpte

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RFC requirement: RFC3209-4.4.3-1 positive — the subobject prependRRO pushes at the head of the RRO is an IPv4 subobject carrying this router's own address, ahead of the downstream route
func TestRFC3209RRONewSubobjectIsOwnAddress(t *testing.T) {
	self := netip.MustParseAddr("10.0.0.5")
	downstream := []rroEntry{{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.9")}}
	out, truncated := prependRRO(self, downstream)
	require.False(t, truncated)
	require.Len(t, out, 2)
	assert.Equal(t, rroEntry{Type: RROSubIPv4, Address: self}, out[0])
	assert.Equal(t, downstream[0], out[1])
}

// RFC requirement: RFC3209-4.4.3-1 negative — with no valid own address there is nothing that could be this router's IP address, so prependRRO pushes no subobject and the downstream route is returned unchanged
func TestRFC3209RRONoSubobjectWithoutOwnAddress(t *testing.T) {
	downstream := []rroEntry{{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.9")}}
	out, truncated := prependRRO(netip.Addr{}, downstream)
	require.False(t, truncated)
	assert.Equal(t, downstream, out, "no subobject is pushed")
}

// RFC requirement: RFC3209-4.4.3-2 positive — when recordRoute records a label it pushes the pair [IPv4 own address, Label Record] onto the RRO, the address first
func TestRFC3209RROLabelRecordFollowsAddress(t *testing.T) {
	e, _, _ := testEngine(t, "10.0.0.5", nil)
	downstream := []rroEntry{{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.9")}}
	rro := e.recordRoute(downstream, 3, 0, 1001)
	require.Len(t, rro, 3)
	assert.Equal(t, rroEntry{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.5")}, rro[0])
	assert.Equal(t, rroEntry{Type: RROSubLabel, Label: 1001}, rro[1])
	assert.Equal(t, downstream[0], rro[2])
}

// RFC requirement: RFC3209-4.4.3-2 negative — when no IPv4 subobject is pushed (no valid own address) recordRoute pushes no Label Record either, even though a label was given to record
func TestRFC3209RRONoLabelRecordWithoutAddress(t *testing.T) {
	e, _, _ := testEngine(t, "10.0.0.5", func(c *rsvpteConfig) { c.RouterID = netip.Addr{} })
	downstream := []rroEntry{{Type: RROSubIPv4, Address: netip.MustParseAddr("10.0.0.9")}}
	rro := e.recordRoute(downstream, 3, 0, 1001)
	assert.Equal(t, downstream, rro, "no address pushed, so no Label Record pushed")
	for _, entry := range rro {
		assert.NotEqual(t, RROSubLabel, entry.Type)
	}
}
