package hub

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

func TestListenAddrIsNonLoopback(t *testing.T) {
	// VALIDATES: the shared boot-guard classifier is fail-closed -- loopback
	// (v4 full 127.0.0.0/8 + ::1) reads as loopback; every wildcard, routable,
	// hostname, or unparseable host reads as NON-loopback so no name or 0.0.0.0
	// bind can smuggle remote reachability past the guard.
	// PREVENTS: an unauthenticated management surface reachable off-box because
	// its listen host did not parse to a literal loopback IP.
	loopback := []string{
		"127.0.0.1:8080",
		"127.0.0.1",
		"127.255.255.254:1",
		"[::1]:9339",
		"::1",
	}
	nonLoopback := []string{
		"0.0.0.0:9339",
		"[::]:9339",
		"::",
		"128.0.0.1:80",
		"10.0.0.1:443",
		"192.168.1.1:8443",
		"localhost:8080", // hostname does not parse as an IP -> non-loopback
		"example.com:443",
		"",        // empty host -> fail closed
		":9339",   // wildcard bind -> fail closed
		"garbage", // unparseable -> fail closed
	}
	for _, addr := range loopback {
		assert.Falsef(t, listenAddrIsNonLoopback(addr), "%q should classify as loopback", addr)
	}
	for _, addr := range nonLoopback {
		assert.Truef(t, listenAddrIsNonLoopback(addr), "%q should classify as non-loopback", addr)
	}
}

func TestCheckMgmtListeners(t *testing.T) {
	tests := []struct {
		name        string
		listeners   []mgmtListener
		wantRefused bool
	}{
		{
			name:        "no listeners",
			listeners:   nil,
			wantRefused: false,
		},
		{
			// AC-4: loopback + unauthenticated is allowed.
			name: "unauth loopback allowed",
			listeners: []mgmtListener{
				{service: "gNMI", addrs: []string{"127.0.0.1:9339"}, authenticated: false},
				{service: "MCP", addrs: []string{"127.0.0.1:8080"}, authenticated: false},
			},
			wantRefused: false,
		},
		{
			// AC-1: gNMI non-loopback without token is refused.
			name: "unauth non-loopback refused",
			listeners: []mgmtListener{
				{service: "gNMI", addrs: []string{"0.0.0.0:9339"}, authenticated: false},
			},
			wantRefused: true,
		},
		{
			// Authenticated non-loopback is allowed regardless of address.
			name: "authenticated non-loopback allowed",
			listeners: []mgmtListener{
				{service: "gNMI", addrs: []string{"0.0.0.0:9339"}, authenticated: true},
			},
			wantRefused: false,
		},
		{
			// AC-3: web-insecure with a non-loopback listen is refused.
			name: "web insecure non-loopback refused",
			listeners: []mgmtListener{
				{service: "web (insecure)", addrs: []string{"127.0.0.1:3443", "0.0.0.0:3443"}, authenticated: false},
			},
			wantRefused: true,
		},
		{
			// AC-3 fail-closed backstop: an unauthenticated surface with no
			// resolved address is refused, not skipped. Skipping it is how an
			// insecure web server reached 0.0.0.0:3443 past this guard.
			name: "unauth with no resolved address refused",
			listeners: []mgmtListener{
				{service: "web (insecure)", addrs: nil, authenticated: false},
			},
			wantRefused: true,
		},
		{
			// An authenticated surface with no address is still fine: the
			// backstop is about what an UNauthenticated bind would expose.
			name: "authenticated with no resolved address allowed",
			listeners: []mgmtListener{
				{service: "API", addrs: nil, authenticated: true},
			},
			wantRefused: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantRefused, checkMgmtListeners(tt.listeners))
		})
	}
}

func TestMcpListenerAuthenticated(t *testing.T) {
	// VALIDATES: the guard's MCP auth predicate matches the server's effective
	// mode. The load-bearing case is an explicit auth-mode "none" WITH a token:
	// the server ignores the token (accept-all), so the guard must read it as
	// unauthenticated. A token with no/blank auth-mode infers single-bearer.
	// PREVENTS: fail-open where `auth-mode none; token x` on a non-loopback bind
	// slips past the guard because a token was present.
	tests := []struct {
		name     string
		cfgOK    bool
		authMode string
		token    string
		want     bool
	}{
		{"explicit none + token is NOT authenticated", true, "none", "x", false},
		{"explicit none no token", true, "none", "", false},
		{"explicit bearer authenticates", true, "bearer", "", true},
		{"explicit oauth authenticates", true, "oauth", "", true},
		{"blank auth-mode + token infers bearer", true, "", "x", true},
		{"blank auth-mode no token", true, "", "", false},
		{"no config block + env token infers bearer", false, "", "x", true},
		{"no config block no token", false, "", "", false},
		// cfgOK=false must ignore any stray authMode string (only YANG sets it).
		{"no config block ignores authMode arg", false, "bearer", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, mcpListenerAuthenticated(tt.cfgOK, tt.authMode, tt.token))
		})
	}
}

