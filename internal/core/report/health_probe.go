// Design: docs/architecture/core-design.md -- report-bus health probes
// Related: report.go -- warning/error bus the probes consult
// Related: ../health/registry.go -- health.Register consumers

package report

import (
	"slices"

	"github.com/ze-software/ze/internal/core/health"
)

// HealthProbeDegraded returns a health check reporting StatusDegraded with
// the matching warning's message when any active warning carries one of the
// given codes. Owners register it with health.Register so the component that
// raises a warning code also owns its health surface
// (ai/rules/plugins.md).
func HealthProbeDegraded(codes ...string) func() (health.Status, string) {
	return probe(health.StatusDegraded, codes)
}

// HealthProbeDown is HealthProbeDegraded with StatusDown severity, for
// warning codes that mean the component is not functioning at all.
func HealthProbeDown(codes ...string) func() (health.Status, string) {
	return probe(health.StatusDown, codes)
}

func probe(severity health.Status, codes []string) func() (health.Status, string) {
	return func() (health.Status, string) {
		for _, w := range Warnings() {
			if slices.Contains(codes, w.Code) {
				return severity, w.Message
			}
		}
		return health.StatusHealthy, ""
	}
}
