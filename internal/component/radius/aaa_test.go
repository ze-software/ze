package radius

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/aaa"
	"github.com/ze-software/ze/internal/component/config"
)

func buildParamsWithTree(tree *config.Tree, logger *slog.Logger) aaa.BuildParams {
	return aaa.BuildParams{
		Ctx:        context.Background(),
		ConfigTree: tree,
		Logger:     logger,
	}
}

// VALIDATES: AC-2 Build returns an empty Contribution with no radius config.
// PREVENTS: the backend contributing an authenticator that always fails when
// RADIUS admin auth is not configured.
func TestRadiusBuildEmptyWhenUnconfigured(t *testing.T) {
	contrib, err := radiusBackend{}.Build(buildParamsWithTree(config.NewTree(), nil))
	require.NoError(t, err)
	assert.Nil(t, contrib.Authenticator)
	assert.Nil(t, contrib.Authorizer)
	assert.Nil(t, contrib.Accountant)
	assert.Nil(t, contrib.Close)
}

// VALIDATES: AC-1 Build returns a non-nil Authenticator + Close when at least
// one server is configured.
// PREVENTS: a configured RADIUS backend contributing nothing to the chain.
func TestRadiusBuildReturnsAuthenticatorWhenConfigured(t *testing.T) {
	inner := config.NewTree()
	srv := config.NewTree()
	srv.Set("key", "secret")
	inner.AddListEntry("server", "127.0.0.1", srv)

	contrib, err := radiusBackend{}.Build(buildParamsWithTree(radiusTree(inner), nil))
	require.NoError(t, err)
	require.NotNil(t, contrib.Authenticator, "configured backend must contribute an authenticator")
	require.NotNil(t, contrib.Close, "client-owning backend must contribute a Close hook")
	assert.NoError(t, contrib.Close())
}

// VALIDATES: the backend keeps its reserved name/priority slot (A-2: 50).
// PREVENTS: an accidental priority change reshuffling the auth chain order.
func TestRadiusBackendNameAndPriority(t *testing.T) {
	assert.Equal(t, "radius", radiusBackend{}.Name())
	assert.Equal(t, 50, radiusBackend{}.Priority())
}

// VALIDATES: a RADIUS client-init failure (unbindable source-address) degrades
// to an empty Contribution instead of erroring, so the AAA bundle build does
// not abort and local auth still works (R-4: never lock the operator out).
// PREVENTS: a bad source-address failing the whole bundle -> SSH never built.
func TestRadiusBuildDegradesOnClientInitFailure(t *testing.T) {
	inner := config.NewTree()
	srv := config.NewTree()
	srv.Set("key", "secret")
	inner.AddListEntry("server", "10.0.0.1", srv)
	// 198.51.100.1 (TEST-NET-2) is not assigned to the host, so binding the
	// outbound socket to it fails inside NewClient.
	inner.Set("source-address", "198.51.100.1")

	contrib, err := radiusBackend{}.Build(buildParamsWithTree(radiusTree(inner), nil))
	require.NoError(t, err, "client-init failure must NOT fail the bundle build")
	assert.Nil(t, contrib.Authenticator, "degraded backend contributes no authenticator")
	assert.Nil(t, contrib.Close)
}
