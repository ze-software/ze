package l2tppool

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
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

func TestPoolHandleFramedIPBypass(t *testing.T) {
	dns1 := netip.MustParseAddr("8.8.8.8")
	dns2 := netip.MustParseAddr("8.8.4.4")
	pool := newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.10"), dns1, dns2)

	p := &poolPlugin{}
	p.pool = pool

	framedIP := netip.MustParseAddr("198.51.100.5")
	l2tp.StoreSessionMetadata(1, 2, &l2tp.AuthMetadata{FramedIP: framedIP})
	defer l2tp.ClearSessionMetadata(1, 2)

	result := p.handle(ppp.EventIPRequest{
		TunnelID:  1,
		SessionID: 2,
		Family:    ppp.AddressFamilyIPv4,
	})

	if !result.Accept {
		t.Fatalf("expected accept, got reject: %s", result.Reason)
	}
	if result.Peer != framedIP {
		t.Errorf("Peer = %v, want %v (RADIUS-assigned)", result.Peer, framedIP)
	}
	if result.Local != testGW {
		t.Errorf("Local = %v, want %v (pool gateway)", result.Local, testGW)
	}
	if result.DNSPrimary != dns1 {
		t.Errorf("DNSPrimary = %v, want %v", result.DNSPrimary, dns1)
	}
	if result.DNSSecondary != dns2 {
		t.Errorf("DNSSecondary = %v, want %v", result.DNSSecondary, dns2)
	}

	_, poolAllocated, _ := pool.stats()
	if poolAllocated != 0 {
		t.Error("pool should not have been used for RADIUS-assigned IP")
	}
}

func TestPoolHandleFramedIPBypassTracksSession(t *testing.T) {
	pool := newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.10"),
		netip.Addr{}, netip.Addr{})

	p := &poolPlugin{}
	p.pool = pool

	framedIP := netip.MustParseAddr("198.51.100.7")
	l2tp.StoreSessionMetadata(3, 4, &l2tp.AuthMetadata{FramedIP: framedIP})
	defer l2tp.ClearSessionMetadata(3, 4)

	p.handle(ppp.EventIPRequest{TunnelID: 3, SessionID: 4, Family: ppp.AddressFamilyIPv4})

	val, ok := p.sessionAddrs.Load(sessionKey{tunnelID: 3, sessionID: 4})
	if !ok {
		t.Fatal("session address should be tracked for teardown release")
	}
	sa, ok2 := val.(sessionAddr)
	if !ok2 {
		t.Fatalf("tracked value type = %T, want sessionAddr", val)
	}
	if sa.addr != framedIP {
		t.Errorf("tracked address = %v, want %v", sa.addr, framedIP)
	}
	if sa.fromPool {
		t.Error("RADIUS-assigned address should have fromPool=false")
	}
}

func TestPoolHandleNoMetadataFallsThrough(t *testing.T) {
	pool := newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.10"),
		netip.Addr{}, netip.Addr{})

	p := &poolPlugin{}
	p.pool = pool

	result := p.handle(ppp.EventIPRequest{TunnelID: 5, SessionID: 6, Family: ppp.AddressFamilyIPv4})

	if !result.Accept {
		t.Fatalf("expected accept from pool, got reject: %s", result.Reason)
	}
	if result.Peer != netip.MustParseAddr("10.0.0.1") {
		t.Errorf("Peer = %v, want 10.0.0.1 (pool-allocated)", result.Peer)
	}

	_, allocated, _ := pool.stats()
	if allocated != 1 {
		t.Errorf("pool allocated = %d, want 1", allocated)
	}
}

func TestPoolHandleNamedPool(t *testing.T) {
	defaultPool := newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.10"),
		netip.Addr{}, netip.Addr{})
	goldGW := netip.MustParseAddr("10.1.0.254")
	goldPool := newIPv4Pool(goldGW, netip.MustParseAddr("10.1.0.1"), netip.MustParseAddr("10.1.0.10"),
		netip.MustParseAddr("1.1.1.1"), netip.Addr{})

	p := &poolPlugin{}
	p.pool = defaultPool
	p.namedPools = map[string]*ipv4Pool{"gold": goldPool}

	l2tp.StoreSessionMetadata(7, 8, &l2tp.AuthMetadata{FramedPool: "gold"})
	defer l2tp.ClearSessionMetadata(7, 8)

	result := p.handle(ppp.EventIPRequest{TunnelID: 7, SessionID: 8, Family: ppp.AddressFamilyIPv4})

	if !result.Accept {
		t.Fatalf("expected accept, got reject: %s", result.Reason)
	}
	if result.Local != goldGW {
		t.Errorf("Local = %v, want %v (gold pool gateway)", result.Local, goldGW)
	}
	if result.Peer != netip.MustParseAddr("10.1.0.1") {
		t.Errorf("Peer = %v, want 10.1.0.1 (gold pool)", result.Peer)
	}
	if result.DNSPrimary != netip.MustParseAddr("1.1.1.1") {
		t.Errorf("DNSPrimary = %v, want 1.1.1.1", result.DNSPrimary)
	}

	_, defaultAllocated, _ := defaultPool.stats()
	if defaultAllocated != 0 {
		t.Error("default pool should not have been used")
	}
	_, goldAllocated, _ := goldPool.stats()
	if goldAllocated != 1 {
		t.Errorf("gold pool allocated = %d, want 1", goldAllocated)
	}
}

