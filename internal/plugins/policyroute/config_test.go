package policyroute

import (
	"math"
	"net/netip"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
)

func TestParsePolicyConfig(t *testing.T) {
	input := `{
		"policy": {
			"route": {
				"surfprotect": {
					"interface": "l2tp*",
					"rule": {
						"bypass-dst": {
							"from": {
								"destination-port": "80,443",
								"protocol": "tcp"
							},
							"then": { "accept": "" }
						}
					}
				}
			}
		}
	}`

	policies, err := parsePolicyConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}
	p := policies[0]
	if p.Name != "surfprotect" {
		t.Errorf("name = %q, want surfprotect", p.Name)
	}
	if len(p.Interfaces) != 1 || p.Interfaces[0].Name != "l2tp" || !p.Interfaces[0].Wildcard {
		t.Errorf("interface = %+v, want l2tp*", p.Interfaces)
	}
	if len(p.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(p.Rules))
	}
	if p.Rules[0].Action.Type != ActionAccept {
		t.Errorf("action type = %d, want ActionAccept", p.Rules[0].Action.Type)
	}
}

func TestParsePolicyConfigTable(t *testing.T) {
	input := `{
		"policy": {
			"route": {
				"test": {
					"interface": "eth0",
					"rule": {
						"r1": {
							"from": { "protocol": "tcp" },
							"then": { "table": "100" }
						}
					}
				}
			}
		}
	}`

	policies, err := parsePolicyConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rule := policies[0].Rules[0]
	if rule.Action.Type != ActionTable {
		t.Errorf("action type = %d, want ActionTable", rule.Action.Type)
	}
	if rule.Action.Table != 100 {
		t.Errorf("table = %d, want 100", rule.Action.Table)
	}
}

func TestParsePolicyConfigTCPMSS(t *testing.T) {
	input := `{
		"policy": {
			"route": {
				"test": {
					"interface": "eth0",
					"rule": {
						"r1": {
							"from": { "protocol": "tcp" },
							"then": { "table": "100", "tcp-mss": "1436" }
						}
					}
				}
			}
		}
	}`

	policies, err := parsePolicyConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rule := policies[0].Rules[0]
	if rule.Action.TCPMSS != 1436 {
		t.Errorf("tcp-mss = %d, want 1436", rule.Action.TCPMSS)
	}
}

func TestParsePolicyConfigNextHop(t *testing.T) {
	input := `{
		"policy": {
			"route": {
				"test": {
					"interface": "eth0",
					"rule": {
						"r1": {
							"from": { "protocol": "tcp" },
							"then": { "next-hop": "10.0.0.1" }
						}
					}
				}
			}
		}
	}`

	policies, err := parsePolicyConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rule := policies[0].Rules[0]
	if rule.Action.Type != ActionNextHop {
		t.Errorf("action type = %d, want ActionNextHop", rule.Action.Type)
	}
	want := netip.MustParseAddr("10.0.0.1")
	if rule.Action.NextHop != want {
		t.Errorf("next-hop = %s, want %s", rule.Action.NextHop, want)
	}
}

func TestParsePolicyConfigRejectReservedTable(t *testing.T) {
	for _, tbl := range []string{"1000", "1500", "2000", "2999"} {
		input := `{
			"policy": {
				"route": {
					"test": {
						"interface": "eth0",
						"rule": {
							"r1": {
								"from": { "protocol": "tcp" },
								"then": { "table": "` + tbl + `" }
							}
						}
					}
				}
			}
		}`

		_, err := parsePolicyConfig(input)
		if err == nil {
			t.Errorf("table %s: expected error for reserved range, got nil", tbl)
		}
	}
}

func TestParsePolicyConfigRejectKernelTable(t *testing.T) {
	for _, tbl := range []string{"253", "254", "255"} {
		input := `{
			"policy": {
				"route": {
					"test": {
						"interface": "eth0",
						"rule": {
							"r1": {
								"from": { "protocol": "tcp" },
								"then": { "table": "` + tbl + `" }
							}
						}
					}
				}
			}
		}`

		_, err := parsePolicyConfig(input)
		if err == nil {
			t.Errorf("table %s: expected error for kernel system table, got nil", tbl)
		}
	}
}

