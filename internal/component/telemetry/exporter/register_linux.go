//go:build ze_telemetry && linux

// Design: docs/architecture/core-design.md -- the Linux-only registrations of the Prometheus exporter
// Related: doctor_linux.go -- checkTelemetryProcfs, the check registered here
// Related: cmd/ze/hub/register_telemetry.go -- the seam that links this package into the hub
//
// The check reads /proc/stat, so it exists on Linux only and registers from
// here; off Linux nothing registers, which is what the doctor runner's stub
// for those platforms reported. The tag pair matches the exporter's own.

package exporter

import (
	"fmt"
	"os"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

func init() {
	// A refusal is a programmer error in the declaration beside it and stops
	// the process rather than leaving `ze doctor` quietly short of a check.
	if err := diagnostic.RegisterDoctorCheck(telemetryProcfsDoctorCheck); err != nil {
		fmt.Fprintf(os.Stderr, "telemetry: doctor check registration failed: %v\n", err)
		os.Exit(1)
	}
}
