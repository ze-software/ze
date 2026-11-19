// Design: docs/architecture/chaos-web-dashboard.md — simulated network topology
// Overview: topology.go — the topology model a generated network is computed by
// Related: generator.go — generateEBGPASNs, the neighbours' AS numbers

package scenario

import (
	"fmt"
	"math/rand"
	"net/netip"
	"slices"
)

// TopologyParams holds the inputs of GenerateTopology. The seed is the only
// source of variation: equal parameters give an equal network.
type TopologyParams struct {
	Seed    uint64
	LocalAS uint32
	// Routers is the number of routers of the AS, 3 to 32.
	Routers int
	// Borders is the number of routers with external neighbours, 2 to Routers-1.
	// The reflector is never one of them.
	Borders int
	// ReflectorClients is the number of routers that are clients of the one
	// route reflector, 0 to Routers-1. With none, no trace crosses an
	// intermediate router and GenerateTopology refuses the network.
	ReflectorClients int
	// Neighbours is the number of external neighbours, Borders to 128. Every
	// border router gets at least one.
	Neighbours int
	// Prefixes is the number of prefixes announced from outside, 2 to 256. The
	// first has two exits on two border routers and the second has one. One
	// more prefix always originates inside the AS.
	Prefixes int
	// RouterFirst is the loopback of the first router. The others follow it.
	RouterFirst netip.Addr
	// NeighbourFirst is the address of the first external neighbour. The
	// others follow it.
	NeighbourFirst netip.Addr
}

// GenerateTopology builds a deterministic network from params and computes it.
// It refuses a network in which no trace crosses an intermediate router,
// because traces of one hop alone would not prove a walk through the core.
func GenerateTopology(params TopologyParams) (*Topology, error) {
	spec, err := generateTopologySpec(params)
	if err != nil {
		return nil, err
	}
	topology, err := NewTopology(spec)
	if err != nil {
		return nil, err
	}
	if !topology.crossesIntermediateRouter() {
		return nil, fmt.Errorf(
			"topology: no router's trace crosses an intermediate router (%d reflector clients), so the network cannot prove a walk through the core: set ReflectorClients to 1 or more",
			params.ReflectorClients)
	}
	return topology, nil
}

func (p TopologyParams) validate() error {
	if p.Routers < 3 {
		return fmt.Errorf("topology: %d routers, and a generated network needs at least 3: raise Routers", p.Routers)
	}
	if p.Routers > topologyRoutersMax {
		return fmt.Errorf("topology: %d routers, and the model holds at most %d: lower Routers", p.Routers, topologyRoutersMax)
	}
	if p.Borders < 2 {
		return fmt.Errorf("topology: %d border routers, and a prefix with two exits needs 2: raise Borders", p.Borders)
	}
	if p.Borders > p.Routers-1 {
		return fmt.Errorf("topology: %d border routers of %d routers, and the reflector is never a border router: lower Borders to %d or less",
			p.Borders, p.Routers, p.Routers-1)
	}
	if p.ReflectorClients < 0 {
		return fmt.Errorf("topology: %d reflector clients: set ReflectorClients to 0 or more", p.ReflectorClients)
	}
	if p.ReflectorClients > p.Routers-1 {
		return fmt.Errorf("topology: %d reflector clients of %d routers, and the reflector is not its own client: lower ReflectorClients to %d or less",
			p.ReflectorClients, p.Routers, p.Routers-1)
	}
	if p.Neighbours < p.Borders {
		return fmt.Errorf("topology: %d external neighbours for %d border routers, and each border router needs one: raise Neighbours",
			p.Neighbours, p.Borders)
	}
	if p.Neighbours > topologyNeighboursMax {
		return fmt.Errorf("topology: %d external neighbours, and the model holds at most %d: lower Neighbours",
			p.Neighbours, topologyNeighboursMax)
	}
	if p.Prefixes < 2 {
		return fmt.Errorf("topology: %d prefixes, and a generated network needs one with two exits and one with one: raise Prefixes to 2 or more",
			p.Prefixes)
	}
	if p.Prefixes > topologyRoutesMax {
		return fmt.Errorf("topology: %d prefixes, and the generator holds at most %d: lower Prefixes", p.Prefixes, topologyRoutesMax)
	}
	return nil
}

// topologyInsidePrefix is the prefix a generated network originates inside the
// AS. The prefixes announced from outside are the /24s of 198.18.0.0/16.
//
// Both come from 198.18.0.0/15, the range RFC 2544 reserves for benchmarking,
// and not from the documentation ranges. IPv4 has three documentation blocks,
// each a /24 (RFC 5737: 192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24), and a
// caller gives the speakers their addresses from those. A generated network
// announces up to topologyRoutesMax /24s, which three /24s cannot hold, and
// carving them into longer prefixes would show a looking glass routes no
// operator accepts from a neighbour. The benchmarking range is routed nowhere
// on the Internet, so no generated prefix is a real network.
var topologyInsidePrefix = netip.PrefixFrom(netip.AddrFrom4([4]byte{198, 19, 0, 0}), 24)

