// Design: docs/architecture/system-architecture.md -- capabilities without root
// Related: register.go -- the init() that registers daemonCapabilitiesCheck
// Related: registry.go -- RunRuntimeChecks, the only runner of the runtime phase
//
// The capabilities a non-root daemon runs with are declared once, in
// internal/core/privilege (DaemonCapabilities), and the daemon warns at start for
// each one it lacks (cmd/ze/hub, run). The privilege package is core and owns no
// component, so the check that reads the same answer back into show doctor lives
// here, as the env-values check does for internal/core/env.

package doctor

import (
	"strings"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/privilege"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// daemonCapabilitiesCheckName names the check in the doctor registry.
	daemonCapabilitiesCheckName = "daemon-capabilities"

	// daemonCapabilitiesDependency names what the check reads: the daemon's own
	// effective capability set.
	daemonCapabilitiesDependency = "process-capabilities"

	// daemonCapabilitiesOrder prints the row before the env values (envValuesOrder):
	// a missing capability explains findings other checks report.
	daemonCapabilitiesOrder = 2800
)

// daemonPrivilegeWarnings answers the daemon's privilege warnings. A variable so a
// test answers it without changing the test process's capabilities.
var daemonPrivilegeWarnings = privilege.CheckPrivileges

// daemonCapabilitiesCheck reports the capabilities the daemon lacks. It runs in the
// runtime phase only: the set that matters is the daemon's, and the local ze doctor
// process, started by the operator with other privileges, would measure itself.
var daemonCapabilitiesCheck = diagnostic.DoctorCheck{
	Name:         daemonCapabilitiesCheckName,
	Phase:        diagnostic.DoctorPhaseRuntime,
	Order:        daemonCapabilitiesOrder,
	Component:    doctorOwnComponent,
	Dependencies: []string{daemonCapabilitiesDependency},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{diagnostic.CodeDoctorDaemonCapabilityMissing},
	Check:        checkDaemonCapabilities,
}

// checkDaemonCapabilities answers one warning when privilege.CheckPrivileges reports
// anything for the daemon's process: a capability of the set it lacks, or a set it
// could not read. A daemon that runs as root, or holds every capability, reports
// nothing.
func checkDaemonCapabilities(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	warnings := daemonPrivilegeWarnings()
	if len(warnings) == 0 {
		return nil
	}
	return []diagnostic.Diagnostic{daemonCapabilitiesDiagnostic(warnings)}
}

// daemonCapabilitiesDiagnostic writes the warnings as one doctor row: what the daemon
// lacks, what that costs, and how to grant the set.
func daemonCapabilitiesDiagnostic(warnings []string) diagnostic.Diagnostic {
	var tb textbuf.Buffer
	tb.Str("The ze daemon's capability check reports: ")
	for i, w := range warnings {
		if i > 0 {
			tb.Str(" ")
		}
		tb.Str(strings.TrimSpace(w))
	}
	tb.Str(". Each missing capability disables the features named beside it. ").
		Str(privilege.DaemonCapabilitiesAdvice)
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorDaemonCapabilityMissing,
		Severity: diagnostic.SeverityWarning,
		Message:  tb.String(),
		Fix:      privilege.DaemonCapabilitiesFix,
	}
}
