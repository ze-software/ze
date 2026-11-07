// Design: docs/guide/configuration.md -- System Host Name: the doctor finding when the kernel name differs
// Related: hostname.go -- ConfiguredHostName, the leaf this check compares
// Related: register_hostname_appliance.go -- registers the check on the appliance build only

package system

import (
	"os"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// codeSystemHostNotApplied names a kernel host name that differs from the
// configured `system host` on the appliance, where Ze owns the kernel name.
// internal/core/diagnostic/codes.go declares it.
const codeSystemHostNotApplied = "doctor-system-host-not-applied"

// kernelHostname reads the kernel host name. A variable so a test can stand in
// a kernel; nothing else assigns it.
var kernelHostname = os.Hostname

// hostnameDoctorCheck is registered only by the appliance build: on a distro
// the operating system owns the host name and a different one is expected.
var hostnameDoctorCheck = diagnostic.DoctorCheck{
	// Runs after the resolv.conf path check (2290, doctor.go).
	Name:         "system-host-name",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2295,
	Component:    doctorComponentSystem,
	Dependencies: []string{"config-tree"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeSystemHostNotApplied},
	Check:        checkKernelHostname,
}

// checkKernelHostname reports a kernel host name that differs from the
// configured `system host`. An unset leaf reports nothing: the kernel keeps
// the image's name then, by design.
func checkKernelHostname(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	configured, ok := ConfiguredHostName(doctorTree(ctx))
	if !ok {
		return nil
	}
	kernel, err := kernelHostname()
	if err != nil {
		return []diagnostic.Diagnostic{hostnameFinding(
			"the kernel host name cannot be read ("+err.Error()+"), so whether system host was applied is unknown",
			configured, "unknown")}
	}
	return hostnameMismatch(configured, kernel)
}

// hostnameMismatch answers the finding for a configured name the kernel does
// not carry, or none when they agree.
func hostnameMismatch(configured, kernel string) []diagnostic.Diagnostic {
	if configured == kernel {
		return nil
	}
	return []diagnostic.Diagnostic{hostnameFinding(
		"the kernel host name is "+kernel+", not the configured system host "+configured+
			": setting it failed; the ze log names the error, and ze must run as root (CAP_SYS_ADMIN) to set it",
		configured, kernel)}
}

// hostnameFinding builds the system host finding.
func hostnameFinding(message, expected, actual string) diagnostic.Diagnostic {
	return diagnostic.Diagnostic{
		Code:     codeSystemHostNotApplied,
		Severity: diagnostic.SeverityWarning,
		Message:  message,
		Path:     "system/host",
		Expected: expected,
		Actual:   actual,
	}
}
