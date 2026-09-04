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
}
