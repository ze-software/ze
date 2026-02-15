package scenario

import (
	"fmt"
	"math/rand"
	"net/netip"
)

// GeneratorParams holds all inputs needed to generate a scenario.
type GeneratorParams struct {
	Seed        uint64
	Peers       int
	IBGPRatio   float64
	LocalAS     uint32
	Routes      int
	HeavyPeers  int
	HeavyRoutes int
	BasePort    int
	ListenBase  int
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
