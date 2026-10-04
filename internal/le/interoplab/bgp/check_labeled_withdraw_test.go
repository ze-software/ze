package bgp

import "testing"

// TestFRRWithdrawalDecodes proves the count the labeled withdrawal scenario
// turns on: a second peer-down withdrawal of 10.10.0.0/24 makes the count two,
// while the announcement, the other prefix, the send direction and another
// neighbor never count. Method: count over a log built from FRR's own lines.
func TestFRRWithdrawalDecodes(t *testing.T) {
	peer := zeLabAddress
	withdrawn := func(prefix string) string {
		return "2026/10/04 03:18:06 BGP: [T1234-56789] " + peer + "(Unknown) rcvd UPDATE about " + prefix + " label 100 IPv4 labeled-unicast -- withdrawn\n"
	}
	announced := "BGP: " + peer + "(Unknown) rcvd " + labeledWithdrawPrefix + " label 100 IPv4 labeled-unicast\n"
	sent := "BGP: " + peer + " send UPDATE about " + labeledWithdrawPrefix + " IPv4 labeled-unicast -- withdrawn\n"
	other := "BGP: 172.30.0.22 rcvd UPDATE about " + labeledWithdrawPrefix + " IPv4 labeled-unicast -- withdrawn\n"

	once := announced + withdrawn(labeledWithdrawPrefix) + sent + other + withdrawn(labeledKeptPrefix)
	if got := frrWithdrawalDecodes(once, labeledWithdrawPrefix); got != 1 {
		t.Fatalf("one withdrawal of %s counted %d", labeledWithdrawPrefix, got)
	}
	twice := once + withdrawn(labeledWithdrawPrefix)
	if got := frrWithdrawalDecodes(twice, labeledWithdrawPrefix); got != 2 {
		t.Fatalf("a repeated withdrawal of %s counted %d, want 2", labeledWithdrawPrefix, got)
	}
	if got := frrWithdrawalDecodes(announced, labeledWithdrawPrefix); got != 0 {
		t.Fatalf("an announcement counted as %d withdrawals", got)
	}
}
