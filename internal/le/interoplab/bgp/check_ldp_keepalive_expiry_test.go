package bgp

import "testing"

// VALIDATES: the ldp-keepalive-expiry-frr checker counts only received
// Notifications naming KeepAlive Timer Expired in FRR's log.
// PREVENTS: a line about FRR's own timer, or any other Notification, passing as
// the one ze sent.
func TestExpiryNotificationsLogged(t *testing.T) {
	log := "2026/10/14 10:00:00 LDP: msg-in: notification: lsr-id 172.30.0.2, status Shutdown\n" +
		"2026/10/14 10:00:01 LDP: keepalive timer: KeepAlive Timer Expired\n" +
		"2026/10/14 10:00:02 LDP: msg-in: notification: lsr-id 172.30.0.2, status KeepAlive Timer Expired\n"
	if got := expiryNotificationsLogged(log); got != 1 {
		t.Fatalf("expiryNotificationsLogged = %d, want 1", got)
	}
	if got := expiryNotificationsLogged(""); got != 0 {
		t.Fatalf("expiryNotificationsLogged(empty) = %d, want 0", got)
	}
}
