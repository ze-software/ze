// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- Virtual IP pool tests

package eap

import (
	"errors"
	"net"
	"testing"
)

func TestVirtualIPPoolAllocate(t *testing.T) {
	pool, err := NewPool("10.10.0.0/24", "", []string{"8.8.8.8"}, "example.com")
	if err != nil {
		t.Fatal(err)
	}

	result, err := pool.Allocate()
	if err != nil {
		t.Fatal(err)
	}
	if result.IPv4 == nil {
		t.Fatal("expected IPv4 allocation")
	}
	if result.IPv4.String() == "10.10.0.0" {
		t.Fatal("should not allocate network address")
	}
	if result.IPv4.String() != "10.10.0.1" {
		t.Fatalf("first allocation: got %s, want 10.10.0.1", result.IPv4)
	}
	if len(result.DNS4) != 1 || result.DNS4[0].String() != "8.8.8.8" {
		t.Fatalf("DNS: got %v", result.DNS4)
	}
	if result.Domain != "example.com" {
		t.Fatalf("Domain: got %q", result.Domain)
	}
}

func TestVirtualIPPoolRelease(t *testing.T) {
	pool, err := NewPool("10.10.0.0/30", "", nil, "")
	if err != nil {
		t.Fatal(err)
	}

	// /30 has 2 usable addresses (4 total - network - broadcast).
	r1, err := pool.Allocate()
	if err != nil {
		t.Fatal(err)
	}
	r2, err := pool.Allocate()
	if err != nil {
		t.Fatal(err)
	}

	// Pool should be exhausted.
	_, err = pool.Allocate()
	if !errors.Is(err, ErrPoolExhausted) {
		t.Fatalf("expected pool exhausted, got %v", err)
	}

	// Release first address.
	if err := pool.Release(r1.IPv4); err != nil {
		t.Fatal(err)
	}

	// Should be able to allocate again.
	r3, err := pool.Allocate()
	if err != nil {
		t.Fatal(err)
	}
	if !r3.IPv4.Equal(r1.IPv4) {
		t.Fatalf("reallocation: got %s, want %s", r3.IPv4, r1.IPv4)
	}
	_ = r2
}

// RFC requirement: RFC3948-5.1-1 negative -- the security gateway refuses rather than hand
// a second client an inner address that is already in use. RFC 3948 Section 5.1 warns that
// two remote peers reaching one SGW on the same inner address leave it unable to tell which
// SA a returning packet belongs to, so a pool that wrapped around under pressure would
// recreate exactly that conflict.
func TestVirtualIPPoolExhausted(t *testing.T) {
	pool, err := NewPool("10.10.0.0/30", "", nil, "")
	if err != nil {
		t.Fatal(err)
	}

	// /30 = 4 addresses, 2 usable.
	for range 2 {
		if _, err := pool.Allocate(); err != nil {
			t.Fatalf("allocation failed: %v", err)
		}
	}

	_, err = pool.Allocate()
	if !errors.Is(err, ErrPoolExhausted) {
		t.Fatalf("expected ErrPoolExhausted, got %v", err)
	}

	if pool.Available() != 0 {
		t.Fatalf("available: got %d, want 0", pool.Available())
	}
}

func TestVirtualIPPoolDualStack(t *testing.T) {
	pool, err := NewPool("10.10.0.0/24", "fd00::/112", []string{"8.8.8.8", "2001:4860:4860::8888"}, "")
	if err != nil {
		t.Fatal(err)
	}

	result, err := pool.Allocate()
	if err != nil {
		t.Fatal(err)
	}
	if result.IPv4 == nil {
		t.Fatal("expected IPv4")
	}
	if result.IPv6 == nil {
		t.Fatal("expected IPv6")
	}
	if len(result.DNS4) != 1 {
		t.Fatalf("DNS4 count: got %d, want 1", len(result.DNS4))
	}
	if len(result.DNS6) != 1 {
		t.Fatalf("DNS6 count: got %d, want 1", len(result.DNS6))
	}
}

