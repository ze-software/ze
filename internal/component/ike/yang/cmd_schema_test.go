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
		`ze:command "ze-ike:show-vpn-ipsec-sa"`,
		`ze:command "ze-ike:show-vpn-ipsec-status"`,
		`ze:command "ze-ike:show-vpn-ipsec-peer"`,
		`ze:command "ze-ike:show-vpn-ipsec-dataplane-sa"`,
		`ze:command "ze-ike:show-vpn-ipsec-dataplane-policy"`,
		`ze:command "ze-ike:show-vpn-ipsec-dataplane-drift"`,
		`ze:command "ze-ike:monitor-vpn-ipsec"`,
		`ze:command "ze-ike:clear-vpn-ipsec-sa"`,
		"container vpn",
		"container ipsec",
		"container dataplane",
	} {
		if !strings.Contains(ZeIPsecCmdYANG, want) {
			t.Errorf("ze-ipsec-cmd.yang must declare %q so removing the ike component removes the vpn ipsec surface", want)
		}
	}
}
