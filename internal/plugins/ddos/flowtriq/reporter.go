// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- DDoS event contract

package flowtriq

import (
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/ddosevent"
)

// reporter holds the one incident this node is reporting to the API, and owns
// every transition of it. The four detector events and a configuration apply
// are its five writers, and they arrive on different goroutines.
//
// The detector publishes from two goroutines: `(*detector).emitPending` fires
// from the trafficstat rate tick and `emitDetected` / `emitCharacterized` from
// the characterization goroutine that `(*detector).onAttackStart` starts.
// `Event[T].Emit` (internal/core/events/typed.go) delivers inline on whichever
// one publishes, so a callback runs on the publisher's goroutine. The
// detector's own `emitMu` orders those callbacks against each other and reaches
// no further. A configuration apply arrives on a third goroutine: the plugin
// SDK invokes `OnConfigApply` from its reader loop, `(*Plugin).eventLoop`
// (pkg/plugin/sdk/sdk_dispatch.go).
//
// Nothing ordered an apply against a delivery, which is what this type exists
// to fix.
type reporter struct {
	// mu guards every field below it, and is held for the WHOLE of each method
	// rather than around a copy of the state. Each method reads the state,
	// posts it to the API and writes back what the post established, so a lock
	// that covered only the reads would still let an apply resolve the incident
	// and clear uuid between onOngoing's "is there an incident" test and its
	// post, sending an update for an incident the remote side has closed.
	//
	// The cost is that an apply waits for a post already in flight, which the
	// HTTP client caps at 10 seconds (newClient, client.go). ApplyBudget covers
	// it (register.go).
	mu         sync.Mutex
	cl         *client
	uuid       string
	family     ddosevent.AttackFamily
	peakPPS    float64
	peakBPS    float64
	confidence int
	start      time.Time
}

// onDetected opens an incident on the API and remembers the identity the API
// gave it. A failed open leaves uuid empty, which is what stops every later
// handler from reporting against an incident the remote side never opened.
func (r *reporter) onDetected(e *ddosevent.AttackDetected) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cl == nil {
		return
	}
	uuid, err := r.cl.openIncident(e)
	if err != nil {
		logger().Warn("ddos-flowtriq: open incident failed", "error", err)
		return
	}
	r.uuid = uuid
	r.family = e.Family
	r.peakPPS = e.PeakRxPps
	r.peakBPS = e.PeakRxBps
	r.confidence = 0
	r.start = time.Now()
	logger().Info("ddos-flowtriq: incident opened", "uuid", uuid)
}

// onCharacterized records the confidence score, which is unavailable when the
// attack is first detected. The next update and the resolve report it.
func (r *reporter) onCharacterized(e *ddosevent.AttackCharacterized) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.uuid == "" {
		return
	}
	r.confidence = e.Confidence
}

// onOngoing raises the recorded peaks and posts the incident's current rates.
func (r *reporter) onOngoing(e *ddosevent.AttackOngoing) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cl == nil || r.uuid == "" {
		return
	}
	if e.CurrentPps > r.peakPPS {
		r.peakPPS = e.CurrentPps
	}
	if e.CurrentBps > r.peakBPS {
		r.peakBPS = e.CurrentBps
	}
	if err := r.cl.updateIncident(r.uuid, e.CurrentPps, e.CurrentBps, r.family, r.confidence); err != nil {
		logger().Warn("ddos-flowtriq: update incident failed", "error", err)
	}
}

// onCleared resolves the incident on the API when the attack stops.
func (r *reporter) onCleared(_ *ddosevent.AttackCleared) {
	r.mu.Lock()
	defer r.mu.Unlock()
	uuid := r.uuid
	duration, had, err := r.resolveLocked()
	if !had {
		return
	}
	if err != nil {
		logger().Warn("ddos-flowtriq: resolve incident failed", "error", err)
		return
	}
	logger().Info("ddos-flowtriq: incident resolved", "uuid", uuid, "duration", duration)
}

// swapClient resolves whatever incident is open and installs the client the new
// configuration asks for, which is nil when the configuration disables
// reporting. The resolve happens first: the incident belongs to the client that
// opened it, and posting it through the new one would name an incident that
// client never saw.
func (r *reporter) swapClient(cl *client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, _, err := r.resolveLocked(); err != nil {
		logger().Warn("ddos-flowtriq: resolve on config reload failed", "error", err)
	}
	r.cl = cl
}

// resolveLocked posts the open incident's resolve and forgets it. It reports
// the duration it posted, whether there was an incident at all, and what the
// post answered. The caller holds mu and writes the log line, because the
// operator reads a different sentence for an attack that ended than for a
// reload that cut one short.
func (r *reporter) resolveLocked() (duration float64, had bool, err error) {
	if r.cl == nil || r.uuid == "" {
		return 0, false, nil
	}
	duration = time.Since(r.start).Seconds()
	err = r.cl.resolveIncident(r.uuid, duration, r.peakPPS, r.peakBPS, r.confidence)
	// The incident is forgotten either way. A resolve that failed cannot be
	// retried from here, and keeping the identity would make the next attack
	// report its rates against the incident this one could not close.
	r.uuid = ""
	r.confidence = 0
	return duration, true, err
}
