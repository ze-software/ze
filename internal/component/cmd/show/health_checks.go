// Design: plan/spec-doctor-health-checks.md -- AC-20/21 health registry extensions

package show

import (
	"context"
	"net"
	"os"
	"slices"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	"codeberg.org/thomas-mangin/ze/internal/core/health"
	"codeberg.org/thomas-mangin/ze/internal/core/report"
)

const vppSocketPath = "/run/vpp/api.sock"

func checkBGPHealth() (health.Status, string) {
	return checkWarningCodes([]string{"session-stuck", "session-flap", "eor-timeout"})
}

func checkFIBHealth() (health.Status, string) {
	return checkWarningCodes([]string{"fib-sync-failure", "fib-orphan", "fib-programming-lag"})
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
	return checkWarningCodes([]string{"firewall-stale-table", "firewall-drift"})
}

func checkIfaceHealth() (health.Status, string) {
	done := make(chan struct{})
	go func() {
		iface.CheckAllInterfaceErrors()
		close(done)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	select {
	case <-done:
	case <-ctx.Done():
		return health.StatusDegraded, "interface error check timed out"
	}
	return checkWarningCodes([]string{"iface-errors"})
}

func checkPluginHealth() (health.Status, string) {
	for _, w := range report.Warnings() {
		if w.Code == "plugin-down" {
			return health.StatusDown, w.Message
		}
	}
	return health.StatusHealthy, ""
}

func checkVPPHealth() (health.Status, string) {
	if _, err := os.Stat(vppSocketPath); err != nil {
		return health.StatusHealthy, ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", vppSocketPath)
	if err != nil {
		return health.StatusDown, "VPP API socket unreachable"
	}
	if closeErr := conn.Close(); closeErr != nil {
		return health.StatusDegraded, "VPP API socket close error"
	}
	return health.StatusHealthy, ""
}

func checkWarningCodes(codes []string) (health.Status, string) {
	for _, w := range report.Warnings() {
		if slices.Contains(codes, w.Code) {
			return health.StatusDegraded, w.Message
		}
	}
	return health.StatusHealthy, ""
}