func generateTopologySpec(params TopologyParams) (TopologySpec, error) {
	if err := params.validate(); err != nil {
		return TopologySpec{}, err
	}
	loopbacks, err := consecutiveAddresses(params.RouterFirst, params.Routers)
	if err != nil {
		return TopologySpec{}, fmt.Errorf("topology: RouterFirst: %w", err)
	}
	links, err := consecutiveAddresses(params.NeighbourFirst, params.Neighbours)
	if err != nil {
		return TopologySpec{}, fmt.Errorf("topology: NeighbourFirst: %w", err)
	}

	//nolint:gosec // Deterministic RNG from seed — not for cryptography.
	rng := rand.New(rand.NewSource(int64(params.Seed)))

	// Roles: the first router of a seeded order is the reflector, the next are
	// its clients, and the border routers are drawn from all but the reflector.
	order := rng.Perm(params.Routers)
	reflector := order[0]
	others := order[1:]
	routers := make([]TopologyRouter, params.Routers)
	for index := range routers {
		routers[index].Loopback = loopbacks[index]
	}
	for _, client := range others[:params.ReflectorClients] {
		routers[reflector].Clients = append(routers[reflector].Clients, loopbacks[client])
	}
	borders := make([]netip.Addr, 0, params.Borders)
	for _, pick := range rng.Perm(len(others))[:params.Borders] {
		borders = append(borders, loopbacks[others[pick]])
	}
	originator := rng.Intn(params.Routers)
	routers[originator].Originates = []netip.Prefix{topologyInsidePrefix}

	neighbourAS := generateEBGPASNs(rng, params.Neighbours, params.LocalAS)
	if len(neighbourAS) < params.Neighbours {
		return TopologySpec{}, fmt.Errorf("topology: the private AS range holds %d numbers for %d external neighbours: lower Neighbours",
			len(neighbourAS), params.Neighbours)
	}
	neighbours := make([]TopologyNeighbour, params.Neighbours)
	for index := range neighbours {
		// The first neighbours take one border router each, so none is left bare.
		border := index
		if index >= params.Borders {
			border = rng.Intn(params.Borders)
		}
		neighbours[index] = TopologyNeighbour{
			Address:       links[index],
			AS:            neighbourAS[index],
			Border:        borders[border],
			BorderNextHop: NextHopPolicySelf,
		}
	}

	remote := remoteASNumbers(params.LocalAS)
	for index := range params.Prefixes {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{198, 18, byte(index), 0}), 24)
		rng.Shuffle(len(remote), func(i, j int) { remote[i], remote[j] = remote[j], remote[i] })
		first := rng.Intn(params.Neighbours)

		// The first prefix has two exits, the second one, the rest either.
		exits := 1
		if index == 0 {
			exits = 2
		}
		if index > 1 {
			exits = 1 + rng.Intn(2)
		}
		if exits == 1 {
			// Zero, one or two AS numbers beyond the neighbour.
			neighbours[first].Announces = append(neighbours[first].Announces,
				NeighbourRoute{Prefix: prefix, ASPathBeyond: slices.Clone(remote[:rng.Intn(3)])})
			continue
		}
		// Two exits on two border routers, toward one origin AS, each through
		// its own transit AS or straight to the origin.
		second := neighbourOnOtherBorder(rng, neighbours, first)
		origin := remote[0]
		neighbours[first].Announces = append(neighbours[first].Announces,
			NeighbourRoute{Prefix: prefix, ASPathBeyond: pathToOrigin(rng, remote[1], origin)})
		neighbours[second].Announces = append(neighbours[second].Announces,
			NeighbourRoute{Prefix: prefix, ASPathBeyond: pathToOrigin(rng, remote[2], origin)})
	}
	return TopologySpec{LocalAS: params.LocalAS, Routers: routers, Neighbours: neighbours}, nil
}

// consecutiveAddresses returns count IPv4 addresses starting at first.
func consecutiveAddresses(first netip.Addr, count int) ([]netip.Addr, error) {
	addresses := make([]netip.Addr, 0, count)
	address := first
	for range count {
		// Next past the last address gives the zero Addr, which is no IPv4 one.
		if !address.Is4() {
			return nil, fmt.Errorf("%d consecutive IPv4 addresses do not fit from %q: set it to an IPv4 address with room after it",
				count, first)
		}
		addresses = append(addresses, address)
		address = address.Next()
	}
	return addresses, nil
}

// remoteASNumbers returns the AS numbers a generated path holds beyond a
// neighbour: the documentation range 64496 to 64511, without the local AS.
// The neighbours take theirs from the private range, so the two never collide.
func remoteASNumbers(localAS uint32) []uint32 {
	remote := make([]uint32, 0, 16)
	for as := uint32(64496); as <= 64511; as++ {
		if as != localAS {
			remote = append(remote, as)
		}
	}
	return remote
}

// neighbourOnOtherBorder picks a neighbour attached to another border router
// than neighbours[first]. One always exists: there are two border routers or
// more, and each holds a neighbour.
func neighbourOnOtherBorder(rng *rand.Rand, neighbours []TopologyNeighbour, first int) int {
	var others []int
	for index := range neighbours {
		if neighbours[index].Border != neighbours[first].Border {
			others = append(others, index)
		}
	}
	return others[rng.Intn(len(others))]
}

// pathToOrigin returns the AS numbers beyond a neighbour for a prefix of
// origin: the origin alone, or one transit AS then the origin.
func pathToOrigin(rng *rand.Rand, transit, origin uint32) []uint32 {
	if rng.Intn(2) == 0 {
		return []uint32{origin}
	}
	return []uint32{transit, origin}
}
