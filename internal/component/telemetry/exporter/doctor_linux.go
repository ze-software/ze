//go:build ze_telemetry && linux

// Design: docs/architecture/core-design.md -- the readiness check the Prometheus exporter owns
// Related: register_linux.go -- the init() that installs the registration below
// Related: exporter.go -- Start, which runs the OS collectors this check stands for
// Related: internal/component/telemetry/collector -- the procfs readers behind those collectors
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. The OS collectors this exporter starts read procfs, so
// a /proc/stat this process cannot read is this package's question
// (ai/patterns/registration.md, "Doctor Check Registry"): it is owned here now
// and dropping the exporter drops its check with it. The tag pair matches the
// exporter's own: a build without ze_telemetry refuses a `telemetry` block
// before any check sees it, and off Linux there is no procfs to read.

package exporter

import (
	"os"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// codeTelemetryProcfs names a /proc/stat the exporter cannot read.
// internal/core/diagnostic/codes.go declares it, so `ze explain
// doctor-telemetry-procfs` answers.
const codeTelemetryProcfs = "doctor-telemetry-procfs"

// procfsReadFile is the probe checkTelemetryProcfs runs. It is a variable so a
// test stands in a procfs that refuses the read; nothing else assigns it.
var procfsReadFile = os.ReadFile

// telemetryProcfsDoctorCheck is the registration register_linux.go installs.
//
// Order 2038 reproduces the sequence the doctor runner printed before the check
// moved onto the registry: after the DNS and TACACS probes and before the
// sysctl check, which internal/component/sysctl registered at 2040. The
// doctor-owned table (internal/component/doctor/doctor_checks.go) spaces that
// scale by tens, so 2038 is the slot that keeps this check ahead of sysctl.
var telemetryProcfsDoctorCheck = diagnostic.DoctorCheck{
	Name:         "telemetry-procfs",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2038,
	Component:    "telemetry",
	Dependencies: []string{"procfs"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeTelemetryProcfs},
	Check:        checkTelemetryProcfs,
}

// checkTelemetryProcfs warns when the Prometheus exporter is enabled and this
// process cannot read /proc/stat, the first file its OS collectors open.
//
// A nil tree is the missing-config phase, which this check does not run in, and
// a context carrying anything else is a runner defect the runner's own type
// assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkTelemetryProcfs(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	prom := tree.GetContainerPath("telemetry/prometheus")
	if prom == nil {
		return nil
	}
	if enabled, _ := prom.Get("enabled"); enabled != "true" {
		return nil
	}
	path := kernelcap.ProcPath("stat")
	_, err := procfsReadFile(path)
	if err == nil {
		return nil
	}
	var tb textbuf.Buffer
	return []diagnostic.Diagnostic{{
		Code:     codeTelemetryProcfs,
		Severity: diagnostic.SeverityWarning,
		Message:  tb.Str("telemetry: cannot read ").Str(path).Str(": ").Err(err).String(),
		Path:     path,
	}}
}
