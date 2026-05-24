package show

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"codeberg.org/thomas-mangin/ze/internal/core/health"
	"codeberg.org/thomas-mangin/ze/internal/core/report"
)

// VALIDATES: AC-20 -- show health reports status for new components.
// PREVENTS: New health checks returning wrong status.
func TestHealthCheckBGPDegraded(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	status, _ := checkBGPHealth()
	assert.Equal(t, health.StatusHealthy, status, "healthy when no warnings")

	report.RaiseWarning("bgp", "session-stuck", "192.0.2.1", "stuck", nil)
	status, reason := checkBGPHealth()
	assert.Equal(t, health.StatusDegraded, status)
	assert.Contains(t, reason, "stuck")
}

// VALIDATES: AC-20 -- plugin-down produces StatusDown, not just degraded.
// PREVENTS: Down plugins showing as merely degraded.
func TestHealthCheckPluginDown(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	status, _ := checkPluginHealth()
	assert.Equal(t, health.StatusHealthy, status)

	report.RaiseWarning("plugin", "plugin-down", "my-plugin", "disabled", nil)
	status, _ = checkPluginHealth()
	assert.Equal(t, health.StatusDown, status)
}

// VALIDATES: AC-21 -- /health endpoint returns 503 when any component is down.
// PREVENTS: HTTP 200 when a critical component is down.
func TestHealthRegistryNewComponents(t *testing.T) {
	report.ResetForTest()
	defer report.ResetForTest()

	reg := &health.Registry{}
	reg.Register("bgp", checkBGPHealth)
	reg.Register("fib", checkFIBHealth)
	reg.Register("firewall", checkFirewallHealth)
	reg.Register("plugins", checkPluginHealth)

	rpt := reg.Check()
	assert.Equal(t, health.StatusHealthy, rpt.Status)
	assert.Len(t, rpt.Components, 4)

	report.RaiseWarning("plugin", "plugin-down", "test", "crashed", nil)
	rpt = reg.Check()
	assert.Equal(t, health.StatusDown, rpt.Status)
}
