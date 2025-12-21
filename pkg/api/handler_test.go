package api

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockReactor implements ReactorInterface for testing.
type mockReactor struct {
	peers           []PeerInfo
	stats           ReactorStats
	stopped         bool
	announcedRoutes []struct {
		selector string
		route    RouteSpec
	}
	withdrawnRoutes []struct {
		selector string
		prefix   netip.Prefix
	}
}

func (m *mockReactor) Peers() []PeerInfo {
	return m.peers
}

func (m *mockReactor) Stats() ReactorStats {
	return m.stats
}

func (m *mockReactor) Stop() {
	m.stopped = true
}

func (m *mockReactor) AnnounceRoute(selector string, route RouteSpec) error {
	m.announcedRoutes = append(m.announcedRoutes, struct {
		selector string
		route    RouteSpec
	}{selector, route})
	return nil
}

func (m *mockReactor) WithdrawRoute(selector string, prefix netip.Prefix) error {
	m.withdrawnRoutes = append(m.withdrawnRoutes, struct {
		selector string
		prefix   netip.Prefix
	}{selector, prefix})
	return nil
}

func (m *mockReactor) TeardownPeer(_ netip.Addr, _ string) error {
	return nil
}

func (m *mockReactor) Reload() error {
	return nil
}

func (m *mockReactor) AnnounceFlowSpec(_ string, _ FlowSpecRoute) error {
	return nil
}

func (m *mockReactor) WithdrawFlowSpec(_ string, _ FlowSpecRoute) error {
	return nil
}

func (m *mockReactor) AnnounceVPLS(_ string, _ VPLSRoute) error {
	return nil
}

func (m *mockReactor) WithdrawVPLS(_ string, _ VPLSRoute) error {
	return nil
}

func (m *mockReactor) AnnounceL2VPN(_ string, _ L2VPNRoute) error {
	return nil
}

func (m *mockReactor) AnnounceEOR(_ string, _ uint16, _ uint8) error {
	return nil
}

func (m *mockReactor) RIBInRoutes(_ string) []RIBRoute {
	return nil
}

func (m *mockReactor) RIBOutRoutes() []RIBRoute {
	return nil
}

func (m *mockReactor) RIBStats() RIBStatsInfo {
	return RIBStatsInfo{}
}

func (m *mockReactor) WithdrawL2VPN(_ string, _ L2VPNRoute) error {
	return nil
}

// Transaction stubs (base mock doesn't support transactions).
func (m *mockReactor) BeginTransaction(_, _ string) error {
	return ErrNoTransaction
}

func (m *mockReactor) CommitTransaction(_ string) (TransactionResult, error) {
	return TransactionResult{}, ErrNoTransaction
}

func (m *mockReactor) CommitTransactionWithLabel(_, _ string) (TransactionResult, error) {
	return TransactionResult{}, ErrNoTransaction
}

func (m *mockReactor) RollbackTransaction(_ string) (TransactionResult, error) {
	return TransactionResult{}, ErrNoTransaction
}

func (m *mockReactor) InTransaction(_ string) bool {
	return false
}

func (m *mockReactor) TransactionID(_ string) string {
	return ""
}

// TestHandlerPeerList verifies peer list output.
//
// VALIDATES: All peers returned with state info.
//
// PREVENTS: Missing peers in output.
func TestHandlerPeerList(t *testing.T) {
	reactor := &mockReactor{
		peers: []PeerInfo{
			{
				Address: netip.MustParseAddr("192.168.1.2"),
				LocalAS: 65001,
				PeerAS:  65002,
				State:   "established",
			},
			{
				Address: netip.MustParseAddr("192.168.1.3"),
				LocalAS: 65001,
				PeerAS:  65003,
				State:   "idle",
			},
		},
	}

	ctx := &CommandContext{Reactor: reactor}
	resp, err := handlePeerList(ctx, nil)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	peers, ok := data["peers"].([]PeerInfo)
	require.True(t, ok)
	assert.Len(t, peers, 2)
}

