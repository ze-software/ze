// Design: docs/architecture/doctor-and-health-checks.md -- host trend checks
// Related: trend.go -- the published state these checks read
// Related: register_kernel_memory.go -- a rule registering its own code and check

package trend

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// The monitor's own codes. A rule's warning code is declared by the rule's
// own file.
const (
	// CodeNotRunning: no trend monitor runs in the daemon. The hub logs it when
	// Start refuses, and the monitor check reports it.
	CodeNotRunning = "doctor-host-trend-not-running"
	// CodeUnreadable: a rule's reader cannot read its resource.
	CodeUnreadable = "doctor-host-trend-unreadable"
)

const (
	monitorCheckName = "host-trend-monitor"
	trendComponent   = "host"
	// trendDependency is what every trend check reports on: the kernel's
	// counters under /proc, which the rules' readers sample.
	trendDependency = "procfs"
	// trendCheckOrder places the trend checks among the runtime checks; the
	// order only sorts the rows.
	trendCheckOrder = 800
	// restartFix restarts the daemon, whose system start starts the monitor.
	restartFix = "ze signal restart"
)

const notRunningMessage = "The host trend monitor is not running, so Ze does not watch host resources " +
	"for growth that ends in exhaustion. The daemon starts it when it starts, so a restart starts it again."

func init() {
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        CodeNotRunning,
		Title:       "The host trend monitor is not running",
		Description: "The daemon starts the host trend monitor at its system start. When it is not running, no trend rule watches the host, so a resource that runs out slowly is not reported before the box fails. Restart the daemon to start it again.",
		Examples:    []string{restartFix, trendFix},
	}); err != nil {
		panic("BUG: host trend not-running code registration: " + err.Error())
	}
	if err := diagnostic.Register(diagnostic.CodeMeta{
		Code:        CodeUnreadable,
		Title:       "A host trend rule cannot read its resource",
		Description: "A trend rule could not read the kernel file it samples, so it cannot see the resource grow. The finding names the file and the error. The rule reads again at its next sample and clears the warning when the read succeeds.",
		Examples:    []string{trendFix},
		HostVerdict: true,
	}); err != nil {
		panic("BUG: host trend unreadable code registration: " + err.Error())
	}
	if err := diagnostic.RegisterEventCode(diagnostic.EventCode{Code: CodeUnreadable}); err != nil {
		panic("BUG: host trend unreadable event registration: " + err.Error())
	}
	if err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name: monitorCheckName, Phase: diagnostic.DoctorPhaseRuntime, Order: trendCheckOrder,
		Component: trendComponent, Dependencies: []string{trendDependency},
		Platforms: []string{diagnostic.DoctorPlatformAny},
		Codes:     []string{CodeNotRunning, CodeUnreadable},
		Check:     checkMonitor,
	}); err != nil {
		panic("BUG: host trend monitor doctor registration: " + err.Error())
	}
}

// checkMonitor reports a monitor that is not running, and every rule whose
// latest read failed. It declares the unreadable code for every rule, so one
// check literal holds it.
func checkMonitor(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	statuses, err := Rules()
	if err != nil {
		return []diagnostic.Diagnostic{{Code: CodeNotRunning, Severity: diagnostic.SeverityWarning,
			Message: notRunningMessage, Fix: restartFix}}
	}
	var findings []diagnostic.Diagnostic
	for i := range statuses {
		status := &statuses[i]
		source, cause, failed := status.Failure()
		if !failed {
			continue
		}
		findings = append(findings, diagnostic.Diagnostic{Code: CodeUnreadable,
			Severity: diagnostic.SeverityWarning, Message: unreadableText(status.Name(), source, cause),
			Fix: trendFix})
	}
	return findings
}

// ruleCheck answers the runtime doctor check a rule's own file registers: the
// rule's standing warning, with the message and the fix the bus holds, and no
// row while the rule is quiet. With no monitor running it answers nothing:
// the monitor check reports that, once, rather than every rule check.
func ruleCheck(name string) diagnostic.DoctorCheckFunc {
	return func(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
		statuses, err := Rules()
		if err != nil {
			return nil
		}
		for i := range statuses {
			status := &statuses[i]
			if status.Name() != name {
				continue
			}
			message, fix, warned := status.Warning()
			if !warned {
				return nil
			}
			return []diagnostic.Diagnostic{{Code: status.Code(), Severity: diagnostic.SeverityWarning,
				Message: message, Fix: fix}}
		}
		// Start copies every rule registered at init, so a running monitor
		// always lists the rule whose file registered this check.
		panic("BUG: host trend rule " + name + " has a doctor check and no published state")
	}
}

// unreadableMessage words a failed read for the report bus.
func unreadableMessage(name string, failure unreadableReading) string {
	return unreadableText(name, failure.source, failure.err.Error())
}

// unreadableText words a failed read: what cannot be read, why, and what it costs.
func unreadableText(name, source, cause string) string {
	return "The host trend rule " + name + " cannot read " + source + ": " + cause +
		". Until it can, Ze does not see this resource grow toward exhaustion."
}
