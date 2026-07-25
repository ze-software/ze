// Design: docs/architecture/core-design.md -- report-bus health registration
// Related: health_probe.go -- warning-code health probes for other owners

package report

import "github.com/ze-software/ze/internal/core/health"

func init() {
	// The bus itself has no failure mode beyond being linked in; a healthy
	// row proves `show health` consults it.
	health.Register("report-bus", func() (health.Status, string) {
		return health.StatusHealthy, ""
	})
}
