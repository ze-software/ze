// Design: docs/architecture/mpls/mpls-kernel.md -- synchronous MPLS acceptance boundary.
package fibkernel

import (
	"errors"
	"net/netip"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/server"
	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/pkg/ze"
)

// newMPLSApplyBus connects Apply to the production event bus and FIB handler.
// No test subscriber supplies an acknowledgment on the forwarding owner's behalf.
func newMPLSApplyBus(t *testing.T, f *fibKernel) ze.EventBus {
	t.Helper()
	bus, err := server.NewServer(&server.ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if f != nil {
		t.Cleanup(mplsfibevents.EntryChange.Subscribe(bus, f.handleMPLSEntry))
	}
	return bus
}

// TestMPLSApplyRequiresOwner rejects delivery without a native forwarding owner.
func TestMPLSApplyRequiresOwner(t *testing.T) {
	entry := mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPush,
		FEC: netip.MustParsePrefix("192.0.2.1/32"), OutLabels: []uint32{100}}
	if err := mplsfibevents.Apply(newMPLSApplyBus(t, nil), []mplsfibevents.Entry{entry}); err == nil {
		t.Fatal("Apply accepted a batch without a forwarding owner")
	}
}

func TestMPLSRemoveLabelSourceRequiresOwner(t *testing.T) {
	if err := mplsfibevents.RemoveLabelSource(newMPLSApplyBus(t, nil), 7); err == nil {
		t.Fatal("source reset accepted without a native forwarding owner")
	}
}

type refusingMPLSPushBackend struct {
	*mplsMockBackend
	err error
}

func (b *refusingMPLSPushBackend) addRichRoute(_ RichRoute) error { return b.err }

// TestMPLSApplyPreservesBackendError requires the exact backend refusal to cross
// the real bus and acknowledgment boundary, including the EEXIST conflict case.
func TestMPLSApplyPreservesBackendError(t *testing.T) {
	refusal := syscall.EEXIST
	backend := &refusingMPLSPushBackend{mplsMockBackend: newMPLSMockBackend(), err: refusal}
	f := newFIBKernel(backend)
	entry := mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPush,
		FEC: netip.MustParsePrefix("192.0.2.1/32"), OutLabels: []uint32{100}}
	err := mplsfibevents.Apply(newMPLSApplyBus(t, f), []mplsfibevents.Entry{entry})
	if !errors.Is(err, refusal) {
		t.Fatalf("Apply error = %v, want backend EEXIST", err)
	}
}

// TestMPLSApplyRejectsScopedSwapPop prevents a table-qualified remove from
// deleting the global incoming-label entry, as well as rejecting such adds.
func TestMPLSApplyRejectsScopedSwapPop(t *testing.T) {
	for _, op := range []mplsfibevents.Op{mplsfibevents.OpSwap, mplsfibevents.OpPop} {
		for _, action := range []mplsfibevents.Action{mplsfibevents.ActionAdd, mplsfibevents.ActionRemove} {
			f := newFIBKernel(newMPLSMockBackend())
			entry := mplsfibevents.Entry{Action: action, Op: op, InLabel: 100,
				TableID: 0x5a000001, OutLabels: []uint32{200}}
			if err := mplsfibevents.Apply(newMPLSApplyBus(t, f), []mplsfibevents.Entry{entry}); err == nil {
				t.Fatalf("Apply accepted table-qualified operation %d action %d", op, action)
			}
		}
	}
}

// TestMPLSApplyRefusesUnsupportedContext prevents a rich IP-route backend from
// acknowledging a private route when it cannot install the packet-mark rule.
func TestMPLSApplyRefusesUnsupportedContext(t *testing.T) {
	f := newFIBKernel(newMPLSMockBackend())
	entry := mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPush,
		FEC: netip.MustParsePrefix("192.0.2.1/32"), TableID: 0x5a000001,
		NextHop: netip.MustParseAddr("198.51.100.1"), OutLabels: []uint32{100}}
	if err := mplsfibevents.Apply(newMPLSApplyBus(t, f), []mplsfibevents.Entry{entry}); err == nil {
		t.Fatal("Apply accepted a private context without packet-mark support")
	}
}

