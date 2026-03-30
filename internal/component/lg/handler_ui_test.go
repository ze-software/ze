package lg

import (
	"testing"
)

func TestExtractPeers(t *testing.T) {
	// VALIDATES: peer data extraction from engine JSON.
	// PREVENTS: nil panic or missing fields in template data.
	t.Run("nil input", func(t *testing.T) {
		if got := extractPeers(nil); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("empty peers array", func(t *testing.T) {
		ze := map[string]any{"peers": []any{}}
		got := extractPeers(ze)
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

		peers := extractPeers(ze)
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
		peers := extractPeers(ze)
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
		peers := extractPeers(ze)
		if len(peers) != 1 {
			t.Errorf("expected 1 peer (skip non-map), got %d", len(peers))
		}
	})
}

func TestExtractRoutes(t *testing.T) {
	// VALIDATES: route extraction with routes/prefixes fallback.
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
