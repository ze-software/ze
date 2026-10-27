// Design: rfc/short/rfc5082.md -- the listen-port floor a GTSM peer owes before the handshake
// Related: netfilter_fixture_gtsm.go -- checkGTSMRuleset, the related-ICMP judge
// Related: netfilter_fixture_gtsm_listener.go -- rulePresent, the listener judge

package fixture

import (
	"strings"
	"testing"
)

// gtsmTestTable renders a ze_gtsm table the way nft lists it for the IPv4
// peer 192.0.2.2, with the listen rules on listenPort: one IPv4 listen rule
// and gtsmRelatedICMPRuleCount related-ICMP rules carrying the quoted read.
// withIPv6 adds the listen rule of the IPv6 peer 2001:db8::2, as the
// gtsm-listener-floor configuration has. The lines are the shape the QEMU
// guest printed for Ze's listen port 1846.
func gtsmTestTable(t *testing.T, listenPort string, withIPv6 bool) string {
	t.Helper()
	quoted, err := quotedDestinationRead(gtsmPeerAddress)
	if err != nil {
		t.Fatalf("quotedDestinationRead: %v", err)
	}
	lines := []string{
		"table inet ze_gtsm {",
		"\tchain input {",
		"\t\ttype filter hook input priority filter; policy accept;",
		"\t\tip saddr 192.0.2.2 tcp dport " + listenPort + " ip ttl < 255 counter packets 0 bytes 0 drop",
	}
	for range gtsmRelatedICMPRuleCount {
		lines = append(lines, "\t\ticmp type destination-unreachable meta l4proto icmp @th,64,8 0x45 "+quoted+
			" meta l4proto icmp @th,64,8 0x45 @th,136,8 0x6 @th,224,16 0x736 ip ttl < 255 counter packets 0 bytes 0 drop")
	}
	if withIPv6 {
		lines = append(lines, "\t\tip6 saddr 2001:db8::2 tcp dport "+listenPort+" ip6 hoplimit < 255 counter packets 0 bytes 0 drop")
	}
	lines = append(lines, "\t}", "}")
	return strings.Join(lines, "\n")
}

// TestCheckGTSMRulesetJudgesTheListenPort proves the related-ICMP driver
// accepts the listen rule only on the port the test was given. Method: the
// same rendered table is judged with the port it carries, then with a port it
// does not carry, including 179 (the port the driver once assumed) and a port
// whose digits are a prefix of the rendered one.
func TestCheckGTSMRulesetJudgesTheListenPort(t *testing.T) {
	quoted, err := quotedDestinationRead(gtsmPeerAddress)
	if err != nil {
		t.Fatalf("quotedDestinationRead: %v", err)
	}
	table := gtsmTestTable(t, "1846", false)
	if err := checkGTSMRuleset(table, gtsmPeerAddress, "1846", quoted); err != nil {
		t.Fatalf("listen rule on the test's port 1846 refused: %v", err)
	}
	for _, port := range []string{"179", "184", "18460", "1844"} {
		if err := checkGTSMRuleset(table, gtsmPeerAddress, port, quoted); err == nil {
			t.Errorf("listen rule on port 1846 accepted for the test's port %s", port)
		}
	}
}

// TestGTSMListenRulePresentJudgesTheListenPort proves the listener driver
// finds each family's listen rule only on the port the test was given. Method:
// one rendered table on port 1846 is searched for the rules of 1846, then of
// 179 and 184, whose text is a prefix of the rendered port and which a
// substring match would accept.
func TestGTSMListenRulePresentJudgesTheListenPort(t *testing.T) {
	table := gtsmTestTable(t, "1846", true)
	if !rulePresent(table, gtsmListenRuleIPv4("1846")) {
		t.Fatal("IPv4 listen rule on the test's port 1846 not found")
	}
	if !rulePresent(table, gtsmListenRuleIPv6("1846")) {
		t.Fatal("IPv6 listen rule on the test's port 1846 not found")
	}
	for _, port := range []string{"179", "184", "18460"} {
		if rulePresent(table, gtsmListenRuleIPv4(port)) {
			t.Errorf("IPv4 listen rule on port 1846 found for the test's port %s", port)
		}
		if rulePresent(table, gtsmListenRuleIPv6(port)) {
			t.Errorf("IPv6 listen rule on port 1846 found for the test's port %s", port)
		}
	}
}

// TestGTSMListenPortRefusesAMissingOrBadPort proves both GTSM drivers refuse
// to run without a usable listen port rather than guess one. Method: each
// malformed argument list is passed to gtsmListenPort, then a valid one.
func TestGTSMListenPortRefusesAMissingOrBadPort(t *testing.T) {
	for _, args := range [][]string{nil, {"1846", "1847"}, {"bgp"}, {"0"}, {"65536"}, {"-1"}} {
		if port, err := gtsmListenPort("firewall/gtsm-related-icmp", args); err == nil {
			t.Errorf("gtsmListenPort(%q) = %q, want an error", args, port)
		}
	}
	port, err := gtsmListenPort("firewall/gtsm-related-icmp", []string{"1846"})
	if err != nil {
		t.Fatalf("gtsmListenPort(1846): %v", err)
	}
	if port != "1846" {
		t.Fatalf("gtsmListenPort(1846) = %q, want 1846", port)
	}
}
