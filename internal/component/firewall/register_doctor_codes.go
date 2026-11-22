// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/firewall.

package firewall

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorFirewallContributedTables = "doctor-firewall-contributed-tables"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeDoctorFirewallContributedTables,
		Title:       "Firewall tables a plugin contributed may not match the kernel",
		Description: "A plugin compiled into ze contributes nft tables it reserved through the firewall engine operation, and a step that keeps the kernel in line with that contribution failed. Either a refused replacement could not put the plugin's previous set back, so the kernel may hold neither set, or the plugin ended and its tables could not be removed. The finding names the plugin and the backend's answer. `ze show firewall ruleset` lists what the kernel holds. Restarting the plugin contributes its set again, and the finding clears at the next firewall reconcile that succeeds, which also removes the tables of a plugin that ended.",
		Examples:    []string{"ze show doctor", "ze show firewall ruleset", "ze show diagnostic doctor-firewall-contributed-tables"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: firewall: doctor code registration refused: " + err.Error())
		}
	}
}
