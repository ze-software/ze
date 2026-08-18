package firewall

import "testing"

// TestProtocolNumber pins the single protocol-name -> IANA-number table that
// all firewall backends share.
//
// VALIDATES: ProtocolNumber maps every supported L4 protocol name to its IANA
// number and reports ok=false for unknown names.
// PREVENTS: the nft, VPP-classify, and VPP-NAT backends keeping divergent
// private copies of this table -- the NAT copy previously handled only tcp/udp
// and silently programmed protocol 0 for every other protocol.
func TestProtocolNumber(t *testing.T) {
	want := map[string]uint8{
		"tcp": 6, "udp": 17, "icmp": 1, "icmpv6": 58,
		"sctp": 132, "gre": 47, "esp": 50, "ah": 51,
		"ospf": 89, "vrrp": 112,
	}
	for name, num := range want {
		got, ok := ProtocolNumber(name)
		if !ok || got != num {
			t.Errorf("ProtocolNumber(%q) = (%d, %v), want (%d, true)", name, got, ok, num)
		}
	}
	for _, name := range []string{"", "TCP", "bogus", "ip", "0"} {
		if got, ok := ProtocolNumber(name); ok {
			t.Errorf("ProtocolNumber(%q) = (%d, true), want ok=false", name, got)
		}
	}
}

// TestProtocolNameRoundTripsEveryCanonicalNumber pins the reverse direction of
// the single protocol table.
//
// VALIDATES: ProtocolName returns the canonical name for every number
// ProtocolNumber hands out, and ProtocolNumber accepts that name back.
// PREVENTS: a producer that starts from a wire protocol number keeping its own
// number -> name table, which is how the FlowSpec translator came to know five
// of the ten names and to render the rest as decimal digits.
func TestProtocolNameRoundTripsEveryCanonicalNumber(t *testing.T) {
	for _, name := range ProtocolNames() {
		num, ok := ProtocolNumber(name)
		if !ok {
			t.Fatalf("ProtocolNumber(%q) = ok false, but the name came from ProtocolNames", name)
		}
		back, ok := ProtocolName(num)
		if !ok || back != name {
			t.Errorf("ProtocolName(%d) = (%q, %v), want (%q, true)", num, back, ok, name)
		}
	}
}

// TestProtocolNameRefusesUnnamedNumber covers the boundary rows of the spec's
// numeric table: 0 sits below the lowest canonical number and 133 above the
// highest, and neither has a name.
//
// VALIDATES: ProtocolName reports ok=false rather than inventing a spelling.
// PREVENTS: a MatchProtocol carrying digits, which no backend can lower and
// which aborts the whole firewall reconcile for every owner.
func TestProtocolNameRefusesUnnamedNumber(t *testing.T) {
	for _, num := range []uint8{0, 2, 4, 41, 133, 255} {
		if got, ok := ProtocolName(num); ok {
			t.Errorf("ProtocolName(%d) = (%q, true), want ok=false", num, got)
		}
	}
}

// TestProtocolNamesListsEveryCanonicalName checks the accessor validators and
// error messages use so they never spell the accepted set a second time.
//
// VALIDATES: ProtocolNames returns each name once, sorted.
// PREVENTS: an error message that names a set the backends do not accept.
func TestProtocolNamesListsEveryCanonicalName(t *testing.T) {
	names := ProtocolNames()
	if len(names) != len(ianaProtocolNumbers) {
		t.Fatalf("ProtocolNames returned %d names, table holds %d", len(names), len(ianaProtocolNumbers))
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Fatalf("ProtocolNames is not sorted and deduplicated: %v", names)
		}
	}
	for _, name := range names {
		if _, ok := ianaProtocolNumbers[name]; !ok {
			t.Errorf("ProtocolNames returned %q, which the table does not carry", name)
		}
	}
}
