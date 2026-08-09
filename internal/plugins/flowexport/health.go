// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- flow export health check
// Related: register.go -- RegisterHealthCheck called from init()
// Related: exporter.go -- status() supplies per-collector send-error counts

package flowexport

import "github.com/ze-software/ze/internal/core/health"

// RegisterHealthCheck registers the flow-export health check with the default
// registry. Flow export is outbound, connectionless UDP, so true collector
// reachability is not observable (there is no handshake or reply). The health
// check therefore reports the next-best signal: whether the exporter is running
// and whether any collector's sends are failing (e.g. no route to the
// collector, which surfaces as UDP send errors).
func RegisterHealthCheck() {
	health.Register("flow-export", checkFlowExportHealth)
}

func checkFlowExportHealth() (health.Status, string) {
	exp := getExporter()
	if exp == nil {
		return health.StatusHealthy, "not configured"
	}

	collectors := exp.status()
	if len(collectors) == 0 {
		return health.StatusHealthy, "no collectors"
	}

	failing := 0
	for _, c := range collectors {
		if errs, ok := c["errors"].(uint64); ok && errs > 0 {
			failing++
		}
	}

	switch {
	case failing == len(collectors):
		return health.StatusDown, "all collectors have send errors"
	case failing > 0:
		return health.StatusDegraded, "some collectors have send errors"
	default:
		return health.StatusHealthy, ""
	}
}
