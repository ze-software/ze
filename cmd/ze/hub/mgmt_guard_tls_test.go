// Design: ai/rules/evidence.md -- boot-time management-listener TLS exposure guard
// Related: mgmt_guard.go -- filterMgmtListener
// Related: gnmi_infra.go -- gnmiMgmtListener

package hub

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

// VALIDATES: filterMgmtListener refuses any non-loopback listener that marks
// requireTLS without TLS configured (while keeping loopback addresses in
// allowed), allows loopback without TLS, allows non-loopback when both auth
// and TLS are configured, and fails closed when an unencrypted requireTLS
// listener carries no resolved addresses.
// PREVENTS: bearer tokens and management traffic crossing the network in clear
// text on a non-loopback listener.
// Method: table-drives filterMgmtListener directly across all address/auth/TLS
// combinations.
func TestCheckMgmtListenersRequireTLS(t *testing.T) {
	tests := []struct {
		name        string
		listener    mgmtListener
		wantRefused bool
		wantAllowed []string
	}{
		{
			name: "non-loopback authenticated without TLS refused",
			listener: mgmtListener{
				service:       "gNMI",
				addrs:         []string{"0.0.0.0:9339"},
				authenticated: true,
				requireTLS:    true,
				tlsConfigured: false,
				tlsRemedy:     "set TLS cert and key",
			},
			wantRefused: true,
			wantAllowed: nil,
		},
		{
			name: "non-loopback unauthenticated without TLS refused",
			listener: mgmtListener{
				service:       "gNMI",
				addrs:         []string{"0.0.0.0:9339"},
				authenticated: false,
				requireTLS:    true,
				tlsConfigured: false,
			},
			wantRefused: true,
			wantAllowed: nil,
		},
		{
			name: "loopback unauthenticated without TLS allowed",
			listener: mgmtListener{
				service:       "gNMI",
				addrs:         []string{"127.0.0.1:9339"},
				authenticated: false,
				requireTLS:    true,
				tlsConfigured: false,
			},
			wantRefused: false,
			wantAllowed: []string{"127.0.0.1:9339"},
		},
		{
			name: "loopback authenticated without TLS allowed",
			listener: mgmtListener{
				service:       "gNMI",
				addrs:         []string{"[::1]:9339"},
				authenticated: true,
				requireTLS:    true,
				tlsConfigured: false,
			},
			wantRefused: false,
			wantAllowed: []string{"[::1]:9339"},
		},
		{
			name: "non-loopback authenticated with TLS allowed",
			listener: mgmtListener{
				service:       "gNMI",
				addrs:         []string{"0.0.0.0:9339"},
				authenticated: true,
				requireTLS:    true,
				tlsConfigured: true,
			},
			wantRefused: false,
			wantAllowed: []string{"0.0.0.0:9339"},
		},
		{
			name: "requireTLS without TLS and no resolved addresses refused",
			listener: mgmtListener{
				service:       "gNMI",
				addrs:         nil,
				authenticated: true,
				requireTLS:    true,
				tlsConfigured: false,
			},
			wantRefused: true,
			wantAllowed: nil,
		},
		{
			name: "mixed loopback and non-loopback without TLS refused",
			listener: mgmtListener{
				service:       "gNMI",
				addrs:         []string{"127.0.0.1:9339", "10.0.0.1:9339"},
				authenticated: true,
				requireTLS:    true,
				tlsConfigured: false,
			},
			wantRefused: true,
			wantAllowed: []string{"127.0.0.1:9339"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, down := filterMgmtListener(tt.listener)
			assert.Equal(t, tt.wantRefused, len(down) > 0)
			assert.Equal(t, tt.wantAllowed, allowed)
		})
	}
}

