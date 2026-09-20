// Design: docs/architecture/firewall/backend-command-dispatch.md -- nft firewall health check

package firewallnft

import (
	"context"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
)

// RegisterHealthCheck registers the firewall health check with the default registry.
func RegisterHealthCheck() {
	health.Register("firewall", checkFirewallHealth)
}

// auditTables runs one audit and sends its error to audited.
//
// The caller MUST give it a buffered channel: the caller stops waiting after
// one second, and an unbuffered send would then block this goroutine for as
// long as the kernel call takes to return.
func auditTables(audited chan<- error) {
	_, err := firewall.AuditTables()
	audited <- err
}

// checkFirewallHealth answers for the ze_* tables ze applied.
//
// It has three outcomes, not two. The audit can find drift, it can find none,
// and it can fail to read the kernel at all. The third one used to be reported
// as the second: AuditTables returned 0 findings with no way to say why, so a
// ruleset an external tool had flushed read as healthy.
func checkFirewallHealth() (health.Status, string) {
	audited := make(chan error, 1)
	go auditTables(audited)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	select {
	case err := <-audited:
		if err != nil {
			return health.StatusDegraded, "firewall audit could not run: " + err.Error()
		}
	case <-ctx.Done():
		return health.StatusDegraded, "firewall audit timed out"
	}
	for _, w := range report.Warnings() {
		if slices.Contains([]string{"firewall-stale-table", "firewall-drift"}, w.Code) {
			return health.StatusDegraded, w.Message
		}
	}
	return health.StatusHealthy, ""
}
