// Design: plan/learned/679-chaos-ai.md -- Watchdog consumer for anomaly detection

package watchdog

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/chaos/peer"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Config holds Watchdog tuning parameters.
type Config struct {
	ReconnectTimeout time.Duration
	PlateauDuration  time.Duration
	WarmupMultiplier float64
	RateLimit        time.Duration
	Warmup           time.Duration
}

// DefaultConfig returns production defaults.
func DefaultConfig() Config {
	return Config{
		ReconnectTimeout: 30 * time.Second,
		PlateauDuration:  10 * time.Second,
		WarmupMultiplier: 2,
		RateLimit:        10 * time.Second,
		Warmup:           5 * time.Second,
	}
}

// Problem describes a detected anomaly.
type Problem struct {
	Type      string
	PeerIndex int
	Message   string
	Time      time.Time
}

type rateLimitKey struct {
	anomaly   string
	peerIndex int
}

type peerRouteState struct {
	lastRecvCount int
	lastChangeAt  time.Time
}

// Watchdog implements report.Consumer and detects anomalies in the chaos
// event stream, printing structured PROBLEM: lines to the given writer.
type Watchdog struct {
	mu  sync.Mutex
	cfg Config
	out io.Writer

	disconnectedAt  map[int]time.Time
	establishedAt   map[int]time.Time
	routeState      map[int]*peerRouteState
	routesSent      map[int]int
	routesRecv      map[int]int
	routesHWM       map[int]int // high-water mark for regression detection
	propertyPass    map[string]bool
	lastPrinted     map[rateLimitKey]time.Time
	problems        []Problem
	maxProblems     int
	chaosWithdrawal map[int]bool
}

// New creates a Watchdog consumer writing to the given output.
func New(out io.Writer, cfg Config) *Watchdog {
	return &Watchdog{
		cfg:             cfg,
		out:             out,
		disconnectedAt:  make(map[int]time.Time),
		establishedAt:   make(map[int]time.Time),
		routeState:      make(map[int]*peerRouteState),
		routesSent:      make(map[int]int),
		routesRecv:      make(map[int]int),
		routesHWM:       make(map[int]int),
		propertyPass:    make(map[string]bool),
		lastPrinted:     make(map[rateLimitKey]time.Time),
		maxProblems:     10000,
		chaosWithdrawal: make(map[int]bool),
	}
}

// ProcessEvent implements report.Consumer.
func (w *Watchdog) ProcessEvent(ev peer.Event) {
	w.mu.Lock()
	defer w.mu.Unlock()

	switch ev.Type {
	case peer.EventEstablished:
		delete(w.disconnectedAt, ev.PeerIndex)
		w.establishedAt[ev.PeerIndex] = ev.Time
		w.routesSent[ev.PeerIndex] = 0
		w.routesRecv[ev.PeerIndex] = 0
		w.routesHWM[ev.PeerIndex] = 0
		delete(w.chaosWithdrawal, ev.PeerIndex)

	case peer.EventDisconnected:
		w.disconnectedAt[ev.PeerIndex] = ev.Time
		delete(w.establishedAt, ev.PeerIndex)

	case peer.EventRouteSent:
		w.routesSent[ev.PeerIndex]++
		w.updateRouteState(ev.PeerIndex, ev.Time)

	case peer.EventRouteReceived:
		w.routesRecv[ev.PeerIndex]++
		if w.routesRecv[ev.PeerIndex] > w.routesHWM[ev.PeerIndex] {
			w.routesHWM[ev.PeerIndex] = w.routesRecv[ev.PeerIndex]
		}
		w.updateRouteState(ev.PeerIndex, ev.Time)

	case peer.EventRouteWithdrawn:
		if w.routesRecv[ev.PeerIndex] > 0 {
			w.routesRecv[ev.PeerIndex]--
		}
		w.updateRouteState(ev.PeerIndex, ev.Time)
		if !w.chaosWithdrawal[ev.PeerIndex] && w.routesRecv[ev.PeerIndex] < w.routesHWM[ev.PeerIndex] {
			var bRegr textbuf.Buffer
			w.emit(ev.Time, "route-regression", ev.PeerIndex,
				bRegr.Reset().Str("PROBLEM: peer ").Int(int64(ev.PeerIndex)).Str(" lost routes (was ").Int(int64(w.routesHWM[ev.PeerIndex])).Str(", now ").Int(int64(w.routesRecv[ev.PeerIndex])).Str(") -- no withdrawal").String())
		}

	case peer.EventWithdrawalSent:
		w.chaosWithdrawal[ev.PeerIndex] = true

	case peer.EventError:
		msg := "unknown error"
		if ev.Err != nil {
			msg = ev.Err.Error()
		}
		var bErr textbuf.Buffer
		w.emit(ev.Time, "error", ev.PeerIndex,
			bErr.Reset().Str("PROBLEM: peer ").Int(int64(ev.PeerIndex)).Str(" error: ").Str(msg).String())

	case peer.EventDroppedEvents:
		var bDrop textbuf.Buffer
		w.emit(ev.Time, "dropped-events", ev.PeerIndex,
			bDrop.Reset().Str("PROBLEM: peer ").Int(int64(ev.PeerIndex)).Str(" dropped ").Int(int64(ev.Count)).Str(" events (overloaded)").String())

	case peer.EventEORSent:
		// EOR clears convergence stall tracking.
		delete(w.establishedAt, ev.PeerIndex)

	case peer.EventChaosExecuted, peer.EventReconnecting, peer.EventRouteAction:
		// No watchdog action.
	}

	w.checkStateful(ev.Time)
}

