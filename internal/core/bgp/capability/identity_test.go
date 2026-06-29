package capability

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPeerIdentityIsIBGP verifies iBGP detection.
//
// VALIDATES: IsIBGP() returns true when LocalASN == PeerASN.
//
// PREVENTS: Wrong path attribute handling (iBGP vs eBGP rules differ).
func TestPeerIdentityIsIBGP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		localASN uint32
		peerASN  uint32
		want     bool
	}{
		{
			name:     "iBGP session",
			localASN: 65000,
			peerASN:  65000,
			want:     true,
		},
		{
			name:     "eBGP session",
			localASN: 65000,
			peerASN:  65001,
			want:     false,
		},
		{
			name:     "4-byte ASN iBGP",
			localASN: 4200000000,
			peerASN:  4200000000,
			want:     true,
		},
		{
			name:     "4-byte ASN eBGP",
			localASN: 4200000000,
			peerASN:  4200000001,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id := &PeerIdentity{
				LocalASN: tt.localASN,
				PeerASN:  tt.peerASN,
			}
			assert.Equal(t, tt.want, id.IsIBGP())
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
