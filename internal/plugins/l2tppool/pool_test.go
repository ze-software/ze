package l2tppool

import (
	"net/netip"
	"testing"
)

var testGW = netip.MustParseAddr("10.0.0.254")

func TestPoolAllocateIPv4(t *testing.T) {
	p := newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.10"),
		netip.Addr{}, netip.Addr{})
	addr, ok := p.allocate()
	if !ok {
		t.Fatal("expected allocation")
	}
	if addr != netip.MustParseAddr("10.0.0.1") {
		t.Fatalf("expected 10.0.0.1, got %s", addr)
	}
}

func TestPoolAllocateSequential(t *testing.T) {
	p := newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.3"),
		netip.Addr{}, netip.Addr{})

	a1, ok := p.allocate()
	if !ok || a1 != netip.MustParseAddr("10.0.0.1") {
		t.Fatalf("first = %s, ok=%v", a1, ok)
	}
	a2, ok := p.allocate()
	if !ok || a2 != netip.MustParseAddr("10.0.0.2") {
		t.Fatalf("second = %s, ok=%v", a2, ok)
	}
	a3, ok := p.allocate()
	if !ok || a3 != netip.MustParseAddr("10.0.0.3") {
		t.Fatalf("third = %s, ok=%v", a3, ok)
	}
}

func TestPoolRelease(t *testing.T) {
	p := newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.1"),
		netip.Addr{}, netip.Addr{})

	addr, ok := p.allocate()
	if !ok {
		t.Fatal("expected first allocation")
	}
	p.release(addr)

	addr2, ok := p.allocate()
	if !ok {
		t.Fatal("expected allocation after release")
	}
	if addr2 != addr {
		t.Fatalf("expected same address after release, got %s", addr2)
	}
}

func TestPoolExhausted(t *testing.T) {
	p := newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.2"),
		netip.Addr{}, netip.Addr{})

	if _, ok := p.allocate(); !ok {
		t.Fatal("first should succeed")
	}
	if _, ok := p.allocate(); !ok {
		t.Fatal("second should succeed")
	}
	if _, ok := p.allocate(); ok {
		t.Fatal("third should fail (pool exhausted)")
	}
}

func TestPoolStats(t *testing.T) {
	p := newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.10"),
		netip.Addr{}, netip.Addr{})

	total, allocated, available := p.stats()
	if total != 10 || allocated != 0 || available != 10 {
		t.Fatalf("initial: total=%d alloc=%d avail=%d", total, allocated, available)
	}

	p.allocate()
	p.allocate()

	total, allocated, available = p.stats()
	if total != 10 || allocated != 2 || available != 8 {
		t.Fatalf("after 2: total=%d alloc=%d avail=%d", total, allocated, available)
	}
}

func TestPoolDNS(t *testing.T) {
	dns1 := netip.MustParseAddr("8.8.8.8")
	dns2 := netip.MustParseAddr("8.8.4.4")
	p := newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.10"),
		dns1, dns2)

	if p.dnsPrimary != dns1 {
		t.Fatalf("dns primary = %s, want 8.8.8.8", p.dnsPrimary)
	}
	if p.dnsSecondary != dns2 {
		t.Fatalf("dns secondary = %s, want 8.8.4.4", p.dnsSecondary)
	}
}

func TestPoolGateway(t *testing.T) {
	gw := netip.MustParseAddr("10.0.0.254")
	p := newIPv4Pool(gw, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.10"),
		netip.Addr{}, netip.Addr{})

	if p.gateway != gw {
		t.Fatalf("gateway = %s, want 10.0.0.254", p.gateway)
	}
}

func TestPoolReleaseUnallocated(t *testing.T) {
	p := newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.10"),
		netip.Addr{}, netip.Addr{})

	p.release(netip.MustParseAddr("10.0.0.5"))

	total, allocated, _ := p.stats()
	if allocated != 0 || total != 10 {
		t.Fatalf("release of unallocated should be no-op: total=%d alloc=%d", total, allocated)
	}
}

func TestPoolReleaseOutOfRange(t *testing.T) {
	p := newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.10"),
		netip.Addr{}, netip.Addr{})

	p.release(netip.MustParseAddr("192.168.0.1"))

	_, allocated, _ := p.stats()
	if allocated != 0 {
		t.Fatal("release of out-of-range should be no-op")
	}
}
