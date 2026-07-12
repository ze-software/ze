package flowtriq

import "testing"

// VALIDATES: string-valued `enabled` ("true") parses to true via cfgBool.
// PREVENTS: regression to m["enabled"].(bool) that left the reporter disabled.
// TestParseConfig_StringValuedDelivery pins the ACTUAL config format the plugin
// framework delivers: every YANG leaf value arrives as a JSON string ("true"),
// not the native type. The prior parser used m["enabled"].(bool), so `enabled`
// silently stayed false and the reporter never ran. This test feeds string-valued
// leaves and asserts the CONFIGURED values (which differ from the defaults).
func TestParseConfig_StringValuedDelivery(t *testing.T) {
	data := `{"ddos":{"flowtriq":{` +
		`"enabled":"true","api-key":"secret-key","node-uuid":"abc-123",` +
		`"api-base":"https://example.test/api"}}}`
	cfg, err := ParseConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Enabled {
		t.Error(`enabled "true" (string) must parse to true -- the reporter-disabled bug`)
	}
	if cfg.APIKey != "secret-key" {
		t.Errorf("api-key = %q, want secret-key", cfg.APIKey)
	}
	if cfg.NodeUUID != "abc-123" {
		t.Errorf("node-uuid = %q, want abc-123", cfg.NodeUUID)
	}
	// Default is https://flowtriq.com/api/v1; configured value must win.
	if cfg.APIBase != "https://example.test/api" {
		t.Errorf("api-base = %q, want https://example.test/api", cfg.APIBase)
	}
}
