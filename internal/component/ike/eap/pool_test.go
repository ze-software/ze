// Design: plan/learned/744-ipsec-9-ikev2-eap-nat.md -- Virtual IP pool tests

package eap

import (
	"errors"
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
