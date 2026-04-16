// Design: docs/research/l2tpv2-ze-integration.md -- per-session PPP state ownership

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

	// Magic-Number for THIS session. Generated via crypto/rand by
	// the goroutine on entry; non-zero per RFC 1661 §6.4.
	magic uint32

	// Auth hook injection (replaced in spec-l2tp-6b-auth).
	authHook AuthHook

	// Iface backend for setting pppN MTU after LCP-Opened.
	backend IfaceBackend

	// pppOps for ioctls (PPPIOCSMRU). Injected for tests.
	ops pppOps

	// Manager's events channel (write-only from this goroutine).
	eventsOut chan<- Event

	// Manager's shutdown signal (the goroutine selects on this and
	// the chan fd's blocking read).
	stopCh <-chan struct{}

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
