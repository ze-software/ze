// Design: docs/architecture/api/process-protocol.md -- firewall table contribution
// Related: contribution.go -- the replacement and withdrawal that raise these warnings
// Related: register_contribution.go -- registers this check and the observer that clears them
//
// The contributed-tables check answers for the kernel state of the tables
// plugins contributed through the engine operation. A failure that can leave
// that state wrong raises a warning on the report bus under
// contributionReportSource, and the next successful reconcile clears it. The
// report bus is the record the check reads, so no second copy is kept here.

package firewall

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
)

// contributionReportSource is the report-bus source of every contributed-table
// warning.
const contributionReportSource = "firewall-contributed-tables"

// contributionSubjectPlugin prefixes the subject of a warning about one owner's
// contributed tables.
const contributionSubjectPlugin = "plugin "

// contributionInspect is the command that lists the applied tables, the fix of
// a finding whose record the report bus evicted.
const contributionInspect = "ze show firewall ruleset"

// contributionDoctorCheck is the doctor-check registration, installed from
// register_contribution.go's init().
var contributionDoctorCheck = diagnostic.DoctorCheck{
	Name:         "firewall-contributed-tables",
	Phase:        diagnostic.DoctorPhaseRuntime,
	Order:        745,
	Component:    reportSourceFirewall,
	Dependencies: []string{"plugin-firewall-tables"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeDoctorFirewallContributedTables},
	Check:        checkContributedTables,
}

// checkContributedTables reports every contributed-table warning the daemon
// holds. It reads no configuration, because a contribution is not config. Safe
// for concurrent use; it changes nothing.
func checkContributedTables(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	var findings []diagnostic.Diagnostic
	for _, issue := range report.Warnings() {
		if issue.Source != contributionReportSource {
			continue
		}
		findings = append(findings, diagnostic.Diagnostic{
			Code: issue.Code, Severity: diagnostic.SeverityWarning,
			Message: issue.Message, Fix: issue.Fix,
		})
	}
	if len(findings) != 0 {
		return findings
	}
	// An evicted warning is unknown state, never evidence the tables are
	// right.
	status, reason := report.HealthProbeDegraded(codeDoctorFirewallContributedTables)()
	if status != health.StatusHealthy {
		return []diagnostic.Diagnostic{{
			Code: codeDoctorFirewallContributedTables, Severity: diagnostic.SeverityWarning,
			Message: "the state of the plugin-contributed firewall tables is unknown: " + reason,
			Fix:     contributionInspect,
		}}
	}
	return findings
}

// clearContributionWarnings is the commit observer that clears every
// contributed-table warning once a reconcile succeeds: the kernel then holds
// the registry's merged set, in which a withdrawn owner holds nothing and a
// restored owner holds its previous set. It runs inside reconcileMu and calls
// only the report bus, never back into this package.
func clearContributionWarnings() func(error) {
	return func(err error) {
		if err != nil {
			return
		}
		for _, issue := range report.Warnings() {
			if issue.Source != contributionReportSource {
				continue
			}
			report.ClearWarning(issue.Source, issue.Code, issue.Subject)
		}
	}
}
