package pki

import (
	"github.com/ze-software/ze/internal/core/diagnostic"

	// The ze:reference sources a consumer's PKI name leaf names.
	_ "github.com/ze-software/ze/internal/component/pki/pkiref"
	_ "github.com/ze-software/ze/internal/component/pki/yang"
)

func init() {
	// The local certificate authority root travels with this package, so
	// removing pki removes its readiness check (doctor.go).
	if err := diagnostic.RegisterDoctorCheck(caRootDoctorCheck); err != nil {
		panic("BUG: pki: doctor check registration failed: " + err.Error())
	}

	// The certificates an operator declares in the pki config block travel with
	// this package too, so removing pki removes the check that reads them
	// (doctor_config_certs.go).
	if err := diagnostic.RegisterDoctorCheck(configCertDoctorCheck); err != nil {
		panic("BUG: pki: configured certificate doctor check registration failed: " + err.Error())
	}
}
