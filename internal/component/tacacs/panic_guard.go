// Design: docs/architecture/aaa-tacacs.md -- panic containment at the request entry
// Related: client.go -- Authenticate, SendAuthorization and SendAccounting, the guarded entries
// Related: register.go -- init hands bindTacacsMetrics to the plugin registry

package tacacs

import (
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// taskRequest is the task label of one TACACS+ request in
// ze_task_panics_total{component="tacacs"}.
const taskRequest = "request"

// metricsHookName names this component's hook in the plugin registry's
// metrics injection.
const metricsHookName = "tacacs-client"

// requestRestart is what follows a contained panic, in the words the guard's
// log line gives the operator.
const requestRestart = "the request fails as if no TACACS+ server could be reached, so the AAA " +
	"chain moves to its next backend; the connection it used is closed"

// requestGuard is the panic boundary of every TACACS+ request. The client has
// no goroutine of its own: each request runs on its caller (an SSH login, the
// command authorizer, the accounting worker), so the boundary is the request
// entry. A contained panic is logged, counted in
// ze_task_panics_total{component="tacacs",task="request"} and crash-recorded.
var requestGuard = taskguard.New("tacacs", slogutil.Logger("tacacs"))

// requestTask is declared once so its count exists before the first panic.
var requestTask = requestGuard.Task(taskRequest, requestRestart)

// containRequest answers the describe and end functions for one request's
// deferred requestTask.Contain. end sets *err to the *taskguard.Panic, which
// every caller treats as an unreachable server.
//
// RFC 8907 Section 5.4.3: "If the status equals TAC_PLUS_AUTHEN_STATUS_ERROR,
// then the host is indicating that it is experiencing an unrecoverable error
// and the authentication will proceed as if that host could not be
// contacted."
func containRequest(purpose string, err *error) (func() taskguard.Context, func(*taskguard.Panic)) {
	describe := func() taskguard.Context { return taskguard.Context{Unit: purpose + " request"} }
	end := func(p *taskguard.Panic) { *err = p }
	return describe, end
}

// bindTacacsMetrics publishes the request task's ze_task_panics_total series
// into reg. It is the hook init hands to the plugin registry.
func bindTacacsMetrics(reg metrics.Registry) {
	if reg == nil {
		return
	}
	requestGuard.SetMetricsRegistry(reg)
}
