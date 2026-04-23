package l2tpauthradius

import (
	"errors"
	"testing"
	"time"
)

func TestParseConfigValid(t *testing.T) {
	tree := map[string]any{
		"auth": map[string]any{
			"radius": map[string]any{
				"nas-identifier": "ze-router",
				"timeout":        float64(5),
				"retries":        float64(2),
				"acct-interval":  float64(120),
				"server": []any{
					map[string]any{
						"address":    "10.0.0.1",
						"port":       float64(1812),
						"shared-key": "secret123",
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
		"auth": map[string]any{
			"radius": map[string]any{
				"server": []any{
					map[string]any{
						"address":    "10.0.0.1",
						"shared-key": "secret",
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
	if cfg.AcctInterval != 300*time.Second {
		t.Errorf("default acct-interval: got %v, want 300s", cfg.AcctInterval)
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
		"auth": map[string]any{
			"local": map[string]any{},
		},
	}
	_, err := parseConfigFromTree(tree)
	if !errors.Is(err, errNoRADIUSConfig) {
		t.Errorf("expected errNoRADIUSConfig, got %v", err)
	}
}

func TestParseConfigMissingAddress(t *testing.T) {
	tree := map[string]any{
		"auth": map[string]any{
			"radius": map[string]any{
				"server": []any{
					map[string]any{
						"shared-key": "secret",
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
		"auth": map[string]any{
			"radius": map[string]any{
				"server": []any{
					map[string]any{
						"address": "10.0.0.1",
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
		"auth": map[string]any{
			"radius": map[string]any{
				"timeout": float64(0),
				"server": []any{
					map[string]any{"address": "10.0.0.1", "shared-key": "s"},
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
		"auth": map[string]any{
			"radius": map[string]any{
				"retries": float64(11),
				"server": []any{
					map[string]any{"address": "10.0.0.1", "shared-key": "s"},
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
		"auth": map[string]any{
			"radius": map[string]any{
				"server": []any{
					map[string]any{
						"address":    "10.0.0.1",
						"port":       float64(0),
						"shared-key": "s",
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
		"auth": map[string]any{
			"radius": map[string]any{
				"acct-interval": float64(59),
				"server": []any{
					map[string]any{"address": "10.0.0.1", "shared-key": "s"},
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
		"auth": map[string]any{
			"radius": map[string]any{
				"source-address": "192.168.1.100",
				"server": []any{
					map[string]any{"address": "10.0.0.1", "shared-key": "s"},
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
		"auth": map[string]any{
			"radius": map[string]any{
				"source-address": "not-an-ip",
				"server": []any{
					map[string]any{"address": "10.0.0.1", "shared-key": "s"},
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
		"auth": map[string]any{
			"radius": map[string]any{
				"source-address": "::1",
				"server": []any{
					map[string]any{"address": "10.0.0.1", "shared-key": "s"},
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
		"auth": map[string]any{
			"radius": map[string]any{
				"server": []any{
					map[string]any{"address": "10.0.0.1", "shared-key": "s"},
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
		"auth": map[string]any{
			"radius": map[string]any{
				"server": []any{
					map[string]any{"address": "10.0.0.1", "shared-key": "s1"},
					map[string]any{"address": "10.0.0.2", "port": float64(1813), "shared-key": "s2"},
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