func TestParsePolicyConfigConflictingActions(t *testing.T) {
	tests := []struct {
		name string
		then string
	}{
		{"table+accept", `"table": "100", "accept": ""`},
		{"table+drop", `"table": "100", "drop": ""`},
		{"table+next-hop", `"table": "100", "next-hop": "10.0.0.1"`},
		{"accept+drop", `"accept": "", "drop": ""`},
		{"next-hop+accept", `"next-hop": "10.0.0.1", "accept": ""`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := `{
				"policy": {
					"route": {
						"test": {
							"interface": "eth0",
							"rule": {
								"r1": {
									"from": { "protocol": "tcp" },
									"then": { ` + tt.then + ` }
								}
							}
						}
					}
				}
			}`

			_, err := parsePolicyConfig(input)
			if err == nil {
				t.Errorf("expected conflict error for %s, got nil", tt.name)
			}
			if err != nil && !strings.Contains(err.Error(), "conflicting") {
				t.Errorf("expected 'conflicting' in error, got: %v", err)
			}
		})
	}
}

func TestParsePolicyConfigRuleOrder(t *testing.T) {
	input := `{
		"policy": {
			"route": {
				"test": {
					"interface": "eth0",
					"rule": {
						"last": {
							"order": "30",
							"from": { "protocol": "udp" },
							"then": { "drop": "" }
						},
						"first": {
							"order": "10",
							"from": { "protocol": "tcp" },
							"then": { "accept": "" }
						},
						"middle": {
							"order": "20",
							"from": { "protocol": "icmp" },
							"then": { "drop": "" }
						}
					}
				}
			}
		}
	}`

	policies, err := parsePolicyConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rules := policies[0].Rules
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}
	if rules[0].Name != "first" || rules[1].Name != "middle" || rules[2].Name != "last" {
		t.Errorf("order wrong: got %s, %s, %s", rules[0].Name, rules[1].Name, rules[2].Name)
	}
}

func TestParsePolicyConfigRuleOrderTiebreakByName(t *testing.T) {
	input := `{
		"policy": {
			"route": {
				"test": {
					"interface": "eth0",
					"rule": {
						"bravo": {
							"from": { "protocol": "tcp" },
							"then": { "accept": "" }
						},
						"alpha": {
							"from": { "protocol": "udp" },
							"then": { "drop": "" }
						}
					}
				}
			}
		}
	}`

	policies, err := parsePolicyConfig(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	rules := policies[0].Rules
	if rules[0].Name != "alpha" || rules[1].Name != "bravo" {
		t.Errorf("tiebreak wrong: got %s, %s", rules[0].Name, rules[1].Name)
	}
}

// VALIDATES: validateActionTable rejects a table value that does not survive
// the conversion to Go int that netlink.Rule.Table performs, while still
// accepting every value at or below the bound it is given.
// PREVENTS: `then table <N>` above MaxInt32 on a 32-bit build silently losing
// its table selection. newIPRule converts to int (rules_linux.go) and the
// encoder emits FRA_TABLE only for Table >= 256 and the compat byte only for
// 0 <= Table < 256 (vendor/github.com/vishvananda/netlink/rule_linux.go:57,126),
// so a negative Table installs a rule with RT_TABLE_UNSPEC and the operator's
// steering is dropped without any error.
func TestValidateActionTableRejectsUnencodable(t *testing.T) {
	// The bound is passed explicitly so a case can pin either side of it
	// without restating maxEncodableTable.
	const maxInt32 = uint64(math.MaxInt32)

	tests := []struct {
		name       string
		tbl        uint64
		maxEncode  uint64
		wantReject bool
	}{
		{"below bound", 4000, maxInt32, false},
		{"at bound", math.MaxInt32, maxInt32, false},
		{"one above bound", math.MaxInt32 + 1, maxInt32, true},
		{"max uint32 on 32-bit", math.MaxUint32, maxInt32, true},
		{"max uint32 under a wider bound", math.MaxUint32, uint64(math.MaxInt), false},
		{"reserved range still rejected", 2000, maxInt32, true},
		{"kernel table still rejected", 254, maxInt32, true},
		{"zero still rejected", 0, maxInt32, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateActionTable(tt.tbl, tt.maxEncode)
			if tt.wantReject && err == nil {
				t.Fatalf("validateActionTable(%d, %d): accepted, want rejection", tt.tbl, tt.maxEncode)
			}
			if !tt.wantReject && err != nil {
				t.Fatalf("validateActionTable(%d, %d): unexpected error: %v", tt.tbl, tt.maxEncode, err)
			}
		})
	}
}

