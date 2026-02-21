// Design: docs/architecture/chaos-web-dashboard.md — scenario generation

package scenario

import (
	"math/rand"
	"net/netip"
)

// GenerateIPv6Routes produces count unique IPv6 prefixes from the 2001:db8::/32
// documentation range (RFC 3849), deterministically from the given seed and
// peer index. Different peer indices produce non-overlapping routes.
//
// Prefix length starts at /48 (1,280 per peer) and automatically increases
// to /49, /50, ... /64 when count exceeds the /48 pool capacity.
func GenerateIPv6Routes(seed uint64, peerIndex, count, totalPeers int) []netip.Prefix {
	//nolint:gosec // Deterministic RNG from seed — not for cryptography.
	rng := rand.New(rand.NewSource(int64(seed) ^ int64(peerIndex*0x9E3779B9)))

	candidates := generateIPv6CandidatePool(peerIndex, count, totalPeers)

	rng.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	if count > len(candidates) {
		count = len(candidates)
	}

	result := make([]netip.Prefix, count)
	copy(result, candidates[:count])

	return result
}

// generateIPv6CandidatePool creates a pool of IPv6 prefixes for the given peer.
// Each peer gets non-overlapping slices of 2001:db8:XX:YY::/48 space.
//
// Partitioning: 256 values for byte[4] (the 5th byte in the 16-byte address),
// each peer gets 5 values (256/51 = 5), giving 5 × 256 = 1280 /48 prefixes.
//
// When count exceeds the /48 pool, the prefix length increases (/49, /50, ...)
// to subdivide each /48 block, doubling capacity per step up to /64.
func generateIPv6CandidatePool(peerIndex, count, totalPeers int) []netip.Prefix {
	// byte[4] ranges from 0x00 to 0xFF (256 values).
	const totalByte4Values = 256
	valuesPerPeer := max(totalByte4Values/max(totalPeers, 1), 1)

	startVal := peerIndex * valuesPerPeer
	endVal := startVal + valuesPerPeer
	if startVal >= totalByte4Values {
		startVal = totalByte4Values - 1
	}
	if endVal > totalByte4Values {
		endVal = totalByte4Values
	}

	nByte4 := endVal - startVal

	// Pick the smallest prefix length (starting at /48) that yields enough
	// candidates. Each step from /N to /(N+1) doubles the pool.
	prefixLen := 48
	poolSize := nByte4 * 256 // base: nByte4 × 256 /48 prefixes
	for poolSize < count && prefixLen < 64 {
		prefixLen++
		poolSize *= 2
	}

	// Sub-prefixes within each /48 block occupy bits 48..(prefixLen-1),
	// which fall in bytes[6:7] of the 16-byte address.
	subnets := 1 << (prefixLen - 48) // e.g. /48→1, /49→2, /56→256
	step := 65536 / subnets          // step in the 16-bit [byte6:byte7] space

	pool := make([]netip.Prefix, 0, poolSize)
	for b4 := startVal; b4 < endVal; b4++ {
		for b5 := range 256 {
			for s := range subnets {
				val := s * step
				//nolint:gosec // G602 false positive: [16]byte is fixed-size, indices 0-7 always valid.
				addr := [16]byte{0x20, 0x01, 0x0d, 0xb8, byte(b4), byte(b5), byte(val >> 8), byte(val & 0xFF)}
				pool = append(pool, netip.PrefixFrom(netip.AddrFrom16(addr), prefixLen))
			}
		}
	}

	return pool
}
