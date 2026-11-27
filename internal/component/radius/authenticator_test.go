package radius

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/config"
)

// TestRadiusConfigCarriesLoginSet verifies that system/authentication/radius/login-service
// flows through ExtractConfig and radiusBackend.Build onto AuthResult.LoginSet,
// and leaves AuthResult.LoginSet empty when absent.
//
// VALIDATES: AC-12 (OD-1 option d) -- system/authentication/radius/login-service
// reaches AuthResult.LoginSet.
// PREVENTS: Extracted login-service configuration being dropped between
// ExtractConfig, newRadiusAuthenticator, and result.
func TestRadiusConfigCarriesLoginSet(t *testing.T) {
	key := []byte("testing123")

	runCase := func(t *testing.T, loginSet string) (ExtractedConfig, aaa.AuthResult) {
		t.Helper()
		srv := newReplyServer(t, key, CodeAccessAccept, []Attr{
			{Type: AttrFilterID, Value: []byte("operator")},
		})
		t.Cleanup(srv.close)

		host, port, err := net.SplitHostPort(srv.addr)
		require.NoError(t, err)

		inner := config.NewTree()
		server := config.NewTree()
		server.Set("key", string(key))
		server.Set("port", port)
		inner.AddListEntry("server", host, server)
		if loginSet != "" {
			inner.Set("login-service", loginSet)
		}
		tree := radiusTree(inner)

		cfg, err := ExtractConfig(tree)
		require.NoError(t, err)

		contrib, err := radiusBackend{}.Build(buildParamsWithTree(tree, nil))
		require.NoError(t, err)
		require.NotNil(t, contrib.Authenticator)
		t.Cleanup(func() { _ = contrib.Close() })

		res, err := contrib.Authenticator.Authenticate(aaa.AuthRequest{
			Username: "alice",
			Password: "secret",
			Service:  "ssh",
		})
		require.NoError(t, err)
		return cfg, res
	}

	t.Run("configured login-service reaches AuthResult.LoginSet", func(t *testing.T) {
		cfg, res := runCase(t, "ssh-only")
		assert.Equal(t, "ssh-only", cfg.LoginSet)
		assert.True(t, res.Authenticated)
		assert.Equal(t, "radius", res.Source)
		assert.Equal(t, []string{"operator"}, res.Profiles)
		assert.Equal(t, "ssh-only", res.LoginSet)
	})

	t.Run("absent login-service leaves AuthResult.LoginSet empty", func(t *testing.T) {
		cfg, res := runCase(t, "")
		assert.Empty(t, cfg.LoginSet)
		assert.True(t, res.Authenticated)
		assert.Equal(t, "radius", res.Source)
		assert.Equal(t, []string{"operator"}, res.Profiles)
		assert.Empty(t, res.LoginSet)
	})
}
