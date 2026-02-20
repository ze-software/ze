// Design: docs/architecture/chaos-web-dashboard.md — chaos action scheduling

package chaos

import (
	"math/rand"
	"time"
)

// SchedulerConfig holds the parameters for the chaos scheduler.
type SchedulerConfig struct {
	// Seed for deterministic PRNG.
	Seed uint64

	// PeerCount is the total number of peers.
	PeerCount int

	// Rate is the probability of firing a chaos event per interval (0.0-1.0).
	Rate float64

	// Interval is the time between chaos checks.
	Interval time.Duration

	// Warmup is the duration before chaos events begin.
	Warmup time.Duration
}

// ScheduledAction pairs a chaos action with its target peer.
type ScheduledAction struct {
	// PeerIndex is the target peer for this action.
	PeerIndex int

	// Action is the chaos action to execute.
	Action ChaosAction
}

// actionWeight pairs an action type with its relative selection weight.
type actionWeight struct {
	action ActionType
	weight int
}

// defaultWeights defines the weighted distribution of chaos events.
// Weights are from the master design document.
var defaultWeights = []actionWeight{
	{ActionTCPDisconnect, 25},
	{ActionNotificationCease, 15},
	{ActionHoldTimerExpiry, 15},
	{ActionDisconnectDuringBurst, 10},
	{ActionReconnectStorm, 10},
	{ActionConnectionCollision, 10},
	{ActionMalformedUpdate, 10},
	{ActionConfigReload, 5},
}

// totalWeight is the sum of all action weights.
var totalWeight int

func init() {
	for _, w := range defaultWeights {
		totalWeight += w.weight
	}
}

// Scheduler generates deterministic chaos events based on a seed.
type Scheduler struct {
	rng       *rand.Rand
	cfg       SchedulerConfig
	nextTick  time.Time
	startTime time.Time
	started   bool
}

// NewScheduler creates a new chaos scheduler with the given config.
func NewScheduler(cfg SchedulerConfig) *Scheduler {
	//nolint:gosec // Deterministic PRNG is intentional — reproducibility from seed.
	rng := rand.New(rand.NewSource(int64(cfg.Seed)))
	return &Scheduler{
		rng: rng,
		cfg: cfg,
	}
}

// Tick checks whether chaos events should fire at the given time.
// It rolls independently for each established peer, so the rate is
// a per-peer probability. Returns zero or more scheduled actions.
func (s *Scheduler) Tick(now time.Time, established []bool) []ScheduledAction {
	// Rate 0 means chaos is disabled.
	if s.cfg.Rate <= 0 {
		return nil
	}

	// Initialize timing on first call.
	if !s.started {
		s.started = true
		s.startTime = now
		s.nextTick = now
	}

	// Not time for next tick yet.
	if now.Before(s.nextTick) {
		return nil
	}

	// Advance to next interval.
	s.nextTick = now.Add(s.cfg.Interval)

	// Check warmup period — compare elapsed time from first tick.
	if s.cfg.Warmup > 0 && now.Sub(s.startTime) < s.cfg.Warmup {
		return nil
	}

	// Find established peers.
	candidates := s.establishedPeers(established)
	if len(candidates) == 0 {
		return nil
	}

	// Roll independently for each established peer.
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

// selectAction picks a chaos action type using weighted random selection.
func (s *Scheduler) selectAction() ChaosAction {
	roll := s.rng.Intn(totalWeight)
	cumulative := 0
	for _, w := range defaultWeights {
		cumulative += w.weight
		if roll < cumulative {
			return ChaosAction{Type: w.action}
		}
	}

	// Should never reach here, but default to TCP disconnect.
	return ChaosAction{Type: ActionTCPDisconnect}
}

// SetRate updates the chaos rate. Safe to call from another goroutine
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
