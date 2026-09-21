package peer

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestBgpSummaryLastErrorSeparatesToldFromCouldNotTell drives the real
// `show bgp peer summary` handler with two peers that ended on the same BFD
// Down Cease: one the socket carried, one it refused.
//
// VALIDATES: last-error names the reason for both, and only the undelivered
// one carries the " (not sent)" qualifier, so an operator can tell "we told
// the peer" from "we could not tell the peer".
// PREVENTS: two failures at once. A peer whose Cease was refused reporting a
// blank last-error, which is the answer a healthy peer gives, and a peer whose
// Cease was delivered being labeled undelivered.
//
// The field is also birdwatcher's `last_error` on the public looking glass
// (internal/component/lg/handler_api.go), whose schema holds one string, which
// is why the qualifier is in the string rather than in a second key.
//
// RFC requirement: RFC9384-4-1 positive -- a Cease NOTIFICATION that could not
// be sent reaches the operator through `show bgp peer summary`, as
// "Cease/BFD Down (not sent)" (internal/component/bgp/plugins/cmd/peer/
// summary.go, lastErrorString).
// RFC requirement: RFC9384-4-1 negative -- the qualifier is reserved for the
// undelivered case: the same code and subcode delivered render
// "Cease/BFD Down" with no qualifier, and a peer that never tore down renders
// the empty string rather than an invented reason.
func TestBgpSummaryLastErrorSeparatesToldFromCouldNotTell(t *testing.T) {
	stamp := time.Date(2026, 9, 20, 11, 0, 0, 0, time.UTC)
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address:            netip.MustParseAddr("192.0.2.1"),
				PeerAS:             65001,
				State:              plugin.PeerStateStopped,
				LastNotifCode:      6,
				LastNotifSubcode:   10,
				LastNotifDirection: plugin.NotifSendFailed,
				LastNotifTime:      stamp,
			},
			{
				Address:            netip.MustParseAddr("192.0.2.2"),
				PeerAS:             65002,
				State:              plugin.PeerStateStopped,
				LastNotifCode:      6,
				LastNotifSubcode:   10,
				LastNotifDirection: plugin.NotifSent,
				LastNotifTime:      stamp,
			},
			{
				Address: netip.MustParseAddr("192.0.2.3"),
				PeerAS:  65003,
				State:   plugin.PeerStateEstablished,
			},
		},
		stats: plugin.ReactorStats{PeerCount: 3, RouterID: 0x0a000001, LocalAS: 65000},
	}
	ctx := newTestContext(reactor)

	resp, err := handleBgpSummary(ctx, nil)
	require.NoError(t, err)

	data, ok := resp.Data.(plugin.Map)
	require.True(t, ok)
	summary := map[string]any(data)
	peers, ok := summary["peers"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, peers, 3)

	assert.Equal(t, "Cease/BFD Down (not sent)", peers[0]["last-error"],
		"the reason must reach operational state even though the write failed")
	assert.Equal(t, "Cease/BFD Down", peers[1]["last-error"],
		"a delivered Cease must not be reported as undelivered")
	assert.Equal(t, "", peers[2]["last-error"],
		"a peer that never tore down has no reason to report")
}
