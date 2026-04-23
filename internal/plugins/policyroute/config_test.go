package policyroute

import (
	"net/netip"
	"strings"
	"testing"
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
