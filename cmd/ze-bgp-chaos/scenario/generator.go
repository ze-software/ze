package scenario

import (
	"fmt"
	"math/rand"
	"net/netip"
	"slices"
)

// GeneratorParams holds all inputs needed to generate a scenario.
type GeneratorParams struct {
	Seed            uint64
	Peers           int
	IBGPRatio       float64
	LocalAS         uint32
	Routes          int
	HeavyPeers      int
	HeavyRoutes     int
	BasePort        int
	ListenBase      int
	Families        []string // Include filter: only these families (empty = all).
	ExcludeFamilies []string // Exclude filter: remove these families.
}

// Generate creates a deterministic set of PeerProfiles from the given parameters.
// The same parameters always produce the same profiles.
func Generate(params GeneratorParams) ([]PeerProfile, error) {
	if params.Peers < 1 || params.Peers > 50 {
		return nil, fmt.Errorf("peers must be 1-50, got %d", params.Peers)
	}

	//nolint:gosec // Deterministic RNG from seed — not for cryptography.
	rng := rand.New(rand.NewSource(int64(params.Seed)))

	ibgpCount := int(float64(params.Peers) * params.IBGPRatio)

	// Generate unique eBGP ASNs from private range 64512-65534, excluding LocalAS.
	ebgpASNs := generateEBGPASNs(rng, params.Peers-ibgpCount, params.LocalAS)

	profiles := make([]PeerProfile, params.Peers)
	ebgpIdx := 0

	for i := range profiles {
		p := &profiles[i]
		p.Index = i
		p.HoldTime = 90

		// First ibgpCount peers are iBGP, rest are eBGP.
		if i < ibgpCount {
			p.IsIBGP = true
			p.ASN = params.LocalAS
		} else {
			p.IsIBGP = false
			p.ASN = ebgpASNs[ebgpIdx]
			ebgpIdx++
		}

		// Router ID: 10.255.P.Q where P and Q are derived from index.
		// Each peer gets a unique IP in 10.255.0.0/16.
		p.RouterID = netip.AddrFrom4([4]byte{10, 255, byte((i + 1) >> 8), byte((i + 1) & 0xFF)})

		// Connection mode: ~50% active, ~50% passive (deterministic from RNG).
		if rng.Intn(2) == 0 {
			p.Mode = ModeActive
		} else {
			p.Mode = ModePassive
		}

		// Port assignment: each peer gets a unique port starting from ListenBase.
		p.Port = params.ListenBase + i

		// Route count: first HeavyPeers peers (by RNG selection) get heavy routes.
		p.RouteCount = params.Routes
	}

	// Assign heavy route counts to randomly selected peers.
	assignHeavyPeers(rng, profiles, params.HeavyPeers, params.HeavyRoutes)

	// Assign address families to each peer.
	pool := buildFamilyPool(params.Families, params.ExcludeFamilies)
	assignFamilies(rng, profiles, pool)

	return profiles, nil
}

// generateEBGPASNs returns count unique ASNs from the private range 64512-65534,
// excluding localAS.
func generateEBGPASNs(rng *rand.Rand, count int, localAS uint32) []uint32 {
	const (
		privateStart = 64512
		privateEnd   = 65534
	)

	available := make([]uint32, 0, privateEnd-privateStart+1)
	for asn := uint32(privateStart); asn <= privateEnd; asn++ {
		if asn != localAS {
			available = append(available, asn)
		}
	}

	// Shuffle and take first count elements.
	rng.Shuffle(len(available), func(i, j int) {
		available[i], available[j] = available[j], available[i]
	})

	if count > len(available) {
		count = len(available)
	}

	return available[:count]
}

// familyIPv4Unicast is the mandatory family present on all peers.
const familyIPv4Unicast = "ipv4/unicast"

// allFamilies is the full set of supported address families.
var allFamilies = []string{
	"ipv4/unicast",
	"ipv6/unicast",
	"ipv4/vpn",
	"ipv6/vpn",
	"l2vpn/evpn",
	"ipv4/flow",
	"ipv6/flow",
}

// optionalFamilyWeights maps each optional family to its assignment probability.
// ipv4/unicast is mandatory and always assigned.
var optionalFamilyWeights = map[string]float64{
	"ipv6/unicast": 0.7,
	"ipv4/vpn":     0.4,
	"ipv6/vpn":     0.3,
	"l2vpn/evpn":   0.35,
	"ipv4/flow":    0.3,
	"ipv6/flow":    0.2,
}

// buildFamilyPool returns the set of families available for assignment after
// applying include and exclude filters.
func buildFamilyPool(include, exclude []string) []string {
	var base []string
	if len(include) > 0 {
		// Include filter: only these families.
		allowed := make(map[string]bool, len(include))
		for _, f := range include {
			allowed[f] = true
		}
		for _, f := range allFamilies {
			if allowed[f] {
				base = append(base, f)
			}
		}
	} else {
		base = append(base, allFamilies...)
	}

	if len(exclude) > 0 {
		excluded := make(map[string]bool, len(exclude))
		for _, f := range exclude {
			excluded[f] = true
		}
		filtered := base[:0]
		for _, f := range base {
			if !excluded[f] {
				filtered = append(filtered, f)
			}
		}
		base = filtered
	}

	// Ensure ipv4/unicast is always present.
	if !slices.Contains(base, familyIPv4Unicast) {
		base = append([]string{familyIPv4Unicast}, base...)
	}

	return base
}

// assignFamilies assigns a random subset of families to each peer.
// ipv4/unicast is always included. Other families are assigned with
// weighted probability from optionalFamilyWeights.
func assignFamilies(rng *rand.Rand, profiles []PeerProfile, pool []string) {
	for i := range profiles {
		families := []string{familyIPv4Unicast}
		for _, f := range pool {
			if f == familyIPv4Unicast {
				continue // Already included.
			}
			weight, ok := optionalFamilyWeights[f]
			if !ok {
				weight = 0.3 // Default for unknown families.
			}
			if rng.Float64() < weight {
				families = append(families, f)
			}
		}
		profiles[i].Families = families
	}
}

// assignHeavyPeers selects heavyCount peers to receive heavyRoutes instead of
// their default route count.
func assignHeavyPeers(rng *rand.Rand, profiles []PeerProfile, heavyCount, heavyRoutes int) {
	if heavyCount <= 0 || heavyCount > len(profiles) {
		return
	}

	// Build index list and shuffle to pick heavy peers.
	indices := make([]int, len(profiles))
	for i := range indices {
		indices[i] = i
	}

	rng.Shuffle(len(indices), func(i, j int) {
		indices[i], indices[j] = indices[j], indices[i]
	})

	for _, idx := range indices[:heavyCount] {
		profiles[idx].RouteCount = heavyRoutes
	}
}
