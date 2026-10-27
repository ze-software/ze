// Design: docs/architecture/provisioning/dhcp-server.md -- panic containment in the request task
// Related: register.go -- serveMulti and serveRequests, the request task's boundary

package dhcpserver

import (
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// taskRequest is the task label of a listener's request task in
// ze_task_panics_total{component="dhcp"}.
const taskRequest = "request"

// requestRestart is what follows a contained request panic, in the words the
// guard's log line gives the operator.
const requestRestart = "the listener serves the next request; the request being handled gets " +
	"no reply, and the client retransmits it"

// dhcpGuard is the panic boundary of every DHCP listener. A contained panic is
// logged, counted in ze_task_panics_total{component="dhcp",task="request"} and
// crash-recorded.
var dhcpGuard = taskguard.New("dhcp", slogutil.Logger("dhcpserver"))

// requestTask is the request task, declared once so its count exists before
// the first panic.
var requestTask = dhcpGuard.Task(taskRequest, requestRestart)
