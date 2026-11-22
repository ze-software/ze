// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/managed.

package managed

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorHubClientMissing      = "doctor-hub-client-missing"
	codeDoctorManagedClientDiverged = "doctor-managed-client-diverged"
	// CodeDoctorManagedDiverged is the doctor diagnostic code "doctor-managed-diverged".
	CodeDoctorManagedDiverged = "doctor-managed-diverged"
	exampleShowDoctor         = "ze show doctor"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeHubUnreachable,
		Title:       "Management hub unreachable",
		Description: "This node is configured as a managed client (plugin/hub/client) but none of its management hubs answered a TCP probe. The node still boots and runs its committed config; it will not receive hub-pushed config updates until a hub becomes reachable.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-hub-unreachable"},
	},
	{
		Code:  codeDoctorHubClientMissing,
		Title: "Managed client follows no hub",
		Description: "This node is a managed client (meta/instance/managed is true) and its configuration has no plugin hub client block, so it connects to no hub and receives no configuration change. " +
			"A hub can no longer correct this node: the configuration it serves for this node lost the block, and the node applied it.",
		Examples: []string{exampleShowDoctor, "ze show diagnostic doctor-hub-client-missing"},
	},
	{
		Code:  codeDoctorManagedClientDiverged,
		Title: "Hub holds the configuration a managed client kept",
		Description: "This hub holds a copy of the configuration a managed client runs in place of the one the hub serves it, " +
			"under meta/config/client-<name>.conf/held: the client holds and applies nothing the hub sends until an operator resolves it. " +
			"`show fleet divergence name <client>` shows what differs, `update fleet divergence name <client> adopt version <hash>` makes the client's configuration the hub's, " +
			"and `request fleet divergence name <client> revert version <hash> hub-version <hash>` makes the client take the hub's, both hashes as the show answers them. " +
			"A copy for a client no hub server block declares is orphaned: the client cannot connect, so declare it again or remove the copy with the hub stopped.",
		Examples:     []string{exampleShowDoctor, "ze show fleet divergence list", "ze show diagnostic doctor-managed-client-diverged"},
		RelatedCodes: []string{CodeDoctorManagedDiverged, "doctor-managed-client-rejected"},
	},
	{
		Code:  CodeDoctorManagedDiverged,
		Title: "Managed client configuration differs from the hub's last one",
		Description: "This node is a managed client (meta/instance/managed is true) and the configuration it runs differs from the last one its hub gave it: " +
			"the hash of the active configuration is not the baseline in meta/managed/base-version. " +
			"The difference can come from a local change or from a recovery that replaced the configuration. " +
			"`show fleet status` shows both hashes.",
		Examples:     []string{exampleShowDoctor, "ze show fleet status", "ze show diagnostic doctor-managed-diverged"},
		RelatedCodes: []string{codeDoctorHubClientMissing},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: managed: doctor code registration refused: " + err.Error())
		}
	}
}
