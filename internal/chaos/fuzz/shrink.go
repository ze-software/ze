// Design: docs/architecture/chaos-web-dashboard.md -- automatic minimization of failing swarm profiles and event logs

package fuzz

import (
	"context"
	"slices"

	"github.com/ze-software/ze/internal/chaos/engine"
	"github.com/ze-software/ze/internal/chaos/inprocess"
	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/chaos/scenario"
	"github.com/ze-software/ze/internal/chaos/shrink"
	"github.com/ze-software/ze/internal/chaos/validation"
)

// ShrinkResult records the minimal failing profile and event-log reduction
// discovered for a failing swarm seed.
type ShrinkResult struct {
	Profile        Profile
	Repro          string
	Steps          int
	OriginalEvents int
	ShrunkEvents   int
	ShrunkProperty string
}

// MinimizeProfile reduces a failing swarm Profile toward the smallest fault
// subset, family list, peer count, and route count that still satisfies fails.
// Returns the minimized Profile and the number of candidate probes executed.
func MinimizeProfile(ctx context.Context, prof Profile, fails func(context.Context, Profile) bool) (Profile, int) {
	cur := cloneProfile(prof)
	steps := 0

	tryCandidate := func(cand Profile) bool {
		if ctx.Err() != nil {
			return false
		}
		steps++
		if fails(ctx, cand) {
			cur = cand
			return true
		}
		return false
	}

	// 1. Strip stream fault mechanisms one by one.
	if cur.StreamFaults.ResetRate > 0 {
		cand := cloneProfile(cur)
		cand.StreamFaults.ResetRate = 0
		tryCandidate(cand)
	}
	if cur.StreamFaults.BlackholeRate > 0 {
		cand := cloneProfile(cur)
		cand.StreamFaults.BlackholeRate = 0
		tryCandidate(cand)
	}
	if cur.StreamFaults.FragmentRate > 0 {
		cand := cloneProfile(cur)
		cand.StreamFaults.FragmentRate = 0
		tryCandidate(cand)
	}

	// 2. Strip optional Graceful Restart negotiation.
	if cur.GracefulRestart {
		cand := cloneProfile(cur)
		cand.GracefulRestart = false
		tryCandidate(cand)
	}

	// 3. Remove individual chaos actions, then try disabling chaos/route rates.
	for _, act := range slices.Clone(cur.EnabledActions) {
		cand := cloneProfile(cur)
		cand.EnabledActions = slices.DeleteFunc(cand.EnabledActions, func(a engine.ActionType) bool {
			return a == act
		})
		tryCandidate(cand)
	}
	if cur.ChaosRate > 0 {
		cand := cloneProfile(cur)
		cand.ChaosRate = 0
		cand.EnabledActions = nil
		tryCandidate(cand)
	}
	if cur.RouteRate > 0 {
		cand := cloneProfile(cur)
		cand.RouteRate = 0
		tryCandidate(cand)
	}

	// 4. Shrink multi-family runs down to a single family if possible.
	if len(cur.Families) > 1 {
		for _, fam := range cur.Families {
			cand := cloneProfile(cur)
			cand.Families = []string{fam}
			if tryCandidate(cand) {
				break
			}
		}
	}

	// 5. Binary-search Peers down toward 2.
	cur.Peers, steps = shrinkInt(ctx, cur.Peers, 2, steps, func(n int) bool {
		cand := cloneProfile(cur)
		cand.Peers = n
		return fails(ctx, cand)
	})

	// 6. Binary-search Routes down toward 1.
	cur.Routes, steps = shrinkInt(ctx, cur.Routes, 1, steps, func(n int) bool {
		cand := cloneProfile(cur)
		cand.Routes = n
		return fails(ctx, cand)
	})

	return cur, steps
}

func shrinkInt(ctx context.Context, hi, lo, steps int, fails func(int) bool) (int, int) {
	if hi <= lo {
		return hi, steps
	}
	// Quick probe at the minimum bound first.
	if ctx.Err() == nil {
		steps++
		if fails(lo) {
			return lo, steps
		}
	}
	best := hi
	low := lo + 1
	for low < best && ctx.Err() == nil {
		mid := low + (best-low)/2
		steps++
		if fails(mid) {
			best = mid
		} else {
			low = mid + 1
		}
	}
	return best, steps
}

func cloneProfile(p Profile) Profile {
	out := p
	out.Families = slices.Clone(p.Families)
	out.EnabledActions = slices.Clone(p.EnabledActions)
	return out
}

// ShrinkFailure minimizes a failing swarm profile against the live in-process
// reactor and also runs event-log shrinking on the pre-cutoff event trace.
func ShrinkFailure(ctx context.Context, prof Profile, res *inprocess.RunResult, cfg Config) *ShrinkResult {
	minProf, steps := MinimizeProfile(ctx, prof, func(c context.Context, cand Profile) bool {
		_, violations, err := executeProfile(c, cand, cfg)
		if err != nil {
			return true
		}
		return len(violations) > 0
	})

	sr := &ShrinkResult{
		Profile: minProf,
		Repro:   minProf.ReproCommand(cfg.Duration),
		Steps:   steps,
	}

	if res == nil {
		return sr
	}
	if len(res.Events) == 0 {
		return sr
	}

	var preCutoff []peer.Event
	for i := range res.Events {
		ev := res.Events[i]
		if ev.PlannedShutdown {
			continue
		}
		if !ev.Time.Before(res.ObservationEnd) {
			continue
		}
		preCutoff = append(preCutoff, ev)
	}
	sr.OriginalEvents = len(preCutoff)
	if len(preCutoff) == 0 {
		return sr
	}

	profiles, err := scenario.Generate(scenario.GeneratorParams{
		Seed:      prof.Seed,
		Peers:     prof.Peers,
		IBGPRatio: prof.IBGPRatio,
		LocalAS:   65000,
		Routes:    prof.Routes,
		BasePort:  1850,
		Families:  prof.Families,
	})
	if err != nil {
		return sr
	}

	ibgp := make([]bool, len(profiles))
	peerFams := make([][]string, len(profiles))
	for i := range profiles {
		ibgp[i] = profiles[i].IsIBGP
		peerFams[i] = profiles[i].Families
	}
	reachable := validation.ScenarioReachability(ibgp, peerFams)

	shrunk, err := shrink.Run(preCutoff, shrink.Config{
		PeerCount:    len(profiles),
		Deadline:     cfg.ConvergenceDeadline,
		Reachability: reachable,
		Properties:   "all",
	})
	if err == nil && shrunk != nil {
		sr.ShrunkEvents = len(shrunk.Events)
		sr.ShrunkProperty = shrunk.Property
	}
	return sr
}

// hasAction reports whether actions contains target.
func hasAction(actions []engine.ActionType, target engine.ActionType) bool {
	return slices.Contains(actions, target)
}
