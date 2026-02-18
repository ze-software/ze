package scenario

import (
	"fmt"
	"net/netip"
)

// FlowSpecRoute represents a generated FlowSpec rule.
type FlowSpecRoute struct {
	// DestPrefix is the destination prefix match component.
	DestPrefix netip.Prefix

	// SourcePrefix is the source prefix match component.
	SourcePrefix netip.Prefix

	// IsIPv6 indicates whether this is an IPv6 FlowSpec rule.
	IsIPv6 bool

	// Key is a unique string identifier for validation tracking.
	Key string
}

// GenerateFlowSpecRoutes produces count unique FlowSpec rules deterministically
// from the given seed and peer index. Each rule matches on destination and
// source prefixes derived from the peer's address space.
func GenerateFlowSpecRoutes(seed uint64, peerIndex, count int, ipv6 bool) []FlowSpecRoute {
	// Generate prefix pairs: dest from this peer, source from this peer.
	// We need 2× prefixes: one for dest, one for source.
	var prefixes []netip.Prefix
	if ipv6 {
		prefixes = GenerateIPv6Routes(seed, peerIndex, count*2)
	} else {
		prefixes = GenerateIPv4Routes(seed, peerIndex, count*2)
	}

	available := len(prefixes) / 2
	if count > available {
		count = available
	}

	routes := make([]FlowSpecRoute, count)
	for i := range count {
		dest := prefixes[i*2]
		src := prefixes[i*2+1]

		routes[i] = FlowSpecRoute{
			DestPrefix:   dest,
			SourcePrefix: src,
			IsIPv6:       ipv6,
			Key:          fmt.Sprintf("flow:%s->%s", dest, src),
		}
	}

	return routes
}
