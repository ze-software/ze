package reactor

import (
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/capability"
)

const emptyConfig = `ze bgp {
}
`

// simpleReloadFunc parses minimal config for testing.
// Supports: neighbor <ip> { local-as <n>; peer-as <n>; hold-time <n>; passive; }.
func simpleReloadFunc(configPath string) ([]*PeerSettings, error) {
	data, err := os.ReadFile(configPath) //nolint:gosec // test file
	if err != nil {
		return nil, err
	}

	// Simple regex to find neighbor blocks.
	neighborRe := regexp.MustCompile(`neighbor\s+(\d+\.\d+\.\d+\.\d+)\s*\{([^}]*)\}`)
	localASRe := regexp.MustCompile(`local-as\s+(\d+)`)
	peerASRe := regexp.MustCompile(`peer-as\s+(\d+)`)
	holdTimeRe := regexp.MustCompile(`hold-time\s+(\d+)`)
	passiveRe := regexp.MustCompile(`\bpassive\b`)

	var peers []*PeerSettings
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

		var localAS, peerAS uint32
		if laMatch := localASRe.FindStringSubmatch(block); len(laMatch) > 1 {
			parseUint32(laMatch[1], &localAS)
		}
		if paMatch := peerASRe.FindStringSubmatch(block); len(paMatch) > 1 {
			parseUint32(paMatch[1], &peerAS)
		}

		peer := NewPeerSettings(addr, localAS, peerAS, 0)
		peer.Connection = ConnectionBoth
		if passiveRe.MatchString(block) {
			peer.Connection = ConnectionPassive
		}

		if htMatch := holdTimeRe.FindStringSubmatch(block); len(htMatch) > 1 {
			var ht uint32
			parseUint32(htMatch[1], &ht)
			peer.HoldTime = time.Duration(ht) * time.Second
		}

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
	// Create temp config file with initial empty config.
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.conf")

	require.NoError(t, os.WriteFile(configPath, []byte(emptyConfig), 0o600))

	// Create reactor with config path.
	cfg := &Config{
		ConfigPath: configPath,
		ListenAddr: "127.0.0.1:0",
	}
	reactor := New(cfg)
	reactor.SetReloadFunc(simpleReloadFunc)
	require.NoError(t, reactor.Start())
	defer reactor.Stop()

	// Verify no peers initially.
	assert.Empty(t, reactor.Peers(), "should start with no peers")

	// Update config to add a peer.
	updatedConfig := `ze bgp {
    neighbor 10.0.0.1 {
        local-as 65001;
        peer-as 65002;
        passive;
    }
}
`
	require.NoError(t, os.WriteFile(configPath, []byte(updatedConfig), 0o600))

	// Reload config.
	adapter := &reactorAPIAdapter{r: reactor}
	err := adapter.Reload()
	require.NoError(t, err)

	// Verify peer was added.
	peers := reactor.Peers()
	require.Len(t, peers, 1, "should have 1 peer after reload")
	assert.Equal(t, "10.0.0.1", peers[0].Settings().Address.String())
}

// TestReloadRemovesPeer verifies that Reload() removes peers not in new config.
//
// VALIDATES: Reload removes peers that are in reactor but not in new config.
// PREVENTS: Reload leaving stale peers from old config.
func TestReloadRemovesPeer(t *testing.T) {
	// Create temp config file with a peer.
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

	// Create reactor with config path.
	cfg := &Config{
		ConfigPath: configPath,
		ListenAddr: "127.0.0.1:0",
	}
	reactor := New(cfg)
	reactor.SetReloadFunc(simpleReloadFunc)

	// Add the peer manually to simulate initial state.
	settings := NewPeerSettings(mustParseAddr("10.0.0.1"), 65001, 65002, 0)
	settings.Connection = ConnectionPassive
	_ = reactor.AddPeer(settings)

	require.NoError(t, reactor.Start())
	defer reactor.Stop()

	// Verify peer exists initially.
	require.Len(t, reactor.Peers(), 1, "should start with 1 peer")

	// Update config to remove all peers.
	require.NoError(t, os.WriteFile(configPath, []byte(emptyConfig), 0o600))

	// Reload config.
	adapter := &reactorAPIAdapter{r: reactor}
	err := adapter.Reload()
	require.NoError(t, err)

	// Verify peer was removed.
	assert.Empty(t, reactor.Peers(), "should have no peers after reload")
}

