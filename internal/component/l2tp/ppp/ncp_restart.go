// Design: docs/research/l2tpv2-ze-integration.md -- independent NCP Restart ownership
// Related: ncp.go -- NCP frame dispatch and initial convergence
// Related: session_run.go -- active-session timer dispatch
// RFC: rfc/short/rfc1661.md -- Sections 4.1, 4.3, 4.4 and 4.6

package ppp

import "time"

// ncpRestart holds one family's timer and bounded transmission budget. Only
// the session goroutine accesses it. run MUST stop both timers at session exit;
// resetNCP MUST stop both before another LCP lifetime starts.
type ncpRestart struct {
	timer *time.Timer
	count int
}

func (s *pppSession) ncpRestart(family AddressFamily) *ncpRestart {
	switch family {
	case AddressFamilyIPv4:
		return &s.ipcpRestart
	case AddressFamilyIPv6:
		return &s.ipv6cpRestart
	default:
		panic("BUG: unknown NCP address family")
	}
}

// A nil channel disables a select arm until that family first starts a timer.
func (s *pppSession) ncpRestartChannel(family AddressFamily) <-chan time.Time {
	restart := s.ncpRestart(family)
	if restart.timer == nil {
		return nil
	}
	return restart.timer.C
}

func (s *pppSession) armNCPRestartTimer(family AddressFamily) {
	restart := s.ncpRestart(family)
	if restart.timer == nil {
		restart.timer = time.NewTimer(s.lcpConfig.RestartTimer())
		return
	}
	restart.timer.Reset(s.lcpConfig.RestartTimer())
}

func (s *pppSession) stopNCPRestartTimer(family AddressFamily) {
	if timer := s.ncpRestart(family).timer; timer != nil {
		timer.Stop()
	}
}

func (s *pppSession) stopNCPRestartTimers() {
	s.stopNCPRestartTimer(AddressFamilyIPv4)
	s.stopNCPRestartTimer(AddressFamilyIPv6)
}

// RFC 1661 Section 4.4: "The counter is decremented for each transmission,
// including the first." Only SCR and STR spend the initialized budget.
func (s *pppSession) spendNCPRestartCount(family AddressFamily) {
	restart := s.ncpRestart(family)
	if restart.count > 0 {
		restart.count--
	}
	s.armNCPRestartTimer(family)
}

// applyNCPTransition shares the existing FSM between packet and timeout
// events. A family reaching its terminal state never signals PPP layer-down.
func (s *pppSession) applyNCPTransition(family AddressFamily, current LCPState, transition lCPTransition, packet LCPPacket) bool {
	restartMax := s.lcpConfig.MaxConfigure()
	for _, action := range transition.Actions {
		if action == LCPActSTR {
			restartMax = s.lcpConfig.MaxTerminate()
		}
	}
	for _, action := range transition.Actions {
		if action == LCPActIRC {
			s.ncpRestart(family).count = restartMax
			continue
		}
		if !s.performNCPAction(family, action, packet) {
			return true
		}
	}
	s.setNCPState(family, transition.NewState)
	if !restartTimerRuns(transition.NewState) {
		s.stopNCPRestartTimer(family)
	}
	if transition.NewState == LCPStateOpened {
		if current != LCPStateOpened {
			return !s.onNCPOpened(family)
		}
	}
	return false
}

func (s *pppSession) handleNCPRestartTimeout(family AddressFamily) bool {
	current := s.ncpState(family)
	if !restartTimerRuns(current) {
		s.stopNCPRestartTimer(family)
		return false
	}
	event := LCPEventTOMinus
	if s.ncpRestart(family).count > 0 {
		event = LCPEventTOPlus
	}
	return s.applyNCPTransition(family, current, LCPDoTransition(current, event), LCPPacket{})
}

// The initial convergence deadline bounds negotiation, not termination grace.
// Once only Closed/Stopped/Opened or closing families remain, each outstanding
// Restart timer supplies its own bound; a normal close cannot kill the link.
func (s *pppSession) ncpInitialNegotiationPending(ipcpEnabled, ipv6cpEnabled bool) bool {
	if ipcpEnabled {
		if ncpNegotiationPending(s.ipcpState) {
			return true
		}
	}
	if ipv6cpEnabled {
		return ncpNegotiationPending(s.ipv6cpState)
	}
	return false
}

func ncpNegotiationPending(state LCPState) bool {
	// Projection of states still awaiting initial Configure negotiation.
	//exhaustive:ignore
	switch state {
	case LCPStateInitial, LCPStateStarting, LCPStateReqSent, LCPStateAckRcvd, LCPStateAckSent:
		return true
	case LCPStateClosed, LCPStateStopped, LCPStateClosing, LCPStateStopping, LCPStateOpened:
		return false
	}
	return false
}
