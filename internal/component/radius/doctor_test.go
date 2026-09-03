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

// TestRadiusAdminDoctorProbeStaysPap drives the real probe, not the seam, so it
// reads what radiusAdminReachable actually sends and what it makes of the
// answer.
//
// VALIDATES: AC-7 -- with `auth-method chap` configured, the doctor probe still
// carries the fixed PAP credential, and a server that answers Access-Reject
// still reads as reachable. The probe tests reachability and the shared secret,
// which a rejection answers as well as an acceptance does.
// PREVENTS: the probe following the configured method onto a CHAP-only path, so
// a server that stores hashes would read as unreachable and raise
// doctor-radius-admin-unreachable on a working deployment. And a probe that
// took the verdict for the answer, which would raise the same false warning
// against any server that rejects the fixed ze-doctor credential -- that is
// every correctly configured server.
func TestRadiusAdminDoctorProbeStaysPap(t *testing.T) {
	key := []byte("testing123")

	chapConfig := func(t *testing.T, addr string) ExtractedConfig {
		t.Helper()
		host, port, err := net.SplitHostPort(addr)
		require.NoError(t, err)
		inner := config.NewTree()
		entry := config.NewTree()
		entry.Set("key", string(key))
		entry.Set("port", port)
		inner.AddListEntry("server", host, entry)
		inner.Set("auth-method", "chap")
		cfg, err := ExtractConfig(radiusTree(inner))
		require.NoError(t, err)
		require.Equal(t, AuthMethodCHAP, cfg.AuthMethod)
		return cfg
	}

	// A server that rejects the probe is still reachable: the verdict is not the
	// answer the check reads.
	rejecting := newReplyServer(t, key, CodeAccessReject, nil)
	defer rejecting.close()
	cfg := chapConfig(t, rejecting.addr)
	assert.True(t, radiusAdminReachable(cfg.Servers[0], cfg.SourceAddress, 2*time.Second),
		"a server that answers verifiably is reachable, whatever verdict it returns")

	// The probe's own attributes: PAP, and the fixed ze-doctor credential.
	capturing := newRequestCaptureServer(t, key, nil)
	defer capturing.close()
	cfg = chapConfig(t, capturing.addr)
	assert.True(t, radiusAdminReachable(cfg.Servers[0], cfg.SourceAddress, 2*time.Second))

	req := capturing.captured(t)
	assert.Equal(t, []byte("ze-doctor"), req.FindAttr(AttrUserName))
	assert.NotNil(t, req.FindAttr(AttrUserPassword), "the probe stays PAP")
	assert.Nil(t, req.FindAttr(AttrCHAPPassword))
	assert.Nil(t, req.FindAttr(AttrCHAPChallenge))
}
