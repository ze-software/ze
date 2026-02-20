// Design: docs/architecture/chaos-web-dashboard.md — scenario generation

package scenario

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net/netip"
)

// VPNRoute represents a generated VPN route with RD, labels, and prefix.
type VPNRoute struct {
	// RDBytes is the Route Distinguisher in wire format (8 bytes).
	RDBytes [8]byte

	// Labels is the MPLS label stack.
	Labels []uint32

	// Prefix is the customer IPv4 or IPv6 prefix.
	Prefix netip.Prefix

	// Key is a unique string identifier for validation tracking.
	Key string
}

// GenerateVPNRoutes produces count unique VPN routes deterministically from
// the given seed and peer index. If ipv6 is true, generates IPv6 VPN routes;
// otherwise IPv4 VPN routes.
//
// RD format: Type 0 (2-byte admin + 4-byte assigned).
// Admin = peer index, Assigned = route sequence.
// Label: 100000 + peerIndex*1000 + sequence (20-bit range).
func GenerateVPNRoutes(seed uint64, peerIndex, count int, ipv6 bool) []VPNRoute {
	// Generate underlying prefixes first.
	var prefixes []netip.Prefix
	if ipv6 {
		prefixes = GenerateIPv6Routes(seed, peerIndex, count)
	} else {
		prefixes = GenerateIPv4Routes(seed, peerIndex, count)
	}

	if count > len(prefixes) {
		count = len(prefixes)
	}

	//nolint:gosec // Deterministic RNG from seed — not for cryptography.
	rng := rand.New(rand.NewSource(int64(seed) ^ int64(peerIndex*0x517CC1B7)))

	routes := make([]VPNRoute, count)
	for i := range count {
		// RD Type 0: 2-byte type (0x0000) + 2-byte admin (peerIndex) + 4-byte assigned (sequence).
		var rd [8]byte
		binary.BigEndian.PutUint16(rd[2:4], uint16(peerIndex)) //nolint:gosec // peerIndex max 50, fits uint16.
		binary.BigEndian.PutUint32(rd[4:8], uint32(i))         //nolint:gosec // i bounded by count.

		// Label: base + jitter for realism. 20-bit max = 1048575.
		label := uint32(100000 + peerIndex*1000 + rng.Intn(900)) //nolint:gosec // deterministic, not crypto.

		routes[i] = VPNRoute{
			RDBytes: rd,
			Labels:  []uint32{label},
			Prefix:  prefixes[i],
			Key:     fmt.Sprintf("%d:%d:%s", peerIndex, i, prefixes[i]),
		}
	}

	return routes
}
