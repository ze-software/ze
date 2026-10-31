// VPP health check: the row exists only while VPP is the interface backend,
// and it reads the socket the running vpp component dials.
package ifacevpp

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/health"
)

// withActiveVPPSocket replaces the running vpp component's socket answer for
// one test and restores it after.
func withActiveVPPSocket(t *testing.T, path string, running bool) {
	t.Helper()
	saved := activeVPPSocket
	activeVPPSocket = func() (string, bool) { return path, running }
	t.Cleanup(func() { activeVPPSocket = saved })
}

// shortSocketDir returns a directory whose paths fit a unix socket address:
// t.TempDir on macOS exceeds the 104-byte sun_path limit.
func shortSocketDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "vpph")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// TestVPPHealthCheckComponentNotRunning proves that VPP as the backend with no
// running vpp component reads down: no connector exists, so no interface can
// be programmed. Method: the seam answers not running.
func TestVPPHealthCheckComponentNotRunning(t *testing.T) {
	withActiveVPPSocket(t, "", false)

	status, reason := checkVPPHealth()
	assert.Equal(t, health.StatusDown, status)
	assert.Contains(t, reason, "vpp component is not running")
	assert.Contains(t, reason, "ze doctor")
}

// TestVPPHealthCheckSocketRefuses proves that a socket file nothing listens on
// reads down with the dial error. Method: create a unix socket, close the
// listener so the file stays and connections are refused.
func TestVPPHealthCheckSocketRefuses(t *testing.T) {
	path := filepath.Join(shortSocketDir(t), "api.sock")
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", path)
	require.NoError(t, err)
	unixListener, ok := ln.(*net.UnixListener)
	require.True(t, ok, "Listen(\"unix\") answered %T", ln)
	unixListener.SetUnlinkOnClose(false)
	require.NoError(t, ln.Close())
	withActiveVPPSocket(t, path, true)

	status, reason := checkVPPHealth()
	assert.Equal(t, health.StatusDown, status)
	assert.Contains(t, reason, path)
	assert.Contains(t, reason, "ze doctor")
}

// TestVPPHealthCheckSocketAccepts proves the healthy answer is measured: it
// comes only from a dial to the configured socket that succeeded. Method: a
// live unix listener at the configured path.
func TestVPPHealthCheckSocketAccepts(t *testing.T) {
	path := filepath.Join(shortSocketDir(t), "api.sock")
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	withActiveVPPSocket(t, path, true)

	status, reason := checkVPPHealth()
	assert.Equal(t, health.StatusHealthy, status, reason)
}

// TestVPPHealthAppliesOnlyToTheVPPBackend proves the row is gated on the
// loaded interface backend, so a host that does not run VPP carries no vpp
// row rather than a healthy one nobody measured. Method: read the gate with
// no backend loaded, then with the vpp backend loaded.
func TestVPPHealthAppliesOnlyToTheVPPBackend(t *testing.T) {
	require.Empty(t, iface.ActiveBackendName(), "the test binary starts with no backend")
	assert.False(t, vppHealthApplies(), "no backend loaded: the check does not apply")

	require.NoError(t, iface.LoadBackend(backendVPP))
	t.Cleanup(func() { _ = iface.CloseBackend() })
	assert.True(t, vppHealthApplies(), "vpp loaded: the check applies")
}
