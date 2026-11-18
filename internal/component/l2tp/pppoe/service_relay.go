// Design: docs/architecture/l2tp/followup-l2tp-call.md -- two-way Service-Name relay lifetime
// Related: session.go -- exact admission ownership; subsystem.go -- existing event consumer

package pppoe

import (
	"context"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/core/callsink"
)

// serviceRelayEnd belongs to one exact admission. The table retains it until
// PADS completes; no call callback sends discovery frames or waits for a worker.
type serviceRelayEnd struct {
	preparation *serviceBridgePreparation
	reason      string
}

type readyServiceRelayEnd struct {
	session *Session
	reason  string
}

// beginServiceRelay installs the handoff before Relay can call Ended inline.
// A nil result refuses a vanished admission; it MUST NOT start another call.
func (s *InterfaceServer) beginServiceRelay(sid uint16, opened dataplane.OpenedSession) *serviceBridgePreparation {
	st := s.sessions
	st.mu.Lock()
	defer st.mu.Unlock()
	session := st.sessions[sid]
	// refuse-by-default: guards relayToL2TP -- a departed admission cannot originate a new call
	if session == nil {
		return nil
	}
	if session.Opened != opened {
		return nil
	}
	if session.State == StateTeardown {
		return nil
	}
	preparation := &serviceBridgePreparation{sessions: st, sid: sid, opened: opened, wake: s.relayWake}
	session.relay = preparation
	return preparation
}

// currentSessionLocked checks the exact holder as well as its created owner.
// Caller MUST hold sessions.mu. A stale callback never owns a replacement SID.
func (p *serviceBridgePreparation) currentSessionLocked() *Session {
	session := p.sessions.sessions[p.sid]
	// refuse-by-default: guards Ended -- callbacks for departed or replaced admissions are harmless
	if session == nil {
		return nil
	}
	if session.Opened != p.opened {
		return nil
	}
	if session.relay != p {
		return nil
	}
	if session.State == StateTeardown {
		return nil
	}
	return session
}

// Ended is nonblocking and safe for concurrent use. The existing event consumer
// owns PADT and teardown after the admitting PADS has been sent.
func (p *serviceBridgePreparation) Ended(reason string) {
	st := p.sessions
	st.mu.Lock()
	if p.currentSessionLocked() == nil {
		st.mu.Unlock()
		return
	}
	if _, pending := st.relayEnds[p.sid]; pending {
		st.mu.Unlock()
		return
	}
	st.relayEnds[p.sid] = serviceRelayEnd{preparation: p, reason: reason}
	st.mu.Unlock()
	p.notify()
}

