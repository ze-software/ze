// Design: docs/architecture/chaos-web-dashboard.md -- TigerBeetle VOPR-style swarm fuzzing profiles

package fuzz

import (
	"math/rand/v2"
	"time"

	"github.com/ze-software/ze/internal/chaos/engine"
	"github.com/ze-software/ze/internal/chaos/mocknet"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	famIPv4Unicast   = "ipv4/unicast"
	famIPv6Unicast   = "ipv6/unicast"
	famIPv4Multicast = "ipv4/multicast"
	famIPv6Multicast = "ipv6/multicast"
)

// swarmActions lists BGP chaos actions effective on in-process mocknet
// connections (includes RFC 7606 malformed-update handling and v2 route/timer
// disruptions; excludes Linux netns iface actions and kernel TCP socket-buffer
// actions like slow-peer/zero-window which require *net.TCPConn, since
// wire-level slowness and stalls are injected via mocknet.FaultConn instead).
var swarmActions = []engine.ActionType{
	engine.ActionMalformedUpdate,
	engine.ActionClockDrift,
	engine.ActionRouteBurst,
	engine.ActionWithdrawalBurst,
	engine.ActionRouteFlap,
}

// Profile describes one deterministic swarm-testing configuration derived from
// a seed. Following TigerBeetle VOPR / Groce et al. swarm testing, each seed
// activates a randomized subset of fault mechanisms and rates so different
// seeds stress distinct extreme combinations rather than averaging them out.
type Profile struct {
	Seed            uint64
	Peers           int
	IBGPRatio       float64
	Routes          int
	Families        []string
	GracefulRestart bool
	ChaosRate       float64
	RouteRate       float64
	EnabledActions  []engine.ActionType
	StreamFaults    mocknet.StreamFaultConfig
}

// SwarmProfile deterministically derives a swarm fuzzing Profile from seed,
// bounded by maxPeers and maxRoutes.
func SwarmProfile(seed uint64, maxPeers, maxRoutes int) Profile {
	if maxPeers < 2 {
		maxPeers = 2
	}
	if maxRoutes < 1 {
		maxRoutes = 1
	}

	rng := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)) //nolint:gosec // Deterministic simulation PRNG.

	peers := 2
	if maxPeers > 2 {
		peers = 2 + rng.IntN(maxPeers-1)
	}

	ibgpChoices := []float64{0.0, 0.25, 0.5}
	ibgpRatio := ibgpChoices[rng.IntN(len(ibgpChoices))]

	minRoutes := min(4, maxRoutes)
	routes := minRoutes
	if maxRoutes > minRoutes {
		routes = minRoutes + rng.IntN(maxRoutes-minRoutes+1)
	}

	familyChoices := [][]string{
		{famIPv4Unicast},
		{famIPv6Unicast},
		{famIPv4Unicast, famIPv6Unicast},
		{famIPv4Unicast, famIPv4Multicast},
		{famIPv4Unicast, famIPv6Unicast, famIPv6Multicast},
	}
	families := familyChoices[rng.IntN(len(familyChoices))]

	gracefulRestart := rng.IntN(2) == 1

	chaosRates := []float64{0.0, 0.05, 0.15, 0.30}
	chaosRate := chaosRates[rng.IntN(len(chaosRates))]

	routeRates := []float64{0.0, 0.05, 0.15}
	routeRate := routeRates[rng.IntN(len(routeRates))]

	var enabled []engine.ActionType
	for _, act := range swarmActions {
		if rng.IntN(2) == 1 {
			enabled = append(enabled, act)
		}
	}

	fragRates := []float64{0.0, 0.25, 0.60}
	blackholeRates := []float64{0.0, 0.0, 0.02}
	resetRates := []float64{0.0, 0.0, 0.002}

	fragRate := fragRates[rng.IntN(len(fragRates))]
	blackholeRate := blackholeRates[rng.IntN(len(blackholeRates))]
	resetRate := resetRates[rng.IntN(len(resetRates))]

	// Ensure every seed stresses at least one disturbance source.
	if chaosRate == 0 && routeRate == 0 && fragRate == 0 && blackholeRate == 0 && resetRate == 0 {
		fragRate = 0.40
	}

	return Profile{
		Seed:            seed,
		Peers:           peers,
		IBGPRatio:       ibgpRatio,
		Routes:          routes,
		Families:        families,
		GracefulRestart: gracefulRestart,
		ChaosRate:       chaosRate,
		RouteRate:       routeRate,
		EnabledActions:  enabled,
		StreamFaults: mocknet.StreamFaultConfig{
			Seed:              seed,
			FragmentRate:      fragRate,
			MaxFragmentBytes:  mocknet.DefaultMaxFragmentBytes,
			BlackholeRate:     blackholeRate,
			BlackholeDuration: 2 * time.Second,
			ResetRate:         resetRate,
		},
	}
}

// ReproCommand returns a copy-paste `./le chaos run --in-process ...` command
// that reproduces this profile's exact parameters.
func (p Profile) ReproCommand(duration time.Duration) string {
	var tb textbuf.Buffer
	tb.Str("./le chaos run --in-process --seed ").Uint(p.Seed).
		Str(" --peers ").Int(int64(p.Peers)).
		Str(" --ibgp-ratio ").Float2(p.IBGPRatio).
		Str(" --routes ").Int(int64(p.Routes)).
		Str(" --families ").Join(p.Families, ",").
		Str(" --chaos-rate ").Float2(p.ChaosRate).
		Str(" --route-rate ").Float2(p.RouteRate).
		Str(" --duration ").Str(duration.String()).
		Str(" --properties all")
	if len(p.EnabledActions) > 0 {
		names := make([]string, len(p.EnabledActions))
		for i, a := range p.EnabledActions {
			names[i] = a.String()
		}
		tb.Str(" --chaos-actions ").Join(names, ",")
	}
	if p.StreamFaults.FragmentRate > 0 {
		tb.Str(" --stream-fragment-rate ").Float2(p.StreamFaults.FragmentRate)
	}
	if p.StreamFaults.BlackholeRate > 0 {
		tb.Str(" --stream-blackhole-rate ").Float2(p.StreamFaults.BlackholeRate)
	}
	if p.StreamFaults.ResetRate > 0 {
		tb.Str(" --stream-reset-rate ").Float(p.StreamFaults.ResetRate, 3)
	}
	return tb.String()
}
