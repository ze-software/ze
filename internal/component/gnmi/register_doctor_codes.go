// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/gnmi.

package gnmi

import "github.com/ze-software/ze/internal/core/diagnostic"

const (
	// CodeDoctorGNMITLSMaterial is the doctor diagnostic code "doctor-gnmi-tls-material".
	CodeDoctorGNMITLSMaterial = "doctor-gnmi-tls-material"
	exampleDoctorJSON         = "ze doctor --json"
)

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:  CodeDoctorGNMITLSMaterial,
		Title: "gNMI TLS material cannot serve",
		Description: "gNMI is set to start and its TLS settings (environment gnmi tls cert and key, or the " +
			"ze.gnmi.tls.cert and ze.gnmi.tls.key overrides) name material it cannot serve: only one of " +
			"the two files is set, a file cannot be read, the pair does not load, or the certificate is " +
			"expired, not yet valid or near expiry. gNMI refuses to start rather than serve its token " +
			"and config in clear text, and the daemon logs the refusal under this code. The finding " +
			"names the file and the fix: restore the file or point the setting at a readable PEM pair, " +
			"then restart Ze. The fix gives the restart for this process: a systemd command only when ze runs " +
			"under ze.service, otherwise the steps for a ze started another way.",
		Examples: []string{exampleDoctorJSON, "ze show diagnostic doctor-gnmi-tls-material"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: gnmi: doctor code registration refused: " + err.Error())
		}
	}
}
