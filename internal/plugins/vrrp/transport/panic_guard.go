// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- panic containment in the receive and advertisement tasks
// Related: backend_linux.go -- linuxInstance.receive, the receive task's boundary

package transport

import (
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
)

// receiveGuard is the panic boundary of every VRRP instance's socket reader. A
// contained panic is logged, counted in
// ze_task_panics_total{component="vrrp",task="receive"} and crash-recorded. A
// test may replace it before opening an instance and restore it after closing
// it: linuxInstance.receive reads it each time the task is entered.
var receiveGuard = taskguard.New("vrrp", slogutil.Logger("vrrp"))

// SetPanicMetrics publishes the receive task's panic counter on reg. The VRRP
// plugin calls it from its ConfigureMetrics hook.
func SetPanicMetrics(reg metrics.Registry) { receiveGuard.SetMetricsRegistry(reg) }
