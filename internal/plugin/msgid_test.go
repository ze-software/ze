package plugin

import (
	"errors"
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/zebgp/internal/bgp/nlri"
	"codeberg.org/thomas-mangin/zebgp/internal/rib"
	"codeberg.org/thomas-mangin/zebgp/internal/selector"
)

// mockReactorMsgID implements ReactorInterface for msg-id command testing.
type mockReactorMsgID struct {
	retainCalled  bool
	retainID      uint64
	retainErr     error
	releaseCalled bool
	releaseID     uint64
	releaseErr    error
	expireCalled  bool
	expireID      uint64
	expireErr     error
	listCalled    bool
	listResult    []uint64
}

func (m *mockReactorMsgID) RetainUpdate(id uint64) error {
	m.retainCalled = true
	m.retainID = id
	return m.retainErr
}

func (m *mockReactorMsgID) ReleaseUpdate(id uint64) error {
	m.releaseCalled = true
	m.releaseID = id
	return m.releaseErr
}

func (m *mockReactorMsgID) DeleteUpdate(id uint64) error {
	m.expireCalled = true
	m.expireID = id
	return m.expireErr
}

func (m *mockReactorMsgID) ListUpdates() []uint64 {
	m.listCalled = true
	return m.listResult
}

// Implement all other ReactorInterface methods as stubs.
func (m *mockReactorMsgID) ForwardUpdate(_ *selector.Selector, _ uint64) error { return nil }
func (m *mockReactorMsgID) Peers() []PeerInfo                                  { return nil }
func (m *mockReactorMsgID) Stats() ReactorStats                                { return ReactorStats{} }
func (m *mockReactorMsgID) Stop()                                              {}
func (m *mockReactorMsgID) Reload() error                                      { return nil }
func (m *mockReactorMsgID) AnnounceRoute(_ string, _ RouteSpec) error          { return nil }
func (m *mockReactorMsgID) WithdrawRoute(_ string, _ netip.Prefix) error       { return nil }
func (m *mockReactorMsgID) AnnounceFlowSpec(_ string, _ FlowSpecRoute) error   { return nil }
func (m *mockReactorMsgID) WithdrawFlowSpec(_ string, _ FlowSpecRoute) error   { return nil }
func (m *mockReactorMsgID) AnnounceVPLS(_ string, _ VPLSRoute) error           { return nil }
func (m *mockReactorMsgID) WithdrawVPLS(_ string, _ VPLSRoute) error           { return nil }
func (m *mockReactorMsgID) AnnounceL2VPN(_ string, _ L2VPNRoute) error         { return nil }
func (m *mockReactorMsgID) WithdrawL2VPN(_ string, _ L2VPNRoute) error         { return nil }
func (m *mockReactorMsgID) AnnounceL3VPN(_ string, _ L3VPNRoute) error         { return nil }
func (m *mockReactorMsgID) WithdrawL3VPN(_ string, _ L3VPNRoute) error         { return nil }
func (m *mockReactorMsgID) AnnounceLabeledUnicast(_ string, _ LabeledUnicastRoute) error {
	return nil
}
func (m *mockReactorMsgID) WithdrawLabeledUnicast(_ string, _ LabeledUnicastRoute) error {
	return nil
}
func (m *mockReactorMsgID) AnnounceMUPRoute(_ string, _ MUPRouteSpec) error { return nil }
func (m *mockReactorMsgID) WithdrawMUPRoute(_ string, _ MUPRouteSpec) error { return nil }
func (m *mockReactorMsgID) TeardownPeer(_ netip.Addr, _ uint8) error        { return nil }
func (m *mockReactorMsgID) AnnounceEOR(_ string, _ uint16, _ uint8) error   { return nil }
func (m *mockReactorMsgID) RIBInRoutes(_ string) []rib.RouteJSON            { return nil }
func (m *mockReactorMsgID) RIBOutRoutes() []rib.RouteJSON                   { return nil }
func (m *mockReactorMsgID) RIBStats() RIBStatsInfo                          { return RIBStatsInfo{} }
func (m *mockReactorMsgID) BeginTransaction(_, _ string) error              { return nil }
func (m *mockReactorMsgID) CommitTransaction(_ string) (TransactionResult, error) {
	return TransactionResult{}, nil
}
func (m *mockReactorMsgID) CommitTransactionWithLabel(_, _ string) (TransactionResult, error) {
	return TransactionResult{}, nil
}
func (m *mockReactorMsgID) RollbackTransaction(_ string) (TransactionResult, error) {
	return TransactionResult{}, nil
}
func (m *mockReactorMsgID) InTransaction(_ string) bool   { return false }
func (m *mockReactorMsgID) TransactionID(_ string) string { return "" }
func (m *mockReactorMsgID) SendRoutes(_ string, _ []*rib.Route, _ []nlri.NLRI, _ bool) (TransactionResult, error) {
	return TransactionResult{}, nil
}
func (m *mockReactorMsgID) AnnounceWatchdog(_, _ string) error                       { return nil }
func (m *mockReactorMsgID) WithdrawWatchdog(_, _ string) error                       { return nil }
func (m *mockReactorMsgID) AddWatchdogRoute(_ RouteSpec, _ string) error             { return nil }
func (m *mockReactorMsgID) RemoveWatchdogRoute(_, _ string) error                    { return nil }
func (m *mockReactorMsgID) ClearRIBIn() int                                          { return 0 }
func (m *mockReactorMsgID) ClearRIBOut() int                                         { return 0 }
func (m *mockReactorMsgID) FlushRIBOut() int                                         { return 0 }
func (m *mockReactorMsgID) GetPeerProcessBindings(_ netip.Addr) []PeerProcessBinding { return nil }
func (m *mockReactorMsgID) GetPeerCapabilityConfigs() []PeerCapabilityConfig         { return nil }
func (m *mockReactorMsgID) SignalAPIReady()                                          {}
func (m *mockReactorMsgID) SignalPeerAPIReady(_ string)                              {}
func (m *mockReactorMsgID) AnnounceNLRIBatch(_ string, _ NLRIBatch) error            { return nil }
func (m *mockReactorMsgID) WithdrawNLRIBatch(_ string, _ NLRIBatch) error            { return nil }
func (m *mockReactorMsgID) SendRawMessage(_ netip.Addr, _ uint8, _ []byte) error     { return nil }
func (m *mockReactorMsgID) SendBoRR(_ string, _ uint16, _ uint8) error               { return nil }
func (m *mockReactorMsgID) SendEoRR(_ string, _ uint16, _ uint8) error               { return nil }

