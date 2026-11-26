// Design: docs/architecture/exabgp-bridge.md -- doctor diagnostic codes owned by internal/plugins/exabgp/bridgerun.
// Related: fleet.go -- invalid base64 UPDATE event warning site
package bridgerun

import "github.com/ze-software/ze/internal/core/diagnostic"

// codeDoctorInvalidBase64Update reports that the ExaBGP wrapper received a
// malformed positional counted `base64` UPDATE event line and refused it.
const codeDoctorInvalidBase64Update = "doctor-exabgp-invalid-base64-update"

var invalidBase64UpdateCodeMeta = diagnostic.CodeMeta{
	Code:  codeDoctorInvalidBase64Update,
	Title: "ExaBGP wrapper refused an invalid base64 UPDATE event",
	Description: "The ExaBGP wrapper received a positional counted `upd` event line that failed " +
		"syntax, bounds, or base64 validation and refused to forward it to operator scripts. " +
		"Inspect the warning error field and verify that the daemon and `ze exabgp plugin` binary " +
		"run the same version.",
	Examples: []string{
		"ze show doctor",
		"ze show warnings",
		"ze show diagnostic " + codeDoctorInvalidBase64Update,
	},
}

func init() {
	if err := diagnostic.Register(invalidBase64UpdateCodeMeta); err != nil {
		panic("BUG: bridgerun: register invalid-base64-update diagnostic code: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: codeDoctorInvalidBase64Update}); err != nil {
		panic("BUG: bridgerun: register invalid-base64-update event code: " + err.Error())
	}
}
