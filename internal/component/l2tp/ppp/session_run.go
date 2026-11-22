// Design: docs/research/l2tpv2-ze-integration.md -- per-session goroutine main loop
// RFC: rfc/short/rfc1661.md -- RFC 1661 Sections 4.1, 4.3, 4.4, 4.6, 5.7
// Related: session.go -- pppSession struct
// Related: manager.go -- Driver that spawns these goroutines
// Related: ppp_fsm.go -- pure FSM transition function
// Related: ncp.go -- NCP phase driver invoked from afterLCPOpen

package ppp

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// Defaults applied per session when StartSession leaves a field zero.
const (
	defaultEchoInterval  = 10 * time.Second
	defaultEchoFailures  = 3
	defaultAuthTimeout   = 30 * time.Second
	magicDrawMaxAttempts = 8
)

// frameBufPool supplies MaxFrameBufLen-byte buffers used for both reads
// from the chan fd and wire-format writes. Eliminates per-packet
// allocation in the hot path. The read side needs the full size: /dev/ppp
// fails a read whose buffer is shorter than the frame, and RFC 1661
// Section 6.1 obliges ze to receive a 1500-octet Information field
// behind its Protocol field whatever MRU it asked for.
var frameBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, MaxFrameBufLen)
		return &b
	},
}

func getFrameBuf() []byte {
	p, ok := frameBufPool.Get().(*[]byte)
	if !ok {
		panic("BUG: frameBufPool produced non-*[]byte")
	}
	return (*p)[:MaxFrameBufLen]
}

func putFrameBuf(b []byte) {
	if cap(b) < MaxFrameBufLen {
		return
	}
	full := b[:MaxFrameBufLen]
	frameBufPool.Put(&full)
}

// drainFramesToPool returns any frame buffers still queued in the
// channel back to frameBufPool. Called via defer on session exit so
// readFrames-produced buffers don't leak. Reads everything currently
// buffered (channel is buffered size 4) and returns. It drains
// readUnitFrames' channel the same way.
func drainFramesToPool(frames <-chan []byte) {
	for n := len(frames); n > 0; n-- {
		frame, ok := <-frames
		if !ok {
			return
		}
		putFrameBuf(frame)
	}
}

// run is the per-session goroutine main loop. Owns chanFile until
// exit. On any error, stop signal, or fatal FSM transition, emits
// EventSessionDown (unless already emitted) and returns.
//
// Uses a SINGLE readFrames goroutine for the session's lifetime to
// avoid the double-reader race between negotiation and opened
// phases, and a SINGLE readUnitFrames goroutine for the unit, which
// carries the NCP frames once the channel is connected. The main loop
// gates the convergence timer at actual LCP transitions and enables the echo
// ticker only after authentication and NCP complete.
func (s *pppSession) run(start *StartSession) {
	// Teardown order must be: close readerStop (ends either reader waiting
	// for a free queue slot, or readUnitFrames waiting for a connect) -> close both files
	// (unblocks a test pipe's read; the kernel ends both reads when the
	// channel unregisters) -> wait for both readers to exit -> drain
	// leftover frames back to the pool. A single defer guarantees the
	// sequence; multiple defers would fire LIFO in a way that lets a
	// reader keep producing frames after the drain already ran.
	var (
		framesCh     chan []byte
		unitFramesCh chan []byte
		readerWG     sync.WaitGroup
	)
	readerStop := make(chan struct{})
	defer func() {
		// A released session (Driver.ReleaseSession) reports no teardown:
		// the transport still carries the subscriber.
		if !s.sessionDownSent && !s.released.Load() {
			select {
			case <-s.sessStop:
				s.sendEvent(EventSessionDown{
					TunnelID:  s.tunnelID,
					SessionID: s.sessionID,
					Reason:    "session stopped",
					Cause:     s.terminateCause(l2tpevents.TerminateCauseNASRequest),
				})
			default:
			}
		}
		close(readerStop)
		// The shared close gate also ends an active network when a global
		// stop interrupts periodic authentication without publishing Down.
		// Final writers, sample and withdrawals MUST finish in that gate;
		// an independent Stop close cannot overtake any of them.
		s.closeFiles()
		readerWG.Wait()
		if framesCh != nil {
			drainFramesToPool(framesCh)
		}
		if unitFramesCh != nil {
			drainFramesToPool(unitFramesCh)
		}
	}()

	defer s.stopNCPRestartTimers()
	defer s.cancelNoNCPBound()
	mag, err := generateMagic()
	if err != nil {
		var tb textbuf.Buffer
		s.fail(tb.Str("magic-rand: ").Err(err).String())
		return
	}
	s.magic = mag

	// Start the single reader goroutine BEFORE LCP / auth so both
	// phases consume frames from the same queue. Running auth reads
	// directly on chanFile while readFrames is also reading would
	// race on the underlying fd; routing every read through framesIn
	// keeps the contract "one reader per chanFile" intact.
	frames := make(chan []byte, 4)
	readDone := make(chan error, 1)
	framesCh = frames
	s.framesIn = frames
	readerWG.Go(func() {
		s.readFrames(frames, readDone, readerStop)
	})

	// The unit reader starts here too, and sleeps until afterLCPOpen
	// connects the channel: before that the unit has no channel and a read
	// answers EOF at once (ppp_read, "Return 0 (EOF) on an interface that
	// has no channels connected").
	var unitReadDone chan error
	if s.unitFile != nil {
		unitFramesCh = make(chan []byte, 4)
		unitReadDone = make(chan error, 1)
		s.unitFramesIn = unitFramesCh
		s.unitConnected = make(chan struct{}, 1)
		s.unitIdle = make(chan struct{}, 1)
		s.unitGone = make(chan struct{})
		readerWG.Go(func() {
			s.readUnitFrames(unitFramesCh, unitReadDone, readerStop)
		})
	}

	// RFC 2661 Section 4.4.5: accept the LAC's proxy LCP, or renegotiate LCP
	// under the operator's `proxy-lcp renegotiate` policy.
	proxy, isProxy, outcome := s.admitProxyLCP(start)
	s.proxyLCPOutcome = outcome

	// RFC 1661 Section 4.6: the Restart timer times transmissions of
	// Configure-Request and Terminate-Request, and "Expiration of the
	// Restart timer causes a Timeout event". It is armed by the scr, str
	// and zrc actions (performAction) and stopped whenever the automaton
	// leaves a state that runs one, so it is created stopped and lives on
	// the session rather than in this frame. It is created before the
	// synthetic Open below, whose scr is the first transmission the timer
	// has to guard. The proxy path starts Opened, which runs no Restart
	// timer, and a later Terminate-Request from the peer arms it through
	// zrc like any other session.
	s.restartTimer = time.NewTimer(s.lcpConfig.RestartTimer())
	s.restartTimer.Stop()
	defer s.restartTimer.Stop()
	// The actual FSM edges own this separate bound. Authentication can see
	// Opened and restart LCP before the outer loop observes either transition.
	convergenceLimit := 2 * time.Duration(s.lcpConfig.MaxConfigure()) * s.lcpConfig.RestartTimer()
	s.lcpConvergenceTimer = time.NewTimer(convergenceLimit)
	s.lcpConvergenceTimer.Stop()
	defer s.lcpConvergenceTimer.Stop()

	if isProxy {
		// RFC 1661 Section 4.4: "This action sets the Restart counter to the
		// appropriate value (Max-Terminate or Max-Configure)."
		// Accepted proxy LCP represents an exchange completed by the LAC.
		// Match the counter left by RCA's IRC so an Opened RCR can spend the
		// configured budget without adding IRC to that edge or arming its timer.
		s.restartCount = s.lcpConfig.MaxConfigure()
		negotiatedMethod := authMethodFromAuthProto(proxy.AuthProto, proxy.AuthData)
		s.initialAuthMethod = s.configuredAuthMethod
		s.authNegotiationStarted = true
		s.lcpProxied = true
		s.logger.Info("ppp: proxy LCP short-circuit",
			"mru", proxy.MRU,
			"auth-proto", strconv.Itoa(int(proxy.AuthProto)),
			"auth-method", negotiatedMethod.String())
		s.mu.Lock()
		s.negotiatedMRU = proxy.MRU
		s.negotiatedAuthMethod = negotiatedMethod
		s.state = LCPStateOpened
		s.mu.Unlock()
		// The LAC negotiated the Magic-Numbers on ze's behalf: the peer
		// acknowledged the LAC's, and the LAC acknowledged the peer's.
		// Without a LAC value the option was not negotiated, and RFC 1661
		// Section 5.8 then has ze transmit zero.
		s.peerMagic = proxy.PeerMagic
		if proxy.LocalMagic != 0 {
			s.magic = proxy.LocalMagic
			s.magicNegotiated = true
		}
		s.sendEvent(EventLCPUp{
			TunnelID:             s.tunnelID,
			SessionID:            s.sessionID,
			NegotiatedMRU:        proxy.MRU,
			NegotiatedAuthMethod: negotiatedMethod,
			ProxyLCP:             s.proxyLCPOutcome,
		})
		if !s.afterLCPOpen() {
			state := s.currentState()
			if state == LCPStateOpened || s.sessionEndedAt(state) {
				return
			}
		}
	} else {
		// Synthetic Up: Initial -> Closed.
		trUp := LCPDoTransition(LCPStateInitial, LCPEventUp)
		s.mu.Lock()
		s.state = trUp.NewState
		s.mu.Unlock()

		// Synthetic Open: Closed -> ReqSent with [IRC, SCR]. It runs through
		// applyTransition so the Restart counter and the Restart timer are
		// loaded and armed here exactly as they are for a peer-driven event.
		trOpen := LCPDoTransition(LCPStateClosed, LCPEventOpen)
		if s.applyTransition(LCPStateClosed, trOpen, LCPPacket{}) {
			return
		}
	}
	opened := s.currentState() == LCPStateOpened

	// Echo ticker. Enabled after Opened. In the proxy path we are
	// already Opened, so enable immediately.
	echoInterval := s.echoInterval
	// Clamp non-positive intervals (zero = "use default"; negative
	// would panic time.NewTicker).
	if echoInterval <= 0 {
		echoInterval = defaultEchoInterval
	}
	echoMax := s.echoFailures
	if echoMax == 0 {
		echoMax = defaultEchoFailures
	}
	echoTicker := time.NewTicker(echoInterval)
	defer echoTicker.Stop()
	var echoTickerC <-chan time.Time
	if opened {
		echoTickerC = echoTicker.C
	} else {
		echoTicker.Stop()
	}

	// Periodic CHAP re-authentication (spec-l2tp-6b-auth AC-14, Phase 9).
	// Enabled only when reauthInterval > 0 AND the negotiated method
	// supports authenticator-initiated challenges (CHAP-MD5 / MS-CHAPv2).
	// PAP is peer-initiated per RFC 1334, so ze cannot force a fresh
	// exchange -- reauth for PAP is silently skipped.
	//
	// Started alongside echoTickerC: after initial auth succeeds and
	// LCP reaches Opened, both tickers are live on the main select.
	// On tick, the main loop calls the same runXAuthPhase handler used
	// for initial auth (drawing a fresh Challenge value + incrementing
	// chapIdentifier, so every round carries a new Identifier per
	// RFC 1994 §4.1).
	var (
		reauthTicker  *time.Ticker
		reauthTickerC <-chan time.Time
	)
	startReauthTicker := func() {
		if s.reauthInterval <= 0 {
			return
		}
		s.mu.Lock()
		method := s.negotiatedAuthMethod
		s.mu.Unlock()
		if method != AuthMethodCHAPMD5 && method != AuthMethodMSCHAPv2 {
			return
		}
		if reauthTicker == nil {
			reauthTicker = time.NewTicker(s.reauthInterval)
		} else {
			reauthTicker.Reset(s.reauthInterval)
		}
		reauthTickerC = reauthTicker.C
	}
	defer func() {
		if reauthTicker != nil {
			reauthTicker.Stop()
		}
	}()
	if opened {
		startReauthTicker()
	}

	for {
		select {
		case <-s.stopCh:
			// The PPP driver was stopped on purpose, so RFC 2866 Section 5.10
			// value 10 applies: "NAS ended session for a non-error reason not
			// otherwise listed here."
			s.sendEvent(EventSessionDown{
				TunnelID: s.tunnelID, SessionID: s.sessionID,
				Reason: "driver stopped",
				Cause:  l2tpevents.TerminateCauseNASRequest,
			})
			return

		case <-s.sessStop:
			// Exact local cancellation uses the existing NAS-request cause.
			// The normal final cleanup still owns descriptor Close; signaling
			// cancellation never closes borrowed I/O ahead of terminal work.
			s.sendEvent(EventSessionDown{
				TunnelID: s.tunnelID, SessionID: s.sessionID,
				Reason: "session stopped", Cause: l2tpevents.TerminateCauseNASRequest,
			})
			return

		case err := <-readDone:
			reason := "chan fd closed"
			if err != nil && !errors.Is(err, io.EOF) {
				var tb textbuf.Buffer
				reason = tb.Str("chan fd read error: ").Err(err).String()
			}
			// The channel fd to the kernel closed or failed to read. Ze
			// cannot tell a local teardown from a peer-side loss here, and
			// RFC 2866 Section 5.10 value 9 is the honest answer for an
			// ending the NAS detected and cannot attribute further.
			s.sendEvent(EventSessionDown{
				TunnelID: s.tunnelID, SessionID: s.sessionID,
				Reason: reason,
				Cause:  s.terminateCause(l2tpevents.TerminateCauseNASError),
			})
			return

		case err := <-unitReadDone:
			// The unit descriptor failed to read. The NCP frames the kernel
			// queues there would go unanswered from now on, so the session
			// ends rather than carry on deaf to IPCP and IPv6CP. RFC 2866
			// Section 5.10 value 9, as for the channel above.
			var tb textbuf.Buffer
			s.sendEvent(EventSessionDown{
				TunnelID: s.tunnelID, SessionID: s.sessionID,
				Reason: tb.Str("unit fd read error: ").Err(err).String(),
				Cause:  s.terminateCause(l2tpevents.TerminateCauseNASError),
			})
			return

		case frame := <-s.unitFramesIn:
			term := s.handleFrame(frame)
			putFrameBuf(frame)
			if term {
				return
			}

		case <-s.lcpConvergenceTimer.C:
			var tb textbuf.Buffer
			s.fail(tb.Str("LCP convergence timeout after ").Str(convergenceLimit.String()).String())
			return

		case <-s.ncpRestartChannel(AddressFamilyIPv4):
			if s.handleNCPRestartTimeout(AddressFamilyIPv4) {
				return
			}
		case <-s.ncpRestartChannel(AddressFamilyIPv6):
			if s.handleNCPRestartTimeout(AddressFamilyIPv6) {
				return
			}
		case <-s.noNCPBoundChannel():
			// Outside the NCP phase the bound runs only after session-up:
			// the phase cancels it by returning with an NCP Opened, and LCP
			// leaving Opened cancels it (resetNCP). RFC 1661 Section 3.7
			if s.closeIdleLink() {
				return
			}
		case <-s.restartTimer.C:
			// RFC 1661 Section 4.3, Timeout: "The TO+ event indicates that
			// the Restart counter continues to be greater than zero, which
			// triggers the corresponding Configure-Request or
			// Terminate-Request packet to be retransmitted. The TO- event
			// indicates that the Restart counter is not greater than zero,
			// and no more packets need to be retransmitted."
			if s.handleRestartTimeout() {
				return
			}

		case <-echoTickerC:
			if s.echoRequestRejected {
				// RFC 1661 Section 4.3: "The implementation MUST stop
				// sending the offending packet type." The peer Code-Rejected
				// an Echo-Request (stopRejectedCode), so the probe stops,
				// and with it the echo-failure count: a peer that implements
				// no Echo-Request never answers one, so counting the missing
				// replies would end a working link. Liveness then rests on
				// the lower layer (the L2TP Hello, RFC 2661 Section 6.5, or
				// the PPPoE transport). The ticker is re-armed only at a
				// later Opened edge, where this arm stops it again.
				echoTicker.Stop()
				echoTickerC = nil
				continue
			}
			s.mu.Lock()
			s.echoOutstanding++
			out := s.echoOutstanding
			s.mu.Unlock()
			if out > echoMax {
				// The peer stopped answering LCP echo probes, which is how a
				// virtual link reports the carrier RFC 2866 Section 5.10
				// value 2 names: "DCD was dropped on the port."
				s.sendEvent(EventSessionDown{
					TunnelID: s.tunnelID, SessionID: s.sessionID,
					Reason: textbuf.StrIntStr("echo timeout: ", int64(out), " consecutive failures"),
					Cause:  l2tpevents.TerminateCauseLostCarrier,
				})
				return
			}
			if !s.sendEchoRequest() {
				return
			}

		case <-reauthTickerC:
			// spec-l2tp-6b-auth AC-14: fire a fresh CHAP Challenge
			// on interval. runAuthPhase dispatches on
			// negotiatedAuthMethod (CHAP-MD5 or MS-CHAPv2, guarded
			// by startReauthTicker), so it increments chapIdentifier
			// and draws a new challenge value per round. Blocking
			// call -- while re-auth is in flight, echo / frame
			// handling pauses; re-auth must complete within
			// s.authTimeout or the session fails closed.
			if !s.runAuthPhase() {
				state := s.currentState()
				// An LCP interruption resumes negotiation or its termination
				// grace timer, just as it does during initial authentication.
				if state == LCPStateOpened || s.sessionEndedAt(state) {
					return
				}
			}

		case frame, ok := <-frames:
			if !ok {
				// readFrames exited and closed the channel without
				// a companion error on readDone (defensive -- this
				// path should not be reached because readFrames
				// always sends an error before returning). Emit a
				// session-down event so the transport reconciles.
				s.sendEvent(EventSessionDown{
					TunnelID: s.tunnelID, SessionID: s.sessionID,
					Reason: "chan fd closed (reader exited without error)",
					Cause:  l2tpevents.TerminateCauseNASError,
				})
				return
			}
			term := s.handleFrame(frame)
			putFrameBuf(frame)
			if term {
				return
			}
		}
		nowOpened := s.currentState() == LCPStateOpened
		if nowOpened == opened {
			continue
		}
		opened = nowOpened
		if !opened {
			// A transition handled inside authentication also reaches this
			// point. Neither ticker may run in the resumed LCP phase.
			echoTicker.Stop()
			echoTickerC = nil
			if reauthTicker != nil {
				reauthTicker.Stop()
			}
			reauthTickerC = nil
			continue
		}
		// afterLCPOpen has completed authentication and NCP. Reset the
		// tickers so a previous LCP lifetime cannot supply a pending tick.
		s.mu.Lock()
		s.echoOutstanding = 0
		s.mu.Unlock()
		echoTicker.Reset(echoInterval)
		echoTickerC = echoTicker.C
		startReauthTicker()
	}
}

