package plugin

import (
	"errors"
	"net/netip"
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/rib"
	"codeberg.org/thomas-mangin/ze/internal/selector"
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

// TestHandlerBgpPeerList verifies peer list output.
//
// VALIDATES: All peers returned with state info.
//
// PREVENTS: Missing peers in output.
func TestHandlerBgpPeerList(t *testing.T) {
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

	ctx := &CommandContext{Reactor: reactor, Peer: "*"}
	resp, err := handleBgpPeerList(ctx, nil)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	peers, ok := data["peers"].([]PeerInfo)
	require.True(t, ok)
	assert.Len(t, peers, 2)
}

// TestHandlerBgpPeerShowAll verifies peer show output for all peers.
//
// VALIDATES: Peer details returned.
//
// PREVENTS: Missing peer details.
func TestHandlerBgpPeerShowAll(t *testing.T) {
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

	ctx := &CommandContext{Reactor: reactor, Peer: "*"}
	resp, err := handleBgpPeerShow(ctx, nil)

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

// TestHandlerPeerShowSpecific verifies single peer output via bgp peer <sel> show.
//
// VALIDATES: Specific peer returned by IP via ctx.Peer.
//
// PREVENTS: Wrong peer returned or error on valid IP.
func TestHandlerPeerShowSpecific(t *testing.T) {
	reactor := &mockReactor{
		peers: []PeerInfo{
			{Address: netip.MustParseAddr("192.168.1.2"), State: "established"},
			{Address: netip.MustParseAddr("192.168.1.3"), State: "idle"},
		},
	}

	// Step 5: Now use handleBgpPeerShow with ctx.Peer set by dispatcher
	ctx := &CommandContext{Reactor: reactor, Peer: "192.168.1.2"}
	resp, err := handleBgpPeerShow(ctx, nil)

	require.NoError(t, err)
	require.NotNil(t, resp)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	peers, ok := data["peers"].([]PeerInfo)
	require.True(t, ok)
	require.Len(t, peers, 1)
	assert.Equal(t, "192.168.1.2", peers[0].Address.String())
}

// TestHandlerPeerShowNotFound verifies unknown peer returns empty list.
//
// VALIDATES: Unknown peer returns empty result, not error.
//
// PREVENTS: Silent failure on typo in peer IP.
func TestHandlerPeerShowNotFound(t *testing.T) {
	reactor := &mockReactor{
		peers: []PeerInfo{
			{Address: netip.MustParseAddr("192.168.1.2")},
		},
	}

	// Step 5: Now use handleBgpPeerShow with ctx.Peer set by dispatcher
	ctx := &CommandContext{Reactor: reactor, Peer: "10.0.0.1"}
	resp, err := handleBgpPeerShow(ctx, nil)

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

	// Step 5: Commands moved to bgp namespace
	requiredCommands := []string{
		"bgp daemon shutdown",
		"bgp daemon status",
		"bgp peer list",
		"bgp peer show",
		"system help",
		"system version software",
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

// TestDispatchBgpPeerTeardownFull verifies full command parsing.
//
// VALIDATES: "bgp peer 127.0.0.1 teardown 4" is correctly dispatched.
//
// PREVENTS: Command not being recognized.
func TestDispatchBgpPeerTeardownFull(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	// Step 5: Now uses bgp peer prefix
	resp, err := d.Dispatch(ctx, "bgp peer 127.0.0.1 teardown 4")

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
// VALIDATES: "bgp peer raw" command is accessible via dispatcher.
//
// PREVENTS: Command not being wired up.
func TestHandleRawCommandRegistered(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	// Step 5: raw moved to bgp peer namespace
	c := d.Lookup("bgp peer raw")
	assert.NotNil(t, c, "bgp peer raw command must be registered")
}

// =============================================================================
// Plugin Namespace Tests
// =============================================================================

// TestDispatchPluginSessionReady verifies plugin session ready command.
//
// VALIDATES: "plugin session ready" dispatches correctly and signals API ready.
//
// PREVENTS: Plugin startup signal not reaching reactor.
func TestDispatchPluginSessionReady(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	resp, err := d.Dispatch(ctx, "plugin session ready")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
}

// TestDispatchPluginSessionPing verifies plugin session ping returns PID.
//
// VALIDATES: "plugin session ping" returns pong with daemon PID.
//
// PREVENTS: Health check endpoint not working.
func TestDispatchPluginSessionPing(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	resp, err := d.Dispatch(ctx, "plugin session ping")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Contains(t, data, "pong", "response should contain pong field with PID")
}

// TestDispatchPluginSessionBye verifies plugin session bye acknowledges.
//
// VALIDATES: "plugin session bye" returns success for disconnect.
//
// PREVENTS: Plugin disconnect not being acknowledged.
func TestDispatchPluginSessionBye(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	resp, err := d.Dispatch(ctx, "plugin session bye")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "goodbye", data["status"])
}

// TestDispatchPluginHelp verifies plugin help lists subcommands.
//
// VALIDATES: "plugin help" returns list of plugin subcommands.
//
// PREVENTS: Plugin introspection not working.
func TestDispatchPluginHelp(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor, Dispatcher: d}

	resp, err := d.Dispatch(ctx, "plugin help")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	subcommands, ok := data["subcommands"].([]string)
	require.True(t, ok)
	assert.Contains(t, subcommands, "session")
	assert.Contains(t, subcommands, "command")
}

// TestDispatchPluginCommandList verifies plugin command list returns commands.
//
// VALIDATES: "plugin command list" returns plugin-registered commands.
//
// PREVENTS: Plugin command discovery not working.
func TestDispatchPluginCommandList(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor, Dispatcher: d}

	resp, err := d.Dispatch(ctx, "plugin command list")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Contains(t, data, "commands")
}

// TestOldSessionCommandsRemoved verifies old session commands are removed.
//
// VALIDATES: "session ping", "session bye", "session api ready" return unknown command.
//
// PREVENTS: Old command paths still working after migration.
func TestOldSessionCommandsRemoved(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	oldCommands := []string{
		"session ping",
		"session bye",
		"session api ready",
		"session reset",
	}

	for _, cmd := range oldCommands {
		t.Run(cmd, func(t *testing.T) {
			c := d.Lookup(cmd)
			assert.Nil(t, c, "command %q should NOT be registered (moved to plugin namespace)", cmd)
		})
	}
}

// TestPluginCommandsRegistered verifies new plugin commands are registered.
//
// VALIDATES: All plugin namespace commands are accessible.
//
// PREVENTS: Plugin commands not wired up.
func TestPluginCommandsRegistered(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	pluginCommands := []string{
		"plugin session ready",
		"plugin session ping",
		"plugin session bye",
		"plugin help",
		"plugin command list",
		"plugin command help",
		"plugin command complete",
	}

	for _, cmd := range pluginCommands {
		t.Run(cmd, func(t *testing.T) {
			c := d.Lookup(cmd)
			assert.NotNil(t, c, "command %q must be registered", cmd)
		})
	}
}

// TestBgpPluginCommandsReplaceSession verifies bgp plugin commands replaced session commands.
//
// VALIDATES: "bgp plugin ack/encoding/format" exist (replaced session sync/encoding).
//
// PREVENTS: Missing replacements for moved session commands.
func TestBgpPluginCommandsReplaceSession(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	// New bgp plugin commands that replace old session commands
	newCommands := []string{
		"bgp plugin ack",      // replaces session sync enable/disable
		"bgp plugin encoding", // replaces session api encoding
		"bgp plugin format",   // new command
	}

	for _, cmd := range newCommands {
		t.Run(cmd, func(t *testing.T) {
			c := d.Lookup(cmd)
			assert.NotNil(t, c, "command %q must be registered", cmd)
		})
	}
}

// =============================================================================
// Step 3: System Namespace Tests
// =============================================================================

// TestDispatchSystemVersionSoftware verifies system version software returns version.
//
// VALIDATES: "system version software" returns ZeBGP version string.
// PREVENTS: Missing version info after command rename.
func TestDispatchSystemVersionSoftware(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ctx := &CommandContext{
		Reactor:    &mockReactor{},
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "system version software")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "data should be a map")
	assert.Equal(t, Version, data["version"], "should return software version")
}

// TestDispatchSystemVersionAPI verifies system version api returns protocol version.
//
// VALIDATES: "system version api" returns IPC protocol version.
// PREVENTS: API version unavailable to clients.
func TestDispatchSystemVersionAPI(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ctx := &CommandContext{
		Reactor:    &mockReactor{},
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "system version api")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "data should be a map")
	assert.Equal(t, APIVersion, data["version"], "should return API version")
}

