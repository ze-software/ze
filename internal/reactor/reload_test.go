package reactor

import (
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const emptyConfig = `ze bgp {
}
`

// simpleReloadFunc parses minimal config for testing.
// Supports: neighbor <ip> { local-as <n>; peer-as <n>; passive; }.
func simpleReloadFunc(configPath string) ([]ReloadPeerConfig, error) {
	data, err := os.ReadFile(configPath) //nolint:gosec // test file
	if err != nil {
		return nil, err
	}

	// Simple regex to find neighbor blocks
	neighborRe := regexp.MustCompile(`neighbor\s+(\d+\.\d+\.\d+\.\d+)\s*\{([^}]*)\}`)
	localASRe := regexp.MustCompile(`local-as\s+(\d+)`)
	peerASRe := regexp.MustCompile(`peer-as\s+(\d+)`)
	passiveRe := regexp.MustCompile(`\bpassive\b`)

	var peers []ReloadPeerConfig
	matches := neighborRe.FindAllStringSubmatch(string(data), -1)
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		addr, err := netip.ParseAddr(m[1])
		if err != nil {
			continue
		}
		block := m[2]

		peer := ReloadPeerConfig{Address: addr}

		if laMatch := localASRe.FindStringSubmatch(block); len(laMatch) > 1 {
			var la uint32
			parseUint32(laMatch[1], &la)
			peer.LocalAS = la
		}
		if paMatch := peerASRe.FindStringSubmatch(block); len(paMatch) > 1 {
			var pa uint32
			parseUint32(paMatch[1], &pa)
			peer.PeerAS = pa
		}
		peer.Passive = passiveRe.MatchString(block)

		peers = append(peers, peer)
	}

	return peers, nil
}

// parseUint32 is a helper for test parsing.
func parseUint32(s string, out *uint32) {
	var n uint64
	for _, c := range s {
		if c < '0' || c > '9' {
			return
		}
		n = n*10 + uint64(c-'0')
	}
	if n <= 0xFFFFFFFF {
		*out = uint32(n)
	}
}

// TestReloadAddsPeer verifies that Reload() adds a new peer from updated config.
//
// VALIDATES: Reload adds peers that are in new config but not in current reactor.
// PREVENTS: Reload silently ignoring new peers in config.
func TestReloadAddsPeer(t *testing.T) {
	// Create temp config file with initial empty config
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.conf")

	require.NoError(t, os.WriteFile(configPath, []byte(emptyConfig), 0o600))

	// Create reactor with config path
	cfg := &Config{
		ConfigPath: configPath,
		ListenAddr: "127.0.0.1:0",
	}
	reactor := New(cfg)
	reactor.SetReloadFunc(simpleReloadFunc)
	require.NoError(t, reactor.Start())
	defer reactor.Stop()

	// Verify no peers initially
	assert.Empty(t, reactor.Peers(), "should start with no peers")

	// Update config to add a peer
	updatedConfig := `ze bgp {
    neighbor 10.0.0.1 {
        local-as 65001;
        peer-as 65002;
        passive;
    }
}
`
	require.NoError(t, os.WriteFile(configPath, []byte(updatedConfig), 0o600))

	// Reload config
	adapter := &reactorAPIAdapter{r: reactor}
	err := adapter.Reload()
	require.NoError(t, err)

	// Verify peer was added
	peers := reactor.Peers()
	require.Len(t, peers, 1, "should have 1 peer after reload")
	assert.Equal(t, "10.0.0.1", peers[0].Settings().Address.String())
}

// TestReloadRemovesPeer verifies that Reload() removes peers not in new config.
//
// VALIDATES: Reload removes peers that are in reactor but not in new config.
// PREVENTS: Reload leaving stale peers from old config.
func TestReloadRemovesPeer(t *testing.T) {
	// Create temp config file with a peer
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.conf")

	initialConfig := `ze bgp {
    neighbor 10.0.0.1 {
        local-as 65001;
        peer-as 65002;
        passive;
    }
}
`
	require.NoError(t, os.WriteFile(configPath, []byte(initialConfig), 0o600))

	// Create reactor with config path
	cfg := &Config{
		ConfigPath: configPath,
		ListenAddr: "127.0.0.1:0",
	}
	reactor := New(cfg)
	reactor.SetReloadFunc(simpleReloadFunc)

	// Add the peer manually to simulate initial state
	settings := NewPeerSettings(mustParseAddr("10.0.0.1"), 65001, 65002, 0)
	settings.Passive = true
	_ = reactor.AddPeer(settings)

	require.NoError(t, reactor.Start())
	defer reactor.Stop()

	// Verify peer exists initially
	require.Len(t, reactor.Peers(), 1, "should start with 1 peer")

	// Update config to remove all peers
	require.NoError(t, os.WriteFile(configPath, []byte(emptyConfig), 0o600))

	// Reload config
	adapter := &reactorAPIAdapter{r: reactor}
	err := adapter.Reload()
	require.NoError(t, err)

	// Verify peer was removed
	assert.Empty(t, reactor.Peers(), "should have no peers after reload")
}

