package observe

import "testing"

// VALIDATES: string-valued YANG leaf delivery parses into Config numeric leaves.
// PREVENTS: regression to int/float64-only toInt that silently used defaults.
// TestParseConfig_StringValuedDelivery pins the ACTUAL config format the plugin
// framework delivers: every YANG leaf value arrives as a JSON string ("100",
// "7200"), not the native JSON number. The prior toInt handled only int/float64,
// so every leaf silently fell back to its default. This test feeds string-valued
// leaves and asserts the CONFIGURED values (which differ from the defaults).
func TestParseConfig_StringValuedDelivery(t *testing.T) {
	data := `{"ddos":{"observe":{"incident-ring-size":"100","stale-incident-timeout":"7200"}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	// Defaults are 1000 / 3600; configured values must win.
	if cfg.IncidentRingSize != 100 {
		t.Errorf("incident-ring-size = %d, want 100 (string-valued delivery must parse)", cfg.IncidentRingSize)
	}
	if cfg.StaleIncidentTimeout != 7200 {
		t.Errorf("stale-incident-timeout = %d, want 7200 (string-valued delivery must parse)", cfg.StaleIncidentTimeout)
	}
}