func TestPoolHandleNamedPoolNotFound(t *testing.T) {
	defaultPool := newIPv4Pool(testGW, netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.0.0.10"),
		netip.Addr{}, netip.Addr{})

	p := &poolPlugin{}
	p.pool = defaultPool

	l2tp.StoreSessionMetadata(9, 10, &l2tp.AuthMetadata{FramedPool: "nonexistent"})
	defer l2tp.ClearSessionMetadata(9, 10)

	result := p.handle(ppp.EventIPRequest{TunnelID: 9, SessionID: 10, Family: ppp.AddressFamilyIPv4})

	if result.Accept {
		t.Fatal("expected reject for nonexistent named pool")
	}
}

func TestParseNamedPoolConfig(t *testing.T) {
	data := `{"l2tp":{"pool":{"ipv4":{"gateway":"10.0.0.254","start":"10.0.0.1","end":"10.0.0.10"},"named-pool":[{"name":"gold","gateway":"10.1.0.254","start":"10.1.0.1","end":"10.1.0.5"},{"name":"silver","gateway":"10.2.0.254","start":"10.2.0.1","end":"10.2.0.3"}]}}}`

	result, err := parseFullPoolConfig(data)
	if err != nil {
		t.Fatalf("parseFullPoolConfig error: %v", err)
	}
	if !result.found {
		t.Fatal("expected found=true")
	}
	if result.defaultPool == nil {
		t.Fatal("expected default pool")
	}
	if len(result.namedPools) != 2 {
		t.Fatalf("expected 2 named pools, got %d", len(result.namedPools))
	}
	gold, ok := result.namedPools["gold"]
	if !ok {
		t.Fatal("gold pool not found")
	}
	total, _, _ := gold.stats()
	if total != 5 {
		t.Errorf("gold pool total = %d, want 5", total)
	}
	silver, ok := result.namedPools["silver"]
	if !ok {
		t.Fatal("silver pool not found")
	}
	total, _, _ = silver.stats()
	if total != 3 {
		t.Errorf("silver pool total = %d, want 3", total)
	}
}

func TestParseNamedPoolMissingName(t *testing.T) {
	data := `{"l2tp":{"pool":{"ipv4":{"gateway":"10.0.0.254","start":"10.0.0.1","end":"10.0.0.10"},"named-pool":[{"gateway":"10.1.0.254","start":"10.1.0.1","end":"10.1.0.5"}]}}}`

	_, err := parseFullPoolConfig(data)
	if err == nil {
		t.Fatal("expected error for named pool without name")
	}
}

func TestPoolBitmapScale(t *testing.T) {
	const poolSize = 2000
	start := netip.MustParseAddr("10.0.0.1")
	endU32 := addrToUint32(start) + poolSize - 1
	end := uint32ToAddr(endU32)

	p := newIPv4Pool(testGW, start, end, netip.Addr{}, netip.Addr{})

	total, _, avail := p.stats()
	if total != poolSize || avail != poolSize {
		t.Fatalf("initial: total=%d avail=%d, want %d", total, avail, poolSize)
	}

	addrs := make([]netip.Addr, 0, poolSize)
	seen := make(map[netip.Addr]bool, poolSize)
	for range poolSize {
		addr, ok := p.allocate()
		if !ok {
			t.Fatalf("allocation failed at %d", len(addrs))
		}
		if seen[addr] {
			t.Fatalf("duplicate address: %s", addr)
		}
		seen[addr] = true
		addrs = append(addrs, addr)
	}

	_, allocated, avail := p.stats()
	if allocated != poolSize || avail != 0 {
		t.Fatalf("after alloc: allocated=%d avail=%d", allocated, avail)
	}

	if _, ok := p.allocate(); ok {
		t.Fatal("allocation should fail on exhausted pool")
	}

	for _, addr := range addrs {
		p.release(addr)
	}

	_, allocated, avail = p.stats()
	if allocated != 0 || avail != poolSize {
		t.Fatalf("after release: allocated=%d avail=%d", allocated, avail)
	}
}

func TestParseNoNamedPools(t *testing.T) {
	data := `{"l2tp":{"pool":{"ipv4":{"gateway":"10.0.0.254","start":"10.0.0.1","end":"10.0.0.10"}}}}`

	result, err := parseFullPoolConfig(data)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if result.defaultPool == nil {
		t.Fatal("expected default pool")
	}
	if len(result.namedPools) != 0 {
		t.Errorf("expected 0 named pools, got %d", len(result.namedPools))
	}
}
