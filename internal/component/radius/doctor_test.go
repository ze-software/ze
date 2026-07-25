package radius

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func withRadiusProbe(t *testing.T, reachable bool) {
	t.Helper()
	prev := radiusAdminProbe
	radiusAdminProbe = func(Server, net.IP, time.Duration) bool { return reachable }
	t.Cleanup(func() { radiusAdminProbe = prev })
}

// VALIDATES: the admin doctor check is a no-op when RADIUS is not configured.
// PREVENTS: a spurious warning on boxes that never use RADIUS admin auth.
func TestRadiusAdminDoctorNoConfig(t *testing.T) {
	withRadiusProbe(t, false)
	diags := checkRadiusAdminServers(diagnostic.DoctorCheckContext{Tree: config.NewTree()})
	assert.Empty(t, diags)
}

// VALIDATES: R-4 an unreachable RADIUS admin server emits the warning code.
// PREVENTS: a silent lockout risk going unnoticed before daemon start.
func TestRadiusAdminDoctorUnreachable(t *testing.T) {
	withRadiusProbe(t, false)
	inner := config.NewTree()
	srv := config.NewTree()
	srv.Set("key", "secret")
	inner.AddListEntry("server", "10.0.0.1", srv)

	diags := checkRadiusAdminServers(diagnostic.DoctorCheckContext{Tree: radiusTree(inner)})
	require.Len(t, diags, 1)
	assert.Equal(t, "doctor-radius-admin-unreachable", diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
}

// VALIDATES: a reachable server produces no diagnostic.
// PREVENTS: false positives when at least one server answers.
func TestRadiusAdminDoctorReachable(t *testing.T) {
	withRadiusProbe(t, true)
	inner := config.NewTree()
	srv := config.NewTree()
	srv.Set("key", "secret")
	inner.AddListEntry("server", "10.0.0.1", srv)

	diags := checkRadiusAdminServers(diagnostic.DoctorCheckContext{Tree: radiusTree(inner)})
	assert.Empty(t, diags)
}

// VALIDATES: the check registered its diagnostic code in the global registry.
// PREVENTS: an unregistered code that `ze explain` cannot describe.
func TestRadiusAdminDoctorRegistered(t *testing.T) {
	assert.Contains(t, diagnostic.DoctorCheckNames(), "radius-admin-unreachable")
}
