package filter_irr

import (
	"maps"
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

// VALIDATES: UsesIRR is set only for a peer that actually opted into IRR
// filtering -- a filter chain naming bgp-filter-irr at global, group, or peer
// level (import or export), or an explicit session.irr.as-set.
// PREVENTS: a peer that merely declares a remote ASN being enrolled, which made
// every BGP config issue an unsolicited PeeringDB + IRR whois lookup per peer at
// startup (see handleConfigure) and contradicted docs/guide/irr-filtering.md,
// which documents that a peer with no bgp-filter-irr chain reference has no
// `show bgp irr` entry.
func TestParseIRRConfigUsesIRR(t *testing.T) {
	peerWith := func(extra map[string]any) map[string]any {
		p := map[string]any{
			"session": map[string]any{"asn": map[string]any{"remote": "65001"}},
		}
		maps.Copy(p, extra)
		return p
	}
	chain := func(dir string, refs ...string) map[string]any {
		anyRefs := make([]any, len(refs))
		for i, r := range refs {
			anyRefs[i] = r
		}
		return map[string]any{"filter": map[string]any{dir: anyRefs}}
	}

	tests := []struct {
		name   string
		bgpCfg map[string]any
		want   bool
	}{
		{
			"bare peer with only a remote ASN is not enrolled",
			map[string]any{"peer": map[string]any{"10.0.0.1": peerWith(nil)}},
			false,
		},
		{
			"peer import chain names the plugin",
			map[string]any{"peer": map[string]any{
				"10.0.0.1": peerWith(chain("import", "bgp-filter-irr:65001")),
			}},
			true,
		},
		{
			"peer export chain names the plugin",
			map[string]any{"peer": map[string]any{
				"10.0.0.1": peerWith(chain("export", "bgp-filter-irr:65001")),
			}},
			true,
		},
		{
			"chain naming another plugin does not enroll",
			map[string]any{"peer": map[string]any{
				"10.0.0.1": peerWith(chain("import", "bgp-filter-prefix:customers")),
			}},
			false,
		},
		{
			"a bare ref cannot reach this plugin, so it does not enroll",
			map[string]any{"peer": map[string]any{
				"10.0.0.1": peerWith(chain("import", "bgp-filter-irr")),
			}},
			false,
		},
		{
			"explicit session as-set enrolls without a chain",
			map[string]any{"peer": map[string]any{
				"10.0.0.1": map[string]any{"session": map[string]any{
					"asn": map[string]any{"remote": "65001"},
					"irr": map[string]any{"as-set": "AS-CUSTOMER1"},
				}},
			}},
			true,
		},
		{
			"group chain enrolls its peers",
			map[string]any{"group": map[string]any{
				"customers": map[string]any{
					"filter": map[string]any{"import": []any{"bgp-filter-irr:65001"}},
					"peer":   map[string]any{"10.0.0.1": peerWith(nil)},
				},
			}},
			true,
		},
		{
			"global chain enrolls every peer",
			map[string]any{
				"filter": map[string]any{"import": []any{"bgp-filter-irr:65001"}},
				"peer":   map[string]any{"10.0.0.1": peerWith(nil)},
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := parseIRRConfig(tt.bgpCfg)
			if len(cfg.Peers) != 1 {
				t.Fatalf("Peers = %d, want 1 (parse must still list the peer)", len(cfg.Peers))
			}
			if cfg.Peers[0].UsesIRR != tt.want {
				t.Errorf("UsesIRR = %v, want %v", cfg.Peers[0].UsesIRR, tt.want)
			}
		})
	}
}
