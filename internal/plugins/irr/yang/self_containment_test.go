package yang

import (
	"strings"
	"testing"
)

// TestIRRSchemaOwnsItsSurface proves the irr plugin's own modules declare its
// whole surface, so removing the plugin removes the config container and the
// commands with it. Method: read the embedded module text.
func TestIRRSchemaOwnsItsSurface(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-irr:show-irr"`,
		`ze:command "ze-irr:update-irr-all"`,
		`ze:command "ze-irr:update-irr-asn"`,
		`ze:command "ze-irr:update-irr-as-set"`,
		`ze:command "ze-irr:clear-irr-asn"`,
		`ze:command "ze-irr:clear-irr-as-set"`,
	} {
		if !strings.Contains(ZeIrrCmdYANG, want) {
			t.Errorf("ze-irr-cmd.yang must declare %q", want)
		}
	}
	for _, want := range []string{
		`augment "/sys:system"`,
		"container irr",
		`range "0 | 60..86400"`,
		`default "3600"`,
		`default "whois.radb.net"`,
	} {
		if !strings.Contains(ZeIrrConfYANG, want) {
			t.Errorf("ze-irr-conf.yang must declare %q", want)
		}
	}
}
