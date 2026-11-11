// Design: docs/architecture/doctor-and-health-checks.md -- recent events on the report bus
// Related: register.go -- the init() that registers recentEventsCheck
// Related: registry.go -- RunRuntimeChecks, the only runner of the runtime phase
//
// The report bus (internal/core/report) already keeps what went wrong
// recently: a bounded ring of error events and the set of active warnings,
// each under its producer's code. show errors and show warnings print that
// store. This check reads the same store, and adds none: a transient problem
// (a peer reset that recovered, one failed retry) is raised there once, and
// show doctor reports it here, so the operator who runs doctor meets it.

package doctor

import (
	"time"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// recentEventsCheckName names the check in the doctor registry.
	recentEventsCheckName = "recent-events"

	// recentEventsDependency names what the check reads: the report bus.
	recentEventsDependency = "report-bus"

	// recentEventsOrder sits above every order the tree registers today, so
	// the rows about past events print after the findings about present state.
	recentEventsOrder = 3000

	// recentEventsFixErrors and recentEventsFixWarnings are the commands that
	// list every event behind a row, with its time and detail.
	recentEventsFixErrors   = "show errors"
	recentEventsFixWarnings = "show warnings"

	// recentEventsTimeLayout writes the last time a row's event happened.
	recentEventsTimeLayout = time.RFC3339
)

// recentEventsCheck reports the report bus's recent error events and active
// warnings. It runs in the runtime phase only: the bus lives in the daemon,
// and in the local ze doctor process it is always empty.
//
// It declares doctor-recent-events and, through DeclaresEventCodes, every
// event code an owner registered with diagnostic.RegisterEventCode: this
// check is what reports those transient codes, so it is their declaring
// check. A row carries the event's own code whenever `ze show diagnostic`
// can describe that code: the code a producer raised names the problem
// better than this check's. Like the plugin bridge, the check also renders
// codes another check declares; a bus code with no metadata is reported under
// doctor-recent-events instead, its own code named in the message, so no
// event is dropped and no row carries a code the operator cannot look up.
var recentEventsCheck = diagnostic.DoctorCheck{
	Name:               recentEventsCheckName,
	Phase:              diagnostic.DoctorPhaseRuntime,
	Order:              recentEventsOrder,
	Component:          doctorOwnComponent,
	Dependencies:       []string{recentEventsDependency},
	Platforms:          []string{diagnostic.DoctorPlatformAny},
	Codes:              []string{diagnostic.CodeDoctorRecentEvents},
	DeclaresEventCodes: true,
	Check:              checkRecentEvents,
}

// recentEventsKey groups the events one row reports: one producer's code on
// one subject, for one kind of record.
type recentEventsKey struct {
	severity report.Severity
	source   string
	code     string
	subject  string
}

// recentEventsGroup is one row in the making: the newest event of its key
// with the fix its raise carried, the time it last happened, and how many
// events the key holds.
type recentEventsGroup struct {
	key     recentEventsKey
	message string
	fix     string
	last    time.Time
	count   int
}

// checkRecentEvents answers one warning row per (kind, source, code,
// subject) on the report bus, newest first: error events, then active
// warnings. A row is a warning whatever the event's severity, because the
// event already happened and its owner may have recovered, so it never makes
// the box read not ready.
func checkRecentEvents(diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	groups := groupRecentEvents(nil, report.Errors(0))
	groups = groupRecentEvents(groups, report.Warnings())
	if len(groups) == 0 {
		return nil
	}

	diags := make([]diagnostic.Diagnostic, 0, len(groups))
	for i := range groups {
		diags = append(diags, recentEventsDiagnostic(&groups[i]))
	}
	return diags
}

// groupRecentEvents folds issues, newest first as the bus answers them, into
// groups, keeping first-seen order. The newest event of a group therefore
// gives its message and its time. The bus bounds issues (its ring and warning
// caps), so the fold is bounded too.
func groupRecentEvents(groups []recentEventsGroup, issues []report.Issue) []recentEventsGroup {
	index := make(map[recentEventsKey]int, len(issues))
	for i := range issues {
		issue := &issues[i]
		key := recentEventsKey{severity: issue.Severity, source: issue.Source, code: issue.Code, subject: issue.Subject}
		if at, seen := index[key]; seen {
			groups[at].count++
			continue
		}
		index[key] = len(groups)
		groups = append(groups, recentEventsGroup{key: key, message: issue.Message, fix: issue.Fix, last: issue.Updated, count: 1})
	}
	return groups
}

// recentEventsDiagnostic writes one group as a doctor row: what happened and
// where, how often and when last, that it does not change readiness, and its
// fix: the command the newest raise carried, or else the show command that
// lists each event.
func recentEventsDiagnostic(group *recentEventsGroup) diagnostic.Diagnostic {
	code := group.key.code
	described := diagnostic.Lookup(code) != nil
	if !described {
		code = diagnostic.CodeDoctorRecentEvents
	}

	var tb textbuf.Buffer
	tb.Str(group.key.source)
	fix := recentEventsFixWarnings
	if group.key.severity == report.SeverityError {
		tb.Str(" error event")
		fix = recentEventsFixErrors
	} else {
		tb.Str(" warning")
	}
	if !described {
		tb.Byte(' ').Str(group.key.code)
	}
	tb.Str(" on ").Str(group.key.subject).Str(": ").Int(int64(group.count))
	if group.count == 1 {
		tb.Str(" time, last at ")
	} else {
		tb.Str(" times, last at ")
	}
	tb.Str(group.last.UTC().Format(recentEventsTimeLayout)).Str(": ").Str(group.message)
	tb.Str(". It may have recovered, so the box still reads ready; check that it no longer happens.")

	// The producer knows the command that fixes its subject (report
	// RaiseErrorFix, RaiseWarningFix); the show command only lists the event.
	if group.fix != "" {
		fix = group.fix
	}

	return diagnostic.Diagnostic{
		Code:     code,
		Severity: diagnostic.SeverityWarning,
		Message:  tb.String(),
		Fix:      fix,
	}
}
