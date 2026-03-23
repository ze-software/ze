package perf

import (
	"net/netip"
	"testing"
)

// VALIDATES: Deterministic IPv4 route generation produces correct, unique, non-reserved /24 prefixes.
// PREVENTS: Reserved range leakage, duplicate prefixes, non-deterministic output.
func TestBuildRoutes(t *testing.T) {
	tests := []struct {
		name  string
		seed  uint64
		count int
	}{
		{name: "100 routes seed 42", seed: 42, count: 100},
		{name: "1000 routes seed 99", seed: 99, count: 1000},
		{name: "zero routes", seed: 1, count: 0},
		{name: "one route", seed: 7, count: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routes := GenerateIPv4Routes(tt.seed, tt.count)

			if len(routes) != tt.count {
				t.Fatalf("expected %d routes, got %d", tt.count, len(routes))
			}

			seen := make(map[netip.Prefix]bool, len(routes))
			for i, p := range routes {
				// All must be /24.
				if p.Bits() != 24 {
					t.Errorf("route[%d] = %s: expected /24, got /%d", i, p, p.Bits())
				}

				// Must be valid.
				if !p.IsValid() {
					t.Errorf("route[%d] = %s: not valid", i, p)
				}

				// Must not be in reserved ranges.
				addr := p.Addr()
				first := addr.As4()[0]
				if isReservedFirstOctet(first) {
					t.Errorf("route[%d] = %s: first octet %d is reserved", i, p, first)
				}

				// No duplicates.
				if seen[p] {
					t.Errorf("route[%d] = %s: duplicate", i, p)
				}
				seen[p] = true
			}
		})
	}
}

// VALIDATES: Deterministic IPv6 route generation produces correct, unique /48 prefixes.
// PREVENTS: Duplicate prefixes, wrong prefix length, non-deterministic output.
func TestBuildRoutesIPv6(t *testing.T) {
	tests := []struct {
		name  string
		seed  uint64
		count int
	}{
		{name: "100 routes seed 42", seed: 42, count: 100},
		{name: "500 routes seed 99", seed: 99, count: 500},
		{name: "zero routes", seed: 1, count: 0},
		{name: "one route", seed: 7, count: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routes := GenerateIPv6Routes(tt.seed, tt.count)

			if len(routes) != tt.count {
				t.Fatalf("expected %d routes, got %d", tt.count, len(routes))
			}

			seen := make(map[netip.Prefix]bool, len(routes))
			for i, p := range routes {
				// All must be /48.
				if p.Bits() != 48 {
					t.Errorf("route[%d] = %s: expected /48, got /%d", i, p, p.Bits())
				}

				// Must be valid.
				if !p.IsValid() {
					t.Errorf("route[%d] = %s: not valid", i, p)
				}

				// Must be in 2001:db8::/32 range.
				raw := p.Addr().As16()
				if raw[0] != 0x20 || raw[1] != 0x01 || raw[2] != 0x0d || raw[3] != 0xb8 {
					t.Errorf("route[%d] = %s: not in 2001:db8::/32 range", i, p)
				}

				// No duplicates.
				if seen[p] {
					t.Errorf("route[%d] = %s: duplicate", i, p)
				}
				seen[p] = true
			}
		})
	}
}

// VALIDATES: Same seed produces identical routes; different seeds produce different routes.
// PREVENTS: Non-deterministic behavior, seed collision.
func TestRoutesDeterministic(t *testing.T) {
	tests := []struct {
		name string
		gen  func(seed uint64, count int) []netip.Prefix
	}{
		{name: "ipv4", gen: GenerateIPv4Routes},
		{name: "ipv6", gen: GenerateIPv6Routes},
	}

	for _, tt := range tests {
		t.Run(tt.name+"/same seed identical", func(t *testing.T) {
			a := tt.gen(42, 100)
			b := tt.gen(42, 100)

			if len(a) != len(b) {
				t.Fatalf("length mismatch: %d vs %d", len(a), len(b))
			}

			for i := range a {
				if a[i] != b[i] {
					t.Errorf("index %d: %s != %s", i, a[i], b[i])
				}
			}
		})

		t.Run(tt.name+"/different seed differs", func(t *testing.T) {
			a := tt.gen(42, 100)
			b := tt.gen(99, 100)

			if len(a) != len(b) {
				t.Fatalf("length mismatch: %d vs %d", len(a), len(b))
			}

			same := 0
			for i := range a {
				if a[i] == b[i] {
					same++
				}
			}

			// Allow up to 5% coincidental matches.
			if same > len(a)/20 {
				t.Errorf("too many identical routes (%d/%d) between different seeds", same, len(a))
			}
		})
	}
}

// isReservedFirstOctet checks if an IPv4 first octet falls in a reserved range.
// Used by tests only to validate generator output.
func isReservedFirstOctet(b byte) bool {
	switch {
	case b == 0: // 0.0.0.0/8
		return true
	case b == 10: // 10.0.0.0/8
		return true
	case b == 127: // 127.0.0.0/8
		return true
	case b >= 224: // multicast + reserved
		return true
	}
	return false
}
