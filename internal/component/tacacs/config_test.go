package tacacs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// VALIDATES: ExtractConfig returns zero config for nil tree.
// PREVENTS: nil pointer panic when config tree is unavailable.
func TestExtractConfigNilTree(t *testing.T) {
	cfg := ExtractConfig(nil)
	assert.False(t, cfg.HasServers())
	assert.Nil(t, cfg.PrivLvlMap)
}

// VALIDATES: ExtractConfig returns zero config for empty tree.
// PREVENTS: panic on tree with no system container.
func TestExtractConfigEmptyTree(t *testing.T) {
	tree := config.NewTree()
	cfg := ExtractConfig(tree)
	assert.False(t, cfg.HasServers())
}

// VALIDATES: ExtractConfig parses TACACS+ server list with ports and keys.
// PREVENTS: wrong field mapping from YANG to Go struct.
func TestExtractConfigServers(t *testing.T) {
	tree := config.NewTree()
	sys := config.NewTree()
	auth := config.NewTree()
	tac := config.NewTree()

	srv1 := config.NewTree()
	srv1.Set("port", "49")
	srv1.Set("key", "secret-one")
	tac.AddListEntry("server", "10.0.0.1", srv1)

	srv2 := config.NewTree()
	srv2.Set("port", "4949")
	srv2.Set("key", "secret-two")
	tac.AddListEntry("server", "10.0.0.2", srv2)

	tac.Set("timeout", "10")
	tac.Set("source-address", "192.168.1.1")
	tac.Set("authorization", "true")
	tac.Set("accounting", "true")

	auth.SetContainer("tacacs", tac)
	sys.SetContainer("authentication", auth)
	tree.SetContainer("system", sys)

	cfg := ExtractConfig(tree)

	require.True(t, cfg.HasServers())
	require.Len(t, cfg.Servers, 2)
	assert.Equal(t, "10.0.0.1:49", cfg.Servers[0].Address)
	assert.Equal(t, []byte("secret-one"), cfg.Servers[0].Key)
	assert.Equal(t, "10.0.0.2:4949", cfg.Servers[1].Address)
	assert.Equal(t, []byte("secret-two"), cfg.Servers[1].Key)
	assert.Equal(t, 10*time.Second, cfg.Timeout)
	assert.Equal(t, "192.168.1.1", cfg.SourceAddress)
	assert.True(t, cfg.Authorization)
	assert.True(t, cfg.Accounting)
}

// VALIDATES: ExtractConfig parses privilege level to profile mapping.
// PREVENTS: wrong priv-lvl parsing or missing profiles.
func TestExtractConfigPrivLvlMap(t *testing.T) {
	tree := config.NewTree()
	sys := config.NewTree()
	auth := config.NewTree()

	lvl15 := config.NewTree()
	lvl15.SetSlice("profile", []string{"admin"})

	lvl1 := config.NewTree()
	lvl1.SetSlice("profile", []string{"read-only"})

	auth.AddListEntry("tacacs-profile", "15", lvl15)
	auth.AddListEntry("tacacs-profile", "1", lvl1)

	sys.SetContainer("authentication", auth)
	tree.SetContainer("system", sys)

	cfg := ExtractConfig(tree)

	require.NotNil(t, cfg.PrivLvlMap)
	assert.Equal(t, []string{"admin"}, cfg.PrivLvlMap[15])
	assert.Equal(t, []string{"read-only"}, cfg.PrivLvlMap[1])
}

// VALIDATES: ExtractConfig uses default timeout when not specified.
// PREVENTS: zero timeout causing immediate connection failure.
func TestExtractConfigDefaultTimeout(t *testing.T) {
	tree := config.NewTree()
	sys := config.NewTree()
	auth := config.NewTree()
	tac := config.NewTree()

	srv := config.NewTree()
	srv.Set("key", "k")
	tac.AddListEntry("server", "10.0.0.1", srv)

	auth.SetContainer("tacacs", tac)
	sys.SetContainer("authentication", auth)
	tree.SetContainer("system", sys)

	cfg := ExtractConfig(tree)

	assert.Equal(t, 5*time.Second, cfg.Timeout, "default timeout should be 5s")
}

// VALIDATES: ExtractConfig with no tacacs container returns no servers.
// PREVENTS: false positive HasServers when only local users configured.
func TestExtractConfigNoTacacs(t *testing.T) {
	tree := config.NewTree()
	sys := config.NewTree()
	auth := config.NewTree()

	user := config.NewTree()
	user.Set("password", "$2a$10$hash")
	auth.AddListEntry("user", "admin", user)

	sys.SetContainer("authentication", auth)
	tree.SetContainer("system", sys)

	cfg := ExtractConfig(tree)
	assert.False(t, cfg.HasServers())
	assert.False(t, cfg.Authorization)
	assert.False(t, cfg.Accounting)
}
