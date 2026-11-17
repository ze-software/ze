// Design: docs/guide/firewall.md -- the destination-local match
// Related: config.go -- parseFromBlock, the leaf under test
// Related: validate.go -- validateMatch, the family check under test

package firewall

import (
	"strings"
	"testing"
)

// localMatchConfig is a firewall config section of one table whose one term
// carries the leaves named in from, in the family given.
func localMatchConfig(family, from string) string {
	return `{"firewall":{"table":{"local":{"family":"` + family + `","chain":{"prerouting":{` +
		`"type":"nat","hook":"prerouting","priority":"-100","policy":"accept",` +
		`"term":{"published":{"from":{` + from + `},"then":{"dnat":{"to":"192.0.2.10:6379"}}}}}}}}}}`
}

// TestParseLocalDestinationMatch proves the destination-local leaf parses to
// the match and only it does: a term with the leaf carries
// MatchDestinationLocal beside its port match, the same term without the leaf
// carries none, an ip table with the match validates, and an inet table and a
// bridge table with it are each refused naming the leaf and the families that
// take it.
func TestParseLocalDestinationMatch(t *testing.T) {
	const port = `"protocol":"tcp","destination-port":"8080"`
	cases := []struct {
		name string
		from string
		want bool
	}{
		{"with the leaf", port + `,"destination-local":""`, true},
		{"without the leaf", port, false},
	}
	for _, tc := range cases {
		tables, err := ParseFirewallConfig(localMatchConfig("ip", tc.from))
		if err != nil {
			t.Fatalf("%s: parse: %v", tc.name, err)
		}
		if len(tables) != 1 || len(tables[0].Chains) != 1 || len(tables[0].Chains[0].Terms) != 1 {
			t.Fatalf("%s: parsed %+v, want one table, chain and term", tc.name, tables)
		}
		found := false
		for _, m := range tables[0].Chains[0].Terms[0].Matches {
			if _, ok := m.(MatchDestinationLocal); ok {
				found = true
			}
		}
		if found != tc.want {
			t.Fatalf("%s: MatchDestinationLocal present = %v, want %v", tc.name, found, tc.want)
		}
		if err := ValidateTables(tables); err != nil {
			t.Fatalf("%s: an ip table was refused: %v", tc.name, err)
		}
	}

	// An inet table: the kernel has no fib lookup for it on Ze's kernel, so
	// verify refuses the leaf there and says which families take it.
	inet, err := ParseFirewallConfig(localMatchConfig("inet", port+`,"destination-local":""`))
	if err != nil {
		t.Fatalf("inet: parse: %v", err)
	}
	inetErr := ValidateTables(inet)
	if inetErr == nil {
		t.Fatal("destination-local was accepted in an inet table")
	}
	if !strings.Contains(inetErr.Error(), "destination-local is valid only in family ip or ip6, got inet") {
		t.Fatalf("inet refusal does not name the leaf and its families: %v", inetErr)
	}

	// A filter chain, so nothing but the match family check can refuse it.
	const bridgeConfig = `{"firewall":{"table":{"local":{"family":"bridge","chain":{"forward":{` +
		`"type":"filter","hook":"forward","priority":"0","policy":"accept",` +
		`"term":{"local":{"from":{"destination-local":""},"then":{"accept":""}}}}}}}}}`
	bridge, err := ParseFirewallConfig(bridgeConfig)
	if err != nil {
		t.Fatalf("bridge: parse: %v", err)
	}
	err = ValidateTables(bridge)
	if err == nil {
		t.Fatal("destination-local was accepted in a bridge table")
	}
	if !strings.Contains(err.Error(), "destination-local is valid only in family ip or ip6, got bridge") {
		t.Fatalf("bridge refusal does not name the leaf and its families: %v", err)
	}
}
