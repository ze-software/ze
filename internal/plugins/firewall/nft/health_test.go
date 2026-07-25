package firewallnft

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/report"
)

func TestFirewallHealthCheckHealthy(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	status, _ := checkFirewallHealth()
	assert.Equal(t, health.StatusHealthy, status, "healthy when no warnings")
}

func TestFirewallHealthCheckWarningCodes(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	report.RaiseWarning("firewall", "firewall-stale-table", "ze_filter", "stale", nil)
	status, reason := checkFirewallHealth()
	assert.Equal(t, health.StatusDegraded, status)
	assert.Contains(t, reason, "stale")
}

func TestFirewallHealthCheckDriftWarning(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	report.RaiseWarning("firewall", "firewall-drift", "ze_filter", "drift detected", nil)
	status, reason := checkFirewallHealth()
	assert.Equal(t, health.StatusDegraded, status)
	assert.Contains(t, reason, "drift")
}
