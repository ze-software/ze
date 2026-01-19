package plugin

import (
	"errors"
	"net/netip"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/zebgp/internal/bgp/message"
	"codeberg.org/thomas-mangin/zebgp/internal/bgp/nlri"
	"codeberg.org/thomas-mangin/zebgp/internal/rib"
	"codeberg.org/thomas-mangin/zebgp/internal/selector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ErrPeerNotFound is a test error matching reactor.ErrPeerNotFound.
// Cannot import reactor due to import cycle (reactor imports api).
var ErrPeerNotFound = errors.New("peer not found")

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
	announcedL3VPNRoutes []struct {
		selector string
		route    L3VPNRoute
	}
	withdrawnL3VPNRoutes []struct {
		selector string
		route    L3VPNRoute
	}
	announcedLabeledUnicastRoutes []struct {
		selector string
		route    LabeledUnicastRoute
	}
	withdrawnLabeledUnicastRoutes []struct {
		selector string
		route    LabeledUnicastRoute
	}
	teardownCalls []struct {
		addr    netip.Addr
		subcode uint8
	}
	rawMessages []struct {
		addr    netip.Addr
		msgType uint8
		payload []byte
	}
	// RIB operation tracking
	ribInCleared  bool
	ribOutCleared bool
	ribOutFlushed bool

	// NLRI batch tracking for wire mode tests
	announcedBatches []struct {
		selector string
		batch    NLRIBatch
	}
	withdrawnBatches []struct {
		selector string
		batch    NLRIBatch
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

func (m *mockReactor) TeardownPeer(addr netip.Addr, subcode uint8) error {
	m.teardownCalls = append(m.teardownCalls, struct {
		addr    netip.Addr
		subcode uint8
	}{addr, subcode})
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

func (m *mockReactor) AnnounceL3VPN(selector string, route L3VPNRoute) error {
	m.announcedL3VPNRoutes = append(m.announcedL3VPNRoutes, struct {
		selector string
		route    L3VPNRoute
	}{selector, route})
	return nil
}

func (m *mockReactor) WithdrawL3VPN(selector string, route L3VPNRoute) error {
	m.withdrawnL3VPNRoutes = append(m.withdrawnL3VPNRoutes, struct {
		selector string
		route    L3VPNRoute
	}{selector, route})
	return nil
}

func (m *mockReactor) AnnounceLabeledUnicast(selector string, route LabeledUnicastRoute) error {
	m.announcedLabeledUnicastRoutes = append(m.announcedLabeledUnicastRoutes, struct {
		selector string
		route    LabeledUnicastRoute
	}{selector, route})
	return nil
}

func (m *mockReactor) WithdrawLabeledUnicast(selector string, route LabeledUnicastRoute) error {
	m.withdrawnLabeledUnicastRoutes = append(m.withdrawnLabeledUnicastRoutes, struct {
		selector string
		route    LabeledUnicastRoute
	}{selector, route})
	return nil
}

func (m *mockReactor) AnnounceMUPRoute(_ string, _ MUPRouteSpec) error {
	return nil
}

func (m *mockReactor) WithdrawMUPRoute(_ string, _ MUPRouteSpec) error {
	return nil
}

func (m *mockReactor) AnnounceEOR(_ string, _ uint16, _ uint8) error {
	return nil
}

func (m *mockReactor) RIBInRoutes(_ string) []rib.RouteJSON {
	return nil
}

func (m *mockReactor) RIBOutRoutes() []rib.RouteJSON {
	return nil
}

func (m *mockReactor) RIBStats() RIBStatsInfo {
	return RIBStatsInfo{}
}

func (m *mockReactor) ClearRIBIn() int {
	m.ribInCleared = true
	return 5 // Mock: pretend we cleared 5 routes
}

func (m *mockReactor) ClearRIBOut() int {
	m.ribOutCleared = true
	return 3 // Mock: pretend we withdrew 3 routes
}

func (m *mockReactor) FlushRIBOut() int {
	m.ribOutFlushed = true
	return 7 // Mock: pretend we flushed 7 routes
}

func (m *mockReactor) GetPeerProcessBindings(_ netip.Addr) []PeerProcessBinding {
	return nil // Mock: no API bindings configured
}

func (m *mockReactor) GetPeerCapabilityConfigs() []PeerCapabilityConfig {
	return nil // Mock: no capability configs
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

func (m *mockReactor) AnnounceWatchdog(_, _ string) error {
	return nil
}

func (m *mockReactor) WithdrawWatchdog(_, _ string) error {
	return nil
}

func (m *mockReactor) AddWatchdogRoute(_ RouteSpec, _ string) error {
	return nil
}

func (m *mockReactor) RemoveWatchdogRoute(_, _ string) error {
	return nil
}

func (m *mockReactor) ForwardUpdate(_ *selector.Selector, _ uint64) error {
	return nil
}

func (m *mockReactor) DeleteUpdate(_ uint64) error {
	return nil
}

func (m *mockReactor) RetainUpdate(_ uint64) error {
	return nil
}

func (m *mockReactor) ReleaseUpdate(_ uint64) error {
	return nil
}

func (m *mockReactor) ListUpdates() []uint64 {
	return nil
}

func (m *mockReactor) SignalAPIReady() {}

func (m *mockReactor) SignalPeerAPIReady(_ string) {}

func (m *mockReactor) AnnounceNLRIBatch(selector string, batch NLRIBatch) error {
	m.announcedBatches = append(m.announcedBatches, struct {
		selector string
		batch    NLRIBatch
	}{selector, batch})
	return nil
}

func (m *mockReactor) WithdrawNLRIBatch(selector string, batch NLRIBatch) error {
	m.withdrawnBatches = append(m.withdrawnBatches, struct {
		selector string
		batch    NLRIBatch
	}{selector, batch})
	return nil
}

func (m *mockReactor) SendRawMessage(addr netip.Addr, msgType uint8, payload []byte) error {
	m.rawMessages = append(m.rawMessages, struct {
		addr    netip.Addr
		msgType uint8
		payload []byte
	}{addr, msgType, payload})
	return nil
}

func (m *mockReactor) SendBoRR(_ string, _ uint16, _ uint8) error {
	return nil
}

func (m *mockReactor) SendEoRR(_ string, _ uint16, _ uint8) error {
	return nil
}

// mockReactorRawError embeds mockReactor but returns error from SendRawMessage.
type mockReactorRawError struct {
	mockReactor
	err error
}

func (m *mockReactorRawError) SendRawMessage(_ netip.Addr, _ uint8, _ []byte) error {
	return m.err
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

// =============================================================================
// Teardown Command Tests
// =============================================================================

// TestHandleTeardown verifies the teardown command with cease subcode.
//
// VALIDATES: Teardown command calls reactor with correct IP and subcode.
//
// PREVENTS: Wrong subcode being sent in NOTIFICATION.
func TestHandleTeardown(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "127.0.0.1", // Set by dispatcher from "neighbor 127.0.0.1 teardown 4"
	}

	// teardown 4 = Cease subcode 4 (Administrative Reset)
	resp, err := handleTeardown(ctx, []string{"4"})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	require.Len(t, reactor.teardownCalls, 1)
	assert.Equal(t, netip.MustParseAddr("127.0.0.1"), reactor.teardownCalls[0].addr)
	assert.Equal(t, uint8(4), reactor.teardownCalls[0].subcode)
}

// TestHandleTeardown_AdminShutdown verifies subcode 2 (Admin Shutdown).
//
// VALIDATES: Subcode 2 works correctly.
//
// PREVENTS: Wrong subcode for shutdown vs reset.
func TestHandleTeardown_AdminShutdown(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "192.168.1.1",
	}

	resp, err := handleTeardown(ctx, []string{"2"})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	require.Len(t, reactor.teardownCalls, 1)
	assert.Equal(t, uint8(2), reactor.teardownCalls[0].subcode)
}

// TestHandleTeardown_MissingSubcode verifies error on missing subcode.
//
// VALIDATES: Missing subcode returns error.
//
// PREVENTS: Default subcode being used silently.
func TestHandleTeardown_MissingSubcode(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "127.0.0.1",
	}

	resp, err := handleTeardown(ctx, []string{})

	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Data.(string), "usage") //nolint:forcetypeassert // test code
}

// TestHandleTeardown_InvalidSubcode verifies error on non-numeric subcode.
//
// VALIDATES: Non-numeric subcode returns error.
//
// PREVENTS: Parsing errors being silently ignored.
func TestHandleTeardown_InvalidSubcode(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "127.0.0.1",
	}

	resp, err := handleTeardown(ctx, []string{"invalid"})

	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
}