// currentState reads s.state under the lock.
func (s *pppSession) currentState() LCPState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// generateMagic returns a non-zero uint32 sourced from crypto/rand.
// Caps retries at magicDrawMaxAttempts to bound a pathological RNG.
// RFC 1661 §6.4 requires non-zero; the zero draw probability is
// 1 / 2^32 per attempt.
func generateMagic() (uint32, error) {
	var b [4]byte
	for range magicDrawMaxAttempts {
		if _, err := rand.Read(b[:]); err != nil {
			return 0, err
		}
		v := binary.BigEndian.Uint32(b[:])
		if v != 0 {
			return v, nil
		}
	}
	return 0, errors.New(textbuf.StrIntStr("ppp: crypto/rand returned zero ", int64(magicDrawMaxAttempts), " times"))
}

// fail emits EventSessionDown with the reason and logs at warn.
// Caller MUST exit immediately after calling fail.
//
// Every failure routed here is one the NAS detected in its own processing: a
// negotiation timeout, an auth rejection, an NCP refusal, an ioctl error. RFC
// 2866 Section 5.10 value 9 covers all of them, and the event reports it as
// the terminate cause.
func (s *pppSession) fail(reason string) {
	s.logger.Warn("ppp: session failed", "reason", reason)
	s.sendEvent(EventSessionDown{
		TunnelID:  s.tunnelID,
		SessionID: s.sessionID,
		Reason:    reason,
		Cause:     l2tpevents.TerminateCauseNASError,
	})
}

// sendEvent writes an event to eventsOut, blocking until the consumer
// reads or the driver stops. The transport MUST drain events promptly
// or PPP processing will stall. Buffer size is defaultEventsOutBuf
// (64); sustained backlog beyond that blocks PPP progress.
//
// EventSessionDown is sent at most once per session: a session that signaled
// This-Layer-Finished keeps running until the lower layer's Down event, and
// that later exit (read error, driver stop) MUST NOT report a second teardown.
func (s *pppSession) sendEvent(ev Event) {
	ev = s.bindEventControl(ev)
	if _, ok := ev.(EventSessionDown); ok && s.sessionDownSent {
		return
	}
	// Only the two Down events end native work before publication; every
	// other event is published as it is.
	switch event := ev.(type) {
	case EventSessionDown:
		s.closeAuthAdmission()
		s.retireProviderRequests()
		if s.released.Load() {
			return
		}
		// The transport may publish PADT as soon as it receives this event.
		// Finish the last IPv6 advertisement before granting that permission.
		// A peer-initiated transport close has already gated the wire, so
		// its cleanup can only fail a write rather than send after PADT.
		ev = s.prepareSessionDown(event)
		// Release the canonical subscriber slot before publishing Down so a
		// replacement attempt admitted by a subscriber reacting to Down never
		// races against the retiring goroutine's reader drain.
		s.retireSubscriberKey()
	case EventLCPDown:
		ev = s.prepareLCPDown(event)
	}
	select {
	case s.eventsOut <- ev:
		if _, ok := ev.(EventSessionDown); ok {
			s.sessionDownSent = true
		}
	case <-s.stopCh:
	}
}

// retireSubscriberKey removes this session's canonical subscriber key from the
// driver table while keeping native dataplane ownership (d.owners) intact until
// physical release is confirmed.
func (s *pppSession) retireSubscriberKey() {
	if s.driver == nil {
		return
	}
	s.driver.mu.Lock()
	if s.driver.sessions[s.subscriberID] == s {
		delete(s.driver.sessions, s.subscriberID)
	}
	s.driver.mu.Unlock()
}

// prepareSessionDown ends native work before its caller publishes the event.
// The caller MUST publish only the returned value, after this method returns.
// Defer releases the gate even if an existing IPv6 provider panics, allowing
// the ordinary final cleanup defer to enter the same original owner's gate.
// This method deliberately does not recover that provider panic.
func (s *pppSession) prepareSessionDown(event EventSessionDown) EventSessionDown {
	s.nativeReleaseMu.Lock()
	defer s.nativeReleaseMu.Unlock()
	if err := s.stopIPv6Writers(); err != nil {
		s.reportIPv6ChildCleanupFailure(err)
		s.logger.Warn("ppp: IPv6 cleanup before session-down failed; resource removal is unconfirmed",
			"code", CodeDoctorPPPCleanup, "error", err, "fix", CleanupFailureFix)
	}
	event.NetworkLifetime, event.TerminalCounters = s.captureNetworkTerminal()
	if err := s.stopIPv6Service(); err != nil {
		s.reportIPv6ChildCleanupFailure(err)
		s.logger.Warn("ppp: IPv6 resource cleanup after terminal counters failed",
			"code", CodeDoctorPPPCleanup, "error", err, "fix", CleanupFailureFix)
	}
	return event
}

// prepareLCPDown confines the retained terminal measurement to the owner gate.
// The caller MUST publish the returned value only after this method returns.
func (s *pppSession) prepareLCPDown(event EventLCPDown) EventLCPDown {
	s.nativeReleaseMu.Lock()
	defer s.nativeReleaseMu.Unlock()
	event.NetworkLifetime, event.TerminalCounters = s.captureNetworkTerminal()
	return event
}

// closeFiles releases the session owner once. nativeReleaseMu serializes
// setup, terminal measurement and Close, preventing reuse during late work.
func (s *pppSession) closeFiles() {
	// The provisional outcome must not wait behind an in-flight native gate.
	s.closeAuthAdmission()
	s.nativeReleaseMu.Lock()
	defer s.nativeReleaseMu.Unlock()
	// The exact result is recorded/reported for the completion observers.
	_ = s.closeFilesLocked()
}

// Caller holds nativeReleaseMu, never Driver.mu.
func (s *pppSession) closeFilesLocked() error {
	s.closeAuthAdmission()
	s.mu.Lock()
	if s.filesClosed {
		s.mu.Unlock()
		if s.driver == nil {
			return nil
		}
		s.driver.mu.Lock()
		err, attempted := s.nativeReleaseErr, s.ownerCloseAttempted
		s.driver.mu.Unlock()
		if !attempted {
			return s.driver.closeSessionOwnerLocked(s)
		}
		return err
	}
	s.filesClosed = true
	s.channelConnected = false
	owner := s.dataplane
	s.mu.Unlock()
	if s.driver != nil {
		s.driver.mu.Lock()
		attempted, confirmed, cause := s.ownerCloseAttempted, s.ownerReleased, s.nativeReleaseErr
		s.driver.mu.Unlock()
		if attempted {
			// A guarded callback may have closed this exact IPv4-only owner
			// while the worker was publishing. Its handle may already refuse
			// native operations even when retained-unit release is uncertain.
			// Retry only that original Close; never issue a post-Close withdrawal.
			if confirmed {
				return cause
			}
			return s.driver.closeSessionOwnerLocked(s)
		}
	}
	if err := s.stopIPv6Writers(); err != nil {
		s.reportIPv6ChildCleanupFailure(err)
		s.logger.Warn("ppp: IPv6 writers before terminal counters failed",
			"code", CodeDoctorPPPCleanup, "error", err, "fix", CleanupFailureFix)
	}
	s.captureNetworkTerminal()
	if err := s.teardownNCPResources(); err != nil {
		ReportCleanupFailure(s.dataplane.Identity(), CleanupNetwork, err)
		s.logger.Warn("ppp: final network cleanup failed; resource removal is unconfirmed",
			"code", CodeDoctorPPPCleanup, "error", err, "fix", CleanupFailureFix)
	}
	if owner == nil {
		return nil
	}
	if s.driver != nil {
		s.driver.mu.Lock()
		attempted, confirmed, err := s.ownerCloseAttempted, s.ownerReleased, s.nativeReleaseErr
		s.driver.mu.Unlock()
		if attempted && confirmed {
			return err
		}
		return s.driver.closeSessionOwnerLocked(s)
	}
	// Standalone FSM fixtures have no admitted Driver or cleanup observer.
	if err := owner.Close(); err != nil {
		s.reportSessionCleanupFailure(owner, err)
		s.logger.Warn("ppp: session release failed; resource removal is unconfirmed",
			"code", CodeDoctorPPPCleanup, "error", err, "fix", CleanupFailureFix)
		return err
	}
	return nil
}

// stopIPv6Service joins service workers and finishes its final advertisement
// before the frame owner closes. Stop is idempotent and resource failures keep
// the service available for a truthful retry while the handle is still live.
// Callers MUST hold no session lock.
func (s *pppSession) stopIPv6Service() error {
	s.mu.Lock()
	service := s.ipv6Svc
	s.mu.Unlock()
	// refuse-by-default: guards pppSession.closeFiles -- no IPv6 service is valid before activation or after a successful stop; owner release must still proceed.
	if service == nil {
		s.setOwnerServiceState(false, nil)
		return nil
	}
	if err := service.Stop(); err != nil {
		s.setOwnerServiceState(true, err)
		return err
	}
	s.mu.Lock()
	if s.ipv6Svc == service {
		s.ipv6Svc = nil
	}
	s.mu.Unlock()
	s.setOwnerServiceState(false, nil)
	return nil
}

// setOwnerServiceState publishes only cleanup metadata, never native I/O.
// The worker owns actual service start/stop; callbacks may observe this under
// Driver.mu without acquiring worker.mu or joining its service goroutines.
func (s *pppSession) setOwnerServiceState(pending bool, cause error) {
	if s.driver == nil {
		return
	}
	s.driver.mu.Lock()
	s.ownerServicePending = pending
	s.ownerServiceErr = cause
	s.driver.mu.Unlock()
	if cause != nil {
		s.reportIPv6ChildCleanupFailure(cause)
	}
}

// Called while the worker owns its ordinary startup serialization. Admission
// cancellation is checked and service ownership published atomically with the
// callback's short-lock check. A callback requires cancellation before closing,
// so it cannot pass this check and race a later new service into a closed owner.
func (s *pppSession) beginOwnerService() bool {
	if s.driver == nil {
		return true
	}
	s.driver.mu.Lock()
	defer s.driver.mu.Unlock()
	select {
	case <-s.sessStop:
		return false
	case <-s.stopCh:
		return false
	default:
	}
	if s.ownerCloseAttempted {
		return false
	}
	s.ownerServicePending = true
	return true
}

