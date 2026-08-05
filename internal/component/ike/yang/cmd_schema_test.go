package yang

import (
	"strings"
	"testing"
)

// TestIPsecCmdSchemaOwnsShowVPNIPsec is the owner half of the self-containment
// invariant: the central show schema must NOT declare `show vpn ipsec ...`, and
// this package MUST. See ai/rules/plugins.md.
func TestIPsecCmdSchemaOwnsShowVPNIPsec(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:vpn-ipsec-sa"`,
		`ze:command "ze-show:vpn-ipsec-status"`,
		`ze:command "ze-show:vpn-ipsec-peer"`,
		`ze:command "ze-show:vpn-ipsec-dataplane-sa"`,
		`ze:command "ze-show:vpn-ipsec-dataplane-policy"`,
		`ze:command "ze-show:vpn-ipsec-dataplane-drift"`,
		`ze:command "ze-monitor:vpn-ipsec"`,
		`ze:command "ze-clear:vpn-ipsec-sa"`,
		"container vpn",
		"container ipsec",
		"container dataplane",
	} {
		if !strings.Contains(ZeIPsecCmdYANG, want) {
			t.Errorf("ze-ipsec-cmd.yang must declare %q so removing the ike component removes the vpn ipsec surface", want)
		}
	}
}
