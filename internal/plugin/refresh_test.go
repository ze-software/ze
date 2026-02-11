package plugin

import (
	"errors"
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/nlri"
	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/rib"
	"codeberg.org/thomas-mangin/ze/internal/selector"
)

// mockReactorRefresh implements ReactorInterface for refresh command testing.
type mockReactorRefresh struct {
	borrCalled bool
	eorrCalled bool
	borrSel    string
	eorrSel    string
	borrAFI    uint16
	borrSAFI   uint8
	eorrAFI    uint16
	eorrSAFI   uint8
	borrErr    error
	eorrErr    error
}

func (m *mockReactorRefresh) SendBoRR(peerSelector string, afi uint16, safi uint8) error {
	m.borrCalled = true
	m.borrSel = peerSelector
	m.borrAFI = afi
	m.borrSAFI = safi
	return m.borrErr
}

func (m *mockReactorRefresh) SendEoRR(peerSelector string, afi uint16, safi uint8) error {
	m.eorrCalled = true
	m.eorrSel = peerSelector
	m.eorrAFI = afi
	m.eorrSAFI = safi
	return m.eorrErr
}

// Implement all other ReactorInterface methods as stubs.
func (m *mockReactorRefresh) Peers() []PeerInfo                                { return nil }
func (m *mockReactorRefresh) Stats() ReactorStats                              { return ReactorStats{} }
func (m *mockReactorRefresh) Stop()                                            {}
func (m *mockReactorRefresh) Reload() error                                    { return nil }
func (m *mockReactorRefresh) AddDynamicPeer(_ DynamicPeerConfig) error         { return nil }
func (m *mockReactorRefresh) RemovePeer(_ netip.Addr) error                    { return nil }
func (m *mockReactorRefresh) AnnounceRoute(_ string, _ RouteSpec) error        { return nil }
func (m *mockReactorRefresh) WithdrawRoute(_ string, _ netip.Prefix) error     { return nil }
func (m *mockReactorRefresh) AnnounceFlowSpec(_ string, _ FlowSpecRoute) error { return nil }
func (m *mockReactorRefresh) WithdrawFlowSpec(_ string, _ FlowSpecRoute) error { return nil }
func (m *mockReactorRefresh) AnnounceVPLS(_ string, _ VPLSRoute) error         { return nil }
func (m *mockReactorRefresh) WithdrawVPLS(_ string, _ VPLSRoute) error         { return nil }
func (m *mockReactorRefresh) AnnounceL2VPN(_ string, _ L2VPNRoute) error       { return nil }
func (m *mockReactorRefresh) WithdrawL2VPN(_ string, _ L2VPNRoute) error       { return nil }
func (m *mockReactorRefresh) AnnounceL3VPN(_ string, _ L3VPNRoute) error       { return nil }
func (m *mockReactorRefresh) WithdrawL3VPN(_ string, _ L3VPNRoute) error       { return nil }
func (m *mockReactorRefresh) AnnounceLabeledUnicast(_ string, _ LabeledUnicastRoute) error {
	return nil
}
func (m *mockReactorRefresh) WithdrawLabeledUnicast(_ string, _ LabeledUnicastRoute) error {
	return nil
}
func (m *mockReactorRefresh) AnnounceMUPRoute(_ string, _ MUPRouteSpec) error { return nil }
func (m *mockReactorRefresh) WithdrawMUPRoute(_ string, _ MUPRouteSpec) error { return nil }
func (m *mockReactorRefresh) TeardownPeer(_ netip.Addr, _ uint8) error        { return nil }
func (m *mockReactorRefresh) AnnounceEOR(_ string, _ uint16, _ uint8) error   { return nil }
func (m *mockReactorRefresh) RIBInRoutes(_ string) []rib.RouteJSON            { return nil }
func (m *mockReactorRefresh) RIBOutRoutes() []rib.RouteJSON                   { return nil }
func (m *mockReactorRefresh) RIBStats() RIBStatsInfo                          { return RIBStatsInfo{} }
func (m *mockReactorRefresh) BeginTransaction(_, _ string) error              { return nil }
func (m *mockReactorRefresh) CommitTransaction(_ string) (TransactionResult, error) {
	return TransactionResult{}, nil
}
func (m *mockReactorRefresh) CommitTransactionWithLabel(_, _ string) (TransactionResult, error) {
	return TransactionResult{}, nil
}
func (m *mockReactorRefresh) RollbackTransaction(_ string) (TransactionResult, error) {
	return TransactionResult{}, nil
}
func (m *mockReactorRefresh) InTransaction(_ string) bool   { return false }
func (m *mockReactorRefresh) TransactionID(_ string) string { return "" }
func (m *mockReactorRefresh) SendRoutes(_ string, _ []*rib.Route, _ []nlri.NLRI, _ bool) (TransactionResult, error) {
	return TransactionResult{}, nil
}
func (m *mockReactorRefresh) AnnounceWatchdog(_, _ string) error           { return nil }
func (m *mockReactorRefresh) WithdrawWatchdog(_, _ string) error           { return nil }
func (m *mockReactorRefresh) AddWatchdogRoute(_ RouteSpec, _ string) error { return nil }
func (m *mockReactorRefresh) RemoveWatchdogRoute(_, _ string) error        { return nil }
func (m *mockReactorRefresh) ClearRIBIn() int                              { return 0 }
func (m *mockReactorRefresh) ClearRIBOut() int                             { return 0 }
func (m *mockReactorRefresh) FlushRIBOut() int                             { return 0 }
func (m *mockReactorRefresh) GetPeerProcessBindings(_ netip.Addr) []PeerProcessBinding {
	return nil
}
func (m *mockReactorRefresh) GetPeerCapabilityConfigs() []PeerCapabilityConfig { return nil }
func (m *mockReactorRefresh) GetConfigTree() map[string]any                    { return nil }
func (m *mockReactorRefresh) SetConfigTree(_ map[string]any)                   {}
func (m *mockReactorRefresh) SignalAPIReady()                                  {}
func (m *mockReactorRefresh) AddAPIProcessCount(_ int)                         {}
func (m *mockReactorRefresh) SignalPluginStartupComplete()                     {}
func (m *mockReactorRefresh) SignalPeerAPIReady(_ string)                      {}
func (m *mockReactorRefresh) AnnounceNLRIBatch(_ string, _ NLRIBatch) error    { return nil }
func (m *mockReactorRefresh) WithdrawNLRIBatch(_ string, _ NLRIBatch) error    { return nil }
func (m *mockReactorRefresh) SendRawMessage(_ netip.Addr, _ uint8, _ []byte) error {
	return nil
}
func (m *mockReactorRefresh) ForwardUpdate(_ *selector.Selector, _ uint64) error { return nil }
func (m *mockReactorRefresh) DeleteUpdate(_ uint64) error                        { return nil }
func (m *mockReactorRefresh) RetainUpdate(_ uint64) error                        { return nil }
func (m *mockReactorRefresh) ReleaseUpdate(_ uint64) error                       { return nil }
func (m *mockReactorRefresh) ListUpdates() []uint64                              { return nil }

