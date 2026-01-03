package api

import (
	"net/netip"
	"testing"
	"time"

	"github.com/exa-networks/zebgp/pkg/bgp/message"
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
	// RIB operation tracking
	ribInCleared  bool
	ribOutCleared bool
	ribOutFlushed bool
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

func (m *mockReactor) RIBInRoutes(_ string) []RIBRoute {
	return nil
}

func (m *mockReactor) RIBOutRoutes() []RIBRoute {
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

func (m *mockReactor) GetPeerAPIBindings(_ netip.Addr) []PeerAPIBinding {
	return nil // Mock: no API bindings configured
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

func (m *mockReactor) ForwardUpdate(_ *Selector, _ uint64) error {
	return nil
}

func (m *mockReactor) DeleteUpdate(_ uint64) error {
	return nil
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
			assert.Contains(t, resp.Data.(string), tt.errMsg) //nolint:forcetypeassert // test code
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
			assert.Contains(t, resp.Data.(string), tt.errMsg) //nolint:forcetypeassert // test code
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
	assert.Contains(t, resp.Data.(string), "rd") //nolint:forcetypeassert // test code
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

// TestHandleAnnounceL3VPN_IPv4 verifies L3VPN IPv4 route announcement.
//
// VALIDATES: L3VPN routes with RD, label, next-hop are announced correctly.
//
// PREVENTS: L3VPN routes not being announced to reactor.
func TestHandleAnnounceL3VPN_IPv4(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	// announce ipv4 mpls-vpn 10.0.0.0/24 rd 100:100 label 100 next-hop 1.2.3.4
	args := []string{"mpls-vpn", "10.0.0.0/24", "rd", "100:100", "label", "100", "next-hop", "1.2.3.4"}
	resp, err := handleAnnounceIPv4(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	require.Len(t, reactor.announcedL3VPNRoutes, 1)

	route := reactor.announcedL3VPNRoutes[0].route
	assert.Equal(t, "100:100", route.RD)
	require.Len(t, route.Labels, 1)
	assert.Equal(t, uint32(100), route.Labels[0])
	assert.Equal(t, netip.MustParseAddr("1.2.3.4"), route.NextHop)
	assert.Equal(t, netip.MustParsePrefix("10.0.0.0/24"), route.Prefix)
}

// TestHandleAnnounceL3VPN_IPv6 verifies L3VPN IPv6 route announcement.
//
// VALIDATES: L3VPN IPv6 routes with RD, label, next-hop are announced correctly.
//
// PREVENTS: IPv6 VPN routes not working.
func TestHandleAnnounceL3VPN_IPv6(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	// announce ipv6 mpls-vpn 2001:db8::/32 rd 100:100 label 100 next-hop 2001::1
	args := []string{"mpls-vpn", "2001:db8::/32", "rd", "100:100", "label", "100", "next-hop", "2001::1"}
	resp, err := handleAnnounceIPv6(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	require.Len(t, reactor.announcedL3VPNRoutes, 1)

	route := reactor.announcedL3VPNRoutes[0].route
	assert.Equal(t, "100:100", route.RD)
	require.Len(t, route.Labels, 1)
	assert.Equal(t, uint32(100), route.Labels[0])
	assert.Equal(t, netip.MustParseAddr("2001::1"), route.NextHop)
	assert.Equal(t, netip.MustParsePrefix("2001:db8::/32"), route.Prefix)
}

// TestHandleAnnounceL3VPN_LabelZero verifies label=0 (Explicit Null) is valid.
//
// VALIDATES: MPLS label 0 is accepted per RFC 3032.
//
// PREVENTS: Label 0 being rejected as "missing".
func TestHandleAnnounceL3VPN_LabelZero(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	// Label 0 = IPv4 Explicit Null per RFC 3032
	args := []string{"mpls-vpn", "10.0.0.0/24", "rd", "100:100", "label", "0", "next-hop", "1.2.3.4"}
	resp, err := handleAnnounceIPv4(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	require.Len(t, reactor.announcedL3VPNRoutes, 1)
	require.Len(t, reactor.announcedL3VPNRoutes[0].route.Labels, 1)
	assert.Equal(t, uint32(0), reactor.announcedL3VPNRoutes[0].route.Labels[0])
}

// TestHandleAnnounceL3VPN_LabelStack verifies label stack support.
//
// VALIDATES: Multiple labels can be specified as [label1, label2, ...].
//
// PREVENTS: Label stacks being rejected.
func TestHandleAnnounceL3VPN_LabelStack(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	args := []string{"mpls-vpn", "10.0.0.0/24", "rd", "100:100", "label", "[100", "200", "300]", "next-hop", "1.2.3.4"}
	resp, err := handleAnnounceIPv4(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	require.Len(t, reactor.announcedL3VPNRoutes, 1)

	labels := reactor.announcedL3VPNRoutes[0].route.Labels
	require.Len(t, labels, 3)
	assert.Equal(t, uint32(100), labels[0])
	assert.Equal(t, uint32(200), labels[1])
	assert.Equal(t, uint32(300), labels[2])
}

// TestHandleAnnounceL3VPN_LabelBoundary verifies label boundary values.
//
// VALIDATES: Max label (1048575) accepted, overflow (1048576) rejected.
//
// PREVENTS: Boundary errors in label validation.
func TestHandleAnnounceL3VPN_LabelBoundary(t *testing.T) {
	tests := []struct {
		name    string
		label   string
		wantErr bool
	}{
		{"max valid", "1048575", false},
		{"overflow", "1048576", true},
		{"special label 3 (implicit null)", "3", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reactor := &mockReactor{}
			ctx := &CommandContext{Reactor: reactor}

			args := []string{"mpls-vpn", "10.0.0.0/24", "rd", "100:100", "label", tt.label, "next-hop", "1.2.3.4"}
			resp, err := handleAnnounceIPv4(ctx, args)

			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, "error", resp.Status)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "done", resp.Status)
			}
		})
	}
}

// TestHandleAnnounceL3VPN_LabelStackFormats verifies different label stack formats.
//
// VALIDATES: Space-separated and comma-separated label stacks work.
//
// PREVENTS: Label stack parsing failures.
func TestHandleAnnounceL3VPN_LabelStackFormats(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		labels []uint32
	}{
		{"comma separated", []string{"mpls-vpn", "10.0.0.0/24", "rd", "100:100", "label", "[100,200,300]", "next-hop", "1.2.3.4"}, []uint32{100, 200, 300}},
		{"space separated", []string{"mpls-vpn", "10.0.0.0/24", "rd", "100:100", "label", "[100", "200", "300]", "next-hop", "1.2.3.4"}, []uint32{100, 200, 300}},
		{"single in brackets", []string{"mpls-vpn", "10.0.0.0/24", "rd", "100:100", "label", "[100]", "next-hop", "1.2.3.4"}, []uint32{100}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reactor := &mockReactor{}
			ctx := &CommandContext{Reactor: reactor}

			resp, err := handleAnnounceIPv4(ctx, tt.args)

			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, "done", resp.Status)
			require.Len(t, reactor.announcedL3VPNRoutes, 1)
			assert.Equal(t, tt.labels, reactor.announcedL3VPNRoutes[0].route.Labels)
		})
	}
}

// TestHandleAnnounceL3VPN_InvalidRDFormat verifies RD format validation.
//
// VALIDATES: Invalid RD formats are rejected.
//
// PREVENTS: Malformed RDs being accepted.
func TestHandleAnnounceL3VPN_InvalidRDFormat(t *testing.T) {
	tests := []struct {
		name string
		rd   string
	}{
		{"no colon", "100100"},
		{"invalid IP", "999.999.999.999:100"},
		{"empty prefix", ":100"},
		{"empty suffix", "100:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reactor := &mockReactor{}
			ctx := &CommandContext{Reactor: reactor}

			args := []string{"mpls-vpn", "10.0.0.0/24", "rd", tt.rd, "label", "100", "next-hop", "1.2.3.4"}
			resp, err := handleAnnounceIPv4(ctx, args)

			require.Error(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, "error", resp.Status)
		})
	}
}

