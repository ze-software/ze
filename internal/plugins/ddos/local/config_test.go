package local

import "testing"

// VALIDATES: string-valued YANG leaf delivery parses into Config numeric leaves.
// PREVENTS: regression to int/float64-only toInt that silently used defaults.
// TestParseConfig_StringValuedDelivery pins the ACTUAL config format the plugin
// framework delivers: every YANG leaf value arrives as a JSON string ("1800",
// "75"), not the native JSON number. The prior toInt handled only int/float64,
// so every numeric leaf silently fell back to its default. This test feeds
// string-valued leaves and asserts the CONFIGURED values (which differ from the
// defaults).
func TestParseConfig_StringValuedDelivery(t *testing.T) {
	data := `{"ddos":{"local":{` +
		`"response-level":"enforce","max-mitigation-duration":"1800",` +
		`"confidence-min":"75","allowlist":["10.0.0.0/8","192.168.0.0/16"]}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	// Defaults are alert / 3600 / 0; configured values must win.
	if cfg.ResponseLevel != "enforce" {
		t.Errorf("response-level = %q, want enforce", cfg.ResponseLevel)
	}
	if cfg.MaxMitigationDuration != 1800 {
		t.Errorf("max-mitigation-duration = %d, want 1800 (string-valued delivery must parse)", cfg.MaxMitigationDuration)
	}
	if cfg.ConfidenceMin != 75 {
		t.Errorf("confidence-min = %d, want 75 (string-valued delivery must parse)", cfg.ConfidenceMin)
	}
	if len(cfg.Allowlist) != 2 {
		t.Errorf("allowlist len = %d, want 2", len(cfg.Allowlist))
	}
}
