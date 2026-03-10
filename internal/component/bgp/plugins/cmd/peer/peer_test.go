package peer

import (
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	pluginserver "codeberg.org/thomas-mangin/ze/internal/component/plugin/server"
)

// TestBgpHandlerRPCs verifies BGP handler RPCs are registered via init().
//
// VALIDATES: All BGP handler RPCs are self-registered via pluginserver.RegisterRPCs.
// PREVENTS: Lost handlers during migration to init()-based registration.
func TestBgpHandlerRPCs(t *testing.T) {
	rpcs := pluginserver.AllBuiltinRPCs()

	// Count BGP handler RPCs (ze-bgp:* + ze-plugin:session-peer-ready)
	// RIB meta-commands (ze-rib:*) are now in the server package.
	var bgpCount int
	wireMethodsSeen := make(map[string]bool)
	for _, reg := range rpcs {
		if !strings.HasPrefix(reg.WireMethod, "ze-bgp:") && reg.WireMethod != "ze-plugin:session-peer-ready" {
			continue
		}

		bgpCount++
		assert.NotEmpty(t, reg.CLICommand, "missing CLI command")
		assert.NotNil(t, reg.Handler, "missing handler for %s", reg.WireMethod)
		assert.NotEmpty(t, reg.Help, "missing help for %s", reg.WireMethod)

		assert.False(t, wireMethodsSeen[reg.WireMethod], "duplicate wire method: %s", reg.WireMethod)
		wireMethodsSeen[reg.WireMethod] = true
	}

	// 8 peer ops + 3 summary/caps/stats + 1 session-peer-ready = 12
	assert.Equal(t, 12, bgpCount, "expected 12 BGP handler RPCs")
}

// TestHandlerPeerList verifies handleBgpPeerList returns peer info.
//
// VALIDATES: Peer list handler returns all peers from reactor.
// PREVENTS: Handler unable to access reactor via CommandContext.
func TestHandlerPeerList(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001, State: "established", Uptime: time.Minute},
			{Address: netip.MustParseAddr("192.0.2.2"), PeerAS: 65002, State: "idle"},
		},
	}
	ctx := newTestContext(reactor)

	resp, err := handleBgpPeerList(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "expected map response data")
	peers, ok := data["peers"].(map[string]any)
	require.True(t, ok, "expected peers map indexed by IP")
	assert.Len(t, peers, 2)
	assert.Contains(t, peers, "192.0.2.1")
	assert.Contains(t, peers, "192.0.2.2")
}

// TestHandlerPeerListNilReactor verifies handleBgpPeerList errors without reactor.
//
// VALIDATES: Handler returns error when reactor is nil.
// PREVENTS: Nil pointer dereference when server has no reactor.
func TestHandlerPeerListNilReactor(t *testing.T) {
	ctx := newTestContext(nil)

	_, err := handleBgpPeerList(ctx, nil)
	require.Error(t, err)
}

// TestHandlerPeerSave verifies handleBgpPeerSave writes peer config to file.
//
// VALIDATES: Save handler creates peer entries in config file via Editor.
// PREVENTS: Save handler silently failing or writing incorrect config values.
func TestHandlerPeerSave(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	// Write minimal valid config for the editor to parse (bgp container required)
	initial := "bgp {\n\trouter-id 1.2.3.4;\n\tlocal-as 65000;\n}\n"
	err := os.WriteFile(configPath, []byte(initial), 0o600)
	require.NoError(t, err)

	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address:  netip.MustParseAddr("192.0.2.1"),
				PeerAS:   65001,
				LocalAS:  65000,
				HoldTime: 90 * time.Second, // default — should NOT be saved
			},
		},
		stats: plugin.ReactorStats{LocalAS: 65000, RouterID: 0x01020304},
	}

	ctx := newTestContextWithConfig(reactor, configPath)
	ctx.Peer = "*"

	resp, err := handleBgpPeerSave(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	saved, ok := data["saved"].([]string)
	require.True(t, ok)
	assert.Equal(t, []string{"192.0.2.1"}, saved)

	// Verify the config file was modified
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "192.0.2.1")
	assert.Contains(t, string(content), "peer-as 65001")
	// local-as matches reactor default, so should NOT be written for this peer
	// hold-time is default 90s, so should NOT be written
}

