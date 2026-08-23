package l2tppool

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
)

func TestIPv6PrefixPoolAllocate(t *testing.T) {
	// 2001:db8::/32 delegating /48 = 2^16 = 65536 prefixes
	p, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8::/32"), 48)
	if err != nil {
		t.Fatal(err)
	}
	prefix, ok := p.allocate()
	if !ok {
		t.Fatal("expected allocation")
	}
	want := netip.MustParsePrefix("2001:db8::/48")
	if prefix != want {
		t.Fatalf("got %s, want %s", prefix, want)
	}
}

func TestIPv6PrefixPoolAllocateSequential(t *testing.T) {
	p, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8::/32"), 48)
	if err != nil {
		t.Fatal(err)
	}

	p1, ok := p.allocate()
	if !ok {
		t.Fatal("first allocation failed")
	}
	if p1 != netip.MustParsePrefix("2001:db8::/48") {
		t.Fatalf("first = %s", p1)
	}

	p2, ok := p.allocate()
	if !ok {
		t.Fatal("second allocation failed")
	}
	if p2 != netip.MustParsePrefix("2001:db8:1::/48") {
		t.Fatalf("second = %s", p2)
	}

	p3, ok := p.allocate()
	if !ok {
		t.Fatal("third allocation failed")
	}
	if p3 != netip.MustParsePrefix("2001:db8:2::/48") {
		t.Fatalf("third = %s", p3)
	}
}

func TestIPv6PrefixPoolRelease(t *testing.T) {
	p, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8::/48"), 48)
	if err != nil {
		t.Fatal(err)
	}
	prefix, ok := p.allocate()
	if !ok {
		t.Fatal("expected allocation")
	}
	p.release(prefix)

	again, ok := p.allocate()
	if !ok {
		t.Fatal("expected allocation after release")
	}
	if again != prefix {
		t.Fatalf("expected same prefix after release, got %s", again)
	}
}

func TestIPv6PrefixPoolExhausted(t *testing.T) {
	// /46 with /48 delegation = 4 prefixes
	p, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8::/46"), 48)
	if err != nil {
		t.Fatal(err)
	}

	for i := range 4 {
		if _, ok := p.allocate(); !ok {
			t.Fatalf("allocation %d should succeed", i)
		}
	}
	if _, ok := p.allocate(); ok {
		t.Fatal("5th allocation should fail (pool exhausted)")
	}
}

func TestIPv6PrefixPoolVariableLengths(t *testing.T) {
	tests := []struct {
		block    string
		delegLen int
		wantSize uint32
		first    string
	}{
		{"2001:db8::/32", 48, 65536, "2001:db8::/48"},
		{"2001:db8::/32", 56, 16777216, "2001:db8::/56"},
		{"2001:db8::/48", 56, 256, "2001:db8::/56"},
		{"2001:db8::/48", 64, 65536, "2001:db8::/64"},
		{"2001:db8:ab00::/40", 48, 256, "2001:db8:ab00::/48"},
	}

	for _, tt := range tests {
		t.Run(tt.block+"->"+netip.MustParsePrefix(tt.first).String(), func(t *testing.T) {
			p, err := newIPv6PrefixPool(netip.MustParsePrefix(tt.block), tt.delegLen)
			if err != nil {
				t.Fatal(err)
			}
			total, _, _ := p.stats()
			if total != tt.wantSize {
				t.Fatalf("size = %d, want %d", total, tt.wantSize)
			}
			first, ok := p.allocate()
			if !ok {
				t.Fatal("first allocation failed")
			}
			want := netip.MustParsePrefix(tt.first)
			if first != want {
				t.Fatalf("first = %s, want %s", first, want)
			}
		})
	}
}

func TestIPv6PrefixPoolStats(t *testing.T) {
	p, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8::/46"), 48)
	if err != nil {
		t.Fatal(err)
	}

	total, allocated, available := p.stats()
	if total != 4 || allocated != 0 || available != 4 {
		t.Fatalf("initial: total=%d alloc=%d avail=%d", total, allocated, available)
	}

	p.allocate()
	p.allocate()

	total, allocated, available = p.stats()
	if total != 4 || allocated != 2 || available != 2 {
		t.Fatalf("after 2: total=%d alloc=%d avail=%d", total, allocated, available)
	}
}

