// Design: docs/research/l2tpv2-ze-integration.md -- per-session goroutine main loop
// Related: session.go -- pppSession struct
// Related: manager.go -- Driver that spawns these goroutines
// Related: ppp_fsm.go -- pure FSM transition function
// Related: ncp.go -- NCP phase driver invoked from afterLCPOpen

package ppp

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"time"
)

// Defaults applied per session when StartSession leaves a field zero.
const (
	defaultEchoInterval  = 10 * time.Second
	defaultEchoFailures  = 3
	defaultNegoTimeout   = 30 * time.Second
	defaultAuthTimeout   = 30 * time.Second
	pppFrameReadBufSize  = MaxFrameLen
	pppFrameWriteBufSize = MaxFrameLen
	magicDrawMaxAttempts = 8

	// minIPMTU is the IPv4 minimum MTU per RFC 1122 §3.3.2. PPP's
	// MRU floor (RFC 1661 §6.1) is 64 -- which would yield a kernel
	// MTU of 60 if naively used. We clamp to ensure the netlink
	// SetMTU call never asks the kernel for a sub-IP-minimum MTU.
	//
	// IPv6-on-PPP (spec-l2tp-6c-ncp's IPv6CP) requires MTU >= 1280
	// per RFC 8200 §5. Spec 6c revisits this: when a session has
	// IPv6 enabled, this floor must be raised to 1280.
	minIPMTU = 68

	// pppEncapOverhead is the bytes deducted from MRU to compute IP
	// MTU: PPP protocol field 2 + framing 2.
	pppEncapOverhead = 4
)

// frameBufPool supplies MaxFrameLen-byte buffers used for both reads
// from the chan fd and wire-format writes. Eliminates per-packet
// allocation in the hot path.
var frameBufPool = sync.Pool{
	New: func() any {
		b := make([]byte, MaxFrameLen)
		return &b
	},
}

func getFrameBuf() []byte {
	p, ok := frameBufPool.Get().(*[]byte)
	if !ok {
		panic("BUG: frameBufPool produced non-*[]byte")
	}
	return (*p)[:MaxFrameLen]
}

func putFrameBuf(b []byte) {
	if cap(b) < MaxFrameLen {
		return
	}
	full := b[:MaxFrameLen]
	frameBufPool.Put(&full)
}