// afterLCPOpenIPv6Service starts the RA sender and DHCPv6 server on
// ifname once IPv6CP has Opened. Callers MUST check s.ipv6cpState
// first; this helper decides only whether the peer's identifier may be
// acted on.
//
// It starts nothing when peerInterfaceIDNegotiated (session.go) is
// false. IPv6CP can reach Opened with peerInterfaceID still all-zero
// (the peer's Configure-Request carried no Interface-Identifier
// option), and starting the service on that value would derive fe80::
// from an identifier that is not the peer's
// (ai/rules/principles.md). An independent NCP's refusal must not fail
// the session, so both refusals here are logged and counted, never
// fatal.
//
// The one error it returns is a Framed-IPv6-Route the service could not
// install, or a profile over the framed-route limit: Ze never serves a
// subscriber a subset of the routes the server authorized, so the caller
// MUST fail the session with it after releasing nativeReleaseMu.
func (s *pppSession) afterLCPOpenIPv6Service() error {
	// Caller owns nativeReleaseMu, never the worker metadata mutex.
	s.mu.Lock()
	retained := s.ipv6Svc != nil
	supported := s.ipv6MRUSupported()
	negotiated := s.peerInterfaceIDNegotiated
	peerMRU, localMRU, cap := s.negotiatedMRU, s.localMRU, s.maxMRU
	localID, peerID := s.localInterfaceID, s.peerInterfaceID
	s.mu.Unlock()
	if retained {
		// A retained partial service still owns cleanup. Never replace it
		// with another service, or its address-removal receipt would be lost.
		reportIPv6Failure(s.logger, s.dataplane.Identity().InterfaceName, "service replacement",
			errors.New("the previous IPv6 service still owns this network lifetime; replacement requires its successful stop"))
		return nil
	}
	// RFC 5072 Section 2: "PPP links supporting IPv6 MUST allow the
	// information field to be at least as large as the minimum link MTU
	// size required for IPv6 [2]."
	if !supported {
		s.logger.Warn("ppp: refusing IPv6 service below the IPv6 minimum MRU",
			"peer_mru", peerMRU, "local_mru", localMRU, "session_mru", cap)
		return nil
	}
	if !negotiated {
		s.logger.Warn("ppp: IPv6CP opened without a negotiated peer interface identifier, refusing to start the IPv6 service",
			"tunnel_id", s.tunnelID, "session_id", s.sessionID, "reason", reasonServiceUnnegotiated)
		countIdentifierRefusal(reasonServiceUnnegotiated)
		return nil
	}

	// RFC 3162 Section 2.5: the same limit as the IPv4 routes, both families
	// counted together, refused before any route reaches the data plane.
	if err := checkFramedRoutesLimit(s.authenticatedFramedRoutes); err != nil {
		return fmt.Errorf("%w: %w", errFramedRouteRefused, err)
	}
	if !s.beginOwnerService() {
		return nil
	}
	svc, err := startIPv6Service(iPv6ServiceConfig{
		TunnelID:         s.tunnelID,
		SessionID:        s.sessionID,
		LocalInterfaceID: localID,
		PeerInterfaceID:  peerID,
		Dataplane:        s.dataplane,
		// RFC 3162 Section 2.5
		FramedRoutes: framedRoutesOfFamily(s.authenticatedFramedRoutes, AddressFamilyIPv6),
	}, DHCPv6DUID{Type: DUIDTypeLL, HWType: 1, ID: append([]byte(nil), localID[:]...)}, nil, s.logger)
	if svc != nil {
		s.setOwnerServiceState(true, err)
	} else {
		s.setOwnerServiceState(false, nil)
	}
	if svc != nil {
		// A failed startup may still own an address whose withdrawal failed.
		// Keep that owner for resetNCP and final session cleanup to retry.
		s.mu.Lock()
		s.ipv6Svc = svc
		s.mu.Unlock()
	}
	if err == nil {
		return nil
	}
	if svc != nil {
		s.reportIPv6ChildCleanupFailure(err)
		s.logger.Warn("ppp: IPv6 service startup cleanup remains unconfirmed",
			"code", CodeDoctorPPPCleanup, "error", err, "fix", CleanupFailureFix)
	}
	if errors.Is(err, errFramedRouteRefused) {
		return err
	}
	if svc == nil {
		s.logger.Warn("ppp: IPv6 service start failed (non-fatal)", "error", err)
	}
	return nil
}

// transmitMRU applies the admitted session cap without replacing the peer's
// negotiated receive capacity. An omitted peer option means 1500, while the
// admitted maxMRU is initialized by Driver.spawnSession for both transports.
// Callers MUST hold no session lock; this method takes it for the snapshot.
func (s *pppSession) transmitMRU() uint16 {
	s.mu.Lock()
	mru, cap := s.negotiatedMRU, s.maxMRU
	s.mu.Unlock()
	if mru == 0 {
		mru = MaxFrameLen
	}
	if cap == 0 {
		// An unspecified MaxMRU has the same default as driver admission.
		cap = MaxFrameLen
	}
	return min(mru, cap)
}

// afterLCPOpen performs the post-Opened work, in this order: check the
// transmit MRU against the IPv4 minimum, run authentication and the NCP
// phase, wait for the unit to drain after a reconnect, set the MTU on pppN
// through the iface backend (the transmit MRU, lowered to an accepted
// RADIUS Framed-MTU by interfaceMTU), connect the channel to the unit, set
// the MRU on the unit fd, bring pppN up, install the IPv4 Framed-Routes,
// start the IPv6 service (which installs the Framed-IPv6-Routes), and publish
// EventSessionUp with that MTU. The framed routes wait for the interface:
// Linux refuses a route through an interface that is down. Returns false on
// failure or an LCP interruption. The caller MUST resume the session loop
// when LCP has entered negotiation or termination.
//
// Order matters: set the interface MTU BEFORE connecting the channel, and
// the fd-side MRU BEFORE asking the kernel to bring the interface up, so
// the first frame is sized correctly.
//
// The IPv6 service is started by afterLCPOpenIPv6Service (above), which
// carries the guard on the peer's negotiated identifier.
func (s *pppSession) afterLCPOpen() bool {
	mru := s.transmitMRU()
	// RFC 791 Section 3.2: "Every internet module must be able to forward a datagram of 68
	// octets without further fragmentation."
	if mru < MinIPv4MRU {
		var reason textbuf.Buffer
		s.fail(reason.Str("negotiated PPP MRU ").Str(strconv.Itoa(int(mru))).
			Str(" is below the IPv4 minimum 68; configure the peer MRU to at least 68").String())
		return false
	}

	if !s.runAuthPhase() {
		return false
	}

	if !s.runNCPPhase() {
		return false
	}
	// Both disabled NCPs cannot ever publish an assignment in this round.
	if s.disableIPCP {
		if s.disableIPv6CP {
			s.abortAuthAdmission()
		}
	}

	// A reconnect after resetNCP waits until the unit has handed out every
	// frame of the NCP lifetime that ended (awaitUnitDrained).
	if s.unitDrainOwed && !s.awaitUnitDrained() {
		return false
	}

	// RFC 2865 Section 5.12: the accepted Framed-MTU lowers the IP MTU.
	mtu := s.interfaceMTU(mru)
	if err := s.activateNetwork(mtu); err != nil {
		if !errors.Is(err, ErrSessionNotFound) {
			s.fail(err.Error())
		}
		return false
	}
	// Publication can block. The original owner gate covers activation only,
	// so its cleanup and terminal receipt never wait for this event consumer.
	s.sendEvent(EventSessionUp{TunnelID: s.tunnelID, SessionID: s.sessionID, MTU: mtu})
	return true
}

// activateNetwork protects actual owner I/O with the release gate. Metadata
// locks cover only validation/state commits, never provider calls/publication.
// mtu is the IP MTU pppN gets (afterLCPOpen, interfaceMTU).
func (s *pppSession) activateNetwork(mtu uint16) error {
	s.nativeReleaseMu.Lock()
	defer s.nativeReleaseMu.Unlock()
	s.mu.Lock()
	invalid := s.filesClosed
	if s.sessionDownSent {
		invalid = true
	}
	s.mu.Unlock()
	select {
	case <-s.stopCh:
		return ErrSessionNotFound
	case <-s.sessStop:
		return ErrSessionNotFound
	default:
	}
	if invalid {
		return ErrSessionNotFound
	}
	var tb textbuf.Buffer
	// RFC 1332 Section 2.1: "The maximum length of an IP packet transmitted
	// over a PPP link is the same as the maximum length of the Information
	// field of a PPP data link layer frame." MRU already excludes framing.
	// The peer's receive capacity does not override the session's admitted
	// transmit limit. An omitted peer MRU still means 1500 for negotiation,
	// while PPPoE and an LNS behind a PPPoE LAC must keep their smaller cap.
	// An accepted RADIUS Framed-MTU may lower it further, never raise it.
	//
	// The MTU goes first because the interface of a reopened lifetime is still
	// up with the previous MTU: resetNCP disconnects the channel and leaves the
	// interface alone, and the connect below is what lets it transmit again.
	// refuse-by-default: guards pppSession.afterLCPOpen -- an MTU error ends the exact original before any channel connect.
	if err := s.dataplane.SetMTU(int(mtu)); err != nil {
		return errors.New(tb.Str("iface SetMTU: ").Err(err).String())
	}
	// Post-NCP: connect channel to unit and bring interface up.
	// PPPIOCCONNECT must happen after NCP because it changes frame
	// routing from channel fd to unit fd (accel-ppp uses the same
	// deferred-connect pattern): from here on the NCP frames arrive on
	// the unit, so readUnitFrames is woken to read them. MRU/AdminUp
	// follow.
	// refuse-by-default: guards pppSession.afterLCPOpen -- a connect error ends the exact original before receive-MRU or admin-up.
	if err := s.dataplane.Connect(); err != nil {
		return errors.New(tb.Reset().Str("PPPIOCCONNECT: ").Err(err).String())
	}
	s.mu.Lock()
	s.channelConnected = true
	s.mu.Unlock()
	s.wakeUnitReader()
	// RFC 1661 Section 6.1: "If smaller packets are requested, an
	// implementation MUST still be able to receive the full 1500 octet
	// information field in case link synchronization is lost."
	// The peer's MRU limits our sends, not the kernel's receive buffer.
	// refuse-by-default: guards pppSession.afterLCPOpen -- a receive-MRU error ends the exact original before admin-up.
	if err := s.dataplane.SetReceiveMRU(MaxFrameLen); err != nil {
		return errors.New(tb.Reset().Str("PPPIOCSMRU: ").Err(err).String())
	}
	// refuse-by-default: guards pppSession.afterLCPOpen -- an admin-up error prevents SessionUp for the exact original.
	if err := s.dataplane.SetAdminUp(); err != nil {
		return errors.New(tb.Reset().Str("iface SetAdminUp: ").Err(err).String())
	}
	s.mu.Lock()
	ipv4Opened := s.ipcpState == LCPStateOpened
	ipv6Opened := s.ipv6cpState == LCPStateOpened
	s.mu.Unlock()
	if ipv4Opened {
		// RFC 2865 Section 5.22: Linux refuses a route through an interface
		// that is down, so the Framed-Routes install only now, through the
		// address onNCPOpened installed. A refused route fails the session.
		if err := s.installNCPFramedRoutesIPv4Locked(); err != nil {
			return err
		}
	}
	if ipv6Opened {
		// RFC 3162 Section 2.5: a refused Framed-IPv6-Route fails the session.
		if err := s.afterLCPOpenIPv6Service(); err != nil {
			return err
		}
	}
	return nil
}

// runAuthPhase dispatches to the per-method authentication handler
// driven by s.negotiatedAuthMethod (set when LCP reaches Opened,
// either via proxy LCP or normal negotiation). AuthMethodNone runs
// the accounting-only path: one EventAuthRequest emitted, the
// handler's decision observed, no wire packets exchanged.
//
// Ordering note for consumers: on the reject path, EventSessionDown
// is emitted on EventsOut before EventAuthFailure is emitted on
// AuthEventsOut. Within each channel order is preserved, but a
// consumer that reads both channels sees them cross-channel.
//
// Returns false on failure, stop, or an LCP interruption. The caller MUST
// resume the session loop for a nonterminal LCP transition.
// An unnamed method is an internal programming error.
func (s *pppSession) runAuthPhase() bool {
	s.mu.Lock()
	method := s.negotiatedAuthMethod
	s.mu.Unlock()

	switch method {
	case AuthMethodNone:
		if s.authRequired {
			reason := "no negotiated authentication method"
			var tb textbuf.Buffer
			s.fail(tb.Str("auth: ").Str(reason).String())
			s.sendAuthEvent(eventAuthFailure{
				TunnelID:  s.tunnelID,
				SessionID: s.sessionID,
				Reason:    reason,
			})
			return false
		}
		return s.runNoAuthPhase()
	case AuthMethodPAP:
		return s.runPAPAuthPhase()
	case AuthMethodCHAPMD5:
		return s.runCHAPAuthPhase()
	case AuthMethodMSCHAPv2:
		return s.runMSCHAPv2AuthPhase()
	default:
		panic("BUG: unknown authentication method")
	}
}

// runNoAuthPhase is the AuthMethodNone dispatch target. It emits one
// EventAuthRequest on the auth channel so an external handler can
// still admit or deny by policy, waits on its exact request, and on accept
// emits EventAuthSuccess. The session's LCP Auth-Protocol option was
// absent or REJECTed, so no wire packets are exchanged here.
func (s *pppSession) runNoAuthPhase() bool {
	req := EventAuthRequest{
		TunnelID:  s.tunnelID,
		SessionID: s.sessionID,
		Method:    AuthMethodNone,
	}
	decision, ok := s.awaitAuthDecision(req, "")
	if !ok {
		return false
	}
	if !decision.accept {
		var tb textbuf.Buffer
		s.fail(tb.Str("auth rejected: ").Str(decision.message).String())
		s.sendAuthEvent(eventAuthFailure{
			TunnelID:  s.tunnelID,
			SessionID: s.sessionID,
			Reason:    decision.message,
		})
		return false
	}
	s.sendAuthEvent(eventAuthSuccess{
		TunnelID:  s.tunnelID,
		SessionID: s.sessionID,
	})
	return true
}

// sendAuthEvent writes to authEventsOut, blocking until the consumer
// reads, the driver stops, or the session is stopped. Mirrors
// sendEvent for the separate auth channel. The consumer MUST drain
// events promptly or PPP processing will stall (buffer is
// defaultAuthEventsOutBuf, 64).
func (s *pppSession) sendAuthEvent(ev AuthEvent) {
	switch value := ev.(type) {
	case EventAuthRequest:
		value.SubscriberID = s.subscriberID
		value.Control = s.control
		ev = value
	case eventAuthSuccess:
		value.Control = s.control
		ev = value
	case eventAuthFailure:
		value.Control = s.control
		ev = value
	}
	select {
	case s.authEventsOut <- ev:
	case <-s.stopCh:
	case <-s.sessStop:
	}
}

// awaitUnitDrained discards every frame the unit still delivers after
// resetNCP disconnected the channel, until readUnitFrames reads EOF, and
// reports whether the session may connect again. ppp_read dequeues a queued
// frame before it checks for a channel, and PPPIOCDISCONN does not purge the
// unit's queue, so the frames the peer sent before the disconnect are read
// after it: some wait in unitFramesIn, and the rest stay in the kernel while
// the reader is blocked on a full unitFramesIn. Nothing new reaches a unit
// with no channel, so every frame before the EOF belongs to the NCP lifetime
// that ended. Connecting first would let the rest be read as frames of the
// new one. Called by the session goroutine, without mu held.
func (s *pppSession) awaitUnitDrained() bool {
	for {
		select {
		case <-s.unitIdle:
			drainFramesToPool(s.unitFramesIn)
			s.unitDrainOwed = false
			return true
		case frame := <-s.unitFramesIn:
			s.logger.Debug("ppp: unit frame from the ended NCP lifetime discarded", "len", len(frame))
			putFrameBuf(frame)
		case <-s.unitGone:
			s.fail("unit fd reader ended before the channel could be connected again")
			return false
		case <-s.stopCh:
			return false
		case <-s.sessStop:
			return false
		}
	}
}

// wakeUnitReader tells readUnitFrames that the channel is connected, so the
// unit now carries NCP frames. The one-slot queue never blocks: a wake still
// pending covers this connect too.
func (s *pppSession) wakeUnitReader() {
	if s.unitConnected == nil {
		return
	}
	select {
	case s.unitConnected <- struct{}{}:
	default:
	}
}

