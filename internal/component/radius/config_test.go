package radius

import (
	"bytes"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
)

// radiusTree builds system/authentication/radius with the given inner tree.
func radiusTree(inner *config.Tree) *config.Tree {
	tree := config.NewTree()
	sys := config.NewTree()
	auth := config.NewTree()
	auth.SetContainer("radius", inner)
	sys.SetContainer("authentication", auth)
	tree.SetContainer("system", sys)
	return tree
}

// VALIDATES: ExtractConfig returns zero config for a nil tree.
// PREVENTS: nil pointer panic when the config tree is unavailable at Build.
func TestExtractRadiusConfigNilTree(t *testing.T) {
	cfg, err := ExtractConfig(nil)
	require.NoError(t, err)
	assert.False(t, cfg.HasServers())
}

// VALIDATES: ExtractConfig returns no servers for an empty tree.
// PREVENTS: false-positive HasServers when RADIUS is not configured (AC-2).
func TestExtractRadiusConfigEmptyTree(t *testing.T) {
	cfg, err := ExtractConfig(config.NewTree())
	require.NoError(t, err)
	assert.False(t, cfg.HasServers())
}

// VALIDATES: ExtractConfig parses servers, timeout, retries, source-address,
// profile-attribute and default-profile from the YANG subtree.
// PREVENTS: wrong field mapping from YANG to ExtractedConfig.
func TestExtractRadiusConfig(t *testing.T) {
	inner := config.NewTree()

	srv1 := config.NewTree()
	srv1.Set("port", "1812")
	srv1.Set("key", "secret-one")
	inner.AddListEntry("server", "10.0.0.1", srv1)

	srv2 := config.NewTree()
	srv2.Set("port", "1645")
	srv2.Set("key", "secret-two")
	inner.AddListEntry("server", "10.0.0.2", srv2)

	inner.Set("timeout", "7")
	inner.Set("retries", "2")
	inner.Set("source-address", "192.168.1.1")
	inner.Set("profile-attribute", "filter-id")
	inner.SetSlice("default-profile", []string{"read-only"})

	cfg, err := ExtractConfig(radiusTree(inner))
	require.NoError(t, err)

	require.True(t, cfg.HasServers())
	require.Len(t, cfg.Servers, 2)
	assert.Equal(t, "10.0.0.1:1812", cfg.Servers[0].Address)
	assert.Equal(t, []byte("secret-one"), cfg.Servers[0].SharedKey)
	assert.Equal(t, "10.0.0.2:1645", cfg.Servers[1].Address)
	assert.Equal(t, []byte("secret-two"), cfg.Servers[1].SharedKey)
	assert.Equal(t, 7*time.Second, cfg.Timeout)
	assert.Equal(t, 2, cfg.Retries)
	assert.True(t, cfg.SourceAddress.Equal(net.ParseIP("192.168.1.1")))
	assert.Equal(t, uint8(AttrFilterID), cfg.ProfileAttr)
	assert.Equal(t, []string{"read-only"}, cfg.DefaultProfiles)
}

// VALIDATES: ExtractConfig applies the YANG defaults when leaves are absent.
// PREVENTS: zero timeout/retries and a missing profile attribute default.
func TestExtractRadiusConfigDefaults(t *testing.T) {
	inner := config.NewTree()
	srv := config.NewTree()
	srv.Set("key", "k")
	inner.AddListEntry("server", "10.0.0.1", srv)

	cfg, err := ExtractConfig(radiusTree(inner))
	require.NoError(t, err)

	require.True(t, cfg.HasServers())
	assert.Equal(t, "10.0.0.1:1812", cfg.Servers[0].Address, "default port 1812")
	assert.Equal(t, defaultTimeout, cfg.Timeout, "default timeout 3s")
	assert.Equal(t, defaultRetries, cfg.Retries, "default retries 3")
	assert.Equal(t, uint8(AttrFilterID), cfg.ProfileAttr, "default profile attr Filter-Id")
}

// VALIDATES: profile-attribute "class" maps to the RADIUS Class attribute (25).
// PREVENTS: silently ignoring an operator's non-default profile carrier (AC-6).
func TestExtractRadiusConfigProfileAttrClass(t *testing.T) {
	inner := config.NewTree()
	srv := config.NewTree()
	srv.Set("key", "k")
	inner.AddListEntry("server", "10.0.0.1", srv)
	inner.Set("profile-attribute", "class")

	cfg, err := ExtractConfig(radiusTree(inner))
	require.NoError(t, err)
	assert.Equal(t, uint8(attrClass), cfg.ProfileAttr)
}

// VALIDATES: ExtractConfig extracts the last-valid boundary values verbatim.
// PREVENTS: overflow/truncation of port 65535, timeout 60, retries 0 and 10.
func TestExtractRadiusConfigBoundaryValues(t *testing.T) {
	inner := config.NewTree()
	srv := config.NewTree()
	srv.Set("port", "65535")
	srv.Set("key", "k")
	inner.AddListEntry("server", "10.0.0.1", srv)
	inner.Set("timeout", "60")
	inner.Set("retries", "10")

	cfg, err := ExtractConfig(radiusTree(inner))
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1:65535", cfg.Servers[0].Address)
	assert.Equal(t, 60*time.Second, cfg.Timeout)
	assert.Equal(t, 10, cfg.Retries)

	inner.Set("retries", "0")
	cfg, err = ExtractConfig(radiusTree(inner))
	require.NoError(t, err)
	assert.Equal(t, 0, cfg.Retries, "explicit retries 0 preserved by extraction")
}

// VALIDATES: the RADIUS shared secret never appears in log output produced
// while building the backend (AC-8).
// PREVENTS: leaking the secret into logs even though the YANG leaf is sensitive.
func TestRadiusSecretNotLogged(t *testing.T) {
	const secret = "top-secret-radius-key"
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	inner := config.NewTree()
	srv := config.NewTree()
	srv.Set("key", secret)
	inner.AddListEntry("server", "127.0.0.1", srv)

	contrib, err := radiusBackend{}.Build(buildParamsWithTree(radiusTree(inner), logger))
	require.NoError(t, err)
	if contrib.Close != nil {
		t.Cleanup(func() { _ = contrib.Close() })
	}
	assert.NotContains(t, buf.String(), secret, "shared secret must never be logged")
}
