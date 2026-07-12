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

// TestParseConfig_StringValuedDelivery pins the ACTUAL config format the plugin
// framework delivers: every YANG leaf value arrives as a JSON string ("true",
// "4.5"), not the native type. The prior parser used v.(bool)/float64-only
// coercion, so `enabled` silently stayed false and the detector never ran --
// anomaly detection was effectively disabled in every daemon. This test would
// have caught that; the pre-existing tests used native JSON types and missed it.
func TestParseConfig_StringValuedDelivery(t *testing.T) {
	data := `{"anomaly":{"detect":{` +
		`"enabled":"true","deviation-threshold":"4.5","min-features-to-correlate":"3",` +
		`"min-cohort-size":"8","corroboration-weight":"0.6","confirm-duration":"5",` +
		`"clear-consecutive":"8","baseline-window":"600",` +
		`"cohort-prefix-len-v4":"24","cohort-prefix-len-v6":"56"}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled {
		t.Error(`enabled "true" (string) must parse to true -- the detector-disabled bug`)
	}
	if cfg.DeviationThreshold != 4.5 {
		t.Errorf("deviation-threshold = %v, want 4.5", cfg.DeviationThreshold)
	}
	if cfg.MinFeaturesToCorrelate != 3 {
		t.Errorf("min-features-to-correlate = %d, want 3", cfg.MinFeaturesToCorrelate)
	}
	if cfg.MinCohortSize != 8 {
		t.Errorf("min-cohort-size = %d, want 8", cfg.MinCohortSize)
	}
	if cfg.CorroborationWeight != 0.6 {
		t.Errorf("corroboration-weight = %v, want 0.6", cfg.CorroborationWeight)
	}
	if cfg.ConfirmDuration != 5 {
		t.Errorf("confirm-duration = %d, want 5", cfg.ConfirmDuration)
	}
	if cfg.ClearConsecutive != 8 {
		t.Errorf("clear-consecutive = %d, want 8", cfg.ClearConsecutive)
	}
	if cfg.BaselineWindow != 600 {
		t.Errorf("baseline-window = %d, want 600", cfg.BaselineWindow)
	}
	if cfg.CohortPrefixLenV4 != 24 {
		t.Errorf("cohort-prefix-len-v4 = %d, want 24", cfg.CohortPrefixLenV4)
	}
	if cfg.CohortPrefixLenV6 != 56 {
		t.Errorf("cohort-prefix-len-v6 = %d, want 56", cfg.CohortPrefixLenV6)
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
