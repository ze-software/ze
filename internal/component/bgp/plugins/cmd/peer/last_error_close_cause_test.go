package peer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestLastErrorNamesCloseWithoutNotification pins what last-error answers for
// a session ze closed by itself with no NOTIFICATION built, because the peer
// accepted no data for the write budget.
//
// VALIDATES: spec-bgp-session-write-wedge AC-10. The cause is the last error
// when it is the newer record, in the fixed words of plugin.CloseCause, and a
// NOTIFICATION newer than it is the last error again.
// PREVENTS: a peer that stopped reading answering "" like a healthy peer, or
// answering a NOTIFICATION from an earlier session as the reason for this one.
func TestLastErrorNamesCloseWithoutNotification(t *testing.T) {
	earlier := time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC)
	later := earlier.Add(time.Minute)
	const budgetClose = "peer accepted no data for the write budget (no NOTIFICATION sent)"

	cases := []struct {
		name string
		info plugin.PeerInfo
		want string
	}{
		{
			name: "write budget close, no notification ever",
			info: plugin.PeerInfo{
				LastCloseCause: plugin.CloseCauseWriteBudget, LastCloseCauseTime: later,
			},
			want: budgetClose,
		},
		{
			name: "write budget close after an older notification",
			info: plugin.PeerInfo{
				LastNotifCode: 6, LastNotifSubcode: 2,
				LastNotifDirection: plugin.NotifSent, LastNotifTime: earlier,
				LastCloseCause: plugin.CloseCauseWriteBudget, LastCloseCauseTime: later,
			},
			want: budgetClose,
		},
		{
			name: "notification after an older write budget close",
			info: plugin.PeerInfo{
				LastNotifCode: 6, LastNotifSubcode: 2,
				LastNotifDirection: plugin.NotifSent, LastNotifTime: later,
				LastCloseCause: plugin.CloseCauseWriteBudget, LastCloseCauseTime: earlier,
			},
			want: "Cease/Administrative Shutdown",
		},
		{
			name: "no close recorded leaves the notification",
			info: plugin.PeerInfo{
				LastNotifCode: 4, LastNotifSubcode: 0,
				LastNotifDirection: plugin.NotifReceived, LastNotifTime: earlier,
			},
			want: "Hold Timer Expired/Unspecific",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, lastErrorString(&tc.info))
		})
	}
}
