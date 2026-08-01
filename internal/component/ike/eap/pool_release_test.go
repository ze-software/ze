// Design: plan/learned/744-ipsec-9-ikev2-eap-nat.md -- Virtual IP pool for road warrior clients
// RFC: rfc/short/rfc7296.md -- Configuration Payload, INTERNAL_IP6_ADDRESS (Section 2.19)

package eap

import (
	"errors"
	"net"
	"testing"
)

// VALIDATES: only the exact address a pool leased can release that lease.
// PREVENTS: the host-bit mask folding many addresses onto one identifier. host6 is
// ^uint64(0) for any prefix shorter than /64. In a /48 the low 64 bits alone name the
// lease, and every bit between the prefix and bit 64 is discarded. 2001:db8::1 and
// 2001:db8:0:1::1 mapped to the same identifier, and net.IPNet.Contains admitted both.
// Releasing the second freed the first client's lease. The pool then handed that
// address to a second client while the first still held it.
func TestPoolV6ReleaseRejectsAliasedAddress(t *testing.T) {
	p, err := NewPool("", "2001:db8::/48", nil, "")
	if err != nil {
		t.Fatalf("build the pool: %v", err)
	}

	leased, err := p.Allocate()
	if err != nil {
		t.Fatalf("allocate: %v", err)
	}
	if leased.IPv6 == nil {
		t.Fatal("the pool leased no IPv6 address")
	}

	// Same low 64 bits, different bits inside the /48's host part. It is inside the
	// prefix, so Contains admits it, and it is NOT an address this pool ever leased.
	alias := make(net.IP, 16)
	copy(alias, leased.IPv6)
	alias[7] ^= 0x01
	if alias.Equal(leased.IPv6) {
		t.Fatal("the fixture did not produce a different address")
	}
	if !p.net6.Contains(alias) {
		t.Fatal("the fixture left the prefix, so it does not exercise the aliasing")
	}

	if err := p.Release(alias); !errors.Is(err, ErrNotAllocated) {
		t.Fatalf("releasing an address the pool never leased returned %v; it freed "+
			"another client's lease", err)
	}

	// The lease must still be held, so the next client cannot be given it.
	next, err := p.Allocate()
	if err != nil {
		t.Fatalf("allocate after the rejected release: %v", err)
	}
	if next.IPv6.Equal(leased.IPv6) {
		t.Fatal("the pool leased one address to two clients")
	}

	// The genuine holder still releases, and the address then comes back.
	if err := p.Release(leased.IPv6); err != nil {
		t.Fatalf("the address the pool leased was refused its own release: %v", err)
	}
	if err := p.Release(leased.IPv6); !errors.Is(err, ErrNotAllocated) {
		t.Fatalf("a second release of a freed address returned %v", err)
	}
}

// VALIDATES: the round trip holds for prefixes on both sides of /64, where host6 changes
// shape.
// PREVENTS: a fix that rejects the aliased address by also rejecting the real one.
func TestPoolV6ReleaseAcceptsItsOwnLeases(t *testing.T) {
	for _, cidr := range []string{"2001:db8::/48", "2001:db8::/64", "2001:db8::/112"} {
		t.Run(cidr, func(t *testing.T) {
			p, err := NewPool("", cidr, nil, "")
			if err != nil {
				t.Fatalf("build the pool: %v", err)
			}
			var leases []net.IP
			for range 4 {
				got, allocErr := p.Allocate()
				if allocErr != nil {
					t.Fatalf("allocate: %v", allocErr)
				}
				if !p.net6.Contains(got.IPv6) {
					t.Fatalf("the pool leased %s from outside its own prefix", got.IPv6)
				}
				leases = append(leases, got.IPv6)
			}
			for _, ip := range leases {
				if err := p.Release(ip); err != nil {
					t.Fatalf("the pool refused to release its own lease %s: %v", ip, err)
				}
			}
		})
	}
}
