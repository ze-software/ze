// Design: docs/architecture/core-design.md -- Firewall literal-prefix input contract
// Related: config.go -- parseAddressMatch and contextual configuration errors

package firewall

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestParseAddressMatchRejectsHostBits checks that either configured address
// direction refuses a host address used as a network prefix, in both families.
func TestParseAddressMatchRejectsHostBits(t *testing.T) {
	cases := []struct {
		prefix  string
		network string
	}{
		{"10.10.10.1/30", "10.10.10.0/30"},
		{"2001:db8:10:10::1/126", "2001:db8:10:10::/126"},
		{"192.0.2.1/0", "0.0.0.0/0"},
		{"2001:db8::1/0", "::/0"},
		{"192.0.2.3/31", "192.0.2.2/31"},
		{"2001:db8::3/127", "2001:db8::2/127"},
	}
	for _, isSource := range []bool{true, false} {
		leaf := "destination-address"
		if isSource {
			leaf = "source-address"
		}
		for _, tc := range cases {
			t.Run(leaf+"/"+tc.prefix, func(t *testing.T) {
				match, err := parseAddressMatch(tc.prefix, isSource)
				if err == nil {
					t.Fatalf("parseAddressMatch(%q) accepted host bits as %v", tc.prefix, match)
				}
				if match != nil {
					t.Errorf("refused prefix returned match %v", match)
				}
				var operator *opererr.Error
				if !errors.As(err, &operator) {
					t.Fatalf("error %v is not an operator error", err)
				}
				if !strings.Contains(operator.Problem(), tc.prefix) {
					t.Errorf("problem %q does not name input %q", operator.Problem(), tc.prefix)
				}
				if !strings.Contains(operator.Why(), tc.network) {
					t.Errorf("explanation %q does not name network %q", operator.Why(), tc.network)
				}
				if got, want := operator.Fix(), leaf+" "+tc.network; got != want {
					t.Errorf("fix = %q, want corrected statement %q", got, want)
				}
			})
		}
	}
}

// TestParseAddressMatchPreservesCanonical proves the refusal discriminates:
// canonical networks, default routes and host routes keep their exact prefix.
func TestParseAddressMatchPreservesCanonical(t *testing.T) {
	for _, isSource := range []bool{true, false} {
		for _, value := range []string{
			"10.10.10.0/30", "2001:db8:10:10::/126",
			"0.0.0.0/0", "::/0", "192.0.2.1/32", "2001:db8::1/128",
			"192.0.2.2/31", "2001:db8::2/127",
		} {
			t.Run(fmt.Sprintf("source=%t/%s", isSource, value), func(t *testing.T) {
				match, err := parseAddressMatch(value, isSource)
				if err != nil {
					t.Fatalf("canonical prefix %q refused: %v", value, err)
				}
				want := netip.MustParsePrefix(value)
				if isSource {
					source, ok := match.(MatchSourceAddress)
					if !ok {
						t.Fatalf("match = %T, want MatchSourceAddress", match)
					}
					if source.Prefix != want {
						t.Errorf("source prefix = %v, want %v", source.Prefix, want)
					}
					return
				}
				destination, ok := match.(MatchDestinationAddress)
				if !ok {
					t.Fatalf("match = %T, want MatchDestinationAddress", match)
				}
				if destination.Prefix != want {
					t.Errorf("destination prefix = %v, want %v", destination.Prefix, want)
				}
			})
		}
	}
}

// TestParseAddressMatchPreservesNamedSet keeps the non-literal branch unchanged:
// the existing set validation still owns name resolution and element type.
func TestParseAddressMatchPreservesNamedSet(t *testing.T) {
	for _, isSource := range []bool{true, false} {
		t.Run(fmt.Sprintf("source=%t", isSource), func(t *testing.T) {
			match, err := parseAddressMatch("@trusted-hosts", isSource)
			if err != nil {
				t.Fatalf("named set refused: %v", err)
			}
			set, ok := match.(MatchInSet)
			if !ok {
				t.Fatalf("match = %T, want MatchInSet", match)
			}
			if set.SetName != "trusted-hosts" {
				t.Errorf("set name = %q, want trusted-hosts", set.SetName)
			}
			wantField := SetFieldDestAddr
			if isSource {
				wantField = SetFieldSourceAddr
			}
			if set.MatchField != wantField {
				t.Errorf("match field = %v, want %v", set.MatchField, wantField)
			}
		})
	}
}

