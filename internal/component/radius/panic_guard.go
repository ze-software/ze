// Design: docs/guide/radius.md -- panic containment in the client's read task
// Related: client.go -- Client.readLoop and readAnswers, the read task's boundary
// Related: metrics.go -- bindRadiusMetrics, which publishes the guard's counter

package radius

import (
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// taskRead is the task label of a client's answer reader in
// ze_task_panics_total{component="radius"}.
const taskRead = "read"

// readRestart is what follows a contained read panic, in the words the guard's
// log line gives the operator.
const readRestart = "the client reads on with the next answer; the answer being dispatched is " +
	"discarded, so its request retransmits and fails over to the next server as on a timeout"

// readGuard is the panic boundary of every RADIUS client's answer reader, the
// admin login backend's and the subscriber plugin's alike. A contained panic is
// logged, counted in ze_task_panics_total{component="radius",task="read"} and
// crash-recorded.
var readGuard = taskguard.New("radius", slogutil.Logger("radius"))

// readTask is the read task, declared once so its count exists before the
// first panic.
var readTask = readGuard.Task(taskRead, readRestart)