// readUnitFrames is the session's one reader of unitFile, for the session's
// lifetime. It sleeps until wakeUnitReader reports a connect, then forwards
// every frame the unit delivers on out. EOF means the unit has no channel
// any more (PPPIOCDISCONN in resetNCP, or the kernel unregistering the
// channel at teardown, both of which wake the read), so it sleeps again
// until the next connect. Any other read error is sent on done and ends it.
// It exits when stop closes.
//
// Linux ppp_input queues a frame on the channel only while "!pch->ppp ||
// proto >= 0xc000 || proto == PPP_CCPFRAG", and once the channel is
// connected ppp_receive_nonmp_frame queues a control or unknown frame on the
// unit: "/* control or unknown frame - pass it to pppd */
// skb_queue_tail(&ppp->file.rq, skb);" (drivers/net/ppp/ppp_generic.c,
// v6.12). Without this reader every IPCP and IPv6CP packet after Opened was
// lost.
func (s *pppSession) readUnitFrames(out chan<- []byte, done chan<- error, stop <-chan struct{}) {
	defer close(s.unitGone)
	for {
		select {
		case <-s.unitConnected:
		case <-stop:
			return
		}
		for {
			buf := getFrameBuf()
			n, err := s.unitFile.Read(buf)
			if errors.Is(err, io.EOF) {
				putFrameBuf(buf)
				// awaitUnitDrained waits for this before the next connect.
				select {
				case s.unitIdle <- struct{}{}:
				default:
				}
				break
			}
			if err != nil {
				putFrameBuf(buf)
				select {
				case done <- err:
				case <-stop:
				}
				return
			}
			if n < 2 {
				putFrameBuf(buf)
				s.logger.Warn("ppp: undersized frame from unit fd", "len", n)
				continue
			}
			select {
			case out <- buf[:n]:
			case <-stop:
				putFrameBuf(buf)
				return
			}
		}
	}
}

// readFrames reads PPP frames from chanFile in a loop and forwards
// them on the channel. Exits on read error, which it reports on done,
// and on a quiesce (Driver.QuiesceSession), which it does not: the session
// is being handed to a realm relay, not ending.
// The owner MUST close stop before closing the frame owner and joining this
// reader, so a queued sender can exit even after its consumer has returned.
func (s *pppSession) readFrames(out chan<- []byte, done chan<- error, stop <-chan struct{}) {
	defer close(out)
	if s.readerGone != nil {
		defer close(s.readerGone)
	}
	for {
		buf := getFrameBuf()
		n, err := s.chanFile.Read(buf)
		if err != nil {
			putFrameBuf(buf)
			if s.readerQuiesced() {
				return
			}
			done <- err
			return
		}
		if n < 2 {
			putFrameBuf(buf)
			s.logger.Warn("ppp: undersized frame from chan fd", "len", n)
			continue
		}
		select {
		case out <- buf[:n]:
		case <-stop:
			putFrameBuf(buf)
			return
		case <-s.stopCh:
			putFrameBuf(buf)
			return
		case <-s.readerQuiesce:
			// The frame is dropped: a quiesced channel is read by nobody,
			// and the subscriber retransmits it after the bridge.
			putFrameBuf(buf)
			return
		}
	}
}

// handleFrame parses one received frame and dispatches by protocol to
// the appropriate FSM handler. LCP, IPCP, and IPv6CP all share the
// same packet shape (RFC 1661 §5); only the Data semantics differ.
// Returns true if the session should terminate.
//
// Wire format (RFC 1661 Sections 2 and 5), byte offsets in the PPP frame:
//
//	0             2       3            4           6
//	+-------------+-------+------------+-----------+---------+
//	| Protocol(2) | Code  | Identifier | Length(2) | Data... |
//	+-------------+-------+------------+-----------+---------+
//
// Length counts the control packet from Code, excluding Protocol.
func (s *pppSession) handleFrame(frame []byte) bool {
	proto, payload, _, err := ParseFrame(frame)
	if err != nil {
		s.logger.Debug("ppp: malformed frame dropped", "error", err.Error())
		return false
	}

	if proto == ProtoIPCP || proto == ProtoIPv6CP {
		// RFC 1661 Section 3.4: "Any non-LCP packets received during this
		// phase MUST be silently discarded." Section 3.5: "Advancement
		// from the Authentication phase to the Network-Layer Protocol
		// phase MUST NOT occur until authentication has completed."
		// LCP Opened alone is insufficient: replacement authentication
		// runs there too. A frame sent while runNCPPhase starts its FSMs
		// waits in the channel, because that phase reads no frame until an
		// enabled FSM has left Initial.
		if !s.ncpStarted {
			return false
		}
	}

	pkt, perr := ParseLCPPacket(payload)

	switch proto {
	case ProtoLCP:
		if perr != nil {
			s.logger.Debug("ppp: malformed LCP packet", "error", perr.Error())
			return false
		}
		return s.handleLCPPacket(pkt)
	case ProtoPAP:
		if s.papReplyCode != 0 {
			return !s.reanswerPAP(payload)
		}
		return false
	case ProtoIPCP:
		// A disabled IPCP stays in Initial for good, so its peer's
		// retransmissions are answered here or never.
		// RFC 1661 Section 5.7: "If the LCP automaton is in the Opened state,
		// then this MUST be reported back to the peer by transmitting a
		// Protocol-Reject."
		if s.disableIPCP {
			return s.rejectUnsupportedProtocol(proto, payload)
		}
		if perr != nil {
			s.logger.Debug("ppp: malformed IPCP packet", "error", perr.Error())
			return false
		}
		return s.handleIPCPPacket(pkt)
	case ProtoIPv6CP:
		// Once IPv6CP is dropped (config, or handler-declined IPv6 in
		// runNCPPhase), the peer keeps retransmitting CONFREQ. Answer them:
		// RFC 1661 Section 5.7 is what stops the retransmissions, where
		// dropping them left a dual-stack peer retrying IPV6CP for its whole
		// Configure-Request budget.
		// RFC 5072 Section 2: "PPP links supporting IPv6 MUST allow the
		// information field to be at least as large as the minimum link MTU
		// size required for IPv6 [2]."
		if s.disableIPv6CP || !s.ipv6MRUSupported() {
			return s.rejectUnsupportedProtocol(proto, payload)
		}
		if perr != nil {
			s.logger.Debug("ppp: malformed IPv6CP packet", "error", perr.Error())
			return false
		}
		return s.handleIPv6CPPacket(pkt)
	}
	return s.rejectUnsupportedProtocol(proto, payload)
}

// protocolHex renders a PPP Protocol field as the hexadecimal form every RFC
// and every operator writes it in, "0x8057" rather than 32855.
func protocolHex(proto uint16) string {
	var raw [2]byte
	binary.BigEndian.PutUint16(raw[:], proto)
	var tb textbuf.Buffer
	return tb.Str("0x").Hex(raw[:]).String()
}

// supportsProtocol reports whether ze implements the protocol on this
// session. It is the question RFC 1661 Section 3.6 asks before it decides
// between a silent discard and a Protocol-Reject: "While LCP is in the Opened
// state, any protocol packet which is unsupported by the implementation MUST
// be returned in a Protocol-Reject (described later). Only protocols which
// are supported are silently discarded."
//
// An NCP the operator turned off takes its network-layer protocol with it: a
// session that will never open IPv6CP does not support IPv6, so an IPv6
// packet on it is unsupported rather than early.
// A negotiated MRU too small for IPv6 disables the same protocol pair for
// this LCP lifetime, even when the admitted configuration enables IPv6CP.
func (s *pppSession) supportsProtocol(proto uint16) bool {
	switch proto {
	case ProtoLCP, ProtoPAP, ProtoCHAP:
		return true
	case ProtoIPCP, ProtoIPv4:
		return !s.disableIPCP
	case ProtoIPv6CP, ProtoIPv6:
		// RFC 5072 Section 2: "PPP links supporting IPv6 MUST allow the
		// information field to be at least as large as the minimum link MTU
		// size required for IPv6 [2]."
		return !s.disableIPv6CP && s.ipv6MRUSupported()
	}
	return false
}

// rejectUnsupportedProtocol answers one frame whose Protocol field names a
// protocol ze does not support, and reports whether the session must end.
//
// RFC 1661 Section 5.7: "Protocol-Reject packets can only be sent in the LCP
// Opened state. Protocol-Reject packets received in any state other than the
// LCP Opened state SHOULD be silently discarded." The send is gated on Opened
// for the first sentence; a frame that arrives before LCP opens is dropped
// without an answer.
func (s *pppSession) rejectUnsupportedProtocol(proto uint16, info []byte) bool {
	if s.supportsProtocol(proto) {
		// RFC 1661 Section 3.6: "Any supported network-layer protocol packets
		// received when the corresponding NCP is not in the Opened state MUST
		// be silently discarded", and its Implementation Note draws the line
		// this branch stands on: "Only protocols which are supported are
		// silently discarded."
		s.logger.Debug("ppp: supported protocol discarded, not configured yet",
			"protocol", protocolHex(proto))
		return false
	}
	cur := s.currentState()
	if cur != LCPStateOpened {
		s.logger.Debug("ppp: unsupported protocol dropped, LCP is not Opened",
			"protocol", protocolHex(proto), "state", cur.String())
		return false
	}
	s.logger.Info("ppp: unsupported protocol rejected", "protocol", protocolHex(proto))
	return !s.sendProtocolReject(proto, info)
}

// sendProtocolReject transmits an LCP Protocol-Reject for a protocol ze does
// not support, and returns false on a fatal write error.
//
// RFC 1661 Section 5.7: "Reception of a PPP packet with an unknown Protocol
// field indicates that the peer is attempting to use a protocol which is
// unsupported. This usually occurs when the peer attempts to configure a new
// protocol. If the LCP automaton is in the Opened state, then this MUST be
// reported back to the peer by transmitting a Protocol-Reject."
//
// Wire format (RFC 1661 Section 5.7), offsets from the start of the LCP
// packet, which itself starts 2 octets into the frame:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|   Code = 8    |  Identifier   |            Length             |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|       Rejected-Protocol       |      Rejected-Information ...
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// Octet 0 Code, 1 Identifier, 2-3 Length, 4-5 Rejected-Protocol, 6 onward
// Rejected-Information.
//
// "The Rejected-Protocol field is two octets, and contains the PPP Protocol
// field of the packet which is being rejected." "The Rejected-Information
// field contains a copy of the packet which is being rejected. It begins with
// the Information field, and does not include any Data Link Layer headers nor
// an FCS", which is what info carries: the frame after its Protocol field.
func (s *pppSession) sendProtocolReject(proto uint16, info []byte) bool {
	// RFC 1661 Section 5.7: "The Rejected-Information MUST be truncated to
	// comply with the peer's established MRU." The MRU bounds the Information
	// field of the frame ze sends. The admitted session cap and pooled buffer
	// can impose a smaller limit, so the copy takes their minimum, less the
	// octets that precede it: the 2-octet Protocol field, the 4-octet LCP
	// header and the 2-octet Rejected-Protocol.
	mru := s.transmitMRU()
	limit := min(frameLen(int(mru)), MaxFrameBufLen)
	room := max(limit-frameLen(lcpHeaderLen+2), 0)
	if len(info) > room {
		info = info[:room]
	}

	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ProtoLCP, nil)
	// The Rejected-Protocol and the copy are written where WriteLCPPacket
	// needs the Data field, so no scratch buffer is allocated. The copy that
	// call then makes is onto itself and moves nothing.
	data := buf[off+lcpHeaderLen:]
	binary.BigEndian.PutUint16(data[:2], proto)
	n := copy(data[2:], info)
	// RFC 1661 Section 5.7: "The Identifier field MUST be changed for each
	// Protocol-Reject sent."
	s.protocolRejectID++
	off += WriteLCPPacket(buf, off, LCPProtocolReject, s.protocolRejectID, data[:2+n])
	return s.writeFrame(buf[:off])
}

// rejectedProtocolLen is the length of the Rejected-Protocol field that opens
// the Data of a Protocol-Reject.
const rejectedProtocolLen = 2

// receiveProtocolReject reads the Rejected-Protocol of a Protocol-Reject the
// caller received in LCP Opened, and stops the NCP it names. It reports
// whether the session must end.
//
// RFC 1661 Section 5.7, the Protocol-Reject packet. ParseLCPPacket consumed
// octets 0 to 3, so pkt.Data starts at octet 4:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|     Code      |  Identifier   |            Length             |  octets 0-3
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|       Rejected-Protocol       |      Rejected-Information ...    octets 4-5, 6-
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// pkt.Data[0:2] is the Rejected-Protocol and pkt.Data[2:] the
// Rejected-Information, which nothing here reads.
//
// RFC 1661 Section 5.7: "Upon reception of a Protocol-Reject, the
// implementation MUST stop sending packets of the indicated protocol at the
// earliest opportunity."
//
// Only an NCP is stopped here. A Protocol-Reject naming LCP, or a protocol Ze
// runs no automaton for, stops nothing here. The caller's transition then
// takes the event protocolRejectEvent names: RXJ- for LCP, which ends the
// session, and RXJ+ for everything else, which leaves LCP Opened.
func (s *pppSession) receiveProtocolReject(pkt LCPPacket) bool {
	if len(pkt.Data) < rejectedProtocolLen {
		// RFC 1661 Section 5.7: "The Rejected-Protocol field is two
		// octets, and contains the PPP Protocol field of the packet which is
		// being rejected." A shorter packet names no protocol, so it stops
		// nothing (pppd lcp_rprotrej returns on it the same way).
		s.logger.Debug("ppp: Protocol-Reject too short to name a protocol, nothing stopped",
			"id", pkt.Identifier, "len", len(pkt.Data))
		return false
	}
	rejected := binary.BigEndian.Uint16(pkt.Data[:rejectedProtocolLen])
	// The Rejected-Protocol comes from the wire, an open set: every value
	// but the two NCPs falls through to the log line below.
	switch rejected {
	case ProtoIPCP:
		// RFC 1661 Section 4.3
		return s.stopRejectedNCP(AddressFamilyIPv4)
	case ProtoIPv6CP:
		// RFC 1661 Section 4.3
		return s.stopRejectedNCP(AddressFamilyIPv6)
	}
	s.logger.Debug("ppp: Protocol-Reject names no NCP ze runs, nothing stopped",
		"id", pkt.Identifier, "protocol", rejected)
	return false
}

// codeToEvent maps a received LCP packet to the FSM event it raises. Maps
// unknown codes to RUC. A Code-Reject and a Protocol-Reject raise RXJ+ or
// RXJ- by what they reject (lcpCodeRejectEvent, protocolRejectEvent), so this
// reads the packet and not its Code alone.
func codeToEvent(pkt LCPPacket, optsBad bool) lCPEvent {
	switch pkt.Code {
	case LCPConfigureRequest:
		if optsBad {
			return LCPEventRCRMinus
		}
		return LCPEventRCRPlus
	case LCPConfigureAck:
		return LCPEventRCA
	case LCPConfigureNak, LCPConfigureReject:
		return LCPEventRCN
	case LCPTerminateRequest:
		return LCPEventRTR
	case LCPTerminateAck:
		return LCPEventRTA
	case LCPCodeReject:
		// RFC 1661 Section 4.3
		return lcpCodeRejectEvent(pkt)
	case LCPProtocolReject:
		// RFC 1661 Section 4.3
		return protocolRejectEvent(pkt)
	case LCPEchoRequest, LCPEchoReply, LCPDiscardRequest:
		return LCPEventRXR
	}
	return LCPEventRUC
}

