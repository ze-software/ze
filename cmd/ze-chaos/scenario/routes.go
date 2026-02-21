// Design: docs/architecture/chaos-web-dashboard.md — scenario generation

package scenario

import (
	"math/rand"
	"net/netip"
)

// GenerateIPv4Routes produces count unique IPv4 prefixes deterministically
// from the given seed and peer index. Different peer indices are guaranteed
// to produce non-overlapping routes.
//
// totalPeers controls address space partitioning: each peer gets
// len(usable)/totalPeers first octets. With fewer peers, each gets more
// address space, supporting 1M+ routes without prefix length subdivision.
//
// Prefix length starts at /24 and automatically increases to /25, /26, ... /32
// when count exceeds the /24 pool capacity for the peer's allocated octets.
//
// The address space is partitioned by peer index using first-octet slicing.
// Within each partition, prefixes are shuffled deterministically.
func GenerateIPv4Routes(seed uint64, peerIndex, count, totalPeers int) []netip.Prefix {
	// Create a per-peer RNG by combining seed and peer index.
	//nolint:gosec // Deterministic RNG from seed — not for cryptography.
	rng := rand.New(rand.NewSource(int64(seed) ^ int64(peerIndex*0x9E3779B9)))

	// Generate candidate prefixes avoiding reserved ranges.
	// Use a large pool and shuffle, then take the first count.
	candidates := generateCandidatePool(peerIndex, count, totalPeers)

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

// generateCandidatePool creates a pool of prefixes for the given peer.
// Each peer gets a non-overlapping slice of the address space to ensure
// no two peers generate the same prefix.
//
// Address space partitioning:
// - Usable first octets: 1-9, 11-126, 128-223 = 221 octets
// - Each peer gets len(usable)/totalPeers first-octets
// - Each first-octet at /24 generates 256*256 = 65536 prefixes
// - Example: 3 peers → 73 octets each → 4,784,128 /24 prefixes per peer
//
// When count exceeds the /24 pool, the prefix length increases (/25, /26, ...)
// to subdivide each /24 block, doubling capacity per step up to /32.
func generateCandidatePool(peerIndex, count, totalPeers int) []netip.Prefix {
	usable := usableFirstOctets()

	// Each peer gets a proportional slice of first octets.
	// With fewer peers, each gets more address space.
	octetsPerPeer := max(len(usable)/max(totalPeers, 1), 1)

	startIdx := peerIndex * octetsPerPeer
	endIdx := startIdx + octetsPerPeer
	if startIdx >= len(usable) {
		startIdx = len(usable) - 1
	}
	if endIdx > len(usable) {
		endIdx = len(usable)
	}

	myOctets := usable[startIdx:endIdx]

	// Limit octets to what's needed for the requested count.
	// Each octet at /24 yields 256*256 = 65536 candidates.
	// Generating millions of candidates for a small count is wasteful.
	candidatesPerOctet := 256 * 256
	octetsNeeded := max((count+candidatesPerOctet-1)/candidatesPerOctet, 1)
	if octetsNeeded < len(myOctets) {
		myOctets = myOctets[:octetsNeeded]
	}

	// Pick the smallest prefix length (starting at /24) that yields enough
	// candidates. Each step from /N to /(N+1) doubles the pool.
	prefixLen := 24
	poolSize := len(myOctets) * 256 * 256
	for poolSize < count && prefixLen < 32 {
		prefixLen++
		poolSize *= 2
	}

	// Number of sub-prefixes within each /24 block and their address step.
	subnets := 1 << (prefixLen - 24) // e.g. /24→1, /25→2, /26→4
	step := 256 / subnets            // e.g. /24→256, /25→128, /26→64

	pool := make([]netip.Prefix, 0, poolSize)
	for _, first := range myOctets {
		for second := range 256 {
			for third := range 256 {
				for s := range subnets {
					addr := netip.AddrFrom4([4]byte{first, byte(second), byte(third), byte(s * step)})
					pool = append(pool, netip.PrefixFrom(addr, prefixLen))
				}
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