// TestReloadChangedSettings verifies that Reload() updates peers with changed settings.
//
// VALIDATES: Reload detects settings changes and re-adds peer with new settings.
// PREVENTS: Peers keeping old settings when config changes.
func TestReloadChangedSettings(t *testing.T) {
	// Create temp config file with initial peer settings.
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.conf")

	initialConfig := `ze bgp {
    neighbor 10.0.0.1 {
        local-as 65001;
        peer-as 65002;
        hold-time 90;
        passive;
    }
}
`
	require.NoError(t, os.WriteFile(configPath, []byte(initialConfig), 0o600))

	// Create reactor with config path.
	cfg := &Config{
		ConfigPath: configPath,
		ListenAddr: "127.0.0.1:0",
	}
	reactor := New(cfg)
	reactor.SetReloadFunc(simpleReloadFunc)

	// Add the peer manually with initial settings.
	settings := NewPeerSettings(mustParseAddr("10.0.0.1"), 65001, 65002, 0)
	settings.Connection = ConnectionPassive
	settings.HoldTime = 90 * time.Second
	_ = reactor.AddPeer(settings)

	require.NoError(t, reactor.Start())
	defer reactor.Stop()

	// Verify initial hold time.
	peers := reactor.Peers()
	require.Len(t, peers, 1, "should start with 1 peer")
	assert.Equal(t, 90*time.Second, peers[0].Settings().HoldTime, "initial hold time")

	// Update config with changed hold time.
	updatedConfig := `ze bgp {
    neighbor 10.0.0.1 {
        local-as 65001;
        peer-as 65002;
        hold-time 30;
        passive;
    }
}
`
	require.NoError(t, os.WriteFile(configPath, []byte(updatedConfig), 0o600))

	// Reload config.
	adapter := &reactorAPIAdapter{r: reactor}
	err := adapter.Reload()
	require.NoError(t, err)

	// Verify peer has new hold time.
	peers = reactor.Peers()
	require.Len(t, peers, 1, "should still have 1 peer after reload")
	assert.Equal(t, "10.0.0.1", peers[0].Settings().Address.String())
	assert.Equal(t, 30*time.Second, peers[0].Settings().HoldTime, "hold time should be updated")
}

