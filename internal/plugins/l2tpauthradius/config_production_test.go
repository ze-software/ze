package l2tpauthradius

import (
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	schema "codeberg.org/thomas-mangin/ze/internal/plugins/l2tpauthradius/schema"
)

// TestParseConfigProductionTreeShape verifies the parser accepts the exact
// map shape produced by the real File -> Tree -> Tree.ToMap() pipeline, which
// is what both `ze doctor` (in-process verify) and runtime configure deliver.
//
// In that shape a YANG keyed list ("server <name> {...}") becomes a
// map[string]any keyed by the entry name, and all scalars are strings -- not
// the []any / float64 shape the hand-written unit tests use. A regression here
// (e.g. server delivered as map but read as a flat list) made `ze doctor`
// reject a valid config with "server entry missing address".
func TestParseConfigProductionTreeShape(t *testing.T) {
	cfg := `l2tp {
    enabled true
    auth {
        radius {
            server radius1 {
                address 127.0.0.1
                port 1812
                shared-key testing123
            }
            timeout 1
            retries 1
        }
    }
}`
	tree, err := config.ParseTreeWithYANG(cfg, map[string]string{
		"l2tp-auth-radius": schema.ZeL2TPAuthRadiusConfYANG,
	})
	if err != nil {
		t.Fatalf("parse tree: %v", err)
	}

	parsed, err := parseConfigFromTree(tree.ToMap())
	if err != nil {
		t.Fatalf("parseConfigFromTree on production shape: %v", err)
	}
	if len(parsed.Servers) != 1 {
		t.Fatalf("servers: got %d, want 1", len(parsed.Servers))
	}
	if parsed.Servers[0].Address != "127.0.0.1:1812" {
		t.Errorf("server address: got %q, want %q", parsed.Servers[0].Address, "127.0.0.1:1812")
	}
	if string(parsed.Servers[0].SharedKey) != "testing123" {
		t.Errorf("shared-key: got %q, want %q", parsed.Servers[0].SharedKey, "testing123")
	}
	if parsed.Timeout.Seconds() != 1 {
		t.Errorf("timeout: got %v, want 1s", parsed.Timeout)
	}
	if parsed.Retries != 1 {
		t.Errorf("retries: got %d, want 1", parsed.Retries)
	}
}

// TestParseConfigKeyedServerMap verifies the keyed-list map shape directly
// (multiple servers keyed by name, scalars as strings), independent of the
// config parser. Order must be deterministic (sorted by name) for predictable
// failover.
func TestParseConfigKeyedServerMap(t *testing.T) {
	tree := map[string]any{
		"l2tp": map[string]any{
			"auth": map[string]any{
				"radius": map[string]any{
					"timeout": "5",
					"server": map[string]any{
						"radius2": map[string]any{
							"address":    "10.0.0.2",
							"port":       "1813",
							"shared-key": "s2",
						},
						"radius1": map[string]any{
							"address":    "10.0.0.1",
							"shared-key": "s1",
						},
					},
				},
			},
		},
	}
	parsed, err := parseConfigFromTree(tree)
	if err != nil {
		t.Fatalf("parse keyed map: %v", err)
	}
	if len(parsed.Servers) != 2 {
		t.Fatalf("servers: got %d, want 2", len(parsed.Servers))
	}
	// Sorted by name: radius1 first, radius2 second.
	if parsed.Servers[0].Address != "10.0.0.1:1812" {
		t.Errorf("server[0] address: got %q, want %q", parsed.Servers[0].Address, "10.0.0.1:1812")
	}
	if parsed.Servers[1].Address != "10.0.0.2:1813" {
		t.Errorf("server[1] address: got %q, want %q", parsed.Servers[1].Address, "10.0.0.2:1813")
	}
	if parsed.Timeout.Seconds() != 5 {
		t.Errorf("timeout: got %v, want 5s", parsed.Timeout)
	}
}
