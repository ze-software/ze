// Design: docs/architecture/forward-congestion-pool.md -- two-threshold enforcement
// Overview: forward_pool.go -- forward pool dispatch and worker lifecycle
// Related: forward_pool_weight_tracker.go -- per-peer weight tracking and ratio calculation

package reactor

import (
	"errors"
	"net/netip"
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/fsm"
	"codeberg.org/thomas-mangin/ze/internal/component/bgp/message"
	"codeberg.org/thomas-mangin/ze/internal/core/clock"
)

// ErrCongestionTeardown is set as the close reason when a session is torn
// down due to forward pool congestion (AC-4). Exported for test assertions.
var ErrCongestionTeardown = errors.New("congestion teardown")

// Congestion thresholds for two-threshold enforcement (AC-2, AC-4).
const (
	// congestionDenialThreshold is the pool usage ratio above which buffer
	// denial activates for culprit source peers (Threshold 1, AC-2).
	congestionDenialThreshold = 0.80

	// congestionTeardownThreshold is the pool usage ratio above which forced
	// teardown is considered for the worst destination peer (Threshold 2, AC-4).
	congestionTeardownThreshold = 0.95

	// congestionTeardownRatio is the minimum usage-to-weight ratio a peer must
	// exceed before it becomes a teardown candidate (AC-4: >2x weight share).
	congestionTeardownRatio = 2.0

	// congestionGraceDefault is the default grace period before forced teardown.
	// Configurable via ze.fwd.teardown.grace.
	congestionGraceDefault = 5 * time.Second
)

// congestionController implements the two-threshold enforcement for forward
// pool congestion (Phase 5). It decides:
//   - Whether to deny overflow buffer acquisition for a destination peer (AC-2)
//   - Whether to force-teardown the worst destination peer (AC-4)
//
// The controller does not own goroutines. It is queried synchronously by the
// forward pool dispatch path (ShouldDeny) and the batch handler (ShouldTeardown).
//
// Thread-safe. All methods may be called from any goroutine.
type congestionController struct {
	mu sync.Mutex

	// poolUsedRatio returns the current overflow pool usage (0.0-1.0).
	poolUsedRatio func() float64

	// overflowDepths returns per-peer overflow item counts.
	overflowDepths func() map[string]int

	// weights provides usage-to-weight ratio calculation.
	weights *weightTracker

	// gracePeriod is the duration a peer must exceed teardown thresholds
	// before being torn down. Configurable via ze.fwd.teardown.grace.
	gracePeriod time.Duration

	// teardownStart tracks when the worst peer first crossed the teardown
	// threshold. Reset when the condition clears. Protected by mu.
	teardownStart time.Time

	// teardownPeer is the address of the peer that crossed the threshold.
	// Reset when the condition clears. Protected by mu.
	teardownPeer string

	// clock for testability.
	clock clock.Clock

	// onTeardown is called when forced teardown fires. The callback receives
	// the peer address and whether the peer has GR capability.
	// Must not block -- the caller holds no locks.
	onTeardown func(peerAddr netip.AddrPort, grCapable bool)

	// peerGRCapable returns whether a peer has negotiated Graceful Restart.
	peerGRCapable func(peerAddr string) bool

	// metrics counters (atomic via callbacks, no lock needed).
	onDenied        func() // called on each buffer denial
	onTeardownFired func() // called when teardown executes
}

// congestionConfig holds configuration for the congestion controller.
type congestionConfig struct {
	gracePeriod     time.Duration
	poolUsedRatio   func() float64
	overflowDepths  func() map[string]int
	weights         *weightTracker
	clock           clock.Clock
	peerGRCapable   func(string) bool
	onTeardown      func(netip.AddrPort, bool)
	onDenied        func()
	onTeardownFired func()
}

// newCongestionController creates a congestion controller.
// Requires poolUsedRatio, overflowDepths, and weights to be non-nil.
func newCongestionController(cfg congestionConfig) *congestionController {
	grace := cfg.gracePeriod
	if grace < time.Second {
		grace = congestionGraceDefault
	}
	clk := cfg.clock
	if clk == nil {
		clk = clock.RealClock{}
	}
	return &congestionController{
		poolUsedRatio:   cfg.poolUsedRatio,
		overflowDepths:  cfg.overflowDepths,
		weights:         cfg.weights,
		gracePeriod:     grace,
		clock:           clk,
		onTeardown:      cfg.onTeardown,
		peerGRCapable:   cfg.peerGRCapable,
		onDenied:        cfg.onDenied,
		onTeardownFired: cfg.onTeardownFired,
	}
}

