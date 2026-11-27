package fuzz

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSwarmProfileDeterministicAndDiverse verifies that SwarmProfile produces
// identical profiles for the same seed and explores diverse fault/stream
// combinations across a seed range.
//
// VALIDATES: VOPR-style swarm testing diversity and seed reproducibility.
// PREVENTS: Identical fault weights on every seed or non-deterministic profiles.
func TestSwarmProfileDeterministicAndDiverse(t *testing.T) {
	p1 := SwarmProfile(42, 6, 20)
	p2 := SwarmProfile(42, 6, 20)
	assert.Equal(t, p1, p2, "same seed must yield identical swarm profile")

	var sawFrag, sawBlackhole, sawReset, sawChaos, sawRoute bool
	peerCounts := make(map[int]bool)
	for seed := uint64(1); seed <= 40; seed++ {
		p := SwarmProfile(seed, 6, 20)
		assert.GreaterOrEqual(t, p.Peers, 2)
		assert.LessOrEqual(t, p.Peers, 6)
		assert.GreaterOrEqual(t, p.Routes, 4)
		assert.LessOrEqual(t, p.Routes, 20)
		peerCounts[p.Peers] = true
		if p.StreamFaults.FragmentRate > 0 {
			sawFrag = true
		}
		if p.StreamFaults.BlackholeRate > 0 {
			sawBlackhole = true
		}
		if p.StreamFaults.ResetRate > 0 {
			sawReset = true
		}
		if p.ChaosRate > 0 {
			sawChaos = true
		}
		if p.RouteRate > 0 {
			sawRoute = true
		}
	}
	assert.GreaterOrEqual(t, len(peerCounts), 3, "swarm should vary peer counts")
	assert.True(t, sawFrag, "swarm should exercise wire fragmentation")
	assert.True(t, sawBlackhole, "swarm should exercise asymmetric blackholes")
	assert.True(t, sawReset, "swarm should exercise stream resets")
	assert.True(t, sawChaos, "swarm should exercise BGP chaos actions")
	assert.True(t, sawRoute, "swarm should exercise route dynamics")

	repro := p1.ReproCommand(15 * time.Second)
	assert.Contains(t, repro, "./le chaos run --in-process --seed 42")
	assert.Contains(t, repro, "--properties all")
}

// TestSwarmRunExecutesMultipleSeeds verifies that fuzz.Run drives multiple
// real in-process Ze reactor seeds with online StateChecker and BGP properties.
//
// VALIDATES: End-to-end multi-seed swarm fuzzing against the live in-process reactor.
// PREVENTS: Swarm runner failing to wire profiles, stream faults, or invariants.
func TestSwarmRunExecutesMultipleSeeds(t *testing.T) {
	var out bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	summary, err := Run(ctx, Config{
		Seeds:               2,
		BaseSeed:            100,
		Duration:            6 * time.Second,
		MaxPeers:            3,
		MaxRoutes:           6,
		StepDelay:           25 * time.Millisecond,
		ConvergenceDeadline: 10 * time.Second,
		Out:                 &out,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, summary.TotalSeeds)
	assert.Equal(t, 2, summary.Passed, "failures: %+v\noutput:\n%s", summary.Failures, out.String())
	assert.Equal(t, 0, summary.Failed)
	assert.Positive(t, summary.TotalEvents)
	assert.Positive(t, summary.TotalFSMTransitions)
}

// TestCLIRunFlagValidation verifies invalid fuzz CLI flags are rejected cleanly.
//
// VALIDATES: Boundary checks on --seeds, --duration, --max-peers, --max-routes.
// PREVENTS: Zero/negative seed counts or unbounded peer counts entering fuzz.Run.
func TestCLIRunFlagValidation(t *testing.T) {
	assert.Equal(t, 0, CLIRun([]string{"--help"}))
	assert.Equal(t, 1, CLIRun([]string{"--seeds", "0"}))
	assert.Equal(t, 1, CLIRun([]string{"--duration", "0s"}))
	assert.Equal(t, 1, CLIRun([]string{"--max-peers", "1"}))
	assert.Equal(t, 1, CLIRun([]string{"--max-peers", "17"}))
	assert.Equal(t, 1, CLIRun([]string{"--max-routes", "0"}))
	assert.Equal(t, 1, CLIRun([]string{"--max-routes", "501"}))
	assert.Equal(t, 1, CLIRun([]string{"--convergence-deadline", "0s"}))
}
