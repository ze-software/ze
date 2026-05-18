package dhcpserver

import (
	"net"
	"net/netip"
	"testing"
)

func makeRange(start, stop string) []addressRange {
	return []addressRange{{Name: "pool", Start: netip.MustParseAddr(start), Stop: netip.MustParseAddr(stop)}}
}

func TestPoolAllocate(t *testing.T) {
	t.Parallel()

	p := newPool(makeRange("192.168.1.100", "192.168.1.105"), nil)

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

	p := newPool(makeRange("192.168.1.100", "192.168.1.102"), statics)

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

	p := newPool(makeRange("10.0.0.1", "10.0.0.3"), nil)

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

	p := newPool(makeRange("10.0.0.1", "10.0.0.1"), nil)

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

	p := newPool(makeRange("10.0.0.1", "10.0.0.5"), nil)

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

	p := newPool(makeRange("10.0.0.1", "10.0.0.3"), nil)

	p.release(netip.MustParseAddr("10.0.0.100"))

	total, allocated, available := p.stats()
	if total != 3 || allocated != 0 || available != 3 {
		t.Errorf("stats after bad release: total=%d alloc=%d avail=%d", total, allocated, available)
	}
}

func TestPoolStats(t *testing.T) {
	t.Parallel()

	p := newPool(makeRange("10.0.0.1", "10.0.0.10"), nil)

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

	p := newPool(makeRange("10.0.0.1", "10.0.0.5"), nil)

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

	emptyPool := newPool(nil, nil)
	ok = emptyPool.reserve(addr, mac)
	if ok {
		t.Error("reserve on empty pool should fail")
	}
}

func TestPoolMarkUnavailable(t *testing.T) {
	t.Parallel()

	p := newPool(makeRange("10.0.0.1", "10.0.0.3"), nil)

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

func TestPoolMultipleRanges(t *testing.T) {
	t.Parallel()

	ranges := []addressRange{
		{Name: "low", Start: netip.MustParseAddr("10.0.0.1"), Stop: netip.MustParseAddr("10.0.0.3")},
		{Name: "high", Start: netip.MustParseAddr("10.0.0.10"), Stop: netip.MustParseAddr("10.0.0.12")},
	}
	p := newPool(ranges, nil)

	total, _, available := p.stats()
	if total != 6 || available != 6 {
		t.Fatalf("stats: total=%d available=%d, want 6/6", total, available)
	}

	var addrs []netip.Addr
	for range 6 {
		addr, ok := p.allocate(nil)
		if !ok {
			t.Fatal("allocate failed before exhaustion")
		}
		addrs = append(addrs, addr)
	}

	for i := range 3 {
		a := addrToUint32(addrs[i])
		if a < addrToUint32(netip.MustParseAddr("10.0.0.1")) || a > addrToUint32(netip.MustParseAddr("10.0.0.3")) {
			t.Errorf("addr[%d]=%v not in first range", i, addrs[i])
		}
	}
	for i := 3; i < 6; i++ {
		a := addrToUint32(addrs[i])
		if a < addrToUint32(netip.MustParseAddr("10.0.0.10")) || a > addrToUint32(netip.MustParseAddr("10.0.0.12")) {
			t.Errorf("addr[%d]=%v not in second range", i, addrs[i])
		}
	}

	_, ok := p.allocate(nil)
	if ok {
		t.Error("expected pool exhaustion after 6 allocations")
	}
}

func TestPoolMultipleRangesExhaustion(t *testing.T) {
	t.Parallel()

	ranges := []addressRange{
		{Name: "small", Start: netip.MustParseAddr("10.0.0.1"), Stop: netip.MustParseAddr("10.0.0.2")},
		{Name: "big", Start: netip.MustParseAddr("10.0.0.10"), Stop: netip.MustParseAddr("10.0.0.13")},
	}
	p := newPool(ranges, nil)

	for range 2 {
		_, ok := p.allocate(nil)
		if !ok {
			t.Fatal("allocate from first range failed")
		}
	}

	addr, ok := p.allocate(nil)
	if !ok {
		t.Fatal("allocate from second range failed after first exhausted")
	}
	a := addrToUint32(addr)
	if a < addrToUint32(netip.MustParseAddr("10.0.0.10")) || a > addrToUint32(netip.MustParseAddr("10.0.0.13")) {
		t.Errorf("expected allocation from second range, got %v", addr)
	}
}

func TestPoolMultipleRangesStatic(t *testing.T) {
	t.Parallel()

	ranges := []addressRange{
		{Name: "low", Start: netip.MustParseAddr("10.0.0.1"), Stop: netip.MustParseAddr("10.0.0.3")},
		{Name: "high", Start: netip.MustParseAddr("10.0.0.10"), Stop: netip.MustParseAddr("10.0.0.12")},
	}
	statics := []staticMapping{
		{Name: "srv", MAC: net.HardwareAddr{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}, IP: netip.MustParseAddr("10.0.0.11")},
	}
	p := newPool(ranges, statics)

	total, allocated, available := p.stats()
	if total != 6 || allocated != 1 || available != 5 {
		t.Fatalf("stats: total=%d alloc=%d avail=%d, want 6/1/5", total, allocated, available)
	}

	seen := make(map[netip.Addr]bool)
	for range 5 {
		addr, ok := p.allocate(nil)
		if !ok {
			t.Fatal("allocate failed")
		}
		seen[addr] = true
	}
	if seen[netip.MustParseAddr("10.0.0.11")] {
		t.Error("static IP in second range was dynamically allocated")
	}

	_, ok := p.allocate(nil)
	if ok {
		t.Error("expected exhaustion after 5 dynamic allocations")
	}
}

func TestPoolMultipleRangesStats(t *testing.T) {
	t.Parallel()

	ranges := []addressRange{
		{Name: "a", Start: netip.MustParseAddr("10.0.0.1"), Stop: netip.MustParseAddr("10.0.0.5")},
		{Name: "b", Start: netip.MustParseAddr("10.0.0.20"), Stop: netip.MustParseAddr("10.0.0.24")},
	}
	p := newPool(ranges, nil)

	total, allocated, available := p.stats()
	if total != 10 {
		t.Errorf("total = %d, want 10", total)
	}
	if allocated != 0 {
		t.Errorf("allocated = %d, want 0", allocated)
	}
	if available != 10 {
		t.Errorf("available = %d, want 10", available)
	}
}
