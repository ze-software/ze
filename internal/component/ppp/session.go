// Design: docs/research/l2tpv2-ze-integration.md -- per-session PPP state ownership
// Related: manager.go -- Driver owns pppSession values in sessions map
// Related: session_run.go -- per-session goroutine main loop
// Related: auth_events.go -- authEventsOut + authRespCh fields on pppSession

package ppp

import (
	"io"
	"log/slog"
	"sync"
	"time"
)

// pppSession is the per-session state owned by one goroutine in the
// Manager. Field writes are exclusive to the goroutine that owns the
// session; reads from outside (e.g. SessionByID for `show l2tp
// session`) MUST acquire mu.
//
// Mirrors the L2TP reactor's per-tunnel ownership model: writer is
// the goroutine, readers lock briefly. RFC 1661 FSM transitions are
// dispatched via LCPDoTransition (pure function, no I/O) and the
// resulting actions are performed inline by the goroutine.
type pppSession struct {
	// Identification (immutable after StartSession).
	tunnelID  uint16
	sessionID uint16

	// Underlying I/O. chanFile is the wrapped chan fd; closing it
	// unblocks the goroutine's blocking Read and signals shutdown.
	chanFile io.ReadWriteCloser
	unitFD   int
	unitNum  int
	lnsMode  bool

	// Configuration captured from StartSession (immutable after
	// goroutine start).
	maxMRU       uint16
	echoInterval time.Duration
	echoFailures uint8
	authTimeout  time.Duration

	// Magic-Number for THIS session. Generated via crypto/rand by
	// the goroutine on entry; non-zero per RFC 1661 §6.4.
	magic uint32

	// Per-session CHAP Identifier counter. Shared between CHAP-MD5
	// (runCHAPAuthPhase) and MS-CHAPv2 (runMSCHAPv2AuthPhase) because
	// LCP negotiates exactly one Auth-Protocol method per session and
	// both methods use the same RFC 1994 Identifier-MUST-change-per-
	// Challenge rule (RFC 2759 Section 4 inherits the discipline).
	// Wraps at 256 which is far beyond any realistic re-auth cadence.
	chapIdentifier uint8

	// Iface backend for setting pppN MTU after LCP-Opened.
	backend IfaceBackend

	// pppOps for ioctls (PPPIOCSMRU). Injected for tests.
	ops pppOps

	// Driver's lifecycle events channel (write-only from this
	// goroutine). Carries Event (EventLCPUp / EventSessionUp / ...).
	eventsOut chan<- Event

	// Driver's auth events channel (write-only from this goroutine).
	// Carries EventAuthRequest / EventAuthSuccess / EventAuthFailure
	// consumed by the external auth handler (l2tp-auth plugin in
	// production, auto-accept responder in tests). Separate from
	// eventsOut so L2TP's reactor is not forced to pattern-match
	// auth types -- see auth_events.go.
	authEventsOut chan<- AuthEvent

	// Auth decision delivered by Driver.AuthResponse. Buffered(1) so
	// a caller that beats the session to the receive position does
	// not block. The session reads exactly once per auth phase.
	authRespCh chan authResponseMsg

	// Driver's shutdown signal (the goroutine selects on this and
	// the chan fd's blocking read).
	stopCh <-chan struct{}

	// Per-session cancellation, closed by Driver.StopSession via
	// sessStopOnce.Do. The auth phase selects on this in addition
	// to stopCh so a single-session teardown (as opposed to a
	// whole-driver shutdown) unblocks a goroutine parked on
	// authRespCh. Closing chanFile alone is insufficient because
	// the auth phase runs before or alongside readFrames rather
	// than consuming from it. sessStopOnce guarantees idempotent
	// close if a future cleanup path joins StopSession.
	sessStop     chan struct{}
	sessStopOnce sync.Once

	// done is closed by the goroutine on exit; the manager waits on
	// it during StopSession to confirm cleanup.
	done chan struct{}

	logger *slog.Logger

	// State below is mu-protected; written by the goroutine, read
	// by SessionByID.
	mu              sync.Mutex
	state           LCPState
	negotiatedMRU   uint16
	echoOutstanding uint8 // count of unanswered Echo-Request
}

// SessionInfo is a snapshot of pppSession state suitable for `show
// l2tp session` and tests. Returned by Manager.SessionByID under
// the per-session lock; safe to use after the lock is released.
type SessionInfo struct {
	TunnelID        uint16
	SessionID       uint16
	UnitNum         int
	State           LCPState
	NegotiatedMRU   uint16
	EchoOutstanding uint8
}

// snapshot copies the locked state into a SessionInfo. Caller MUST
// hold s.mu.
func (s *pppSession) snapshot() SessionInfo {
	return SessionInfo{
		TunnelID:        s.tunnelID,
		SessionID:       s.sessionID,
		UnitNum:         s.unitNum,
		State:           s.state,
		NegotiatedMRU:   s.negotiatedMRU,
		EchoOutstanding: s.echoOutstanding,
	}
}

// IfaceBackend is the small subset of internal/component/iface.Backend
// that the PPP package needs, expressed locally so ppp does not import
// iface (which would create a cycle through transport packages later).
// Production callers wrap iface.GetBackend(); tests inject fakes.
type IfaceBackend interface {
	SetMTU(name string, mtu int) error
	SetAdminUp(name string) error
}
