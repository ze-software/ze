// Design: docs/architecture/api/architecture.md -- gNMI observability
// Related: server.go -- gNMI server lifecycle

package gnmi

import (
	"github.com/ze-software/ze/internal/core/metrics"
)

type gnmiMetrics struct {
	requestsTotal   metrics.CounterVec
	errorsTotal     metrics.CounterVec
	subscribeActive metrics.Gauge
}

func initGNMIMetrics(reg metrics.Registry) *gnmiMetrics {
	return &gnmiMetrics{
		requestsTotal:   reg.CounterVec("ze_gnmi_requests_total", "Total gNMI RPC requests.", []string{"rpc"}),
		errorsTotal:     reg.CounterVec("ze_gnmi_errors_total", "Total gNMI RPC errors.", []string{"rpc", "code"}),
		subscribeActive: reg.Gauge("ze_gnmi_subscribe_active", "Active gNMI Subscribe streams."),
	}
}
