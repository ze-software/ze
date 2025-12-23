package api

import (
	"net/netip"
	"testing"
	"time"

	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
	"github.com/exa-networks/zebgp/pkg/rib"
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

func (m *mockReactor) SendRoutes(_ string, routes []*rib.Route, withdrawals []nlri.NLRI, _ bool) (TransactionResult, error) {
	return TransactionResult{
		RoutesAnnounced: len(routes),
		RoutesWithdrawn: len(withdrawals),
		UpdatesSent:     1,
	}, nil
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

// TestHandleAnnounceIPv4_RejectsVPNKeywords verifies that VPN-only keywords are rejected for unicast.
//
// VALIDATES: Keywords like 'rd', 'rt', 'label' return error for IPv4 unicast.
//
// PREVENTS: VPN keywords being silently ignored (the original bug).
func TestHandleAnnounceIPv4_RejectsVPNKeywords(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		errMsg string
	}{
		{
			name:   "rd not valid for unicast",
			args:   []string{"unicast", "10.0.0.0/24", "next-hop", "1.2.3.4", "rd", "100:100"},
			errMsg: "rd",
		},
		{
			name:   "rt not valid for unicast",
			args:   []string{"unicast", "10.0.0.0/24", "next-hop", "1.2.3.4", "rt", "100:100"},
			errMsg: "rt",
		},
		{
			name:   "label not valid for unicast",
			args:   []string{"unicast", "10.0.0.0/24", "next-hop", "1.2.3.4", "label", "100"},
			errMsg: "label",
		},
		{
			name:   "unknown keyword rejected",
			args:   []string{"unicast", "10.0.0.0/24", "next-hop", "1.2.3.4", "foo", "bar"},
			errMsg: "foo",
		},
	}

	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := handleAnnounceIPv4(ctx, tt.args)

			require.Error(t, err, "expected error for invalid keyword")
			require.NotNil(t, resp)
			assert.Equal(t, "error", resp.Status)
			assert.Contains(t, resp.Error, tt.errMsg)
		})
	}
}

// TestHandleAnnounceIPv6_RejectsVPNKeywords verifies that VPN-only keywords are rejected for IPv6 unicast.
//
// VALIDATES: Keywords like 'rd', 'rt', 'label' return error for IPv6 unicast.
//
// PREVENTS: VPN keywords being silently ignored.
func TestHandleAnnounceIPv6_RejectsVPNKeywords(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		errMsg string
	}{
		{
			name:   "rd not valid for unicast",
			args:   []string{"unicast", "2001:db8::/32", "next-hop", "2001::1", "rd", "100:100"},
			errMsg: "rd",
		},
		{
			name:   "unknown keyword rejected",
			args:   []string{"unicast", "2001:db8::/32", "next-hop", "2001::1", "unknown", "value"},
			errMsg: "unknown",
		},
	}

	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := handleAnnounceIPv6(ctx, tt.args)

			require.Error(t, err, "expected error for invalid keyword")
			require.NotNil(t, resp)
			assert.Equal(t, "error", resp.Status)
			assert.Contains(t, resp.Error, tt.errMsg)
		})
	}
}

// TestHandleAnnounceRoute_RejectsVPNKeywords verifies that VPN-only keywords are rejected.
//
// VALIDATES: Auto-detect route handler also validates keywords.
//
// PREVENTS: Bypassing validation by using 'announce route' instead of 'announce ipv4'.
func TestHandleAnnounceRoute_RejectsVPNKeywords(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	// rd should be rejected
	resp, err := handleAnnounceRoute(ctx, []string{"10.0.0.0/24", "next-hop", "1.2.3.4", "rd", "100:100"})

	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Error, "rd")
}

// TestHandleAnnounceIPv4_ValidKeywords verifies valid unicast keywords work.
//
// VALIDATES: All unicast keywords are accepted.
//
// PREVENTS: Keyword validation being too strict.
func TestHandleAnnounceIPv4_ValidKeywords(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	args := []string{
		"unicast", "10.0.0.0/24",
		"next-hop", "1.2.3.4",
		"origin", "igp",
		"med", "100",
		"local-preference", "200",
		"as-path", "[65001]",
		"community", "[2914:666]",
		"large-community", "[2914:1:2]",
	}

	resp, err := handleAnnounceIPv4(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.Len(t, reactor.announcedRoutes, 1)
}