// rejectedCodeOffset is where the rejected packet's own Code sits in the Data
// of a received Code-Reject: the Rejected-Packet opens the Data, and the
// rejected packet's header opens the Rejected-Packet.
const rejectedCodeOffset = 0

// lcpCodeRejectEvent decides whether a received LCP Code-Reject is RXJ+ or
// RXJ-, from the Code of the packet it rejects.
//
// RFC 1661 Section 5.6, the Code-Reject packet. ParseLCPPacket consumed
// octets 0 to 3, so pkt.Data starts at octet 4, the Rejected-Packet:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|   Code = 7    |  Identifier   |            Length             |  octets 0-3
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	| Rejected-Packet ...                                              octet 4 on
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// "The Rejected-Packet field contains a copy of the LCP packet which is being
// rejected." So pkt.Data[0] is the rejected packet's Code, pkt.Data[1] its
// Identifier, pkt.Data[2:4] its Length and pkt.Data[4:] its Data, cut to the
// rejecter's MRU. Only the Code is read here.
//
// RFC 1661 Section 4.3: "The RXJ+ event arises when the rejected value is
// acceptable, such as a Code-Reject of an extended code, or a
// Protocol-Reject of a NCP." and "The RXJ- event arises when the rejected
// value is catastrophic, such as a Code-Reject of Configure-Request, or a
// Protocol-Reject of LCP!  This event communicates an unrecoverable error
// that terminates the connection."
//
// The catastrophic Codes are the ones the automaton itself needs: Codes 1 to
// 7, which every RFC 1661 control protocol exchanges, and Protocol-Reject,
// which RFC 1661 Section 3.6 makes LCP send for every unsupported protocol:
// "While LCP is in the Opened state, any protocol packet which is unsupported
// by the implementation MUST be returned in a Protocol-Reject (described
// later)." A peer that cannot take one of them cannot run LCP with Ze.
// Echo-Request, Echo-Reply and Discard-Request are the extended Codes: LCP
// works without them, so their reject is RXJ+ and stopRejectedCode stops what
// Ze sends of them. pppd v2.5.5 fsm.c fsm_rcoderej treats every Code-Reject
// as RXJ+ ("if( f->state == ACKRCVD ) f->state = REQSENT;"), which the RFC
// sentence above does not allow for a Code-Reject of Configure-Request.
//
// A Code-Reject too short to carry the rejected Code names nothing, so it
// stays RXJ+ and stops nothing, as every Code-Reject did before this function
// existed.
func lcpCodeRejectEvent(pkt LCPPacket) lCPEvent {
	if len(pkt.Data) <= rejectedCodeOffset {
		return LCPEventRXJPlus
	}
	// The rejected Code comes from the wire, an open set: every value not
	// named here was never sent by Ze, an extended Code, and is RXJ+.
	switch pkt.Data[rejectedCodeOffset] {
	case LCPConfigureRequest, LCPConfigureAck, LCPConfigureNak, LCPConfigureReject,
		LCPTerminateRequest, LCPTerminateAck, LCPCodeReject, LCPProtocolReject:
		return LCPEventRXJMinus
	case LCPEchoRequest, LCPEchoReply, LCPDiscardRequest:
		return LCPEventRXJPlus
	}
	return LCPEventRXJPlus
}

// protocolRejectEvent decides whether a Protocol-Reject received in LCP
// Opened is RXJ+ or RXJ-, from its Rejected-Protocol (layout above
// receiveProtocolReject).
//
// RFC 1661 Section 4.3: "The RXJ- event arises when the rejected value is
// catastrophic, such as a Code-Reject of Configure-Request, or a
// Protocol-Reject of LCP!  This event communicates an unrecoverable error
// that terminates the connection." pppd v2.5.5 lcp.c lcp_protrej answers a
// Protocol-Reject naming LCP the same way: "error("Received Protocol-Reject
// for LCP!"); fsm_protreject(&lcp_fsm[unit]);", whose comment reads "Treat
// this as a catastrophic error (RXJ-)."
//
// A Protocol-Reject naming an NCP, any other protocol, or none (shorter than
// the Rejected-Protocol field) is RXJ+.
func protocolRejectEvent(pkt LCPPacket) lCPEvent {
	if len(pkt.Data) < rejectedProtocolLen {
		return LCPEventRXJPlus
	}
	if binary.BigEndian.Uint16(pkt.Data[:rejectedProtocolLen]) == ProtoLCP {
		return LCPEventRXJMinus
	}
	return LCPEventRXJPlus
}

// stopRejectedCode records the Code a received LCP Code-Reject rejects, when
// it is one Ze sends and can stop sending: Echo-Request (the echo ticker in
// run) and Echo-Reply (the ser action in performAction). Ze sends no
// Discard-Request, Identification or Time-Remaining, and the Codes LCP needs
// raise RXJ- instead (lcpCodeRejectEvent), so nothing else is recorded.
//
// RFC 1661 Section 4.3: "The RXJ+ event arises when the rejected value is
// acceptable, such as a Code-Reject of an extended code, or a
// Protocol-Reject of a NCP.  These are within the scope of normal
// operation.  The implementation MUST stop sending the offending packet
// type."
//
// pppd v2.5.5 fsm.c fsm_rcoderej only logs the rejected Code ("warn("%s: Rcvd
// Code-Reject for code %d, id %d", ...)") and LcpSendEchoRequest keeps
// sending, so pppd meets this sentence only for Codes it never sends.
func (s *pppSession) stopRejectedCode(pkt LCPPacket) {
	if len(pkt.Data) <= rejectedCodeOffset {
		return
	}
	// Only the Codes Ze can stop sending are recorded; every other rejected
	// Code is RXJ- or one Ze never sends.
	switch pkt.Data[rejectedCodeOffset] {
	case LCPEchoRequest:
		s.echoRequestRejected = true
		s.logger.Debug("ppp: peer Code-Rejected Echo-Request, echo probes stopped",
			"id", pkt.Identifier)
	case LCPEchoReply:
		s.echoReplyRejected = true
		s.logger.Debug("ppp: peer Code-Rejected Echo-Reply, Echo-Requests go unanswered",
			"id", pkt.Identifier)
	}
}

// fatalRejectReason names what a received reject that raised RXJ- rejected,
// for the session-down reason: the Code a Code-Reject carries, or LCP for a
// Protocol-Reject.
func fatalRejectReason(pkt LCPPacket) string {
	var tb textbuf.Buffer
	if pkt.Code == LCPProtocolReject {
		return tb.Str("LCP terminated: peer Protocol-Rejected LCP").String()
	}
	if len(pkt.Data) <= rejectedCodeOffset {
		// lcpCodeRejectEvent raises RXJ- only for a Code it read, so this
		// arm is not reached; it keeps a short packet from indexing past Data.
		return tb.Str("LCP terminated: peer Code-Reject").String()
	}
	return tb.Str("LCP terminated: peer Code-Rejected ").
		Str(LCPCodeName(pkt.Data[rejectedCodeOffset])).String()
}

// lcpRequestVerdict is what a received LCP Configure-Request earns. A bool
// carries only the first two: RFC 1661 Section 6 has a third answer for a
// packet whose options do not fit inside it, and that answer is no answer
// at all.
type lcpRequestVerdict uint8

const (
	// lcpRequestAcceptable feeds the FSM an RCR+, which Acks.
	lcpRequestAcceptable lcpRequestVerdict = iota
	// lcpRequestUnacceptable feeds the FSM an RCR-, which Naks or Rejects.
	lcpRequestUnacceptable
	// lcpRequestDiscard feeds the FSM nothing: the packet is invalid and is
	// dropped without a reply and without a state change.
	lcpRequestDiscard
)

// negPolicy is the policy ze judges a peer Configure-Request against. One
// declaration serves both the verdict and the reply it leads to, so the two
// can never disagree about what ze wants.
func (s *pppSession) negPolicy() LCPNegPolicy {
	// RFC 791 Section 3.2: "Every internet module must be able to forward a datagram of 68
	// octets without further fragmentation."
	return LCPNegPolicy{MinMRU: MinIPv4MRU, MaxMRU: s.maxMRU, LocalMagic: s.magic, PPPoE: s.pppoe}
}

// evalLCPRequest judges the peer's LCP Configure-Request and returns the
// options it carried. The option list is nil on anything but a clean walk,
// because a caller that reads options out of a request ze could not parse
// is reading the prefix that happened to precede the fault.
func (s *pppSession) evalLCPRequest(pkt LCPPacket) (lcpRequestVerdict, []LCPOption) {
	w := WalkLCPOptions(pkt.Data)
	if w.Fault.Discards() {
		// RFC 1661 Section 6: "When the Data field is indicated by the
		// Length to extend beyond the end of the Information field, the
		// entire packet is silently discarded without affecting the
		// automaton."
		return lcpRequestDiscard, nil
	}
	if w.Fault == LCPOptionsBadLength {
		// The list is contained in the packet, so the fault is one option's
		// own Length. RFC 1661 Section 6: "If a negotiable Configuration
		// Option is received in a Configure-Request, but with an invalid or
		// unrecognized Length, a Configure-Nak SHOULD be transmitted which
		// includes the desired Configuration Option with an appropriate
		// Length and Data."
		return lcpRequestUnacceptable, nil
	}
	_, naks, rejects := NegotiatePeerOptions(w.Options, s.negPolicy())
	if len(naks) > 0 || len(rejects) > 0 {
		return lcpRequestUnacceptable, w.Options
	}
	return lcpRequestAcceptable, w.Options
}

// handleLCPPacket maps the received LCP code to an FSM event and
// drives the resulting actions. Returns true if the session should
// terminate.
func (s *pppSession) handleLCPPacket(pkt LCPPacket) bool {
	cur := s.currentState()

	// RFC 1661 Section 6.4: "All received Magic-Number fields MUST be
	// equal to either zero or the peer's unique Magic-Number, depending on
	// whether or not the peer negotiated a Magic-Number." peerMagic is
	// exactly that value: the one ze acknowledged, or zero. Any other
	// value comes from a looped-back link or a link joined to a different
	// peer, so the packet is dropped before it can answer an Echo-Request
	// or count as a reply to ze's own.
	switch pkt.Code {
	case LCPEchoRequest, LCPEchoReply, LCPDiscardRequest:
		received, err := parseLCPEchoMagic(pkt.Data)
		if err != nil {
			s.logger.Debug("ppp: LCP echo packet dropped, no Magic-Number field",
				"code", LCPCodeName(pkt.Code), "id", pkt.Identifier, "len", len(pkt.Data))
			return false
		}
		if received != s.peerMagic {
			s.logger.Warn("ppp: LCP echo packet dropped, Magic-Number is not the peer's",
				"code", LCPCodeName(pkt.Code), "id", pkt.Identifier,
				"received", received, "peer", s.peerMagic, "local", s.magic)
			return false
		}
	}

	if pkt.Code == LCPEchoReply {
		s.mu.Lock()
		s.echoOutstanding = 0
		s.mu.Unlock()
		// RFC 1661 Section 5.8: emit RTT for CQM aggregation.
		if !s.lastEchoSentAt.IsZero() {
			s.sendEvent(EventEchoRTT{
				TunnelID:  s.tunnelID,
				SessionID: s.sessionID,
				RTT:       time.Since(s.lastEchoSentAt),
			})
		}
	}

	// RFC 1661 Section 6: "When the Data field is indicated by the Length to
	// extend beyond the end of the Information field, the entire packet is
	// silently discarded without affecting the automaton."
	//
	// The sentence sits under the Data field of the Configuration Option
	// format, and names no Code, where the Configure-Nak sentence one
	// paragraph above it is scoped to "received in a Configure-Request". So
	// it binds every packet whose Data field carries Configuration Options,
	// not the request alone. RFC 1661 Section 5.2 reaches the same answer for
	// an Ack in its own words: "Invalid packets are silently discarded."
	//
	// The Configure-Request is held to it by evalLCPRequest below, which has
	// to tell the fault apart from an unacceptable value to pick its verdict.
	// The three reply codes need no verdict, so they are held to it here.
	switch pkt.Code {
	case LCPConfigureAck, LCPConfigureNak, LCPConfigureReject:
		if !s.lastLCPRequestSent || !ValidateLCPReply(pkt, s.lastLCPRequestID, s.lastLCPRequest[:s.lastLCPRequestLen]) {
			s.logger.Debug("ppp: LCP reply silently discarded, no matching request",
				"state", cur.String(),
				"code", LCPCodeName(pkt.Code),
				"id", pkt.Identifier,
				"len", len(pkt.Data))
			return false
		}
		s.lastLCPReplyReceived = true
		if pkt.Code == LCPConfigureAck {
			s.proxyCapture.noteOwnRequestAcked(s.lastLCPRequest[:s.lastLCPRequestLen])
		}
	}

	// optsBad chooses between RCR+ and RCR- and carries nothing else. The
	// third answer RFC 1661 Section 6 has for a Configure-Request -- no
	// answer at all -- returns below, before the automaton runs.
	optsBad := false
	var peerOpts []LCPOption
	if pkt.Code == LCPConfigureRequest {
		verdict, opts := s.evalLCPRequest(pkt)
		if verdict == lcpRequestDiscard {
			// RFC 1661 Section 6: "When the Data field is indicated by the
			// Length to extend beyond the end of the Information field, the
			// entire packet is silently discarded without affecting the
			// automaton."
			//
			// The silence is owed twice. Any reply to a packet an off-path
			// sender can forge makes ze a reflector, and this packet is one
			// no conforming peer emits.
			s.logger.Debug("ppp: LCP Configure-Request silently discarded",
				"state", cur.String(),
				"id", pkt.Identifier,
				"len", len(pkt.Data))
			return false
		}
		peerOpts = opts
		optsBad = verdict == lcpRequestUnacceptable
	}

	// A validated configuration packet or a Terminate-Ack leaving Opened starts
	// a fresh LCP lifetime: each of them is a tld,scr edge in LCPDoTransition.
	// Reset before applying its Nak/Reject, never after that mutation.
	if cur == LCPStateOpened {
		switch pkt.Code {
		case LCPConfigureRequest, LCPConfigureAck, LCPConfigureNak, LCPConfigureReject, LCPTerminateAck:
			s.resetAuthNegotiation()
		}
	}

	// RFC 1661 §5.3 (Configure-Nak) / §5.4 (Configure-Reject): when
	// the peer rejects or suggests an alternative to the Auth-
	// Protocol ze advertised, adjust configuredAuthMethod BEFORE the
	// FSM fires LCPActSCR. The resent CONFREQ is built from
	// configuredAuthMethod, so the mutation must happen in the same
	// handleLCPPacket call as the received Nak/Reject.
	//
	// Gated on the negotiating states whose RCN edge emits SCR per
	// the RFC 1661 §4.1 transition table (see ppp_fsm.go). Naks in
	// Closed / Stopped / Closing / Stopping states are handled with
	// STA or ignored and never trigger a resend, so mutating the
	// method there would be wasted work on a stale packet.
	if pkt.Code == LCPConfigureNak || pkt.Code == LCPConfigureReject {
		// Only negotiating states apply Nak/Reject option mutations before the FSM.
		//exhaustive:ignore
		switch cur {
		case LCPStateReqSent, LCPStateAckSent, LCPStateAckRcvd, LCPStateOpened:
			s.adjustAuthOnNakOrReject(pkt)
			if s.authRequired && s.configuredAuthMethod == AuthMethodNone {
				return s.refuseAuthMethod(cur, pkt)
			}
			opts, _ := ParseLCPOptions(pkt.Data)
			for _, opt := range opts {
				if pkt.Code == LCPConfigureReject {
					switch opt.Type {
					case LCPOptMRU:
						s.lcpRejectedMRU = true
					case LCPOptMagic:
						s.lcpRejectedMagic = true
					}
				} else if opt.Type == LCPOptMRU && len(opt.Data) == 2 {
					mru := binary.BigEndian.Uint16(opt.Data)
					// RFC 791 Section 3.2: "Every internet module must be able to forward a datagram of 68
					// octets without further fragmentation."
					if mru >= MinIPv4MRU && mru <= s.maxMRU {
						s.localMRU = mru
					}
				}
			}
			if pkt.Code == LCPConfigureNak {
				if !s.redrawMagicOnNak(pkt) {
					return true
				}
			}
		case LCPStateInitial, LCPStateStarting, LCPStateClosed,
			LCPStateStopped, LCPStateClosing, LCPStateStopping:
			// Nak/Reject in these states never triggers SCR per RFC
			// 1661 §4.1 transition table: handled with STA (Stopped),
			// ignored (Starting / Initial), or no-op (Closing /
			// Stopping). Skip the mutation -- the packet is stale.
		}
	}

	if pkt.Code == LCPProtocolReject {
		if cur != LCPStateOpened {
			// RFC 1661 Section 5.7: "Protocol-Reject packets received in
			// any state other than the LCP Opened state SHOULD be silently
			// discarded." Discarded means before the automaton too: the
			// RXJ+ codeToEvent maps it to would otherwise take Ack-Rcvd to
			// Req-Sent, and the RXJ- of one naming LCP would end the
			// negotiation (pppd lcp_rprotrej returns before any dispatch).
			s.logger.Debug("ppp: Protocol-Reject silently discarded outside LCP Opened",
				"state", cur.String(), "id", pkt.Identifier, "len", len(pkt.Data))
			return false
		}
		// RFC 1661 Section 5.7
		if s.receiveProtocolReject(pkt) {
			return true
		}
	}

	ev := codeToEvent(pkt, optsBad)
	if pkt.Code == LCPCodeReject {
		// RFC 1661 Section 4.3. Recorded before the transition, because
		// RXJ+ in Opened is a no-op the automaton returns from below. Only
		// the RXJ+ Codes Ze sends are recorded, so an RXJ- records nothing.
		s.stopRejectedCode(pkt)
	}

	tr := LCPDoTransition(cur, ev)
	if tr.NewState == cur && len(tr.Actions) == 0 {
		s.logFSMNoOp(cur, ev, pkt)
		return false
	}
	if ev == LCPEventRXJMinus {
		// RFC 1661 Section 4.3: RXJ- "communicates an unrecoverable error
		// that terminates the connection." The Closed or Stopped it leads to
		// ends the session (sessionEndedAt), and applyTransition reports it
		// with this reason.
		s.lcpRejectFatalReason = fatalRejectReason(pkt)
	}

	// ISSUE 3 fix: capture the peer's MRU from its accepted CR.
	// The "negotiated MRU" for the send direction is what the PEER
	// said it will receive (from its own CR). Update only on RCR+,
	// because that is the moment ze accepts the peer's options.
	//
	// It runs before the transition rather than after it because the Opened
	// branch inside applyTransition reads negotiatedMRU, and no FSM action
	// does.
	if ev == LCPEventRCRPlus {
		mru, present := lookupMRUOption(peerOpts)
		if !present {
			mru = MaxFrameLen
		}
		s.mu.Lock()
		s.negotiatedMRU = mru
		s.mu.Unlock()
	}
	// The peer's Magic-Number is the one ze acknowledges here, and it is
	// zero while the peer requests none: RFC 1661 Section 6.4 holds every
	// Echo packet the peer sends to that value.
	if ev == LCPEventRCRPlus {
		s.peerMagic, _ = lookupMagicNumber(peerOpts)
	}
	// RFC 1661 Section 5.8: the option is "successfully negotiated" when
	// the peer acknowledges the Configure-Request that carried it. The
	// Ack echoes ze's options, so option 5 in it is that acknowledgement.
	if ev == LCPEventRCA {
		s.magicNegotiated = !s.lcpRejectedMagic && s.magic != 0
	}

	return s.applyTransition(cur, tr, pkt)
}

