// Design: docs/features/ai-first.md -- the readiness check this component owns
// Overview: register.go -- the init() that installs the registration below
// Related: doctor_linux.go, doctor_other.go -- the writability probe, per platform
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. A check that probes /proc/sys belongs to the package
// that writes /proc/sys (ai/patterns/registration.md, "Doctor Check Registry"),
// so it is owned here now and dropping this component drops its check with it.

package sysctl

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/kernelcap"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// doctorSysctlProcfsCode names a procfs tree this process cannot write.
const doctorSysctlProcfsCode = "doctor-sysctl-procfs"

// sysctlProcfsDoctorCheck is the registration register.go installs.
//
// Order 2040 reproduces the sequence the doctor runner printed before the check
// moved onto the registry. It ran after the disk-space check and before the
// clock-skew check, and those two are registered at 2020 and 2100
// (internal/component/doctor/doctor_checks.go). The 700-1000 band that file
// names is for a check the runner ran at the phase dispatch itself; this one
// ran later, so it sorts later.
func sysctlProcfsDoctorCheck() diagnostic.DoctorCheck {
	return diagnostic.DoctorCheck{
		Name:         "sysctl-procfs",
		Phase:        diagnostic.DoctorPhasePostConfig,
		Order:        2040,
		Component:    "sysctl",
		Dependencies: []string{"procfs"},
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Codes:        []string{doctorSysctlProcfsCode},
		Check:        checkSysctlProcfs,
	}
}

// checkSysctlProcfs warns when the config carries a sysctl block and this
// process cannot write the procfs tree every sysctl is applied through.
//
// A nil tree is the missing-config phase, which this check does not run in, and
// a context carrying anything else is a runner defect the runner's own type
// assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkSysctlProcfs(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	if tree.GetContainer("sysctl") == nil {
		return nil
	}
	path := kernelcap.ProcPath("sys")
	err := procSysWritable(path)
	if err == nil {
		return nil
	}
	var tb textbuf.Buffer
	return []diagnostic.Diagnostic{{
		Code:     doctorSysctlProcfsCode,
		Severity: diagnostic.SeverityWarning,
		Message:  tb.Str("sysctl: ").Str(path).Str(" is not writable: ").Err(err).String(),
		Path:     path,
	}}
}
