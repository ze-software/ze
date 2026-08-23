// VALIDATES: shadow-first config -- mode/action enums, rate-limit params, and the
// safety-rail numeric ranges (auto-revert-ttl, blast-radius-cap) parse and validate.
// PREVENTS: an out-of-range TTL or cap, or an unknown mode/action, reaching the
// autonomous responder.

package shape

import (
	"net/netip"
	"testing"
)

func TestParseConfigLeaves(t *testing.T) {
	data := `{"anomaly":{"shape":{"mode":"armed","action":"drop","limit-rate":500,` +
		`"limit-unit":"second","limit-burst":10,"auto-revert-ttl":120,` +
		`"blast-radius-cap":8,"kill-switch":true,"allowlist":"10.0.0.0/8"}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != "armed" || cfg.Action != "drop" || cfg.LimitRate != 500 ||
		cfg.LimitUnit != "second" || cfg.LimitBurst != 10 || cfg.AutoRevertTTL != 120 ||
		cfg.BlastRadiusCap != 8 || !cfg.KillSwitch || len(cfg.Allowlist) != 1 {
		t.Fatalf("parsed config mismatch: %+v", cfg)
	}
	if cfg.Allowlist[0] != netip.MustParsePrefix("10.0.0.0/8") {
		t.Errorf("allowlist[0] = %v", cfg.Allowlist[0])
	}
}

// TestParseConfig_StringValuedDelivery pins the ACTUAL config format the plugin
// framework delivers: every YANG leaf value arrives as a JSON string ("true",
// "500"), not the native type. The prior parser used v.(bool)/float64-only
// coercion for the numeric and boolean leaves, so `kill-switch` and the
// rate-limit params silently fell back to defaults. This test feeds every scalar
// leaf as a string and asserts the configured (non-default) value is parsed.
func TestParseConfig_StringValuedDelivery(t *testing.T) {
	data := `{"anomaly":{"shape":{` +
		`"mode":"armed","action":"drop","limit-rate":"500","limit-unit":"minute",` +
		`"limit-burst":"10","auto-revert-ttl":"120","blast-radius-cap":"8",` +
		`"kill-switch":"true","allowlist":"10.0.0.0/8"}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mode != "armed" {
		t.Errorf("mode = %q, want armed", cfg.Mode)
	}
	if cfg.Action != "drop" {
		t.Errorf("action = %q, want drop", cfg.Action)
	}
	if cfg.LimitRate != 500 {
		t.Errorf("limit-rate = %d, want 500", cfg.LimitRate)
	}
	if cfg.LimitUnit != "minute" {
		t.Errorf("limit-unit = %q, want minute", cfg.LimitUnit)
	}
	if cfg.LimitBurst != 10 {
		t.Errorf("limit-burst = %d, want 10", cfg.LimitBurst)
	}
	if cfg.AutoRevertTTL != 120 {
		t.Errorf("auto-revert-ttl = %d, want 120", cfg.AutoRevertTTL)
	}
	if cfg.BlastRadiusCap != 8 {
		t.Errorf("blast-radius-cap = %d, want 8", cfg.BlastRadiusCap)
	}
	if !cfg.KillSwitch {
		t.Error(`kill-switch "true" (string) must parse to true -- the emergency-revert-ignored bug`)
	}
	if len(cfg.Allowlist) != 1 || cfg.Allowlist[0] != netip.MustParsePrefix("10.0.0.0/8") {
		t.Errorf("allowlist = %v, want [10.0.0.0/8]", cfg.Allowlist)
	}
}

func TestDefaultConfigValidatesAndIsShadow(t *testing.T) {
	c := DefaultConfig()
	if err := c.Validate(); err != nil {
		t.Errorf("DefaultConfig must validate: %v", err)
	}
	if c.Mode != "shadow" {
		t.Errorf("default mode = %q, want shadow (shadow-first)", c.Mode)
	}
}

func TestConfigBoundaries(t *testing.T) {
	base := func() *Config { c := DefaultConfig(); c.Mode = "armed"; return c }
	cases := []struct {
		name    string
		set     func(*Config)
		wantErr bool
	}{
		{"mode-bad", func(c *Config) { c.Mode = "on" }, true},
		{"mode-shadow", func(c *Config) { c.Mode = "shadow" }, false},
		{"action-bad", func(c *Config) { c.Action = "block" }, true},
		{"action-drop", func(c *Config) { c.Action = "drop" }, false},
		{"unit-bad", func(c *Config) { c.LimitUnit = "fortnight" }, true},
		{"rate-zero-limit", func(c *Config) { c.Action = "limit"; c.LimitRate = 0 }, true},
		{"ttl-below", func(c *Config) { c.AutoRevertTTL = 4 }, true},
		{"ttl-min", func(c *Config) { c.AutoRevertTTL = 5 }, false},
		{"ttl-max", func(c *Config) { c.AutoRevertTTL = 3600 }, false},
		{"ttl-above", func(c *Config) { c.AutoRevertTTL = 3601 }, true},
		{"cap-below", func(c *Config) { c.BlastRadiusCap = 0 }, true},
		{"cap-min", func(c *Config) { c.BlastRadiusCap = 1 }, false},
		{"cap-max", func(c *Config) { c.BlastRadiusCap = 1024 }, false},
		{"cap-above", func(c *Config) { c.BlastRadiusCap = 1025 }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := base()
			tc.set(c)
			err := c.Validate()
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