// TestParseAddressMatchKeepsUnsupportedFormsRejected prevents an input cleanup
// from adding bare addresses, ranges, negation or invalid CIDR lengths.
func TestParseAddressMatchKeepsUnsupportedFormsRejected(t *testing.T) {
	for _, isSource := range []bool{true, false} {
		for _, value := range []string{
			"", "192.0.2.1", "2001:db8::1", "192.0.2.1-192.0.2.4",
			"!192.0.2.0/24", "!2001:db8::/32", "not-a-prefix",
			"192.0.2.0/-1", "192.0.2.0/33", "2001:db8::/-1", "2001:db8::/129",
		} {
			t.Run(fmt.Sprintf("source=%t/%s", isSource, value), func(t *testing.T) {
				if _, err := parseAddressMatch(value, isSource); err == nil {
					t.Errorf("unsupported input %q was accepted", value)
				}
			})
		}
	}
}

// TestParseFirewallConfigPrefixErrorContext proves each parse wrapper adds its
// location without losing the typed operator error or moving its repair line.
func TestParseFirewallConfigPrefixErrorContext(t *testing.T) {
	for _, leaf := range []string{"source-address", "destination-address"} {
		t.Run(leaf, func(t *testing.T) {
			_, err := ParseFirewallConfig(prefixConfigInput(leaf, "10.10.10.1/30"))
			if err == nil {
				t.Fatal("ParseFirewallConfig accepted a prefix with host bits")
			}
			var operator *opererr.Error
			if !errors.As(err, &operator) {
				t.Fatalf("wrapped error %v lost its operator error", err)
			}
			for _, part := range []string{`table "wan"`, `chain "input"`, `term "allow-service"`, "10.10.10.1/30"} {
				if !strings.Contains(err.Error(), part) {
					t.Errorf("error %q does not name %q", err.Error(), part)
				}
			}
			wantLast := "fix: " + leaf + " 10.10.10.0/30"
			lines := strings.Split(err.Error(), "\n")
			if got := lines[len(lines)-1]; got != wantLast {
				t.Errorf("last line = %q, want %q", got, wantLast)
			}
		})
	}
}

// TestParseFirewallConfigCanonicalPrefix proves the contextual test input is
// valid and preserves the literal through the full table-to-term parser.
func TestParseFirewallConfigCanonicalPrefix(t *testing.T) {
	for _, leaf := range []string{"source-address", "destination-address"} {
		t.Run(leaf, func(t *testing.T) {
			tables, err := ParseFirewallConfig(prefixConfigInput(leaf, "10.10.10.0/30"))
			if err != nil {
				t.Fatalf("canonical contextual config refused: %v", err)
			}
			if len(tables) != 1 {
				t.Fatalf("table count = %d, want 1", len(tables))
			}
			if len(tables[0].Chains) != 1 {
				t.Fatalf("chain count = %d, want 1", len(tables[0].Chains))
			}
			if len(tables[0].Chains[0].Terms) != 1 {
				t.Fatalf("term count = %d, want 1", len(tables[0].Chains[0].Terms))
			}
			if len(tables[0].Chains[0].Terms[0].Matches) != 1 {
				t.Errorf("match count = %d, want 1", len(tables[0].Chains[0].Terms[0].Matches))
			}
		})
	}
}

// TestVerifyConfigRejectsPrefixHostBits reaches the registered offline verifier;
// the refusal must arise before backend capability or application is needed.
func TestVerifyConfigRejectsPrefixHostBits(t *testing.T) {
	for _, leaf := range []string{"source-address", "destination-address"} {
		for _, value := range []string{"10.10.10.1/30", "2001:db8:10:10::1/126"} {
			t.Run(leaf+"/"+value, func(t *testing.T) {
				sections := []sdk.ConfigSection{{Root: configRootFirewall, Data: prefixConfigInput(leaf, value)}}
				err := VerifyConfig(sections)
				if err == nil {
					t.Fatal("VerifyConfig accepted a prefix with host bits")
				}
				var operator *opererr.Error
				if !errors.As(err, &operator) {
					t.Fatalf("VerifyConfig returned a different error: %v", err)
				}
				if !strings.Contains(operator.Problem(), value) {
					t.Errorf("problem %q does not name rejected prefix %q", operator.Problem(), value)
				}
			})
		}
	}
}

// prefixConfigInput returns a minimal valid table around one literal leaf.
// Its arguments are fixed test inputs, not operator-supplied JSON fragments.
func prefixConfigInput(leaf, value string) string {
	return fmt.Sprintf(`{"firewall":{"backend":"nft","table":{"wan":{"family":"inet","chain":{"input":{"term":{"allow-service":{"from":{%q:%q},"then":{"accept":""}}}}}}}}}`, leaf, value)
}
