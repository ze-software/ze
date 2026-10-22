package reactor

import (
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestClosingNotification: the down event's NOTIFICATION is the last one that
// crossed the wire, with its direction and codes, and a NOTIFICATION ze could
// not send answers none, because the peer never saw it.
func TestClosingNotification(t *testing.T) {
	peer := newTestPeer()
	if got := peer.closingNotification(); got.Direction != plugin.NotifNone {
		t.Fatalf("no NOTIFICATION yet, got %+v", got)
	}

	peer.incrNotificationSent(6, 4)
	want := plugin.CloseNotification{Direction: plugin.NotifSent, Code: 6, Subcode: 4}
	if got := peer.closingNotification(); got != want {
		t.Errorf("after a sent Cease: got %+v, want %+v", got, want)
	}

	peer.incrNotificationReceived(4, 0)
	want = plugin.CloseNotification{Direction: plugin.NotifReceived, Code: 4, Subcode: 0}
	if got := peer.closingNotification(); got != want {
		t.Errorf("after a received Hold Timer Expired: got %+v, want %+v", got, want)
	}

	peer.recordNotificationUnsent(6, 2)
	if got := peer.closingNotification(); got.Direction != plugin.NotifNone {
		t.Errorf("an unsent NOTIFICATION reads as one the peer saw: %+v", got)
	}
}
