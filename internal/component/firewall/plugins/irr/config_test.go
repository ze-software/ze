package irr

// VALIDATES: AC-4 config with source-asn extracts IRR references
// VALIDATES: AC-7 destination-asn/destination-as-set handled
// PREVENTS: config parsing silently dropping ASN/AS-SET references

import (
	"encoding/json"
	"testing"

	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
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
