package pki

import (
	"os"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"

	_ "github.com/ze-software/ze/internal/component/pki/yang"
)

func init() {
	// The local certificate authority root travels with this package, so
	// removing pki removes its readiness check (doctor.go).
	if err := diagnostic.RegisterDoctorCheck(caRootDoctorCheck); err != nil {
		var tb textbuf.Buffer
		tb.Str("pki: doctor check registration failed: ").Err(err).Byte('\n')
		tb.StdErr() //nolint:errcheck // the process is exiting on the next line
		os.Exit(1)
	}

	// The certificates an operator declares in the pki config block travel with
	// this package too, so removing pki removes the check that reads them
	// (doctor_config_certs.go).
	if err := diagnostic.RegisterDoctorCheck(configCertDoctorCheck); err != nil {
		var tb textbuf.Buffer
		tb.Str("pki: configured certificate doctor check registration failed: ").Err(err).Byte('\n')
		tb.StdErr() //nolint:errcheck // the process is exiting on the next line
		os.Exit(1)
	}
}
