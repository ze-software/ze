// Design: docs/architecture/core-design.md -- l2tp health surface
// Related: service_locator.go -- LookupService consulted by the check
// Related: subsystem.go -- kernelDataPlane, the state the check reads
// Related: register.go -- health.Register wiring

package l2tp

import "github.com/ze-software/ze/internal/core/health"

// kernelDataPlaneReporter is the published service's answer to whether its
// sessions are programmed into the kernel. *Subsystem implements it.
type kernelDataPlaneReporter interface {
	kernelDataPlane() error
}

var _ kernelDataPlaneReporter = (*Subsystem)(nil)

// checkHealth reports degraded while the l2tp subsystem is not running, and
// while it runs with no kernel data plane: sessions then establish on the
// control plane and carry no traffic. Registered from register.go so deleting
// this component removes its health row (ai/rules/plugins.md).
func checkHealth() (health.Status, string) {
	svc := LookupService()
	if svc == nil {
		return health.StatusDegraded, "subsystem not running"
	}
	reporter, ok := svc.(kernelDataPlaneReporter)
	if !ok {
		return health.StatusDegraded, "the running L2TP service does not report its kernel data plane"
	}
	return dataPlaneHealth(reporter.kernelDataPlane())
}

// dataPlaneHealth turns the subsystem's kernel data-plane fault into the
// health row: healthy only when every listener has a kernel worker.
func dataPlaneHealth(fault error) (health.Status, string) {
	if fault != nil {
		return health.StatusDegraded, "kernel data plane not programming sessions: " + fault.Error() +
			"; run `ze doctor` to check the CONFIG_L2TP and CONFIG_PPPOL2TP kernel support"
	}
	return health.StatusHealthy, ""
}
