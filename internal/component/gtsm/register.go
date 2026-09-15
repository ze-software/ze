// Design: ai/patterns/registration.md -- Doctor Check Registry
// Overview: doctor.go -- the check this file installs, and the seam the reactor fills
//
// This package is called by the BGP reactor rather than discovered through
// the component registry, so the one registration it makes is its readiness
// check. A refusal is a programmer error in the table beside the call, and
// the daemon must not start one readiness check short of what it declares.

package gtsm

import (
	"os"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	if err := diagnostic.RegisterDoctorCheck(kernelStateDoctorCheck()); err != nil {
		var tb textbuf.Buffer
		tb.Str("gtsm: doctor check registration failed: ").Err(err).Byte('\n')
		tb.StdErr() //nolint:errcheck // the process is exiting on the next line
		os.Exit(1)
	}
}