// TestHandleMPLSEntryIgnoresForeignReplay proves fib-kernel leaves a replay
// answer addressed to another writer alone: it programs nothing from it and
// does not acknowledge it, so Apply's answer belongs to the writer that asked.
//
// VALIDATES: AC-25 -- fib-kernel ignores a batch carrying a token it does not
// hold and does not acknowledge it; an incremental batch still programs.
// METHOD: the production bus with fib-kernel as the only writer; ApplyReplay
// with a fresh token must fail for want of an owner and leave the mock empty.
func TestHandleMPLSEntryIgnoresForeignReplay(t *testing.T) {
	mb := newMPLSMockBackend()
	f := newFIBKernel(mb)
	bus := newMPLSApplyBus(t, f)
	entry := mplsfibevents.Entry{Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpSwap,
		InLabel: 1000, OutLabels: []uint32{2000}, NextHop: netip.MustParseAddr("10.0.0.5"), Source: 2}

	if err := mplsfibevents.ApplyReplay(bus, mplsfibevents.NewReplayToken(), []mplsfibevents.Entry{entry}); err == nil {
		t.Fatal("fib-kernel acknowledged a replay batch it did not ask for")
	}
	if len(mb.swapsAdded) != 0 {
		t.Fatalf("fib-kernel programmed %d entries from a foreign replay", len(mb.swapsAdded))
	}
	if len(f.mplsSwaps) != 0 {
		t.Fatalf("fib-kernel recorded ownership from a foreign replay: %v", f.mplsSwaps)
	}

	if err := mplsfibevents.Apply(bus, []mplsfibevents.Entry{entry}); err != nil {
		t.Fatalf("incremental Apply after the foreign replay: %v", err)
	}
	if len(mb.swapsAdded) != 1 {
		t.Fatalf("incremental batch programmed %d entries, want 1", len(mb.swapsAdded))
	}
}

// TestHandleMPLSEntryAppliesSharedValidation proves fib-kernel refuses, at
// its subscription, every entry the shared mplsfib.ValidateEntry refuses, and
// programs nothing for it: the rule lives once, in the core package.
//
// VALIDATES: AC-23 -- kernel half: in-label above 2^20-1, empty swap stack, a
// stack deeper than MaxLabelStack, a pop carrying an out-label, a push label
// above 2^20-1.
func TestHandleMPLSEntryAppliesSharedValidation(t *testing.T) {
	deep := make([]uint32, mplsfibevents.MaxLabelStack+1)
	for i := range deep {
		deep[i] = uint32(100 + i)
	}
	nh := netip.MustParseAddr("10.0.0.5")
	cases := map[string]mplsfibevents.Entry{
		"in-label above 20 bits": {Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpSwap,
			InLabel: mplsfibevents.MaxLabel + 1, OutLabels: []uint32{200}, NextHop: nh},
		"empty swap stack": {Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpSwap,
			InLabel: 100, NextHop: nh},
		"stack too deep": {Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpSwap,
			InLabel: 100, OutLabels: deep, NextHop: nh},
		"pop with out-label": {Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPop,
			InLabel: 100, OutLabels: []uint32{200}, NextHop: nh},
		"push label above 20 bits": {Action: mplsfibevents.ActionAdd, Op: mplsfibevents.OpPush,
			FEC: netip.MustParsePrefix("192.0.2.1/32"), OutLabels: []uint32{mplsfibevents.MaxLabel + 1}, NextHop: nh},
	}
	for name, entry := range cases {
		t.Run(name, func(t *testing.T) {
			mb := newMPLSMockBackend()
			f := newFIBKernel(mb)
			if err := mplsfibevents.Apply(newMPLSApplyBus(t, f), []mplsfibevents.Entry{entry}); err == nil {
				t.Fatal("fib-kernel accepted an entry the shared validation refuses")
			}
			if len(mb.swapsAdded) != 0 || len(f.mplsSwaps) != 0 || len(f.mplsInstalled) != 0 {
				t.Fatal("fib-kernel programmed a refused entry")
			}
		})
	}
}
