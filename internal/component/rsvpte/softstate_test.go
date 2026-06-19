// Design: plan/spec-mpls-3-rsvp-te.md -- RSVP-TE soft-state expiry (F8) test
package rsvpte

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
)

// VALIDATES: F8 -- the refresh loop must NOT locally re-stamp a transit/egress
// PSB. That state is refreshed only by the incoming PATH it relays (RFC 2205
// Section 3.4); stamping it locally would stop the cleanup loop from ever
// expiring the LSP after the upstream stops refreshing, leaking the reservation
// and FIB state.
func TestRefreshDoesNotStampEgressPSB(t *testing.T) {
	e, _, _ := testEngine(t, "10.0.0.9", nil)
	path := buildPath(egressTestPSB(), netip.MustParseAddr("10.0.0.1"), 64)
	e.handlePacket(Packet{Src: netip.MustParseAddr("10.0.0.1"), Payload: path})

	key := lspKey{
		TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1,
		ExtTunnelID: 0x0a000001, SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1,
	}
	lsp, ok := e.table.Get(key)
	require.True(t, ok)
	require.Equal(t, RoleEgress, lsp.Role)

	old := time.Now().Add(-time.Hour)
	lsp.mu.Lock()
	lsp.PSB.LastRefresh = old
	lsp.mu.Unlock()

	refreshPaths(slogutil.DiscardLogger(), e.table, e)

	lsp.mu.Lock()
	got := lsp.PSB.LastRefresh
	lsp.mu.Unlock()
	assert.True(t, got.Equal(old), "egress PSB must not be re-stamped by the refresh loop")

	// And it is therefore expirable: with a refresh multiplier of 1 the hour-old
	// PSB is past its deadline.
	expired := e.table.expiredPSBs(time.Now(), 1)
	assert.Contains(t, expired, key, "stale egress LSP must be eligible for cleanup")
}
