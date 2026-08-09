// Design: docs/architecture/forked-route-install.md -- forked route install via Loc-RIB RPC
//
// The forked route-install/route-remove RPC handlers
// (internal/component/plugin/server/dispatch_route.go applyRouteInstall /
// applyRouteRemove) do exactly two things to the engine Loc-RIB:
// rib.InsertForward(fam, prefix, path, nil) and
// rib.Remove(fam, prefix, source, instance). This file drives that exact pair
// through the REAL locrib -> sysrib.run chain and asserts sysrib publishes an
// Add and then a Withdraw on (system-rib, best-change), which is what fib-kernel
// consumes to program and unprogram the kernel FIB.

package sysrib

import (
	"context"
	"net/netip"
	"testing"
	"time"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
)

// bestChangeEntries drains every (system-rib, best-change) batch the test bus
// saw and returns the flattened entries, so a test can assert on the sequence
// of actions sysrib published.
func bestChangeEntries(b *testEventBus) []sysribevents.BestChangeEntry {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []sysribevents.BestChangeEntry
	for i := range b.events {
		batch, ok := b.events[i].Payload.(*sysribevents.BestChangeBatch)
		if !ok {
			continue
		}
		out = append(out, batch.Changes...)
	}
	return out
}

// waitForEntry polls the bus until match finds an entry, or the deadline
// expires. sysrib's Loc-RIB worker is a goroutine, so publication is async.
func waitForEntry(t *testing.T, b *testEventBus, match func(sysribevents.BestChangeEntry) bool) (sysribevents.BestChangeEntry, bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		entries := bestChangeEntries(b)
		for i := range entries {
			if match(entries[i]) {
				return entries[i], true
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	return sysribevents.BestChangeEntry{}, false
}

// VALIDATES: a forked plugin's route-remove (applyRouteRemove -> locrib.Remove)
// makes sysrib publish a Withdraw on (system-rib, best-change) for the prefix,
// which is the event fib-kernel turns into a kernel route delete.
// PREVENTS: test/plugin/forked-route-install-kernel.ci's
// "route-remove: <prefix> still in kernel after withdrawal" -- a withdrawal that
// is applied to the Loc-RIB but never reaches the FIB as a Withdraw.
func TestForkedRouteRemovePublishesWithdraw(t *testing.T) {
	bus := newTestEventBus()
	setEventBus(bus)
	t.Cleanup(clearEventBus)

	rib := locrib.NewRIB()
	SetLocRIB(rib)
	t.Cleanup(func() { SetLocRIB(nil) })

	s := newSysRIB()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.run(ctx)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	// The .ci test installs protocol "bgp" (the engine always registers it);
	// RegisterProtocol is idempotent on name.
	bgpID := redistevents.RegisterProtocol("bgp")
	fam := family.IPv4Unicast
	pfx := netip.MustParsePrefix("10.99.0.0/24")
	nh := netip.MustParseAddr("192.0.2.1")

	// applyRouteInstall's Loc-RIB call, with the .ci test's field values.
	rib.InsertForward(fam, pfx, locrib.Path{
		Source:        bgpID,
		Instance:      0,
		NextHop:       nh,
		AdminDistance: 110,
		Metric:        10,
	}, nil)

	add, ok := waitForEntry(t, bus, func(e sysribevents.BestChangeEntry) bool {
		return e.Prefix == pfx && e.Action == routeaction.Add
	})
	if !ok {
		t.Fatalf("no Add published for %v; entries=%+v", pfx, bestChangeEntries(bus))
	}
	if add.NextHop != nh {
		t.Errorf("Add next-hop = %v, want %v", add.NextHop, nh)
	}

	// applyRouteRemove's Loc-RIB call.
	rib.Remove(fam, pfx, bgpID, 0)

	if _, ok := waitForEntry(t, bus, func(e sysribevents.BestChangeEntry) bool {
		return e.Prefix == pfx && e.Action.Verb() == routeaction.VerbRemove
	}); !ok {
		t.Fatalf("no Withdraw published for %v after locrib.Remove; entries=%+v", pfx, bestChangeEntries(bus))
	}
}