// TestHandleAnnounceL3VPN_ValidRDFormats verifies all valid RD formats work.
//
// VALIDATES: Type 0, 1, and 2 RD formats are accepted, including boundary values.
//
// PREVENTS: Valid RDs being rejected.
func TestHandleAnnounceL3VPN_ValidRDFormats(t *testing.T) {
	tests := []struct {
		name string
		rd   string
	}{
		// Type 0: 2-byte ASN : 4-byte value
		{"Type 0: basic", "65000:100"},
		{"Type 0: min", "0:0"},
		{"Type 0: max ASN", "65535:100"},
		{"Type 0: max value", "65535:4294967295"},

		// Type 1: IPv4 : 2-byte value
		{"Type 1: basic", "1.2.3.4:100"},
		{"Type 1: max IP", "255.255.255.255:100"},
		{"Type 1: zero IP", "0.0.0.0:100"},
		{"Type 1: max value", "1.2.3.4:65535"},

		// Type 2: 4-byte ASN : 2-byte value
		{"Type 2: basic", "4200000000:100"},
		{"Type 2: max ASN", "4294967295:100"},
		{"Type 2: max value", "4294967295:65535"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reactor := &mockReactor{}
			ctx := &CommandContext{Reactor: reactor}

			args := []string{"mpls-vpn", "10.0.0.0/24", "rd", tt.rd, "label", "100", "next-hop", "1.2.3.4"}
			resp, err := handleAnnounceIPv4(ctx, args)

			require.NoError(t, err)
			require.NotNil(t, resp)
			assert.Equal(t, "done", resp.Status)
		})
	}
}

