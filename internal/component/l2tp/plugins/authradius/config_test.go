package l2tpauthradius

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/configorder"
)

func TestParseConfigValid(t *testing.T) {
	tree := map[string]any{
		"l2tp": map[string]any{
			"auth": map[string]any{
				"radius": map[string]any{
					"nas-identifier": "ze-router",
					"timeout":        float64(5),
					"retries":        float64(2),
					"acct-interval":  float64(120),
					"server": []any{
						map[string]any{
							"name":       "radius1",
							"address":    "10.0.0.1",
							"port":       float64(1812),
							"shared-key": "secret123",
						},
					},
				},
			},
		},
	}

	cfg, err := parseConfigFromTree(tree)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NASIdentifier != "ze-router" {
		t.Errorf("NAS-Identifier: got %q, want %q", cfg.NASIdentifier, "ze-router")
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("timeout: got %v, want 5s", cfg.Timeout)
	}
	if cfg.Retries != 2 {
		t.Errorf("retries: got %d, want 2", cfg.Retries)
	}
	if cfg.AcctInterval != 120*time.Second {
		t.Errorf("acct-interval: got %v, want 120s", cfg.AcctInterval)
	}
	if len(cfg.Servers) != 1 {
		t.Fatalf("servers: got %d, want 1", len(cfg.Servers))
	}
	if cfg.Servers[0].Address != "10.0.0.1:1812" {
		t.Errorf("server address: got %q", cfg.Servers[0].Address)
	}
}

func TestParseConfigDefaults(t *testing.T) {
	tree := map[string]any{
		"l2tp": map[string]any{
			"auth": map[string]any{
				"radius": map[string]any{
					"server": []any{
						map[string]any{
							"name":       "radius2",
							"address":    "10.0.0.1",
							"shared-key": "secret",
						},
					},
				},
			},
		},
	}

	cfg, err := parseConfigFromTree(tree)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Timeout != 3*time.Second {
		t.Errorf("default timeout: got %v, want 3s", cfg.Timeout)
	}
	if cfg.Retries != 3 {
		t.Errorf("default retries: got %d, want 3", cfg.Retries)
	}
	// acct-interval is the one leaf here with no default. RFC 2869 Section 2.1
	// gives a locally configured value precedence over the Access-Accept. An
	// absent leaf must stay absent through the parse: a default reads as an
	// operator's choice. The 300 second fallback lives in acctIntervalDefault
	// (acct.go), which TestAcctIntervalPrecedence asserts.
	if cfg.AcctInterval != 0 {
		t.Errorf("absent acct-interval: got %v, want 0 (unset)", cfg.AcctInterval)
	}
	if cfg.Servers[0].Address != "10.0.0.1:1812" {
		t.Errorf("default port: got %q", cfg.Servers[0].Address)
	}
}

func TestParseConfigNoAuthBlock(t *testing.T) {
	_, err := parseConfigFromTree(map[string]any{"other": "stuff"})
	if !errors.Is(err, errNoRADIUSConfig) {
		t.Errorf("expected errNoRADIUSConfig, got %v", err)
	}
}

func TestParseConfigNoRadiusBlock(t *testing.T) {
	tree := map[string]any{
		"l2tp": map[string]any{
			"auth": map[string]any{
				"local": map[string]any{},
			},
		},
	}
	_, err := parseConfigFromTree(tree)
	if !errors.Is(err, errNoRADIUSConfig) {
		t.Errorf("expected errNoRADIUSConfig, got %v", err)
	}
}

func TestParseConfigMissingAddress(t *testing.T) {
	tree := map[string]any{
		"l2tp": map[string]any{
			"auth": map[string]any{
				"radius": map[string]any{
					"server": []any{
						map[string]any{
							"shared-key": "secret",
						},
					},
				},
			},
		},
	}
	_, err := parseConfigFromTree(tree)
	if err == nil {
		t.Fatal("expected error for missing address")
	}
}

func TestParseConfigMissingSharedKey(t *testing.T) {
	tree := map[string]any{
		"l2tp": map[string]any{
			"auth": map[string]any{
				"radius": map[string]any{
					"server": []any{
						map[string]any{
							"name":    "radius3",
							"address": "10.0.0.1",
						},
					},
				},
			},
		},
	}
	_, err := parseConfigFromTree(tree)
	if err == nil {
		t.Fatal("expected error for missing shared-key")
	}
}

