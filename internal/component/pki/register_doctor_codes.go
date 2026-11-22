// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/pki.

package pki

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	codeDoctorPKICARootMissing = "doctor-pki-ca-root-missing"
	codeDoctorPKICARootExpiry  = "doctor-pki-ca-root-expiry"
	codeDoctorPKICert          = "doctor-pki-cert"
	exampleDoctorJSON          = "ze doctor --json"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  codeDoctorPKICARootMissing,
		Title: "Local certificate authority root not stored",
		Description: "Ze issues its own components' certificates from a local certificate authority root it generates once and keeps. " +
			"No root is stored, so the next daemon start generates one. " +
			"Every copy of the previous root that an operator distributed stops working at that point, because the leaves the new root issues carry a different issuer. " +
			"Export the new root with `show pki local-ca pem` and give it to each client configured to trust this node.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-pki-ca-root-missing", "show pki local-ca pem"},
		RelatedCodes: []string{codeDoctorPKICARootExpiry},
	},
	{
		Code:  codeDoctorPKICARootExpiry,
		Title: "Local certificate authority root is expiring",
		Description: "The stored local certificate authority root expires within 90 days, or has expired already. " +
			"The window is 90 days rather than the 30 a configured certificate gets, because the root has to be replaced on every client that trusts it and Ze distributes it by hand. " +
			"An expired root is an error: every leaf it issued is refused, so plugin and fleet connections fail.",
		Examples:     []string{exampleDoctorJSON, "ze show diagnostic doctor-pki-ca-root-expiry", "show pki local-ca pem"},
		RelatedCodes: []string{codeDoctorPKICARootMissing},
	},
	{
		Code:        codeDoctorPKICert,
		Title:       "PKI certificate invalid",
		Description: "A PKI CA or device certificate is missing, malformed, expired, or not yet valid.",
		Examples:    []string{exampleDoctorJSON, "ze show diagnostic doctor-pki-cert"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: pki: doctor code registration refused: " + err.Error())
		}
	}
}