// TestHandleAnnounceL3VPN_RequiresRD verifies RD is required for L3VPN.
//
// VALIDATES: Missing RD returns error.
//
// PREVENTS: Routes without RD being accepted.
func TestHandleAnnounceL3VPN_RequiresRD(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	// Missing RD should fail
	args := []string{"mpls-vpn", "10.0.0.0/24", "label", "100", "next-hop", "1.2.3.4"}
	resp, err := handleAnnounceIPv4(ctx, args)

	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Data.(string), "rd") //nolint:forcetypeassert // test code
}

// TestHandleAnnounceL3VPN_RequiresLabel verifies label is required for L3VPN.
//
// VALIDATES: Missing label returns error.
//
// PREVENTS: Routes without label being accepted.
func TestHandleAnnounceL3VPN_RequiresLabel(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	// Missing label should fail
	args := []string{"mpls-vpn", "10.0.0.0/24", "rd", "100:100", "next-hop", "1.2.3.4"}
	resp, err := handleAnnounceIPv4(ctx, args)

	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Data.(string), "label") //nolint:forcetypeassert // test code
}

// TestHandleAnnounceL3VPN_WithRT verifies Route Target support.
//
// VALIDATES: Route target (extended community) is parsed.
//
// PREVENTS: RT being ignored.
func TestHandleAnnounceL3VPN_WithRT(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	args := []string{"mpls-vpn", "10.0.0.0/24", "rd", "100:100", "label", "100", "next-hop", "1.2.3.4", "rt", "100:200"}
	resp, err := handleAnnounceIPv4(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	require.Len(t, reactor.announcedL3VPNRoutes, 1)
	assert.Equal(t, "100:200", reactor.announcedL3VPNRoutes[0].route.RT)
}

// TestHandleAnnounceL3VPN_WithAttributes verifies standard attributes work.
//
// VALIDATES: Origin, local-pref, MED, AS-path, communities work with L3VPN.
//
// PREVENTS: Standard attributes being rejected for L3VPN.
func TestHandleAnnounceL3VPN_WithAttributes(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	args := []string{
		"mpls-vpn", "10.0.0.0/24",
		"rd", "100:100",
		"label", "100",
		"next-hop", "1.2.3.4",
		"origin", "igp",
		"local-preference", "200",
		"community", "[65000:100]",
	}
	resp, err := handleAnnounceIPv4(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	require.Len(t, reactor.announcedL3VPNRoutes, 1)

	route := reactor.announcedL3VPNRoutes[0].route
	require.NotNil(t, route.Origin)
	assert.Equal(t, uint8(0), *route.Origin) // IGP
	require.NotNil(t, route.LocalPreference)
	assert.Equal(t, uint32(200), *route.LocalPreference)
}

// TestHandleWithdrawL3VPN_IPv4 verifies L3VPN IPv4 route withdrawal.
//
// VALIDATES: L3VPN routes can be withdrawn.
//
// PREVENTS: Inability to withdraw L3VPN routes.
func TestHandleWithdrawL3VPN_IPv4(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	// withdraw ipv4 mpls-vpn 10.0.0.0/24 rd 100:100
	args := []string{"mpls-vpn", "10.0.0.0/24", "rd", "100:100"}
	resp, err := handleWithdrawIPv4(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
}

// =============================================================================
// MPLS Labeled Unicast Tests (SAFI 4)
// =============================================================================

// TestHandleAnnounceLabeledUnicast_IPv4 verifies labeled unicast IPv4 route announcement.
//
// VALIDATES: Labeled unicast routes with label stack are announced correctly.
//
// PREVENTS: SAFI 4 routes not being recognized.
func TestHandleAnnounceLabeledUnicast_IPv4(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	// announce ipv4 labeled-unicast 10.0.0.0/24 label 100 next-hop 1.2.3.4
	args := []string{"labeled-unicast", "10.0.0.0/24", "label", "100", "next-hop", "1.2.3.4"}
	resp, err := handleAnnounceIPv4(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	require.Len(t, reactor.announcedLabeledUnicastRoutes, 1)

	route := reactor.announcedLabeledUnicastRoutes[0].route
	require.Len(t, route.Labels, 1)
	assert.Equal(t, uint32(100), route.Labels[0])
	assert.Equal(t, netip.MustParseAddr("1.2.3.4"), route.NextHop)
	assert.Equal(t, netip.MustParsePrefix("10.0.0.0/24"), route.Prefix)
}

// TestHandleAnnounceLabeledUnicast_IPv6 verifies labeled unicast IPv6 route announcement.
//
// VALIDATES: Labeled unicast IPv6 routes work correctly.
//
// PREVENTS: IPv6 SAFI 4 routes not working.
func TestHandleAnnounceLabeledUnicast_IPv6(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	// announce ipv6 labeled-unicast 2001:db8::/32 label 100 next-hop 2001::1
	args := []string{"labeled-unicast", "2001:db8::/32", "label", "100", "next-hop", "2001::1"}
	resp, err := handleAnnounceIPv6(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	require.Len(t, reactor.announcedLabeledUnicastRoutes, 1)

	route := reactor.announcedLabeledUnicastRoutes[0].route
	require.Len(t, route.Labels, 1)
	assert.Equal(t, uint32(100), route.Labels[0])
	assert.Equal(t, netip.MustParseAddr("2001::1"), route.NextHop)
	assert.Equal(t, netip.MustParsePrefix("2001:db8::/32"), route.Prefix)
}

// TestHandleAnnounceLabeledUnicast_LabelStack verifies label stack support.
//
// VALIDATES: Multiple labels can be specified as [label1, label2, ...].
//
// PREVENTS: Label stacks being rejected for labeled-unicast.
func TestHandleAnnounceLabeledUnicast_LabelStack(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	args := []string{"labeled-unicast", "10.0.0.0/24", "label", "[100", "200", "300]", "next-hop", "1.2.3.4"}
	resp, err := handleAnnounceIPv4(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	require.Len(t, reactor.announcedLabeledUnicastRoutes, 1)

	labels := reactor.announcedLabeledUnicastRoutes[0].route.Labels
	require.Len(t, labels, 3)
	assert.Equal(t, uint32(100), labels[0])
	assert.Equal(t, uint32(200), labels[1])
	assert.Equal(t, uint32(300), labels[2])
}

// TestHandleAnnounceLabeledUnicast_RequiresLabel verifies label is required.
//
// VALIDATES: Missing label returns error.
//
// PREVENTS: Routes without label being accepted for labeled-unicast.
func TestHandleAnnounceLabeledUnicast_RequiresLabel(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	// Missing label should fail
	args := []string{"labeled-unicast", "10.0.0.0/24", "next-hop", "1.2.3.4"}
	resp, err := handleAnnounceIPv4(ctx, args)

	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Data.(string), "label") //nolint:forcetypeassert // test code
}

// TestHandleAnnounceLabeledUnicast_RejectsRD verifies RD is rejected.
//
// VALIDATES: RD keyword returns error for labeled-unicast.
//
// PREVENTS: VPN-only keywords being accepted for labeled-unicast.
func TestHandleAnnounceLabeledUnicast_RejectsRD(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	// RD should not be valid for labeled-unicast
	args := []string{"labeled-unicast", "10.0.0.0/24", "label", "100", "next-hop", "1.2.3.4", "rd", "100:100"}
	resp, err := handleAnnounceIPv4(ctx, args)

	require.Error(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "error", resp.Status)
	assert.Contains(t, resp.Data.(string), "rd") //nolint:forcetypeassert // test code
}

// TestHandleAnnounceLabeledUnicast_WithAttributes verifies standard attributes work.
//
// VALIDATES: Origin, local-pref, MED, AS-path, communities work with labeled-unicast.
//
// PREVENTS: Standard attributes being rejected for labeled-unicast.
func TestHandleAnnounceLabeledUnicast_WithAttributes(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	args := []string{
		"labeled-unicast", "10.0.0.0/24",
		"label", "100",
		"next-hop", "1.2.3.4",
		"origin", "igp",
		"local-preference", "200",
		"community", "[65000:100]",
	}
	resp, err := handleAnnounceIPv4(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	require.Len(t, reactor.announcedLabeledUnicastRoutes, 1)

	route := reactor.announcedLabeledUnicastRoutes[0].route
	require.NotNil(t, route.Origin)
	assert.Equal(t, uint8(0), *route.Origin) // IGP
	require.NotNil(t, route.LocalPreference)
	assert.Equal(t, uint32(200), *route.LocalPreference)
}

// TestHandleWithdrawLabeledUnicast_IPv4 verifies labeled unicast IPv4 route withdrawal.
//
// VALIDATES: Labeled unicast routes can be withdrawn.
//
// PREVENTS: Inability to withdraw labeled-unicast routes.
func TestHandleWithdrawLabeledUnicast_IPv4(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	// withdraw ipv4 labeled-unicast 10.0.0.0/24 label 100
	args := []string{"labeled-unicast", "10.0.0.0/24", "label", "100"}
	resp, err := handleWithdrawIPv4(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
}

// TestHandleWithdrawLabeledUnicast_IPv6 verifies labeled unicast IPv6 route withdrawal.
//
// VALIDATES: IPv6 labeled unicast routes can be withdrawn.
//
// PREVENTS: IPv6 withdraw not working.
func TestHandleWithdrawLabeledUnicast_IPv6(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	args := []string{"labeled-unicast", "2001:db8::/32", "label", "100"}
	resp, err := handleWithdrawIPv6(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
}

// TestHandleAnnounceLabeledUnicast_LabelZero verifies label=0 (Explicit Null) is valid.
//
// VALIDATES: MPLS label 0 is accepted per RFC 3032.
//
// PREVENTS: Label 0 being rejected as "missing".
func TestHandleAnnounceLabeledUnicast_LabelZero(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	// Label 0 = IPv4 Explicit Null per RFC 3032
	args := []string{"labeled-unicast", "10.0.0.0/24", "label", "0", "next-hop", "1.2.3.4"}
	resp, err := handleAnnounceIPv4(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	require.Len(t, reactor.announcedLabeledUnicastRoutes, 1)
	require.Len(t, reactor.announcedLabeledUnicastRoutes[0].route.Labels, 1)
	assert.Equal(t, uint32(0), reactor.announcedLabeledUnicastRoutes[0].route.Labels[0])
}

// TestHandleAnnounceLabeledUnicast_LabelBoundary verifies label boundary values.
//
// VALIDATES: Max label (1048575) accepted, overflow (1048576) rejected.
//
// PREVENTS: Boundary errors in label validation.
func TestHandleAnnounceLabeledUnicast_LabelBoundary(t *testing.T) {
	tests := []struct {
		name    string
		label   string
		wantErr bool
	}{
		{"max valid", "1048575", false},
		{"overflow", "1048576", true},
		{"special label 3 (implicit null)", "3", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reactor := &mockReactor{}
			ctx := &CommandContext{Reactor: reactor}

			args := []string{"labeled-unicast", "10.0.0.0/24", "label", tt.label, "next-hop", "1.2.3.4"}
			resp, err := handleAnnounceIPv4(ctx, args)

			if tt.wantErr {
				require.Error(t, err)
				assert.Equal(t, "error", resp.Status)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "done", resp.Status)
			}
		})
	}
}

// TestHandleAnnounceLabeledUnicast_NlriMplsAlias verifies ExaBGP-compatible nlri-mpls SAFI.
//
// VALIDATES: "nlri-mpls" works as alias for "labeled-unicast".
//
// PREVENTS: ExaBGP compatibility issues.
func TestHandleAnnounceLabeledUnicast_NlriMplsAlias(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	// ExaBGP uses "nlri-mpls" as the SAFI name
	args := []string{"nlri-mpls", "10.0.0.0/24", "label", "100", "next-hop", "1.2.3.4"}
	resp, err := handleAnnounceIPv4(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	require.Len(t, reactor.announcedLabeledUnicastRoutes, 1)
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

// TestHandleAnnounceLabeledUnicast_AttributeValues verifies attribute values are parsed correctly.
//
// VALIDATES: MED, AS-path, and other attributes have correct values.
//
// PREVENTS: Attributes being accepted but values ignored.
func TestHandleAnnounceLabeledUnicast_AttributeValues(t *testing.T) {
	reactor := &mockReactor{}
	ctx := &CommandContext{Reactor: reactor}

	args := []string{
		"labeled-unicast", "10.0.0.0/24",
		"label", "100",
		"next-hop", "1.2.3.4",
		"origin", "egp",
		"med", "500",
		"local-preference", "150",
		"as-path", "[65001", "65002]",
		"community", "[65000:100", "65000:200]",
		"large-community", "[65000:1:2]",
	}
	resp, err := handleAnnounceIPv4(ctx, args)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "done", resp.Status)
	require.Len(t, reactor.announcedLabeledUnicastRoutes, 1)

	route := reactor.announcedLabeledUnicastRoutes[0].route

	// Verify all attribute values
	require.NotNil(t, route.Origin)
	assert.Equal(t, uint8(1), *route.Origin) // EGP = 1

	require.NotNil(t, route.MED)
	assert.Equal(t, uint32(500), *route.MED)

	require.NotNil(t, route.LocalPreference)
	assert.Equal(t, uint32(150), *route.LocalPreference)

	require.Len(t, route.ASPath, 2)
	assert.Equal(t, uint32(65001), route.ASPath[0])
	assert.Equal(t, uint32(65002), route.ASPath[1])

	require.Len(t, route.Communities, 2)

	require.Len(t, route.LargeCommunities, 1)
	assert.Equal(t, uint32(65000), route.LargeCommunities[0].GlobalAdmin)
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
		binding PeerAPIBinding
		msgType message.MessageType
		want    bool
	}{
		{
			name:    "update subscribed",
			binding: PeerAPIBinding{ReceiveUpdate: true},
			msgType: message.TypeUPDATE,
			want:    true,
		},
		{
			name:    "update not subscribed",
			binding: PeerAPIBinding{ReceiveUpdate: false},
			msgType: message.TypeUPDATE,
			want:    false,
		},
		{
			name:    "open subscribed",
			binding: PeerAPIBinding{ReceiveOpen: true},
			msgType: message.TypeOPEN,
			want:    true,
		},
		{
			name:    "notification subscribed",
			binding: PeerAPIBinding{ReceiveNotification: true},
			msgType: message.TypeNOTIFICATION,
			want:    true,
		},
		{
			name:    "keepalive subscribed",
			binding: PeerAPIBinding{ReceiveKeepalive: true},
			msgType: message.TypeKEEPALIVE,
			want:    true,
		},
		{
			name:    "mixed flags - only update",
			binding: PeerAPIBinding{ReceiveUpdate: true, ReceiveOpen: false},
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
