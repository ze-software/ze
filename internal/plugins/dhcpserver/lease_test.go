package dhcpserver

import (
	"net"
	"net/netip"
	"testing"
	"time"
)

func TestLeaseExpiry(t *testing.T) {
	t.Parallel()

	p := newPool([]addressRange{{Name: "pool", Start: netip.MustParseAddr("10.0.0.1"), Stop: netip.MustParseAddr("10.0.0.5")}}, nil)
	lt := newLeaseTable(p)
	defer lt.stop()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	addr := netip.MustParseAddr("10.0.0.1")

	lt.add(mac, addr, 1)

	time.Sleep(500 * time.Millisecond)
	if l := lt.lookup(mac); l == nil {
		t.Error("lease should still be active at 500ms")
	}

	time.Sleep(700 * time.Millisecond)
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

	p := newPool([]addressRange{{Name: "pool", Start: netip.MustParseAddr("10.0.0.1"), Stop: netip.MustParseAddr("10.0.0.5")}}, nil)
	lt := newLeaseTable(p)
	defer lt.stop()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	addr := netip.MustParseAddr("10.0.0.1")

	lt.add(mac, addr, 1)

	time.Sleep(600 * time.Millisecond)
	lt.add(mac, addr, 2)

	time.Sleep(600 * time.Millisecond)
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
	lt := newLeaseTable(p)
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
	lt := newLeaseTable(p)
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
