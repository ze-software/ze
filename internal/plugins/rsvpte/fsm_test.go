package rsvpte

import (
	"net/netip"
	"testing"
	"time"
)

func TestRSVPSessionFSM(t *testing.T) {
	table := newLSPTable()

	key := lspKey{
		TunnelEndpoint: netip.MustParseAddr("10.0.0.2"),
		TunnelID:       100,
		ExtTunnelID:    0x0a000001,
		SenderAddr:     netip.MustParseAddr("10.0.0.1"),
		LSPID:          1,
	}

	lsp, existed := table.GetOrCreate(key)
	if existed {
		t.Fatal("LSP should not pre-exist")
	}
	if lsp.State != LSPStateDown {
		t.Errorf("initial state = %s, want down", lsp.State)
	}

	lsp.setState(LSPStatePathSent)
	if lsp.State != LSPStatePathSent {
		t.Errorf("state = %s, want path-sent", lsp.State)
	}

	lsp.setState(LSPStateResvReceived)
	if lsp.State != LSPStateResvReceived {
		t.Errorf("state = %s, want resv-received", lsp.State)
	}

	lsp.setState(LSPStateUp)
	if lsp.State != LSPStateUp {
		t.Errorf("state = %s, want up", lsp.State)
	}

	lsp2, existed2 := table.GetOrCreate(key)
	if !existed2 {
		t.Fatal("LSP should exist on second GetOrCreate")
	}
	if lsp2 != lsp {
		t.Fatal("GetOrCreate returned different LSP pointer")
	}
}

func TestLSPTableAllocateLabel(t *testing.T) {
	table := newLSPTable()

	l1 := table.AllocateLabel()
	l2 := table.AllocateLabel()
	if l2 <= l1 {
		t.Errorf("labels not monotonic: %d then %d", l1, l2)
	}
}

// TestLSPTableAllocateSkipsReservedLabels checks the dynamic label allocator never
// hands out a reserved label (0-15): it starts at firstDynamicLabel (1000) and every
// label it returns stays at or above that floor, across the wrap at MaxLabel.
//
// The second half drives the wrap on purpose. A fresh table walks 1000, 1001, ...
// and 100 allocations never reach MaxLabel, so the floor check alone passes with the
// wrap target set to any value at all, including a reserved one.
func TestLSPTableAllocateSkipsReservedLabels(t *testing.T) {
	// RFC requirement: RFC3209-4.1-3 positive -- allocated labels start at firstDynamicLabel (1000, fsm.go:184/205) and stay >= it (wrap resets to firstDynamicLabel, fsm.go:215-217), so the reserved 0-15 label range is never allocated.
	table := newLSPTable()
	for range 100 {
		l := table.AllocateLabel()
		if l < firstDynamicLabel {
			t.Fatalf("AllocateLabel returned %d, want >= %d (reserved labels 0-15 must never be allocated)", l, firstDynamicLabel)
		}
	}

	wrapping := newLSPTable()
	wrapping.nextLabel = MaxLabel

	last := wrapping.AllocateLabel()
	if last != MaxLabel {
		t.Fatalf("the allocation before the wrap returned %d, want %d (MaxLabel)", last, MaxLabel)
	}

	afterWrap := wrapping.AllocateLabel()
	if afterWrap != firstDynamicLabel {
		t.Fatalf("the allocation after the wrap returned %d, want %d (firstDynamicLabel)", afterWrap, firstDynamicLabel)
	}
	for range 100 {
		l := wrapping.AllocateLabel()
		if l < firstDynamicLabel {
			t.Fatalf("AllocateLabel returned %d after the wrap, want >= %d (reserved labels 0-15 must never be allocated)", l, firstDynamicLabel)
		}
	}
}