// Close implements report.Consumer.
func (w *Watchdog) Close() error { return nil }

// Problems returns all detected problems (thread-safe).
func (w *Watchdog) Problems() []Problem {
	w.mu.Lock()
	defer w.mu.Unlock()
	result := make([]Problem, len(w.problems))
	copy(result, w.problems)
	return result
}

// SetPropertyResult updates whether a property is passing. When a property
// transitions from pass to fail, a PROBLEM line is emitted.
func (w *Watchdog) SetPropertyResult(name string, pass bool, firstViolation string, at time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()

	wasPassing, known := w.propertyPass[name]
	w.propertyPass[name] = pass

	if known && wasPassing && !pass {
		w.emit(at, "property-violation", -1,
			"PROBLEM: property "+name+" FAILED: "+firstViolation)
	}
}

func (w *Watchdog) checkStateful(now time.Time) {
	// Peer not reconnecting.
	for peerIdx, disconnectedAt := range w.disconnectedAt {
		if now.Sub(disconnectedAt) >= w.cfg.ReconnectTimeout {
			var bStuck textbuf.Buffer
			w.emit(now, "peer-stuck-down", peerIdx,
				bStuck.Reset().Str("PROBLEM: peer ").Int(int64(peerIdx)).Str(" not reconnected after ").Str(now.Sub(disconnectedAt).Truncate(time.Second).String()).String())
		}
	}

	// Route count plateau.
	for peerIdx, rs := range w.routeState {
		sent := w.routesSent[peerIdx]
		recv := w.routesRecv[peerIdx]
		if recv < sent && recv == rs.lastRecvCount && now.Sub(rs.lastChangeAt) >= w.cfg.PlateauDuration {
			var bPlat textbuf.Buffer
			w.emit(now, "route-plateau", peerIdx,
				bPlat.Reset().Str("PROBLEM: peer ").Int(int64(peerIdx)).Str(" stuck at ").Int(int64(recv)).Byte('/').Int(int64(sent)).Str(" routes (no change for ").Str(now.Sub(rs.lastChangeAt).Truncate(time.Second).String()).Byte(')').String())
		}
	}

	// Convergence stall (no EOR within warmup * multiplier after Established).
	eorDeadline := time.Duration(float64(w.cfg.Warmup) * w.cfg.WarmupMultiplier)
	for peerIdx, estAt := range w.establishedAt {
		if now.Sub(estAt) >= eorDeadline {
			var bStall textbuf.Buffer
			w.emit(now, "convergence-stall", peerIdx,
				bStall.Reset().Str("PROBLEM: peer ").Int(int64(peerIdx)).Str(" initial sync stalled (no EOR after ").Str(now.Sub(estAt).Truncate(time.Second).String()).Byte(')').String())
		}
	}
}

func (w *Watchdog) updateRouteState(peerIdx int, at time.Time) {
	recv := w.routesRecv[peerIdx]
	rs, ok := w.routeState[peerIdx]
	if !ok {
		w.routeState[peerIdx] = &peerRouteState{lastRecvCount: recv, lastChangeAt: at}
		return
	}
	if recv != rs.lastRecvCount {
		rs.lastRecvCount = recv
		rs.lastChangeAt = at
	}
}

func (w *Watchdog) emit(at time.Time, anomaly string, peerIdx int, msg string) {
	key := rateLimitKey{anomaly: anomaly, peerIndex: peerIdx}
	if last, ok := w.lastPrinted[key]; ok && at.Sub(last) < w.cfg.RateLimit {
		return
	}
	w.lastPrinted[key] = at
	if len(w.problems) >= w.maxProblems {
		w.problems = w.problems[1:]
	}
	w.problems = append(w.problems, Problem{
		Type:      anomaly,
		PeerIndex: peerIdx,
		Message:   msg,
		Time:      at,
	})
	if _, err := fmt.Fprintln(w.out, msg); err != nil { //nolint:errcheck // output
		return
	}
}