// TestDispatchSystemShutdown verifies system shutdown triggers reactor stop.
//
// VALIDATES: "system shutdown" calls reactor.Stop().
// PREVENTS: Application-level shutdown broken.
func TestDispatchSystemShutdown(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	mock := &mockReactor{}
	ctx := &CommandContext{
		Reactor:    mock,
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "system shutdown")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.True(t, mock.stopped, "reactor.Stop() should have been called")
}

// TestDispatchSystemSubsystemList verifies system subsystem list returns subsystems.
//
// VALIDATES: "system subsystem list" returns available subsystems.
// PREVENTS: Subsystem discovery broken.
func TestDispatchSystemSubsystemList(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ctx := &CommandContext{
		Reactor:    &mockReactor{},
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "system subsystem list")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "data should be a map")
	subsystems, ok := data["subsystems"].([]string)
	require.True(t, ok, "subsystems should be a string slice")
	assert.Contains(t, subsystems, "bgp", "should include bgp subsystem")
}

// TestOldSystemVersionRemoved verifies bare "system version" fails.
//
// VALIDATES: "system version" alone returns unknown command error.
// PREVENTS: Accidental backward compatibility.
func TestOldSystemVersionRemoved(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	// "system version" should not be registered as a command
	cmd := d.Lookup("system version")
	assert.Nil(t, cmd, "bare 'system version' should not be a registered command")

	// But "system version software" should exist
	cmd = d.Lookup("system version software")
	assert.NotNil(t, cmd, "'system version software' should be registered")
}