func TestIPv6PrefixPoolReleaseOutOfRange(t *testing.T) {
	p, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8::/46"), 48)
	if err != nil {
		t.Fatal(err)
	}

	// Release a prefix from a different block
	p.release(netip.MustParsePrefix("2001:db9::/48"))
	_, allocated, _ := p.stats()
	if allocated != 0 {
		t.Fatal("release of out-of-range prefix should be no-op")
	}
}

func TestIPv6PrefixPoolReleaseWrongLength(t *testing.T) {
	p, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8::/46"), 48)
	if err != nil {
		t.Fatal(err)
	}

	// Release a prefix with wrong delegation length
	p.release(netip.MustParsePrefix("2001:db8::/56"))
	_, allocated, _ := p.stats()
	if allocated != 0 {
		t.Fatal("release of wrong-length prefix should be no-op")
	}
}

func TestIPv6PrefixPoolInvalidConfig(t *testing.T) {
	// delegLen shorter than block
	if _, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8::/48"), 32); err == nil {
		t.Fatal("expected error: delegLen < block prefix len")
	}

	// delegLen > 64
	if _, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8::/48"), 65); err == nil {
		t.Fatal("expected error: delegLen > 64")
	}

	// delegLen < 48
	if _, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8::/32"), 47); err == nil {
		t.Fatal("expected error: delegLen < 48")
	}

	// delegLen == block len (pool of 1, should work)
	p, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8::/48"), 48)
	if err != nil {
		t.Fatalf("single-prefix pool should be valid: %v", err)
	}
	total, _, _ := p.stats()
	if total != 1 {
		t.Fatalf("single-prefix pool size = %d, want 1", total)
	}
}

func TestPrefixHandlerFromPool(t *testing.T) {
	pool, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8::/46"), 48)
	if err != nil {
		t.Fatal(err)
	}

	p := &poolPlugin{}
	p.v6pool = pool

	result := p.handlePrefix(l2tp.PrefixRequest{TunnelID: 1, SessionID: 2})
	if !result.OK {
		t.Fatalf("expected OK, got: %s", result.Reason)
	}
	want := netip.MustParsePrefix("2001:db8::/48")
	if result.Prefix != want {
		t.Errorf("prefix = %s, want %s", result.Prefix, want)
	}

	_, allocated, _ := pool.stats()
	if allocated != 1 {
		t.Errorf("allocated = %d, want 1", allocated)
	}
}

func TestPrefixHandlerRADIUSOverride(t *testing.T) {
	pool, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8::/46"), 48)
	if err != nil {
		t.Fatal(err)
	}

	p := &poolPlugin{}
	p.v6pool = pool

	radiusPrefix := netip.MustParsePrefix("2001:db8:abcd::/48")
	l2tp.StoreSessionMetadata(3, 4, &l2tp.AuthMetadata{DelegatedIPv6Prefix: radiusPrefix})
	defer l2tp.ClearSessionMetadata(3, 4)

	result := p.handlePrefix(l2tp.PrefixRequest{TunnelID: 3, SessionID: 4})
	if !result.OK {
		t.Fatalf("expected OK, got: %s", result.Reason)
	}
	if result.Prefix != radiusPrefix {
		t.Errorf("prefix = %s, want %s (RADIUS)", result.Prefix, radiusPrefix)
	}

	_, allocated, _ := pool.stats()
	if allocated != 0 {
		t.Error("pool should not be used for RADIUS-assigned prefix")
	}
}

func TestPrefixHandlerPoolExhausted(t *testing.T) {
	pool, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8::/48"), 48)
	if err != nil {
		t.Fatal(err)
	}

	p := &poolPlugin{}
	p.v6pool = pool

	r1 := p.handlePrefix(l2tp.PrefixRequest{TunnelID: 1, SessionID: 1})
	if !r1.OK {
		t.Fatalf("first should succeed: %s", r1.Reason)
	}
	r2 := p.handlePrefix(l2tp.PrefixRequest{TunnelID: 1, SessionID: 2})
	if r2.OK {
		t.Fatal("second should fail (pool exhausted)")
	}
}

