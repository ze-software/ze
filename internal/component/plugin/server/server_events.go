// Design: docs/architecture/doctor-and-health-checks.md -- event codes
// Related: register.go -- serverEventCodes, which declares the codes these events carry
//
// The plugin server's own error events: the faults whose only record was a
// log line. A payload only a defect produces, a plugin request the server
// refused, an answer that never reached its caller, a drain that did not
// finish and a configuration reload the server refused each log under their
// doctor code, and each raises that code here too, because a log line never
// reaches `ze doctor`: the recent-events check reads the report bus.

package server

import (
	"errors"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// serverEventSource is the report-bus source of the plugin server's events.
const serverEventSource = "plugin-server"

// fixShowErrors is the fix of an event that no command clears: the operator
// reads the event, with the log lines around it, to report or correct it.
const fixShowErrors = "ze show errors"

// configTxSubject names the config transaction in an event that no plugin owns.
const configTxSubject = "config-transaction"

// raiseServerEvent raises code as an error event about subject, with
// fixShowErrors as its fix: no server event has a command that clears it. The
// caller logs the same code on the line beside the call. The bus refuses an
// empty field, so a caller that holds no plugin or command name passes the
// area it works in.
func raiseServerEvent(code, subject, message string) {
	report.RaiseErrorFix(serverEventSource, code, subject, report.FitMessage(message), fixShowErrors)
}

// configReloadSubject names a configuration reload in an event that no one
// plugin owns: a refused reload can name several plugins, or none.
const configReloadSubject = "config-reload"

// raiseReloadRefused raises doctor-config-reload-refused about the reload,
// with the fix reloadRefusalFix finds on refusal. message says what refused
// the change, what ze kept and why, from refusal. The caller logs the same
// code on the line beside the call.
func raiseReloadRefused(message string, refusal error) {
	report.RaiseErrorFix(serverEventSource, diagnostic.CodeDoctorConfigReloadRefused, configReloadSubject,
		report.FitMessage(message), reloadRefusalFix(refusal))
}

// reloadRefusalFix answers the fix of the operator error refusal's chain
// carries, the producer's own command for its own refusal (a plugin
// dependency this build does not hold names the build that does), and
// fixShowErrors when the chain carries none. A plugin's verify or apply
// refusal arrives as text over the plugin connection, so it carries none.
//
// `ze config validate` is not the fallback: it takes a file path, reads the
// file system and never the store a daemon may load from, and runs no live
// plugin's verify callback, which is what refuses a reload
// (ai/doors/config-yang.md, "Traps").
func reloadRefusalFix(refusal error) string {
	if operatorError, ok := errors.AsType[*opererr.Error](refusal); ok {
		return operatorError.Fix()
	}
	return fixShowErrors
}

// reloadRefusalMessage writes the event of a refused reload: what refused it
// and what ze did (problem), the plugins concerned when there are any, then
// the refusal itself (why).
func reloadRefusalMessage(problem string, plugins []string, refusal error) string {
	var tb textbuf.Buffer
	tb.Str("config reload refused: ").Str(problem)
	for i, name := range plugins {
		if i == 0 {
			tb.Str(" (plugins ")
		} else {
			tb.Str(", ")
		}
		tb.Str(name)
	}
	if len(plugins) > 0 {
		tb.Byte(')')
	}
	tb.Str(": ").Err(refusal)
	return tb.String()
}

// drainSubject names a drain round in an event that no one plugin owns.
const drainSubject = "drain"

// streamingHandlerSubject names the streaming handler registry in an event
// whose registration carried no prefix to name it by.
const streamingHandlerSubject = "streaming-handler"