// drainFramesToPool returns any frame buffers still queued in the
// channel back to frameBufPool. Called via defer on session exit so
// readFrames-produced buffers don't leak. Reads everything currently
// buffered (channel is buffered size 4) and returns.
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
// phases. The main loop differentiates phases via timer channel
// enablement (negoTimerC vs echoTickerC).
func (s *pppSession) run(start StartSession) {
	// Teardown order must be: close chanFile (unblocks readFrames
	// reads) -> wait for readFrames to exit -> drain leftover frames
	// back to the pool. A single defer guarantees the sequence;
	// multiple defers would fire LIFO in a way that lets readFrames
	// keep producing frames after the drain already ran.
	var (
		framesCh chan []byte
		readerWG sync.WaitGroup
	)
	defer func() {
		s.teardownNCPResources()
		if s.chanFile != nil {
			_ = s.chanFile.Close() //nolint:errcheck // exit cleanup
		}
		readerWG.Wait()
		if framesCh != nil {
			drainFramesToPool(framesCh)
		}
	}()

	mag, err := generateMagic()
	if err != nil {
		s.fail("magic-rand: " + err.Error())
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
		s.readFrames(frames, readDone)
	})

	// RFC 2661 Section 18: when the peer LAC provides Initial-Received-
	// LCP-CONFREQ, Last-Sent-LCP-CONFREQ, and Last-Received-LCP-CONFREQ
	// AVPs in ICCN, the LNS MAY skip LCP negotiation and use the
	// proxied options as if they had been negotiated directly.
	proxy, perr := EvaluateProxyLCP(
		start.ProxyLCPInitialRecv, start.ProxyLCPLastSent, start.ProxyLCPLastRecv,
	)
	isProxy := perr == nil

	if isProxy {
		// proxy.AuthProto is zero when no Auth-Protocol option was
		// carried in the LAC's Last-Sent CONFREQ; authMethodFromAuthProto
		// maps that to AuthMethodNone just like every other unknown
		// value, so no explicit zero check is needed.
		negotiatedMethod := authMethodFromAuthProto(proxy.AuthProto, proxy.AuthData)
		s.logger.Info("ppp: proxy LCP short-circuit",
			"mru", proxy.MRU,
			"auth-proto", strconv.Itoa(int(proxy.AuthProto)),
			"auth-method", negotiatedMethod.String())
		mru := proxy.MRU
		if mru == 0 {
			mru = MaxFrameLen
		}
		s.mu.Lock()
		s.negotiatedMRU = mru
		s.negotiatedAuthMethod = negotiatedMethod
		s.mu.Unlock()
		s.sendEvent(EventLCPUp{
			TunnelID:      s.tunnelID,
			SessionID:     s.sessionID,
			NegotiatedMRU: mru,
		})
		if !s.afterLCPOpen() {
			return
		}
		// Commit Opened state only after afterLCPOpen succeeds so
		// SessionByID never reports "opened" on a session that is
		// actually tearing down (NOTE 5 from /ze-review Phase 10).
		s.mu.Lock()
		s.state = LCPStateOpened
		s.mu.Unlock()
	} else {
		// Synthetic Up: Initial -> Closed.
		trUp := LCPDoTransition(LCPStateInitial, LCPEventUp)
		s.mu.Lock()
		s.state = trUp.NewState
		s.mu.Unlock()

		// Synthetic Open: Closed -> ReqSent with [IRC, SCR].
		trOpen := LCPDoTransition(LCPStateClosed, LCPEventOpen)
		for _, act := range trOpen.Actions {
			if !s.performAction(act, LCPPacket{}) {
				return
			}
		}
		s.mu.Lock()
		s.state = trOpen.NewState
		s.mu.Unlock()
	}

	// Negotiation timeout timer. Active until Opened is reached
	// (or skipped via proxy). After Opened the timer is stopped and
	// negoTimerC is nil, so the select ignores it.
	var (
		negoTimer  *time.Timer
		negoTimerC <-chan time.Time
	)
	// RFC 1661 Section 4.6: restart timer retransmits ConfReq while
	// in ReqSent or AckSent. Fires every 3s until LCP reaches Opened,
	// then is stopped alongside negoTimer.
	var (
		restartTicker  *time.Ticker
		restartTickerC <-chan time.Time
	)
	if !isProxy {
		negoTimer = time.NewTimer(defaultNegoTimeout)
		defer negoTimer.Stop()
		negoTimerC = negoTimer.C
		restartTicker = time.NewTicker(3 * time.Second)
		defer restartTicker.Stop()
		restartTickerC = restartTicker.C
	}

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
	if isProxy {
		echoTickerC = echoTicker.C
	}
	echoID := uint8(0)

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
		if s.reauthInterval <= 0 || reauthTicker != nil {
			return
		}
		s.mu.Lock()
		method := s.negotiatedAuthMethod
		s.mu.Unlock()
		if method != AuthMethodCHAPMD5 && method != AuthMethodMSCHAPv2 {
			return
		}
		reauthTicker = time.NewTicker(s.reauthInterval)
		reauthTickerC = reauthTicker.C
	}
	defer func() {
		if reauthTicker != nil {
			reauthTicker.Stop()
		}
	}()
	if isProxy {
		startReauthTicker()
	}

	for {
		select {
		case <-s.stopCh:
			s.sendEvent(EventSessionDown{
				TunnelID: s.tunnelID, SessionID: s.sessionID,
				Reason: "driver stopped",
			})
			return

		case err := <-readDone:
			reason := "chan fd closed"
			if err != nil && !errors.Is(err, io.EOF) {
				reason = "chan fd read error: " + err.Error()
			}
			s.sendEvent(EventSessionDown{
				TunnelID: s.tunnelID, SessionID: s.sessionID,
				Reason: reason,
			})
			return

		case <-negoTimerC:
			s.fail("LCP negotiation timeout after " + defaultNegoTimeout.String())
			return

		case <-restartTickerC:
			s.mu.Lock()
			st := s.state
			s.mu.Unlock()
			if st == LCPStateReqSent || st == LCPStateAckSent {
				s.sendConfigureRequest()
			}

		case <-echoTickerC:
			s.mu.Lock()
			s.echoOutstanding++
			out := s.echoOutstanding
			s.mu.Unlock()
			if out > echoMax {
				s.sendEvent(EventSessionDown{
					TunnelID: s.tunnelID, SessionID: s.sessionID,
					Reason: "echo timeout: " + strconv.Itoa(int(out)) +
						" consecutive failures",
				})
				return
			}
			echoID++
			if !s.sendEchoRequest(echoID) {
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
				return
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
				})
				return
			}
			wasOpened := s.currentState() == LCPStateOpened
			term := s.handleFrame(frame)
			putFrameBuf(frame)
			if term {
				return
			}
			if !wasOpened && s.currentState() == LCPStateOpened {
				// Transitioned into Opened. Disable negotiation
				// timeout and start echo ticker. negoTimer.Stop
				// returns false if it already fired; the fired
				// value on negoTimer.C is harmless because we null
				// out negoTimerC and never select on it again.
				if negoTimer != nil {
					negoTimer.Stop()
					negoTimerC = nil
				}
				if restartTicker != nil {
					restartTicker.Stop()
					restartTickerC = nil
				}
				echoTickerC = echoTicker.C
				// Initial auth has completed (afterLCPOpen ran
				// from handleLCPPacket's Opened branch). Enable
				// periodic re-auth now that we know the negotiated
				// method; no-op for PAP / None.
				startReauthTicker()
			}
		}
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
	return 0, errors.New("ppp: crypto/rand returned zero " +
		strconv.Itoa(magicDrawMaxAttempts) + " times")
}