func TestPrefixHandlerRelease(t *testing.T) {
	pool, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8::/48"), 48)
	if err != nil {
		t.Fatal(err)
	}

	p := &poolPlugin{}
	p.v6pool = pool

	p.handlePrefix(l2tp.PrefixRequest{TunnelID: 5, SessionID: 6})
	_, allocated, _ := pool.stats()
	if allocated != 1 {
		t.Fatalf("after alloc: allocated = %d, want 1", allocated)
	}

	p.releasePrefix(5, 6)
	_, allocated, _ = pool.stats()
	if allocated != 0 {
		t.Fatalf("after release: allocated = %d, want 0", allocated)
	}
}

func TestPrefixHandlerDuplicateAllocation(t *testing.T) {
	pool, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8::/46"), 48)
	if err != nil {
		t.Fatal(err)
	}

	p := &poolPlugin{}
	p.v6pool = pool

	r1 := p.handlePrefix(l2tp.PrefixRequest{TunnelID: 10, SessionID: 11})
	if !r1.OK {
		t.Fatalf("first should succeed: %s", r1.Reason)
	}

	r2 := p.handlePrefix(l2tp.PrefixRequest{TunnelID: 10, SessionID: 11})
	if r2.OK {
		t.Fatal("duplicate allocation for same session should be rejected")
	}

	_, allocated, _ := pool.stats()
	if allocated != 1 {
		t.Errorf("allocated = %d, want 1 (no leak)", allocated)
	}
}

func TestParseIPv6PDPoolConfig(t *testing.T) {
	data := `{"l2tp":{"pool":{"ipv6-pd":{"block":"2001:db8::/32","delegation-length":56}}}}`

	result, err := parseFullPoolConfig(data)
	if err != nil {
		t.Fatalf("parseFullPoolConfig error: %v", err)
	}
	if !result.found {
		t.Fatal("expected found=true")
	}
	if result.defaultV6Pool == nil {
		t.Fatal("expected default v6 pool")
	}
	total, _, _ := result.defaultV6Pool.stats()
	if total != 16777216 {
		t.Errorf("total = %d, want 16777216 (2^24)", total)
	}
}

func TestParseNamedIPv6PoolConfig(t *testing.T) {
	// A map of pool name to entry, which is what Tree.ToMap emits for a YANG
	// list. The array-of-entries form this used to carry has no producer.
	data := `{"l2tp":{"pool":{"named-ipv6-pool":{"v6-gold":{"block":"2001:db8:aa00::/40","delegation-length":48}}}}}`

	result, err := parseFullPoolConfig(data)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(result.namedV6Pools) != 1 {
		t.Fatalf("expected 1 named v6 pool, got %d", len(result.namedV6Pools))
	}
	gold, ok := result.namedV6Pools["v6-gold"]
	if !ok {
		t.Fatal("v6-gold pool not found")
	}
	total, _, _ := gold.stats()
	if total != 256 {
		t.Errorf("gold pool total = %d, want 256", total)
	}
}

func TestPrefixHandlerNamedPool(t *testing.T) {
	gold, err := newIPv6PrefixPool(netip.MustParsePrefix("2001:db8:aa00::/40"), 48)
	if err != nil {
		t.Fatal(err)
	}

	p := &poolPlugin{}
	p.v6namedPools = map[string]*ipv6PrefixPool{"v6-gold": gold}

	l2tp.StoreSessionMetadata(7, 8, &l2tp.AuthMetadata{FramedIPv6Pool: "v6-gold"})
	defer l2tp.ClearSessionMetadata(7, 8)

	result := p.handlePrefix(l2tp.PrefixRequest{TunnelID: 7, SessionID: 8})
	if !result.OK {
		t.Fatalf("expected OK, got: %s", result.Reason)
	}
	want := netip.MustParsePrefix("2001:db8:aa00::/48")
	if result.Prefix != want {
		t.Errorf("prefix = %s, want %s", result.Prefix, want)
	}
}
