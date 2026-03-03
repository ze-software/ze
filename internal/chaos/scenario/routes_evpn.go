// Design: docs/architecture/chaos-web-dashboard.md — scenario generation

package scenario

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"net/netip"
)

// EVPNRoute represents a generated EVPN Type-2 MAC/IP route.
type EVPNRoute struct {
	// RDBytes is the Route Distinguisher in wire format (8 bytes).
	RDBytes [8]byte

	// MAC is the 6-byte Ethernet MAC address.
	MAC [6]byte

	// IP is the associated IP address (IPv4 for chaos testing).
	IP netip.Addr

	// EthernetTag is the Ethernet tag ID.
	EthernetTag uint32

	// Labels is the MPLS label stack.
	Labels []uint32

	// Key is a unique string identifier for validation tracking.
	Key string
}

// GenerateEVPNRoutes produces count unique EVPN Type-2 MAC/IP routes
// deterministically from the given seed and peer index.
//
// MAC format: 02:PP:XX:XX:XX:XX where PP encodes peer index.
// The 0x02 prefix sets the locally-administered bit (unicast).
func GenerateEVPNRoutes(seed uint64, peerIndex, count, totalPeers int) []EVPNRoute {
	//nolint:gosec // Deterministic RNG from seed — not for cryptography.
	rng := rand.New(rand.NewSource(int64(seed) ^ int64(peerIndex*0x6C62272E)))

	// Use IPv4 prefixes for the IP portion of Type-2 routes.
	prefixes := GenerateIPv4Routes(seed, peerIndex, count, totalPeers)
	if count > len(prefixes) {
		count = len(prefixes)
	}

	routes := make([]EVPNRoute, count)
	for i := range count {
		// MAC: locally-administered unicast — 02:PP:RR:RR:RR:RR.
		mac := [6]byte{0x02, byte(peerIndex)}              //nolint:gosec // peerIndex max 50.
		binary.BigEndian.PutUint32(mac[2:6], rng.Uint32()) //nolint:gosec // deterministic.

		// RD Type 0: admin=peerIndex, assigned=sequence.
		var rd [8]byte
		binary.BigEndian.PutUint16(rd[2:4], uint16(peerIndex)) //nolint:gosec // peerIndex max 50.
		binary.BigEndian.PutUint32(rd[4:8], uint32(i))         //nolint:gosec // i bounded by count.

		// Label: base per peer.
		label := uint32(200000 + peerIndex*1000 + i%1000) //nolint:gosec // deterministic.

		ip := prefixes[i].Addr()

		routes[i] = EVPNRoute{
			RDBytes:     rd,
			MAC:         mac,
			IP:          ip,
			EthernetTag: 0,
			Labels:      []uint32{label},
			Key:         fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x-%s", mac[0], mac[1], mac[2], mac[3], mac[4], mac[5], ip), //nolint:gosec // mac is [6]byte, indices 0-5 are valid.
		}
	}

	return routes
}