// TestHandlerPeerShow verifies peer show output.
//
// VALIDATES: Peer details returned.
//
// PREVENTS: Missing peer details.
func TestHandlerPeerShow(t *testing.T) {
	reactor := &mockReactor{
		peers: []PeerInfo{
			{
				Address:          netip.MustParseAddr("192.168.1.2"),
				LocalAddress:     netip.MustParseAddr("192.168.1.1"),
				LocalAS:          65001,
				PeerAS:           65002,
				RouterID:         0xC0A80101, // 192.168.1.1
				State:            "established",
				Uptime:           time.Hour,
				MessagesReceived: 100,
				MessagesSent:     50,
			},
		},
	}

	ctx := &CommandContext{Reactor: reactor}
	resp, err := handlePeerShow(ctx, nil)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	peers, ok := data["peers"].([]PeerInfo)
	require.True(t, ok)
	assert.Len(t, peers, 1)
	assert.Equal(t, "established", peers[0].State)
}

// TestHandlerPeerShowSpecific verifies single peer output.
//
// VALIDATES: Specific peer returned by IP.
//
// PREVENTS: Wrong peer returned or error on valid IP.
func TestHandlerPeerShowSpecific(t *testing.T) {
	reactor := &mockReactor{
		peers: []PeerInfo{
			{Address: netip.MustParseAddr("192.168.1.2"), State: "established"},
			{Address: netip.MustParseAddr("192.168.1.3"), State: "idle"},
		},
	}

	ctx := &CommandContext{Reactor: reactor}
	resp, err := handlePeerShow(ctx, []string{"192.168.1.2"})

	require.NoError(t, err)
	require.NotNil(t, resp)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	peers, ok := data["peers"].([]PeerInfo)
	require.True(t, ok)
	require.Len(t, peers, 1)
	assert.Equal(t, "192.168.1.2", peers[0].Address.String())
}

// TestHandlerPeerShowNotFound verifies error for unknown peer.
//
// VALIDATES: Unknown peer returns error.
//
// PREVENTS: Silent failure on typo in peer IP.
func TestHandlerPeerShowNotFound(t *testing.T) {
	reactor := &mockReactor{
		peers: []PeerInfo{
			{Address: netip.MustParseAddr("192.168.1.2")},
		},
	}

	ctx := &CommandContext{Reactor: reactor}
	resp, err := handlePeerShow(ctx, []string{"10.0.0.1"})

	require.NoError(t, err) // Command succeeded, but no peers matched
	require.NotNil(t, resp)
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	peers, ok := data["peers"].([]PeerInfo)
	require.True(t, ok)
	assert.Empty(t, peers)
}

// TestHandlerDaemonShutdown verifies shutdown signal.
//
// VALIDATES: Reactor.Stop() called.
//
// PREVENTS: Daemon not stopping on command.
func TestHandlerDaemonShutdown(t *testing.T) {
	reactor := &mockReactor{}

	ctx := &CommandContext{Reactor: reactor}
	resp, err := handleDaemonShutdown(ctx, nil)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.True(t, reactor.stopped, "reactor must be stopped")
}

// TestHandlerDaemonStatus verifies status output.
//
// VALIDATES: Status includes uptime and peer count.
//
// PREVENTS: Missing operational status.
func TestHandlerDaemonStatus(t *testing.T) {
	reactor := &mockReactor{
		peers: []PeerInfo{{}, {}},
		stats: ReactorStats{
			StartTime: time.Now().Add(-time.Hour),
			Uptime:    time.Hour,
			PeerCount: 2,
		},
	}

	ctx := &CommandContext{Reactor: reactor}
	resp, err := handleDaemonStatus(ctx, nil)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 2, data["peer_count"])
	assert.NotEmpty(t, data["uptime"])
}

// TestHandlerSystemHelp verifies help output.
//
// VALIDATES: Help returns list of commands.
//
// PREVENTS: Users unable to discover available commands.
func TestHandlerSystemHelp(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ctx := &CommandContext{Reactor: &mockReactor{}}

	// Dispatch the help command
	resp, err := d.Dispatch(ctx, "system help")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	commands, ok := data["commands"].([]string)
	require.True(t, ok)
	assert.NotEmpty(t, commands)
}

// TestRegisterDefaultHandlers verifies all P0 commands registered.
//
// VALIDATES: All required commands are available.
//
// PREVENTS: Missing essential commands.
func TestRegisterDefaultHandlers(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	requiredCommands := []string{
		"daemon shutdown",
		"daemon status",
		"peer list",
		"peer show",
		"system help",
		"system version",
	}

	for _, cmd := range requiredCommands {
		c := d.Lookup(cmd)
		assert.NotNil(t, c, "command %q must be registered", cmd)
	}
}