// TestHandleTeardown_MissingPeer verifies error when no peer specified.
//
// VALIDATES: Teardown without peer target returns error.
//
// PREVENTS: Tearing down wrong peer.
func TestHandleTeardown_MissingPeer(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "", // No peer specified
	}

	resp, err := handleTeardown(ctx, []string{"4"})

	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
}

// TestDispatchNeighborTeardown verifies full command parsing.
//
// VALIDATES: "neighbor 127.0.0.1 teardown 4" is correctly dispatched.
//
// PREVENTS: Command not being recognized.
func TestDispatchNeighborTeardown(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	resp, err := d.Dispatch(ctx, "neighbor 127.0.0.1 teardown 4")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	require.Len(t, reactor.teardownCalls, 1)
	assert.Equal(t, netip.MustParseAddr("127.0.0.1"), reactor.teardownCalls[0].addr)
	assert.Equal(t, uint8(4), reactor.teardownCalls[0].subcode)
}

// TestRIBClearIn verifies clearing Adj-RIB-In removes all received routes.
//
// VALIDATES: API command correctly clears incoming route storage.
//
// PREVENTS: Memory leaks from accumulated routes, stale route data.
func TestRIBClearIn(t *testing.T) {
	reactor := &mockReactor{
		ribInCleared: false,
	}

	ctx := &CommandContext{Reactor: reactor}
	resp, err := handleRIBClearIn(ctx, nil)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.True(t, reactor.ribInCleared, "RIB-In should be cleared")

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Contains(t, data, "routes_cleared")
}

