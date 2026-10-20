package bgp

import "testing"

// VALIDATES: the ldp-keepalive-expiry-frr checker counts only received
// Notifications naming KeepAlive Timer Expired in FRR's log.
// PREVENTS: a line about FRR's own timer, or any other Notification, passing as
// the one ze sent.
func TestExpiryNotificationsLogged(t *testing.T) {
	// The lines are FRR 10.3's own format, read from FRR's log in a green run of
	// the scenario (2026-10-20); the msg[out] line is the form FRR logs for a
	// Notification it sends, when send debug is on.
	log := "2026/10/20 05:51:30 LDP: [ZM8R3-SDQ91] msg[in]: initialization: lsr-id 172.30.0.2\n" +
		"2026/10/20 05:51:31 LDP: [SKMP8-5DYH7] msg[in]: notification: lsr-id 172.30.0.2, status Shutdown (fatal error)\n" +
		"2026/10/20 05:51:33 LDP: [HJQMA-Z6XE5] nbr_ktimeout: lsr-id 172.30.0.2\n" +
		"2026/10/20 05:51:34 LDP: msg[out]: notification: lsr-id 172.30.0.2, status KeepAlive Timer Expired (fatal error)\n" +
		"2026/10/20 05:51:35 LDP: [SKMP8-5DYH7] msg[in]: notification: lsr-id 172.30.0.2, status KeepAlive Timer Expired (fatal error)\n"
	if got := expiryNotificationsLogged(log); got != 1 {
		t.Fatalf("expiryNotificationsLogged = %d, want 1", got)
	}
	if got := expiryNotificationsLogged(""); got != 0 {
		t.Fatalf("expiryNotificationsLogged(empty) = %d, want 0", got)
	}
}
