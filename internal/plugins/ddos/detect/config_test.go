package detect

import "testing"

func TestParseCharacterizeLeaves(t *testing.T) {
	data := `{"ddos":{"detect":{` +
		`"characterize-enable":false,"top-n-sources":25,` +
		`"characterize-window":30,"characterize-timeout":500,"entropy-threshold":3.5}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CharacterizeEnable {
		t.Error("characterize-enable should parse false")
	}
	if cfg.TopNSources != 25 {
		t.Errorf("top-n-sources = %d, want 25", cfg.TopNSources)
	}
	if cfg.CharacterizeWindow != 30 {
		t.Errorf("characterize-window = %d, want 30", cfg.CharacterizeWindow)
	}
	if cfg.CharacterizeTimeout != 500 {
		t.Errorf("characterize-timeout = %d, want 500", cfg.CharacterizeTimeout)
	}
	if cfg.EntropyThreshold != 3.5 {
		t.Errorf("entropy-threshold = %v, want 3.5", cfg.EntropyThreshold)
	}
}

func TestParseBpsLeaves(t *testing.T) {
	// VALIDATES: the bandwidth-trigger leaves parse into Config.
	data := `{"ddos":{"detect":{` +
		`"bps-trigger-enable":false,"bps-threshold-multiplier":5.0,"bps-floor":100000000}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BpsTriggerEnable {
		t.Error("bps-trigger-enable should parse false")
	}
	if cfg.BpsThresholdMultiplier != 5.0 {
		t.Errorf("bps-threshold-multiplier = %v, want 5.0", cfg.BpsThresholdMultiplier)
	}
	if cfg.BpsFloor != 100000000 {
		t.Errorf("bps-floor = %v, want 100000000", cfg.BpsFloor)
	}
}

func TestBpsDefaults(t *testing.T) {
	def := DefaultConfig()
	if !def.BpsTriggerEnable {
		t.Error("bps-trigger-enable should default true")
	}
	if def.BpsThresholdMultiplier != 3.0 {
		t.Errorf("bps-threshold-multiplier default = %v, want 3.0", def.BpsThresholdMultiplier)
	}
	if def.BpsFloor != 50_000_000 {
		t.Errorf("bps-floor default = %v, want 50000000 (50 Mbps)", def.BpsFloor)
	}
}

func TestBpsBoundaries(t *testing.T) {
	base := func() *Config { c := DefaultConfig(); c.Enabled = true; return c }
	cases := []struct {
		name    string
		set     func(*Config)
		wantErr bool
	}{
		{"mult-low-invalid", func(c *Config) { c.BpsThresholdMultiplier = 0.99 }, true},
		{"mult-low-valid", func(c *Config) { c.BpsThresholdMultiplier = 1.0 }, false},
		{"mult-high-valid", func(c *Config) { c.BpsThresholdMultiplier = 100.0 }, false},
		{"mult-high-invalid", func(c *Config) { c.BpsThresholdMultiplier = 100.01 }, true},
		{"floor-zero-invalid", func(c *Config) { c.BpsFloor = 0 }, true},
		{"floor-one-valid", func(c *Config) { c.BpsFloor = 1 }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := base()
			tc.set(c)
			err := c.Validate()
			if tc.wantErr && err == nil {
				t.Error("expected validation error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestDefaultConfigValidatesWithCharacterizeDefaults(t *testing.T) {
	if err := DefaultConfig().Validate(); err != nil {
		t.Errorf("DefaultConfig must validate (characterization defaults included): %v", err)
	}
	if def := DefaultConfig(); !def.CharacterizeEnable {
		t.Error("characterize-enable should default true")
	}
}

func TestCharacterizeBoundaries(t *testing.T) {
	base := func() *Config { c := DefaultConfig(); c.Enabled = true; return c }
	cases := []struct {
		name    string
		set     func(*Config)
		wantErr bool
	}{
		{"topn-below", func(c *Config) { c.TopNSources = 0 }, true},
		{"topn-min", func(c *Config) { c.TopNSources = 1 }, false},
		{"topn-max", func(c *Config) { c.TopNSources = 100 }, false},
		{"topn-above", func(c *Config) { c.TopNSources = 101 }, true},
		{"window-below", func(c *Config) { c.CharacterizeWindow = 0 }, true},
		{"window-min", func(c *Config) { c.CharacterizeWindow = 1 }, false},
		{"window-max", func(c *Config) { c.CharacterizeWindow = 60 }, false},
		{"window-above", func(c *Config) { c.CharacterizeWindow = 61 }, true},
		{"timeout-below", func(c *Config) { c.CharacterizeTimeout = 49 }, true},
		{"timeout-min", func(c *Config) { c.CharacterizeTimeout = 50 }, false},
		{"timeout-max", func(c *Config) { c.CharacterizeTimeout = 5000 }, false},
		{"timeout-above", func(c *Config) { c.CharacterizeTimeout = 5001 }, true},
		{"entropy-below", func(c *Config) { c.EntropyThreshold = -0.1 }, true},
		{"entropy-max", func(c *Config) { c.EntropyThreshold = 16 }, false},
		{"entropy-above", func(c *Config) { c.EntropyThreshold = 16.1 }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := base()
			tc.set(c)
			err := c.Validate()
			if tc.wantErr && err == nil {
				t.Error("expected validation error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

// TestParseConfig_StringValuedDelivery pins the ACTUAL config format the plugin
// framework delivers: every YANG leaf value arrives as a JSON string ("true",
// "50000"), not the native type. The prior parser used v.(bool)/float64-only
// coercion, so `enabled` silently stayed false and the detector never ran --
// ddos detection was effectively disabled in every daemon. This test would have
// caught that; the pre-existing tests used native JSON types and missed it.
func TestParseConfig_StringValuedDelivery(t *testing.T) {
	data := `{"ddos":{"detect":{` +
		`"enabled":"true","absolute-floor":"50000","baseline-window":"10",` +
		`"bps-trigger-enable":"true","bps-floor":"8000000","bps-threshold-multiplier":"5.0",` +
		`"characterize-enable":"false","top-n-sources":"25","entropy-threshold":"3.5"}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled {
		t.Error(`enabled "true" (string) must parse to true -- the detector-disabled bug`)
	}
	if cfg.AbsoluteFloor != 50000 {
		t.Errorf("absolute-floor = %v, want 50000", cfg.AbsoluteFloor)
	}
	if cfg.BaselineWindow != 10 {
		t.Errorf("baseline-window = %d, want 10", cfg.BaselineWindow)
	}
	if !cfg.BpsTriggerEnable {
		t.Error(`bps-trigger-enable "true" (string) must parse to true`)
	}
	if cfg.BpsFloor != 8000000 {
		t.Errorf("bps-floor = %v, want 8000000", cfg.BpsFloor)
	}
	if cfg.BpsThresholdMultiplier != 5.0 {
		t.Errorf("bps-threshold-multiplier = %v, want 5.0", cfg.BpsThresholdMultiplier)
	}
	if cfg.CharacterizeEnable {
		t.Error(`characterize-enable "false" (string) must parse to false`)
	}
	if cfg.TopNSources != 25 {
		t.Errorf("top-n-sources = %d, want 25", cfg.TopNSources)
	}
	if cfg.EntropyThreshold != 3.5 {
		t.Errorf("entropy-threshold = %v, want 3.5", cfg.EntropyThreshold)
	}
}
