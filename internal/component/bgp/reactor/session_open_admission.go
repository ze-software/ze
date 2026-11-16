// Design: docs/architecture/behavior/peer-lifecycle.md -- validated OPEN ownership.
// Related: session_handlers.go -- the only semantic OPEN constructor.
// RFC 4271 Sections 6.2 and 8.2.2 -- collision arbitration requires a valid OPEN.
package reactor

import (
	"net"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// openAdmission owns a validated remote OPEN and the pure negotiated result.
// Its owner MUST advance it once, or retire its Session. Construction performs
// no wire transition and begins no selected Peer resource generation.
type openAdmission struct {
	session    *Session
	proposal   *preparedOpen
	conn       net.Conn
	open       *message.Open
	negotiated *capability.Negotiated
	localCaps  []capability.Capability
}

func (a *openAdmission) advance() error {
	s := a.session
	s.openMu.Lock()
	defer s.unlockOpen()
	// RFC 4271 Section 8.2.2, Event10: negotiation cannot reset an expired wait.
	if s.rejectExpiredHold() {
		return ErrHoldTimerExpired
	}
	if s.tearingDown.Load() {
		return ErrNotConnected
	}
	if s.socketClosedIs(a.conn) {
		return ErrNotConnected
	}
	if s.Conn() != a.conn {
		return ErrNotConnected
	}
	s.publishNegotiation(a.proposal, a.open, a.localCaps, a.negotiated)
	s.applyBFDStrictNegotiation()
	return s.advanceAfterOpen(a.conn)
}

// negotiationEpoch separates a private connection's negotiated wire inputs
// from the selected Peer epoch. Every constructor chooses a sealed variant.
type negotiationEpoch interface {
	begin(*Session, bool)
	negotiationEpoch()
}

type localNegotiationEpoch struct{}
type selectedNegotiationEpoch struct{}
type privateNegotiationEpoch struct{}

func (localNegotiationEpoch) negotiationEpoch()            {}
func (selectedNegotiationEpoch) negotiationEpoch()         {}
func (privateNegotiationEpoch) negotiationEpoch()          {}
func (localNegotiationEpoch) begin(s *Session, gr bool)    { s.heldTypedNLRI().beginSession(gr) }
func (selectedNegotiationEpoch) begin(s *Session, gr bool) { s.heldTypedNLRI().beginSession(gr) }
func (privateNegotiationEpoch) begin(*Session, bool)       {}

// selectNegotiationEpoch MUST run after joining the private reader, under
// openMu, after incumbent cleanup. The transition begins the selected hold
// once if private negotiation already completed; otherwise publication does.
func (s *Session) selectNegotiationEpoch() {
	if _, private := s.negotiationEpoch.(privateNegotiationEpoch); !private {
		return
	}
	s.negotiationEpoch = selectedNegotiationEpoch{}
	if neg := s.Negotiated(); neg != nil {
		s.negotiationEpoch.begin(s, neg.GracefulRestart != nil)
	}
}
