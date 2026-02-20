// Design: docs/architecture/chaos-web-dashboard.md — scenario generation

package scenario

import (
	"math/rand"
	"net/netip"
)

// GenerateIPv6Routes produces count unique /48 prefixes from the 2001:db8::/32
// documentation range (RFC 3849), deterministically from the given seed and
// peer index. Different peer indices produce non-overlapping routes.
//
// Address space partitioning:
// - 2001:0db8:PPSS::/48 where PP = peer-specific and SS = sequence
// - Each peer gets a slice of the 5th byte (bytes[4]) values
// - Within each 5th-byte value, 256 /48 prefixes (varying byte[5])
// - Total per peer: octetsPerPeer × 256 = at least 1280 prefixes.
func GenerateIPv6Routes(seed uint64, peerIndex, count int) []netip.Prefix {
	//nolint:gosec // Deterministic RNG from seed — not for cryptography.
	rng := rand.New(rand.NewSource(int64(seed) ^ int64(peerIndex*0x9E3779B9)))

	candidates := generateIPv6CandidatePool(peerIndex)

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

// generateIPv6CandidatePool creates a pool of /48 prefixes for the given peer.
// Each peer gets non-overlapping slices of 2001:db8:XX:YY::/48 space.
//
// Partitioning: 256 values for byte[4] (the 5th byte in the 16-byte address),
// each peer gets 5 values (256/51 = 5), giving 5 × 256 = 1280 /48 prefixes.
func generateIPv6CandidatePool(peerIndex int) []netip.Prefix {
	// byte[4] ranges from 0x00 to 0xFF (256 values).
	const totalByte4Values = 256
	valuesPerPeer := max(totalByte4Values/51, 1) // 51 to ensure 50 peers fit

	startVal := peerIndex * valuesPerPeer
	endVal := startVal + valuesPerPeer
	if startVal >= totalByte4Values {
		startVal = totalByte4Values - 1
	}
	if endVal > totalByte4Values {
		endVal = totalByte4Values
	}

	// Generate 2001:0db8:00XX:00YY::/48 for each (XX, YY) combination.
	pool := make([]netip.Prefix, 0, (endVal-startVal)*256)
	for b4 := startVal; b4 < endVal; b4++ {
		for b5 := range 256 {
			//nolint:gosec // G602 false positive: [16]byte is fixed-size, indices 0-5 always valid.
			addr := [16]byte{0x20, 0x01, 0x0d, 0xb8, byte(b4), byte(b5)}
			pool = append(pool, netip.PrefixFrom(netip.AddrFrom16(addr), 48))
		}
	}

	return pool
}
