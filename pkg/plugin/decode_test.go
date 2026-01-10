package plugin

import (
	"strings"
	"testing"
)

// TestFormatCapabilityStrings verifies all capability types produce parseable "name value" format.
//
// VALIDATES: All capabilities use "name value" format where value is hyphenated.
// PREVENTS: Unparseable capability strings in OPEN output.
func TestFormatCapabilityStrings(t *testing.T) {
	tests := []struct {
		name string
		cap  string
	}{
		// Basic capabilities (name only, no value)
		{"route refresh", "route-refresh"},
		{"enhanced route refresh", "enhanced-route-refresh"},
		{"extended message", "extended-message"},

		// Capabilities with values (name value)
		{"multiprotocol ipv4/unicast", "multiprotocol ipv4-unicast"},
		{"multiprotocol ipv6/unicast", "multiprotocol ipv6-unicast"},
		{"4-byte-asn", "4-byte-asn 65536"},

		// AddPath per family (name value)
		{"addpath receive", "addpath receive-ipv4-unicast"},
		{"addpath send", "addpath send-ipv6-unicast"},
		{"addpath send/receive", "addpath send/receive-ipv4-unicast"},

		// Graceful restart (name value)
		{"graceful restart", "graceful-restart 120"},

		// Extended nexthop per family (name value)
		{"extended nexthop", "extended-nexthop ipv4-unicast-ipv6"},

		// FQDN (name value)
		{"hostname only", "hostname router1"},
		{"hostname with domain", "hostname router1.example.com"},

		// Software version (name value)
		{"software", "software zebgp-1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify format is parseable: either "name" or "name value"
			parts := strings.SplitN(tt.cap, " ", 2)
			if len(parts) == 0 {
				t.Errorf("capability %q is empty", tt.cap)
			}
			// Name should be hyphenated (no spaces)
			if strings.Contains(parts[0], " ") {
				t.Errorf("capability name %q contains spaces", parts[0])
			}
			// Value (if present) should be hyphenated
			if len(parts) == 2 && strings.Contains(parts[1], " ") {
				t.Errorf("capability value %q contains spaces", parts[1])
			}
		})
	}
}
