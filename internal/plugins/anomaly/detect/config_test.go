// VALIDATES: AC-5/AC-6 config -- the anomaly-detect leaves parse from JSON and
// every numeric leaf enforces its documented range (boundary table in the spec).
// PREVENTS: an out-of-range threshold, cohort prefix length, or correlation count
// slipping through and destabilizing scoring.

package detect

import "testing"

func TestParseConfigLeaves(t *testing.T) {
	data := `{"anomaly":{"detect":{"enabled":true,"deviation-threshold":4.5,` +
		`"min-features-to-correlate":2,"min-cohort-size":8,"corroboration-weight":0.6,` +
		`"confirm-duration":5,"clear-consecutive":8,"baseline-window":600,` +
		`"cohort-prefix-len-v4":24,"cohort-prefix-len-v6":48}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled || cfg.DeviationThreshold != 4.5 || cfg.MinFeaturesToCorrelate != 2 ||
		cfg.MinCohortSize != 8 || cfg.CorroborationWeight != 0.6 || cfg.ConfirmDuration != 5 ||
		cfg.ClearConsecutive != 8 || cfg.BaselineWindow != 600 ||
		cfg.CohortPrefixLenV4 != 24 || cfg.CohortPrefixLenV6 != 48 {
		t.Fatalf("parsed config mismatch: %+v", cfg)
	}
}

func TestDefaultConfigValidates(t *testing.T) {
	if err := DefaultConfig().Validate(); err != nil {
		t.Errorf("DefaultConfig must validate: %v", err)
	}
}

func TestConfigBoundaries(t *testing.T) {
	base := func() *Config { c := DefaultConfig(); c.Enabled = true; return c }
	cases := []struct {
		name    string
		set     func(*Config)
		wantErr bool
	}{
		{"dev-below", func(c *Config) { c.DeviationThreshold = 0.9 }, true},
		{"dev-min", func(c *Config) { c.DeviationThreshold = 1.0 }, false},
		{"dev-max", func(c *Config) { c.DeviationThreshold = 100.0 }, false},
		{"dev-above", func(c *Config) { c.DeviationThreshold = 100.1 }, true},
		{"minfeat-below", func(c *Config) { c.MinFeaturesToCorrelate = 0 }, true},
		{"minfeat-max", func(c *Config) { c.MinFeaturesToCorrelate = 6 }, false},
		{"minfeat-above", func(c *Config) { c.MinFeaturesToCorrelate = 7 }, true},
		{"cohort-below", func(c *Config) { c.MinCohortSize = 1 }, true},
		{"cohort-min", func(c *Config) { c.MinCohortSize = 2 }, false},
		{"weight-below", func(c *Config) { c.CorroborationWeight = -0.1 }, true},
		{"weight-max", func(c *Config) { c.CorroborationWeight = 1.0 }, false},
		{"weight-above", func(c *Config) { c.CorroborationWeight = 1.1 }, true},
		{"v4-below", func(c *Config) { c.CohortPrefixLenV4 = 7 }, true},
		{"v4-max", func(c *Config) { c.CohortPrefixLenV4 = 32 }, false},
		{"v4-above", func(c *Config) { c.CohortPrefixLenV4 = 33 }, true},
		{"v6-below", func(c *Config) { c.CohortPrefixLenV6 = 15 }, true},
		{"v6-max", func(c *Config) { c.CohortPrefixLenV6 = 64 }, false},
		{"confirm-below", func(c *Config) { c.ConfirmDuration = 0 }, true},
		{"baseline-below", func(c *Config) { c.BaselineWindow = 9 }, true},
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
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
