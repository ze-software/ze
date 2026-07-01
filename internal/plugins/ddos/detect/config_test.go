package detect

import "testing"

func TestParseCharacterizeLeaves(t *testing.T) {
	data := `{"ddos-detect":{` +
		`"characterize-enable":false,"top-n-sources":25,` +
		`"characterize-window":30,"characterize-timeout":500,"entropy-threshold":3.5}}`
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
