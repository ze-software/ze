// Design: docs/architecture/chaos-web-dashboard.md — route action scheduling

package route

import (
	"math/rand"
	"time"
)

// SchedulerConfig holds the parameters for the route dynamics scheduler.
type SchedulerConfig struct {
	// Seed for deterministic PRNG (offset from main seed to decorrelate from chaos).
	Seed uint64

	// PeerCount is the total number of peers.
	PeerCount int

	// Rate is the per-peer probability of a route action per interval (0.0-1.0).
	Rate float64

	// Interval is the time between route action checks.
	Interval time.Duration

	// Warmup is the duration before route dynamics begin.
	Warmup time.Duration

	// BaseRoutes is the base route count per peer (for computing churn counts).
	BaseRoutes int
}

// ScheduledAction pairs a route action with its target peer.
type ScheduledAction struct {
	// PeerIndex is the target peer for this action.
	PeerIndex int

	// Action is the route action to execute.
	Action Action
}

// actionWeight pairs an action type with its relative selection weight.
type actionWeight struct {
	action ActionType
	weight int
}

// defaultWeights defines the weighted distribution of route dynamics events.
// Churn (normal background noise) dominates; full-withdraw is rare.
var defaultWeights = []actionWeight{
	{ActionChurn, 60},
	{ActionPartialWithdraw, 30},
	{ActionFullWithdraw, 10},
}

// totalWeight is the sum of all action weights.
var totalWeight int

func init() {
	for _, w := range defaultWeights {
		totalWeight += w.weight
	}
}

// Scheduler generates deterministic route dynamics events based on a seed.
type Scheduler struct {
	rng       *rand.Rand
	cfg       SchedulerConfig
	nextTick  time.Time
	startTime time.Time
	started   bool
}

// NewScheduler creates a new route dynamics scheduler with the given config.
func NewScheduler(cfg SchedulerConfig) *Scheduler {
	//nolint:gosec // Deterministic PRNG is intentional — reproducibility from seed.
	rng := rand.New(rand.NewSource(int64(cfg.Seed)))
	return &Scheduler{
		rng: rng,
		cfg: cfg,
	}
}

// Tick checks whether route dynamics events should fire at the given time.
// It rolls independently for each established peer, so the rate is
// a per-peer probability. Returns zero or more scheduled actions.
func (s *Scheduler) Tick(now time.Time, established []bool) []ScheduledAction {
	if s.cfg.Rate <= 0 {
		return nil
	}

	if !s.started {
		s.started = true
		s.startTime = now
		s.nextTick = now
	}

	if now.Before(s.nextTick) {
		return nil
	}

	s.nextTick = now.Add(s.cfg.Interval)

	if s.cfg.Warmup > 0 && now.Sub(s.startTime) < s.cfg.Warmup {
		return nil
	}

	candidates := s.establishedPeers(established)
	if len(candidates) == 0 {
		return nil
	}

	var actions []ScheduledAction
	for _, peerIdx := range candidates {
		if s.cfg.Rate < 1.0 && s.rng.Float64() >= s.cfg.Rate {
			continue
		}
		action := s.selectAction()
		actions = append(actions, ScheduledAction{
			PeerIndex: peerIdx,
			Action:    action,
		})
	}
	return actions
}

// selectAction picks a route action type using weighted random selection.
func (s *Scheduler) selectAction() Action {
	roll := s.rng.Intn(totalWeight)
	cumulative := 0
	for _, w := range defaultWeights {
		cumulative += w.weight
		if roll < cumulative {
			action := Action{Type: w.action}
			switch w.action {
			case ActionPartialWithdraw:
				// Random fraction between 0.1 and 0.5.
				action.WithdrawFraction = 0.1 + s.rng.Float64()*0.4
			case ActionFullWithdraw:
				// No extra parameters needed — withdraws all routes.
			case ActionChurn:
				// Churn 1-5% of base routes.
				if s.cfg.BaseRoutes > 0 {
					minChurn := max(1, s.cfg.BaseRoutes/100)
					maxChurn := max(2, s.cfg.BaseRoutes*5/100)
					action.ChurnCount = minChurn + s.rng.Intn(maxChurn-minChurn+1)
				} else {
					action.ChurnCount = 1
				}
			}
			return action
		}
	}

	return Action{Type: ActionChurn, ChurnCount: 1}
}

// SetRate updates the route dynamics rate. Safe to call from another goroutine
// only if the caller ensures no concurrent Tick calls (scheduler loop
// processes control commands and ticks sequentially).
func (s *Scheduler) SetRate(rate float64) {
	s.cfg.Rate = rate
}

// establishedPeers returns indices of peers that are currently established.
func (s *Scheduler) establishedPeers(established []bool) []int {
	peers := make([]int, 0, len(established))
	for i, est := range established {
		if est {
			peers = append(peers, i)
		}
	}
	return peers
}