// TestReloadParseError verifies that Reload() returns error on bad config.
//
// VALIDATES: Reload returns error when config file is invalid.
// PREVENTS: Reload silently ignoring parse errors.
func TestReloadParseError(t *testing.T) {
	// Create a reload function that fails on specific content.
	failingReloadFunc := func(configPath string) ([]*PeerSettings, error) {
		data, err := os.ReadFile(configPath) //nolint:gosec // test file
		if err != nil {
			return nil, err
		}
		if regexp.MustCompile(`invalid`).MatchString(string(data)) {
			return nil, os.ErrInvalid
		}
		return simpleReloadFunc(configPath)
	}

	// Create temp config file with valid initial config.
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.conf")

	require.NoError(t, os.WriteFile(configPath, []byte(emptyConfig), 0o600))

	// Create reactor with config path.
	cfg := &Config{
		ConfigPath: configPath,
		ListenAddr: "127.0.0.1:0",
	}
	reactor := New(cfg)
	reactor.SetReloadFunc(failingReloadFunc)
	require.NoError(t, reactor.Start())
	defer reactor.Stop()

	// Update config to invalid content.
	invalidConfig := `ze bgp {
    invalid syntax here!!!
}
`
	require.NoError(t, os.WriteFile(configPath, []byte(invalidConfig), 0o600))

	// Reload should fail.
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
		// ConfigPath not set.
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
// VALIDATES: Reload returns ErrNoReloadFunc when SetReloadFunc not called.
// PREVENTS: Reload panicking on nil function or returning misleading error.
func TestReloadNoReloadFunc(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.conf")
	require.NoError(t, os.WriteFile(configPath, []byte("ze bgp {}"), 0o600))

	cfg := &Config{
		ConfigPath: configPath,
		ListenAddr: "127.0.0.1:0",
	}
	reactor := New(cfg)
	// SetReloadFunc NOT called.
	require.NoError(t, reactor.Start())
	defer reactor.Stop()

	adapter := &reactorAPIAdapter{r: reactor}
	err := adapter.Reload()
	require.Error(t, err, "reload should fail when SetReloadFunc not called")
	assert.ErrorIs(t, err, ErrNoReloadFunc)
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

// TestPeerSettingsEqual verifies the peer settings comparison function.
//
// VALIDATES: peerSettingsEqual correctly detects setting differences.
// PREVENTS: Reload missing setting changes due to bad comparison.
func TestPeerSettingsEqual(t *testing.T) {
	base := NewPeerSettings(mustParseAddr("10.0.0.1"), 65001, 65002, 0)
	base.HoldTime = 90 * time.Second
	base.Connection = ConnectionPassive

	tests := []struct {
		name  string
		mod   func(*PeerSettings)
		equal bool
	}{
		{"identical", func(p *PeerSettings) {}, true},
		{"different_local_as", func(p *PeerSettings) { p.LocalAS = 65099 }, false},
		{"different_peer_as", func(p *PeerSettings) { p.PeerAS = 65099 }, false},
		{"different_hold_time", func(p *PeerSettings) { p.HoldTime = 30 * time.Second }, false},
		{"different_connection", func(p *PeerSettings) { p.Connection = ConnectionBoth }, false},
		{"different_port", func(p *PeerSettings) { p.Port = 1179 }, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of base settings.
			other := NewPeerSettings(base.Address, base.LocalAS, base.PeerAS, base.RouterID)
			other.HoldTime = base.HoldTime
			other.Connection = base.Connection
			other.Port = base.Port

			// Apply modification.
			tt.mod(other)

			result := peerSettingsEqual(base, other)
			assert.Equal(t, tt.equal, result, "peerSettingsEqual mismatch")
		})
	}
}

// TestPeerSettingsEqualCapabilityChange verifies that capability content changes
// are detected by peerSettingsEqual, not just capability count changes.
//
// VALIDATES: Changing ADD-PATH mode triggers peer restart on reload.
// PREVENTS: Capability content changes being silently ignored (count-only comparison).
func TestPeerSettingsEqualCapabilityChange(t *testing.T) {
	base := NewPeerSettings(mustParseAddr("10.0.0.1"), 65001, 65002, 0)
	base.Capabilities = []capability.Capability{
		&capability.RouteRefresh{},
		&capability.AddPath{Families: []capability.AddPathFamily{
			{AFI: 1, SAFI: 1, Mode: capability.AddPathReceive},
		}},
	}

	tests := []struct {
		name  string
		caps  []capability.Capability
		equal bool
	}{
		{
			"identical_caps",
			[]capability.Capability{
				&capability.RouteRefresh{},
				&capability.AddPath{Families: []capability.AddPathFamily{
					{AFI: 1, SAFI: 1, Mode: capability.AddPathReceive},
				}},
			},
			true,
		},
		{
			"addpath_mode_changed",
			[]capability.Capability{
				&capability.RouteRefresh{},
				&capability.AddPath{Families: []capability.AddPathFamily{
					{AFI: 1, SAFI: 1, Mode: capability.AddPathBoth},
				}},
			},
			false,
		},
		{
			"cap_added",
			[]capability.Capability{
				&capability.RouteRefresh{},
				&capability.AddPath{Families: []capability.AddPathFamily{
					{AFI: 1, SAFI: 1, Mode: capability.AddPathReceive},
				}},
				&capability.ExtendedMessage{},
			},
			false,
		},
		{
			"cap_removed",
			[]capability.Capability{
				&capability.RouteRefresh{},
			},
			false,
		},
		{
			"empty_vs_nonempty",
			nil,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			other := NewPeerSettings(base.Address, base.LocalAS, base.PeerAS, base.RouterID)
			other.Capabilities = tt.caps

			result := peerSettingsEqual(base, other)
			assert.Equal(t, tt.equal, result, "peerSettingsEqual mismatch for %s", tt.name)
		})
	}
}

// TestCapabilitiesEqualEmpty verifies edge cases for capabilitiesEqual.
//
// VALIDATES: Both nil and empty capability slices compare equal.
// PREVENTS: Nil vs empty slice causing false positives.
func TestCapabilitiesEqualEmpty(t *testing.T) {
	assert.True(t, capabilitiesEqual(nil, nil))
	assert.True(t, capabilitiesEqual([]capability.Capability{}, []capability.Capability{}))
	assert.True(t, capabilitiesEqual(nil, []capability.Capability{}))
}

// --- VerifyConfig / ApplyConfigDiff tests ---

// makeBGPTree builds a bgp config tree with the given peers.
// Each peer is defined by address key → field map (strings, matching config tree format).
func makeBGPTree(peers map[string]map[string]string) map[string]any {
	peerMap := make(map[string]any, len(peers))
	for addr, fields := range peers {
		m := make(map[string]any, len(fields))
		for k, v := range fields {
			m[k] = v
		}
		peerMap[addr] = m
	}
	return map[string]any{
		"peer": peerMap,
	}
}