// TestMsgIDRetain verifies msg-id retain command.
//
// VALIDATES: Retain command parses ID and calls RetainUpdate.
// PREVENTS: Command parsing errors, missed calls.
func TestMsgIDRetain(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantID  uint64
		wantErr bool
	}{
		{
			name:   "valid retain",
			input:  "msg-id retain 12345",
			wantID: 12345,
		},
		{
			name:    "missing id",
			input:   "msg-id retain",
			wantErr: true,
		},
		{
			name:    "invalid id",
			input:   "msg-id retain abc",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockReactorMsgID{}
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

			if !mock.retainCalled {
				t.Error("RetainUpdate not called")
			}

			if mock.retainID != tc.wantID {
				t.Errorf("retainID = %d, want %d", mock.retainID, tc.wantID)
			}
		})
	}
}

// TestMsgIDRelease verifies msg-id release command.
//
// VALIDATES: Release command parses ID and calls ReleaseUpdate.
// PREVENTS: Command parsing errors, missed calls.
func TestMsgIDRelease(t *testing.T) {
	mock := &mockReactorMsgID{}
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ctx := &CommandContext{
		Reactor: mock,
	}

	resp, err := d.Dispatch(ctx, "msg-id release 99999")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Status != statusDone {
		t.Errorf("status = %s, want done", resp.Status)
	}

	if !mock.releaseCalled {
		t.Error("ReleaseUpdate not called")
	}

	if mock.releaseID != 99999 {
		t.Errorf("releaseID = %d, want 99999", mock.releaseID)
	}
}

// TestMsgIDExpire verifies msg-id expire command.
//
// VALIDATES: Expire command parses ID and calls DeleteUpdate.
// PREVENTS: Command parsing errors, missed calls.
func TestMsgIDExpire(t *testing.T) {
	mock := &mockReactorMsgID{}
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ctx := &CommandContext{
		Reactor: mock,
	}

	resp, err := d.Dispatch(ctx, "msg-id expire 55555")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Status != statusDone {
		t.Errorf("status = %s, want done", resp.Status)
	}

	if !mock.expireCalled {
		t.Error("DeleteUpdate not called")
	}

	if mock.expireID != 55555 {
		t.Errorf("expireID = %d, want 55555", mock.expireID)
	}
}

// TestMsgIDList verifies msg-id list command.
//
// VALIDATES: List command returns all cached IDs.
// PREVENTS: Missing IDs, wrong response format.
func TestMsgIDList(t *testing.T) {
	mock := &mockReactorMsgID{
		listResult: []uint64{100, 200, 300},
	}
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ctx := &CommandContext{
		Reactor: mock,
	}

	resp, err := d.Dispatch(ctx, "msg-id list")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Status != statusDone {
		t.Errorf("status = %s, want done", resp.Status)
	}

	if !mock.listCalled {
		t.Error("ListUpdates not called")
	}

	// Check response data contains IDs
	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("data is not map[string]any: %T", resp.Data)
	}

	ids, ok := data["msg_ids"].([]uint64)
	if !ok {
		t.Fatalf("msg_ids is not []uint64: %T", data["msg_ids"])
	}

	if len(ids) != 3 {
		t.Errorf("len(ids) = %d, want 3", len(ids))
	}
}

// TestMsgIDRetainError verifies error handling on retain failure.
//
// VALIDATES: Errors from RetainUpdate propagate correctly.
// PREVENTS: Silent failures, wrong error messages.
func TestMsgIDRetainError(t *testing.T) {
	mock := &mockReactorMsgID{
		retainErr: errors.New("update-id expired or not found"),
	}
	d := NewDispatcher()
	RegisterDefaultHandlers(d)

	ctx := &CommandContext{
		Reactor: mock,
	}

	resp, _ := d.Dispatch(ctx, "msg-id retain 99999")

	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if resp.Status != statusError {
		t.Error("expected error status for expired ID")
	}
}

// TestMsgIDUsageMessages verifies error messages show correct syntax.
//
// VALIDATES: Error messages match actual command syntax.
// PREVENTS: Misleading usage hints (regression of syntax mismatch).
func TestMsgIDUsageMessages(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantUsage string
	}{
		{"retain", "msg-id retain", "msg-id retain <id>"},
		{"release", "msg-id release", "msg-id release <id>"},
		{"expire", "msg-id expire", "msg-id expire <id>"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockReactorMsgID{}
			d := NewDispatcher()
			RegisterDefaultHandlers(d)

			ctx := &CommandContext{
				Reactor: mock,
			}

			resp, _ := d.Dispatch(ctx, tc.input)

			if resp == nil {
				t.Fatal("expected response, got nil")
			}

			data, ok := resp.Data.(string)
			if !ok {
				t.Fatalf("expected string data, got %T", resp.Data)
			}

			if data != "usage: "+tc.wantUsage {
				t.Errorf("usage = %q, want %q", data, "usage: "+tc.wantUsage)
			}
		})
	}
}
