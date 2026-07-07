package flowspec

import "testing"

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