// TestReactorVerifyConfigValid verifies that VerifyConfig accepts valid peer settings.
//
// VALIDATES: Valid peer config tree passes verification without error.
// PREVENTS: VerifyConfig rejecting well-formed config.
func TestReactorVerifyConfigValid(t *testing.T) {
	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	r := New(cfg)
	require.NoError(t, r.Start())
	defer r.Stop()

	adapter := &reactorAPIAdapter{r: r}
	bgpTree := makeBGPTree(map[string]map[string]string{
		"10.0.0.1": {"peer-as": "65001", "local-as": "65000"},
		"10.0.0.2": {"peer-as": "65002", "local-as": "65000"},
	})

	err := adapter.VerifyConfig(bgpTree)
	require.NoError(t, err)
}

// TestReactorVerifyConfigInvalidAddress verifies that VerifyConfig rejects invalid peer address.
//
// VALIDATES: Invalid peer address key → error returned.
// PREVENTS: Bad address silently accepted, causing crash on AddPeer.
func TestReactorVerifyConfigInvalidAddress(t *testing.T) {
	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	r := New(cfg)
	require.NoError(t, r.Start())
	defer r.Stop()

	adapter := &reactorAPIAdapter{r: r}
	bgpTree := makeBGPTree(map[string]map[string]string{
		"not-an-ip": {"peer-as": "65001", "local-as": "65000"},
	})

	err := adapter.VerifyConfig(bgpTree)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not-an-ip")
}

// TestReactorVerifyConfigNoMutation verifies that VerifyConfig does NOT modify reactor state.
//
// VALIDATES: Calling VerifyConfig does not add, remove, or modify peers.
// PREVENTS: Verify phase accidentally applying config changes.
func TestReactorVerifyConfigNoMutation(t *testing.T) {
	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	r := New(cfg)

	// Add an existing peer.
	settings := NewPeerSettings(mustParseAddr("10.0.0.1"), 65001, 65002, 0)
	settings.Connection = ConnectionPassive
	_ = r.AddPeer(settings)

	require.NoError(t, r.Start())
	defer r.Stop()

	// Snapshot peer count before verify.
	peersBefore := len(r.Peers())

	adapter := &reactorAPIAdapter{r: r}
	// Verify a config that would add a new peer and remove existing one.
	bgpTree := makeBGPTree(map[string]map[string]string{
		"10.0.0.99": {"peer-as": "65099", "local-as": "65000"},
	})

	err := adapter.VerifyConfig(bgpTree)
	require.NoError(t, err)

	// Peer count must be unchanged — verify is read-only.
	peersAfter := len(r.Peers())
	assert.Equal(t, peersBefore, peersAfter, "VerifyConfig must not mutate reactor state")
}

// TestReactorApplyConfigDiffAddPeer verifies that ApplyConfigDiff adds new peers.
//
// VALIDATES: Peer in new config but not current → added to reactor.
// PREVENTS: New peers being silently ignored during apply.
func TestReactorApplyConfigDiffAddPeer(t *testing.T) {
	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	r := New(cfg)
	require.NoError(t, r.Start())
	defer r.Stop()

	assert.Empty(t, r.Peers(), "should start with no peers")

	adapter := &reactorAPIAdapter{r: r}
	bgpTree := makeBGPTree(map[string]map[string]string{
		"10.0.0.1": {"peer-as": "65001", "local-as": "65000"},
	})

	err := adapter.ApplyConfigDiff(bgpTree)
	require.NoError(t, err)

	peers := r.Peers()
	require.Len(t, peers, 1)
	assert.Equal(t, "10.0.0.1", peers[0].Settings().Address.String())
}

// TestReactorApplyConfigDiffRemovePeer verifies that ApplyConfigDiff removes peers not in new config.
//
// VALIDATES: Peer in reactor but not in new config → removed.
// PREVENTS: Stale peers remaining after config change.
func TestReactorApplyConfigDiffRemovePeer(t *testing.T) {
	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	r := New(cfg)

	settings := NewPeerSettings(mustParseAddr("10.0.0.1"), 65001, 65002, 0)
	settings.Connection = ConnectionPassive
	_ = r.AddPeer(settings)

	require.NoError(t, r.Start())
	defer r.Stop()
	require.Len(t, r.Peers(), 1)

	adapter := &reactorAPIAdapter{r: r}
	// Empty peer map — peer should be removed.
	bgpTree := makeBGPTree(map[string]map[string]string{})

	err := adapter.ApplyConfigDiff(bgpTree)
	require.NoError(t, err)

	assert.Empty(t, r.Peers(), "peer should be removed")
}

