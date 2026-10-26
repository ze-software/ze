// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- Virtual IP pool and lease registry tests

package eap

import (
	"errors"
	"net/netip"
	"strconv"
	"sync"
	"testing"
)

func mustPool(t *testing.T, v4, v6 string, dns ...string) *Pool {
	t.Helper()
	p, err := NewPool("p", v4, v6, dns)
	if err != nil {
		t.Fatalf("NewPool(%q, %q): %v", v4, v6, err)
	}
	return p
}

func mustAcquire(t *testing.T, r *Registry, p *Pool, owner string, f Family, req netip.Addr) *Lease {
	t.Helper()
	l, err := r.Acquire(p, owner, f, req)
	if err != nil {
		t.Fatalf("Acquire(%s, %s, %v): %v", owner, f, req, err)
	}
	return l
}

// VALIDATES: the first lease of a fresh pool is the first host address, never the network
// address, and the pool's DNS servers are split by family.
//
// The pool's search domain is gone: commit refuses a `domain` leaf because the IKEv2
// Configuration payload has no attribute for it (spec Q-1); that refusal is asserted in
// ipsec.TestVirtualIPPoolCommitRefusals and test/parse/ipsec-virtual-ip-pool-domain.ci.
func TestVirtualIPPoolAllocate(t *testing.T) {
	p, err := NewPool("p", "10.10.0.0/24", "", []string{"8.8.8.8", "2001:4860:4860::8888"})
	if err != nil {
		t.Fatal(err)
	}
	r := NewRegistry()

	l, err := r.Acquire(p, "alice", FamilyIPv4, netip.Addr{})
	if err != nil {
		t.Fatal(err)
	}
	if !l.Addr().Is4() {
		t.Fatalf("expected an IPv4 allocation, got %s", l.Addr())
	}
	if l.Addr() == p.Prefix4().Addr() {
		t.Fatal("should not allocate network address")
	}
	if got := l.Addr().String(); got != "10.10.0.1" {
		t.Fatalf("first allocation: got %s, want 10.10.0.1", got)
	}
	if dns := p.DNS(FamilyIPv4); len(dns) != 1 || dns[0].String() != "8.8.8.8" {
		t.Fatalf("DNS4: got %v", dns)
	}
	if dns := p.DNS(FamilyIPv6); len(dns) != 1 || dns[0].String() != "2001:4860:4860::8888" {
		t.Fatalf("DNS6: got %v", dns)
	}
}