// TestRIBCommandsRegistered verifies all RIB commands are registered.
//
// VALIDATES: Commands are discoverable via dispatcher.
//
// PREVENTS: Missing commands causing API failures.
func TestRIBCommandsRegistered(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	// Note: rib show/clear/flush out removed - Adj-RIB-Out tracking delegated to external API
	ribCommands := []string{
		"rib show in",
		"rib clear in",
	}

	for _, cmd := range ribCommands {
		c := d.Lookup(cmd)
		assert.NotNil(t, c, "command %q must be registered", cmd)
	}
}

// TestWantsMessageType verifies message type filtering.
//
// VALIDATES: Only subscribed message types are forwarded to processes.
//
// PREVENTS: Processes receiving unwanted messages.
func TestWantsMessageType(t *testing.T) {
	tests := []struct {
		name    string
		binding PeerProcessBinding
		msgType message.MessageType
		want    bool
	}{
		{
			name:    "update subscribed",
			binding: PeerProcessBinding{ReceiveUpdate: true},
			msgType: message.TypeUPDATE,
			want:    true,
		},
		{
			name:    "update not subscribed",
			binding: PeerProcessBinding{ReceiveUpdate: false},
			msgType: message.TypeUPDATE,
			want:    false,
		},
		{
			name:    "open subscribed",
			binding: PeerProcessBinding{ReceiveOpen: true},
			msgType: message.TypeOPEN,
			want:    true,
		},
		{
			name:    "notification subscribed",
			binding: PeerProcessBinding{ReceiveNotification: true},
			msgType: message.TypeNOTIFICATION,
			want:    true,
		},
		{
			name:    "keepalive subscribed",
			binding: PeerProcessBinding{ReceiveKeepalive: true},
			msgType: message.TypeKEEPALIVE,
			want:    true,
		},
		{
			name:    "mixed flags - only update",
			binding: PeerProcessBinding{ReceiveUpdate: true, ReceiveOpen: false},
			msgType: message.TypeOPEN,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wantsMessageType(tt.binding, tt.msgType)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// Raw Passthrough Tests
// =============================================================================

// TestHandleRaw_UpdateHex verifies raw UPDATE with hex encoding.
//
// VALIDATES: "peer X raw update hex <data>" sends UPDATE payload.
//
// PREVENTS: Raw bytes not being sent to peer.
func TestHandleRaw_UpdateHex(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "10.0.0.1",
	}

	// raw update hex 0000000e40010100400200400304c0a80101180a00
	args := []string{"update", "hex", "0000000e40010100400200400304c0a80101180a00"}
	resp, err := handleRaw(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	require.Len(t, reactor.rawMessages, 1)
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), reactor.rawMessages[0].addr)
	assert.Equal(t, uint8(2), reactor.rawMessages[0].msgType) // UPDATE = 2
	assert.NotEmpty(t, reactor.rawMessages[0].payload)
}

// TestHandleRaw_NotificationHex verifies raw NOTIFICATION.
//
// VALIDATES: "peer X raw notification hex 0602" sends NOTIFICATION.
//
// PREVENTS: Only UPDATE being supported.
func TestHandleRaw_NotificationHex(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "10.0.0.1",
	}

	// raw notification hex 0602 (Cease/Admin Shutdown)
	args := []string{"notification", "hex", "0602"}
	resp, err := handleRaw(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	require.Len(t, reactor.rawMessages, 1)
	assert.Equal(t, uint8(3), reactor.rawMessages[0].msgType) // NOTIFICATION = 3
	assert.Equal(t, []byte{0x06, 0x02}, reactor.rawMessages[0].payload)
}

// TestHandleRaw_KeepaliveEmpty verifies raw KEEPALIVE with empty payload.
//
// VALIDATES: "peer X raw keepalive hex" sends empty KEEPALIVE.
//
// PREVENTS: Empty payload being rejected.
func TestHandleRaw_KeepaliveEmpty(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "10.0.0.1",
	}

	// raw keepalive hex (empty payload is valid)
	args := []string{"keepalive", "hex", ""}
	resp, err := handleRaw(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	require.Len(t, reactor.rawMessages, 1)
	assert.Equal(t, uint8(4), reactor.rawMessages[0].msgType) // KEEPALIVE = 4
	assert.Empty(t, reactor.rawMessages[0].payload)
}

// TestHandleRaw_FullPacketHex verifies full packet mode (no type).
//
// VALIDATES: "peer X raw hex <marker+header+body>" sends full packet.
//
// PREVENTS: Full packet mode not working.
func TestHandleRaw_FullPacketHex(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "10.0.0.1",
	}

	// raw hex ffffffffffffffffffffffffffffffff001304 (full KEEPALIVE packet)
	// 16 bytes marker + 2 bytes length (19) + 1 byte type (4)
	args := []string{"hex", "ffffffffffffffffffffffffffffffff001304"}
	resp, err := handleRaw(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	require.Len(t, reactor.rawMessages, 1)
	assert.Equal(t, uint8(0), reactor.rawMessages[0].msgType) // 0 = full packet
	assert.Len(t, reactor.rawMessages[0].payload, 19)         // Full BGP header
}

// TestHandleRaw_B64Encoding verifies base64 encoding.
//
// VALIDATES: "peer X raw notification b64 BgI=" decodes correctly.
//
// PREVENTS: Base64 encoding not working.
func TestHandleRaw_B64Encoding(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "10.0.0.1",
	}

	// raw notification b64 BgI= (0x0602 in base64)
	args := []string{"notification", "b64", "BgI="}
	resp, err := handleRaw(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	require.Len(t, reactor.rawMessages, 1)
	assert.Equal(t, []byte{0x06, 0x02}, reactor.rawMessages[0].payload)
}

// TestHandleRaw_InvalidHex verifies invalid hex is rejected.
//
// VALIDATES: Invalid hex string returns error.
//
// PREVENTS: Malformed data being sent.
func TestHandleRaw_InvalidHex(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "10.0.0.1",
	}

	args := []string{"update", "hex", "not-valid-hex"}
	resp, err := handleRaw(ctx, args)

	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
}

