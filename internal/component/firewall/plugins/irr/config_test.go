package irr

// VALIDATES: AC-4 config with source-asn extracts IRR references
// VALIDATES: AC-7 destination-asn/destination-as-set handled
// PREVENTS: config parsing silently dropping ASN/AS-SET references

import (
	"encoding/json"
	"testing"

	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

func TestParseFirewallIRRConfig(t *testing.T) {
	data := `{
		"firewall": {
			"irr": {
				"server": "whois.radb.net",
				"refresh-interval": "0"
			},
			"table": {
				"wan": {
					"family": "inet",
					"chain": {
						"input": {
							"term": {
								"allow-cloudflare": {
									"from": {
										"source-asn": "13335"
									},
									"then": {
										"accept": {}
									}
								}
							}
						}
					}
				}
			}
		}
	}`
	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	refs := extractRefsFromConfig(root)
	if len(refs) == 0 {
		t.Fatal("expected at least one IRR reference")
	}
	found := false
	for _, ref := range refs {
		if ref.Name == "AS13335" && !ref.IsASSet && ref.TableName == "ze_wan" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected ASN ref for AS13335 in table ze_wan, got %v", refs)
	}
}

func TestExtractRefsDestinationASN(t *testing.T) {
	data := `{
		"firewall": {
			"table": {
				"wan": {
					"family": "inet",
					"chain": {
						"forward": {
							"term": {
								"block-bad": {
									"from": {
										"destination-asn": "64496"
									},
									"then": {
										"drop": {}
									}
								}
							}
						}
					}
				}
			}
		}
	}`
	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	refs := extractRefsFromConfig(root)
	found := false
	for _, ref := range refs {
		if ref.Name == "AS64496" && !ref.IsASSet {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected ASN ref for AS64496, got %v", refs)
	}
}

// VALIDATES: same ASN in multiple tables creates refs for each table.
// PREVENTS: cross-table dedup dropping refs, causing "unknown set" at apply.
func TestExtractRefsCrossTable(t *testing.T) {
	data := `{
		"firewall": {
			"table": {
				"wan": {
					"family": "inet",
					"chain": {
						"input": {
							"term": {
								"t1": {
									"from": { "source-asn": "13335" }
								}
							}
						}
					}
				},
				"dmz": {
					"family": "inet",
					"chain": {
						"input": {
							"term": {
								"t1": {
									"from": { "source-asn": "13335" }
								}
							}
						}
					}
				}
			}
		}
	}`
	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	refs := extractRefsFromConfig(root)
	if len(refs) != 2 {
		t.Fatalf("expected 2 refs (one per table), got %d: %v", len(refs), refs)
	}
	tables := make(map[string]bool)
	for _, ref := range refs {
		tables[ref.TableName] = true
	}
	if !tables["ze_wan"] || !tables["ze_dmz"] {
		t.Errorf("expected refs for ze_wan and ze_dmz, got tables %v", tables)
	}
}

func TestExtractRefsASSet(t *testing.T) {
	data := `{
		"firewall": {
			"table": {
				"wan": {
					"family": "inet",
					"chain": {
						"input": {
							"term": {
								"allow-set": {
									"from": {
										"source-as-set": "AS-CLOUDFLARE"
									},
									"then": {
										"accept": {}
									}
								}
							}
						}
					}
				}
			}
		}
	}`
	var root map[string]any
	if err := json.Unmarshal([]byte(data), &root); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	refs := extractRefsFromConfig(root)
	found := false
	for _, ref := range refs {
		if ref.Name == "AS-CLOUDFLARE" && ref.IsASSet {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected AS-SET ref for AS-CLOUDFLARE, got %v", refs)
	}
}

// VALIDATES: refresh-interval parsed from both string and float64.
// PREVENTS: numeric JSON value silently defaulting to 0.
func TestParseRefreshIntervalNumeric(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "firewall",
		Data: `{"firewall":{"irr":{"refresh-interval":3600}}}`,
	}}
	cfg := parseIRRConfig(sections)
	if cfg.RefreshInterval != 3600 {
		t.Errorf("refresh-interval from float64 = %d, want 3600", cfg.RefreshInterval)
	}
}

func TestParseRefreshIntervalString(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "firewall",
		Data: `{"firewall":{"irr":{"refresh-interval":"1800"}}}`,
	}}
	cfg := parseIRRConfig(sections)
	if cfg.RefreshInterval != 1800 {
		t.Errorf("refresh-interval from string = %d, want 1800", cfg.RefreshInterval)
	}
}

