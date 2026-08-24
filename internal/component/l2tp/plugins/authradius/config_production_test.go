package l2tpauthradius

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/l2tp/plugins/authradius/yang"
	"github.com/ze-software/ze/internal/core/configorder"
)

// TestParseConfigProductionTreeShape verifies the parser accepts the exact
// map shape produced by the real File -> Tree -> Tree.ToPluginMap() pipeline, which
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
		"l2tp-auth-radius": yang.ZeL2TPAuthRadiusConfYANG,
	})
	if err != nil {
		t.Fatalf("parse tree: %v", err)
	}

	parsed, err := parseConfigFromTree(tree.ToPluginMap())
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

// TestParseConfigCoAPortProductionPath verifies `coa-port` survives the real
// File -> Tree -> Tree.ToPluginMap() pipeline and reaches Config.CoAPort.
//
// VALIDATES: an operator writing `coa-port 3799` in a real config file gets a
// Config whose CoAPort is 3799, which is the sole gate on the CoA listener
// (`cfg.CoAPort > 0`, register.go applyConfig).
// PREVENTS: the leaf being parsed by parseConfigFromTree (config.go:93-100) but
// absent from the YANG schema, so the production parser rejects the file with
// "unknown field in radius: coa-port" (config/parser.go) and the whole CoA
// listener branch is unreachable from production config.
func TestParseConfigCoAPortProductionPath(t *testing.T) {
	cfg := `l2tp {
    enabled true
    auth {
        radius {
            server radius1 {
                address 127.0.0.1
                shared-key testing123
            }
            coa-port 3799
        }
    }
}`
	tree, err := config.ParseTreeWithYANG(cfg, map[string]string{
		"l2tp-auth-radius": yang.ZeL2TPAuthRadiusConfYANG,
	})
	if err != nil {
		t.Fatalf("parse tree with coa-port: %v", err)
	}
	parsed, err := parseConfigFromTree(tree.ToPluginMap())
	if err != nil {
		t.Fatalf("parseConfigFromTree: %v", err)
	}
	if parsed.CoAPort != 3799 {
		t.Errorf("coa-port: got %d, want 3799", parsed.CoAPort)
	}
}

// TestParseConfigCoAPortAbsentDisablesListener pins the opt-in contract: with
// no `coa-port` leaf the CoAPort stays 0, so applyConfig's `cfg.CoAPort > 0`
// gate keeps the listener off.
//
// VALIDATES: existing RADIUS configs that never mentioned coa-port keep their
// current behavior (no CoA listener, no new open UDP port).
// PREVENTS: someone giving the YANG leaf a `default 3799`, which would
// materialize into every parsed tree and silently start a CoA listener on every
// deployment that upgrades.
func TestParseConfigCoAPortAbsentDisablesListener(t *testing.T) {
	cfg := `l2tp {
    enabled true
    auth {
        radius {
            server radius1 {
                address 127.0.0.1
                shared-key testing123
            }
        }
    }
}`
	tree, err := config.ParseTreeWithYANG(cfg, map[string]string{
		"l2tp-auth-radius": yang.ZeL2TPAuthRadiusConfYANG,
	})
	if err != nil {
		t.Fatalf("parse tree: %v", err)
	}
	parsed, err := parseConfigFromTree(tree.ToPluginMap())
	if err != nil {
		t.Fatalf("parseConfigFromTree: %v", err)
	}
	if parsed.CoAPort != 0 {
		t.Errorf("coa-port absent: got %d, want 0 (listener disabled)", parsed.CoAPort)
	}
}

// TestParseConfigCoAPortBounds checks the YANG range at its edges, per
// ai/rules/testing.md boundary testing: last valid, first invalid below, first
// invalid above.
//
// VALIDATES: the schema rejects out-of-range ports at parse time rather than
// letting them reach the listener.
// PREVENTS: a range typo (e.g. "0..65535") admitting port 0, which would read
// as "disabled" to the gate while the operator believes CoA is configured.
func TestParseConfigCoAPortBounds(t *testing.T) {
	tests := []struct {
		name    string
		port    string
		wantErr bool
	}{
		{name: "last valid", port: "65535", wantErr: false},
		{name: "first valid", port: "1", wantErr: false},
		{name: "invalid below", port: "0", wantErr: true},
		{name: "invalid above", port: "65536", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := `l2tp {
    enabled true
    auth {
        radius {
            server radius1 {
                address 127.0.0.1
                shared-key testing123
            }
            coa-port ` + tt.port + `
        }
    }
}`
			_, err := config.ParseTreeWithYANG(cfg, map[string]string{
				"l2tp-auth-radius": yang.ZeL2TPAuthRadiusConfYANG,
			})
			if tt.wantErr && err == nil {
				t.Errorf("coa-port %s: got nil error, want rejection", tt.port)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("coa-port %s: got error %v, want accepted", tt.port, err)
			}
		})
	}
}