// TestReactorApplyConfigDiffChangedPeer verifies that ApplyConfigDiff restarts changed peers.
//
// VALIDATES: Peer with changed settings → removed and re-added with new settings.
// PREVENTS: Changed peer keeping old settings after config reload.
func TestReactorApplyConfigDiffChangedPeer(t *testing.T) {
	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	r := New(cfg)

	settings := NewPeerSettings(mustParseAddr("10.0.0.1"), 65001, 65002, 0)
	settings.Connection = ConnectionPassive
	settings.HoldTime = 90 * time.Second
	_ = r.AddPeer(settings)

	require.NoError(t, r.Start())
	defer r.Stop()

	peers := r.Peers()
	require.Len(t, peers, 1)
	assert.Equal(t, 90*time.Second, peers[0].Settings().HoldTime)

	adapter := &reactorAPIAdapter{r: r}
	// Same peer, different hold time.
	bgpTree := makeBGPTree(map[string]map[string]string{
		"10.0.0.1": {"peer-as": "65002", "local-as": "65001", "hold-time": "30"},
	})

	err := adapter.ApplyConfigDiff(bgpTree)
	require.NoError(t, err)

	peers = r.Peers()
	require.Len(t, peers, 1)
	assert.Equal(t, "10.0.0.1", peers[0].Settings().Address.String())
	assert.Equal(t, 30*time.Second, peers[0].Settings().HoldTime)
}

// TestReactorReloadBackwardCompat verifies that existing Reload() still works.
//
// VALIDATES: Reload() wrapper using reloadFunc still adds/removes peers.
// PREVENTS: Refactoring breaking the existing Reload() code path.
func TestReactorReloadBackwardCompat(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.conf")

	require.NoError(t, os.WriteFile(configPath, []byte(emptyConfig), 0o600))

	cfg := &Config{
		ConfigPath: configPath,
		ListenAddr: "127.0.0.1:0",
	}
	r := New(cfg)
	r.SetReloadFunc(simpleReloadFunc)
	require.NoError(t, r.Start())
	defer r.Stop()

	assert.Empty(t, r.Peers())

	// Update config to add a peer.
	updatedConfig := `ze bgp {
    neighbor 10.0.0.1 {
        local-as 65001;
        peer-as 65002;
        passive;
    }
}
`
	require.NoError(t, os.WriteFile(configPath, []byte(updatedConfig), 0o600))

	adapter := &reactorAPIAdapter{r: r}
	err := adapter.Reload()
	require.NoError(t, err)

	require.Len(t, r.Peers(), 1)
	assert.Equal(t, "10.0.0.1", r.Peers()[0].Settings().Address.String())
}

// TestReactorVerifyConfigNoPeerSection verifies VerifyConfig with no peer section.
//
// VALIDATES: BGP tree without "peer" key → valid (no peers to configure).
// PREVENTS: Nil pointer or panic when peer section is missing.
func TestReactorVerifyConfigNoPeerSection(t *testing.T) {
	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	r := New(cfg)
	require.NoError(t, r.Start())
	defer r.Stop()

	adapter := &reactorAPIAdapter{r: r}
	bgpTree := map[string]any{
		"router-id": "1.2.3.4",
	}

	err := adapter.VerifyConfig(bgpTree)
	require.NoError(t, err)
}

// TestReactorApplyConfigDiffNoPeerSection verifies ApplyConfigDiff removes all when no peer section.
//
// VALIDATES: BGP tree without "peer" key → all existing peers removed.
// PREVENTS: Missing peer section silently leaving stale peers.
func TestReactorApplyConfigDiffNoPeerSection(t *testing.T) {
	cfg := &Config{ListenAddr: "127.0.0.1:0"}
	r := New(cfg)

	settings := NewPeerSettings(mustParseAddr("10.0.0.1"), 65001, 65002, 0)
	settings.Connection = ConnectionPassive
	_ = r.AddPeer(settings)

	require.NoError(t, r.Start())
	defer r.Stop()
	require.Len(t, r.Peers(), 1)

	adapter := &reactorAPIAdapter{r: r}
	bgpTree := map[string]any{
		"router-id": "1.2.3.4",
	}

	err := adapter.ApplyConfigDiff(bgpTree)
	require.NoError(t, err)

	assert.Empty(t, r.Peers(), "all peers should be removed when no peer section")
}