// ShouldDeny returns true if buffer acquisition should be denied for a
// destination peer. Called from the overflow dispatch path (AC-2).
//
// Denial activates when: pool > 80% AND the destination peer has the
// highest usage-to-weight ratio. This is the soft backpressure threshold.
func (cc *congestionController) ShouldDeny(destPeerAddr string) bool {
	if cc == nil {
		return false
	}

	ratio := cc.poolUsedRatio()
	if ratio < congestionDenialThreshold {
		return false
	}

	depths := cc.overflowDepths()
	worstAddr, _ := cc.weights.WorstPeerRatio(depths)

	if worstAddr == destPeerAddr {
		if cc.onDenied != nil {
			cc.onDenied()
		}
		return true
	}
	return false
}

// CheckTeardown evaluates whether forced teardown should fire for the
// given peer. Called by the worker goroutine after each batch (AC-4).
//
// Teardown fires when ALL conditions hold:
//   - Pool > 95% full
//   - The peer has the highest usage-to-weight ratio AND ratio > 2x
//   - This condition has persisted for the grace period
//
// When teardown fires, onTeardown is called with the peer address and
// GR capability. The caller (batch handler) should not write further.
func (cc *congestionController) CheckTeardown(failedPeerAddr netip.AddrPort) {
	if cc == nil {
		return
	}

	ratio := cc.poolUsedRatio()
	if ratio < congestionTeardownThreshold {
		cc.clearGrace()
		return
	}

	depths := cc.overflowDepths()
	worstAddr, worstRatio := cc.weights.WorstPeerRatio(depths)

	// The failed peer must be the worst offender AND exceed 2x weight.
	// Compare IP-only (no port) to match weightTracker and OverflowDepths key format.
	failedAddr := failedPeerAddr.Addr().String()
	if worstAddr != failedAddr || worstRatio < congestionTeardownRatio {
		if worstAddr == failedAddr {
			// This peer IS worst but ratio < 2x. Don't clear grace for
			// other peers (finding 5: non-worst workers were resetting grace).
			return
		}
		// Different peer or no worst peer -- don't reset grace for the
		// actual worst peer. Only clear when pool drops below threshold
		// (handled at line 169-172 above).
		return
	}

	now := cc.clock.Now()

	cc.mu.Lock()
	if cc.teardownPeer != worstAddr {
		// New worst peer or first time crossing threshold.
		cc.teardownPeer = worstAddr
		cc.teardownStart = now
		cc.mu.Unlock()
		return
	}

	elapsed := now.Sub(cc.teardownStart)
	if elapsed < cc.gracePeriod {
		cc.mu.Unlock()
		return
	}

	// Grace period elapsed. Fire teardown.
	cc.teardownPeer = ""
	cc.teardownStart = time.Time{}
	cc.mu.Unlock()

	grCapable := false
	if cc.peerGRCapable != nil {
		grCapable = cc.peerGRCapable(worstAddr)
	}

	fwdLogger().Warn("congestion forced teardown",
		"peer", failedPeerAddr,
		"ratio", worstRatio,
		"pool_used", ratio,
		"grace_seconds", elapsed.Seconds(),
		"gr_capable", grCapable,
	)

	if cc.onTeardownFired != nil {
		cc.onTeardownFired()
	}

	if cc.onTeardown != nil {
		cc.onTeardown(failedPeerAddr, grCapable)
	}
}

// clearGrace resets the teardown grace tracking. Called when conditions
// no longer meet the teardown threshold.
func (cc *congestionController) clearGrace() {
	cc.mu.Lock()
	cc.teardownPeer = ""
	cc.teardownStart = time.Time{}
	cc.mu.Unlock()
}

// congestionTeardownPeer performs GR-aware session teardown for congestion.
// For GR-capable peers: close TCP without NOTIFICATION (route retention).
// For non-GR peers: send Cease/OutOfResources NOTIFICATION then close.
// This is the default onTeardown callback wired by the reactor.
func congestionTeardownPeer(peers func(netip.AddrPort) *Peer) func(netip.AddrPort, bool) {
	return func(peerAddr netip.AddrPort, grCapable bool) {
		peer := peers(peerAddr)
		if peer == nil {
			return
		}

		if grCapable {
			// GR-aware: close TCP without NOTIFICATION.
			// Per RFC 4724 Section 4: TCP failure without NOTIFICATION
			// triggers route retention (Event 18), not route deletion.
			// Uses EventTCPConnectionFails (not EventManualStop) to match
			// the semantic intent: we are simulating a TCP failure.
			peer.mu.Lock()
			session := peer.session
			peer.mu.Unlock()
			if session != nil {
				session.setCloseReason(ErrCongestionTeardown)
				session.closeConn()
				_ = session.fsm.Event(fsm.EventTCPConnectionFails)
			}
		} else {
			// Non-GR: send NOTIFICATION Cease/OutOfResources (subcode 8).
			_ = peer.Teardown(message.NotifyCeaseOutOfResources, "")
		}
	}
}
