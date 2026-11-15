package filter_irr

import (
	"maps"
	"testing"
)

// VALIDATES: AC-8 -- explicit AS-SET parsed from config.
// VALIDATES: AC-14 -- irr enable disable parsed.
// PREVENTS: Config parsing silently ignores IRR settings.
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

	cfg := parseIRRConfigOf(bgpCfg)

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

	cfg := parseIRRConfigOf(bgpCfg)

	for _, p := range cfg.Peers {
		if p.RemoteASN == 65001 && !p.Disabled {
			t.Error("peer 65001 should be marked Disabled")
		}
		if p.RemoteASN == 65002 && p.Disabled {
			t.Error("peer 65002 should not be Disabled")
		}
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
	for _, p := range parseIRRConfigOf(bgpCfg).Peers {
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
		return map[string]any{"filter": map[string]any{dir: map[string]any{"filters": anyRefs, "final": "accept"}}}
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
					"filter": map[string]any{"import": map[string]any{"filters": []any{"bgp-filter-irr:65001"}, "final": "accept"}},
					"peer":   map[string]any{"10.0.0.1": peerWith(nil)},
				},
			}},
			true,
		},
		{
			"global chain enrolls every peer",
			map[string]any{
				"filter": map[string]any{"import": map[string]any{"filters": []any{"bgp-filter-irr:65001"}, "final": "accept"}},
				"peer":   map[string]any{"10.0.0.1": peerWith(nil)},
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := parseIRRConfigOf(tt.bgpCfg)
			if len(cfg.Peers) != 1 {
				t.Fatalf("Peers = %d, want 1 (parse must still list the peer)", len(cfg.Peers))
			}
			if cfg.Peers[0].UsesIRR != tt.want {
				t.Errorf("UsesIRR = %v, want %v", cfg.Peers[0].UsesIRR, tt.want)
			}
		})
	}
}

// TestIRRPolicyFinalOnlyDoesNotEnroll checks that the core-owned final is not
// mistaken for a filter, while either final leaves real IRR references visible.
func TestIRRPolicyFinalOnlyDoesNotEnroll(t *testing.T) {
	for _, direction := range []string{"import", "export"} {
		for _, final := range []string{"accept", "reject"} {
			policy := map[string]any{"final": final}
			level := map[string]any{"filter": map[string]any{direction: policy}}
			if chainReferencesIRR(level) {
				t.Fatal("final-only policy enrolled IRR")
			}
			policy["filters"] = []string{"bgp-filter-irr:65001"}
			if !chainReferencesIRR(level) {
				t.Fatalf("%s final %s hid its IRR reference", direction, final)
			}
		}
	}
}

// TestIRRDynamicTemplateDoesNotInventAPeer parses a listen-range group with an
// IRR chain. The template has no remote ASN to resolve; a named member with its
// own ASN must still enroll through the inherited chain.
func TestIRRDynamicTemplateDoesNotInventAPeer(t *testing.T) {
	group := map[string]any{
		"connection": map[string]any{"remote": map[string]any{
			"ip": "dynamic", "range": []any{"192.0.2.0/24"},
		}},
		"filter": map[string]any{"import": map[string]any{
			"filters": []any{"bgp-filter-irr:65001"}, "final": "accept",
		}},
	}
	tree := map[string]any{"group": map[string]any{"ix": group}}
	if cfg := parseIRRConfigOf(tree); len(cfg.Peers) != 0 {
		t.Fatalf("peerless template invented IRR peers: %+v", cfg.Peers)
	}

	group["peer"] = map[string]any{"named": map[string]any{
		"connection": map[string]any{"remote": map[string]any{"ip": "192.0.2.7"}},
		"session": map[string]any{
			"asn": map[string]any{"remote": "65001"},
		},
	}}
	cfg := parseIRRConfigOf(tree)
	if len(cfg.Peers) != 1 {
		t.Fatalf("IRR peers = %+v, want only the named member", cfg.Peers)
	}
	peer := cfg.Peers[0]
	if peer.PeerAddr != "named" {
		t.Errorf("peer identity = %q, want named", peer.PeerAddr)
	}
	if peer.RemoteASN != 65001 {
		t.Errorf("remote ASN = %d, want 65001", peer.RemoteASN)
	}
	if peer.RemoteIP.String() != "192.0.2.7" {
		t.Errorf("remote IP = %v, want 192.0.2.7", peer.RemoteIP)
	}
	if peer.ASSet != "" {
		t.Errorf("AS set = %q, want automatic discovery from the named member's ASN", peer.ASSet)
	}
	if !peer.enrolled() {
		t.Error("the named member lost its IRR enrollment")
	}
}
