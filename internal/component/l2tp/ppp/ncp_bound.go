// Design: docs/research/l2tpv2-ze-integration.md -- the no-NCP bound
// Related: ncp.go -- runNCPPhase arms the bound and fails the phase at its expiry
// Related: ncp_restart.go -- applyNCPTransition, the one NCP state edge
// Related: session_run.go -- run closes LCP at the bound's expiry after session-up
// RFC: rfc/short/rfc1661.md -- Section 3.7

package ppp

import (
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The no-NCP bound limits how long a session holds LCP Opened while no
// enabled NCP is Opened. RFC 1661 Section 3.7 forbids ending the link for one
// NCP closing alone: "Conversely, the fact that one NCP has Closed is not
// sufficient reason to cause the termination of the PPP link, even if that
// NCP was the only NCP currently in the Opened state." The same section names
// the timer that may end it: "This might happen because of the loss of
// carrier, authentication failure, link quality failure, the expiration of an
// idle-period timer, or the administrative closing of the link."
//
// The bound is armed when the session enters "no enabled NCP is Opened": at
// the start of the NCP phase (runNCPPhase), or when the last Opened NCP
// leaves Opened for any reason (setNCPState). While no enabled NCP is Opened
// nothing stops or resets it, whatever the families do, so a peer that keeps
// one family negotiating with Configure-Naks cannot extend it. An NCP reaching
// Opened cancels it, and so does LCP leaving Opened (resetNCP); the next NCP
// phase arms a new one. Its expiry in the NCP phase fails the session
// (runNCPPhase, NAS Error); after session-up it closes the link
// (closeIdleLink, Idle Timeout). A disabled NCP never leaves Initial, so it
// never counts as Opened: the bound counts enabled NCPs only.

// noNCPTimeout is the length of the no-NCP bound: the session's configured
// ipTimeout (`ncp timeout` on L2TP), or defaultIPTimeout where the transport
// configures none (PPPoE).
func (s *pppSession) noNCPTimeout() time.Duration {
	if s.ipTimeout <= 0 {
		return defaultIPTimeout
	}
	return s.ipTimeout
}

// anyNCPOpened reports whether IPCP or IPv6CP is Opened.
func (s *pppSession) anyNCPOpened() bool {
	if s.ipcpState == LCPStateOpened {
		return true
	}
	return s.ipv6cpState == LCPStateOpened
}

// armNoNCPBound starts the bound for one noNCPTimeout from now. Only the start
// of the NCP phase and the last Opened NCP leaving Opened call it, so nothing
// re-arms a running bound. The timer is created on the first arm; Go 1.23 and
// later make Reset drop an unreceived expiry of the previous arm.
func (s *pppSession) armNoNCPBound() {
	if s.noNCPBound == nil {
		s.noNCPBound = time.NewTimer(s.noNCPTimeout())
		return
	}
	s.noNCPBound.Reset(s.noNCPTimeout())
}

// cancelNoNCPBound stops the bound. Safe before the first arm.
func (s *pppSession) cancelNoNCPBound() {
	if s.noNCPBound != nil {
		s.noNCPBound.Stop()
	}
}

// noNCPBoundChannel is the bound's expiry. A nil channel disables the select
// arm until the first NCP phase arms the bound.
func (s *pppSession) noNCPBoundChannel() <-chan time.Time {
	if s.noNCPBound == nil {
		return nil
	}
	return s.noNCPBound.C
}

// trackNoNCPBound follows the bound across one NCP state edge. wasOpened is
// anyNCPOpened before the edge. setNCPState MUST call it after every write, so
// the bound follows each transition, including those handled inside
// authentication or a nested NCP phase before the run loop regains control.
func (s *pppSession) trackNoNCPBound(wasOpened bool) {
	opened := s.anyNCPOpened()
	if opened == wasOpened {
		return
	}
	if opened {
		s.cancelNoNCPBound()
		return
	}
	// RFC 1661 Section 3.7
	s.armNoNCPBound()
}

// closeIdleLink ends a session that came up and then held no enabled NCP
// Opened for one no-NCP bound: it drives the LCP Close event, whose tld
// withdraws the network and whose str sends the LCP Terminate-Request. The
// Closed that follows (the peer's Terminate-Ack, or TO- after Max-Terminate)
// reports Idle Timeout (applyTransition reads noNCPBoundExpired). It reports
// whether the session ended at once. run MUST call it only for the bound's
// expiry outside the NCP phase.
//
// RFC 1661 Section 3.7: "PPP can terminate the link at any time.  This might
// happen because of the loss of carrier, authentication failure, link
// quality failure, the expiration of an idle-period timer, or the
// administrative closing of the link."
func (s *pppSession) closeIdleLink() bool {
	s.noNCPBoundExpired = true
	s.logger.Info("ppp: no NCP opened within the no-NCP bound, closing the link",
		"timeout", s.noNCPTimeout().String())
	cur := s.currentState()
	// RFC 1661 Section 4.1: drive Close through the shared FSM actions.
	return s.applyTransition(cur, LCPDoTransition(cur, LCPEventClose), LCPPacket{})
}

// idleCloseReason is the EventSessionDown reason of the Closed closeIdleLink
// leads to.
func (s *pppSession) idleCloseReason() string {
	var tb textbuf.Buffer
	return tb.Str("LCP closed: no NCP opened within ").Str(s.noNCPTimeout().String()).String()
}
