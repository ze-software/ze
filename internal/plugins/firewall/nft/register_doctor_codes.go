// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/plugins/firewall/nft.

package firewallnft

import "github.com/ze-software/ze/internal/core/diagnostic"

// codeFirewallNftables names a firewall config on this backend whose kernel
// has not loaded nf_tables. internal/core/diagnostic/codes.go declares it, so
// `ze show diagnostic doctor-firewall-nftables` answers.
const codeFirewallNftables = "doctor-firewall-nftables"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeFirewallNftables,
		Title:       "nftables kernel support unavailable",
		Description: "The firewall is configured with the kernel backend but nf_tables kernel support is not loaded.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-firewall-nftables"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: firewallnft: doctor code registration refused: " + err.Error())
		}
	}
}