// redrawMagicOnNak chooses a new Magic-Number when the peer Configure-Naks
// the one ze requested.
//
// RFC 1661 Section 6.4: "If the Magic-Number is equal to the one sent in
// the last Configure-Nak, the possibility of a looped-back link is
// increased, and a new Magic-Number MUST be chosen. In either case, a new
// Configure-Request SHOULD be sent with the new Magic-Number." Both arms
// end in a new value, so every Nak of option 5 draws one; the resent
// Configure-Request (scr on the RCN edge) then carries it.
func (s *pppSession) redrawMagicOnNak(nak LCPPacket) bool {
	opts, err := ParseLCPOptions(nak.Data)
	if err != nil {
		return true
	}
	if _, ok := lookupMagicNumber(opts); !ok {
		return true
	}
	// Bound repeated draws even if the entropy source keeps returning
	// the previous value. A failure must end negotiation, since another
	// Configure-Request with that value would not satisfy the Nak.
	for range magicDrawMaxAttempts {
		mag, err := generateMagic()
		if err != nil {
			var tb textbuf.Buffer
			s.fail(tb.Str("magic-rand after Configure-Nak: ").Err(err).String())
			return false
		}
		if mag == s.magic {
			continue
		}
		s.logger.Debug("ppp: Magic-Number redrawn after Configure-Nak",
			"old", s.magic, "new", mag)
		s.magic = mag
		return true
	}
	s.fail("magic-rand repeated the previous Magic-Number after Configure-Nak")
	return false
}

// transmitMagic is the Magic-Number ze puts in an Echo-Request, Echo-Reply
// or Discard-Request. RFC 1661 Section 5.8: "Until the Magic-Number
// Configuration Option has been successfully negotiated, the Magic-Number
// MUST be transmitted as zero.".
func (s *pppSession) transmitMagic() uint32 {
	if !s.magicNegotiated {
		return 0
	}
	return s.magic
}

// applyTransition runs one FSM transition's actions, commits the new state,
// and reports whether the session must end. Every LCP event shares this tail:
// a received packet through handleLCPPacket, and a Restart timer expiry
// through handleRestartTimeout.
//
// pkt is the packet that raised the event, which the sca, scn, sta, scj and
// ser actions echo fields from. A Timeout event has no packet and passes the
// zero value, which those actions never see because RFC 1661 Section 4.1
// pairs TO+ and TO- with scr, str and tlf alone.
func (s *pppSession) applyTransition(cur LCPState, tr lCPTransition, pkt LCPPacket) bool {
	if cur == LCPStateStopped {
		if tr.NewState == LCPStateReqSent || tr.NewState == LCPStateAckSent {
			s.resetAuthNegotiation()
		}
	}
	// Required authentication must never publish Opened without an option,
	// even if a direct StartSession caller supplied no initial method.
	if tr.NewState == LCPStateOpened {
		if s.authRequired && s.configuredAuthMethod == AuthMethodNone {
			return s.refuseAuthMethod(cur, pkt)
		}
	}
	// RFC 1661 Section 4.5.
	s.updateLCPConvergence(cur, tr.NewState)
	// RFC 1661 Section 4.4, Initialize-Restart-Count: "This action sets the
	// Restart counter to the appropriate value (Max-Terminate or
	// Max-Configure)." The Section 4.1 table pairs irc with either str or
	// scr inside one transition, so the companion action is what names which
	// of the two values the counter takes.
	restartMax := s.lcpConfig.MaxConfigure()
	for _, act := range tr.Actions {
		if act == LCPActSTR {
			restartMax = s.lcpConfig.MaxTerminate()
		}
	}

	for _, act := range tr.Actions {
		if act == LCPActIRC {
			// RFC 1661 Section 4.4, Initialize-Restart-Count: "This action
			// sets the Restart counter to the appropriate value
			// (Max-Terminate or Max-Configure). The counter is decremented
			// for each transmission, including the first." The decrement is
			// in sendConfigureRequest and sendTerminateRequest, which are
			// the transmissions.
			s.restartCount = restartMax
			continue
		}
		if act == LCPActZRC {
			// RFC 1661 Section 4.4, Zero-Restart-Count: "This action sets
			// the Restart counter to zero." It is one of the three actions
			// Section 4.1 names as starting the Restart timer, so the next
			// expiry raises TO- and carries the automaton out of Closing or
			// Stopping: "This action enables the FSA to pause before
			// proceeding to the desired final state, allowing traffic to be
			// processed by the peer."
			s.restartCount = 0
			s.armRestartTimer()
			continue
		}
		if !s.performAction(act, pkt) {
			return true
		}
	}

	if cur != LCPStateOpened && tr.NewState == LCPStateOpened {
		if s.sessionDownSent {
			// This-Layer-Finished already told the lower layer it is no
			// longer needed (signalLayerFinished), and the L2TP or PPPoE
			// side is tearing it down. A renegotiation that completes
			// before its Down event ends here rather than starting
			// authentication and the NCPs over that transport.
			s.logger.Debug("ppp: LCP reached Opened after This-Layer-Finished; ending session")
			return true
		}
		s.mu.Lock()
		if s.negotiatedMRU == 0 {
			// Peer did not propose an MRU; PPP default per
			// RFC 1661 §6.1.
			s.negotiatedMRU = MaxFrameLen
		}
		// Normal (non-proxy) path: the effective auth method is
		// whatever configuredAuthMethod holds at Opened. Phase 8
		// mutates configuredAuthMethod when the peer Naks or Rejects
		// the Auth-Protocol option (adjustAuthOnNakOrReject runs
		// BEFORE the FSM resends CONFREQ), so by the time an Ack
		// brings us here the field already reflects what the peer
		// accepted.
		s.negotiatedAuthMethod = s.configuredAuthMethod
		mru := s.negotiatedMRU
		// LCP is Opened before authentication/NCP run. Those phases can
		// process LCP packets themselves; they must see the actual FSM
		// state and may interrupt this transition.
		s.state = LCPStateOpened
		s.mu.Unlock()
		s.stopRestartTimer()
		s.sendEvent(EventLCPUp{
			TunnelID:             s.tunnelID,
			SessionID:            s.sessionID,
			NegotiatedMRU:        mru,
			NegotiatedAuthMethod: s.configuredAuthMethod,
			ProxyLCP:             s.proxyLCPOutcome,
		})
		if !s.afterLCPOpen() {
			state := s.currentState()
			// A nested restart resumes in the main LCP loop. A nested
			// termination keeps its grace timer; never overwrite either
			// transition with the old Opened result.
			return state == LCPStateOpened || s.sessionEndedAt(state)
		}
		return false
	}

	s.mu.Lock()
	s.state = tr.NewState
	s.mu.Unlock()

	// RFC 1661 Section 4.1: "The Restart timer is stopped when transitioning
	// from any state where the timer is running to a state where the timer is
	// not running."
	if !restartTimerRuns(tr.NewState) {
		s.stopRestartTimer()
	}

	// Only stopping and stopped preserve peer-termination bookkeeping.
	//exhaustive:ignore
	switch tr.NewState {
	case LCPStateStopping:
		if pkt.Code == LCPTerminateRequest {
			s.peerTerminated = true
		}
	case LCPStateStopped:
	default:
		s.peerTerminated = false
	}

	if s.sessionEndedAt(tr.NewState) {
		var tb textbuf.Buffer
		reason := tb.Str("LCP terminated: state=").Str(tr.NewState.String()).String()
		cause := l2tpevents.TerminateCauseUserRequest
		switch {
		case s.authRefusalReason != "":
			reason = s.authRefusalReason
			cause = l2tpevents.TerminateCauseNASRequest
		case s.noNCPBoundExpired:
			// The Close closeIdleLink drove: RFC 2866 Section 5.10 value 4,
			// "Idle timer expired."
			reason = s.idleCloseReason()
			cause = l2tpevents.TerminateCauseIdleTimeout
		case s.lcpRejectFatalReason != "":
			// The RXJ- handleLCPPacket recorded: the peer rejected what LCP
			// needs, an error and no request of the user's. RFC 2866 Section
			// 5.10 value 9: "NAS detected some error (other than on the
			// port) which required ending the session."
			reason = s.lcpRejectFatalReason
			cause = l2tpevents.TerminateCauseNASError
		}
		s.sendEvent(EventSessionDown{
			TunnelID:  s.tunnelID,
			SessionID: s.sessionID,
			Reason:    reason,
			Cause:     cause,
		})
		return true
	}
	if tr.NewState == LCPStateStopped {
		s.signalLayerFinished()
	}

	return false
}

// updateLCPConvergence bounds a responsive peer's Nak loop separately from the
// RFC Restart counter. Entering a fresh negotiating lifetime renews the bound;
// replies within that lifetime do not. run MUST create and stop the owned timer.
// RFC 1661 Section 4.5: "Implementors should keep this in mind and SHOULD
// implement loop detection mechanisms or higher level timeouts."
func (s *pppSession) updateLCPConvergence(previous, next LCPState) {
	if s.lcpConvergenceTimer == nil {
		// Direct transition tests need no running session or lifetime timer.
		return
	}
	if !lcpNegotiating(next) {
		s.lcpConvergenceTimer.Stop()
		return
	}
	if lcpNegotiating(previous) {
		return
	}
	limit := 2 * time.Duration(s.lcpConfig.MaxConfigure()) * s.lcpConfig.RestartTimer()
	s.lcpConvergenceTimer.Reset(limit)
}

// lcpNegotiating classifies the three negotiation states; other states have
// no convergence deadline because authentication and termination own theirs.
func lcpNegotiating(state LCPState) bool {
	// This predicate intentionally handles only the negotiating-state subset.
	//exhaustive:ignore
	switch state {
	case LCPStateReqSent, LCPStateAckRcvd, LCPStateAckSent:
		return true
	default:
		return false
	}
}

