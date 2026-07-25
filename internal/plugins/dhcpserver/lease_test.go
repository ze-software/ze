package dhcpserver

// VALIDATES: leaseTable expiry, re-add (timer extension), release, and
// lookup-by-address semantics. A lease is present before its TTL elapses and
// gone after; re-adding before expiry extends the lease with the new TTL;
// release removes the lease and frees the pool address immediately.
// PREVENTS: Regression where lease TTLs fail to expire, expire early, fail to
// extend on re-add, or leak pool addresses. Expiry timers are driven by an
// injected sim.FakeClock so AfterFunc callbacks fire on Add() instead of via
// wall-clock sleeps, keeping the suite deterministic and race-clean.

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/test/sim"
)

// newTestLeaseClock returns a sim.FakeClock: an Add-driven clock whose AfterFunc
// callbacks (the lease-expiry timers) fire synchronously in the caller's
// goroutine when fake time is advanced past their deadline. This lets lease
// expiry happen deterministically and instantly, with no wall-clock sleeping.
// Add() returns only after the expiry callback has fully applied, so a
// subsequent lookup observes the post-expiry state with no race.
func newTestLeaseClock() *sim.FakeClock {
	return sim.NewFakeClock(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
}

func TestLeaseExpiry(t *testing.T) {
	t.Parallel()

	clk := newTestLeaseClock()
	p := newPool([]addressRange{{Name: "pool", Start: netip.MustParseAddr("10.0.0.1"), Stop: netip.MustParseAddr("10.0.0.5")}}, nil)
	lt := newLeaseTable(p, clk)
	defer lt.stop()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	addr := netip.MustParseAddr("10.0.0.1")

	lt.add(mac, addr, 1)

	clk.Add(500 * time.Millisecond)
	if l := lt.lookup(mac); l == nil {
		t.Error("lease should still be active at 500ms")
	}

	clk.Add(700 * time.Millisecond)
	if l := lt.lookup(mac); l != nil {
		t.Error("lease should have expired after 1.2s")
	}

	_, allocated, _ := p.stats()
	if allocated != 0 {
		t.Errorf("expected pool freed after expiry, allocated=%d", allocated)
	}
}

func TestLeaseReAddExtends(t *testing.T) {
	t.Parallel()

	clk := newTestLeaseClock()
	p := newPool([]addressRange{{Name: "pool", Start: netip.MustParseAddr("10.0.0.1"), Stop: netip.MustParseAddr("10.0.0.5")}}, nil)
	lt := newLeaseTable(p, clk)
	defer lt.stop()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	addr := netip.MustParseAddr("10.0.0.1")

	lt.add(mac, addr, 1)

	clk.Add(600 * time.Millisecond)
	lt.add(mac, addr, 2)

	clk.Add(600 * time.Millisecond)
	if l := lt.lookup(mac); l == nil {
		t.Error("lease should still be active after re-add")
	}

	if l := lt.lookup(mac); l != nil && l.addr != addr {
		t.Errorf("re-added lease has wrong address: %v", l.addr)
	}
}

func TestLeaseRelease(t *testing.T) {
	t.Parallel()

	p := newPool([]addressRange{{Name: "pool", Start: netip.MustParseAddr("10.0.0.1"), Stop: netip.MustParseAddr("10.0.0.5")}}, nil)
	lt := newLeaseTable(p, newTestLeaseClock())
	defer lt.stop()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	addr := netip.MustParseAddr("10.0.0.1")

	lt.add(mac, addr, 3600)
	lt.release(mac)

	if l := lt.lookup(mac); l != nil {
		t.Error("lease should be gone after release")
	}

	_, allocated, _ := p.stats()
	if allocated != 0 {
		t.Errorf("expected pool freed after release, allocated=%d", allocated)
	}
}

func TestLeaseLookupByAddr(t *testing.T) {
	t.Parallel()

	p := newPool([]addressRange{{Name: "pool", Start: netip.MustParseAddr("10.0.0.1"), Stop: netip.MustParseAddr("10.0.0.5")}}, nil)
	lt := newLeaseTable(p, newTestLeaseClock())
	defer lt.stop()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	addr := netip.MustParseAddr("10.0.0.1")

	lt.add(mac, addr, 3600)

	l := lt.lookupByAddr(addr)
	if l == nil {
		t.Fatal("lookupByAddr returned nil")
	}
	if l.addr != addr {
		t.Errorf("lookupByAddr returned wrong address: %v", l.addr)
	}
}
