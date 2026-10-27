// Design: docs/architecture/bfd.md -- panic containment in the receive and express-loop tasks
// Related: udp.go -- UDP.receive, the receive task's boundary

package transport

import (
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// taskReceive is the task label of the socket reader in
// ze_task_panics_total{component="bfd"}.
const taskReceive = "receive"

// receiveRestart is what brings the receive task back after a contained panic,
// in the words the guard's log line gives the operator.
const receiveRestart = "the socket reader resumes with the next packet; " +
	"the packet it was reading is dropped and no session changes state"

// receiveGuard is the panic boundary of every BFD socket reader. A contained
// panic is logged, counted in ze_task_panics_total{component="bfd",task="receive"}
// and crash-recorded. A test may replace it before Start and restore it after
// Stop: UDP.receive reads it each time the task is entered.
var receiveGuard = taskguard.New("bfd", slogutil.Logger("bfd.transport"))

// SetPanicMetrics publishes the receive task's panic counter on reg. The BFD
// component calls it from its metrics hook (bindMetricsRegistry).
func SetPanicMetrics(reg metrics.Registry) { receiveGuard.SetMetricsRegistry(reg) }
