package scenario

import (
	"math/rand"
	"net/netip"
)

// GenerateIPv4Routes produces count unique /24 prefixes deterministically
// from the given seed and peer index. Different peer indices are guaranteed
// to produce non-overlapping routes.
//
// The address space is partitioned by peer index:
// - Peer 0 uses 10.0.0.0/8 range starting at 10.0.0.0
// - Peer 1 uses 10.0.0.0/8 range starting at 10.64.0.0
// - etc.
// Within each partition, prefixes are shuffled deterministically.
func GenerateIPv4Routes(seed uint64, peerIndex, count int) []netip.Prefix {
	// Create a per-peer RNG by combining seed and peer index.
	//nolint:gosec // Deterministic RNG from seed — not for cryptography.
	rng := rand.New(rand.NewSource(int64(seed) ^ int64(peerIndex*0x9E3779B9)))

	// Generate candidate /24 prefixes avoiding reserved ranges.
	// Use a large pool and shuffle, then take the first count.
	candidates := generateCandidatePool(peerIndex)

	// Shuffle deterministically.
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

// generateCandidatePool creates a pool of /24 prefixes for the given peer.
// Each peer gets a non-overlapping slice of the address space to ensure
// no two peers generate the same prefix.
//
// Address space partitioning:
// - Usable first octets: 1-9, 11-126, 128-223 = 221 octets
// - Each peer gets 4 first-octets (221/51 = 4)
// - Each first-octet generates 256*256 = 65536 /24 prefixes (second.third.0/24)
// - Total per peer: 4 * 65536 = 262,144 prefixes (enough for any --heavy-routes).
func generateCandidatePool(peerIndex int) []netip.Prefix {
	usable := usableFirstOctets()

	// Each peer gets a slice of first octets.
	octetsPerPeer := max(len(usable)/51, 1) // 51 to ensure 50 peers fit

	startIdx := peerIndex * octetsPerPeer
	endIdx := startIdx + octetsPerPeer
	if startIdx >= len(usable) {
		startIdx = len(usable) - 1
	}
	if endIdx > len(usable) {
		endIdx = len(usable)
	}

	myOctets := usable[startIdx:endIdx]

	// Generate first.second.third.0/24 for all combinations.
	pool := make([]netip.Prefix, 0, len(myOctets)*256*256)
	for _, first := range myOctets {
		for second := range 256 {
			for third := range 256 {
				addr := netip.AddrFrom4([4]byte{first, byte(second), byte(third), 0})
				pool = append(pool, netip.PrefixFrom(addr, 24))
			}
		}
	}

	return pool
}

// usableFirstOctets returns IPv4 first octets suitable for route generation:
// 1-9, 11-126, 128-223 (excludes 0, 10, 127, and 224+).
func usableFirstOctets() []byte {
	octets := make([]byte, 0, 221)
	for i := 1; i <= 223; i++ {
		if i == 10 || i == 127 {
			continue
		}
		octets = append(octets, byte(i))
	}
	return octets
}
