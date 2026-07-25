// Tests for the bounded CoA source-address resolution (startup-resilience FIX 2):
// serverIPs must never block config apply on a dead DNS resolver.
package l2tpauthradius

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/radius"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errTestResolve = errors.New("test: resolver failure")

// restoreAuthSeams captures and restores the serverIPs test seams so an
// override does not leak into other tests. Tests using it MUST NOT call
// t.Parallel().
func restoreAuthSeams(t *testing.T) {
	t.Helper()
	origLookup := lookupIPAddr
	origTimeout := coaResolveTimeout
	t.Cleanup(func() {
		lookupIPAddr = origLookup
		coaResolveTimeout = origTimeout
	})
}

// TestServerIPsBoundedTimeout verifies that a hostname RADIUS server whose
// resolver never answers does not hang serverIPs: the call returns within the
// bounded coaResolveTimeout and the unresolved server contributes no IPs.
//
// VALIDATES: startup-resilience AC-6 / FIX 2 - the latent apply-path DNS lookup
// is bounded before a coa-port YANG leaf can make it reachable.
// PREVENTS: reverting serverIPs to context.Background(), which would block
// OnConfigApply indefinitely on a dead resolver.
func TestServerIPsBoundedTimeout(t *testing.T) {
	restoreAuthSeams(t)
	coaResolveTimeout = 50 * time.Millisecond
	lookupIPAddr = func(ctx context.Context, _ string) ([]net.IPAddr, error) {
		// Simulate a dead resolver: block until the bounded context expires.
		<-ctx.Done()
		return nil, ctx.Err()
	}

	servers := []radius.Server{{Address: "radius.example.invalid:1812"}}

	start := time.Now()
	ips := serverIPs(servers)
	elapsed := time.Since(start)

	assert.Empty(t, ips, "an unresolved server must not contribute source IPs")
	assert.GreaterOrEqual(t, elapsed, coaResolveTimeout,
		"serverIPs should wait for the bounded resolver context")
	assert.Less(t, elapsed, 2*time.Second,
		"serverIPs must return promptly after the bounded timeout, not hang")
}

// TestServerIPsSharedDeadlineAcrossServers verifies that the resolve budget is
// shared across ALL hostname servers, not applied per-server: several dead-
// resolver hostnames together stay within one coaResolveTimeout, so a multi-
// server CoA config cannot overrun the plugin's 1s ApplyBudget.
//
// VALIDATES: startup-resilience AC-6 - apply never exceeds the declared budget.
// PREVENTS: a per-server timeout (N x coaResolveTimeout) blowing the apply
// deadline once the CoA branch goes live.
func TestServerIPsSharedDeadlineAcrossServers(t *testing.T) {
	restoreAuthSeams(t)
	coaResolveTimeout = 100 * time.Millisecond
	lookupIPAddr = func(ctx context.Context, _ string) ([]net.IPAddr, error) {
		<-ctx.Done() // dead resolver: block until the shared deadline expires
		return nil, ctx.Err()
	}

	servers := []radius.Server{
		{Address: "a.example.invalid:1812"},
		{Address: "b.example.invalid:1812"},
		{Address: "c.example.invalid:1812"},
	}

	start := time.Now()
	ips := serverIPs(servers)
	elapsed := time.Since(start)

	assert.Empty(t, ips)
	assert.Less(t, elapsed, 3*coaResolveTimeout,
		"total resolve time must be the shared deadline, not per-server (3x)")
}

// TestServerIPsSkipsUnresolvedHostname verifies that one unresolvable server
// is skipped while the resolvable servers still populate the allow list.
//
// VALIDATES: startup-resilience FIX 2 - the CoA allow list degrades to the
// resolvable subset (fail-closed) instead of failing apply.
// PREVENTS: a single bad hostname dropping every server from source filtering.
func TestServerIPsSkipsUnresolvedHostname(t *testing.T) {
	restoreAuthSeams(t)
	lookupIPAddr = func(_ context.Context, host string) ([]net.IPAddr, error) {
		if host == "good.example.invalid" {
			return []net.IPAddr{{IP: net.ParseIP("192.0.2.10")}}, nil
		}
		return nil, errTestResolve
	}

	servers := []radius.Server{
		{Address: "bad.example.invalid:1812"},
		{Address: "good.example.invalid:1812"},
	}

	ips := serverIPs(servers)

	require.Len(t, ips, 1, "only the resolvable server should contribute")
	assert.Equal(t, "192.0.2.10", ips[0].String())
}

// TestServerIPsIPLiteralNoResolver verifies that IP-literal server addresses are
// used directly and never invoke the resolver (so they cannot block apply).
//
// VALIDATES: startup-resilience FIX 2 - IP-literal servers never touch DNS.
// PREVENTS: an accidental unconditional resolver call on every server.
func TestServerIPsIPLiteralNoResolver(t *testing.T) {
	restoreAuthSeams(t)
	var called atomic.Bool
	lookupIPAddr = func(context.Context, string) ([]net.IPAddr, error) {
		called.Store(true)
		return nil, errTestResolve
	}

	servers := []radius.Server{
		{Address: "192.0.2.1:1812"},
		{Address: "198.51.100.2:1812"},
	}

	ips := serverIPs(servers)

	require.Len(t, ips, 2)
	assert.False(t, called.Load(), "IP-literal servers must not invoke the resolver")
	assert.Equal(t, "192.0.2.1", ips[0].String())
	assert.Equal(t, "198.51.100.2", ips[1].String())
}