// TestSystemCommandsRegistered verifies all system commands are registered.
//
// VALIDATES: System namespace commands exist.
// PREVENTS: Missing command registrations.
func TestSystemCommandsRegistered(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	systemCommands := []string{
		"system help",
		"system version software",
		"system version api",
		"system shutdown",
		"system subsystem list",
		"system command list",
		"system command help",
		"system command complete",
	}

	for _, cmd := range systemCommands {
		t.Run(cmd, func(t *testing.T) {
			c := d.Lookup(cmd)
			assert.NotNil(t, c, "command %q must be registered", cmd)
		})
	}
}

// =============================================================================
// Step 4: BGP Namespace Foundation Tests
// =============================================================================

// TestDispatchBgpHelp verifies bgp help returns subcommands.
//
// VALIDATES: "bgp help" returns list of bgp subcommands.
// PREVENTS: BGP namespace introspection broken.
func TestDispatchBgpHelp(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ctx := &CommandContext{
		Reactor:    &mockReactor{},
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp help")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "data should be a map")
	_, hasCommands := data["commands"]
	assert.True(t, hasCommands, "should have commands list")
}

// TestDispatchBgpCommandList verifies bgp command list returns bgp commands.
//
// VALIDATES: "bgp command list" returns commands in bgp namespace.
// PREVENTS: BGP command listing broken.
func TestDispatchBgpCommandList(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ctx := &CommandContext{
		Reactor:    &mockReactor{},
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp command list")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "data should be a map")
	_, hasCommands := data["commands"]
	assert.True(t, hasCommands, "should have commands list")
}

// TestDispatchBgpEventList verifies bgp event list returns event types.
//
// VALIDATES: "bgp event list" returns available BGP event types.
// PREVENTS: Event type discovery broken.
func TestDispatchBgpEventList(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ctx := &CommandContext{
		Reactor:    &mockReactor{},
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp event list")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "data should be a map")
	events, ok := data["events"].([]string)
	require.True(t, ok, "events should be string slice")
	assert.Contains(t, events, "update", "should include update event")
	assert.Contains(t, events, "state", "should include state event")
}

// TestDispatchBgpPluginEncoding verifies bgp plugin encoding sets encoding.
//
// VALIDATES: "bgp plugin encoding json" sets encoding mode.
// PREVENTS: Encoding configuration broken.
func TestDispatchBgpPluginEncoding(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ctx := &CommandContext{
		Reactor:    &mockReactor{},
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp plugin encoding json")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "data should be a map")
	assert.Equal(t, "json", data["encoding"])
}

// TestDispatchBgpPluginFormat verifies bgp plugin format sets format.
//
// VALIDATES: "bgp plugin format full" sets format mode.
// PREVENTS: Format configuration broken.
func TestDispatchBgpPluginFormat(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ctx := &CommandContext{
		Reactor:    &mockReactor{},
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp plugin format full")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "data should be a map")
	assert.Equal(t, "full", data["format"])
}

// TestDispatchBgpPluginAck verifies bgp plugin ack sets ack mode.
//
// VALIDATES: "bgp plugin ack sync" sets sync mode.
// PREVENTS: ACK timing configuration broken.
func TestDispatchBgpPluginAck(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	mock := &mockReactor{}
	ctx := &CommandContext{
		Reactor:    mock,
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp plugin ack sync")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "data should be a map")
	assert.Equal(t, "sync", data["ack"])
}

// TestOldSessionSyncRemoved verifies session sync commands are removed.
//
// VALIDATES: "session sync enable/disable" and "session api encoding" return unknown.
// PREVENTS: Old paths accidentally still working.
func TestOldSessionSyncRemoved(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	oldCommands := []string{
		"session sync enable",
		"session sync disable",
		"session api encoding",
	}

	for _, cmd := range oldCommands {
		t.Run(cmd, func(t *testing.T) {
			c := d.Lookup(cmd)
			assert.Nil(t, c, "old command %q should not be registered", cmd)
		})
	}
}

// TestBgpCommandsRegistered verifies all bgp commands are registered.
//
// VALIDATES: BGP namespace commands exist.
// PREVENTS: Missing command registrations.
func TestBgpCommandsRegistered(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	bgpCommands := []string{
		"bgp help",
		"bgp command list",
		"bgp command help",
		"bgp command complete",
		"bgp event list",
		"bgp plugin encoding",
		"bgp plugin format",
		"bgp plugin ack",
	}

	for _, cmd := range bgpCommands {
		t.Run(cmd, func(t *testing.T) {
			c := d.Lookup(cmd)
			assert.NotNil(t, c, "command %q must be registered", cmd)
		})
	}
}

// TestBgpPluginEncodingAllValues verifies all encoding values work.
//
// VALIDATES: Both json and text encodings are accepted.
// PREVENTS: Valid encoding values being rejected.
func TestBgpPluginEncodingAllValues(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	proc := NewProcess(PluginConfig{Name: "test", Run: "echo"})
	ctx := &CommandContext{
		Reactor:    &mockReactor{},
		Dispatcher: d,
		Process:    proc,
	}

	tests := []struct {
		value string
		want  string
	}{
		{"json", "json"},
		{"text", "text"},
		{"JSON", "json"}, // case insensitive
		{"TEXT", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			resp, err := d.Dispatch(ctx, "bgp plugin encoding "+tt.value)
			require.NoError(t, err)
			assert.Equal(t, "done", resp.Status)

			data, ok := resp.Data.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tt.want, data["encoding"])
			assert.Equal(t, tt.want, proc.Encoding())
		})
	}
}

// TestBgpPluginEncodingInvalid verifies invalid encoding is rejected.
//
// VALIDATES: Invalid encoding returns error.
// PREVENTS: Accepting unknown encoding formats.
func TestBgpPluginEncodingInvalid(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ctx := &CommandContext{
		Reactor:    &mockReactor{},
		Dispatcher: d,
	}

	// Invalid value
	resp, err := d.Dispatch(ctx, "bgp plugin encoding invalid")
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "invalid encoding")

	// Missing argument
	resp, err = d.Dispatch(ctx, "bgp plugin encoding")
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "missing encoding")
}

// TestBgpPluginFormatAllValues verifies all format values work.
//
// VALIDATES: All format values (hex, base64, parsed, full) are accepted.
// PREVENTS: Valid format values being rejected.
func TestBgpPluginFormatAllValues(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	proc := NewProcess(PluginConfig{Name: "test", Run: "echo"})
	ctx := &CommandContext{
		Reactor:    &mockReactor{},
		Dispatcher: d,
		Process:    proc,
	}

	tests := []struct {
		value string
		want  string
	}{
		{"hex", "hex"},
		{"base64", "base64"},
		{"parsed", "parsed"},
		{"full", "full"},
		{"HEX", "hex"}, // case insensitive
		{"PARSED", "parsed"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			resp, err := d.Dispatch(ctx, "bgp plugin format "+tt.value)
			require.NoError(t, err)
			assert.Equal(t, "done", resp.Status)

			data, ok := resp.Data.(map[string]any)
			require.True(t, ok)
			assert.Equal(t, tt.want, data["format"])
			assert.Equal(t, tt.want, proc.Format())
		})
	}
}

// TestBgpPluginFormatInvalid verifies invalid format is rejected.
//
// VALIDATES: Invalid format returns error.
// PREVENTS: Accepting unknown format values.
func TestBgpPluginFormatInvalid(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ctx := &CommandContext{
		Reactor:    &mockReactor{},
		Dispatcher: d,
	}

	// Invalid value
	resp, err := d.Dispatch(ctx, "bgp plugin format invalid")
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "invalid format")

	// Missing argument
	resp, err = d.Dispatch(ctx, "bgp plugin format")
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "missing format")
}

