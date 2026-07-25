package report

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/core/health"
)

// VALIDATES: warning-code probes report degraded with the warning message
// (moved from cmd/show health_checks_test.go when health rows moved to
// their owning components).
// PREVENTS: a component health row staying healthy while its warning codes
// are active on the report bus.
func TestHealthProbeDegraded(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	probe := HealthProbeDegraded("session-stuck", "session-flap", "eor-timeout")
	status, _ := probe()
	assert.Equal(t, health.StatusHealthy, status, "healthy when no warnings")

	RaiseWarning("bgp", "session-stuck", "192.0.2.1", "stuck", nil)
	status, reason := probe()
	assert.Equal(t, health.StatusDegraded, status)
	assert.Contains(t, reason, "stuck")
}

// VALIDATES: down-severity probes produce StatusDown, not just degraded.
// PREVENTS: dead plugins showing as merely degraded in show health.
func TestHealthProbeDown(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	probe := HealthProbeDown("plugin-down")
	status, _ := probe()
	assert.Equal(t, health.StatusHealthy, status)

	RaiseWarning("plugin", "plugin-down", "my-plugin", "disabled", nil)
	status, _ = probe()
	assert.Equal(t, health.StatusDown, status)
}

// VALIDATES: a registry mixing degraded and down probes aggregates to the
// worst status (the /health endpoint 503 path).
// PREVENTS: HTTP 200 when a critical component is down.
func TestHealthProbesInRegistry(t *testing.T) {
	ResetForTest()
	defer ResetForTest()

	reg := &health.Registry{}
	reg.Register("bgp", HealthProbeDegraded("session-stuck"))
	reg.Register("fib", HealthProbeDegraded("fib-sync-failure"))
	reg.Register("plugins", HealthProbeDown("plugin-down"))

	rpt := reg.Check()
	assert.Equal(t, health.StatusHealthy, rpt.Status)
	assert.Len(t, rpt.Components, 3)

	RaiseWarning("plugin", "plugin-down", "test", "crashed", nil)
	rpt = reg.Check()
	assert.Equal(t, health.StatusDown, rpt.Status)
}
