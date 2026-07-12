package flowspec

import "testing"

// TestParseConfig_StringValuedDelivery pins the ACTUAL config format the plugin
// framework delivers: every leaf value arrives as a JSON string. toInt/toFloat
// previously handled only native numeric types, so every numeric leaf (including
// the new confidence-min gate) silently reverted to its default.
func TestParseConfig_StringValuedDelivery(t *testing.T) {
	data := `{"ddos":{"flowspec":{` +
		`"response-level":"enforce","action":"discard",` +
		`"hold-down":"120","probe-interval":"30","probe-window":"20",` +
		`"probe-rate":"2000000","announce-rate-limit":"5",` +
		`"max-mitigation-duration":"1800","backoff-cap":"7200",` +
		`"confidence-min":"75","blackhole-fallback":"true"}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.HoldDown != 120 {
		t.Errorf("hold-down = %d, want 120", cfg.HoldDown)
	}
	if cfg.ProbeInterval != 30 {
		t.Errorf("probe-interval = %d, want 30", cfg.ProbeInterval)
	}
	if cfg.ProbeRate != 2000000 {
		t.Errorf("probe-rate = %v, want 2000000", cfg.ProbeRate)
	}
	if cfg.MaxMitigationDuration != 1800 {
		t.Errorf("max-mitigation-duration = %d, want 1800", cfg.MaxMitigationDuration)
	}
	if cfg.ConfidenceMin != 75 {
		t.Errorf("confidence-min = %d, want 75 (string-valued numeric leaf ignored)", cfg.ConfidenceMin)
	}
	if !cfg.BlackholeFallback {
		t.Error("blackhole-fallback string \"true\" must parse true")
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("string-delivered config must validate: %v", err)
	}
}

func TestParseBlackholeFallback(t *testing.T) {
	// Default is off (announce only on characterization).
	if def := DefaultConfig(); def.BlackholeFallback {
		t.Error("blackhole-fallback should default to false")
	}

	// Accept both the array-form bool and the daemon's string form.
	for _, data := range []string{
		`{"ddos":{"flowspec":{"blackhole-fallback":true}}}`,
		`{"ddos":{"flowspec":{"blackhole-fallback":"true"}}}`,
	} {
		cfg, err := ParseConfig(data)
		if err != nil {
			t.Fatalf("ParseConfig(%s): %v", data, err)
		}
		if !cfg.BlackholeFallback {
			t.Errorf("ParseConfig(%s): blackhole-fallback not parsed as true", data)
		}
	}
}

func TestValidateAcceptsValidAndRejectsRange(t *testing.T) {
	valid := DefaultConfig()
	valid.ResponseLevel = "enforce"
	valid.Action = "discard" // action is mandatory (no default)
	valid.BlackholeFallback = true
	if err := valid.Validate(); err != nil {
		t.Errorf("valid config rejected: %v", err)
	}

	bad := DefaultConfig()
	bad.Action = "discard"
	bad.HoldDown = 0 // out of [1, 86400]
	if err := bad.Validate(); err == nil {
		t.Error("expected error for hold-down=0")
	}
}

func TestConfigRateLimitRequiresBytes(t *testing.T) {
	// VALIDATES: AC-4/AC-5 -- rate-limit with an ABSENT rate is a config error
	// (no fabricated rate); rate-limit-bytes 0 == discard is valid; discard needs
	// no rate. Boundary: 0 is the last valid value (unsigned; there is no below).
	cases := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{"rate-limit absent bytes", `{"ddos":{"flowspec":{"response-level":"enforce","action":"rate-limit"}}}`, true},
		{"rate-limit bytes 0 (== discard)", `{"ddos":{"flowspec":{"response-level":"enforce","action":"rate-limit","rate-limit-bytes":0}}}`, false},
		{"rate-limit bytes N", `{"ddos":{"flowspec":{"response-level":"enforce","action":"rate-limit","rate-limit-bytes":100000}}}`, false},
		{"discard needs no rate", `{"ddos":{"flowspec":{"response-level":"enforce","action":"discard"}}}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseConfig(tc.data)
			if err != nil {
				t.Fatalf("ParseConfig: %v", err)
			}
			err = cfg.Validate()
			if tc.wantErr && err == nil {
				t.Error("expected a validation error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
		})
	}
}