// signalLayerFinished performs This-Layer-Finished for a Stopped the peer's
// Terminate-Request led to, which sessionEndedAt keeps running. The lower
// layer learns it through the one EventSessionDown the session emits: the
// L2TP reactor answers with a CDN, the PPPoE subsystem with a PADT, and the
// teardown that follows closes sessStop or the channel, which is the Down
// event that ends run(). Until then the session still answers a
// Configure-Request (sessionEndedAt), so the wait in Stopped is bounded by
// the lower layer's teardown and never by the peer.
func (s *pppSession) signalLayerFinished() {
	// RFC 1661 Section 4.4, This-Layer-Finished: "This action indicates to
	// the lower layers that the automaton is entering the Initial, Closed or
	// Stopped states, and the lower layer is no longer needed for the link.
	// The lower layer SHOULD respond with a Down event when the lower layer
	// has terminated."
	// RFC 2866 Section 5.10 value 1: "User requested termination of service,
	// for example with LCP Terminate or by logging out."
	s.sendEvent(EventSessionDown{
		TunnelID:  s.tunnelID,
		SessionID: s.sessionID,
		Reason:    "LCP terminated by peer: stopped, lower layer released",
		Cause:     l2tpevents.TerminateCauseUserRequest,
	})
}

// sessionEndedAt reports whether reaching state ends the session. Closed
// always does, because only ze's own Close leads there. Stopped does unless
// the peer's Terminate-Request led there: that Stopped signals
// This-Layer-Finished (signalLayerFinished) and keeps the session answering
// the peer's next Configure-Request, which LCPDoTransition answers with irc,
// scr and sca or scn, until the lower layer's teardown (sessStop) or the
// channel closing ends it.
func (s *pppSession) sessionEndedAt(state LCPState) bool {
	if state == LCPStateClosed {
		return true
	}
	if state != LCPStateStopped {
		return false
	}
	// RFC 1661 Section 4.3, RTR: "The implementation MUST be prepared to
	// receive a new Configure-Request without network administrator
	// intervention."
	return !s.peerTerminated
}

// terminateCause returns the RFC 2866 Section 5.10 cause for a teardown that
// ends the session while it waits in Stopped after the peer's
// Terminate-Request, and fallback otherwise.
func (s *pppSession) terminateCause(fallback l2tpevents.TerminateCause) l2tpevents.TerminateCause {
	if s.peerTerminated {
		// RFC 2866 Section 5.10 value 1: "User requested termination of
		// service, for example with LCP Terminate or by logging out."
		return l2tpevents.TerminateCauseUserRequest
	}
	return fallback
}

// logFSMNoOp emits a debug log for an FSM (state, event) combination
// that produced no transition. Includes a hex sample of the offending
// packet so operators can diagnose buggy or hostile peers.
//
// Guarded by an Enabled() check so the sample buffer + hex encoding
// are only computed when debug-level logging is actually on. FSM
// no-ops should be rare but a hostile peer could spam them.
func (s *pppSession) logFSMNoOp(state LCPState, ev lCPEvent, pkt LCPPacket) {
	if !s.logger.Enabled(context.Background(), slog.LevelDebug) {
		return
	}
	sample := make([]byte, 4+len(pkt.Data))
	sample[0] = pkt.Code
	sample[1] = pkt.Identifier
	binary.BigEndian.PutUint16(sample[2:4], uint16(4+len(pkt.Data)))
	copy(sample[4:], pkt.Data)
	if len(sample) > 32 {
		sample = sample[:32]
	}
	s.logger.Debug("ppp: LCP no-op transition",
		"state", state.String(),
		"event", strconv.Itoa(int(ev)),
		"code", LCPCodeName(pkt.Code),
		"identifier", pkt.Identifier,
		"sample-hex", hex.EncodeToString(sample),
	)
}

// performAction translates one LCP FSM action into wire I/O on the
// chan fd. Returns false when a required reply cannot be sent; the caller
// must abort before committing the transition's next state.
func (s *pppSession) performAction(act LCPAction, current LCPPacket) bool {
	switch act {
	case LCPActSCR:
		return s.sendConfigureRequest()
	case LCPActSCA:
		return s.sendConfigureAck(current)
	case LCPActSCN:
		return s.sendConfigureNakOrReject(current)
	case LCPActSTR:
		return s.sendTerminateRequest()
	case LCPActSTA:
		return s.sendTerminateAck(current)
	case LCPActSCJ:
		return s.sendCodeReject(current)
	case LCPActSER:
		if current.Code != LCPEchoRequest {
			return true
		}
		if s.echoReplyRejected {
			// RFC 1661 Section 4.3: "The implementation MUST stop sending
			// the offending packet type." The peer Code-Rejected an
			// Echo-Reply (stopRejectedCode), so its Echo-Request goes
			// unanswered.
			return true
		}
		return s.sendEchoReply(current)
	case LCPActIRC, LCPActZRC:
		// The Restart counter actions touch no wire. applyTransition
		// performs them, because the value irc loads depends on the
		// companion action in the same transition and only the transition
		// carries that.
		return true
	case LCPActTLD:
		s.retireProviderRequests()
		if s.released.Load() {
			return false
		}
		s.papReplyCode = 0
		ev := EventLCPDown{
			Control:      s.control,
			TunnelID:     s.tunnelID,
			SessionID:    s.sessionID,
			Reason:       "LCP left Opened",
			NetworkPhase: s.ncpStarted,
		}
		if !s.resetNCP() {
			return false
		}
		if ev.NetworkPhase {
			ev.done = make(chan struct{})
		}
		s.sendEvent(ev)
		if !ev.NetworkPhase {
			return true
		}
		// The transport MUST finish the old network lifetime's cleanup and
		// acknowledge it before PPP can authenticate the replacement. PPP
		// MUST wait here so delayed cleanup cannot erase new session state.
		select {
		case <-ev.done:
			return true
		case <-s.stopCh:
			return false
		case <-s.sessStop:
			return false
		}
	case LCPActTLU, LCPActTLS, LCPActTLF:
		// "Notify upper layers" is handled inline in applyTransition via the
		// state-transition check.
		return true
	default:
		panic("BUG: unknown LCP action")
	}
}

// armRestartTimer starts the Restart timer, so that the next expiry raises a
// Timeout event. RFC 1661 Section 4.1: "Only the Send-Configure-Request,
// Send-Terminate-Request and Zero-Restart-Count actions start or re-start the
// Restart timer", so only those three call it.
//
// Runs on the session goroutine, which is also the goroutine that receives
// from the timer channel, so a Stop that races an expiry cannot deliver a
// stale tick: Go 1.23 and later make Stop and Reset drop an unreceived value.
func (s *pppSession) armRestartTimer() {
	if s.restartTimer == nil {
		return
	}
	s.restartTimer.Stop()
	s.restartTimer.Reset(s.lcpConfig.RestartTimer())
}

// spendRestartCount books one transmission of a Configure-Request or a
// Terminate-Request against the Restart counter and restarts the Restart
// timer to guard it.
//
// RFC 1661 Section 4.4, Initialize-Restart-Count: "The counter is decremented
// for each transmission, including the first." Section 4.4, Send-Configure-
// Request: "The Restart timer is started when the Configure-Request packet is
// transmitted, to guard against packet loss."
//
// The counter floors at zero rather than going negative, because zero is what
// the Timeout event reads to pick TO- and nothing else reads the value.
func (s *pppSession) spendRestartCount() {
	if s.restartCount > 0 {
		s.restartCount--
	}
	s.armRestartTimer()
}

// stopRestartTimer stops the Restart timer, for the rule RFC 1661 Section 4.1
// states: "The Restart timer is stopped when transitioning from any state
// where the timer is running to a state where the timer is not running".
func (s *pppSession) stopRestartTimer() {
	if s.restartTimer == nil {
		return
	}
	s.restartTimer.Stop()
}

// restartTimerRuns reports whether the Restart timer runs in this state.
// RFC 1661 Section 4.1: "The states in which the Restart timer is running are
// identifiable by the presence of TO events", which the table gives to
// Closing, Stopping, Req-Sent, Ack-Rcvd and Ack-Sent.
func restartTimerRuns(state LCPState) bool {
	// Predicate selecting states whose restart timer runs.
	//exhaustive:ignore
	switch state {
	case LCPStateClosing, LCPStateStopping, LCPStateReqSent, LCPStateAckRcvd, LCPStateAckSent:
		return true
	case LCPStateInitial, LCPStateStarting, LCPStateClosed, LCPStateStopped, LCPStateOpened:
		return false
	}
	return false
}

// handleRestartTimeout raises the Timeout event the expired Restart timer
// stands for, and reports whether the session must end.
//
// RFC 1661 Section 4.3, Timeout: "The TO+ event indicates that the Restart
// counter continues to be greater than zero, which triggers the corresponding
// Configure-Request or Terminate-Request packet to be retransmitted. The TO-
// event indicates that the Restart counter is not greater than zero, and no
// more packets need to be retransmitted."
//
// The counter is what tells the two apart, which is why zrc exists: a peer's
// Terminate-Request puts Opened into Stopping with the counter at zero
// (Section 4.1, "tld,zrc,sta/5"), so the first expiry after it is TO-, and
// Stopping answers TO- with "tlf/3". The session then waits in Stopped for the
// peer's next Configure-Request (sessionEndedAt), and the L2TP teardown ends it.
func (s *pppSession) handleRestartTimeout() bool {
	cur := s.currentState()
	ev := LCPEventTOMinus
	if s.restartCount > 0 {
		ev = LCPEventTOPlus
	}
	tr := LCPDoTransition(cur, ev)
	if tr.NewState == cur && len(tr.Actions) == 0 {
		// A state with no TO edge left the timer armed. Stop it rather than
		// letting it fire again on a state that has no answer for it.
		s.logger.Debug("ppp: Restart timer expired in a state with no Timeout edge",
			"state", cur.String(), "restart-count", s.restartCount)
		s.stopRestartTimer()
		return false
	}
	s.logger.Debug("ppp: LCP Restart timer expired",
		"state", cur.String(), "restart-count", s.restartCount)
	return s.applyTransition(cur, tr, LCPPacket{})
}

func (s *pppSession) sendConfigureRequest() bool {
	if !s.authNegotiationStarted {
		s.initialAuthMethod = s.configuredAuthMethod
		s.authNegotiationStarted = true
	}
	s.spendRestartCount()
	// A Configure-Request opens a negotiation the peer has not yet
	// acknowledged, so the Magic-Number is unnegotiated until its Ack
	// arrives (RFC 1661 Section 5.8).
	s.magicNegotiated = false
	s.papReplyCode = 0
	mru := s.localMRU
	if mru == 0 {
		mru = s.maxMRU
	}
	if s.lcpRejectedMRU {
		mru = 0
	}
	magic := s.magic
	if s.lcpRejectedMagic {
		magic = 0
	}
	authProto, authData := authMethodToLCPOptions(s.configuredAuthMethod)
	opts := BuildLocalConfigRequest(LCPOptions{
		MRU:       mru,
		Magic:     magic,
		AuthProto: authProto,
		AuthData:  authData,
	})
	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ProtoLCP, nil)
	dataOff := off + lcpHeaderLen
	dataLen, fits := WriteLCPOptions(buf, dataOff, opts)
	if !fits {
		// Ze chose every one of these options, and BuildLocalConfigRequest
		// emits at most six of them at seven octets each, so a frame cannot
		// be too small for them. Saying so beats sending a Configure-Request
		// that offers a prefix of what ze wants: the negotiation would then
		// settle on terms ze never chose.
		s.fail("LCP Configure-Request options do not fit a frame")
		return false
	}
	if dataLen > len(s.lastLCPRequest) {
		s.fail("LCP Configure-Request exceeds the local option bound")
		return false
	}
	data := buf[dataOff : dataOff+dataLen]
	// RFC 1661 Section 5.1: "The Identifier field MUST be changed
	// whenever the contents of the Options field changes, and whenever a
	// valid reply has been received for a previous request." An unchanged
	// timeout retransmission retains its Identifier.
	if !s.lastLCPRequestSent || s.lastLCPReplyReceived ||
		!bytes.Equal(data, s.lastLCPRequest[:s.lastLCPRequestLen]) {
		s.lastLCPRequestID++
	}
	s.lastLCPRequestLen = copy(s.lastLCPRequest[:], data)
	s.lastLCPRequestSent = true
	s.lastLCPReplyReceived = false
	off += WriteLCPPacket(buf, off, LCPConfigureRequest, s.lastLCPRequestID, buf[dataOff:dataOff+dataLen])
	if !s.writeFrame(buf[:off]) {
		return false
	}
	if s.configuredAuthMethod != AuthMethodNone {
		s.authMethodsTried |= 1 << s.configuredAuthMethod
	}
	return true
}

func (s *pppSession) sendConfigureAck(req LCPPacket) bool {
	// An Ack must preserve every option. Failure must stop the automaton
	// before it commits AckSent or Opened for an acknowledgment never sent.
	limit := s.transmitMRU()
	if lcpHeaderLen+len(req.Data) > int(limit) {
		var reason textbuf.Buffer
		s.fail(reason.Str("LCP Configure-Ack id ").Uint8(req.Identifier).
			Str(" length ").Int(int64(lcpHeaderLen + len(req.Data))).
			Str(" exceeds session transmit MRU ").Uint16(limit).String())
		return false
	}
	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ProtoLCP, nil)
	off += WriteLCPPacket(buf, off, LCPConfigureAck, req.Identifier, req.Data)
	if !s.writeFrame(buf[:off]) {
		return false
	}
	s.proxyCapture.noteRequestAnswered(req)
	s.proxyCapture.noteRequestAcked(req)
	// RFC 1661 Section 4.6 counts Naks "without sending a Configure-Ack".
	// Only a successfully transmitted Ack starts a fresh Nak budget.
	s.lcpNakCount = 0
	return true
}

// LCPNakOrReject picks the reply a Configure-Request ze judged unacceptable
// earns, and the options that reply carries.
//
// RFC 1661 Section 5.4 orders Reject over Nak: "If some Configuration Options
// received in a Configure-Request are not recognizable or are not acceptable
// for negotiation (as configured by a network administrator), then the
// implementation MUST transmit a Configure-Reject." That MUST outranks the
// Configure-Nak Section 6 only says an invalid Length SHOULD draw, so a
// request carrying one of each is answered with the Reject alone.
//
// The options read before an invalid Length are therefore judged first. They
// are the whole list on a clean walk, and the prefix that precedes the fault
// otherwise -- WalkLCPOptions cannot step over an option whose own Length is
// impossible, so an unrecognized Type after the fault is one ze never sees.
//
// RFC 1661 Section 6: "(None of the Configuration Options in this
// specification can be listed more than once.)" The prefix can carry an
// earlier instance of the faulting option, so the entry the fault earns is
// added only when the reply does not already name that Type.
//
// reply is false when the walk earned no entry at all. RFC 1661 Section 5.3:
// "The Options field is filled with only the unacceptable Configuration
// Options from the Configure-Request." A Configure-Nak with an empty Options
// field carries no unacceptable option, so it is a packet neither Section 5.3
// nor Section 5.4 describes, and the caller sends nothing instead. code and
// opts are then the zero value and MUST NOT be transmitted.
func LCPNakOrReject(w LCPOptionWalk, policy LCPNegPolicy) (code uint8, opts []LCPOption, reply bool) {
	_, naks, rejects := NegotiatePeerOptions(w.Options, policy)

	if w.Fault == LCPOptionsBadLength {
		// RFC 1661 Section 6: "If a negotiable Configuration Option is
		// received in a Configure-Request, but with an invalid or
		// unrecognized Length, a Configure-Nak SHOULD be transmitted which
		// includes the desired Configuration Option with an appropriate
		// Length and Data."
		want, ok := desiredLCPOption(w.FaultOpt, policy)
		if ok {
			naks = appendUnlessListed(naks, want)
		}
		if !ok {
			// Ze negotiates no value for this Type, so it has nothing to put
			// in a Nak. RFC 1661 Section 5.4 takes the option instead, and it
			// travels exactly as it arrived, malformed Length octet included:
			// "The Options field is filled with only the unacceptable
			// Configuration Options from the Configure-Request. All
			// recognizable and negotiable Configuration Options are filtered
			// out of the Configure-Reject, but otherwise the Configuration
			// Options MUST NOT be reordered or modified in any way."
			//
			// Naming the Type at the shortest well-formed Length instead
			// would put an option in the reply that the peer never sent, for
			// the one Length the peer cannot have meant. Section 5.4 answers
			// that trade already, and the peer that chose the Length is the
			// party that has to make sense of the reply.
			//
			// WalkLCPOptions sets FaultRaw on every LCPOptionsBadLength walk.
			// A walk assembled without it can only produce the modified
			// option Section 5.4 forbids, so it produces no entry at all.
			if len(w.FaultRaw) >= lcpOptHeaderLen {
				rejects = appendUnlessListed(rejects, LCPOption{Type: w.FaultOpt, Raw: w.FaultRaw})
			}
		}
	}

	if len(rejects) > 0 {
		return LCPConfigureReject, rejects, true
	}
	if len(naks) > 0 {
		return LCPConfigureNak, naks, true
	}
	// Every option ze refused produced an entry above, so an empty pair means
	// ze refused nothing it can name. That is reachable only from a walk
	// assembled without FaultRaw, which the branch above leaves without an
	// entry rather than modify the option Section 5.4 says it MUST NOT
	// modify. The reply it would build is an empty Configure-Nak, and the
	// answer to "nothing to say" is silence.
	return 0, nil, false
}