func TestParseConfigBadTimeout(t *testing.T) {
	tree := map[string]any{
		"l2tp": map[string]any{
			"auth": map[string]any{
				"radius": map[string]any{
					"timeout": float64(0),
					"server": []any{
						map[string]any{"name": "radius1", "address": "10.0.0.1", "shared-key": "s"},
					},
				},
			},
		},
	}
	_, err := parseConfigFromTree(tree)
	if err == nil {
		t.Fatal("expected error for timeout=0")
	}
}

func TestParseConfigBadRetries(t *testing.T) {
	tree := map[string]any{
		"l2tp": map[string]any{
			"auth": map[string]any{
				"radius": map[string]any{
					"retries": float64(11),
					"server": []any{
						map[string]any{"name": "radius2", "address": "10.0.0.1", "shared-key": "s"},
					},
				},
			},
		},
	}
	_, err := parseConfigFromTree(tree)
	if err == nil {
		t.Fatal("expected error for retries=11")
	}
}

func TestParseConfigBadPort(t *testing.T) {
	tree := map[string]any{
		"l2tp": map[string]any{
			"auth": map[string]any{
				"radius": map[string]any{
					"server": []any{
						map[string]any{
							"name":       "radius4",
							"address":    "10.0.0.1",
							"port":       float64(0),
							"shared-key": "s",
						},
					},
				},
			},
		},
	}
	_, err := parseConfigFromTree(tree)
	if err == nil {
		t.Fatal("expected error for port=0")
	}
}

func TestParseConfigBadAcctInterval(t *testing.T) {
	tree := map[string]any{
		"l2tp": map[string]any{
			"auth": map[string]any{
				"radius": map[string]any{
					"acct-interval": float64(59),
					"server": []any{
						map[string]any{"name": "radius3", "address": "10.0.0.1", "shared-key": "s"},
					},
				},
			},
		},
	}
	_, err := parseConfigFromTree(tree)
	if err == nil {
		t.Fatal("expected error for acct-interval=59")
	}
}

func TestParseConfigSourceAddress(t *testing.T) {
	tree := map[string]any{
		"l2tp": map[string]any{
			"auth": map[string]any{
				"radius": map[string]any{
					"source-address": "192.168.1.100",
					"server": []any{
						map[string]any{"name": "radius4", "address": "10.0.0.1", "shared-key": "s"},
					},
				},
			},
		},
	}

	cfg, err := parseConfigFromTree(tree)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SourceAddress == nil {
		t.Fatal("source-address: got nil, want 192.168.1.100")
	}
	if cfg.SourceAddress.String() != "192.168.1.100" {
		t.Errorf("source-address: got %q, want %q", cfg.SourceAddress, "192.168.1.100")
	}
}

func TestParseConfigBadSourceAddress(t *testing.T) {
	tree := map[string]any{
		"l2tp": map[string]any{
			"auth": map[string]any{
				"radius": map[string]any{
					"source-address": "not-an-ip",
					"server": []any{
						map[string]any{"name": "radius5", "address": "10.0.0.1", "shared-key": "s"},
					},
				},
			},
		},
	}

	_, err := parseConfigFromTree(tree)
	if err == nil {
		t.Fatal("expected error for invalid source-address")
	}
}

func TestParseConfigIPv6SourceAddress(t *testing.T) {
	tree := map[string]any{
		"l2tp": map[string]any{
			"auth": map[string]any{
				"radius": map[string]any{
					"source-address": "::1",
					"server": []any{
						map[string]any{"name": "radius6", "address": "10.0.0.1", "shared-key": "s"},
					},
				},
			},
		},
	}

	_, err := parseConfigFromTree(tree)
	if err == nil {
		t.Fatal("expected error for IPv6 source-address")
	}
}

func TestParseConfigNoSourceAddress(t *testing.T) {
	tree := map[string]any{
		"l2tp": map[string]any{
			"auth": map[string]any{
				"radius": map[string]any{
					"server": []any{
						map[string]any{"name": "radius7", "address": "10.0.0.1", "shared-key": "s"},
					},
				},
			},
		},
	}

	cfg, err := parseConfigFromTree(tree)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SourceAddress != nil {
		t.Errorf("source-address: got %v, want nil", cfg.SourceAddress)
	}
}

