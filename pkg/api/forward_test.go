package api

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/exa-networks/zebgp/pkg/bgp/nlri"
	"github.com/exa-networks/zebgp/pkg/rib"
)

// errUpdateExpired is a test error matching reactor.ErrUpdateExpired.
// Cannot import reactor due to import cycle (reactor imports api).
var errUpdateExpired = errors.New("update-id expired or not found")

// mockReactorForward implements ReactorInterface for forward command testing.
type mockReactorForward struct {
	forwardCalled bool
	forwardSel    *Selector
	forwardID     uint64
	forwardErr    error
}

func (m *mockReactorForward) ForwardUpdate(sel *Selector, updateID uint64) error {
	m.forwardCalled = true
	m.forwardSel = sel
	m.forwardID = updateID
	return m.forwardErr
}

func (m *mockReactorForward) DeleteUpdate(_ uint64) error {
	return nil
}

// Implement all other ReactorInterface methods as stubs.
func (m *mockReactorForward) Peers() []PeerInfo                                { return nil }
func (m *mockReactorForward) Stats() ReactorStats                              { return ReactorStats{} }
func (m *mockReactorForward) Stop()                                            {}
func (m *mockReactorForward) Reload() error                                    { return nil }
func (m *mockReactorForward) AnnounceRoute(_ string, _ RouteSpec) error        { return nil }
func (m *mockReactorForward) WithdrawRoute(_ string, _ netip.Prefix) error     { return nil }
func (m *mockReactorForward) AnnounceFlowSpec(_ string, _ FlowSpecRoute) error { return nil }
func (m *mockReactorForward) WithdrawFlowSpec(_ string, _ FlowSpecRoute) error { return nil }
func (m *mockReactorForward) AnnounceVPLS(_ string, _ VPLSRoute) error         { return nil }
func (m *mockReactorForward) WithdrawVPLS(_ string, _ VPLSRoute) error         { return nil }
func (m *mockReactorForward) AnnounceL2VPN(_ string, _ L2VPNRoute) error       { return nil }
func (m *mockReactorForward) WithdrawL2VPN(_ string, _ L2VPNRoute) error       { return nil }
func (m *mockReactorForward) AnnounceL3VPN(_ string, _ L3VPNRoute) error       { return nil }
func (m *mockReactorForward) WithdrawL3VPN(_ string, _ L3VPNRoute) error       { return nil }
func (m *mockReactorForward) AnnounceLabeledUnicast(_ string, _ LabeledUnicastRoute) error {
	return nil
}
func (m *mockReactorForward) WithdrawLabeledUnicast(_ string, _ LabeledUnicastRoute) error {
	return nil
}
func (m *mockReactorForward) AnnounceMUPRoute(_ string, _ MUPRouteSpec) error { return nil }
func (m *mockReactorForward) WithdrawMUPRoute(_ string, _ MUPRouteSpec) error { return nil }
func (m *mockReactorForward) TeardownPeer(_ netip.Addr, _ uint8) error        { return nil }
func (m *mockReactorForward) AnnounceEOR(_ string, _ uint16, _ uint8) error   { return nil }
func (m *mockReactorForward) RIBInRoutes(_ string) []RIBRoute                 { return nil }
func (m *mockReactorForward) RIBOutRoutes() []RIBRoute                        { return nil }
func (m *mockReactorForward) RIBStats() RIBStatsInfo                          { return RIBStatsInfo{} }
func (m *mockReactorForward) BeginTransaction(_, _ string) error              { return nil }
func (m *mockReactorForward) CommitTransaction(_ string) (TransactionResult, error) {
	return TransactionResult{}, nil
}
func (m *mockReactorForward) CommitTransactionWithLabel(_, _ string) (TransactionResult, error) {
	return TransactionResult{}, nil
}
func (m *mockReactorForward) RollbackTransaction(_ string) (TransactionResult, error) {
	return TransactionResult{}, nil
}
func (m *mockReactorForward) InTransaction(_ string) bool   { return false }
func (m *mockReactorForward) TransactionID(_ string) string { return "" }
func (m *mockReactorForward) SendRoutes(_ string, _ []*rib.Route, _ []nlri.NLRI, _ bool) (TransactionResult, error) {
	return TransactionResult{}, nil
}
func (m *mockReactorForward) AnnounceWatchdog(_, _ string) error               { return nil }
func (m *mockReactorForward) WithdrawWatchdog(_, _ string) error               { return nil }
func (m *mockReactorForward) AddWatchdogRoute(_ RouteSpec, _ string) error     { return nil }
func (m *mockReactorForward) RemoveWatchdogRoute(_, _ string) error            { return nil }
func (m *mockReactorForward) ClearRIBIn() int                                  { return 0 }
func (m *mockReactorForward) ClearRIBOut() int                                 { return 0 }
func (m *mockReactorForward) FlushRIBOut() int                                 { return 0 }
func (m *mockReactorForward) GetPeerAPIBindings(_ netip.Addr) []PeerAPIBinding { return nil }

// TestForwardCommand verifies forward parsing and execution.
//
// VALIDATES: Forward command works end-to-end.
// PREVENTS: Command parsing errors, forwarding failures.
func TestForwardCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantID  uint64
		wantSel string
		wantErr bool
	}{
		{
			name:    "forward to specific peer",
			input:   "peer 10.0.0.2 forward update-id 12345",
			wantID:  12345,
			wantSel: "10.0.0.2",
		},
		{
			name:    "forward to all except source",
			input:   "peer !10.0.0.1 forward update-id 12345",
			wantID:  12345,
			wantSel: "!10.0.0.1",
		},
		{
			name:    "forward to all peers",
			input:   "peer * forward update-id 12345",
			wantID:  12345,
			wantSel: "*",
		},
		{
			name:    "missing update-id",
			input:   "peer 10.0.0.1 forward update-id",
			wantErr: true,
		},
		{
			name:    "invalid update-id",
			input:   "peer 10.0.0.1 forward update-id abc",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockReactorForward{}
			d := NewDispatcher()
			RegisterDefaultHandlers(d)

			ctx := &CommandContext{
				Reactor: mock,
			}

			resp, err := d.Dispatch(ctx, tc.input)

			if tc.wantErr {
				if err == nil && (resp == nil || resp.Status != statusError) {
					t.Error("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !mock.forwardCalled {
				t.Error("ForwardUpdate not called")
			}

			if mock.forwardID != tc.wantID {
				t.Errorf("updateID = %d, want %d", mock.forwardID, tc.wantID)
			}

			if mock.forwardSel.String() != tc.wantSel {
				t.Errorf("selector = %s, want %s", mock.forwardSel.String(), tc.wantSel)
			}
		})
	}
}

// TestForwardExpiredID verifies error on expired ID.
//
// VALIDATES: Expired IDs return clear error.
// PREVENTS: Silent failures, undefined behavior.
func TestForwardExpiredID(t *testing.T) {
	mock := &mockReactorForward{
		forwardErr: errUpdateExpired,
	}
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ctx := &CommandContext{
		Reactor: mock,
	}

	resp, _ := d.Dispatch(ctx, "peer * forward update-id 99999")

	if resp.Status != statusError {
		t.Error("expected error status for expired ID")
	}
}
