// Design: docs/guide/l2tp.md -- panic containment in the CoA listener's request task
// Related: coa.go -- coaListener.serveRequests, the request task's boundary
// Related: register.go -- ConfigureMetrics, which publishes the guard's counter

package l2tpauthradius

import (
	"github.com/ze-software/ze/internal/component/radius"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

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