// notify is only a hint. A full channel coalesces another hint; the pending
// table entry remains, so the consumer cannot lose work. A bare test can drain
// directly when it supplies no consumer channel.
func (p *serviceBridgePreparation) notify() {
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

// accept consumes the returned Cancel token. If teardown won before Relay
// returned, the token is canceled outside the table lock instead of orphaned.
// The sink MUST supply a nonnil token whenever it accepts a request.
func (p *serviceBridgePreparation) accept(call callsink.RelayCall) {
	st := p.sessions
	st.mu.Lock()
	if p.currentSessionLocked() != nil {
		p.call = call
		st.mu.Unlock()
		return
	}
	st.mu.Unlock()
	call.Cancel()
}

// abandon removes an unmatched or failed attempt before local PPP takes over.
// A returned partial token is canceled outside the table lock.
func (p *serviceBridgePreparation) abandon(call callsink.RelayCall) {
	st := p.sessions
	st.mu.Lock()
	if p.currentSessionLocked() != nil {
		st.sessions[p.sid].relay = nil
		delete(st.relayEnds, p.sid)
	}
	st.mu.Unlock()
	if call != nil {
		call.Cancel()
	}
}

// cancel transfers the token out exactly once. It MUST run outside every table
// lock: a sink may synchronously report Ended while it tears its call down.
func (p *serviceBridgePreparation) cancel() {
	st := p.sessions
	st.mu.Lock()
	call := p.call
	p.call = nil
	st.mu.Unlock()
	if call != nil {
		call.Cancel()
	}
}

// takeServiceRelayEnds claims ready teardowns while keeping their SIDs reserved.
// Discovery entries stay pending until markSessionFor follows the PADS write.
// The result is bounded by this access table's admission limit.
func (st *SessionTable) takeServiceRelayEnds() []readyServiceRelayEnd {
	st.mu.Lock()
	defer st.mu.Unlock()
	var ready []readyServiceRelayEnd
	for sid, ended := range st.relayEnds {
		session := ended.preparation.currentSessionLocked()
		if session == nil {
			delete(st.relayEnds, sid)
			continue
		}
		if session.State == StateDiscovery {
			continue
		}
		session.State = StateTeardown
		delete(st.relayEnds, sid)
		ready = append(ready, readyServiceRelayEnd{session: session, reason: ended.reason})
	}
	return ready
}

// drainServiceRelayEnds runs only on the existing event consumer (or directly
// in a test before its consumer starts). Callbacks never acquire Subsystem.mu.
func (s *Subsystem) drainServiceRelayEnds() {
	s.mu.Lock()
	servers := make([]*InterfaceServer, 0, len(s.servers))
	for _, server := range s.servers {
		servers = append(servers, server)
	}
	s.mu.Unlock()
	for _, server := range servers {
		for _, ended := range server.sessions.takeServiceRelayEnds() {
			server.logger.Info("pppoe: relayed L2TP call ended", "sid", ended.session.SID, "reason", ended.reason)
			server.finishSessionDown(ended.session)
		}
	}
}

// admissionSnapshot captures the table-owned session and whether PADS completed.
// A closing admission stays in the table until closeDone closes, keeping this
// snapshot and the reserved SID valid throughout cleanup.
type admissionSnapshot struct {
	session     *Session
	publication PADSPublication
}

// admissions snapshots at most the configured admission limit under mu.
// The caller MUST NOT read mutable Session fields through the returned rows.
func (st *SessionTable) admissions() []admissionSnapshot {
	st.mu.Lock()
	defer st.mu.Unlock()
	admissions := make([]admissionSnapshot, 0, len(st.sessions))
	for _, session := range st.sessions {
		admissions = append(admissions, admissionSnapshot{session: session, publication: session.padsPublication})
	}
	return admissions
}

// beginAdmissionClose claims one cleanup attempt without freeing the SID. Duplicate
// event handlers MUST return immediately: waiting here would block the sole
// consumer needed by a PPP finalizer with a full EventsOut queue.
func (st *SessionTable) beginAdmissionClose(expected *Session) (*Session, removedAdmission) {
	if expected == nil {
		return nil, removedAdmission{}
	}
	sid := expected.SID
	st.mu.Lock()
	defer st.mu.Unlock()
	session := st.sessions[sid]
	// refuse-by-default: guards closeAdmitted -- an absent admission owns no resources
	if session != expected {
		return nil, removedAdmission{}
	}
	if session.closeStarted {
		return nil, removedAdmission{}
	}
	if publication := session.wireEnd; publication != nil {
		select {
		case <-publication.done:
		default:
			// refuse-by-default: guards closeAdmitted -- only the original
			// wire-end handler may release its in-flight publication receipt.
			// Its deferred completion retries cleanup without blocking readers.
			return nil, removedAdmission{}
		}
	}
	if session.pppTransferred && !session.pppTerminal && !session.pppRelayReleased {
		return nil, removedAdmission{}
	}
	session.closeStarted = true
	session.State = StateTeardown
	removed := removedAdmission{opened: session.Opened, unusedLease: session.unusedLease, relay: session.relay, retryUnused: session.closeErr != nil}
	delete(st.relayEnds, sid)
	return session, removed
}

// completeAdmissionClose removes only this owner after every resource cleanup.
// The caller MUST defer it before invoking cleanup that can panic.
func (st *SessionTable) completeAdmissionClose(session *Session) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.sessions[session.SID] != session {
		return
	}
	session.Opened = nil
	session.unusedLease = nil
	session.relay = nil
	st.removeAdmittedLocked(session.SID)
	close(session.closeDone)
}

// closeAdmissions runs after discovery and PPP workers joined. Published
// admissions receive PADT after transport closure; an entry
// still in discovery never advertised a session and receives no PADT.
// Caller MUST hold lifecycleMu and MUST NOT hold mu or dispatchMu.
func (s *Subsystem) closeAdmissions(ctx context.Context) error {
	s.mu.Lock()
	servers := make([]*InterfaceServer, 0, len(s.servers))
	for _, server := range s.servers {
		servers = append(servers, server)
	}
	s.mu.Unlock()
	for _, server := range servers {
		for _, admission := range server.sessions.admissions() {
			owner := server.sessions.admissionControl(admission.session)
			session := server.sessions.markTeardown(admission.session)
			if session != nil {
				if admission.publication != PADSNotAttempted || owner.transferred {
					server.finishSessionDown(session)
				} else {
					server.closeAdmitted(session)
				}
			}
			if err := s.finishStoppedAdmission(server, admission.session); err != nil {
				return err
			}
			s.retryRefusedAdmission(server, admission.session)
			// A stop observer can finish an admission for which Driver emitted
			// no terminal event. Complete only that same retained row now.
			server.handleSessionDown(admission.session)
			if err := s.waitAdmission(ctx, server, admission.session); err != nil {
				return err
			}
		}
	}
	return nil
}

var _ callsink.RelayHandoff = (*serviceBridgePreparation)(nil)

// failAdmissionClose keeps every exact resource root and makes the same owner
// retryable. A concurrent drain learns the actual failure without polling.
func (st *SessionTable) failAdmissionClose(session *Session, err error) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.sessions[session.SID] != session {
		return
	}
	session.closeStarted = false
	session.closeErr = err
	select {
	case session.closeNotice <- struct{}{}:
	default:
	}
}

// AdmissionCleanupStatus describes the current producer cleanup attempt.
// Err is the latest actual failure and stays visible during a retry.
// Neither nil Err nor !InProgress confirms resource deletion.
type AdmissionCleanupStatus struct {
	InProgress bool
	Err        error
}

// CleanupStatus snapshots only this exact current row, under the table lock.
// A false match never grants authority over a replacement or proves release.
func (st *SessionTable) CleanupStatus(expected *Session) (AdmissionCleanupStatus, bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if expected == nil || st.sessions[expected.SID] != expected {
		return AdmissionCleanupStatus{}, false
	}
	return AdmissionCleanupStatus{InProgress: expected.closeStarted, Err: expected.closeErr}, true
}
