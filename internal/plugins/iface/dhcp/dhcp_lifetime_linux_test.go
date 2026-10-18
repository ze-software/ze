//go:build linux

// Design: docs/features/interfaces.md -- DHCPv4 client lifecycle
// Related: dhcp_v4_linux.go -- v4AddressLifetime, v4RenewalTimes

package ifacedhcp

import (
	"testing"
	"time"
)

// TestV4AddressLifetimeIsTheLease checks the lifetime the leased address is
// installed with, for finite leases and for the lease RFC 2131 Section 3.3
// reserves as infinite.
//
// VALIDATES: a finite lease installs its own seconds; 0xffffffff installs the
// kernel's infinite lifetime, never a value just below it.
// PREVENTS: a lease whose installed lifetime differs from what the server
// granted.
func TestV4AddressLifetimeIsTheLease(t *testing.T) {
	cases := []struct {
		name  string
		lease time.Duration
		want  int
	}{
		{"one second", time.Second, 1},
		{"one hour", time.Hour, 3600},
		{"longest finite", 0xfffffffe * time.Second, 0xfffffffe},
		{"infinite", v4LeaseInfinite, 0xffffffff},
	}
	for _, tc := range cases {
		if got := v4AddressLifetime(tc.lease); got != tc.want {
			t.Errorf("%s: v4AddressLifetime(%v) = %d, want %d", tc.name, tc.lease, got, tc.want)
		}
	}
}

// TestV4RenewalTimesNeverOverflow checks T1 and T2 against RFC 2131 Section
// 4.4.5 defaults, including the longest finite lease.
//
// VALIDATES: T1 is half the lease and T2 seven eighths, both positive and
// T1 < T2 < lease, for a lease up to 0xfffffffe seconds.
// PREVENTS: leaseTime*7/8 overflowing time.Duration and turning T2 negative
// for a lease above 41 years.
func TestV4RenewalTimesNeverOverflow(t *testing.T) {
	for _, lease := range []time.Duration{8 * time.Second, time.Hour, 0xfffffffe * time.Second} {
		t1, t2 := v4RenewalTimes(lease)
		if t1 != lease/2 {
			t.Errorf("lease %v: T1 = %v, want %v", lease, t1, lease/2)
		}
		if t2 != lease-lease/8 {
			t.Errorf("lease %v: T2 = %v, want %v", lease, t2, lease-lease/8)
		}
		if t1 <= 0 || t1 >= t2 || t2 >= lease {
			t.Errorf("lease %v: want 0 < T1 (%v) < T2 (%v) < lease", lease, t1, t2)
		}
	}
	if t1, t2 := v4RenewalTimes(8 * time.Second); t1 != 4*time.Second || t2 != 7*time.Second {
		t.Errorf("8s lease: T1, T2 = %v, %v, want 4s, 7s", t1, t2)
	}
}