// VALIDATES: AC-1 interface binding parsed with source-as-set.
// PREVENTS: interface binding silently ignored during config parsing.
func TestParseInterfaceBinding(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "firewall",
		Data: `{"firewall":{"irr":{"interface":{"eth1":{"source-as-set":"AS-FOO"}}}}}`,
	}}
	cfg := parseIRRConfig(sections)
	if len(cfg.ifaceBindings) != 1 {
		t.Fatalf("expected 1 interface binding, got %d", len(cfg.ifaceBindings))
	}
	ib := cfg.ifaceBindings[0]
	if ib.Interface != "eth1" {
		t.Errorf("interface = %q, want eth1", ib.Interface)
	}
	if ib.ASSet != "AS-FOO" {
		t.Errorf("as-set = %q, want AS-FOO", ib.ASSet)
	}
}

// VALIDATES: AC-3 multiple interfaces with different AS-SETs parsed independently.
// PREVENTS: map iteration dropping bindings or conflating them.
func TestParseMultipleInterfaceBindings(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "firewall",
		Data: `{"firewall":{"irr":{"interface":{"eth1":{"source-as-set":"AS-FOO"},"eth2":{"source-as-set":"AS-BAR"}}}}}`,
	}}
	cfg := parseIRRConfig(sections)
	if len(cfg.ifaceBindings) != 2 {
		t.Fatalf("expected 2 interface bindings, got %d", len(cfg.ifaceBindings))
	}
	found := make(map[string]string)
	for _, ib := range cfg.ifaceBindings {
		found[ib.Interface] = ib.ASSet
	}
	if found["eth1"] != "AS-FOO" {
		t.Errorf("eth1 as-set = %q, want AS-FOO", found["eth1"])
	}
	if found["eth2"] != "AS-BAR" {
		t.Errorf("eth2 as-set = %q, want AS-BAR", found["eth2"])
	}
}

// VALIDATES: AC-1 interface binding refs included in allRefs for refresh/verify.
// PREVENTS: interface AS-SET names excluded from cache refresh cycle.
func TestAllRefsIncludesIfaceBindings(t *testing.T) {
	cfg := &irrConfig{
		refs: []irrRef{{Name: "AS13335", TableName: "ze_wan"}},
		ifaceBindings: []ifaceBinding{
			{Interface: "eth1", ASSet: "AS-FOO"},
		},
	}
	refs := cfg.allRefs()
	found := make(map[string]bool)
	for _, r := range refs {
		found[r.Name] = true
	}
	if !found["AS13335"] {
		t.Error("expected term ref AS13335 in allRefs")
	}
	if !found["AS-FOO"] {
		t.Error("expected iface ref AS-FOO in allRefs")
	}
}

// VALIDATES: interface bindings sorted by name for deterministic chain order.
// PREVENTS: non-deterministic map iteration causing unnecessary kernel churn.
func TestParseIfaceBindingsSorted(t *testing.T) {
	sections := []sdk.ConfigSection{{
		Root: "firewall",
		Data: `{"firewall":{"irr":{"interface":{"eth3":{"source-as-set":"AS-C"},"eth1":{"source-as-set":"AS-A"},"eth2":{"source-as-set":"AS-B"}}}}}`,
	}}
	cfg := parseIRRConfig(sections)
	if len(cfg.ifaceBindings) != 3 {
		t.Fatalf("expected 3 bindings, got %d", len(cfg.ifaceBindings))
	}
	for i := 1; i < len(cfg.ifaceBindings); i++ {
		if cfg.ifaceBindings[i].Interface < cfg.ifaceBindings[i-1].Interface {
			t.Errorf("bindings not sorted: %q before %q",
				cfg.ifaceBindings[i-1].Interface, cfg.ifaceBindings[i].Interface)
		}
	}
}

// VALIDATES: allRefs deduplicates when same AS-SET appears in both term and iface.
// PREVENTS: double refresh of same name wasting bandwidth.
func TestAllRefsDeduplicates(t *testing.T) {
	cfg := &irrConfig{
		refs: []irrRef{{Name: "AS-FOO", IsASSet: true, TableName: "ze_wan"}},
		ifaceBindings: []ifaceBinding{
			{Interface: "eth1", ASSet: "AS-FOO"},
		},
	}
	refs := cfg.allRefs()
	count := 0
	for _, r := range refs {
		if r.Name == "AS-FOO" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected AS-FOO once in allRefs, got %d", count)
	}
}