// TestRefreshCommands verifies borr/eorr command parsing and execution.
// RFC 7313 Section 4 defines BoRR/EoRR semantics for Enhanced Route Refresh.
//
// VALIDATES: borr/eorr commands work with peer selector and family.
// PREVENTS: Command parsing errors, missing refresh markers.
func TestRefreshCommands(t *testing.T) {
	tests := []struct {
		name     string
		cmd      string // "borr" or "eorr"
		peer     string
		family   string
		wantSel  string
		wantAFI  uint16
		wantSAFI uint8
		wantErr  bool
	}{
		// BoRR tests
		{"borr specific peer", "borr", "10.0.0.1", "ipv4/unicast", "10.0.0.1", 1, 1, false},
		{"borr all peers ipv6", "borr", "*", "ipv6/unicast", "*", 2, 1, false},
		{"borr exclude peer", "borr", "!10.0.0.1", "ipv4/unicast", "!10.0.0.1", 1, 1, false},
		{"borr missing family", "borr", "10.0.0.1", "", "", 0, 0, true},
		{"borr invalid family", "borr", "10.0.0.1", "invalid", "", 0, 0, true},
		// EoRR tests
		{"eorr specific peer", "eorr", "10.0.0.1", "ipv4/unicast", "10.0.0.1", 1, 1, false},
		{"eorr all peers ipv6", "eorr", "*", "ipv6/unicast", "*", 2, 1, false},
		{"eorr exclude peer", "eorr", "!10.0.0.1", "ipv4/unicast", "!10.0.0.1", 1, 1, false},
		{"eorr missing family", "eorr", "10.0.0.1", "", "", 0, 0, true},
		{"eorr invalid family", "eorr", "10.0.0.1", "badformat", "", 0, 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockReactorRefresh{}
			d := NewDispatcher()
			RegisterDefaultHandlers(d)

			// Build command string (Step 5: now uses bgp peer prefix)
			input := "bgp peer " + tc.peer + " " + tc.cmd
			if tc.family != "" {
				input += " " + tc.family
			}

			ctx := &CommandContext{Reactor: mock}
			resp, err := d.Dispatch(ctx, input)

			if tc.wantErr {
				if err == nil && (resp == nil || resp.Status != statusError) {
					t.Error("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check correct method was called
			if tc.cmd == "borr" {
				if !mock.borrCalled {
					t.Error("SendBoRR not called")
				}
				if mock.borrSel != tc.wantSel {
					t.Errorf("selector = %s, want %s", mock.borrSel, tc.wantSel)
				}
				if mock.borrAFI != tc.wantAFI {
					t.Errorf("AFI = %d, want %d", mock.borrAFI, tc.wantAFI)
				}
				if mock.borrSAFI != tc.wantSAFI {
					t.Errorf("SAFI = %d, want %d", mock.borrSAFI, tc.wantSAFI)
				}
			} else {
				if !mock.eorrCalled {
					t.Error("SendEoRR not called")
				}
				if mock.eorrSel != tc.wantSel {
					t.Errorf("selector = %s, want %s", mock.eorrSel, tc.wantSel)
				}
				if mock.eorrAFI != tc.wantAFI {
					t.Errorf("AFI = %d, want %d", mock.eorrAFI, tc.wantAFI)
				}
				if mock.eorrSAFI != tc.wantSAFI {
					t.Errorf("SAFI = %d, want %d", mock.eorrSAFI, tc.wantSAFI)
				}
			}
		})
	}
}

// TestRefreshErrors verifies error propagation from reactor.
//
// VALIDATES: Reactor errors are propagated correctly.
// PREVENTS: Silent failures when reactor method fails.
func TestRefreshErrors(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		borrErr error
		eorrErr error
	}{
		{"borr error", "borr", errors.New("peer not found"), nil},
		{"eorr error", "eorr", nil, errors.New("peer not found")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockReactorRefresh{borrErr: tc.borrErr, eorrErr: tc.eorrErr}
			d := NewDispatcher()
			RegisterDefaultHandlers(d)

			ctx := &CommandContext{Reactor: mock}
			resp, err := d.Dispatch(ctx, "bgp peer 10.0.0.1 "+tc.cmd+" ipv4/unicast")

			if err == nil && (resp == nil || resp.Status != statusError) {
				t.Error("expected error status when reactor fails")
			}
		})
	}
}
