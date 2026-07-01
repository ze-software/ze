package flowspec

import "testing"

func TestParseBlackholeFallback(t *testing.T) {
	// Default is off (announce only on characterization).
	if def := DefaultConfig(); def.BlackholeFallback {
		t.Error("blackhole-fallback should default to false")
	}

	// Accept both the array-form bool and the daemon's string form.
	for _, data := range []string{
		`{"ddos-flowspec":{"blackhole-fallback":true}}`,
		`{"ddos-flowspec":{"blackhole-fallback":"true"}}`,
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
	valid.BlackholeFallback = true
	if err := valid.Validate(); err != nil {
		t.Errorf("valid config rejected: %v", err)
	}

	bad := DefaultConfig()
	bad.HoldDown = 0 // out of [1, 86400]
	if err := bad.Validate(); err == nil {
		t.Error("expected error for hold-down=0")
	}
}