// VALIDATES: every IPv6 prefix ValidateRemoteAccess accepts (/48 through /126) leases
// addresses that lie inside that prefix.
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
			pool, err := NewPool("", cidr, nil, "")
			if err != nil {
				t.Fatalf("NewPool(%s): %v", cidr, err)
			}
			_, ipNet, err := net.ParseCIDR(cidr)
			if err != nil {
				t.Fatalf("ParseCIDR: %v", err)
			}
			result, err := pool.Allocate()
			if err != nil {
				t.Fatalf("Allocate from %s: %v", cidr, err)
			}
			if result.IPv6 == nil {
				t.Fatalf("no IPv6 leased from %s", cidr)
			}
			if !ipNet.Contains(result.IPv6) {
				t.Errorf("leased %s from pool %s: address is outside the configured prefix",
					result.IPv6, cidr)
			}
			if result.IPv6.Equal(ipNet.IP) {
				t.Errorf("leased the network address %s from pool %s", result.IPv6, cidr)
			}
			// The lease round-trips: Release accepts the address it just handed out.
			if err := pool.Release(result.IPv6); err != nil {
				t.Errorf("Release(%s) from pool %s: %v", result.IPv6, cidr, err)
			}
		})
	}
}

// VALIDATES: NewPool bounds an IPv6 range itself instead of trusting its caller.
// PREVENTS: a pool built past the config validator computing a host width of zero and
// leasing the network address as though it were a client address.
func TestVirtualIPPoolV6RejectsUnrepresentableRange(t *testing.T) {
	if _, err := NewPool("", "2001:db8::1/128", nil, ""); err == nil {
		t.Error("NewPool accepted a /128 IPv6 range, which names no host address")
	}
}

// VALIDATES: Release refuses an address from outside the pool's own prefix.
// PREVENTS: the host-bit mask folding a foreign address onto an in-pool identifier, which
// would free a lease belonging to a different client.
func TestVirtualIPPoolV6ReleaseRejectsForeignAddress(t *testing.T) {
	pool, err := NewPool("", "2001:db8:0:0:1234:5678::/96", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	leased, err := pool.Allocate()
	if err != nil {
		t.Fatal(err)
	}
	// Same low-order host bits, different prefix: it must not free the lease above.
	foreign := net.ParseIP("2001:db8:0:0:9999:9999::1")
	if foreign == nil {
		t.Fatal("bad test fixture address")
	}
	if err := pool.Release(foreign); !errors.Is(err, ErrNotAllocated) {
		t.Fatalf("Release(%s): got %v, want ErrNotAllocated", foreign, err)
	}
	// The real lease is still held, so releasing it now still succeeds.
	if err := pool.Release(leased.IPv6); err != nil {
		t.Fatalf("Release(%s) after the foreign release: %v", leased.IPv6, err)
	}
}

func TestVirtualIPPoolReleaseNotAllocated(t *testing.T) {
	pool, err := NewPool("10.10.0.0/24", "", nil, "")
	if err != nil {
		t.Fatal(err)
	}

	err = pool.Release([]byte{10, 10, 0, 5})
	if !errors.Is(err, ErrNotAllocated) {
		t.Fatalf("expected ErrNotAllocated, got %v", err)
	}
}

// VALIDATES: every address the virtual IP pool hands out is distinct, for as long as the
// pool has addresses left.
// PREVENTS: two road warrior clients reaching one security gateway on the same inner
// address, which leaves the gateway holding two SAs that lead to that address and no way
// to choose between them for traffic coming back from the protected network.
//
// RFC requirement: RFC3948-5.1-1 positive -- ze prevents the RFC 3948 Section 5.1 conflict
// the way the section recommends: the gateway assigns each client a locally unique address
// instead of carrying the address the client brought with it.
func TestVirtualIPPoolNeverHandsOneAddressTwice(t *testing.T) {
	pool, err := NewPool("10.10.0.0/28", "", nil, "")
	if err != nil {
		t.Fatal(err)
	}

	// A /28 leaves 14 usable addresses once the network and broadcast addresses are out.
	seen := make(map[string]bool, 14)
	for i := range 14 {
		result, allocErr := pool.Allocate()
		if allocErr != nil {
			t.Fatalf("allocation %d failed: %v", i, allocErr)
		}
		addr := result.IPv4.String()
		if seen[addr] {
			t.Fatalf("allocation %d handed out %s a second time; two clients would share an inner address", i, addr)
		}
		seen[addr] = true
	}
}