func TestLSPTableRemove(t *testing.T) {
	table := newLSPTable()
	key := lspKey{
		TunnelEndpoint: netip.MustParseAddr("10.0.0.2"),
		TunnelID:       1,
		SenderAddr:     netip.MustParseAddr("10.0.0.1"),
		LSPID:          1,
	}

	table.GetOrCreate(key)
	if table.Len() != 1 {
		t.Fatalf("Len = %d, want 1", table.Len())
	}

	removed := table.Remove(key)
	if removed == nil {
		t.Fatal("Remove returned nil")
	}
	if table.Len() != 0 {
		t.Fatalf("Len = %d after remove, want 0", table.Len())
	}

	_, ok := table.Get(key)
	if ok {
		t.Fatal("Get succeeded after Remove")
	}
}

func TestLSPTableAll(t *testing.T) {
	table := newLSPTable()
	for i := range uint16(5) {
		table.GetOrCreate(lspKey{
			TunnelEndpoint: netip.MustParseAddr("10.0.0.2"),
			TunnelID:       i,
			SenderAddr:     netip.MustParseAddr("10.0.0.1"),
			LSPID:          i,
		})
	}
	all := table.All()
	if len(all) != 5 {
		t.Fatalf("All returned %d LSPs, want 5", len(all))
	}
}

func TestLSPTableExpiredPSBs(t *testing.T) {
	table := newLSPTable()
	key := lspKey{
		TunnelEndpoint: netip.MustParseAddr("10.0.0.2"),
		TunnelID:       1,
		SenderAddr:     netip.MustParseAddr("10.0.0.1"),
		LSPID:          1,
	}

	lsp, _ := table.GetOrCreate(key)
	lsp.PSB = &pathStateBlock{
		RefreshPeriod: 30 * time.Second,
		LastRefresh:   time.Now().Add(-120 * time.Second),
	}

	expired := table.expiredPSBs(time.Now(), 3)
	if len(expired) != 1 {
		t.Fatalf("ExpiredPSBs returned %d, want 1", len(expired))
	}
	if expired[0] != key {
		t.Errorf("expired key mismatch")
	}

	lsp.PSB.LastRefresh = time.Now()
	expired = table.expiredPSBs(time.Now(), 3)
	if len(expired) != 0 {
		t.Fatalf("ExpiredPSBs returned %d for fresh PSB, want 0", len(expired))
	}
}

func TestKeyFromMessage(t *testing.T) {
	msg := &ParsedMessage{
		Session: sessionIPv4{
			TunnelEndpoint: netip.MustParseAddr("10.0.0.2"),
			TunnelID:       100,
			ExtTunnelID:    0x0a000001,
		},
		SenderTemplate: senderTemplateIPv4{
			SenderAddr: netip.MustParseAddr("10.0.0.1"),
			LSPID:      1,
		},
	}

	key := keyFromMessage(msg)
	if key.TunnelEndpoint != msg.Session.TunnelEndpoint {
		t.Errorf("TunnelEndpoint mismatch")
	}
	if key.TunnelID != msg.Session.TunnelID {
		t.Errorf("TunnelID mismatch")
	}
	if key.SenderAddr != msg.SenderTemplate.SenderAddr {
		t.Errorf("SenderAddr mismatch")
	}
	if key.LSPID != msg.SenderTemplate.LSPID {
		t.Errorf("LSPID mismatch")
	}
}

func TestLSPStateString(t *testing.T) {
	tests := []struct {
		state lspState
		want  string
	}{
		{LSPStateDown, "down"},
		{LSPStatePathSent, "path-sent"},
		{LSPStatePathReceived, "path-received"},
		{LSPStateResvSent, "resv-sent"},
		{LSPStateResvReceived, "resv-received"},
		{LSPStateUp, "up"},
		{lspState(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("LSPState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestLSPRoleString(t *testing.T) {
	tests := []struct {
		role lspRole
		want string
	}{
		{RoleIngress, "ingress"},
		{RoleTransit, "transit"},
		{RoleEgress, "egress"},
		{lspRole(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.role.String(); got != tt.want {
			t.Errorf("LSPRole(%d).String() = %q, want %q", tt.role, got, tt.want)
		}
	}
}