// TestBgpPluginAckAllValues verifies both ack values work.
//
// VALIDATES: Both sync and async modes are accepted and set Process state.
// PREVENTS: Valid ack modes being rejected.
func TestBgpPluginAckAllValues(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	proc := NewProcess(PluginConfig{Name: "test", Run: "echo"})
	ctx := &CommandContext{
		Reactor:    &mockReactor{},
		Dispatcher: d,
		Process:    proc,
	}

	// Test sync
	resp, err := d.Dispatch(ctx, "bgp plugin ack sync")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.True(t, proc.SyncEnabled(), "sync should enable SyncEnabled")

	// Test async
	resp, err = d.Dispatch(ctx, "bgp plugin ack async")
	require.NoError(t, err)
	assert.Equal(t, "done", resp.Status)
	assert.False(t, proc.SyncEnabled(), "async should disable SyncEnabled")

	// Test case insensitive
	_, err = d.Dispatch(ctx, "bgp plugin ack SYNC")
	require.NoError(t, err)
	assert.True(t, proc.SyncEnabled())
}

// TestBgpPluginAckInvalid verifies invalid ack mode is rejected.
//
// VALIDATES: Invalid ack mode returns error.
// PREVENTS: Accepting unknown ack modes.
func TestBgpPluginAckInvalid(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ctx := &CommandContext{
		Reactor:    &mockReactor{},
		Dispatcher: d,
	}

	// Invalid value
	resp, err := d.Dispatch(ctx, "bgp plugin ack invalid")
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "invalid mode")

	// Missing argument
	resp, err = d.Dispatch(ctx, "bgp plugin ack")
	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "missing mode")
}