// TestReloadParseError verifies that Reload() returns error on bad config.
//
// VALIDATES: Reload returns error when config file is invalid.
// PREVENTS: Reload silently ignoring parse errors.
func TestReloadParseError(t *testing.T) {
	// Create a reload function that fails on specific content
	failingReloadFunc := func(configPath string) ([]ReloadPeerConfig, error) {
		data, err := os.ReadFile(configPath) //nolint:gosec // test file
		if err != nil {
			return nil, err
		}
		if regexp.MustCompile(`invalid`).MatchString(string(data)) {
			return nil, os.ErrInvalid
		}
		return simpleReloadFunc(configPath)
	}

	// Create temp config file with valid initial config
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.conf")

	require.NoError(t, os.WriteFile(configPath, []byte(emptyConfig), 0o600))

	// Create reactor with config path
	cfg := &Config{
		ConfigPath: configPath,
		ListenAddr: "127.0.0.1:0",
	}
	reactor := New(cfg)
	reactor.SetReloadFunc(failingReloadFunc)
	require.NoError(t, reactor.Start())
	defer reactor.Stop()

	// Update config to invalid content
	invalidConfig := `ze bgp {
    invalid syntax here!!!
}
`
	require.NoError(t, os.WriteFile(configPath, []byte(invalidConfig), 0o600))

	// Reload should fail
	adapter := &reactorAPIAdapter{r: reactor}
	err := adapter.Reload()
	require.Error(t, err, "reload should fail on invalid config")
}

// TestReloadNoConfigPath verifies that Reload() returns error when no config path.
//
// VALIDATES: Reload returns clear error when config path not set.
// PREVENTS: Reload panicking or giving confusing error.
func TestReloadNoConfigPath(t *testing.T) {
	cfg := &Config{
		ListenAddr: "127.0.0.1:0",
		// ConfigPath not set
	}
	reactor := New(cfg)
	require.NoError(t, reactor.Start())
	defer reactor.Stop()

	adapter := &reactorAPIAdapter{r: reactor}
	err := adapter.Reload()
	require.Error(t, err, "reload should fail when config path not set")
	assert.ErrorIs(t, err, ErrNoConfigPath)
}

// TestReloadNoReloadFunc verifies that Reload() returns error when no reload func.
//
// VALIDATES: Reload returns error when SetReloadFunc not called.
// PREVENTS: Reload panicking on nil function.
func TestReloadNoReloadFunc(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.conf")
	require.NoError(t, os.WriteFile(configPath, []byte("ze bgp {}"), 0o600))

	cfg := &Config{
		ConfigPath: configPath,
		ListenAddr: "127.0.0.1:0",
	}
	reactor := New(cfg)
	// SetReloadFunc NOT called
	require.NoError(t, reactor.Start())
	defer reactor.Stop()

	adapter := &reactorAPIAdapter{r: reactor}
	err := adapter.Reload()
	require.Error(t, err, "reload should fail when SetReloadFunc not called")
}

// TestReloadFileNotFound verifies that Reload() returns error when config file missing.
//
// VALIDATES: Reload returns error when config file doesn't exist.
// PREVENTS: Reload silently ignoring missing file.
func TestReloadFileNotFound(t *testing.T) {
	cfg := &Config{
		ConfigPath: "/nonexistent/config.conf",
		ListenAddr: "127.0.0.1:0",
	}
	reactor := New(cfg)
	reactor.SetReloadFunc(simpleReloadFunc)
	require.NoError(t, reactor.Start())
	defer reactor.Stop()

	adapter := &reactorAPIAdapter{r: reactor}
	err := adapter.Reload()
	require.Error(t, err, "reload should fail when config file not found")
}