func TestParseConfigMultipleServers(t *testing.T) {
	tree := map[string]any{
		"l2tp": map[string]any{
			"auth": map[string]any{
				"radius": map[string]any{
					"server": []any{
						map[string]any{"name": "radius8", "address": "10.0.0.1", "shared-key": "s1"},
						map[string]any{"name": "radius9", "address": "10.0.0.2", "port": float64(1813), "shared-key": "s2"},
					},
				},
			},
		},
	}

	cfg, err := parseConfigFromTree(tree)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Servers) != 2 {
		t.Fatalf("servers: got %d, want 2", len(cfg.Servers))
	}
	if cfg.Servers[1].Address != "10.0.0.2:1813" {
		t.Errorf("second server: got %q", cfg.Servers[1].Address)
	}
}

// TestServerEntriesFollowTheOperatorNotTheAlphabet parses a two-server RADIUS
// block in the shape production delivers -- a map keyed by the server name,
// with the order beside it -- and then parses the same two servers with the
// order reversed.
//
// VALIDATES: AC-5. Servers reach the client in the order the operator wrote
// them, which is the failover order: cfg.Servers[0] is tried first.
// PREVENTS: the silent half of the ordered-list defect on this reader.
// serverEntries used to sort the keyed map by name and call that "deterministic
// server (failover) ordering". It is deterministic and it is wrong: the
// operator's primary server became whichever name sorted first.
//
// "zurich" is written first and "amsterdam" second, so the alphabet and the
// config disagree, and the reversed row disagrees with the alphabet the other
// way. Neither row can pass by sorting.
func TestServerEntriesFollowTheOperatorNotTheAlphabet(t *testing.T) {
	tree := func(order []string) map[string]any {
		return map[string]any{
			"l2tp": map[string]any{
				"auth": map[string]any{
					"radius": map[string]any{
						"server": map[string]any{
							"zurich":    map[string]any{"address": "10.0.0.2", "shared-key": "secret123"},
							"amsterdam": map[string]any{"address": "10.0.0.1", "shared-key": "secret123"},
						},
						configorder.OrderKey("server"): order,
					},
				},
			},
		}
	}

	for _, tc := range []struct {
		name    string
		order   []string
		primary string
	}{
		{"as the operator wrote them", []string{"zurich", "amsterdam"}, "10.0.0.2:1812"},
		{"with the two servers swapped", []string{"amsterdam", "zurich"}, "10.0.0.1:1812"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := parseConfigFromTree(tree(tc.order))
			if err != nil {
				t.Fatalf("parseConfigFromTree: %v", err)
			}
			if len(cfg.Servers) != 2 {
				t.Fatalf("servers: got %d, want 2", len(cfg.Servers))
			}
			if cfg.Servers[0].Address != tc.primary {
				t.Errorf("primary server is %q, want %q", cfg.Servers[0].Address, tc.primary)
			}
		})
	}
}

// TestServerEntriesRefuseTwoServersWithNoOrder parses two servers with no order
// delivered beside them.
//
// VALIDATES: a multi-server list whose order was not delivered is refused.
// PREVENTS: a fallback to sorting. A RADIUS failover order that is wrong is
// invisible until the primary server fails, which is the worst moment to learn
// that the secondary was being tried first all along.
func TestServerEntriesRefuseTwoServersWithNoOrder(t *testing.T) {
	tree := map[string]any{
		"l2tp": map[string]any{
			"auth": map[string]any{
				"radius": map[string]any{
					"server": map[string]any{
						"zurich":    map[string]any{"address": "10.0.0.2", "shared-key": "secret123"},
						"amsterdam": map[string]any{"address": "10.0.0.1", "shared-key": "secret123"},
					},
				},
			},
		},
	}

	if _, err := parseConfigFromTree(tree); err == nil {
		t.Fatal("two servers with no delivered order were accepted")
	} else if !strings.Contains(err.Error(), "no order") {
		t.Errorf("error %q does not say the order was missing", err.Error())
	}
}
