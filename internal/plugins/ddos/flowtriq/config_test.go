package flowtriq

import (
	"strings"
	"testing"
)

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

// flowtriqConfig answers the config section the plugin framework delivers for
// an enabled reporter that posts to apiBase.
func flowtriqConfig(apiBase string) string {
	return `{"ddos":{"flowtriq":{"enabled":"true","api-key":"secret-key",` +
		`"node-uuid":"abc-123","api-base":"` + apiBase + `"}}}`
}

// TestValidate_RefusesAnAPIBaseWithoutHTTPS walks the two calls the plugin's
// verify makes (parseSections in register.go: ParseConfig, then Validate) over
// an api-base that does not use https.
//
// VALIDATES: verify refuses an api-base whose scheme is not https, or that
// names no host, with an error naming the leaf and ending with the command
// that fixes it. A loopback http URL is refused too: nothing in Ze posts to a
// local endpoint, and the unit tests reach their httptest servers through
// newClient, not through verify.
// PREVENTS: the api-key bearer token going out in clear text over http.
func TestValidate_RefusesAnAPIBaseWithoutHTTPS(t *testing.T) {
	cases := []struct {
		name    string
		apiBase string
		fix     string
	}{
		{"http", "http://flowtriq.com/api/v1", "set ddos flowtriq api-base https://flowtriq.com/api/v1"},
		{"http to loopback", "http://127.0.0.1:8080/api", "set ddos flowtriq api-base https://127.0.0.1:8080/api"},
		{"another scheme", "ftp://flowtriq.com/api/v1", "set ddos flowtriq api-base https://flowtriq.com/api/v1"},
		{"no scheme", "flowtriq.com/api/v1", "set ddos flowtriq api-base https://flowtriq.com/api/v1"},
		{"no host", "https:///api/v1", "set ddos flowtriq api-base https://flowtriq.com/api/v1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseConfig(flowtriqConfig(tc.apiBase))
			if err != nil {
				t.Fatal(err)
			}
			err = cfg.Validate()
			if err == nil {
				t.Fatalf("api-base %q was accepted; it must be refused", tc.apiBase)
			}
			message := err.Error()
			if !strings.Contains(message, "api-base") {
				t.Errorf("error does not name the api-base leaf:\n%s", message)
			}
			if !strings.HasSuffix(message, "fix: "+tc.fix) {
				t.Errorf("error does not end with %q:\n%s", "fix: "+tc.fix, message)
			}
		})
	}
}

// TestValidate_AcceptsAnHTTPSAPIBase is the positive half: the default and a
// configured https URL pass verify, with the scheme in either case.
//
// VALIDATES: the https check refuses only what is not https.
// PREVENTS: a check that refuses every api-base.
func TestValidate_AcceptsAnHTTPSAPIBase(t *testing.T) {
	for _, apiBase := range []string{"https://flowtriq.com/api/v1", "HTTPS://example.test/api"} {
		cfg, err := ParseConfig(flowtriqConfig(apiBase))
		if err != nil {
			t.Fatal(err)
		}
		if err := cfg.Validate(); err != nil {
			t.Errorf("api-base %q was refused: %v", apiBase, err)
		}
	}
	if err := DefaultConfig().Validate(); err != nil {
		t.Errorf("the default config was refused: %v", err)
	}
}
