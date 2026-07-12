// Related: config.go -- ExtractParameters string-delivery coercion

package pppoe

import (
	"testing"
	"time"
)

// TestParseConfig_StringValuedDelivery pins the ACTUAL config format the plugin
// config framework delivers: every YANG leaf value arrives as a JSON string
// ("true", "1000"), not the native type (Tree.values is map[string]string).
// ExtractParameters previously asserted .(bool)/.(float64), so `enabled`
// silently stayed false (the PPPoE subsystem never registered) and every
// numeric leaf fell back to its default.
func TestParseConfig_StringValuedDelivery(t *testing.T) {
	tree := map[string]any{
		"pppoe": map[string]any{
			"enabled":         "true",
			"ac-name":         "my-ac",
			"cookie-timeout":  "10",
			"max-sessions":    "1000",
			"padi-rate-limit": "50",
			"interface": []any{
				map[string]any{"name": "eth0", "max-sessions": "200"},
			},
		},
	}
	p := ExtractParameters(tree)
	if !p.Enabled {
		t.Error(`enabled "true" (string) must parse to true -- the subsystem-disabled bug`)
	}
	if p.ACName != "my-ac" {
		t.Errorf("ac-name = %q, want my-ac", p.ACName)
	}
	if p.CookieTimeout != 10*time.Second {
		t.Errorf("cookie-timeout = %v, want 10s (string-valued)", p.CookieTimeout)
	}
	if p.MaxSessions != 1000 {
		t.Errorf("max-sessions = %d, want 1000 (string-valued)", p.MaxSessions)
	}
	if p.PADIRateLimit != 50 {
		t.Errorf("padi-rate-limit = %d, want 50 (string-valued)", p.PADIRateLimit)
	}
	if len(p.Interfaces) != 1 {
		t.Fatalf("got %d interfaces, want 1", len(p.Interfaces))
	}
	if p.Interfaces[0].MaxSessions != 200 {
		t.Errorf("interface max-sessions = %d, want 200 (string-valued)", p.Interfaces[0].MaxSessions)
	}
}
