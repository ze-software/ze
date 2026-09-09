package irr

// VALIDATES: a dotted `source-asn` names one nftables set on both sides.
// PREVENTS: the rule matching irr_v4_AS1.10 while the owner supplies
// irr_v4_AS65546, which leaves dropTablesMissingAProvidedSet holding the
// table back while the commit still reports success.

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
)

// asnConfig returns a one-term firewall config whose source-asn is spelled as
// given.
func asnConfig(spelling string) string {
	return `{
		"firewall": {
			"table": {
				"wan": {
					"family": "inet",
					"chain": {
						"input": {
							"term": {
								"allow-peer": {
									"from": {"source-asn": "` + spelling + `"},
									"then": {"accept": {}}
								}
							}
						}
					}
				}
			}
		}
	}`
}

// matchedSetName returns the set name the firewall rule parser produces for
// the config's single MatchInSet.
func matchedSetName(t *testing.T, config string) string {
	t.Helper()
	tables, err := firewall.ParseFirewallConfig(config)
	if err != nil {
		t.Fatalf("ParseFirewallConfig: %v", err)
	}
	for _, table := range tables {
		for _, chain := range table.Chains {
			for _, term := range chain.Terms {
				for _, match := range term.Matches {
					if inSet, ok := match.(firewall.MatchInSet); ok {
						return inSet.SetName
					}
				}
			}
		}
	}
	t.Fatal("the parsed config carries no MatchInSet, so this test guards nothing")
	return ""
}

// suppliedSetName returns the IPv4 set name the firewall-irr owner registers
// for the config's single source-asn reference.
func suppliedSetName(t *testing.T, config string) string {
	t.Helper()
	var root map[string]any
	if err := json.Unmarshal([]byte(config), &root); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	refs := extractRefsFromConfig(root)
	if len(refs) != 1 {
		t.Fatalf("extractRefsFromConfig returned %d references, want 1", len(refs))
	}
	v4, _ := setNames(refs[0].Name)
	return v4
}

// TestASNSetNameAgreesAcrossBothDerivations proves the rule and the set owner
// name one set for every RFC 5396 spelling of the same AS number. The method
// parses the same config through the firewall rule parser and through this
// plugin's reference extractor, then compares the two names.
func TestASNSetNameAgreesAcrossBothDerivations(t *testing.T) {
	for _, spelling := range []string{"65546", "1.10", "AS65546"} {
		config := asnConfig(spelling)
		matched := matchedSetName(t, config)
		supplied := suppliedSetName(t, config)
		if matched != supplied {
			t.Errorf("source-asn %q: the rule matches %q but the owner supplies %q",
				spelling, matched, supplied)
		}
		if want := "irr_v4_AS65546"; matched != want {
			t.Errorf("source-asn %q named the set %q, want %q", spelling, matched, want)
		}
	}
}

// TestASNWhoisKeyIsDecimal proves the IRR whois key a dotted AS number
// produces is the decimal spelling, which is the only one a whois server
// answers. The method renders each spelling and compares the key.
//
// VALIDATES: `source-asn 1.10` queries AS65546, not AS1.10.
// PREVENTS: an empty prefix set, and so a term that matches no packet.
func TestASNWhoisKeyIsDecimal(t *testing.T) {
	for _, spelling := range []string{"65546", "1.10", "0.65535"} {
		got := firewall.IRRASNName(spelling)
		want := map[string]string{"65546": "AS65546", "1.10": "AS65546", "0.65535": "AS65535"}[spelling]
		if got != want {
			t.Errorf("IRRASNName(%q) = %q, want %q", spelling, got, want)
		}
	}
}

// TestClearASNReadsEveryNotation proves `clear firewall irr asn` reaches the
// entry cached under the decimal name whichever RFC 5396 spelling the
// operator types. The method caches the entry, clears it by each spelling,
// and reads the store back.
//
// VALIDATES: the clear command word reads asplain, asdot and asdot+.
// PREVENTS: "no cached data for AS1.10" for an entry that is cached.
func TestClearASNReadsEveryNotation(t *testing.T) {
	for _, tc := range []struct {
		spelling string
		cached   string
	}{
		{"65546", "AS65546"}, // asplain
		{"1.10", "AS65546"},  // asdot
		{"0.100", "AS100"},   // asdot+, which asdot writes as plain 100
	} {
		ps := store.New(nil, nil, "")
		ps.Put(tc.cached, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/24")}, nil)
		plug := &irrPlugin{prefixStore: ps, config: &irrConfig{Server: defaultServer}}

		status, _, err := plug.handleCommand("clear firewall irr asn", []string{tc.spelling})
		if err != nil {
			t.Errorf("clear firewall irr asn %q: %v", tc.spelling, err)
			continue
		}
		if status != statusDone {
			t.Errorf("clear firewall irr asn %q: status = %q, want %q", tc.spelling, status, statusDone)
		}
		if got := ps.Get(tc.cached); got != nil {
			t.Errorf("clear firewall irr asn %q left %s in place: %+v", tc.spelling, tc.cached, got)
		}
	}

	// A dotted token whose low field overflows names no AS number, so the
	// command still refuses it.
	plug := &irrPlugin{prefixStore: store.New(nil, nil, ""), config: &irrConfig{Server: defaultServer}}
	if _, _, err := plug.handleCommand("clear firewall irr asn", []string{"0.65546"}); err == nil {
		t.Error("clear firewall irr asn accepted 0.65546, which names no AS number")
	}
}