// appendUnlessListed adds opt to the reply list, and returns the list
// unchanged when it already carries opt's Type.
//
// RFC 1661 Section 6: "(None of the Configuration Options in this
// specification can be listed more than once.)" That holds of the reply as
// well as of the request, so a Configure-Nak or Configure-Reject ze builds
// names each Type at most once.
//
// The entry already listed is the one kept. It came from an instance of the
// option ze could read, so a Nak entry holds a value ze accepts and a Reject
// entry holds the whole option that earned the refusal, where the entry the
// fault would add carries only the two header octets its own Length left
// readable.
func appendUnlessListed(list []LCPOption, opt LCPOption) []LCPOption {
	for _, listed := range list {
		if listed.Type == opt.Type {
			return list
		}
	}
	return append(list, opt)
}

func (s *pppSession) sendConfigureNakOrReject(req LCPPacket) bool {
	s.proxyCapture.noteRequestAnswered(req)
	w := WalkLCPOptions(req.Data)
	if w.Fault.Discards() {
		// Not reachable from handleLCPPacket, which returns on that verdict
		// before the automaton fires this action. RFC 1661 Section 6 answers
		// such a packet with silence, so the fail-closed answer here is the
		// same silence rather than a reply built from options ze could not
		// read.
		s.logger.Debug("ppp: LCP reply suppressed, options do not fit the packet",
			"id", req.Identifier,
			"len", len(req.Data))
		return true
	}

	policy := s.negPolicy()
	code, opts, reply := LCPNakOrReject(w, policy)
	if !reply {
		// Ze judged the request unacceptable and holds no option to name in
		// the answer. RFC 1661 Section 5.3 fills the Options field with
		// "only the unacceptable Configuration Options from the
		// Configure-Request", so an empty Configure-Nak is a packet the
		// section does not describe. Ze sends nothing and says so, and the
		// peer's retransmission timer drives the negotiation.
		s.logger.Warn("ppp: LCP reply suppressed, no option to carry in it",
			"id", req.Identifier,
			"len", len(req.Data))
		return true
	}
	if code == LCPConfigureNak {
		if s.lcpNakCount >= s.lcpConfig.MaxFailure() {
			// RFC 1661 Section 4.6: "Any further Configure-Nak packets for
			// peer requested options are converted to Configure-Reject
			// packets, and locally desired options are no longer appended."
			// A Reject copies the peer's option, never the Nak's suggestion.
			// RFC 1661 Section 5.4.
			opts = LCPRejectNakOptions(w, opts, policy)
			code = LCPConfigureReject
		}
	}

	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ProtoLCP, nil)
	dataOff := off + lcpHeaderLen
	limit := s.transmitMRU()
	replyBuf := buf[:min(frameLen(int(limit)), len(buf))]
	dataLen, fits := WriteLCPOptions(replyBuf, dataOff, opts)
	if !fits {
		// The peer sizes this reply, and one refused option can draw more
		// octets than it occupied: a Magic-Number at Length 2 is answered by
		// the six RFC 1661 Section 6.4 gives it. A frame filled with those
		// asks for a reply three times its own size, which no frame holds.
		//
		// RFC 1661 Section 5.4 fills the Options field with "only the
		// unacceptable Configuration Options from the Configure-Request", so
		// a reply carrying the prefix that fit is a packet the section does
		// not describe. Fail before the automaton commits a transition that
		// assumes this required response was transmitted.
		var reason textbuf.Buffer
		s.fail(reason.Str("LCP ").Str(LCPCodeName(code)).Str(" id ").Uint8(req.Identifier).
			Str(" complete options exceed session transmit MRU ").Uint16(limit).String())
		return false
	}
	off += WriteLCPPacket(buf, off, code, req.Identifier, buf[dataOff:dataOff+dataLen])
	if !s.writeFrame(buf[:off]) {
		return false
	}
	if code == LCPConfigureNak {
		s.lcpNakCount++
	}
	return true
}

// LCPRejectNakOptions converts an actual Nak's suggestions back to the original
// peer-requested instances. Callers MUST supply the walk and policy used for
// that Nak; suggestions may also include their own authentication-policy Nak.
// The received frame bounds both walks. Each Type is returned at most once.
//
// RFC 1661 Section 4.6: "Any further Configure-Nak packets for peer requested
// options are converted to Configure-Reject packets, and locally desired
// options are no longer appended."
// RFC 1661 Section 5.4: "The Options field is filled with only the unacceptable
// Configuration Options from the Configure-Request." The returned options
// retain the original bytes and order required by the same section.
func LCPRejectNakOptions(w LCPOptionWalk, suggestions []LCPOption, policy LCPNegPolicy) []LCPOption {
	var refused [4]uint64
	for _, opt := range suggestions {
		refused[opt.Type/64] |= uint64(1) << (opt.Type % 64)
	}
	var listed [4]uint64
	var rejects []LCPOption
	for _, opt := range w.Options {
		word, bit := opt.Type/64, uint64(1)<<(opt.Type%64)
		if refused[word]&bit == 0 {
			continue
		}
		if listed[word]&bit != 0 {
			continue
		}
		listed[word] |= bit
		if w.Fault == LCPOptionsBadLength {
			if opt.Type == w.FaultOpt {
				// The trailing malformed instance, rather than an acceptable
				// earlier instance of this Type, may have caused the Nak.
				// RFC 1661 Section 6.
				outcome, _ := negotiatePeerOption(opt, policy)
				if outcome == negAck {
					continue
				}
			}
		}
		rejects = append(rejects, opt)
		refused[word] &^= bit
	}
	if w.Fault == LCPOptionsBadLength {
		if refused[w.FaultOpt/64]&(uint64(1)<<(w.FaultOpt%64)) != 0 {
			if len(w.FaultRaw) >= lcpOptHeaderLen {
				rejects = append(rejects, LCPOption{Type: w.FaultOpt, Raw: w.FaultRaw})
			}
		}
	}
	return rejects
}

// sendTerminateRequest emits a Terminate-Request with a fresh Identifier.
//
// RFC 1661 Section 5.5: "On transmission, the Identifier field MUST be
// changed whenever the content of the Data field changes, and whenever a
// valid reply has been received for a previous request." Every send draws
// the next value, which covers both cases and the retransmission that
// Section 5.5 allows to keep or change it.
func (s *pppSession) sendTerminateRequest() bool {
	s.spendRestartCount()
	s.terminateID++
	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ProtoLCP, nil)
	off += WriteLCPPacket(buf, off, LCPTerminateRequest, s.terminateID, nil)
	return s.writeFrame(buf[:off])
}

func (s *pppSession) sendTerminateAck(req LCPPacket) bool {
	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ProtoLCP, nil)
	off += WriteLCPPacket(buf, off, LCPTerminateAck, req.Identifier, nil)
	return s.writeFrame(buf[:off])
}

// sendCodeReject replies with an LCP Code-Reject when an unknown or
// unsupported Code is received in a valid LCP packet.
//
// RFC 1661 Section 5.6: "Reception of a LCP packet with an unknown Code
// indicates that the peer is operating with a different version. This MUST
// be reported back to the sender of the unknown Code by transmitting a
// Code-Reject."
func (s *pppSession) sendCodeReject(req LCPPacket) bool {
	// RFC 1661 Section 5.6: "The Identifier field MUST be changed for each
	// Code-Reject sent."
	s.lcpCodeRejectID++
	return s.writeCodeReject(ProtoLCP, s.lcpCodeRejectID, req)
}

// writeCodeReject writes a Code-Reject of req on proto, with Identifier id,
// and returns false on a fatal write error. LCP (sendCodeReject) and the
// NCPs (sendNCPCodeReject) share it: RFC 1332 Section 2 and RFC 5072
// Section 3 give IPCP and IPv6CP the LCP packet exchange, Code-Reject
// included. The caller draws id from its own protocol's counter.
//
// Wire format (RFC 1661 Section 5.6), offsets from the start of the
// Code-Reject, which itself starts 2 octets into the frame:
//
//	 0                   1                   2                   3
//	 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|     Code = 7  |  Identifier   |            Length             |
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	| Rejected-Packet ...
//	+-+-+-+-+-+-+-+-+
//
// Octet 0 Code, 1 Identifier, 2-3 Length, 4 onward the Rejected-Packet: the
// rejected packet's own Code (octet 4), Identifier (5), Length (6-7) and Data
// (8 onward). "The Rejected-Packet field contains a copy of the LCP packet
// which is being rejected. It begins with the Information field, and does not
// include any Data Link Layer headers nor an FCS." The rejected packet's
// Length is written as it arrived, so a cut copy still tells the peer how
// long the packet was.
func (s *pppSession) writeCodeReject(proto uint16, id uint8, req LCPPacket) bool {
	// RFC 1661 Section 5.6: "The Rejected-Packet MUST be truncated to comply
	// with the peer's established MRU." The MRU bounds the Information field
	// of the frame ze sends. The admitted session cap and pooled buffer can
	// impose a smaller limit, so the copy of the rejected Data takes their
	// minimum, less the octets that precede it: the 2-octet Protocol field, the
	// 4-octet Code-Reject header and the rejected packet's own 4-octet header.
	mru := s.transmitMRU()
	limit := min(frameLen(int(mru)), MaxFrameBufLen)
	room := max(limit-frameLen(lcpHeaderLen+lcpHeaderLen), 0)
	rejected := req.Data
	if len(rejected) > room {
		rejected = rejected[:room]
	}

	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, proto, nil)
	// The Rejected-Packet is written where WriteLCPPacket needs the Data
	// field, so no scratch buffer is allocated. The copy that call then makes
	// is onto itself and moves nothing.
	data := buf[off+lcpHeaderLen:]
	data[0] = req.Code
	data[1] = req.Identifier
	binary.BigEndian.PutUint16(data[2:4], uint16(lcpHeaderLen+len(req.Data)))
	n := lcpHeaderLen + copy(data[lcpHeaderLen:], rejected)
	off += WriteLCPPacket(buf, off, LCPCodeReject, id, data[:n])
	return s.writeFrame(buf[:off])
}

func (s *pppSession) sendEchoReply(req LCPPacket) bool {
	buf := getFrameBuf()
	defer putFrameBuf(buf)
	// Keep the reply's LCP header and local Magic-Number within the same
	// effective transmit limit as data and rejection packets. Fitting peer
	// data remains unchanged; only the tail beyond the link limit is cut.
	limit := min(int(s.transmitMRU()), MaxFrameLen)
	data := req.Data[:min(len(req.Data), max(limit-lcpHeaderLen, 4))]
	off := WriteFrame(buf, 0, ProtoLCP, nil)
	off += BuildLCPEchoReply(buf, off, req.Identifier, s.transmitMagic(), data)
	return s.writeFrame(buf[:off])
}

// sendEchoRequest emits an Echo-Request with a fresh Identifier.
//
// RFC 1661 Section 5.8: "On transmission, the Identifier field MUST be
// changed whenever the content of the Data field changes, and whenever a
// valid reply has been received for a previous request." Every send draws
// the next value, which covers both cases.
func (s *pppSession) sendEchoRequest() bool {
	s.echoID++
	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ProtoLCP, nil)
	off += WriteLCPEcho(buf, off, LCPEchoRequest, s.echoID, s.transmitMagic(), nil)
	s.lastEchoSentAt = time.Now()
	return s.writeFrame(buf[:off])
}

func (s *pppSession) writeFrame(frame []byte) bool {
	// Every producer must build a complete packet within the effective
	// limit. This final boundary refuses a missed writer without cutting a
	// digest, a structured authentication result or configuration options.
	if len(frame) > frameLen(int(s.transmitMRU())) {
		s.fail("PPP writer produced a frame larger than the session transmit MRU")
		return false
	}
	n, err := s.chanFile.Write(frame)
	if err == nil && n != len(frame) {
		err = io.ErrShortWrite
	}
	if err != nil {
		var tb textbuf.Buffer
		s.fail(tb.Str("chan fd write: ").Err(err).String())
		return false
	}
	return true
}

// retryRetiredIPv6Cleanup is called only with nativeReleaseMu held after run
// returned. Driver/table locks are absent across provider cleanup/callbacks.
// Receipt callbacks MUST return busy/failure instead of joining this worker.
func (s *pppSession) retryRetiredIPv6Cleanup(ctx context.Context, unitReleased bool) error {
	if s.driver == nil {
		return nil
	}
	s.driver.mu.Lock()
	pending, previous := s.ownerServicePending, s.ownerServiceErr
	s.driver.mu.Unlock()
	if !pending {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	service := s.ipv6Svc
	s.mu.Unlock()
	if service == nil {
		cause := errors.Join(previous, errors.New("retired PPP IPv6 cleanup owner is unavailable"))
		s.setOwnerServiceState(true, cause)
		return cause
	}
	if err := service.retryStoppedCleanup(ctx, unitReleased); err != nil {
		if ctx.Err() != nil {
			return err
		}
		if errors.Is(err, ErrCleanupInProgress) {
			return err
		}
		// Retain the original actual child cause; contention/deadlines never
		// replace it. The latest retry error is still returned to this caller.
		cause := previous
		if cause == nil {
			cause = err
		}
		s.setOwnerServiceState(true, cause)
		return errors.Join(cause, err)
	}
	s.mu.Lock()
	if s.ipv6Svc == service {
		s.ipv6Svc = nil
	}
	s.mu.Unlock()
	s.setOwnerServiceState(false, nil)
	s.clearIPv6ChildCleanupFailure()
	return nil
}