// =============================================================================
// Step 5: BGP Command Migration Tests
// =============================================================================

// TestDispatchBgpDaemonShutdown verifies bgp daemon shutdown triggers reactor stop.
//
// VALIDATES: "bgp daemon shutdown" calls reactor.Stop().
// PREVENTS: Daemon shutdown command not working.
func TestDispatchBgpDaemonShutdown(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	mock := &mockReactor{}
	ctx := &CommandContext{
		Reactor:    mock,
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp daemon shutdown")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.True(t, mock.stopped, "reactor.Stop() should have been called")
}

// TestDispatchBgpDaemonStatus verifies bgp daemon status returns uptime and peer count.
//
// VALIDATES: "bgp daemon status" returns operational stats.
// PREVENTS: Status information not accessible.
func TestDispatchBgpDaemonStatus(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	mock := &mockReactor{
		peers: []PeerInfo{{}, {}},
		stats: ReactorStats{
			StartTime: time.Now().Add(-time.Hour),
			Uptime:    time.Hour,
			PeerCount: 2,
		},
	}
	ctx := &CommandContext{
		Reactor:    mock,
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp daemon status")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 2, data["peer_count"])
}

// TestDispatchBgpDaemonReload verifies bgp daemon reload triggers config reload.
//
// VALIDATES: "bgp daemon reload" calls reactor.Reload().
// PREVENTS: Config reload command not working.
func TestDispatchBgpDaemonReload(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	mock := &mockReactor{}
	ctx := &CommandContext{
		Reactor:    mock,
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp daemon reload")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
}

// TestDispatchBgpPeerList verifies bgp peer * list returns all peers.
//
// VALIDATES: "bgp peer * list" returns peer list.
// PREVENTS: Peer listing not working.
func TestDispatchBgpPeerList(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	mock := &mockReactor{
		peers: []PeerInfo{
			{Address: netip.MustParseAddr("192.168.1.2"), State: "established"},
			{Address: netip.MustParseAddr("192.168.1.3"), State: "idle"},
		},
	}
	ctx := &CommandContext{
		Reactor:    mock,
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp peer * list")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	peers, ok := data["peers"].([]PeerInfo)
	require.True(t, ok)
	assert.Len(t, peers, 2)
}

// TestDispatchBgpPeerShowAll verifies bgp peer * show returns all peers with details.
//
// VALIDATES: "bgp peer * show" returns peer details.
// PREVENTS: Peer show not working.
func TestDispatchBgpPeerShowAll(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	mock := &mockReactor{
		peers: []PeerInfo{
			{Address: netip.MustParseAddr("192.168.1.2"), State: "established", PeerAS: 65002},
		},
	}
	ctx := &CommandContext{
		Reactor:    mock,
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp peer * show")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	peers, ok := data["peers"].([]PeerInfo)
	require.True(t, ok)
	assert.Len(t, peers, 1)
}

// TestDispatchBgpPeerShow verifies bgp peer <ip> show returns specific peer.
//
// VALIDATES: "bgp peer 192.168.1.2 show" returns specific peer.
// PREVENTS: Peer-specific show not working.
func TestDispatchBgpPeerShow(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	mock := &mockReactor{
		peers: []PeerInfo{
			{Address: netip.MustParseAddr("192.168.1.2"), State: "established"},
			{Address: netip.MustParseAddr("192.168.1.3"), State: "idle"},
		},
	}
	ctx := &CommandContext{
		Reactor:    mock,
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp peer 192.168.1.2 show")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	peers, ok := data["peers"].([]PeerInfo)
	require.True(t, ok)
	require.Len(t, peers, 1)
	assert.Equal(t, "192.168.1.2", peers[0].Address.String())
}

// TestDispatchBgpPeerTeardown verifies bgp peer <ip> teardown sends notification.
//
// VALIDATES: "bgp peer 127.0.0.1 teardown 4" tears down peer.
// PREVENTS: Peer teardown not working.
func TestDispatchBgpPeerTeardown(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	mock := &mockReactor{}
	ctx := &CommandContext{
		Reactor:    mock,
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp peer 127.0.0.1 teardown 4")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	require.Len(t, mock.teardownCalls, 1)
	assert.Equal(t, netip.MustParseAddr("127.0.0.1"), mock.teardownCalls[0].addr)
	assert.Equal(t, uint8(4), mock.teardownCalls[0].subcode)
}

// TestDispatchBgpCommit verifies bgp commit commands work.
//
// VALIDATES: "bgp commit list" returns commit list.
// PREVENTS: Commit commands not working.
func TestDispatchBgpCommit(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	mock := &mockReactor{}
	cm := NewCommitManager()
	ctx := &CommandContext{
		Reactor:       mock,
		Dispatcher:    d,
		CommitManager: cm,
	}

	resp, err := d.Dispatch(ctx, "bgp commit list")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
}

// TestDispatchBgpWatchdog verifies bgp watchdog commands work.
//
// VALIDATES: "bgp watchdog announce test" works.
// PREVENTS: Watchdog commands not working.
func TestDispatchBgpWatchdog(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	mock := &mockReactor{}
	ctx := &CommandContext{
		Reactor:    mock,
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp watchdog announce test")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
}

// TestDispatchBgpPeerUpdate verifies bgp peer <sel> update works.
//
// VALIDATES: "bgp peer * update text ..." announces routes.
// PREVENTS: Update commands not working.
func TestDispatchBgpPeerUpdate(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	mock := &mockReactor{}
	ctx := &CommandContext{
		Reactor:    mock,
		Dispatcher: d,
	}

	// Simple update with family and prefix
	resp, err := d.Dispatch(ctx, "bgp peer * update text origin set igp nhop set 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
}

// TestDispatchBgpPeerBorr verifies bgp peer <sel> borr works.
//
// VALIDATES: "bgp peer * borr ipv4/unicast" sends BoRR.
// PREVENTS: Route refresh commands not working.
func TestDispatchBgpPeerBorr(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	mock := &mockReactor{}
	ctx := &CommandContext{
		Reactor:    mock,
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp peer * borr ipv4/unicast")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
}

// TestDispatchBgpPeerEorr verifies bgp peer <sel> eorr works.
//
// VALIDATES: "bgp peer * eorr ipv4/unicast" sends EoRR.
// PREVENTS: Route refresh commands not working.
func TestDispatchBgpPeerEorr(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	mock := &mockReactor{}
	ctx := &CommandContext{
		Reactor:    mock,
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp peer * eorr ipv4/unicast")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
}

// TestDispatchBgpPeerRaw verifies bgp peer <ip> raw works.
//
// VALIDATES: "bgp peer 10.0.0.1 raw update hex <data>" sends raw bytes.
// PREVENTS: Raw passthrough not working.
func TestDispatchBgpPeerRaw(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	mock := &mockReactor{}
	ctx := &CommandContext{
		Reactor:    mock,
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp peer 10.0.0.1 raw update hex 0000")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	require.Len(t, mock.rawMessages, 1)
	assert.Equal(t, netip.MustParseAddr("10.0.0.1"), mock.rawMessages[0].addr)
}

// TestOldDaemonCommandsRemoved verifies old daemon commands are removed.
//
// VALIDATES: "daemon shutdown/status/reload" return unknown command.
// PREVENTS: Old command paths still working.
func TestOldDaemonCommandsRemoved(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	oldCommands := []string{
		"daemon shutdown",
		"daemon status",
		"daemon reload",
	}

	for _, cmd := range oldCommands {
		t.Run(cmd, func(t *testing.T) {
			c := d.Lookup(cmd)
			assert.Nil(t, c, "old command %q should NOT be registered", cmd)
		})
	}
}

// TestOldPeerCommandsRemoved verifies old peer commands are removed.
//
// VALIDATES: "peer list/show" and "neighbor * teardown" return unknown command.
// PREVENTS: Old command paths still working.
func TestOldPeerCommandsRemoved(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	oldCommands := []string{
		"peer list",
		"peer show",
		"teardown", // standalone teardown removed
	}

	for _, cmd := range oldCommands {
		t.Run(cmd, func(t *testing.T) {
			c := d.Lookup(cmd)
			assert.Nil(t, c, "old command %q should NOT be registered", cmd)
		})
	}
}

// TestOldUpdateWatchdogCommitRemoved verifies old route commands are removed.
//
// VALIDATES: "update", "watchdog", "commit", "raw", "borr", "eorr" return unknown.
// PREVENTS: Old standalone commands still working.
func TestOldUpdateWatchdogCommitRemoved(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	oldCommands := []string{
		"update",
		"watchdog announce",
		"watchdog withdraw",
		"commit",
		"raw",
		"borr",
		"eorr",
	}

	for _, cmd := range oldCommands {
		t.Run(cmd, func(t *testing.T) {
			c := d.Lookup(cmd)
			assert.Nil(t, c, "old command %q should NOT be registered", cmd)
		})
	}
}

// TestNeighborPrefixRemoved verifies neighbor prefix is no longer supported.
//
// VALIDATES: "neighbor 127.0.0.1 teardown 4" returns unknown command.
// PREVENTS: Old neighbor prefix still working.
func TestNeighborPrefixRemoved(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	mock := &mockReactor{}
	ctx := &CommandContext{
		Reactor:    mock,
		Dispatcher: d,
	}

	_, err := d.Dispatch(ctx, "neighbor 127.0.0.1 teardown 4")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrUnknownCommand)
}

// TestBgpStep5CommandsRegistered verifies all Step 5 commands are registered.
//
// VALIDATES: All migrated commands are accessible.
// PREVENTS: Missing command registrations.
func TestBgpStep5CommandsRegistered(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	commands := []string{
		// Daemon commands
		"bgp daemon shutdown",
		"bgp daemon status",
		"bgp daemon reload",
		// Peer commands (registered without selector - dispatcher extracts it)
		"bgp peer list",
		"bgp peer show",
		"bgp peer teardown",
		"bgp peer update",
		"bgp peer borr",
		"bgp peer eorr",
		"bgp peer raw",
		// Commit/watchdog
		"bgp commit",
		"bgp watchdog announce",
		"bgp watchdog withdraw",
	}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			c := d.Lookup(cmd)
			assert.NotNil(t, c, "command %q must be registered", cmd)
		})
	}
}

// TestBgpPeerSelectorExtraction verifies dispatcher extracts peer selector correctly.
//
// VALIDATES: "bgp peer 192.168.1.1 show" sets ctx.Peer to "192.168.1.1".
// PREVENTS: Peer selector not being extracted from command.
func TestBgpPeerSelectorExtraction(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	mock := &mockReactor{
		peers: []PeerInfo{
			{Address: netip.MustParseAddr("192.168.1.1"), State: "established"},
		},
	}
	ctx := &CommandContext{
		Reactor:    mock,
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp peer 192.168.1.1 show")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	assert.Equal(t, "192.168.1.1", ctx.Peer, "peer selector should be extracted")
}

// TestBgpPeerWildcardSelector verifies bgp peer * works.
//
// VALIDATES: "bgp peer * update ..." targets all peers.
// PREVENTS: Wildcard selector not working.
func TestBgpPeerWildcardSelector(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	mock := &mockReactor{}
	ctx := &CommandContext{
		Reactor:    mock,
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "bgp peer * borr ipv4/unicast")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "*", ctx.Peer, "wildcard selector should be extracted")
}

// =============================================================================
// Step 7: RIB Namespace & Plugin Commands Tests
// =============================================================================

// TestDispatchRibHelp verifies rib help returns subcommands.
//
// VALIDATES: "rib help" returns list of rib subcommands.
// PREVENTS: RIB namespace introspection broken.
func TestDispatchRibHelp(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ctx := &CommandContext{
		Reactor:    &mockReactor{},
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "rib help")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "data should be a map")

	subcommands, ok := data["subcommands"].([]string)
	require.True(t, ok, "subcommands should be []string")

	// Verify required subcommands are present
	assert.Contains(t, subcommands, "show", "should include show subcommand")
	assert.Contains(t, subcommands, "clear", "should include clear subcommand")
	assert.Contains(t, subcommands, "command", "should include command subcommand")
	assert.Contains(t, subcommands, "event", "should include event subcommand")
}

// TestDispatchRibCommandList verifies rib command list returns rib commands.
//
// VALIDATES: "rib command list" returns commands in rib namespace.
// PREVENTS: RIB command listing broken.
func TestDispatchRibCommandList(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ctx := &CommandContext{
		Reactor:    &mockReactor{},
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "rib command list")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "data should be a map")
	_, hasCommands := data["commands"]
	assert.True(t, hasCommands, "should have commands list")
}

// TestDispatchRibEventList verifies rib event list returns event types.
//
// VALIDATES: "rib event list" returns available RIB event types.
// PREVENTS: Event type discovery broken.
func TestDispatchRibEventList(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ctx := &CommandContext{
		Reactor:    &mockReactor{},
		Dispatcher: d,
	}

	resp, err := d.Dispatch(ctx, "rib event list")
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "data should be a map")
	events, ok := data["events"].([]string)
	require.True(t, ok, "events should be string slice")
	assert.Contains(t, events, "cache", "should include cache event")
	assert.Contains(t, events, "route", "should include route event")
}

// TestOldMsgIdCommandsRemoved verifies msg-id commands are no longer registered.
//
// VALIDATES: "msg-id *" commands migrated to "bgp cache *".
// PREVENTS: Old command syntax still working after migration.
func TestOldMsgIdCommandsRemoved(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	removedCommands := []string{
		"msg-id retain",
		"msg-id release",
		"msg-id expire",
		"msg-id list",
	}

	for _, cmd := range removedCommands {
		t.Run(cmd, func(t *testing.T) {
			c := d.Lookup(cmd)
			assert.Nil(t, c, "command %q should NOT be registered (migrated to bgp cache)", cmd)
		})
	}
}

// TestOldForwardCommandsRemoved verifies forward commands are no longer registered.
//
// VALIDATES: "bgp peer forward update-id" migrated to "bgp cache <id> forward".
// PREVENTS: Old command syntax still working after migration.
func TestOldForwardCommandsRemoved(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	removedCommands := []string{
		"bgp peer forward update-id",
		"bgp delete update-id",
	}

	for _, cmd := range removedCommands {
		t.Run(cmd, func(t *testing.T) {
			c := d.Lookup(cmd)
			assert.Nil(t, c, "command %q should NOT be registered (migrated to bgp cache)", cmd)
		})
	}
}

// TestBgpCacheCommandRegistered verifies bgp cache command is registered.
//
// VALIDATES: "bgp cache" command available after migration.
// PREVENTS: Missing new cache command registration.
func TestBgpCacheCommandRegistered(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	c := d.Lookup("bgp cache")
	assert.NotNil(t, c, "command 'bgp cache' must be registered")
}

// TestRibCommandsRegistered verifies all rib namespace commands are registered.
//
// VALIDATES: RIB namespace commands (introspection + operations) exist.
// PREVENTS: Missing command registrations.
func TestRibCommandsRegistered(t *testing.T) {
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ribCommands := []string{
		// Introspection
		"rib help",
		"rib command list",
		"rib command help",
		"rib command complete",
		"rib event list",
		// Operations
		"rib show in",
		"rib clear in",
	}

	for _, cmd := range ribCommands {
		t.Run(cmd, func(t *testing.T) {
			c := d.Lookup(cmd)
			assert.NotNil(t, c, "command %q must be registered", cmd)
		})
	}
}
