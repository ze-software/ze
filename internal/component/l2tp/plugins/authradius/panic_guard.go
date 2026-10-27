// Design: docs/guide/l2tp.md -- panic containment in the CoA listener, the
// subscriber login and the per-session accounting task
// Related: coa.go -- coaListener.serveRequests, the CoA request task's boundary
// Related: handler.go -- radiusAuth.doRADIUS, the login request task's boundary
// Related: acct.go -- radiusAcct.runAccounting, the accounting task's boundary
// Related: register.go -- ConfigureMetrics, which publishes the guards' counters

package l2tpauthradius

import (
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Task labels of the subscriber RADIUS tasks in
// ze_task_panics_total{component="radius-auth"}.
const (
	taskAuthRequest = "request"
	taskAccounting  = "accounting"
)

// authRequestRestart is what follows a contained login panic, in the words the
// guard's log line gives the operator.
const authRequestRestart = "the session's login is rejected as an internal error; " +
	"every other login is served as before"

// accountingRestart is what follows a contained accounting panic. The session
// stays up and keeps its accounting entry, so its Accounting-Stop is still sent
// when the session ends.
const accountingRestart = "this session sends no further Start or Interim-Update record; " +
	"its Accounting-Stop is still sent when it ends, and every other session accounts as before"

// authGuard is the panic boundary of the subscriber login and accounting
// tasks. A contained panic is logged, counted in
// ze_task_panics_total{component="radius-auth"} and crash-recorded.
var authGuard = taskguard.New("radius-auth", slogutil.Logger("l2tp.auth.radius"))

// authRequestTask and accountingTask are declared once so their counts exist
// before the first panic.
var (
	authRequestTask = authGuard.Task(taskAuthRequest, authRequestRestart)
	accountingTask  = authGuard.Task(taskAccounting, accountingRestart)
)

// sessionUnit names one L2TP session for a guard's log line and crash record.
func sessionUnit(tunnelID, sessionID uint16) taskguard.Context {
	var b textbuf.Buffer
	return taskguard.Context{
		Unit: b.Reset().Str("tunnel ").Int(int64(tunnelID)).Str(" session ").Int(int64(sessionID)).String(),
	}
}

// taskCoARequest is the task label of the CoA listener's request task in
// ze_task_panics_total{component="radius-coa"}.
const taskCoARequest = "request"

// coaRequestRestart is what follows a contained request panic, in the words the
// guard's log line gives the operator.
const coaRequestRestart = "the listener serves the next request; an authenticated request is " +
	"answered with a CoA-NAK or Disconnect-NAK carrying Error-Cause 506 (Resources Unavailable), " +
	"any other is dropped"

// coaGuard is the panic boundary of the CoA and Disconnect listener. A
// contained panic is logged, counted in
// ze_task_panics_total{component="radius-coa",task="request"} and
// crash-recorded.
var coaGuard = taskguard.New("radius-coa", slogutil.Logger("l2tp.auth.radius.coa"))

// coaRequestTask is the request task, declared once so its count exists before
// the first panic.
var coaRequestTask = coaGuard.Task(taskCoARequest, coaRequestRestart)

// coaCodeName names a request Code for the guard's log line. Only the two
// request codes are admitted (admitRequest), so the fallback names a Code a
// defect let through, never a peer's choice.
func coaCodeName(code uint8) string {
	switch code {
	case radius.CodeCoARequest:
		return "CoA-Request"
	case radius.CodeDisconnectRequest:
		return "Disconnect-Request"
	}
	return "unadmitted code"
}