// VALIDATES: the bound parsePolicyAction applies is this build's own int limit,
// so a table value is accepted exactly when it survives the int conversion.
// PREVENTS: the parser being wired to a hardcoded 32-bit bound, which would
// reject kernel-legal table IDs on the 64-bit targets Ze ships. That narrowing
// was shipped once to quiet CodeQL alert 171 and reverted: the int conversion
// is answered where it happens (netlinkTableInt), not by refusing config.
func TestParsePolicyConfigTableBoundMatchesBuild(t *testing.T) {
	for _, tbl := range []uint64{4000, math.MaxInt32, math.MaxInt32 + 1, math.MaxUint32} {
		input := `{
			"policy": {
				"route": {
					"test": {
						"interface": "eth0",
						"rule": {
							"r1": {
								"from": { "protocol": "tcp" },
								"then": { "table": "` + strconv.FormatUint(tbl, 10) + `" }
							}
						}
					}
				}
			}
		}`

		_, err := parsePolicyConfig(input)
		fits := tbl <= uint64(math.MaxInt)
		if fits != (err == nil) {
			t.Errorf("table %d: fits in int = %v, accepted = %v (err=%v)", tbl, fits, err == nil, err)
		}
	}
}

func TestParsePolicyConfigAllowNonReservedTable(t *testing.T) {
	for _, tbl := range []string{"1", "100", "999", "3000", "4000"} {
		input := `{
			"policy": {
				"route": {
					"test": {
						"interface": "eth0",
						"rule": {
							"r1": {
								"from": { "protocol": "tcp" },
								"then": { "table": "` + tbl + `" }
							}
						}
					}
				}
			}
		}`

		_, err := parsePolicyConfig(input)
		if err != nil {
			t.Errorf("table %s: unexpected error: %v", tbl, err)
		}
	}
}

func policyConfigWithProtocol(proto string) string {
	return `{
		"policy": {
			"route": {
				"test": {
					"interface": "eth0",
					"rule": {
						"r1": {
							"from": { "protocol": "` + proto + `" },
							"then": { "table": "100" }
						}
					}
				}
			}
		}
	}`
}

// TestPolicyRouteProtocolRejectsUnknownName closes the config-side instance of
// the protocol-name drift.
//
// VALIDATES: a protocol spelling outside the canonical firewall table is
// refused at parse time, and the error names the leaf and the accepted values.
// PREVENTS: an operator committing a protocol that only fails hours later, at
// reconcile time, inside the nft backend -- where the failure is not local to
// the offending rule, because Apply returns before its single Flush and leaves
// every firewall owner's ruleset unapplied.
func TestPolicyRouteProtocolRejectsUnknownName(t *testing.T) {
	for _, proto := range []string{"TCP", "132", "igmp", "bogus", "tcp6"} {
		_, err := parsePolicyConfig(policyConfigWithProtocol(proto))
		if err == nil {
			t.Errorf("protocol %q: expected the commit to be refused, got nil", proto)
			continue
		}
		for _, want := range []string{"protocol", "tcp"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("protocol %q: error %q must name %q", proto, err, want)
			}
		}
	}
}

// TestPolicyRouteProtocolAcceptsEveryCanonicalName is the other half: the leaf
// must accept everything the backends can lower.
//
// VALIDATES: every name firewall.ProtocolNames returns is accepted.
// PREVENTS: the validator becoming a second, narrower protocol table.
func TestPolicyRouteProtocolAcceptsEveryCanonicalName(t *testing.T) {
	for _, name := range firewall.ProtocolNames() {
		routes, err := parsePolicyConfig(policyConfigWithProtocol(name))
		if err != nil {
			t.Fatalf("canonical protocol %q must be accepted: %v", name, err)
		}
		if got := routes[0].Rules[0].Match.Protocol; got != name {
			t.Errorf("protocol %q parsed as %q", name, got)
		}
	}
}

// TestPolicyRouteEmptyProtocolStaysOptional pins the leaf as optional.
//
// VALIDATES: a rule with no protocol leaf still parses.
// PREVENTS: the new validator turning an optional match into a required one.
func TestPolicyRouteEmptyProtocolStaysOptional(t *testing.T) {
	input := `{
		"policy": {
			"route": {
				"test": {
					"interface": "eth0",
					"rule": {
						"r1": {
							"from": { "destination-port": "80" },
							"then": { "table": "100" }
						}
					}
				}
			}
		}
	}`
	if _, err := parsePolicyConfig(input); err != nil {
		t.Fatalf("a rule with no protocol must parse: %v", err)
	}
}