// VALIDATES: a /30 holds two leases, refuses a third, and serves again once one is released.
func TestVirtualIPPoolRelease(t *testing.T) {
	p, err := NewPool("p", "10.10.0.0/30", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	r := NewRegistry()

	// A /30 has 2 usable addresses (4 total - network - broadcast).
	l1, err := r.Acquire(p, "a", FamilyIPv4, netip.Addr{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Acquire(p, "b", FamilyIPv4, netip.Addr{}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Acquire(p, "c", FamilyIPv4, netip.Addr{}); !errors.Is(err, ErrPoolExhausted) {
		t.Fatalf("third lease from a /30: got %v, want ErrPoolExhausted", err)
	}
	if err := r.Release(l1); err != nil {
		t.Fatal(err)
	}
	l3, err := r.Acquire(p, "c", FamilyIPv4, netip.Addr{})
	if err != nil {
		t.Fatal(err)
	}
	if l3.Addr() != l1.Addr() {
		t.Fatalf("reallocation: got %s, want %s", l3.Addr(), l1.Addr())
	}
}

// VALIDATES: an exhausted pool refuses rather than wrapping around and handing an address
// out twice, and HeldIn reports every address held.
func TestVirtualIPPoolExhausted(t *testing.T) {
	p, err := NewPool("p", "10.10.0.0/30", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	r := NewRegistry()
	// A /30 = 4 addresses, 2 usable.
	for i := range 2 {
		if _, err := r.Acquire(p, strconv.Itoa(i), FamilyIPv4, netip.Addr{}); err != nil {
			t.Fatalf("allocation failed: %v", err)
		}
	}
	if _, err := r.Acquire(p, "x", FamilyIPv4, netip.Addr{}); !errors.Is(err, ErrPoolExhausted) {
		t.Fatalf("expected ErrPoolExhausted, got %v", err)
	}
	held, capacity, ok := r.HeldIn(p, FamilyIPv4)
	if !ok {
		t.Fatal("HeldIn answered that the IPv4 pool serves no IPv4")
	}
	if held != capacity {
		t.Fatalf("available: got %d, want 0", capacity-held)
	}
}

// VALIDATES: a dual-stack pool serves each family on request, and only the requested one,
// and its mixed DNS list is split into one server per family.
// PREVENTS: the old Allocate, which leased one address of EVERY family whatever was asked.
func TestVirtualIPPoolDualStack(t *testing.T) {
	p := mustPool(t, "10.10.0.0/24", "fd00::/112", "8.8.8.8", "2001:4860:4860::8888")
	if got := len(p.DNS(FamilyIPv4)); got != 1 {
		t.Fatalf("DNS4 count: got %d, want 1", got)
	}
	if got := len(p.DNS(FamilyIPv6)); got != 1 {
		t.Fatalf("DNS6 count: got %d, want 1", got)
	}
	r := NewRegistry()
	v4 := mustAcquire(t, r, p, "a", FamilyIPv4, netip.Addr{})
	if !v4.Addr().Is4() {
		t.Fatalf("IPv4 lease is %s", v4.Addr())
	}
	if got := r.Held("p"); got != 1 {
		t.Fatalf("an IPv4 request leased %d addresses, want 1", got)
	}
	v6 := mustAcquire(t, r, p, "a", FamilyIPv6, netip.Addr{})
	if !v6.Addr().Is6() {
		t.Fatalf("IPv6 lease is %s", v6.Addr())
	}
	for _, family := range []Family{FamilyIPv4, FamilyIPv6} {
		held, _, ok := r.HeldIn(p, family)
		if !ok {
			t.Fatalf("HeldIn answered that the dual-stack pool does not serve family %d", family)
		}
		if held != 1 {
			t.Fatalf("family %d holds %d leases, want 1", family, held)
		}
	}
}

// VALIDATES: an owner whose live lease a pool edit moved outside the range is leased an
// address inside the edited range on its next Acquire, while the old holder keeps the old
// lease until it releases it, and that release does not disturb the new lease.
// PREVENTS: a re-initiating peer handed back a lease the pool no longer serves.
func TestAcquireLeasesInsideTheEditedRange(t *testing.T) {
	before := mustPool(t, "10.10.0.0/24", "")
	r := NewRegistry()
	old := mustAcquire(t, r, before, "a", FamilyIPv4, netip.MustParseAddr("10.10.0.200"))
	if old.Addr() != netip.MustParseAddr("10.10.0.200") {
		t.Fatalf("setup: leased %s, want the requested 10.10.0.200", old.Addr())
	}
	after := mustPool(t, "10.10.0.0/25", "")
	if after.Contains(old.Addr()) {
		t.Fatal("setup: the edited range still contains the old lease")
	}

	fresh := mustAcquire(t, r, after, "a", FamilyIPv4, netip.Addr{})
	if !after.Contains(fresh.Addr()) {
		t.Fatalf("re-initiation got %s, outside the edited range %s", fresh.Addr(), after.Prefix4())
	}
	if !r.IsHeldForTest(old.Addr()) {
		t.Fatal("the old holder lost its lease when the owner was leased afresh")
	}
	if err := r.Release(old); err != nil {
		t.Fatalf("release the out-of-range lease: %v", err)
	}
	again := mustAcquire(t, r, after, "a", FamilyIPv4, netip.Addr{})
	if again != fresh {
		t.Fatalf("after the old lease was released the owner got %s, want its live lease %s", again.Addr(), fresh.Addr())
	}
}

// VALIDATES: releasing a lease a pool edit moved off its slot leaves no last-address memory
// behind, so when the range is edited back and another owner is leased that address, the
// slot's owner still reconnects to the last address its own lease held.
// PREVENTS: Release recording the out-of-range lease as the slot's last address, whose
// reverse entry outlives the slot's next release and later makes grantLocked delete the
// owner's real hint.
func TestReleaseOfMovedLeaseKeepsTheOwnersLastAddress(t *testing.T) {
	wide := mustPool(t, "10.10.0.0/24", "")
	r := NewRegistry()
	old := mustAcquire(t, r, wide, "a", FamilyIPv4, netip.MustParseAddr("10.10.0.200"))
	narrow := mustPool(t, "10.10.0.0/25", "")
	fresh := mustAcquire(t, r, narrow, "a", FamilyIPv4, netip.MustParseAddr("10.10.0.50"))
	if fresh.Addr() != netip.MustParseAddr("10.10.0.50") {
		t.Fatalf("setup: the owner was leased %s in the edited range, want the requested 10.10.0.50", fresh.Addr())
	}
	if err := r.Release(old); err != nil {
		t.Fatalf("release the out-of-range lease: %v", err)
	}
	if err := r.Release(fresh); err != nil {
		t.Fatalf("release the slot's lease: %v", err)
	}

	back := mustPool(t, "10.10.0.0/24", "")
	other := mustAcquire(t, r, back, "b", FamilyIPv4, netip.MustParseAddr("10.10.0.200"))
	if other.Addr() != netip.MustParseAddr("10.10.0.200") {
		t.Fatalf("setup: owner b was leased %s, want the requested 10.10.0.200", other.Addr())
	}
	again := mustAcquire(t, r, back, "a", FamilyIPv4, netip.Addr{})
	if again.Addr() != fresh.Addr() {
		t.Fatalf("owner a reconnected to %s, want its last address %s: leasing the moved lease's "+
			"address to owner b deleted a's hint", again.Addr(), fresh.Addr())
	}
}

// VALIDATES: an IPv4-only pool refuses an IPv6 lease with ErrFamilyNotServed (AC-15 at the
// pool layer).
func TestVirtualIPPoolFamilyNotServed(t *testing.T) {
	p := mustPool(t, "10.10.0.0/24", "")
	if _, err := NewRegistry().Acquire(p, "a", FamilyIPv6, netip.Addr{}); !errors.Is(err, ErrFamilyNotServed) {
		t.Fatalf("IPv6 from an IPv4-only pool: got %v, want ErrFamilyNotServed", err)
	}
}

// VALIDATES: every IPv6 prefix the config accepts (/48 through /126) leases addresses
// inside that prefix, never its subnet-router anycast address (AC-18 covers the /96).
// PREVENTS: writing the host identifier over octets 8 through 15, which for a prefix longer
// than /64 overwrites prefix octets.
func TestIPv6LongPrefixInRange(t *testing.T) {
	for _, cidr := range []string{
		"2001:db8::/48",
		"2001:db8::/64",
		"2001:db8:0:0:1234:5678::/96",
		"2001:db8::/112",
		"2001:db8::/126",
	} {
		t.Run(cidr, func(t *testing.T) {
			p := mustPool(t, "", cidr)
			prefix := netip.MustParsePrefix(cidr)
			r := NewRegistry()
			for i := range 3 {
				l := mustAcquire(t, r, p, strconv.Itoa(i), FamilyIPv6, netip.Addr{})
				if !prefix.Contains(l.Addr()) {
					t.Errorf("leased %s from pool %s: outside the prefix", l.Addr(), cidr)
				}
				if l.Addr() == prefix.Addr() {
					t.Errorf("leased the subnet-router anycast address %s", l.Addr())
				}
			}
		})
	}
}

// VALIDATES: every IPv6 prefix ValidateRemoteAccess accepts (/48 through /126) leases an
// address that lies inside that prefix, is not its network address, and round-trips:
// Release accepts the lease it just handed out and the address is free again.
// PREVENTS: a return to writing the host identifier over octets 8 through 15, which for a
// prefix longer than /64 overwrites prefix octets and hands the client an address from a
// different subnet.
func TestVirtualIPPoolV6LeasesStayInsidePrefix(t *testing.T) {
	// The bounds ValidateRemoteAccess permits, plus the widths on either side of the
	// 64-bit boundary where the host-width arithmetic changes shape.
	for _, cidr := range []string{
		"2001:db8::/48",
		"2001:db8::/64",
		"2001:db8:0:0:1234:5678::/96",
		"2001:db8::/112",
		"2001:db8::/126",
	} {
		t.Run(cidr, func(t *testing.T) {
			p, err := NewPool("p", "", cidr, nil)
			if err != nil {
				t.Fatalf("NewPool(%s): %v", cidr, err)
			}
			prefix, err := netip.ParsePrefix(cidr)
			if err != nil {
				t.Fatalf("ParsePrefix: %v", err)
			}
			r := NewRegistry()
			l, err := r.Acquire(p, "a", FamilyIPv6, netip.Addr{})
			if err != nil {
				t.Fatalf("Acquire from %s: %v", cidr, err)
			}
			if !l.Addr().Is6() {
				t.Fatalf("no IPv6 leased from %s, got %s", cidr, l.Addr())
			}
			if !prefix.Contains(l.Addr()) {
				t.Errorf("leased %s from pool %s: address is outside the configured prefix", l.Addr(), cidr)
			}
			if l.Addr() == prefix.Masked().Addr() {
				t.Errorf("leased the network address %s from pool %s", l.Addr(), cidr)
			}
			if err := r.Release(l); err != nil {
				t.Errorf("Release(%s) from pool %s: %v", l.Addr(), cidr, err)
			}
			if r.IsHeldForTest(l.Addr()) {
				t.Errorf("%s is still held after its release from pool %s", l.Addr(), cidr)
			}
		})
	}
}

// VALIDATES: NewPool bounds an IPv6 range itself instead of trusting its caller.
// PREVENTS: a pool built past the config validator computing a host width of zero and
// leasing the network address as though it were a client address.
func TestVirtualIPPoolV6RejectsUnrepresentableRange(t *testing.T) {
	if _, err := NewPool("p", "", "2001:db8::1/128", nil); err == nil {
		t.Error("NewPool accepted a /128 IPv6 range, which names no host address")
	}
}

// VALIDATES: an address from outside the pool's own prefix that shares a lease's low-order
// host bits neither frees nor takes that lease.
// PREVENTS: the host-bit mask folding a foreign address onto an in-pool identifier, which
// would free a lease belonging to a different client or hand it out a second time.
func TestVirtualIPPoolV6ReleaseRejectsForeignAddress(t *testing.T) {
	p := mustPool(t, "", "2001:db8:0:0:1234:5678::/96")
	r := NewRegistry()
	leased := mustAcquire(t, r, p, "alice", FamilyIPv6, netip.Addr{})

	// Same low-order host bits as the lease, different prefix.
	a := leased.Addr().As16()
	a[8], a[9], a[10], a[11] = 0x99, 0x99, 0x99, 0x99
	foreign := netip.AddrFrom16(a)
	if p.Contains(foreign) {
		t.Fatalf("bad test fixture: %s is inside the pool", foreign)
	}

	// Releasing a lease the registry never granted, naming the foreign address, is refused.
	forged := &Lease{addr: foreign, family: FamilyIPv6, pool: "p", owner: "alice", holders: 1}
	if err := r.Release(forged); !errors.Is(err, ErrNotAllocated) {
		t.Fatalf("Release(%s): got %v, want ErrNotAllocated", foreign, err)
	}
	if !r.IsHeldForTest(leased.Addr()) {
		t.Fatalf("the refused foreign release freed the lease %s", leased.Addr())
	}

	// Asking for the foreign address folds it into the prefix, onto the held lease; the
	// registry must hand another address instead of the held one.
	other := mustAcquire(t, r, p, "bob", FamilyIPv6, foreign)
	if other.Addr() == leased.Addr() {
		t.Fatalf("the foreign request %s was given alice's lease %s", foreign, leased.Addr())
	}
	if !p.Contains(other.Addr()) {
		t.Fatalf("bob was leased %s, outside the pool", other.Addr())
	}

	// The real lease is still held, so releasing it now still succeeds.
	if err := r.Release(leased); err != nil {
		t.Fatalf("Release(%s) after the foreign release: %v", leased.Addr(), err)
	}
}

// VALIDATES: NewPool bounds a range itself instead of trusting its caller (AC-20).
// PREVENTS: a pool with no host address, or none at all, being built past the validator.
func TestNewPoolRefusesUnrepresentable(t *testing.T) {
	for _, tc := range []struct{ v4, v6 string }{
		{"10.0.0.0/31", ""},
		{"10.0.0.1/32", ""},
		{"10.0.0.0/7", ""},
		{"", "2001:db8::/127"},
		{"", "2001:db8::1/128"},
		{"", "2001:db8::/47"},
		{"2001:db8::/64", ""},
		{"", "10.0.0.0/24"},
		{"", "::ffff:10.0.0.0/120"},
		{"", ""},
	} {
		if p, err := NewPool("p", tc.v4, tc.v6, nil); err == nil {
			t.Errorf("NewPool(%q, %q) accepted, pool %v", tc.v4, tc.v6, p)
		}
	}
	if _, err := NewPool("p", "10.0.0.0/24", "", []string{"not-an-address"}); err == nil {
		t.Error("NewPool accepted a DNS server that is not an address")
	}
	if _, err := NewPool("p", "10.0.0.0/30", "2001:db8::/126", nil); err != nil {
		t.Errorf("NewPool refused the /30 and /126 boundaries: %v", err)
	}
}

// VALIDATES: a registry refuses to release a lease it never granted or already freed.
func TestVirtualIPPoolReleaseNotAllocated(t *testing.T) {
	p := mustPool(t, "10.10.0.0/24", "")
	foreign := mustAcquire(t, NewRegistry(), p, "a", FamilyIPv4, netip.Addr{})
	if err := NewRegistry().Release(foreign); !errors.Is(err, ErrNotAllocated) {
		t.Fatalf("release of another registry's lease: got %v, want ErrNotAllocated", err)
	}
	if err := NewRegistry().Release(nil); !errors.Is(err, ErrNotAllocated) {
		t.Fatalf("release of nil: got %v, want ErrNotAllocated", err)
	}
	// An in-pool address nobody was ever leased.
	never := &Lease{addr: netip.MustParseAddr("10.10.0.5"), family: FamilyIPv4, pool: "p", owner: "a", holders: 1}
	if err := NewRegistry().Release(never); !errors.Is(err, ErrNotAllocated) {
		t.Fatalf("release of a never-allocated address: got %v, want ErrNotAllocated", err)
	}
}

// VALIDATES: every address the registry hands out is distinct while the pool has addresses
// left. This is the registry's arithmetic; the gateway behavior is proven in the engine.
func TestVirtualIPPoolNeverHandsOneAddressTwice(t *testing.T) {
	p := mustPool(t, "10.10.0.0/28", "")
	r := NewRegistry()
	seen := make(map[netip.Addr]bool, 14)
	for i := range 14 {
		l := mustAcquire(t, r, p, strconv.Itoa(i), FamilyIPv4, netip.Addr{})
		if seen[l.Addr()] {
			t.Fatalf("allocation %d handed out %s a second time", i, l.Addr())
		}
		seen[l.Addr()] = true
	}
}

// VALIDATES: a free in-pool requested address is returned (AC-8); a held one, the network
// address, the broadcast address and an out-of-range one are not.
func TestRequestedAddressHonored(t *testing.T) {
	p := mustPool(t, "10.1.2.0/24", "2001:db8:1::/64")
	r := NewRegistry()
	want := netip.MustParseAddr("10.1.2.3")
	if l := mustAcquire(t, r, p, "a", FamilyIPv4, want); l.Addr() != want {
		t.Fatalf("requested %s, got %s", want, l.Addr())
	}
	if l := mustAcquire(t, r, p, "b", FamilyIPv4, want); l.Addr() == want {
		t.Fatalf("a held address was handed to a second owner")
	}
	for _, bad := range []string{"10.1.2.0", "10.1.2.255", "192.0.2.1", "0.0.0.0"} {
		l := mustAcquire(t, r, p, "c-"+bad, FamilyIPv4, netip.MustParseAddr(bad))
		if l.Addr().String() == bad {
			t.Errorf("request for %s was honored", bad)
		}
	}
	// IPv6 outside the prefix: its interface identifier is placed inside the prefix.
	l := mustAcquire(t, r, p, "d", FamilyIPv6, netip.MustParseAddr("fe80::abcd"))
	if want6 := netip.MustParseAddr("2001:db8:1::abcd"); l.Addr() != want6 {
		t.Fatalf("IPv6 interface identifier request: got %s, want %s", l.Addr(), want6)
	}
}

// VALIDATES: an owner that releases and reconnects while its address is free gets the same
// address back (AC-25), and a second Acquire for a live slot returns the same lease with one
// more holder, so the held count does not rise (AC-26 at the pool layer).
func TestOwnerGetsLastAddress(t *testing.T) {
	p := mustPool(t, "10.1.0.0/24", "")
	r := NewRegistry()
	first := mustAcquire(t, r, p, "alice", FamilyIPv4, netip.Addr{})
	mustAcquire(t, r, p, "bob", FamilyIPv4, netip.Addr{})

	again := mustAcquire(t, r, p, "alice", FamilyIPv4, netip.Addr{})
	if again != first {
		t.Fatal("a second Acquire for a live slot returned a different lease")
	}
	if got := r.Held("p"); got != 2 {
		t.Fatalf("held after a shared acquire: got %d, want 2", got)
	}
	if err := r.Release(again); err != nil {
		t.Fatal(err)
	}
	if !r.IsHeldForTest(first.Addr()) {
		t.Fatal("the lease was freed while a holder remained")
	}

	for cycle := range 5 {
		if err := r.Release(first); err != nil {
			t.Fatalf("cycle %d release: %v", cycle, err)
		}
		next := mustAcquire(t, r, p, "alice", FamilyIPv4, netip.Addr{})
		if next.Addr() != first.Addr() {
			t.Fatalf("cycle %d: reconnect got %s, want the previous %s", cycle, next.Addr(), first.Addr())
		}
		first = next
	}
	if got := r.Held("p"); got != 2 {
		t.Fatalf("held after the cycles: got %d, want 2", got)
	}
}

// VALIDATES: the rotating cursor does not hand the most recently released address to a
// different owner while another address is free (AC-27).
func TestRotatingCursor(t *testing.T) {
	p := mustPool(t, "10.1.0.0/29", "") // 6 hosts
	r := NewRegistry()
	a := mustAcquire(t, r, p, "a", FamilyIPv4, netip.Addr{})
	mustAcquire(t, r, p, "b", FamilyIPv4, netip.Addr{})
	if err := r.Release(a); err != nil {
		t.Fatal(err)
	}
	for i := range 4 {
		l := mustAcquire(t, r, p, "n"+strconv.Itoa(i), FamilyIPv4, netip.Addr{})
		if l.Addr() == a.Addr() {
			t.Fatalf("lease %d reused the just-released %s while other addresses were free", i, a.Addr())
		}
	}
	// Only the released address is left now, so the next owner gets it.
	if l := mustAcquire(t, r, p, "last", FamilyIPv4, netip.Addr{}); l.Addr() != a.Addr() {
		t.Fatalf("the only free address %s was not leased, got %s", a.Addr(), l.Addr())
	}
}

// VALIDATES: concurrent acquire and release from many goroutines never leases one address
// to two owners, and the race detector finds no unguarded access (AC-17).
func TestRegistryConcurrentNoDuplicate(t *testing.T) {
	p := mustPool(t, "10.2.0.0/24", "2001:db8:2::/112")
	r := NewRegistry()
	var (
		mu    sync.Mutex
		owner = make(map[netip.Addr]string)
		wg    sync.WaitGroup
		fail  = make(chan string, 64)
	)
	for g := range 32 {
		wg.Go(func() {
			name := "g" + strconv.Itoa(g)
			for i := range 50 {
				f := FamilyIPv4
				if i%2 == 1 {
					f = FamilyIPv6
				}
				l, err := r.Acquire(p, name, f, netip.Addr{})
				if err != nil {
					fail <- err.Error()
					return
				}
				mu.Lock()
				if prev, ok := owner[l.Addr()]; ok && prev != name {
					mu.Unlock()
					fail <- l.Addr().String() + " held by " + prev + " and " + name
					return
				}
				owner[l.Addr()] = name
				mu.Unlock()

				mu.Lock()
				delete(owner, l.Addr())
				mu.Unlock()
				if err := r.Release(l); err != nil {
					fail <- err.Error()
					return
				}
			}
		})
	}
	wg.Wait()
	close(fail)
	for msg := range fail {
		t.Error(msg)
	}
	if got := r.Held("p"); got != 0 {
		t.Fatalf("held after every release: %d", got)
	}
}
