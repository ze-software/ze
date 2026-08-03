// Design: docs/architecture/core-design.md -- l2tp health surface
// Related: service_locator.go -- LookupService consulted by the check
// Related: register.go -- health.Register wiring

package l2tp

import "github.com/ze-software/ze/internal/core/health"

// checkHealth reports degraded while the l2tp subsystem is not running.
// Registered from register.go so deleting this component removes its
// health row (ai/rules/plugins.md).
func checkHealth() (health.Status, string) {
	if LookupService() == nil {
		return health.StatusDegraded, "subsystem not running"
	}
	return health.StatusHealthy, ""
}