func TestMcpAuthModeAuthenticates(t *testing.T) {
	// VALIDATES: only a real auth-mode gates requests; "" and "none" select the
	// accept-all authenticator so they must read as unauthenticated.
	authed := []string{"bearer", "bearer-list", "oauth"}
	unauthed := []string{"", "none", "unknown", "NONE"}
	for _, m := range authed {
		assert.Truef(t, mcpAuthModeAuthenticates(m), "%q should authenticate", m)
	}
	for _, m := range unauthed {
		assert.Falsef(t, mcpAuthModeAuthenticates(m), "%q should NOT authenticate", m)
	}
}

// nonLoopbackServiceTree builds an enabled environment.<name> block bound to a
// single non-loopback endpoint, for exercising the reload gate.
func nonLoopbackServiceTree(port string) *zeconfig.Tree {
	svc := zeconfig.NewTree()
	svc.Set("enabled", "true")
	srv := zeconfig.NewTree()
	srv.Set("ip", "0.0.0.0")
	srv.Set("port", port)
	svc.AddListEntry("server", "main", srv)
	return svc
}

func webOnlyTree(svc *zeconfig.Tree) *zeconfig.Tree {
	tree := zeconfig.NewTree()
	env := zeconfig.NewTree()
	env.SetContainer("web", svc)
	tree.SetContainer("environment", env)
	return tree
}

func TestReloadListenersRefusesUnauthNonLoopback(t *testing.T) {
	// AC-7: a service built without authentication must not be migrated to a
	// non-loopback address on SIGHUP reload; the daemon keeps its old listeners.
	// PREVENTS: a boot-only exposure guard failing open when a reload moves an
	// unauthenticated listener off loopback.
	web := &recordingReconfigurable{addrs: []string{"127.0.0.1:3443"}}
	migrator := NewListenerMigrator(nil)
	migrator.web = web
	migrator.MarkUnauthenticated("web")

	err := migrator.ReloadListeners(context.Background(), webOnlyTree(nonLoopbackServiceTree("3443")))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "web")
	assert.Contains(t, err.Error(), "0.0.0.0:3443")

	// Nothing was applied: the daemon stays on the previous listener.
	assert.Empty(t, web.calls, "no Reconfigure should have been attempted")
	assert.Equal(t, []string{"127.0.0.1:3443"}, web.addrs)
}

func TestReloadListenersAllowsAuthenticatedNonLoopback(t *testing.T) {
	// A service NOT marked unauthenticated may migrate to a non-loopback
	// address (auth middleware protects it): the gate must not over-refuse.
	web := &recordingReconfigurable{addrs: []string{"127.0.0.1:3443"}}
	migrator := NewListenerMigrator(nil)
	migrator.web = web

	err := migrator.ReloadListeners(context.Background(), webOnlyTree(nonLoopbackServiceTree("3443")))
	require.NoError(t, err)
	assert.Equal(t, [][]string{{"0.0.0.0:3443"}}, web.calls)
	assert.Equal(t, []string{"0.0.0.0:3443"}, web.addrs)
}

// TestResolveWebListenersClosesTheGuardHole pins the fix for the fail-open that
// let an insecure web server bind every interface without authentication.
//
// VALIDATES: fixit-mgmt-listener-auth-guard AC-3 -- a web server that is
// enabled and insecure with no configured listen address is refused, because
// the address it will actually bind is resolved before the guard evaluates it.
//
// PREVENTS: the shipped fail-open. `ze.web.enabled=1` set webEnabled without
// touching webAddrs and `ze.web.insecure=1` cleared authentication, so the
// guard was handed an EMPTY address slice, iterated it zero times, refused
// nothing -- and buildWebService then filled in 0.0.0.0:3443 and served
// unauthenticated on every interface. Reachable from environment variables
// alone, with no config file and no CLI flag.
func TestResolveWebListenersClosesTheGuardHole(t *testing.T) {
	t.Run("enabled with no address resolves to the non-loopback default", func(t *testing.T) {
		got := resolveWebListeners(true, nil)
		assert.Equal(t, []string{defaultWebListen}, got)
		assert.True(t, listenAddrIsNonLoopback(defaultWebListen),
			"the default must be non-loopback, which is why it has to be resolved before the guard")
	})

	t.Run("configured address is left alone", func(t *testing.T) {
		assert.Equal(t, []string{"127.0.0.1:3443"},
			resolveWebListeners(true, []string{"127.0.0.1:3443"}))
	})

	t.Run("disabled web resolves to nothing", func(t *testing.T) {
		assert.Empty(t, resolveWebListeners(false, nil),
			"a disabled web server binds nothing, so it must not gain an address here")
	})

	t.Run("insecure web with no configured address is refused", func(t *testing.T) {
		// The declaration main.go builds for `ze.web.enabled=1 ze.web.insecure=1`.
		refused := checkMgmtListeners([]mgmtListener{{
			service:       "web (insecure)",
			addrs:         resolveWebListeners(true, nil),
			authenticated: false,
		}})
		assert.True(t, refused,
			"an insecure web server that will bind 0.0.0.0 must not start")
	})
}
