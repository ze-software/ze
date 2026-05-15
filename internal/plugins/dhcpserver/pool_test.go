package dhcpserver

import (
	"net"
	"net/netip"
	"testing"
)

func TestPoolAllocate(t *testing.T) {
	t.Parallel()

	p := newPool(
		netip.MustParseAddr("192.168.1.100"),
		netip.MustParseAddr("192.168.1.105"),
		nil,
	)

	seen := make(map[netip.Addr]bool)
	for range 6 {
		addr, ok := p.allocate(nil)
		if !ok {
			t.Fatal("allocate failed before pool exhaustion")
		}
		if seen[addr] {
			t.Fatalf("duplicate allocation: %v", addr)
		}
		seen[addr] = true
	}

	if len(seen) != 6 {
		t.Errorf("expected 6 allocations, got %d", len(seen))
	}

	start := netip.MustParseAddr("192.168.1.100")
	stop := netip.MustParseAddr("192.168.1.105")
	for addr := range seen {
		if addrToUint32(addr) < addrToUint32(start) || addrToUint32(addr) > addrToUint32(stop) {
			t.Errorf("allocated address %v outside range", addr)
		}
	}
}

func TestPoolAllocateSkipsStaticMappings(t *testing.T) {
	t.Parallel()

	staticIP := netip.MustParseAddr("192.168.1.101")
	statics := []staticMapping{
		{Name: "reserved", MAC: net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}, IP: staticIP},
	}

	p := newPool(
		netip.MustParseAddr("192.168.1.100"),
		netip.MustParseAddr("192.168.1.102"),
		statics,
	)

	for range 2 {
		addr, ok := p.allocate(nil)
		if !ok {
			t.Fatal("allocate failed")
		}
		if addr == staticIP {
			t.Errorf("allocated static IP %v from dynamic pool", staticIP)
		}
	}

	_, ok := p.allocate(nil)
	if ok {
		t.Error("expected pool exhaustion after allocating non-static IPs")
	}
}

func TestPoolExhaustion(t *testing.T) {
	t.Parallel()

	p := newPool(
		netip.MustParseAddr("10.0.0.1"),
		netip.MustParseAddr("10.0.0.3"),
		nil,
	)

	for range 3 {
		_, ok := p.allocate(nil)
		if !ok {
			t.Fatal("allocate failed before exhaustion")
		}
	}

	_, ok := p.allocate(nil)
	if ok {
		t.Error("expected pool exhaustion")
	}
}

func TestPoolRelease(t *testing.T) {
	t.Parallel()

	p := newPool(
		netip.MustParseAddr("10.0.0.1"),
		netip.MustParseAddr("10.0.0.1"),
		nil,
	)

	addr, ok := p.allocate(nil)
	if !ok {
		t.Fatal("first allocate failed")
	}

	_, ok = p.allocate(nil)
	if ok {
		t.Error("expected exhaustion on second allocate")
	}

	p.release(addr)

	addr2, ok := p.allocate(nil)
	if !ok {
		t.Fatal("allocate after release failed")
	}
	if addr2 != addr {
		t.Errorf("expected %v after release, got %v", addr, addr2)
	}
}

func TestPoolAllocateForMAC(t *testing.T) {
	t.Parallel()

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}

	p := newPool(
		netip.MustParseAddr("10.0.0.1"),
		netip.MustParseAddr("10.0.0.5"),
		nil,
	)

	addr1, ok := p.allocate(mac)
	if !ok {
		t.Fatal("first allocate failed")
	}

	addr2, ok := p.allocate(mac)
	if !ok {
		t.Fatal("second allocate for same MAC failed")
	}
	if addr2 != addr1 {
		t.Errorf("same MAC got different addresses: %v vs %v", addr1, addr2)
	}
}

func TestPoolReleaseOutOfRange(t *testing.T) {
	t.Parallel()

	p := newPool(
		netip.MustParseAddr("10.0.0.1"),
		netip.MustParseAddr("10.0.0.3"),
		nil,
	)

	p.release(netip.MustParseAddr("10.0.0.100"))

	total, allocated, available := p.stats()
	if total != 3 || allocated != 0 || available != 3 {
		t.Errorf("stats after bad release: total=%d alloc=%d avail=%d", total, allocated, available)
	}
}

func TestPoolStats(t *testing.T) {
	t.Parallel()

	p := newPool(
		netip.MustParseAddr("10.0.0.1"),
		netip.MustParseAddr("10.0.0.10"),
		nil,
	)

	total, allocated, available := p.stats()
	if total != 10 || allocated != 0 || available != 10 {
		t.Errorf("initial stats: total=%d alloc=%d avail=%d", total, allocated, available)
	}

	addr, _ := p.allocate(nil)
	total, allocated, available = p.stats()
	if total != 10 || allocated != 1 || available != 9 {
		t.Errorf("after alloc stats: total=%d alloc=%d avail=%d", total, allocated, available)
	}

	p.release(addr)
	total, allocated, available = p.stats()
	if total != 10 || allocated != 0 || available != 10 {
		t.Errorf("after release stats: total=%d alloc=%d avail=%d", total, allocated, available)
	}
}

func TestPoolReserve(t *testing.T) {
	t.Parallel()

	p := newPool(
		netip.MustParseAddr("10.0.0.1"),
		netip.MustParseAddr("10.0.0.5"),
		nil,
	)

	mac := net.HardwareAddr{0x11, 0x22, 0x33, 0x44, 0x55, 0x66}
	addr := netip.MustParseAddr("10.0.0.3")

	ok := p.reserve(addr, mac)
	if !ok {
		t.Error("reserve in-range address should succeed")
	}
	_, allocated, _ := p.stats()
	if allocated != 1 {
		t.Errorf("allocated = %d after reserve, want 1", allocated)
	}

	ok = p.reserve(addr, mac)
	if !ok {
		t.Error("re-reserve same address should succeed")
	}
	_, allocated, _ = p.stats()
	if allocated != 1 {
		t.Errorf("allocated = %d after re-reserve, want 1", allocated)
	}

	outOfRange := netip.MustParseAddr("10.0.0.100")
	ok = p.reserve(outOfRange, mac)
	if ok {
		t.Error("reserve out-of-range should fail")
	}

	emptyPool := newPool(netip.Addr{}, netip.Addr{}, nil)
	ok = emptyPool.reserve(addr, mac)
	if ok {
		t.Error("reserve on empty pool should fail")
	}
}

func TestPoolMarkUnavailable(t *testing.T) {
	t.Parallel()

	p := newPool(
		netip.MustParseAddr("10.0.0.1"),
		netip.MustParseAddr("10.0.0.3"),
		nil,
	)

	target := netip.MustParseAddr("10.0.0.2")
	p.markUnavailable(target)

	_, allocated, _ := p.stats()
	if allocated != 1 {
		t.Errorf("allocated = %d after markUnavailable, want 1", allocated)
	}

	seen := make(map[netip.Addr]bool)
	for range 2 {
		addr, ok := p.allocate(nil)
		if !ok {
			t.Fatal("allocate failed")
		}
		seen[addr] = true
	}
	if seen[target] {
		t.Error("marked-unavailable address was allocated")
	}

	p.release(target)
	_, allocated, _ = p.stats()
	if allocated != 3 {
		t.Errorf("release of unavailable addr should be no-op, allocated = %d, want 3", allocated)
	}
}
