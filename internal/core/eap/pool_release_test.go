// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- Virtual IP pool for road warrior clients
// RFC: rfc/short/rfc7296.md -- Configuration Payload, INTERNAL_IP6_ADDRESS (Section 2.19)

package eap

import (
	"errors"
	"net/netip"
	"testing"
)

// VALIDATES: only the lease a registry handed out can release that address, and an
// address that shares the low 64 bits of a lease is a different address.
// PREVENTS: the aliasing defect of the old per-pool identifier map, where in a /48
// 2001:db8::1 and 2001:db8:0:1::1 mapped to one identifier, so releasing the second freed
// the first client's lease and the pool then handed that address to a second client.
// The registry keys every lease by its full address, so this test releases a forged lease
// naming the alias (refused), then asks for the alias as a requested address while the
// original is held and checks it is leased as asked and both stay distinct.
func TestPoolV6ReleaseRejectsAliasedAddress(t *testing.T) {
	p, err := NewPool("p", "", "2001:db8::/48", nil)
	if err != nil {
		t.Fatalf("build the pool: %v", err)
	}
	r := NewRegistry()

	leased, err := r.Acquire(p, "alice", FamilyIPv6, netip.Addr{})
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	// Same low 64 bits, different bits inside the /48's host part.
	a := leased.Addr().As16()
	a[7] ^= 0x01
	alias := netip.AddrFrom16(a)
	if alias == leased.Addr() {
		t.Fatal("the fixture did not produce a different address")
	}
	if !p.Contains(alias) {
		t.Fatal("the fixture left the prefix, so it does not exercise the aliasing")
	}

	// A lease the registry never granted, naming the alias, frees nothing.
	forged := &Lease{addr: alias, family: FamilyIPv6, pool: "p", owner: "alice", holders: 1}
	if err := r.Release(forged); !errors.Is(err, ErrNotAllocated) {
		t.Fatalf("releasing an address the registry never leased returned %v; it freed "+
			"another client's lease", err)
	}
	if !r.IsHeldForTest(leased.Addr()) {
		t.Fatal("the rejected release of the alias freed the genuine lease")
	}

	other, err := r.Acquire(p, "bob", FamilyIPv6, alias)
	if err != nil {
		t.Fatalf("acquire the alias: %v", err)
	}
	if other.Addr() != alias {
		t.Fatalf("the free in-prefix alias %s was not leased as requested, got %s", alias, other.Addr())
	}
	if !r.IsHeldForTest(alias) {
		t.Fatalf("the alias %s is not held after it was leased", alias)
	}
	if !r.IsHeldForTest(leased.Addr()) {
		t.Fatalf("leasing the alias freed the original %s", leased.Addr())
	}
	if other.Addr() == leased.Addr() {
		t.Fatal("the registry leased one address to two clients")
	}
	if err := r.Release(other); err != nil {
		t.Fatalf("release the alias lease: %v", err)
	}
	if !r.IsHeldForTest(leased.Addr()) {
		t.Fatal("releasing the alias freed another client's lease")
	}

	if err := r.Release(leased); err != nil {
		t.Fatalf("the genuine holder was refused its own release: %v", err)
	}
	if err := r.Release(leased); !errors.Is(err, ErrNotAllocated) {
		t.Fatalf("a second release of a freed lease returned %v, want ErrNotAllocated", err)
	}
}

// VALIDATES: the round trip holds for prefixes on both sides of /64, where the host mask
// changes shape.
// PREVENTS: a fix that rejects an aliased address by also rejecting the real one.
func TestPoolV6ReleaseAcceptsItsOwnLeases(t *testing.T) {
	for _, cidr := range []string{"2001:db8::/48", "2001:db8::/64", "2001:db8::/112"} {
		t.Run(cidr, func(t *testing.T) {
			p, err := NewPool("p", "", cidr, nil)
			if err != nil {
				t.Fatalf("build the pool: %v", err)
			}
			r := NewRegistry()
			var leases []*Lease
			for i := range 4 {
				got, acqErr := r.Acquire(p, string(rune('a'+i)), FamilyIPv6, netip.Addr{})
				if acqErr != nil {
					t.Fatalf("acquire: %v", acqErr)
				}
				if !p.Contains(got.Addr()) {
					t.Fatalf("the pool leased %s from outside its own prefix", got.Addr())
				}
				leases = append(leases, got)
			}
			for _, l := range leases {
				if err := r.Release(l); err != nil {
					t.Fatalf("the registry refused to release its own lease %s: %v", l.Addr(), err)
				}
			}
		})
	}
}