// fail emits EventSessionDown with the reason and logs at warn.
// Caller MUST exit immediately after calling fail.
func (s *pppSession) fail(reason string) {
	s.logger.Warn("ppp: session failed", "reason", reason)
	s.sendEvent(EventSessionDown{
		TunnelID:  s.tunnelID,
		SessionID: s.sessionID,
		Reason:    reason,
	})
}

// sendEvent writes an event to eventsOut, blocking until the consumer
// reads or the driver stops. The transport MUST drain events promptly
// or PPP processing will stall. Buffer size is defaultEventsOutBuf
// (64); sustained backlog beyond that blocks PPP progress.
func (s *pppSession) sendEvent(ev Event) {
	select {
	case s.eventsOut <- ev:
	case <-s.stopCh:
	}
}

// afterLCPOpen performs the post-Opened side effects: set MRU on the
// unit fd, set MTU on pppN via iface backend, bring pppN up, run the
// auth hook. Returns false (and emits EventSessionDown) on a fatal
// error.
//
// Order matters: set the fd-side MRU BEFORE asking the kernel to
// bring the interface up, so the first frame is sized correctly.
func (s *pppSession) afterLCPOpen() bool {
	s.mu.Lock()
	mru := s.negotiatedMRU
	s.mu.Unlock()
	if mru == 0 {
		mru = MaxFrameLen
	}

	if err := s.ops.setMRU(s.unitFD, mru); err != nil {
		s.fail("PPPIOCSMRU: " + err.Error())
		return false
	}

	ifname := "ppp" + strconv.Itoa(s.unitNum)
	// MTU at the netlink layer is the IP MTU = MRU - PPP encap (4).
	// Clamp to minIPMTU so a small but RFC-1661-legal MRU (>= 64)
	// never asks the kernel to set an MTU below the IPv4 minimum.
	mtu := int(mru) - pppEncapOverhead
	if mtu < minIPMTU {
		// Info, not Warn: this is a peer-configuration observation,
		// not a ze fault. At scale with many small-MRU peers the
		// Warn level would be noisy; Info keeps it visible for
		// operators who opt in to ppp.info.
		s.logger.Info("ppp: clamping MTU up to IPv4 minimum",
			"mru", mru, "computed-mtu", mtu, "clamped-mtu", minIPMTU)
		mtu = minIPMTU
	}
	if err := s.backend.SetMTU(ifname, mtu); err != nil {
		s.fail("iface SetMTU: " + err.Error())
		return false
	}
	if err := s.backend.SetAdminUp(ifname); err != nil {
		s.fail("iface SetAdminUp: " + err.Error())
		return false
	}

	if !s.runAuthPhase() {
		return false
	}

	if !s.runNCPPhase() {
		return false
	}

	s.sendEvent(EventSessionUp{
		TunnelID:  s.tunnelID,
		SessionID: s.sessionID,
	})
	return true
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
// Returns false (and fails the session) on reject, timeout, driver
// stop, or per-session stop.
func (s *pppSession) runAuthPhase() bool {
	s.mu.Lock()
	method := s.negotiatedAuthMethod
	s.mu.Unlock()

	switch method {
	case AuthMethodNone:
		if s.authRequired {
			reason := "no negotiated authentication method"
			s.fail("auth: " + reason)
			s.sendAuthEvent(EventAuthFailure{
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
	}
	// Unreachable for all defined AuthMethod constants; any value
	// arriving here is a programmer error (out-of-range cast).
	// AuthMethod.String() would itself panic on such a value, so
	// stringify the numeric code directly to preserve the intent of
	// "fail cleanly, do not crash the session goroutine."
	s.fail("auth: unknown method " + strconv.Itoa(int(method)))
	return false
}

// runNoAuthPhase is the AuthMethodNone dispatch target. It emits one
// EventAuthRequest on the auth channel so an external handler can
// still admit or deny by policy, waits on authRespCh, and on accept
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
		s.fail("auth rejected: " + decision.message)
		s.sendAuthEvent(EventAuthFailure{
			TunnelID:  s.tunnelID,
			SessionID: s.sessionID,
			Reason:    decision.message,
		})
		return false
	}
	s.sendAuthEvent(EventAuthSuccess{
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
	select {
	case s.authEventsOut <- ev:
	case <-s.stopCh:
	case <-s.sessStop:
	}
}

// readFrames reads PPP frames from chanFile in a loop and forwards
// them on the channel. Exits on read error.
func (s *pppSession) readFrames(out chan<- []byte, done chan<- error) {
	defer close(out)
	for {
		buf := getFrameBuf()
		n, err := s.chanFile.Read(buf)
		if err != nil {
			putFrameBuf(buf)
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
		case <-s.stopCh:
			putFrameBuf(buf)
			return
		}
	}
}

// handleFrame parses one received frame and dispatches by protocol to
// the appropriate FSM handler. LCP, IPCP, and IPv6CP all share the
// same packet shape (RFC 1661 §5); only the Data semantics differ.
// Returns true if the session should terminate.
func (s *pppSession) handleFrame(frame []byte) bool {
	proto, payload, _, err := ParseFrame(frame)
	if err != nil {
		s.logger.Debug("ppp: malformed frame dropped", "error", err.Error())
		return false
	}

	pkt, perr := ParseLCPPacket(payload)

	switch proto {
	case ProtoLCP:
		if perr != nil {
			s.logger.Debug("ppp: malformed LCP packet", "error", perr.Error())
			return false
		}
		return s.handleLCPPacket(pkt)
	case ProtoIPCP:
		if s.disableIPCP {
			s.logger.Debug("ppp: IPCP frame dropped (disabled)")
			return false
		}
		if perr != nil {
			s.logger.Debug("ppp: malformed IPCP packet", "error", perr.Error())
			return false
		}
		return s.handleIPCPPacket(pkt)
	case ProtoIPv6CP:
		if s.disableIPv6CP {
			s.logger.Debug("ppp: IPv6CP frame dropped (disabled)")
			return false
		}
		if perr != nil {
			s.logger.Debug("ppp: malformed IPv6CP packet", "error", perr.Error())
			return false
		}
		return s.handleIPv6CPPacket(pkt)
	}
	s.logger.Debug("ppp: non-control-plane protocol dropped",
		"protocol", strconv.FormatUint(uint64(proto), 16))
	return false
}

// codeToEvent maps an LCP code to the FSM event for a "received"
// packet. Maps unknown codes to RUC.
func codeToEvent(code uint8, optsBad bool) LCPEvent {
	switch code {
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
	case LCPCodeReject, LCPProtocolReject:
		return LCPEventRXJPlus
	case LCPEchoRequest, LCPEchoReply, LCPDiscardRequest:
		return LCPEventRXR
	}
	return LCPEventRUC
}

// handleLCPPacket maps the received LCP code to an FSM event and
// drives the resulting actions. Returns true if the session should
// terminate.
func (s *pppSession) handleLCPPacket(pkt LCPPacket) bool {
	cur := s.currentState()

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

	optsBad := false
	var peerOpts []LCPOption
	if pkt.Code == LCPConfigureRequest {
		opts, perr := ParseLCPOptions(pkt.Data)
		if perr != nil {
			optsBad = true
		} else {
			peerOpts = opts
			policy := LCPNegPolicy{MaxMRU: s.maxMRU}
			_, naks, rejects := NegotiatePeerOptions(opts, policy)
			if len(rejects) > 0 || len(naks) > 0 {
				optsBad = true
			}
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
		switch cur {
		case LCPStateReqSent, LCPStateAckSent, LCPStateAckRcvd, LCPStateOpened:
			s.adjustAuthOnNakOrReject(pkt)
		case LCPStateInitial, LCPStateStarting, LCPStateClosed,
			LCPStateStopped, LCPStateClosing, LCPStateStopping:
			// Nak/Reject in these states never triggers SCR per RFC
			// 1661 §4.1 transition table: handled with STA (Stopped),
			// ignored (Starting / Initial), or no-op (Closing /
			// Stopping). Skip the mutation -- the packet is stale.
		}
	}

	ev := codeToEvent(pkt.Code, optsBad)

	tr := LCPDoTransition(cur, ev)
	if tr.NewState == cur && len(tr.Actions) == 0 {
		s.logFSMNoOp(cur, ev, pkt)
		return false
	}

	for _, act := range tr.Actions {
		if !s.performAction(act, pkt) {
			return true
		}
	}

	// ISSUE 3 fix: capture the peer's MRU from its accepted CR.
	// The "negotiated MRU" for the send direction is what the PEER
	// said it will receive (from its own CR). Update only on RCR+,
	// because that is the moment ze accepts the peer's options.
	if ev == LCPEventRCRPlus && len(peerOpts) > 0 {
		if v, ok := lookupMRUOption(peerOpts); ok {
			s.mu.Lock()
			s.negotiatedMRU = v
			s.mu.Unlock()
		}
	}

	if cur != LCPStateOpened && tr.NewState == LCPStateOpened {
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
		s.mu.Unlock()
		s.sendEvent(EventLCPUp{
			TunnelID:      s.tunnelID,
			SessionID:     s.sessionID,
			NegotiatedMRU: mru,
		})
		if !s.afterLCPOpen() {
			return true
		}
		// Commit state only after afterLCPOpen succeeds.
		s.mu.Lock()
		s.state = LCPStateOpened
		s.mu.Unlock()
		return false
	}

	s.mu.Lock()
	s.state = tr.NewState
	s.mu.Unlock()

	if tr.NewState == LCPStateClosed || tr.NewState == LCPStateStopped {
		s.sendEvent(EventSessionDown{
			TunnelID:  s.tunnelID,
			SessionID: s.sessionID,
			Reason:    "LCP terminated: state=" + tr.NewState.String(),
		})
		return true
	}

	return false
}

// logFSMNoOp emits a debug log for an FSM (state, event) combination
// that produced no transition. Includes a hex sample of the offending
// packet so operators can diagnose buggy or hostile peers.
//
// Guarded by an Enabled() check so the sample buffer + hex encoding
// are only computed when debug-level logging is actually on. FSM
// no-ops should be rare but a hostile peer could spam them.
func (s *pppSession) logFSMNoOp(state LCPState, ev LCPEvent, pkt LCPPacket) {
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
// chan fd. Returns false on a fatal write error.
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
		return s.sendEchoReply(current)
	case LCPActIRC, LCPActZRC, LCPActTLU, LCPActTLD, LCPActTLS, LCPActTLF:
		// IRC/ZRC: restart-counter management deferred to a 6a
		// hardening pass (see plan/deferrals.md).
		// TLU/TLD/TLS/TLF: "notify upper layers" handled inline in
		// handleLCPPacket via the state-transition check.
		return true
	}
	return true
}

func (s *pppSession) sendConfigureRequest() bool {
	authProto, authData := authMethodToLCPOptions(s.configuredAuthMethod)
	opts := BuildLocalConfigRequest(LCPOptions{
		MRU:       s.maxMRU,
		Magic:     s.magic,
		AuthProto: authProto,
		AuthData:  authData,
	})
	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ProtoLCP, nil)
	dataOff := off + lcpHeaderLen
	dataLen := WriteLCPOptions(buf, dataOff, opts)
	off += WriteLCPPacket(buf, off, LCPConfigureRequest, 1, buf[dataOff:dataOff+dataLen])
	return s.writeFrame(buf[:off])
}

func (s *pppSession) sendConfigureAck(req LCPPacket) bool {
	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ProtoLCP, nil)
	off += WriteLCPPacket(buf, off, LCPConfigureAck, req.Identifier, req.Data)
	return s.writeFrame(buf[:off])
}

func (s *pppSession) sendConfigureNakOrReject(req LCPPacket) bool {
	opts, perr := ParseLCPOptions(req.Data)
	if perr != nil {
		return s.sendCodeReject(req)
	}
	policy := LCPNegPolicy{MaxMRU: s.maxMRU}
	_, naks, rejects := NegotiatePeerOptions(opts, policy)

	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ProtoLCP, nil)
	dataOff := off + lcpHeaderLen
	var (
		code    uint8
		dataLen int
	)
	if len(rejects) > 0 {
		code = LCPConfigureReject
		dataLen = WriteLCPOptions(buf, dataOff, rejects)
	} else {
		code = LCPConfigureNak
		dataLen = WriteLCPOptions(buf, dataOff, naks)
	}
	off += WriteLCPPacket(buf, off, code, req.Identifier, buf[dataOff:dataOff+dataLen])
	return s.writeFrame(buf[:off])
}

func (s *pppSession) sendTerminateRequest() bool {
	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ProtoLCP, nil)
	off += WriteLCPPacket(buf, off, LCPTerminateRequest, 1, nil)
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
// RFC 1661 §5.7: "Reception of a Code-Reject of a code which is
// fundamental to this version of the protocol indicates an
// implementation which is running a catastrophically different
// version... In this case, the implementation SHOULD report the
// problem and drop the connection". The rejected packet's original
// Code, Identifier, Length, and Data are included in the Rejected-
// Packet field verbatim so the peer can identify what was rejected.
func (s *pppSession) sendCodeReject(req LCPPacket) bool {
	body := make([]byte, lcpHeaderLen+len(req.Data))
	body[0] = req.Code
	body[1] = req.Identifier
	binary.BigEndian.PutUint16(body[2:4], uint16(lcpHeaderLen+len(req.Data)))
	copy(body[lcpHeaderLen:], req.Data)

	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ProtoLCP, nil)
	off += WriteLCPPacket(buf, off, LCPCodeReject, req.Identifier, body)
	return s.writeFrame(buf[:off])
}

func (s *pppSession) sendEchoReply(req LCPPacket) bool {
	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ProtoLCP, nil)
	off += BuildLCPEchoReply(buf, off, req.Identifier, s.magic, req.Data)
	return s.writeFrame(buf[:off])
}

func (s *pppSession) sendEchoRequest(id uint8) bool {
	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ProtoLCP, nil)
	off += WriteLCPEcho(buf, off, LCPEchoRequest, id, s.magic, nil)
	s.lastEchoSentAt = time.Now()
	return s.writeFrame(buf[:off])
}

func (s *pppSession) writeFrame(frame []byte) bool {
	_, err := s.chanFile.Write(frame)
	if err != nil {
		s.fail("chan fd write: " + err.Error())
		return false
	}
	return true
}
