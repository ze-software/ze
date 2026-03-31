package lg

import (
	"testing"
)

// testServer returns a minimal LGServer for unit tests (no decorator).
func testServer() *LGServer {
	return &LGServer{}
}

// testServerWithDecorator returns an LGServer with an ASN decorator for testing.
func testServerWithDecorator(fn ASNDecorator) *LGServer {
	return &LGServer{decorateASN: fn}
}

func TestExtractPeers(t *testing.T) {
	// VALIDATES: peer data extraction from engine JSON.
	// PREVENTS: nil panic or missing fields in template data.
	srv := testServer()

	t.Run("nil input", func(t *testing.T) {
		if got := srv.extractPeers(nil); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("empty peers array", func(t *testing.T) {
		ze := map[string]any{"peers": []any{}}
		got := srv.extractPeers(ze)
		if len(got) != 0 {
			t.Errorf("expected 0 peers, got %d", len(got))
		}
	})

	t.Run("valid peer", func(t *testing.T) {
		ze := map[string]any{
			"peers": []any{
				map[string]any{
					"peer-address":    "10.0.0.1",
					"remote-as":       "65001",
					"state":           "established",
					"uptime":          "3600",
					"routes-received": "100",
					"routes-accepted": "95",
					"routes-sent":     "50",
					"description":     "test",
					"name":            "peer1",
				},
			},
		}

		peers := srv.extractPeers(ze)
		if len(peers) != 1 {
			t.Fatalf("expected 1 peer, got %d", len(peers))
		}

		p := peers[0]
		checks := map[string]string{
			"Address":        "10.0.0.1",
			"RemoteAS":       "65001",
			"State":          "established",
			"Uptime":         "3600",
			"RoutesReceived": "100",
			"RoutesAccepted": "95",
			"RoutesSent":     "50",
			"Description":    "test",
			"Name":           "peer1",
		}
		for key, want := range checks {
			got, _ := p[key].(string)
			if got != want {
				t.Errorf("peer[%q] = %q, want %q", key, got, want)
			}
		}
	})

	t.Run("summary envelope", func(t *testing.T) {
		// Real summary handler returns {"summary": {"peers": [...]}}.
		ze := map[string]any{
			"summary": map[string]any{
				"router-id":         "10.0.0.1",
				"local-as":          float64(65000),
				"peers-configured":  float64(1),
				"peers-established": float64(1),
				"peers": []any{
					map[string]any{
						"address":   "10.0.0.2",
						"remote-as": float64(65001),
						"state":     "established",
						"uptime":    "1h0m0s",
						"name":      "peer1",
					},
				},
			},
		}
		peers := srv.extractPeers(ze)
		if len(peers) != 1 {
			t.Fatalf("expected 1 peer, got %d", len(peers))
		}
		if peers[0]["Address"] != "10.0.0.2" {
			t.Errorf("Address = %q, want 10.0.0.2", peers[0]["Address"])
		}
		if peers[0]["Name"] != "peer1" {
			t.Errorf("Name = %q, want peer1", peers[0]["Name"])
		}
	})

	t.Run("non-map entry skipped", func(t *testing.T) {
		ze := map[string]any{
			"peers": []any{"not-a-map", map[string]any{"peer-address": "10.0.0.1"}},
		}
		peers := srv.extractPeers(ze)
		if len(peers) != 1 {
			t.Errorf("expected 1 peer (skip non-map), got %d", len(peers))
		}
	})
}

func TestExtractPeersWithASNNames(t *testing.T) {
	// VALIDATES: AC-2 -- ASN names populated from decorator.
	// PREVENTS: empty RemoteASName when decorator is wired.
	srv := testServerWithDecorator(func(asn string) string {
		if asn == "65001" {
			return "Test Org"
		}
		return ""
	})

	ze := map[string]any{
		"peers": []any{
			map[string]any{
				"peer-address": "10.0.0.1",
				"remote-as":    "65001",
				"state":        "established",
			},
		},
	}

	peers := srv.extractPeers(ze)
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}

	name, _ := peers[0]["RemoteASName"].(string)
	if name != "Test Org" {
		t.Errorf("RemoteASName = %q, want %q", name, "Test Org")
	}
}

func TestExtractRoutes(t *testing.T) {
	// VALIDATES: route extraction with routes/prefixes/adj-rib-in formats.
	t.Run("nil input", func(t *testing.T) {
		if got := extractRoutes(nil); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("routes key", func(t *testing.T) {
		ze := map[string]any{"routes": []any{"r1", "r2"}}
		got := extractRoutes(ze)
		if len(got) != 2 {
			t.Errorf("expected 2 routes, got %d", len(got))
		}
	})

	t.Run("prefixes fallback", func(t *testing.T) {
		ze := map[string]any{"prefixes": []any{"p1"}}
		got := extractRoutes(ze)
		if len(got) != 1 {
			t.Errorf("expected 1 route from prefixes fallback, got %d", len(got))
		}
	})

	t.Run("adj-rib-in format", func(t *testing.T) {
		ze := map[string]any{
			"adj-rib-in": map[string]any{
				"10.0.0.1": []any{
					map[string]any{"prefix": "192.168.0.0/24", "family": "ipv4/unicast"},
					map[string]any{"prefix": "192.168.1.0/24", "family": "ipv4/unicast"},
				},
			},
		}
		got := extractRoutes(ze)
		if len(got) != 2 {
			t.Fatalf("expected 2 routes, got %d", len(got))
		}
		r0, _ := got[0].(map[string]any)
		if r0["peer-address"] != "10.0.0.1" {
			t.Errorf("peer-address = %q, want 10.0.0.1", r0["peer-address"])
		}
	})

	t.Run("adj-rib-in and adj-rib-out combined", func(t *testing.T) {
		ze := map[string]any{
			"adj-rib-in": map[string]any{
				"10.0.0.1": []any{
					map[string]any{"prefix": "192.168.0.0/24"},
				},
			},
			"adj-rib-out": map[string]any{
				"10.0.0.1": []any{
					map[string]any{"prefix": "10.0.0.0/8"},
				},
			},
		}
		got := extractRoutes(ze)
		if len(got) != 2 {
			t.Errorf("expected 2 routes (in + out), got %d", len(got))
		}
	})

	t.Run("adj-rib-in preserves existing peer-address", func(t *testing.T) {
		ze := map[string]any{
			"adj-rib-in": map[string]any{
				"10.0.0.1": []any{
					map[string]any{"prefix": "192.168.0.0/24", "peer-address": "10.0.0.1"},
				},
			},
		}
		got := extractRoutes(ze)
		if len(got) != 1 {
			t.Fatalf("expected 1 route, got %d", len(got))
		}
		r0, _ := got[0].(map[string]any)
		if r0["peer-address"] != "10.0.0.1" {
			t.Errorf("peer-address = %q, want 10.0.0.1", r0["peer-address"])
		}
	})
}

func TestFindPeer(t *testing.T) {
	// VALIDATES: peer lookup by address and name.
	ze := map[string]any{
		"peers": []any{
			map[string]any{"peer-address": "10.0.0.1", "name": "peer1"},
			map[string]any{"peer-address": "10.0.0.2", "name": "peer2"},
		},
	}

	t.Run("nil input", func(t *testing.T) {
		if got := findPeer(nil, "10.0.0.1"); got != nil {
			t.Error("expected nil for nil input")
		}
	})

	t.Run("find by address", func(t *testing.T) {
		got := findPeer(ze, "10.0.0.2")
		if got == nil || getStr(got, "name") != "peer2" {
			t.Error("expected peer2")
		}
	})

	t.Run("find by name", func(t *testing.T) {
		got := findPeer(ze, "peer1")
		if got == nil || getStr(got, "peer-address") != "10.0.0.1" {
			t.Error("expected 10.0.0.1")
		}
	})

	t.Run("not found", func(t *testing.T) {
		if got := findPeer(ze, "nonexistent"); got != nil {
			t.Error("expected nil for missing peer")
		}
	})

	t.Run("summary envelope with address field", func(t *testing.T) {
		ze := map[string]any{
			"summary": map[string]any{
				"peers": []any{
					map[string]any{"address": "10.0.0.3", "name": "peer3"},
				},
			},
		}
		got := findPeer(ze, "10.0.0.3")
		if got == nil || getStr(got, "name") != "peer3" {
			t.Error("expected peer3")
		}
	})
}

func TestEngineError(t *testing.T) {
	// VALIDATES: error message for nil vs non-nil engine data.
	if got := engineError(nil); got != "BGP engine unavailable" {
		t.Errorf("nil: got %q, want BGP engine unavailable", got)
	}
	if got := engineError(map[string]any{}); got != "" {
		t.Errorf("non-nil: got %q, want empty", got)
	}
}

func TestResolveASN(t *testing.T) {
	// VALIDATES: resolveASN handles nil decorator and empty input.
	t.Run("nil decorator", func(t *testing.T) {
		srv := testServer()
		if got := srv.resolveASN("65001"); got != "" {
			t.Errorf("nil decorator: got %q, want empty", got)
		}
	})

	t.Run("empty asn", func(t *testing.T) {
		srv := testServerWithDecorator(func(asn string) string { return "should not be called" })
		if got := srv.resolveASN(""); got != "" {
			t.Errorf("empty asn: got %q, want empty", got)
		}
	})

	t.Run("decorator returns name", func(t *testing.T) {
		srv := testServerWithDecorator(func(asn string) string {
			if asn == "65001" {
				return "Acme Corp"
			}
			return ""
		})
		if got := srv.resolveASN("65001"); got != "Acme Corp" {
			t.Errorf("got %q, want %q", got, "Acme Corp")
		}
	})

	t.Run("decorator returns empty", func(t *testing.T) {
		srv := testServerWithDecorator(func(string) string { return "" })
		if got := srv.resolveASN("99999"); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestDecorateGraphNodes(t *testing.T) {
	// VALIDATES: AC-6 -- graph node names populated from decorator.
	// PREVENTS: empty node labels when decorator is wired.
	srv := testServerWithDecorator(func(asn string) string {
		if asn == "65001" {
			return "Acme Corp"
		}
		return ""
	})

	g := &Graph{
		Nodes: []GraphNode{
			{ASN: 65001, Layer: 1},
			{ASN: 65002, Layer: 0},
		},
	}

	srv.decorateGraphNodes(g)

	if g.Nodes[0].Name != "Acme Corp" {
		t.Errorf("Node[0].Name = %q, want %q", g.Nodes[0].Name, "Acme Corp")
	}
	if g.Nodes[1].Name != "" {
		t.Errorf("Node[1].Name = %q, want empty", g.Nodes[1].Name)
	}
}

func TestExtractPeersSortOrder(t *testing.T) {
	// VALIDATES: peers sorted by IP address numerically, not lexicographically.
	srv := testServer()
	ze := map[string]any{
		"peers": []any{
			map[string]any{"peer-address": "10.0.0.10", "state": "established"},
			map[string]any{"peer-address": "10.0.0.2", "state": "established"},
			map[string]any{"peer-address": "10.0.0.1", "state": "established"},
		},
	}

	peers := srv.extractPeers(ze)
	if len(peers) != 3 {
		t.Fatalf("expected 3 peers, got %d", len(peers))
	}

	want := []string{"10.0.0.1", "10.0.0.2", "10.0.0.10"}
	for i, w := range want {
		got, _ := peers[i]["Address"].(string)
		if got != w {
			t.Errorf("peers[%d].Address = %q, want %q", i, got, w)
		}
	}
}

func TestIsBestTemplateFunc(t *testing.T) {
	// VALIDATES: AC-8 -- isBest correctly reads "best" field from route map.
	tests := []struct {
		name string
		in   any
		want bool
	}{
		{"best true", map[string]any{"best": true, "prefix": "10.0.0.0/24"}, true},
		{"best false", map[string]any{"best": false, "prefix": "10.0.0.0/24"}, false},
		{"missing best", map[string]any{"prefix": "10.0.0.0/24"}, false},
		{"nil input", nil, false},
		{"non-map", "not-a-map", false},
	}

	fn, ok := lgFuncMap["isBest"].(func(any) bool)
	if !ok {
		t.Fatal("isBest not found in lgFuncMap")
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fn(tt.in); got != tt.want {
				t.Errorf("isBest(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
