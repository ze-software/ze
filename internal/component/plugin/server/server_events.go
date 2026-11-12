// Design: docs/architecture/doctor-and-health-checks.md -- event codes
// Related: register.go -- serverEventCodes, which declares the codes these events carry
//
// The plugin server's own error events: the faults whose only record was a
// log line. A payload only a defect produces, a plugin request the server
// refused, an answer that never reached its caller and a drain that did not
// finish each log under their doctor code, and each raises that code here
// too, because a log line never reaches `ze doctor`: the recent-events check
// reads the report bus.

package server

import "github.com/ze-software/ze/internal/core/report"

// serverEventSource is the report-bus source of the plugin server's events.
const serverEventSource = "plugin-server"

// fixShowErrors is the fix of an event that no command clears: the operator
// reads the event, with the log lines around it, to report or correct it.
const fixShowErrors = "ze show errors"

// configTxSubject names the config transaction in an event that no plugin owns.
const configTxSubject = "config-transaction"

// raiseServerEvent raises code as an error event about subject. The caller
// logs the same code on the line beside the call. The bus refuses an empty
// field, so a caller that holds no plugin or command name passes the area it
// works in.
func raiseServerEvent(code, subject, message, fix string) {
	report.RaiseErrorFix(serverEventSource, code, subject, message, fix)
}

// drainSubject names a drain round in an event that no one plugin owns.
const drainSubject = "drain"

// streamingHandlerSubject names the streaming handler registry in an event
// whose registration carried no prefix to name it by.
const streamingHandlerSubject = "streaming-handler"
