// Design: plan/spec-doctor-health-checks.md -- BGP session anomaly detection
// Overview: peer.go -- Peer struct and FSM state machine
// Related: session_prefix.go -- existing report bus usage pattern

package reactor

import (
	"sync"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/report"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

const reportCodeSessionStuck = "session-stuck"
const reportCodeSessionFlap = "session-flap"

const (
	sessionStuckTimeout = 5 * time.Minute
	flapWindow          = 5 * time.Minute
	flapThreshold       = 3
)

// sessionHealth tracks session-stuck and flap detection state for a single peer.
// All methods are safe for concurrent use.
type sessionHealth struct {
	mu           sync.Mutex
	peerAddr     string
	clock        interface{ Now() time.Time }
	stuckTick    *time.Timer
	stuckTimeout time.Duration
	stopped      bool
	// Ring buffer of Established->non-Established transition timestamps.
	// Bounded at flapThreshold+1 entries.
	flapTimes []time.Time
	flapWarn  bool
}

func newSessionHealth(peerAddr string, clk interface{ Now() time.Time }) *sessionHealth {
	return &sessionHealth{
		peerAddr:     peerAddr,
		clock:        clk,
		stuckTimeout: sessionStuckTimeout,
	}
}

// onStateChange is called from Peer.setState on every state transition.
func (sh *sessionHealth) onStateChange(from, to PeerState) {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	switch to {
	case PeerStateEstablished:
		sh.clearStuckLocked()
		report.ClearWarning(reportSourceBGP, reportCodeSessionStuck, sh.peerAddr)

	case PeerStateStopped:
		sh.clearStuckLocked()
		report.ClearWarning(reportSourceBGP, reportCodeSessionStuck, sh.peerAddr)

	case PeerStateConnecting, PeerStateActive:
		if from == PeerStateEstablished {
			sh.recordFlapLocked()
		}
		sh.startStuckTimerLocked()
	}
}

func (sh *sessionHealth) startStuckTimerLocked() {
	sh.clearStuckLocked()
	sh.stuckTick = time.AfterFunc(sh.stuckTimeout, func() {
		sh.mu.Lock()
		if sh.stopped {
			sh.mu.Unlock()
			return
		}
		addr := sh.peerAddr
		timeout := sh.stuckTimeout
		report.RaiseWarning(
			reportSourceBGP,
			reportCodeSessionStuck,
			addr,
			"peer has not reached Established for "+textbuf.Int(int64(timeout/time.Minute))+" minutes",
			nil,
		)
		sh.mu.Unlock()
	})
}

func (sh *sessionHealth) clearStuckLocked() {
	if sh.stuckTick != nil {
		sh.stuckTick.Stop()
		sh.stuckTick = nil
	}
}

func (sh *sessionHealth) recordFlapLocked() {
	now := sh.clock.Now()

	sh.flapTimes = append(sh.flapTimes, now)
	// Trim to bounded size.
	if len(sh.flapTimes) > flapThreshold+1 {
		sh.flapTimes = sh.flapTimes[len(sh.flapTimes)-flapThreshold-1:]
	}

	// Check if we have enough transitions within the window.
	if len(sh.flapTimes) >= flapThreshold {
		oldest := sh.flapTimes[len(sh.flapTimes)-flapThreshold]
		if now.Sub(oldest) <= flapWindow {
			if !sh.flapWarn {
				sh.flapWarn = true
				report.RaiseWarning(
					reportSourceBGP,
					reportCodeSessionFlap,
					sh.peerAddr,
					"session flapping: "+textbuf.Int(int64(flapThreshold))+" transitions in "+textbuf.Int(int64(now.Sub(oldest)/time.Second))+" seconds",
					map[string]any{"transitions": flapThreshold},
				)
			}
			return
		}
	}

	// If we had a flap warning but the window has passed, clear it.
	if sh.flapWarn && len(sh.flapTimes) >= flapThreshold {
		oldest := sh.flapTimes[len(sh.flapTimes)-flapThreshold]
		if now.Sub(oldest) > flapWindow {
			sh.flapWarn = false
			report.ClearWarning(reportSourceBGP, reportCodeSessionFlap, sh.peerAddr)
		}
	}
}

// stop cleans up timers and clears any active warnings.
func (sh *sessionHealth) stop() {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	sh.stopped = true
	sh.clearStuckLocked()
	sh.flapTimes = nil
	sh.flapWarn = false
	report.ClearWarning(reportSourceBGP, reportCodeSessionStuck, sh.peerAddr)
	report.ClearWarning(reportSourceBGP, reportCodeSessionFlap, sh.peerAddr)
}