// VALIDATES: gnmiMgmtListener produces a guard declaration that refuses
// non-loopback gNMI without TLS (or with only half a TLS pair), passes
// loopback without TLS, and passes non-loopback when token and both TLS paths
// are configured (via YANG or environment overrides).
// PREVENTS: drift between gNMI's config/env resolution and the boot-time
// exposure guard.
// Method: builds config trees with and without TLS paths, calls
// gnmiMgmtListener, and passes the declaration to filterMgmtListener.
func TestGNMIMgmtListenerDeclaration(t *testing.T) {
	resetEnv := func(t *testing.T) {
		t.Helper()
		setGNMIEnv(t, "ze.gnmi.enabled", "")
		setGNMIEnv(t, "ze.gnmi.listen", "")
		setGNMIEnv(t, "ze.gnmi.token", "")
		setGNMIEnv(t, "ze.gnmi.tls.cert", "")
		setGNMIEnv(t, "ze.gnmi.tls.key", "")
	}

	t.Run("disabled gNMI produces no declaration", func(t *testing.T) {
		resetEnv(t)
		_, ok := gnmiMgmtListener(zeconfig.NewTree())
		assert.False(t, ok)
	})

	t.Run("non-loopback with token and no TLS is refused", func(t *testing.T) {
		resetEnv(t)
		tree := zeconfig.NewTree()
		gnmi := tree.GetOrCreateContainer("environment").GetOrCreateContainer("gnmi")
		gnmi.Set("enabled", "true")
		gnmi.Set("token", "s3cret")

		decl, ok := gnmiMgmtListener(tree)
		require.True(t, ok)
		assert.True(t, decl.requireTLS)
		assert.False(t, decl.tlsConfigured)
		allowed, down := filterMgmtListener(decl)
		assert.Empty(t, allowed)
		assert.NotEmpty(t, down)
	})

	t.Run("non-loopback with token and cert only is refused", func(t *testing.T) {
		resetEnv(t)
		tree := zeconfig.NewTree()
		gnmi := tree.GetOrCreateContainer("environment").GetOrCreateContainer("gnmi")
		gnmi.Set("enabled", "true")
		gnmi.Set("token", "s3cret")
		tlsBlock := gnmi.GetOrCreateContainer("tls")
		tlsBlock.Set("cert", "/etc/ze/gnmi.pem")

		decl, ok := gnmiMgmtListener(tree)
		require.True(t, ok)
		assert.False(t, decl.tlsConfigured)
		allowed, down := filterMgmtListener(decl)
		assert.Empty(t, allowed)
		assert.NotEmpty(t, down)
	})

	t.Run("non-loopback with token and both TLS paths is allowed", func(t *testing.T) {
		resetEnv(t)
		tree := zeconfig.NewTree()
		gnmi := tree.GetOrCreateContainer("environment").GetOrCreateContainer("gnmi")
		gnmi.Set("enabled", "true")
		gnmi.Set("token", "s3cret")
		tlsBlock := gnmi.GetOrCreateContainer("tls")
		tlsBlock.Set("cert", "/etc/ze/gnmi.pem")
		tlsBlock.Set("key", "/etc/ze/gnmi.key")

		decl, ok := gnmiMgmtListener(tree)
		require.True(t, ok)
		assert.True(t, decl.authenticated)
		assert.True(t, decl.tlsConfigured)
		allowed, down := filterMgmtListener(decl)
		assert.Equal(t, []string{"0.0.0.0:9339"}, allowed)
		assert.Empty(t, down)
	})

	t.Run("non-loopback with env TLS overrides is allowed", func(t *testing.T) {
		resetEnv(t)
		setGNMIEnv(t, "ze.gnmi.enabled", "true")
		setGNMIEnv(t, "ze.gnmi.token", "s3cret")
		setGNMIEnv(t, "ze.gnmi.tls.cert", "/etc/ze/gnmi.pem")
		setGNMIEnv(t, "ze.gnmi.tls.key", "/etc/ze/gnmi.key")

		decl, ok := gnmiMgmtListener(zeconfig.NewTree())
		require.True(t, ok)
		assert.True(t, decl.tlsConfigured)
		allowed, down := filterMgmtListener(decl)
		assert.Equal(t, []string{"0.0.0.0:9339"}, allowed)
		assert.Empty(t, down)
	})

	t.Run("loopback without token or TLS is allowed", func(t *testing.T) {
		resetEnv(t)
		tree := zeconfig.NewTree()
		gnmi := tree.GetOrCreateContainer("environment").GetOrCreateContainer("gnmi")
		gnmi.Set("enabled", "true")
		srv := zeconfig.NewTree()
		srv.Set("ip", "127.0.0.1")
		srv.Set("port", "9339")
		gnmi.AddListEntry("server", "main", srv)

		decl, ok := gnmiMgmtListener(tree)
		require.True(t, ok)
		assert.False(t, decl.authenticated)
		assert.False(t, decl.tlsConfigured)
		allowed, down := filterMgmtListener(decl)
		assert.Equal(t, []string{"127.0.0.1:9339"}, allowed)
		assert.Empty(t, down)
	})
}
