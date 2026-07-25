package managed

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// restoreHubProbe captures and restores the hub probe seam so an override does
// not leak into other tests. Tests using it MUST NOT call t.Parallel().
func restoreHubProbe(t *testing.T) {
	t.Helper()
	orig := hubProbe
	t.Cleanup(func() { hubProbe = orig })
}

// TestHubDoctorCheckUnreachable verifies the managed-hub doctor diagnostic:
// no client block is a no-op, an unreachable primary hub warns
// doctor-hub-unreachable, a reachable primary clears the warning, and -- because
// the daemon connects only to the FIRST client block -- a reachable secondary
// does NOT mask an unreachable primary.
//
// VALIDATES: startup-resilience AC-5 - an unreachable management hub is surfaced
// via doctor, not left silent (the only touchpoint that was log-only).
// PREVENTS: the check firing when no hub client is configured; failing to warn
// when the daemon's hub (Clients[0]) is down; or a reachable spare hub producing
// a false-healthy result (the daemon ignores Clients[1:]).
func TestHubDoctorCheckUnreachable(t *testing.T) {
	restoreHubProbe(t)

	clients := []plugin.HubClientConfig{
		{Name: "hub-a", Host: "203.0.113.9", Port: 8443},
		{Name: "hub-b", Host: "203.0.113.10", Port: 8443},
	}

	t.Run("no client configured is a no-op", func(t *testing.T) {
		hubProbe = func(string, string) bool {
			t.Fatal("probe must not run when no hub client is configured")
			return false
		}
		assert.Nil(t, diagnoseHubReachability(nil))
	})

	t.Run("primary hub unreachable warns", func(t *testing.T) {
		var probed []string
		hubProbe = func(addr, _ string) bool {
			probed = append(probed, addr)
			return false
		}
		diags := diagnoseHubReachability(clients)
		require.Len(t, diags, 1)
		assert.Equal(t, "doctor-hub-unreachable", diags[0].Code)
		assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
		assert.Equal(t, []string{"203.0.113.9:8443"}, probed,
			"only the daemon-selected (first) hub should be probed")
	})

	t.Run("primary hub reachable is clean", func(t *testing.T) {
		hubProbe = func(addr, _ string) bool { return addr == "203.0.113.9:8443" }
		assert.Nil(t, diagnoseHubReachability(clients))
	})

	t.Run("reachable secondary does not mask a down primary", func(t *testing.T) {
		// The daemon connects to Clients[0] (203.0.113.9). A reachable spare
		// (203.0.113.10) must NOT clear the warning: the daemon's hub is down.
		hubProbe = func(addr, _ string) bool { return addr == "203.0.113.10:8443" }
		diags := diagnoseHubReachability(clients)
		require.Len(t, diags, 1)
		assert.Equal(t, "doctor-hub-unreachable", diags[0].Code)
	})
}

// TestCheckHubReachableNonTreeContext verifies the check is defensive against a
// missing or wrongly-typed config tree (returns no diagnostics rather than
// panicking).
//
// VALIDATES: startup-resilience - the doctor check never turns a missing tree
// into a false unreachable warning.
func TestCheckHubReachableNonTreeContext(t *testing.T) {
	assert.Nil(t, checkHubReachable(diagnostic.DoctorCheckContext{Tree: nil}))
	assert.Nil(t, checkHubReachable(diagnostic.DoctorCheckContext{Tree: "not a tree"}))
}

// TestHubReachableProbe exercises the real (non-seam) TCP probe: a live listener
// reads as reachable, a closed port reads as unreachable, both promptly.
//
// VALIDATES: startup-resilience AC-5 - hubReachable is a bounded TCP probe.
// PREVENTS: a probe that reports reachable on a refused connection.
func TestHubReachableProbe(t *testing.T) {
	origTimeout := hubProbeTimeout
	t.Cleanup(func() { hubProbeTimeout = origTimeout })
	hubProbeTimeout = 2 * time.Second

	var lc net.ListenConfig
	ln, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	reachableAddr := ln.Addr().String()
	assert.True(t, hubReachable(reachableAddr, ""), "live listener should be reachable")

	// Close the listener; the same address now refuses connections promptly.
	require.NoError(t, ln.Close())
	assert.False(t, hubReachable(reachableAddr, ""), "closed port should be unreachable")
}
