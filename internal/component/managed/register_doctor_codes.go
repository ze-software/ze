// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/managed.

package managed

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorHubClientMissing      = "doctor-hub-client-missing"
	codeDoctorManagedClientDiverged = "doctor-managed-client-diverged"
	// CodeDoctorManagedDiverged is the doctor diagnostic code "doctor-managed-diverged".
	CodeDoctorManagedDiverged = "doctor-managed-diverged"
	// CodeDoctorHubCertificateUnverified reports a managed client that refused its hub's certificate or has an undefined anchor.
	CodeDoctorHubCertificateUnverified = "doctor-hub-certificate-unverified"
	// CodeDoctorHubHandshakeFailed reports a managed client TLS handshake that failed without a certificate verification error.
	CodeDoctorHubHandshakeFailed = "doctor-hub-handshake-failed"
	// CodeDoctorHubAuthRejected reports a managed client whose hub refused its authentication.
	CodeDoctorHubAuthRejected = "doctor-hub-auth-rejected"
	exampleShowDoctor         = "ze show doctor"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:         codeHubUnreachable,
		Title:        "Management hub unreachable",
		Description:  "This node is configured as a managed client (bus/hub/client), and a TCP probe of the first hub block did not answer. The node still boots and runs its committed config; it will not receive hub-pushed config updates until that hub becomes reachable.",
		Examples:     []string{"ze doctor --json", "ze show diagnostic doctor-hub-unreachable"},
		RelatedCodes: []string{CodeDoctorHubCertificateUnverified, CodeDoctorHubHandshakeFailed, CodeDoctorHubAuthRejected},
	},
	{
		Code:  CodeDoctorHubCertificateUnverified,
		Title: "Hub certificate refused by managed client",
		Description: "The running hub client refused the certificate its hub presented (unknown issuer, wrong host name, outside its validity dates, or another verification failure) or its configuration names a pki ca entry that pki does not define. " +
			"The device holds no hub session, receives no configuration change and keeps retrying while it keeps running the configuration it has.",
		Examples:     []string{exampleShowDoctor, "ze show diagnostic doctor-hub-certificate-unverified"},
		RelatedCodes: []string{CodeDoctorHubHandshakeFailed, CodeDoctorHubAuthRejected, codeHubUnreachable},
	},
	{
		Code:  CodeDoctorHubHandshakeFailed,
		Title: "Hub TLS handshake failed",
		Description: "The running hub client reached the hub's TCP port, and the TLS handshake failed without a certificate verification error (the hub closed the connection, the endpoint does not speak TLS, or the handshake timed out). " +
			"The device holds no hub session, receives no configuration change and keeps retrying while it keeps running the configuration it has.",
		Examples:     []string{exampleShowDoctor, "ze show diagnostic doctor-hub-handshake-failed"},
		RelatedCodes: []string{CodeDoctorHubCertificateUnverified, CodeDoctorHubAuthRejected, codeHubUnreachable},
	},
	{
		Code:  CodeDoctorHubAuthRejected,
		Title: "Hub refused managed client authentication",
		Description: "The running hub client completed TLS with its hub, and the hub refused its authentication (unknown client name, wrong secret, or name already connected). " +
			"The device holds no hub session, receives no configuration change and keeps retrying while it keeps running the configuration it has.",
		Examples:     []string{exampleShowDoctor, "ze show diagnostic doctor-hub-auth-rejected"},
		RelatedCodes: []string{CodeDoctorHubCertificateUnverified, CodeDoctorHubHandshakeFailed, "doctor-managed-client-rejected"},
	},
	{
		Code:  codeDoctorHubClientMissing,
		Title: "Managed client follows no hub",
		Description: "This node is a managed client (meta/instance/managed is true) and its configuration has no bus hub client block, so it connects to no hub and receives no configuration change. " +
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
