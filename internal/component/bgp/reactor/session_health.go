// Design: plan/learned/768-doctor-health-checks.md -- BGP session anomaly detection
// Overview: peer.go -- Peer struct and FSM state machine
// Related: session_prefix.go -- existing report bus usage pattern

package reactor

import (
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const reportCodeSessionStuck = "session-stuck"
const reportCodeSessionFlap = "session-flap"
const reportCodeEORTimeout = "eor-timeout"

const (
	sessionStuckTimeout = 5 * time.Minute
	flapWindow          = 5 * time.Minute
	flapThreshold       = 3
)

// sessionHealth tracks session-stuck, flap detection, and EOR timeout state
// for a single peer. All methods are safe for concurrent use.
type sessionHealth struct {
	mu           sync.Mutex
	peerAddr     string
	clock        clock.Clock
	stuckTick    clock.Timer
	stuckTimeout time.Duration
	stopped      bool
	// Ring buffer of Established->non-Established transition timestamps.
	// Bounded at flapThreshold+1 entries.
	flapTimes    []time.Time
	flapWarn     bool
	flapLifetime uint32 // Lifetime flap count (not windowed).
	// EOR timeout: timer fires warning if not all End-of-RIB markers are
	// received within the negotiated GR restart-time after Established.
	eorTick     clock.Timer
	eorExpected int // number of negotiated families expecting EOR
	eorReceived int // how many family EORs received so far
}

func newSessionHealth(peerAddr string, clk clock.Clock) *sessionHealth {
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
		sh.clearEORLocked()
		report.ClearWarning(reportSourceBGP, reportCodeSessionStuck, sh.peerAddr)
		report.ClearWarning(reportSourceBGP, reportCodeEORTimeout, sh.peerAddr)

	case PeerStateConnecting, PeerStateActive:
		if from == PeerStateEstablished {
			sh.recordFlapLocked()
		}
		sh.clearEORLocked()
		report.ClearWarning(reportSourceBGP, reportCodeEORTimeout, sh.peerAddr)
		sh.startStuckTimerLocked()
	}
}

func (sh *sessionHealth) startStuckTimerLocked() {
	sh.clearStuckLocked()
	sh.stuckTick = sh.clock.AfterFunc(sh.stuckTimeout, func() {
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
			"peer has not reached Established for "+textbuf.StringInt(int64(timeout/time.Minute))+" minutes",
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

// startEORTimer begins a timer that raises an eor-timeout warning if not all
// End-of-RIB markers are received within restartSeconds. familyCount is the
// number of negotiated address families; the warning clears only when that
// many EORs have been received (or the session ends). Called from peer_run.go
// after the session reaches Established with GR negotiated.
func (sh *sessionHealth) startEORTimer(restartSeconds uint16, familyCount int) {
	if restartSeconds == 0 || familyCount == 0 {
		return
	}
	timeout := time.Duration(restartSeconds) * time.Second

	sh.mu.Lock()
	defer sh.mu.Unlock()

	sh.clearEORLocked()
	sh.eorExpected = familyCount
	sh.eorReceived = 0
	sh.eorTick = sh.clock.AfterFunc(timeout, func() {
		sh.mu.Lock()
		if sh.stopped {
			sh.mu.Unlock()
			return
		}
		remaining := sh.eorExpected - sh.eorReceived
		if remaining <= 0 {
			sh.mu.Unlock()
			return
		}
		addr := sh.peerAddr
		report.RaiseWarning(
			reportSourceBGP,
			reportCodeEORTimeout,
			addr,
			"End-of-RIB incomplete: "+textbuf.StringInt(int64(remaining))+" of "+textbuf.StringInt(int64(sh.eorExpected))+" families pending after "+textbuf.StringInt(int64(restartSeconds))+"s",
			map[string]any{"restart_time": restartSeconds, "pending": remaining, "expected": sh.eorExpected},
		)
		sh.mu.Unlock()
	})
}

// onEORReceived records one family EOR. If all expected families have sent
// EOR, cancels the timeout timer and clears any warning. Called from
// reactor_notify.go when an EOR marker is detected.
func (sh *sessionHealth) onEORReceived() {
	sh.mu.Lock()
	defer sh.mu.Unlock()

	sh.eorReceived++
	if sh.eorExpected > 0 && sh.eorReceived >= sh.eorExpected {
		sh.clearEORLocked()
		report.ClearWarning(reportSourceBGP, reportCodeEORTimeout, sh.peerAddr)
	}
}

func (sh *sessionHealth) clearEORLocked() {
	if sh.eorTick != nil {
		sh.eorTick.Stop()
		sh.eorTick = nil
	}
	sh.eorExpected = 0
	sh.eorReceived = 0
}

// FlapCount returns the lifetime flap count for this peer.
func (sh *sessionHealth) FlapCount() uint32 {
	sh.mu.Lock()
	defer sh.mu.Unlock()
	return sh.flapLifetime
}

func (sh *sessionHealth) recordFlapLocked() {
	now := sh.clock.Now()
	sh.flapLifetime++

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
					"session flapping: "+textbuf.StringInt(int64(flapThreshold))+" transitions in "+textbuf.StringInt(int64(now.Sub(oldest)/time.Second))+" seconds",
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
	sh.clearEORLocked()
	sh.flapTimes = nil
	sh.flapWarn = false
	report.ClearWarning(reportSourceBGP, reportCodeSessionStuck, sh.peerAddr)
	report.ClearWarning(reportSourceBGP, reportCodeSessionFlap, sh.peerAddr)
	report.ClearWarning(reportSourceBGP, reportCodeEORTimeout, sh.peerAddr)
}
