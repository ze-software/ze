// Design: docs/architecture/system-architecture.md -- capabilities without root
// Related: register.go -- the init() that registers daemonCapabilitiesCheck
// Related: registry.go -- RunRuntimeChecks, the only runner of the runtime phase
//
// The capabilities a non-root daemon runs with are declared once, in
// internal/core/privilege (DaemonCapabilities), and the daemon logs at debug
// which of them it lacks (cmd/ze/hub, run). The privilege package is core and
// owns no component, so the check that reads the same inventory back into show
// doctor lives here, as the env-values check does for internal/core/env.

package doctor

import (
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
	// the inventory is context for the per-feature findings other checks report.
	daemonCapabilitiesOrder = 2800
)

// daemonCapabilityInventory answers the daemon's capability inventory. A
// variable so a test answers it without changing the test process's
// capabilities.
var daemonCapabilityInventory = privilege.CheckPrivileges

// daemonCapabilitiesCheck reports the daemon's capability inventory as
// information. A capability of the set the daemon lacks is not a problem by
// itself: a config that uses nothing it serves runs without it, so each
// feature that needs one checks for it and reports its own impact. It runs in
// the runtime phase only: the set that matters is the daemon's, and the local
// ze doctor process, started by the operator with other privileges, would
// measure itself.
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

// checkDaemonCapabilities answers one informational row when the daemon lacks a
// capability of the set, or could not read its set. A daemon that holds every
// one reports nothing.
func checkDaemonCapabilities(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	missing, err := daemonCapabilityInventory()
	if err != nil {
		return []diagnostic.Diagnostic{daemonCapabilitiesUnknown(err)}
	}
	if len(missing) == 0 {
		return nil
	}
	return []diagnostic.Diagnostic{daemonCapabilitiesMissing(missing)}
}

// daemonCapabilitiesMissing writes the row for a set the daemon lacks part of:
// each capability and what it serves, that the features needing one report
// their own impact, and the remedy privilege.CapabilityRemedy gives this
// process, which names a systemd command only under ze.service.
func daemonCapabilitiesMissing(missing []privilege.Capability) diagnostic.Diagnostic {
	names := make([]string, 0, len(missing))
	var tb textbuf.Buffer
	tb.Str("The ze daemon runs without ")
	for i, c := range missing {
		if i > 0 {
			tb.Str(", ")
		}
		tb.Str(c.Name).Str(" (").Str(c.Use).Byte(')')
		names = append(names, c.Name)
	}
	tb.Str(". This is not a problem by itself: each configured feature that needs one checks for it and reports its own impact.")
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorDaemonCapabilityMissing,
		Severity: diagnostic.SeverityInfo,
		Message:  tb.String(),
		Fix:      privilege.CapabilityRemedy(names...),
	}
}

// daemonCapabilitiesUnknown writes the row for a capability set the daemon
// could not read: it says Ze cannot determine them and why. The remedy covers
// the whole set, the one way to be sure the daemon holds what a feature needs.
func daemonCapabilitiesUnknown(err error) diagnostic.Diagnostic {
	var tb textbuf.Buffer
	tb.Str("Ze cannot determine the capabilities the ze daemon runs with: ").Err(err).
		Str(". Each configured feature that needs one checks for it and reports its own impact.")
	return diagnostic.Diagnostic{
		Code:     diagnostic.CodeDoctorDaemonCapabilityMissing,
		Severity: diagnostic.SeverityInfo,
		Message:  tb.String(),
		Fix:      privilege.CapabilityRemedy(),
	}
}
