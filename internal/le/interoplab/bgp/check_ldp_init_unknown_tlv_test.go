package bgp

import "testing"

// VALIDATES: the ldp-init-unknown-tlv-frr checker counts only RECEIVED Unknown
// TLV Notifications in FRR's log, and reads the rewrite rule's counter out of
// an nft listing without reading a missing counter as a count.
// PREVENTS: a Notification FRR sent, or a rule that never matched, passing the
// scenario.
func TestUnknownTLVNotificationsLogged(t *testing.T) {
	log := "ldpd: msg[in]: notification: lsr-id 172.30.0.2, status Unknown TLV\n" +
		"ldpd: msg[out]: notification: lsr-id 172.30.0.2, status Unknown TLV\n" +
		"ldpd: msg[in]: notification: lsr-id 172.30.0.2, status Shutdown (fatal error)\n"
	if got := unknownTLVNotificationsLogged(log); got != 1 {
		t.Fatalf("received Unknown TLV Notifications: want 1, got %d", got)
	}
}

func TestRewriteMatched(t *testing.T) {
	listing := "table inet ldplab {\n\tchain prerouting {\n\t\tip saddr 172.30.0.3 tcp sport 646 counter packets 2 bytes 180 @th,544,16 set 0x3f00\n\t}\n}\n"
	packets, counted := rewriteMatched(listing)
	if !counted || packets != 2 {
		t.Fatalf("rewriteMatched = %d, %v; want 2, true", packets, counted)
	}
	if _, counted := rewriteMatched("table inet ldplab {\n}\n"); counted {
		t.Fatal("a listing with no counter read as a count")
	}
}
