package ospf

import (
	"net/netip"
	"testing"

	mplsfibevents "github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/plugins/ospf/sr"
)

type srCaptureBus struct {
	entries []mplsfibevents.Entry
}

func (c *srCaptureBus) Emit(namespace, eventType string, payload any) (int, error) {
	if batch, ok := payload.(*mplsfibevents.EntryBatch); ok {
		c.entries = append(c.entries, batch.Entries...)
	}
	return 0, nil
}

func (c *srCaptureBus) Subscribe(_, _ string, _ func(any)) func() { return func() {} }

func (c *srCaptureBus) last() mplsfibevents.Entry {
	return c.entries[len(c.entries)-1]
}

func TestSRFIBProgramPush(t *testing.T) {
	bus := &srCaptureBus{}
	f := newSRFIB(bus, mplsSourceOSPFSR)
	fec := netip.MustParsePrefix("10.0.0.9/32")
	nh := netip.MustParseAddr("10.0.0.2")
	f.programPush(fec, 16009, nh)
	e := bus.last()
	if e.Action != mplsfibevents.ActionAdd || e.Op != mplsfibevents.OpPush {
		t.Fatalf("push action/op = %v/%v", e.Action, e.Op)
	}
	if e.Source != mplsSourceOSPFSR {
		t.Fatalf("source = %d want %d", e.Source, mplsSourceOSPFSR)
	}
	if len(e.OutLabels) != 1 || e.OutLabels[0] != 16009 || e.NextHop != nh || e.FEC != fec {
		t.Fatalf("push entry = %+v", e)
	}
}

func TestSRFIBInstallPrefixSIDPush(t *testing.T) {
	bus := &srCaptureBus{}
	f := newSRFIB(bus, mplsSourceOSPFSR)
	fec := netip.MustParsePrefix("10.0.0.9/32")
	nh := netip.MustParseAddr("10.0.0.2")
	// ActionKeep (NP=1, E=0 penultimate, or a transit swap): ingress push of outLabel,
	// transit swap. The caller resolves the action; the FIB just programs it.
	f.installPrefixSID(fec, sr.ActionKeep, 16509, true, 16009, sr.ExplicitNullV4, nh)
	var push, swap *mplsfibevents.Entry
	for i := range bus.entries {
		switch bus.entries[i].Op {
		case mplsfibevents.OpPush:
			push = &bus.entries[i]
		case mplsfibevents.OpSwap:
			swap = &bus.entries[i]
		default:
		}
	}
	if push == nil || push.OutLabels[0] != 16009 {
		t.Fatalf("expected ingress push of 16009, got %+v", push)
	}
	if swap == nil || swap.InLabel != 16509 || swap.OutLabels[0] != 16009 {
		t.Fatalf("expected transit swap 16509->16009, got %+v", swap)
	}
}

func TestSRFIBPHPBehavior(t *testing.T) {
	bus := &srCaptureBus{}
	f := newSRFIB(bus, mplsSourceOSPFSR)
	fec := netip.MustParsePrefix("10.0.0.9/32")
	nh := netip.MustParseAddr("10.0.0.2")
	// ActionPHP (NP=0 at the penultimate hop): no ingress push, transit pop.
	f.installPrefixSID(fec, sr.ActionPHP, 16509, true, 16009, sr.ExplicitNullV4, nh)
	var sawPush, sawPop bool
	for _, e := range bus.entries {
		if e.Op == mplsfibevents.OpPush && e.Action == mplsfibevents.ActionAdd {
			sawPush = true
		}
		if e.Op == mplsfibevents.OpPop {
			sawPop = true
			if e.InLabel != 16509 {
				t.Fatalf("PHP pop InLabel = %d want 16509", e.InLabel)
			}
		}
	}
	if sawPush {
		t.Fatalf("PHP must not install an ingress push")
	}
	if !sawPop {
		t.Fatalf("PHP must install a transit pop")
	}
}

func TestSRFIBExplicitNull(t *testing.T) {
	bus := &srCaptureBus{}
	f := newSRFIB(bus, mplsSourceOSPFv3SR)
	fec := netip.MustParsePrefix("10.0.0.9/32")
	nh := netip.MustParseAddr("10.0.0.2")
	// ActionExplicitNull (NP=1,E=1 at the penultimate hop): Explicit NULL (IPv6 = 2).
	f.installPrefixSID(fec, sr.ActionExplicitNull, 16509, true, 16009, sr.ExplicitNullV6, nh)
	var push *mplsfibevents.Entry
	for i := range bus.entries {
		if bus.entries[i].Op == mplsfibevents.OpPush {
			push = &bus.entries[i]
		}
	}
	if push == nil || push.OutLabels[0] != 2 {
		t.Fatalf("expected ingress push of Explicit NULL 2, got %+v", push)
	}
}

func TestSRFIBInstallAdjSIDPop(t *testing.T) {
	bus := &srCaptureBus{}
	f := newSRFIB(bus, mplsSourceOSPFSR)
	nh := netip.MustParseAddr("10.0.0.2")
	f.installAdjSID(40001, nh)
	e := bus.last()
	if e.Op != mplsfibevents.OpPop || e.InLabel != 40001 || e.NextHop != nh {
		t.Fatalf("adj-SID pop entry = %+v", e)
	}
}

func TestSRFIBWithdrawSwapAndAdjSID(t *testing.T) {
	bus := &srCaptureBus{}
	f := newSRFIB(bus, mplsSourceOSPFSR)
	nh := netip.MustParseAddr("10.0.0.2")
	// removeSwap of an uninstalled label is a no-op; after a swap it emits once.
	f.removeSwap(16509)
	if len(bus.entries) != 0 {
		t.Fatalf("removeSwap of uninstalled label must be a no-op")
	}
	f.programSwap(16509, 16009, nh)
	f.removeSwap(16509)
	if e := bus.last(); e.Action != mplsfibevents.ActionRemove || e.Op != mplsfibevents.OpSwap || e.InLabel != 16509 {
		t.Fatalf("removeSwap entry = %+v", e)
	}
	// Adj-SID lifecycle: install pop then withdraw it (RFC 8665 §7.4.1).
	f.installAdjSID(40001, nh)
	f.withdrawAdjSID(40001)
	if e := bus.last(); e.Action != mplsfibevents.ActionRemove || e.Op != mplsfibevents.OpPop || e.InLabel != 40001 {
		t.Fatalf("withdrawAdjSID entry = %+v", e)
	}
}

func TestSRFIBIdempotentRemoval(t *testing.T) {
	bus := &srCaptureBus{}
	f := newSRFIB(bus, mplsSourceOSPFSR)
	fec := netip.MustParsePrefix("10.0.0.9/32")
	// Removing a never-pushed FEC emits nothing.
	f.removePush(fec)
	if len(bus.entries) != 0 {
		t.Fatalf("removing a never-pushed FEC must be a no-op")
	}
	f.programPush(fec, 16009, netip.MustParseAddr("10.0.0.2"))
	n := len(bus.entries)
	f.removePush(fec)
	if len(bus.entries) != n+1 {
		t.Fatalf("removePush of a pushed FEC must emit exactly one removal")
	}
	// Second removal is a no-op.
	f.removePush(fec)
	if len(bus.entries) != n+1 {
		t.Fatalf("double removePush must be idempotent")
	}
}
