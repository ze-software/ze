package filter_irr

import (
	"testing"
)

// VALIDATES: AC-8 -- explicit AS-SET parsed from config.
// VALIDATES: AC-14 -- irr enable disable parsed.
// VALIDATES: AC-19 -- refresh-interval parsed with default 3600.
// PREVENTS: Config parsing silently ignores IRR settings.

func TestParseIRRConfig(t *testing.T) {
	tests := []struct {
		name       string
		bgpCfg     map[string]any
		wantSrv    string
		wantPDBURL string
		wantInt    uint32
	}{
		{
			"defaults",
			map[string]any{},
			"whois.radb.net", "https://www.peeringdb.com", 3600,
		},
		{
			"explicit server and interval",
			map[string]any{
				"policy": map[string]any{
					"irr": map[string]any{
						"server":           "whois.ripe.net",
						"refresh-interval": "1800",
					},
				},
			},
			"whois.ripe.net", "https://www.peeringdb.com", 1800,
		},
		{
			"explicit peeringdb-url",
			map[string]any{
				"policy": map[string]any{
					"irr": map[string]any{
						"peeringdb-url": "http://127.0.0.1:9999",
					},
				},
			},
			"whois.radb.net", "http://127.0.0.1:9999", 3600,
		},
		{
			"interval as float64",
			map[string]any{
				"policy": map[string]any{
					"irr": map[string]any{
						"refresh-interval": float64(7200),
					},
				},
			},
			"whois.radb.net", "https://www.peeringdb.com", 7200,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := parseIRRConfig(tt.bgpCfg)
			if cfg.Server != tt.wantSrv {
				t.Errorf("Server = %q, want %q", cfg.Server, tt.wantSrv)
			}
			if cfg.PeeringDBURL != tt.wantPDBURL {
				t.Errorf("PeeringDBURL = %q, want %q", cfg.PeeringDBURL, tt.wantPDBURL)
			}
			if cfg.RefreshInterval != tt.wantInt {
				t.Errorf("RefreshInterval = %d, want %d", cfg.RefreshInterval, tt.wantInt)
			}
		})
	}
}

func TestParseIRRConfigPeerEntries(t *testing.T) {
	bgpCfg := map[string]any{
		"peer": map[string]any{
			"10.0.0.1": map[string]any{
				"session": map[string]any{
					"asn": map[string]any{
						"remote": "65001",
					},
					"irr": map[string]any{
						"as-set": "AS-CUSTOMER1",
					},
				},
			},
			"10.0.0.2": map[string]any{
				"session": map[string]any{
					"asn": map[string]any{
						"remote": "65002",
					},
				},
			},
		},
	}

	cfg := parseIRRConfig(bgpCfg)

	if len(cfg.Peers) != 2 {
		t.Fatalf("Peers count = %d, want 2", len(cfg.Peers))
	}

	var found65001, found65002 bool
	for _, p := range cfg.Peers {
		switch p.RemoteASN {
		case 65001:
			found65001 = true
			if p.ASSet != "AS-CUSTOMER1" {
				t.Errorf("peer 65001 ASSet = %q, want AS-CUSTOMER1", p.ASSet)
			}
		case 65002:
			found65002 = true
			if p.ASSet != "" {
				t.Errorf("peer 65002 ASSet = %q, want empty (auto-discover)", p.ASSet)
			}
		}
	}
	if !found65001 {
		t.Error("peer with ASN 65001 not found")
	}
	if !found65002 {
		t.Error("peer with ASN 65002 not found")
	}
}

// VALIDATES: AC-14 -- peer with irr enable disable is excluded.
func TestParseIRRConfigDisabledPeer(t *testing.T) {
	bgpCfg := map[string]any{
		"peer": map[string]any{
			"10.0.0.1": map[string]any{
				"session": map[string]any{
					"asn": map[string]any{"remote": "65001"},
					"irr": map[string]any{"enable": "disable"},
				},
			},
			"10.0.0.2": map[string]any{
				"session": map[string]any{
					"asn": map[string]any{"remote": "65002"},
				},
			},
		},
	}

	cfg := parseIRRConfig(bgpCfg)

	for _, p := range cfg.Peers {
		if p.RemoteASN == 65001 && !p.Disabled {
			t.Error("peer 65001 should be marked Disabled")
		}
		if p.RemoteASN == 65002 && p.Disabled {
			t.Error("peer 65002 should not be Disabled")
		}
	}
}

// VALIDATES: refresh-interval boundary: 60-86400.
func TestParseIRRConfigBoundary(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  uint32
	}{
		{"minimum valid", "60", 60},
		{"maximum valid", "86400", 86400},
		{"below minimum clamps", "30", 60},
		{"above maximum clamps", "100000", 86400},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bgpCfg := map[string]any{
				"policy": map[string]any{
					"irr": map[string]any{
						"refresh-interval": tt.value,
					},
				},
			}
			cfg := parseIRRConfig(bgpCfg)
			if cfg.RefreshInterval != tt.want {
				t.Errorf("RefreshInterval = %d, want %d", cfg.RefreshInterval, tt.want)
			}
		})
	}
}

// VALIDATES: a peer as-set that fails IRR name validation is dropped at parse
// (left empty), not stored -- defense-in-depth against malformed/injected names.
func TestParsePeerIRRRejectsBadASSet(t *testing.T) {
	bgpCfg := map[string]any{
		"peer": map[string]any{
			"10.0.0.1": map[string]any{
				"session": map[string]any{
					"asn": map[string]any{"remote": "65001"},
					"irr": map[string]any{"as-set": "AS-OK"},
				},
			},
			"10.0.0.2": map[string]any{
				"session": map[string]any{
					"asn": map[string]any{"remote": "65002"},
					"irr": map[string]any{"as-set": "AS BAD WITH SPACE"},
				},
			},
		},
	}
	got := map[uint32]string{}
	for _, p := range parseIRRConfig(bgpCfg).Peers {
		got[p.RemoteASN] = p.ASSet
	}
	if got[65001] != "AS-OK" {
		t.Errorf("AS65001 as-set = %q, want AS-OK", got[65001])
	}
	if got[65002] != "" {
		t.Errorf("AS65002 as-set = %q, want empty (malformed rejected)", got[65002])
	}
}
