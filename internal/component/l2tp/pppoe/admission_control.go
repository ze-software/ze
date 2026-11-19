// Design: docs/architecture/l2tp/subscriber-session-model.md -- exact PPPoE admission binding
// Related: server.go -- attempts are minted before the PPP lease transfers

package pppoe

import (
	"context"
	"errors"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// sessionForAttempt selects the original transport admission. The canonical ID
// is opaque; wire TunnelID is never used as an access-interface index.
func (s *Subsystem) sessionForAttempt(attempt ppp.SessionAttempt, sid uint16) (*InterfaceServer, *Session, bool) {
	id := attempt.SubscriberID()
	if id == "" {
		return nil, nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, server := range s.servers {
		server.sessions.mu.Lock()
		session := server.sessions.sessions[sid]
		match := session != nil && session.SubscriberID == id && session.Attempt == attempt
		server.sessions.mu.Unlock()
		if match {
			return server, session, true
		}
	}
	return nil, nil, false
}

// sessionForControl adopts only the original worker bound to this admission.
// Final lifecycle events can carry a retired control, so liveness is checked
// separately by consumers that publish a live network state.
func (s *Subsystem) sessionForControl(control ppp.SessionControl, sid uint16) (*InterfaceServer, *Session, bool) {
	server, session, found := s.sessionForAttempt(control.Attempt(), sid)
	if !found {
		return nil, nil, false
	}
	st := server.sessions
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.sessions[sid] != session || session.SubscriberID != control.SubscriberID() {
		return nil, nil, false
	}
	bound, admitted := session.Attempt.Control()
	if !admitted || bound != control {
		return nil, nil, false
	}
	if session.Control != (ppp.SessionControl{}) && session.Control != control {
		return nil, nil, false
	}
	session.Control = control
	return server, session, true
}

type pppAdmission struct {
	opened        dataplane.OpenedSession
	attempt       ppp.SessionAttempt
	transferred   bool
	terminal      bool
	relayReleased bool
	notice        <-chan struct{}
}

type pppTerminalState uint8

const (
	pppTerminalPending pppTerminalState = iota
	pppTerminalDelivering
	pppTerminalDelivered
)

// admissionControl captures this owner's resources under the table lock.
func (st *SessionTable) admissionControl(expected *Session) pppAdmission {
	st.mu.Lock()
	defer st.mu.Unlock()
	if expected == nil || st.sessions[expected.SID] != expected {
		return pppAdmission{}
	}
	return pppAdmission{opened: expected.Opened, attempt: expected.Attempt, transferred: expected.pppTransferred, terminal: expected.pppTerminal == pppTerminalDelivered, relayReleased: expected.pppRelayReleased, notice: expected.pppNotice}
}

// stopPPPAdmission signals only this attempt and its once-bound worker.
// A queued attempt stays canceled even when no control has been delivered yet.
// The caller gates only its captured physical transport; the terminal consumer
// owns completion. Cancellation never joins a worker on the discovery reader.
func (s *InterfaceServer) stopPPPAdmission(attempt ppp.SessionAttempt) {
	attempt.Cancel()
}

func pendingAuthFor(control ppp.SessionControl) pendingAuthKey {
	return pendingAuthKey{subscriberID: control.SubscriberID(), attempt: control.Attempt()}
}

// liveAdmission fences synchronous consumer work before success acknowledgment.
func (s *Subsystem) liveAdmission(control ppp.SessionControl, sid uint16) bool {
	server, session, found := s.sessionForControl(control, sid)
	if !found {
		return false
	}
	st := server.sessions
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.sessions[sid] == session && session.State != StateTeardown && control.Active()
}

// publishPPPTerminal claims the exact original while shared Down is delivered.
// Close can still gate the wire, but MUST keep this SID until publication
// returns. The callback runs outside the table lock. Concurrent claims return
// immediately, and a callback panic leaves this same admission retryable.
func (st *SessionTable) publishPPPTerminal(expected *Session, attempt ppp.SessionAttempt, publish func()) bool {
	st.mu.Lock()
	if expected == nil || st.sessions[expected.SID] != expected || expected.Attempt != attempt ||
		expected.closeStarted || expected.pppTerminal != pppTerminalPending {
		st.mu.Unlock()
		return false
	}
	expected.pppTerminal = pppTerminalDelivering
	st.mu.Unlock()
	delivered := false
	defer func() { st.finishPPPTerminal(expected, delivered) }()
	publish()
	delivered = true
	return true
}

func (st *SessionTable) finishPPPTerminal(expected *Session, delivered bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if !delivered {
		expected.pppTerminal = pppTerminalPending
		return
	}
	expected.pppTerminal = pppTerminalDelivered
	if expected.pppNotice != nil {
		select {
		case <-expected.pppNotice:
		default:
			close(expected.pppNotice)
		}
	}
	expected.pppRefusal = nil
}

// finishRejectedAdmission does not confuse a missing Control with deletion.
// A refused handoff may own a cleanup-only lease, or may have acquired none.
func (s *Subsystem) finishRejectedAdmission(server *InterfaceServer, session *Session, event ppp.EventSessionRejected) {
	status, err := event.Attempt.CleanupStatus()
	if err != nil || status.Admitted || !status.Finished || status.LeaseAcquired && !status.Released || status.Err != nil {
		server.sessions.mu.Lock()
		if !status.Admitted && server.sessions.sessions[session.SID] == session && session.Attempt == event.Attempt {
			captured := event
			session.pppRefusal = &captured
			if session.pppNotice != nil {
				select {
				case <-session.pppNotice:
				default:
					close(session.pppNotice)
				}
			}
		}
		server.sessions.mu.Unlock()
		s.logger.Warn("pppoe: refused PPP admission cleanup is not confirmed",
			"code", diagnostic.CodeDoctorPPPCleanup, "fix", ppp.CleanupFailureFix,
			"subscriber", event.Attempt.SubscriberID(), "status", status, "error", err)
		return
	}
	if !server.sessions.publishPPPTerminal(session, event.Attempt, func() {
		s.pendingAuth.Delete(pendingAuthKey{subscriberID: session.SubscriberID, attempt: session.Attempt})
		// A refused admission never published an assigned network.
		s.withdrawSubscriberIdentity(session, l2tpevents.NetworkLifetime{}, event.Reason)
	}) {
		return
	}
	server.handleSessionDown(session)
}

// retryRefusedAdmission re-observes only a retained real refusal after a cleanup
// retry. The terminal flag excludes duplicate shared withdrawal under table.mu.
func (s *Subsystem) retryRefusedAdmission(server *InterfaceServer, session *Session) {
	server.sessions.mu.Lock()
	var event *ppp.EventSessionRejected
	if server.sessions.sessions[session.SID] == session {
		event = session.pppRefusal
	}
	server.sessions.mu.Unlock()
	if event != nil {
		s.finishRejectedAdmission(server, session, *event)
	}
}

// waitAdmission holds no subsystem/table/discovery lock. An observed refused
// cleanup failure returns promptly while its exact row/server stay reachable.
func (s *Subsystem) waitAdmission(ctx context.Context, server *InterfaceServer, session *Session) error {
	notice := server.sessions.admissionControl(session).notice
	for {
		cleanup, current := server.sessions.CleanupStatus(session)
		if !current {
			select {
			case <-session.closeDone:
				return nil
			default:
				return errors.New("pppoe: draining admission owner changed before completion")
			}
		}
		if cleanup.Err != nil && !cleanup.InProgress {
			return cleanup.Err
		}
		select {
		case <-session.closeNotice:
			continue
		case <-session.closeDone:
			return nil
		case <-notice:
			server.sessions.mu.Lock()
			event := session.pppRefusal
			server.sessions.mu.Unlock()
			if event != nil {
				status, err := event.Attempt.CleanupStatus()
				if err != nil {
					return err
				}
				if status.Err != nil {
					return status.Err
				}
				if !status.Finished || status.Admitted || status.LeaseAcquired && !status.Released {
					return errors.New("pppoe: refused admission cleanup remains unconfirmed")
				}
				s.retryRefusedAdmission(server, session)
			}
			notice = nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// finishStoppedAdmission runs only after Driver.Stop and its event/drain readers
// joined. Stop may suppress terminal emission; observe the exact original
// attempt instead of waiting for a notice that can no longer be delivered.
func (s *Subsystem) finishStoppedAdmission(server *InterfaceServer, session *Session) error {
	owner := server.sessions.admissionControl(session)
	if !owner.transferred || owner.terminal || owner.relayReleased {
		return nil
	}
	status, err := owner.attempt.CleanupStatus()
	if err != nil {
		return err
	}
	if status.Err != nil {
		return status.Err
	}
	if !status.Finished {
		return errors.New("pppoe: stopped PPP admission has not finished")
	}
	if status.Admitted && !status.LeaseAcquired {
		return errors.New("pppoe: stopped admitted worker has no lease acquisition receipt")
	}
	if status.LeaseAcquired && !status.Released {
		return errors.New("pppoe: stopped PPP admission release remains unconfirmed")
	}
	server.sessions.publishPPPTerminal(session, owner.attempt, func() {
		s.pendingAuth.Delete(pendingAuthKey{subscriberID: session.SubscriberID, attempt: owner.attempt})
		// The joined worker delivered no terminal epoch. Discovery is stopped
		// and the captured original admission still owns its subscriber ID.
		s.withdrawSubscriberIdentity(session, l2tpevents.NetworkLifetime{}, "PPP driver stopped before terminal delivery")
	})
	return nil
}
