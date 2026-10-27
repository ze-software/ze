package rsvpte

import (
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: a panic in a call the engine makes while it holds lsp.mu leaves
// the lock free once the panic is recovered, and cleanupTick, which takes every
// LSP lock under engine.control, still completes.
// PREVENTS: one contained Ze defect stopping every RSVP session on the node,
// because cleanupTick blocks forever in expiredPSBs on the lock left held.
// METHOD: an LSP holding a reservation but no path state (a state only a Ze
// defect produces) makes handleResvErr read lsp.PSB through a nil pointer while
// it holds lsp.mu. The test recovers that panic as the signaling task's guard
// does, then asserts TryLock succeeds and cleanupTick returns within a bound.
func TestLSPLockReleasedAfterContainedPanic(t *testing.T) {
	e, _, _ := testEngine(t, "10.0.0.5", nil)
	key := lspKey{TunnelEndpoint: netip.MustParseAddr("10.0.0.9"), TunnelID: 1, ExtTunnelID: 0x0a000001,
		SenderAddr: netip.MustParseAddr("10.0.0.1"), LSPID: 1}
	prev := netip.MustParseAddr("10.0.0.2")
	lsp, _ := e.table.GetOrCreate(key)
	lsp.mu.Lock()
	lsp.Role, lsp.PrevHop = RoleTransit, prev
	lsp.RSB = &resvStateBlock{Style: StyleFixedFilter}
	lsp.mu.Unlock()

	msg := &ParsedMessage{
		Session: sessionIPv4{TunnelEndpoint: key.TunnelEndpoint, TunnelID: key.TunnelID, ExtTunnelID: key.ExtTunnelID},
		Style:   StyleFixedFilter,
		FlowDescriptors: []flowDescriptor{{Filters: []reservationFilter{
			{Filter: senderTemplateIPv4{SenderAddr: key.SenderAddr, LSPID: key.LSPID}},
		}}},
	}
	recovered := func() (value any) {
		defer func() { value = recover() }()
		e.handleResvErr(prev, msg)
		return nil
	}()
	if recovered == nil {
		t.Fatal("handleResvErr did not panic on an LSP with no path state: the test injected no panic")
	}

	if !lsp.mu.TryLock() {
		t.Fatal("lsp.mu is still held after the panic under it was recovered")
	}
	// Give the LSP live path state, so the cleanupTick below only has to take
	// its lock and finds nothing to expire.
	lsp.RSB = nil
	lsp.PSB = &pathStateBlock{LastRefresh: time.Now(), RefreshPeriod: DefaultRefreshPeriod}
	lsp.mu.Unlock()

	done := make(chan struct{})
	go func() {
		defer close(done)
		cleanupTick(slogutil.DiscardLogger(), e.table, e.cfg(), e, time.Now(), DefaultRefreshPeriod)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("cleanupTick did not complete: an LSP lock is still held")
	}
}
