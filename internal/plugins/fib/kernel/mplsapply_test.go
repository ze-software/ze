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