// TestHandlerPeerSaveNonDefaultHoldTime verifies hold-time 0 (RFC 4271) is saved.
//
// VALIDATES: Non-default hold-time (including 0) is persisted to config.
// PREVENTS: RFC 4271 hold-time 0 (no keepalives) being silently dropped.
func TestHandlerPeerSaveNonDefaultHoldTime(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	initial := "bgp {\n\trouter-id 1.2.3.4;\n\tlocal-as 65000;\n}\n"
	err := os.WriteFile(configPath, []byte(initial), 0o600)
	require.NoError(t, err)

	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address:    netip.MustParseAddr("10.0.0.1"),
				PeerAS:     65002,
				LocalAS:    65000,
				HoldTime:   0, // RFC 4271: no keepalives
				Connection: "passive",
			},
		},
		stats: plugin.ReactorStats{LocalAS: 65000, RouterID: 0x01020304},
	}

	ctx := newTestContextWithConfig(reactor, configPath)
	ctx.Peer = "*"

	resp, err := handleBgpPeerSave(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	s := string(content)
	assert.Contains(t, s, "10.0.0.1")
	assert.Contains(t, s, "hold-time 0")
	assert.Contains(t, s, "connection passive")
}

// TestHandlerPeerSaveLocalAddress verifies local-address is saved to config.
//
// VALIDATES: Save handler persists local-address when set on a peer.
// PREVENTS: LocalAddress being silently dropped during config save.
func TestHandlerPeerSaveLocalAddress(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.conf")

	initial := "bgp {\n\trouter-id 1.2.3.4;\n\tlocal-as 65000;\n}\n"
	err := os.WriteFile(configPath, []byte(initial), 0o600)
	require.NoError(t, err)

	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{
				Address:      netip.MustParseAddr("192.0.2.1"),
				PeerAS:       65001,
				LocalAS:      65000,
				LocalAddress: netip.MustParseAddr("192.168.1.1"),
				HoldTime:     90 * time.Second, // default — should NOT be saved
			},
		},
		stats: plugin.ReactorStats{LocalAS: 65000, RouterID: 0x01020304},
	}

	ctx := newTestContextWithConfig(reactor, configPath)
	ctx.Peer = "*"

	resp, err := handleBgpPeerSave(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, plugin.StatusDone, resp.Status)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "local-address 192.168.1.1")
}

// TestDefaultHoldTimeMatchesReactor verifies the local defaultHoldTime constant
// matches the reactor's DefaultHoldTime (90s per RFC 4271 Section 10).
// Source: internal/component/bgp/reactor/peersettings.go
//
// VALIDATES: defaultHoldTime is consistent with reactor.DefaultHoldTime.
// PREVENTS: Silent divergence between command handler and reactor defaults.
func TestDefaultHoldTimeMatchesReactor(t *testing.T) {
	// Cannot import reactor (import cycle via blank import in reactor.go).
	// Verify the value matches the expected RFC 4271 default directly.
	// reactor.DefaultHoldTime is defined in internal/component/bgp/reactor/peersettings.go
	// and is tested by TestDefaultHoldTime in peersettings_test.go.
	assert.Equal(t, 90*time.Second, defaultHoldTime,
		"defaultHoldTime must be 90s to match reactor.DefaultHoldTime (reactor/peersettings.go)")
}

// TestHandlerPeerSaveNoConfigPath verifies save errors when config path is empty.
//
// VALIDATES: Handler returns error when config path is not set.
// PREVENTS: Nil pointer or empty-string file operations.
func TestHandlerPeerSaveNoConfigPath(t *testing.T) {
	reactor := &mockReactor{
		peers: []plugin.PeerInfo{
			{Address: netip.MustParseAddr("192.0.2.1"), PeerAS: 65001},
		},
	}

	ctx := newTestContext(reactor) // no config path
	ctx.Peer = "*"

	_, err := handleBgpPeerSave(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "config path")
}
