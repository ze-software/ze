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

func checkFirewallHealth() (health.Status, string) {
	done := make(chan struct{})
	go func() {
		firewall.AuditTables()
		close(done)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	select {
	case <-done:
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
