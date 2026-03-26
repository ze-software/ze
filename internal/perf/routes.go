// Design: (none -- new tool, predates documentation)
// Overview: benchmark.go -- prefix generation

package perf

import (
	"math/rand"
	"net/netip"
)

// GenerateIPv4Routes produces count unique IPv4/24 prefixes deterministically
// from the given seed. Avoids reserved ranges (0/8, 10/8, 127/8, 169.254/16,
// 172.16/12, 192.168/16, 224+). Results are shuffled deterministically.
func GenerateIPv4Routes(seed uint64, count int) []netip.Prefix {
	if count <= 0 {
		return nil
	}

	//nolint:gosec // Deterministic RNG from seed -- not for cryptography.
	rng := rand.New(rand.NewSource(int64(seed)))

	candidates := ipv4CandidatePool()

	if count > len(candidates) {
		count = len(candidates)
	}

	// Partial Fisher-Yates: only shuffle the first count positions.
	// Produces the same first-count elements as a full shuffle would.
	for i := range count {
		j := i + rng.Intn(len(candidates)-i)
		candidates[i], candidates[j] = candidates[j], candidates[i]
	}

	result := make([]netip.Prefix, count)
	copy(result, candidates[:count])

	return result
}

// GenerateIPv6Routes produces count unique IPv6/48 prefixes deterministically
// from the given seed. Uses 2001:db8::/32 documentation range. Results are
// shuffled deterministically.
func GenerateIPv6Routes(seed uint64, count int) []netip.Prefix {
	if count <= 0 {
		return nil
	}

	//nolint:gosec // Deterministic RNG from seed -- not for cryptography.
	rng := rand.New(rand.NewSource(int64(seed) ^ 0x4950_7636))

	candidates := ipv6CandidatePool()

	if count > len(candidates) {
		count = len(candidates)
	}

	// Partial Fisher-Yates: only shuffle the first count positions.
	for i := range count {
		j := i + rng.Intn(len(candidates)-i)
		candidates[i], candidates[j] = candidates[j], candidates[i]
	}

	result := make([]netip.Prefix, count)
	copy(result, candidates[:count])

	return result
}

// ipv4CandidatePool builds all routable /24 prefixes from non-reserved first octets.
// Usable first octets: 1-9, 11-126, 128-169, 171, 173-191, 193-223.
// This excludes: 0/8, 10/8, 127/8, 169.254/16, 172.16-31/12, 192.168/16, 224+/4.
func ipv4CandidatePool() []netip.Prefix {
	// Estimate: ~221 first octets * 256 * 256 = ~14M candidates.
	// We only need to exclude a few second-octet ranges for 169 and 172.
	pool := make([]netip.Prefix, 0, 14_000_000)

	for first := range 224 {
		if first == 0 || first == 10 || first == 127 {
			continue
		}

		for second := range 256 {
			// Skip 169.254.0.0/16.
			if first == 169 && second == 254 {
				continue
			}

			// Skip 172.16.0.0/12 (172.16-31.x.x).
			if first == 172 && second >= 16 && second <= 31 {
				continue
			}

			// Skip 192.168.0.0/16.
			if first == 192 && second == 168 {
				continue
			}

			for third := range 256 {
				addr := netip.AddrFrom4([4]byte{byte(first), byte(second), byte(third), 0})
				pool = append(pool, netip.PrefixFrom(addr, 24))
			}
		}
	}

	return pool
}

// ipv6CandidatePool builds /48 prefixes in the 2001:db8::/32 documentation range.
// Varies the 5th and 6th bytes (positions [4] and [5]) for up to 65536 candidates.
func ipv6CandidatePool() []netip.Prefix {
	pool := make([]netip.Prefix, 0, 65536)

	for hi := range 256 {
		for lo := range 256 {
			var raw [16]byte
			raw[0] = 0x20
			raw[1] = 0x01
			raw[2] = 0x0d
			raw[3] = 0xb8
			raw[4] = byte(hi)
			raw[5] = byte(lo)

			addr := netip.AddrFrom16(raw)
			pool = append(pool, netip.PrefixFrom(addr, 48))
		}
	}

	return pool
}
