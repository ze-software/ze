// Design: docs/architecture/doctor-and-health-checks.md -- event codes
// Related: external_schema_report.go -- the refusal of an external plugin's module, which this mirrors for the build
// Related: last_stop.go -- the record a refused start leaves for the next ze doctor
// Related: ../../../../cmd/ze/hub/build_registrations.go -- boot calls these before it parses the config

package server

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// buildRegistrationSubject names the build in the event: the refusal is about
// the ze binary, which no plugin owns.
const buildRegistrationSubject = "ze-build"

// ReportUnheldRegistration logs and raises doctor-build-registration-unheld: a
// module compiled into ze names a ze:validate function or a ze:reference
// source this ze does not hold, and refusal names the module, the extension,
// the name and the fix (config.RefuseUnheldRegistrations). The caller MUST end
// the start after it: ze does not run a build that would leave such a leaf
// unchecked.
func ReportUnheldRegistration(refusal error) {
	message, fix := refusalParts(refusal, fixShowErrors)
	logger().Error("this ze was built without a registration its YANG names, ze does not start", "error", refusal,
		"fix", fix, "code", diagnostic.CodeDoctorBuildRegistrationUnheld)
	report.RaiseErrorFix(serverEventSource, diagnostic.CodeDoctorBuildRegistrationUnheld, buildRegistrationSubject,
		report.FitMessage(message), busFix(fix, fixShowErrors))
}

// RecordUnheldRegistrationStop writes to the last-stop record that this start
// ended because a compiled-in module names a registration this ze does not
// hold: the refusal's problem and why, and its fix. The next `ze doctor` reads
// it with no daemon running to report doctor-build-registration-unheld; the
// next clean start deletes it. It reports whether the record was written; a
// failed write is logged under the same code.
//
// The caller MUST call it only where the start ends.
func RecordUnheldRegistrationStop(refusal error) bool {
	detail, fix := refusalParts(refusal, "")
	return recordLastStopLogged(LastStopEntry{Reason: LastStopBuildRegistrationUnheld, Detail: detail, Fix: fix})
}