// TestHandleRaw_InvalidB64 verifies invalid base64 is rejected.
//
// VALIDATES: Invalid base64 string returns error.
//
// PREVENTS: Malformed data being sent.
func TestHandleRaw_InvalidB64(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "10.0.0.1",
	}

	args := []string{"update", "b64", "not-valid-base64!!!"}
	resp, err := handleRaw(ctx, args)

	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
}

// TestHandleRaw_MissingPeer verifies error when no peer specified.
//
// VALIDATES: Raw without peer target returns error.
//
// PREVENTS: Sending to wrong peer.
func TestHandleRaw_MissingPeer(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "", // No peer
	}

	args := []string{"update", "hex", "0000"}
	resp, err := handleRaw(ctx, args)

	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
}

// TestHandleRaw_InvalidMsgType verifies unknown message type is rejected.
//
// VALIDATES: Unknown type like "foo" returns error.
//
// PREVENTS: Typos being silently accepted.
func TestHandleRaw_InvalidMsgType(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "10.0.0.1",
	}

	args := []string{"foo", "hex", "0000"}
	resp, err := handleRaw(ctx, args)

	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
}

// TestHandleRaw_AllMessageTypes verifies all BGP message types work.
//
// VALIDATES: open, update, notification, keepalive, route-refresh all work.
//
// PREVENTS: Missing message type support.
func TestHandleRaw_AllMessageTypes(t *testing.T) {
	tests := []struct {
		name    string
		msgType string
		wantTyp uint8
	}{
		{"open", "open", 1},
		{"update", "update", 2},
		{"notification", "notification", 3},
		{"keepalive", "keepalive", 4},
		{"route-refresh", "route-refresh", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reactor := &mockReactor{}
			ctx := &CommandContext{
				Reactor: reactor,
				Peer:    "10.0.0.1",
			}

			args := []string{tt.msgType, "hex", "00"}
			resp, err := handleRaw(ctx, args)

			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, "done", resp.Status)
			require.Len(t, reactor.rawMessages, 1)
			assert.Equal(t, tt.wantTyp, reactor.rawMessages[0].msgType)
		})
	}
}

