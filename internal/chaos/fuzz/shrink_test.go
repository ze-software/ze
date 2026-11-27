package fuzz

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/chaos/engine"
	"github.com/ze-software/ze/internal/chaos/mocknet"
)

// TestMinimizeProfileReducesAllIrrelevantAxes verifies MinimizeProfile strips
// unrelated stream faults, actions, Graceful Restart, families, peers, and
// routes while keeping the exact trigger intact.
//
// VALIDATES: Automatic shrinking isolates the minimal reproducing swarm profile.
// PREVENTS: Noisy multi-fault repro commands when a single fault triggers the bug.
func TestMinimizeProfileReducesAllIrrelevantAxes(t *testing.T) {
	initial := Profile{
		Seed:            99,
		Peers:           8,
		IBGPRatio:       0.5,
		Routes:          40,
		Families:        []string{"ipv4/unicast", "ipv6/unicast", "ipv4/multicast"},
		GracefulRestart: true,
		ChaosRate:       0.3,
		RouteRate:       0.15,
		EnabledActions: []engine.ActionType{
			engine.ActionMalformedUpdate,
			engine.ActionClockDrift,
			engine.ActionRouteFlap,
		},
		StreamFaults: mocknet.StreamFaultConfig{
			Seed:          99,
			FragmentRate:  0.6,
			BlackholeRate: 0.02,
			ResetRate:     0.002,
		},
	}

	// Synthetic bug: requires FragmentRate > 0, ActionRouteFlap, ipv6/unicast,
	// at least 3 peers, and at least 5 routes.
	fails := func(_ context.Context, p Profile) bool {
		return p.StreamFaults.FragmentRate > 0 &&
			hasAction(p.EnabledActions, engine.ActionRouteFlap) &&
			slices.Contains(p.Families, "ipv6/unicast") &&
			p.Peers >= 3 &&
			p.Routes >= 5
	}

	minProf, steps := MinimizeProfile(context.Background(), initial, fails)

	assert.Positive(t, steps)
	assert.Zero(t, minProf.StreamFaults.ResetRate)
	assert.Zero(t, minProf.StreamFaults.BlackholeRate)
	assert.Equal(t, 0.6, minProf.StreamFaults.FragmentRate)
	assert.False(t, minProf.GracefulRestart)
	assert.Equal(t, []engine.ActionType{engine.ActionRouteFlap}, minProf.EnabledActions)
	assert.Zero(t, minProf.RouteRate)
	assert.Equal(t, []string{"ipv6/unicast"}, minProf.Families)
	assert.Equal(t, 3, minProf.Peers)
	assert.Equal(t, 5, minProf.Routes)
}
