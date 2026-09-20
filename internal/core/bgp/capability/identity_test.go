package capability

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPeerIdentityIsIBGP verifies that the internal verdict is CARRIED, not re-derived.
//
// VALIDATES: IsIBGP() answers the Internal field the session set, for equal ASNs, for
// different ASNs, and for the RFC 7705 Section 4.2 migration case where the two ASNs differ
// and the session is internal all the same.
//
// PREVENTS: a second declaration of the iBGP rule. Recomputing LocalASN == PeerASN here
// disagreed with PeerSettings.isIBGPWith (reactor/session_as_migration.go), which is the one
// rule, and it read a PeerASN that negotiateWith left at 0 on every session. The last row is
// the case the equality gets WRONG even when the ASNs are truthful: RFC 7705 Section 4.2
// requires a renumbering speaker to "treat UPDATEs sent and received to this peer as if this
// was a natively configured iBGP session", and eBGP rules there send the wrong AS_PATH to a
// peer that trusts it.
func TestPeerIdentityIsIBGP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		localASN uint32
		peerASN  uint32
		internal bool
	}{
		{
			name:     "iBGP session",
			localASN: 65000,
			peerASN:  65000,
			internal: true,
		},
		{
			name:     "eBGP session",
			localASN: 65000,
			peerASN:  65001,
			internal: false,
		},
		{
			name:     "4-byte ASN iBGP",
			localASN: 4200000000,
			peerASN:  4200000000,
			internal: true,
		},
		{
			name:     "4-byte ASN eBGP",
			localASN: 4200000000,
			peerASN:  4200000001,
			internal: false,
		},
		{
			name:     "RFC 7705 migration session under the legacy ASN",
			localASN: 65000,
			peerASN:  64500,
			internal: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id := &PeerIdentity{
				LocalASN: tt.localASN,
				PeerASN:  tt.peerASN,
				Internal: tt.internal,
			}
			assert.Equal(t, tt.internal, id.IsIBGP())
		})
	}
}

// TestPeerIdentityRouterIDs verifies router ID storage.
//
// VALIDATES: Router IDs are stored and accessible.
//
// PREVENTS: Missing router IDs for route reflection (ORIGINATOR_ID).
func TestPeerIdentityRouterIDs(t *testing.T) {
	t.Parallel()
	id := &PeerIdentity{
		LocalASN:      65000,
		PeerASN:       65001,
		LocalRouterID: 0x0a000001, // 10.0.0.1
		PeerRouterID:  0x0a000002, // 10.0.0.2
	}

	assert.Equal(t, uint32(0x0a000001), id.LocalRouterID)
	assert.Equal(t, uint32(0x0a000002), id.PeerRouterID)
}