// TestHandleRaw_MissingEncoding verifies error when type given but no encoding.
//
// VALIDATES: "peer X raw keepalive" (no encoding) returns error.
//
// PREVENTS: Confusing behavior with partial arguments.
func TestHandleRaw_MissingEncoding(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "10.0.0.1",
	}

	// Only type, no encoding
	args := []string{"keepalive"}
	resp, err := handleRaw(ctx, args)

	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
}

// TestHandleRaw_PeerNotFound verifies error when peer doesn't exist.
//
// VALIDATES: Unknown peer returns error from reactor.
//
// PREVENTS: Silent failure when peer not configured.
func TestHandleRaw_PeerNotFound(t *testing.T) {
	reactor := &mockReactorRawError{err: ErrPeerNotFound}
	ctx := &CommandContext{
		Reactor: reactor,
		Peer:    "10.0.0.99", // Non-existent peer
	}

	args := []string{"update", "hex", "0000"}
	resp, err := handleRaw(ctx, args)

	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Data.(string), "send error") //nolint:forcetypeassert // test code
}

// TestHandleRawCommandRegistered verifies raw command is registered.
//
// VALIDATES: "raw" command is accessible via dispatcher.
//
// PREVENTS: Command not being wired up.
func TestHandleRawCommandRegistered(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	c := d.Lookup("raw")
	assert.NotNil(t, c, "raw command must be registered")
}