// TestParseConfigKeyedServerMap verifies the keyed-list map shape directly
// (multiple servers keyed by name, scalars as strings), independent of the
// config parser.
//
// The failover order is the operator's, not the alphabet's. This test used to
// assert the opposite -- "sorted by name for predictable failover" -- which
// pinned the defect rather than the requirement: sorting IS deterministic, and
// it still made the operator's primary whichever name sorted first. The fixture
// now writes radius2 first, so the two orders disagree and only the right one
// passes.
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
					configorder.OrderKey("server"): []string{"radius2", "radius1"},
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
	// As the operator wrote them: radius2 first, radius1 second.
	if parsed.Servers[0].Address != "10.0.0.2:1813" {
		t.Errorf("server[0] address: got %q, want %q", parsed.Servers[0].Address, "10.0.0.2:1813")
	}
	if parsed.Servers[1].Address != "10.0.0.1:1812" {
		t.Errorf("server[1] address: got %q, want %q", parsed.Servers[1].Address, "10.0.0.1:1812")
	}
	if parsed.Timeout.Seconds() != 5 {
		t.Errorf("timeout: got %v, want 5s", parsed.Timeout)
	}
}

// TestParseConfigProductionTwoServersKeepsTheOperatorOrder drives the whole
// production pipeline -- config text, Tree, ToPluginMap, parse -- with two
// RADIUS servers written in a non-alphabetical order.
//
// VALIDATES: AC-5 end to end. The failover order an operator writes is the
// failover order the RADIUS client gets, with nothing hand-built in between.
// PREVENTS: a fix that works on a fixture and not on the daemon. Every other
// test in this file uses one server, which cannot tell an ordered reader from
// an unordered one.
func TestParseConfigProductionTwoServersKeepsTheOperatorOrder(t *testing.T) {
	cfg := `l2tp {
    enabled true
    auth {
        radius {
            server zurich {
                address 10.0.0.2
                shared-key testing123
            }
            server amsterdam {
                address 10.0.0.1
                shared-key testing123
            }
        }
    }
}`
	tree, err := config.ParseTreeWithYANG(cfg, map[string]string{
		"l2tp-auth-radius": yang.ZeL2TPAuthRadiusConfYANG,
	})
	if err != nil {
		t.Fatalf("parse tree: %v", err)
	}

	lowered := tree.ToPluginMap()
	parsed, err := parseConfigFromTree(lowered)
	if err != nil {
		t.Fatalf("parseConfigFromTree on production shape: %v", err)
	}
	if len(parsed.Servers) != 2 {
		t.Fatalf("servers: got %d, want 2", len(parsed.Servers))
	}
	if parsed.Servers[0].Address != "10.0.0.2:1812" {
		t.Errorf("primary server is %q, want the one written first (10.0.0.2:1812)", parsed.Servers[0].Address)
	}
	if parsed.Servers[1].Address != "10.0.0.1:1812" {
		t.Errorf("secondary server is %q, want 10.0.0.1:1812", parsed.Servers[1].Address)
	}

	// The lowering, not the reader, is what carries the order: assert the key
	// is actually in the payload, so a reader that guessed right would not pass
	// this test by accident.
	l2tpBlock, ok := lowered["l2tp"].(map[string]any)
	if !ok {
		t.Fatalf("lowered l2tp is %T, want a container", lowered["l2tp"])
	}
	authBlock, ok := l2tpBlock["auth"].(map[string]any)
	if !ok {
		t.Fatalf("lowered auth is %T, want a container", l2tpBlock["auth"])
	}
	radiusBlock, ok := authBlock["radius"].(map[string]any)
	if !ok {
		t.Fatalf("lowered radius is %T, want a container", authBlock["radius"])
	}
	if _, ok := radiusBlock[configorder.OrderKey("server")]; !ok {
		t.Error("the lowered config carries no server order")
	}
}
