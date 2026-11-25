// Design: docs/architecture/l2tp/bng-5-pppoe.md -- per-interface PPPoE server
// RFC: rfc/short/rfc2516.md -- Sections 5.1 through 5.4, discovery admission
// Related: discovery.go -- ParseDiscovery, Build* frame constructors
// Related: cookie.go -- GenerateCookie, VerifyCookie
// Related: session.go -- SessionTable, Session
// Related: ratelimit.go -- PADILimiter

package pppoe

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/core/callsink"
)

const (
	// reasonPADTMACMismatch labels the Debug log emitted when a PADT names a
	// live SESSION_ID from a source MAC that does not own that session.
	reasonPADTMACMismatch = "padt-mac-mismatch"
	// padtMismatchLogWindowSec and padtMismatchLogBurst bound how many
	// PADT MAC-mismatch Debug lines one InterfaceServer emits per second so a
	// host forging PADTs at line rate cannot flood the log.
	padtMismatchLogWindowSec = 1
	padtMismatchLogBurst     = 5
)

// padtMismatchLogLimiter bounds PADT MAC-mismatch Debug logging per
// InterfaceServer using a fixed one-second window. Its zero value is ready
// to use and safe to copy on reload.
// NOT safe for concurrent use: only the discovery reader goroutine calls it.
type padtMismatchLogLimiter struct {
	started     bool
	windowStart int64
	logged      int
	suppressed  int
}

// allow reports whether a PADT MAC-mismatch log line may be emitted now.
// When a new window opens after suppressed frames, suppressed carries how
// many frames were omitted in the previous window.
func (l *padtMismatchLogLimiter) allow() (allowed bool, suppressed int) {
	now := nowUnix()
	if l.started {
		elapsed := now - l.windowStart
		if elapsed >= 0 {
			if elapsed < padtMismatchLogWindowSec {
				if l.logged >= padtMismatchLogBurst {
					l.suppressed++
					return false, 0
				}
				l.logged++
				return true, 0
			}
		}
	}
	prevSuppressed := l.suppressed
	l.started = true
	l.windowStart = now
	l.logged = 1
	l.suppressed = 0
	return true, prevSuppressed
}

// InterfaceServer owns the PPPoE state for one access interface.
// The shared discovery reader goroutine dispatches parsed packets
// here by ifindex.
type InterfaceServer struct {
	// ifName is the configured (logical) access interface name: what show,
	// logs, events and RADIUS report. device is the kernel device it resolved
	// to when the server was built, from the same binding as ifIndex and
	// hwAddr. Every kernel call that takes a device name takes device, never
	// ifName: with an os-name or mac match selector the two differ.
	ifName    string
	device    string
	ifIndex   int
	hwAddr    [EthALen]byte
	mtu       int
	sessions  *SessionTable
	cookieKey CookieKey
	limiter   *PADILimiter

	// maxSessionsPerMAC bounds how many sessions one subscriber MAC address
	// may hold on this interface (config.go: InterfaceConfig.MaxSessionsPerMAC,
	// resolved global-then-override the way MaxSessions already is).
	// admitPerMACCap enforces it in handlePADR.
	maxSessionsPerMAC int

	cookieTimeout time.Duration
	acName        string
	serviceNames  []string
	authMethod    ppp.AuthMethod
	authRequired  bool
	authAllow     []ppp.AuthMethod
	lcpConfig     ppp.LCPConfig

	discFD    int
	pppDriver *ppp.Driver
	logger    *slog.Logger

	// peerLeft reports a subscriber PADT to the subsystem, so a realm
	// relay cancels its call (realm_relay.go subscriberGone). A session the
	// PPP driver released into an L2TP bridge produces no PPP session-down,
	// so PADT is the only news of its end. Nil in tests that build a bare
	// server.
	peerLeft func(*Session)
	// relayWake coalesces ends for the existing event consumer.
	// A bare server test can drain its table directly instead.
	relayWake chan<- struct{}

	// sendFrameFn, when set, replaces the real discovery-socket write.
	// sendFrame calls it instead of sendDiscoveryFrame. Production code
	// never sets it, so the zero value (nil) keeps the real send path.
	// A test sets it to capture the frame a handler attempted to send,
	// because the non-Linux stub in socket_other.go always returns an
	// error and so cannot tell a test whether a send was attempted.
	sendFrameFn func(frame []byte)

	// openSession is the selected registry provider, captured for this
	// interface generation. Tests inject the same ownership-returning boundary.
	openSession func(context.Context, dataplane.Request) (dataplane.OpenedSession, error)

	// admitting is the session the PADR in dispatch is admitting, so a
	// contained discovery panic can end it (panic_guard.go). Only the
	// discovery reader goroutine reads or writes it.
	admitting admission

	// padtMismatchLog rate-limits PADT MAC-mismatch Debug logging. Only the
	// discovery reader goroutine reads or writes it.
	padtMismatchLog padtMismatchLogLimiter
}

// HandleDiscovery dispatches a parsed discovery packet to the
// appropriate handler based on its code.
func (s *InterfaceServer) HandleDiscovery(pkt *Packet) {
	switch pkt.Code {
	case CodePADI:
		s.handlePADI(pkt)
	case CodePADR:
		s.handlePADR(pkt)
	case CodePADT:
		s.handlePADT(pkt)
	}
}

func (s *InterfaceServer) handlePADI(pkt *Packet) {
	if pkt.SID != 0 {
		return
	}

	if s.limiter != nil && !s.limiter.Check(pkt.SrcMAC) {
		// No log line here on purpose: the limiter's whole job is to survive
		// a PADI flood, and a Debug line per dropped packet would turn that
		// flood into a logging problem. The counter carries the visibility.
		countRefusal(reasonRateLimited)
		return
	}

	if !MatchServiceName(pkt, s.serviceNames) {
		s.logger.Debug("pppoe: PADI service-name mismatch", "src", net.HardwareAddr(pkt.SrcMAC[:]), "reason", reasonServiceNameMismatch)
		countRefusal(reasonServiceNameMismatch)
		return
	}

	cookie := GenerateCookie(s.cookieKey, s.hwAddr[:], pkt.SrcMAC[:], relayIDFromPacket(pkt))

	var buf [EthMaxLen]byte
	frame := BuildPADO(buf[:], s.hwAddr, pkt, s.acName, s.serviceNames, cookie)
	if frame == nil {
		s.logger.Warn("pppoe: PADO frame too large")
		return
	}

	s.sendFrame(frame)
}

// requireServiceNameTag reports whether pkt carries a Service-Name tag,
// any value included. MatchServiceName cannot answer this question on
// its own: an empty allow-list accepts every packet regardless of tag
// presence, which is correct for a PADI (AC-1) but not for a PADR,
// where RFC 2516 Section 5.3 makes the tag mandatory (AC-3).
func requireServiceNameTag(pkt *Packet) bool {
	return pkt.FindTag(TagServiceName) != nil
}

// admitPerMACCap reports whether pkt's source MAC may receive a new PPPoE
// session under s.maxSessionsPerMAC. RFC 2516 places no per-peer session
// limit (rfc/short/rfc2516.md): Section 9 only names the concept -- an AC
// using the AC-Cookie "can then limit concurrent sessions for [a PADI
// SOURCE_ADDR]" -- without setting a bound, so the cap itself is a Ze policy
// decision, not an RFC obligation.
//
// The count includes every session the table still holds for the MAC,
// including one mid-teardown (State == StateTeardown). Its SID remains
// reserved while a final PADT is being sent, even though the transport has
// already disconnected. Excluding it would admit a replacement during that
// outstanding teardown. A cap that resolves to zero refuses every PADR rather than
// admitting without limit: len(...) < 0 is never true.
func (s *InterfaceServer) admitPerMACCap(pkt *Packet) bool {
	return len(s.sessions.sessionsByMAC(net.HardwareAddr(pkt.SrcMAC[:]))) < s.maxSessionsPerMAC
}

func (s *InterfaceServer) handlePADR(pkt *Packet) {
	if pkt.SID != 0 {
		return
	}

	cookieTag := pkt.FindTag(TagACCookie)
	if cookieTag == nil {
		s.logger.Debug("pppoe: PADR without AC-Cookie", "src", net.HardwareAddr(pkt.SrcMAC[:]), "reason", reasonCookieInvalid)
		countRefusal(reasonCookieInvalid)
		return
	}

	if !VerifyCookie(s.cookieKey, cookieTag.Value, s.hwAddr[:], pkt.SrcMAC[:], relayIDFromPacket(pkt), s.cookieTimeout) {
		s.logger.Debug("pppoe: PADR invalid cookie", "src", net.HardwareAddr(pkt.SrcMAC[:]), "reason", reasonCookieInvalid)
		countRefusal(reasonCookieInvalid)
		return
	}

	// RFC 2516 Section 5.3: "The PADR packet MUST contain exactly one TAG
	// of TAG_TYPE Service-Name". MatchServiceName alone cannot enforce
	// this: an empty allow-list matches any packet, tag present or not.
	// Ze refuses a tagless PADR under any configuration and replies the
	// way RFC 2516 Section 5.4 requires: "If the Access Concentrator does
	// not like the Service-Name in the PADR, then it MUST reply with a
	// PADS containing a TAG of TAG_TYPE Service-Name-Error ... In this
	// case the SESSION_ID MUST be set to 0x0000."
	if !requireServiceNameTag(pkt) {
		s.logger.Debug("pppoe: PADR refused: missing Service-Name tag (RFC 2516 5.3/5.4)", "src", net.HardwareAddr(pkt.SrcMAC[:]), "reason", reasonServiceNameMissing)
		countRefusal(reasonServiceNameMissing)
		var buf [EthMaxLen]byte
		frame := BuildPADSError(buf[:], s.hwAddr, pkt, s.acName, TagSvcNameError)
		if frame != nil {
			s.sendFrame(frame)
		}
		return
	}

	if !MatchServiceName(pkt, s.serviceNames) {
		s.logger.Debug("pppoe: PADR service-name mismatch", "src", net.HardwareAddr(pkt.SrcMAC[:]), "reason", reasonServiceNameMismatch)
		countRefusal(reasonServiceNameMismatch)
		var buf [EthMaxLen]byte
		frame := BuildPADSError(buf[:], s.hwAddr, pkt, s.acName, TagSvcNameError)
		if frame != nil {
			s.sendFrame(frame)
		}
		return
	}

	// PADR dedup: a MAC that already holds a session admitted by the exact
	// cookie this PADR carries is retransmitting a request the AC already
	// answered, so the reply repeats that session's SID rather than
	// allocating a new one. The correlator is the cookie, not the MAC and
	// state alone: RFC 2516 places no per-peer session limit
	// (rfc/short/rfc2516.md), so a MAC under the cap can be admitting a
	// second, genuinely distinct session (its own fresh PADI/PADO cookie) at
	// the same moment a replay of its first session's PADR arrives, and
	// matching on MAC and state alone would answer that second PADR with the
	// first session's SID instead of admitting it (spec-pppoe-padr-replay-
	// allocates-unbounded-sessions, Key Design Decisions). matchLiveCookie
	// also excludes a session the event-consumer goroutine has already begun
	// tearing down, so a dying session is never handed back to a
	// concurrently-processed PADR (R-2).
	if sid, ok := s.sessions.matchLiveCookie(net.HardwareAddr(pkt.SrcMAC[:]), cookieTag.Value); ok {
		var buf [EthMaxLen]byte
		frame := BuildPADS(buf[:], s.hwAddr, pkt, s.acName, sid)
		if frame != nil {
			s.sendFrame(frame)
		}
		return
	}

	if !s.admitPerMACCap(pkt) {
		s.logger.Debug("pppoe: PADR refused: per-MAC session cap reached",
			"src", net.HardwareAddr(pkt.SrcMAC[:]), "cap", s.maxSessionsPerMAC, "reason", reasonPerMACCapReached)
		countRefusal(reasonPerMACCapReached)
		// RFC 2516 Section 5.4: AC-System-Error "indicates that the Access
		// Concentrator experienced some error in performing the Host request
		// (For example insufficient resources ...) It MAY be included in
		// PADS packets." A per-MAC cap is exactly that: Ze's own resource
		// policy, not an objection to the requested Service-Name, so
		// Service-Name-Error would misname the reason. This is the same tag
		// AllocSID exhaustion answers with below, for the same class of
		// refusal. The RFC does not spell out SESSION_ID 0x0000 for this tag
		// the way it does for Service-Name-Error, but no session was
		// allocated, so 0 is the only value that can be true.
		var buf [EthMaxLen]byte
		frame := BuildPADSError(buf[:], s.hwAddr, pkt, s.acName, TagACSystemError)
		if frame != nil {
			s.sendFrame(frame)
		}
		return
	}

	sid, err := s.sessions.AllocSID()
	if err != nil {
		s.logger.Warn("pppoe: session ID exhausted", "error", err, "reason", reasonSessionIDExhausted)
		countRefusal(reasonSessionIDExhausted)
		var buf [EthMaxLen]byte
		frame := BuildPADSError(buf[:], s.hwAddr, pkt, s.acName, TagACSystemError)
		if frame != nil {
			s.sendFrame(frame)
		}
		return
	}

	sess := &Session{
		SubscriberID:  pppoeSessionID(s.ifIndex, sid),
		AccessIfIndex: s.ifIndex,
		SID:           sid,
		MAC:           net.HardwareAddr(append([]byte(nil), pkt.SrcMAC[:]...)),
		IfName:        s.ifName,
		ServiceName:   pkt.ServiceNameString(),
		Cookie:        append([]byte(nil), cookieTag.Value...),
		State:         StateDiscovery,
		CreatedAt:     time.Now(),
	}
	if hostUniq := pkt.FindTag(TagHostUniq); hostUniq != nil {
		sess.HostUniq = append([]byte(nil), hostUniq.Value...)
	}
	if err := s.sessions.Add(sess); err != nil {
		s.sessions.freeSID(sid)
		s.logger.Warn("pppoe: session add failed", "sid", sid, "error", err)
		return
	}
	s.admitting = admission{session: sess}

	// No PPP driver check here: a Service-Name relay needs only the session
	// socket and the PPP channel below, never the driver, so a LAC with no
	// interface backend still relays the subscribers its bindings name. The
	// paths that need the driver refuse after the relay decision.

	// RFC 2516 Section 4: "For PPP session traffic, this field MUST contain
	// the peer's unicast address as determined from the Discovery stage."
	// RFC 2516 Section 6: "The SESSION_ID MUST NOT change for that PPPoE
	// session and MUST be the value assigned in the Discovery stage."
	// The session socket is bound once, here, to the SID the PADS below
	// carries and to the PADR's unicast source (ParseDiscovery refuses a
	// broadcast or multicast source).
	if s.openSession == nil {
		s.reportSessionAdmissionFailure(sid, errors.New("no session data-plane provider is configured"))
		s.closeAdmitted(sess)
		return
	}
	opened, err := s.openSession(context.Background(), dataplane.Request{
		AccessName: s.ifName, Device: s.device, SessionID: sid,
		PeerMAC: pkt.SrcMAC, CaptureProxyLCP: realmRelayConfigured(),
	})
	if err != nil {
		s.reportSessionAdmissionFailure(sid, err)
		s.closeAdmitted(sess)
		return
	}
	attempt, err := ppp.NewSessionAttempt(sess.SubscriberID, opened.Dataplane())
	if err != nil {
		s.reportSessionAdmissionFailure(sid, err)
		s.closeSession(opened)
		s.closeAdmitted(sess)
		return
	}
	if !s.sessions.setOpened(sess, opened, attempt) {
		s.closeSession(opened)
		return
	}

	// Send PADS only after kernel setup succeeds and the session has a
	// taker (the L2TP relay or the PPP driver). If we sent it earlier and
	// setup failed, or nothing could run the session, the subscriber would
	// wait for LCP that never comes. The frame is built before the relay
	// decision, so a relay is never placed for a session whose PADS cannot
	// be sent.
	var buf [EthMaxLen]byte
	frame := BuildPADS(buf[:], s.hwAddr, pkt, s.acName, sid)
	if frame == nil {
		s.closeAdmitted(sess)
		return
	}

	// spec-followup-l2tp-call AC-3: if a relay binding matches this service,
	// hand the subscriber to the L2TP subsystem (LAC incoming call) instead
	// of terminating PPP locally. The relay decision crosses the pppoe -> l2tp
	// boundary through the neutral callsink registry (pppoe never imports
	// l2tp). The request carries the PPPoX transport owner so L2TP can borrow
	// its channel for the kernel bridge (A-4). The unused local PPP lease
	// stays owned here until bridge preparation. The transport remains
	// available to the bridge until the PPPoE session ends. The
	// PPP driver plays no part, so a LAC with none still relays.
	if s.relayToL2TP(pkt, sid, opened) {
		// refuse-by-default: guards sendPADS -- a call canceled during Relay cannot publish an accepted admission it no longer owns
		if s.sessions.transport(sid) != opened {
			return
		}
		s.sendPADS(sid, frame)
		s.logger.Info("pppoe: subscriber relayed to L2TP; local PPP not started", "sid", sid)
		return
	}

	// Local termination needs the PPP driver, and so does a realm relay: it
	// decides only after local LCP and authentication have run on the driver
	// (realm_relay.go decide, called from the auth drain). Without one the
	// subscriber is refused now, before any PADS, with the tag RFC 2516
	// Section 5.4 names for an AC that cannot perform the request:
	// AC-System-Error "indicates that the Access Concentrator experienced
	// some error in performing the Host request (For example insufficient
	// resources ...) It MAY be included in PADS packets." No session was
	// started, so SESSION_ID is 0, as for the other refusals above.
	if s.pppDriver == nil {
		s.logger.Warn("pppoe: PADR refused: no PPP driver, so this subscriber can be neither terminated locally nor relayed by realm. "+
			"The PPP driver needs an interface backend, and none is loaded. "+
			"Fix: set interface backend kernel, or bind the subscriber's Service-Name to an L2TP relay",
			"src", net.HardwareAddr(pkt.SrcMAC[:]), "service", pkt.ServiceNameString(), "code", codeDoctorPPPoEUnavailable)
		s.closeAdmitted(sess)
		refusal := BuildPADSError(buf[:], s.hwAddr, pkt, s.acName, TagACSystemError)
		if refusal != nil {
			s.sendFrame(refusal)
		}
		return
	}
	if !s.sessions.localPPPAvailable(sess) {
		s.reportSessionAdmissionFailure(sid, errors.New("local PPP lease is closing after relay preparation"))
		s.closeAdmitted(sess)
		refusal := BuildPADSError(buf[:], s.hwAddr, pkt, s.acName, TagACSystemError)
		if refusal != nil {
			s.sendFrame(refusal)
		}
		return
	}
	s.sendPADS(sid, frame)

	// AuthMethod is what the AC advertises in its own LCP Configure-Request:
	// without it every subscriber reaches the accounting-only no-auth phase,
	// where an auth handler that holds credentials sees an empty username and
	// refuses. AuthRequired disconnects a client that rejects the method
	// rather than admitting it unauthenticated.
	start := ppp.StartSession{
		LCP:               s.lcpConfig,
		TunnelID:          uint16(s.ifIndex),
		SessionID:         sid,
		SubscriberID:      sess.SubscriberID,
		Attempt:           attempt,
		Frames:            opened.Frames(),
		Dataplane:         opened.Dataplane(),
		LNSMode:           true,
		PPPoE:             true,
		AuthMethod:        s.authMethod,
		AuthRequired:      s.authRequired,
		AuthFallbackOrder: s.authAllow,
		MaxMRU:            PPPoEMaxMTU,
		AccessInterface:   s.ifName,
		SubscriberMAC:     net.HardwareAddr(append([]byte(nil), pkt.SrcMAC[:]...)),
		ServiceName:       pkt.ServiceNameString(),
		VendorTags:        vendorTagsFromPacket(pkt),
		// A LAC with relay-realm bindings records the LCP exchange the
		// realm relay forwards (RFC 2661 Section 4.4.5); the Service-Name
		// relay above already took every subscriber it binds (AC-7).
		CaptureProxyLCP: realmRelayConfigured(),
	}

	// Mark transfer intent before queueing: a fast Driver event can already
	// reach the consumer when the channel send returns. A panicking unsent
	// handoff restores only the producer-owned lease before containment runs.
	lease := s.sessions.takePPPLease(sess)
	if lease == nil {
		return
	}
	transferred := false
	defer func() {
		if !transferred {
			s.sessions.restoreUnsentPPPLease(sess, lease)
		}
	}()
	s.pppDriver.SessionsIn() <- start
	transferred = true
}

// sendPADS sends the admitting PADS frame built for session sid and marks
// the session established: handlePADR calls it once the session has a taker.
func (s *InterfaceServer) sendPADS(sid uint16, frame []byte) {
	session := s.admitting.session
	if session == nil || session.SID != sid {
		return
	}
	if !s.sessions.notePADS(session, PADSAttempted) {
		return
	}
	// This transient boundary marker deliberately precedes the call: a
	// sender panic may occur after PADS escaped. It is never confirmation.
	s.admitting.padsSent = true
	if s.submitFrame(frame) {
		s.sessions.notePADS(session, PADSSubmitted)
	}
	s.sessions.markSessionFor(session)
}

func (s *InterfaceServer) handlePADT(pkt *Packet) {
	if pkt.SID == 0 {
		return
	}
	if pkt.DstMAC != s.hwAddr {
		return
	}

	sess := s.sessions.Lookup(pkt.SID)
	if sess == nil {
		return
	}

	if !bytes.Equal(sess.MAC, pkt.SrcMAC[:]) {
		if allowed, suppressed := s.padtMismatchLog.allow(); allowed {
			if suppressed > 0 {
				s.logger.Debug("pppoe: PADT MAC mismatch", "sid", pkt.SID,
					"expected", sess.MAC, "got", net.HardwareAddr(pkt.SrcMAC[:]),
					"reason", reasonPADTMACMismatch, "suppressed", suppressed)
			} else {
				s.logger.Debug("pppoe: PADT MAC mismatch", "sid", pkt.SID,
					"expected", sess.MAC, "got", net.HardwareAddr(pkt.SrcMAC[:]),
					"reason", reasonPADTMACMismatch)
			}
		}
		return
	}

	s.logger.Info("pppoe: PADT received", "sid", pkt.SID)
	// RFC 2516 Section 5.5: "Even normal PPP termination packets MUST NOT
	// be sent after sending or receiving a PADT." Disconnect the kernel
	// transport before waiting for the PPP driver to stop.
	if s.sessions.markTeardown(sess) == nil {
		return
	}
	publication := s.sessions.claimPADT(sess) // a received PADT also ends this wire
	if publication != nil {
		defer s.finishPADT(publication)
	} else {
		defer s.closeAdmitted(sess)
	}
	owner := s.sessions.admissionControl(sess)
	s.closeTransport(owner.opened)

	s.stopPPPAdmission(owner.attempt)
	if s.peerLeft != nil {
		s.peerLeft(sess)
	}
}

// handleSessionDown is called by the event consumer when PPP reports
// a session has ended. Sends PADT to the subscriber and cleans up.
//
// It marks the session StateTeardown (rather than calling Lookup) before
// doing anything else, because it runs on the PPP driver's event-consumer
// goroutine while a PADR replay's dedup match runs on the discovery-reader
// goroutine: markTeardown's write and matchLiveCookie's read of the same
// field both go through the session table's lock, so the two goroutines
// cannot race on it (spec-pppoe-padr-replay-allocates-unbounded-sessions
// R-2).
func (s *InterfaceServer) handleSessionDown(expected *Session) {
	sess := s.sessions.markTeardown(expected)
	if sess == nil {
		return
	}
	s.finishSessionDown(sess)
}

// finishSessionDown publishes PADT for an admission the table already claimed.
// The caller MUST keep its SID reserved until this method returns.
func (s *InterfaceServer) finishSessionDown(sess *Session) {
	owner := s.sessions.admissionControl(sess)
	// Gate this exact wire before PADT. A transferred/queued lease stays with
	// Driver; its genuine Down/refusal will finish this same admission.
	s.closeTransport(owner.opened)
	s.stopPPPAdmission(owner.attempt)
	publication := s.sessions.claimPADT(sess)
	if publication == nil {
		s.closeAdmitted(sess)
		return
	}
	defer s.finishPADT(publication)
	var dstMAC [EthALen]byte
	copy(dstMAC[:], sess.MAC)
	var buf [EthMaxLen]byte
	frame := BuildPADT(buf[:], s.hwAddr, dstMAC, sess.SID, s.acName)
	if frame != nil {
		s.sendFrame(frame)
	}
}

// finishPADT completes only the original handler's wire-end work, including a
// failed or panicking writer. The caller MUST defer it once after claimPADT.
// A genuine terminal event can arrive during the write, so closeAdmitted reads
// the current transfer/terminal state rather than a pre-publication snapshot.
func (s *InterfaceServer) finishPADT(publication *padtPublication) {
	close(publication.done)
	s.closeAdmitted(publication.session)
}

// closeAdmitted releases only resources the access server still owns. The PPP
// driver keeps a transferred lease through its final control writes. Transport
// cleanup still runs if an unused-lease close panics at the discovery boundary.
func (s *InterfaceServer) closeAdmitted(expected *Session) {
	admission, removed := s.sessions.beginAdmissionClose(expected)
	if admission == nil {
		return
	}
	var cleanupErr error
	defer func() {
		if failure := recover(); failure != nil {
			cause, ok := failure.(error)
			if !ok {
				cause = errors.New("pppoe: producer cleanup panicked; original owner retained")
			}
			s.sessions.failAdmissionClose(admission, errors.Join(cleanupErr, cause))
			panic(failure)
		}
		if cleanupErr != nil {
			s.sessions.failAdmissionClose(admission, cleanupErr)
			return
		}
		s.sessions.completeAdmissionClose(admission)
	}()
	// Each independent transport/token cleanup still runs if lease Close
	// panics. The table roots are detached only by confirmed completion.
	defer func() { cleanupErr = errors.Join(cleanupErr, s.closeTransportRetained(removed.opened)) }()
	if removed.relay != nil {
		defer removed.relay.cancel()
	}
	admission.Attempt.Cancel()
	cleanupErr = errors.Join(cleanupErr, s.closeTransportRetained(removed.opened))
	if removed.unusedLease != nil {
		var err error
		if removed.retryUnused {
			err = removed.unusedLease.retryClose()
		} else {
			err = removed.unusedLease.Close()
		}
		if err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
			ppp.ReportCleanupFailure(removed.unusedLease.handle.Identity(), ppp.CleanupSession, err)
			s.logger.Warn("pppoe: unused local PPP lease cleanup failed", "sid", admission.SID, "error", err,
				"code", ppp.CodeDoctorPPPCleanup, "fix", ppp.CleanupFailureFix)
		}
	}
}

// closeSession releases an unpublished owner that local PPP never received.
// Admitted sessions use closeAdmitted, which preserves any PPP-owned lease.
func (s *InterfaceServer) closeSession(opened dataplane.OpenedSession) {
	if opened == nil {
		return
	}
	defer s.closeTransport(opened)
	if err := opened.Dataplane().Close(); err != nil {
		ppp.ReportCleanupFailure(opened.Dataplane().Identity(), ppp.CleanupSession, err)
		s.logger.Warn("pppoe: session resource cleanup failed", "error", err,
			"code", ppp.CodeDoctorPPPCleanup, "fix", ppp.CleanupFailureFix)
	}
}

func (s *InterfaceServer) closeTransport(opened dataplane.OpenedSession) {
	if err := s.closeTransportRetained(opened); err != nil {
		// The exact failure is already recorded below. Gating callers retain
		// their admission; final cleanup also propagates this same result.
		return
	}
}

func (s *InterfaceServer) closeTransportRetained(opened dataplane.OpenedSession) error {
	if opened == nil {
		return nil
	}
	if err := opened.CloseTransport(); err != nil {
		ppp.ReportCleanupFailure(opened.Dataplane().Identity(), ppp.CleanupTransport, err)
		s.logger.Warn("pppoe: session transport cleanup failed", "error", err,
			"code", ppp.CodeDoctorPPPCleanup, "fix", ppp.CleanupFailureFix)
		return err
	}
	return nil
}

func (s *InterfaceServer) sendFrame(frame []byte) {
	s.submitFrame(frame)
}

// submitFrame distinguishes kernel submission from a void test observer. A
// returning sendFrameFn records only an attempted send, never kernel evidence.
func (s *InterfaceServer) submitFrame(frame []byte) bool {
	if s.sendFrameFn != nil {
		s.sendFrameFn(frame)
		return false
	}
	if err := sendDiscoveryFrame(s.discFD, s.ifIndex, frame); err != nil {
		s.logger.Debug("pppoe: send failed", "error", err)
		return false
	}
	return true
}

// relayToL2TP consults the registered call-sink to decide whether an admitted
// subscriber, whose PADS is built but not yet sent, should be relayed into an
// L2TP tunnel rather than terminated locally. The transport owner lends the
// PPPoX descriptor only while the L2TP worker performs its kernel bridge ioctl.
// Returns true when the subscriber was handed off to L2TP (an incoming call
// was originated). A nil sink (no L2TP subsystem) or an unmatched service
// returns false; a matched-but-failed relay logs and returns false so the
// subscriber falls back to local termination, or to the refusal handlePADR
// answers when no PPP driver exists.
func (s *InterfaceServer) relayToL2TP(pkt *Packet, sid uint16, opened dataplane.OpenedSession) bool {
	sink := callsink.Lookup()
	if sink == nil {
		return false
	}
	preparation := s.beginServiceRelay(sid, opened)
	if preparation == nil {
		return false
	}
	call, accepted, err := sink.Relay(callsink.Request{
		Service:       pkt.ServiceNameString(),
		Interface:     s.ifName,
		SubscriberMAC: net.HardwareAddr(pkt.SrcMAC[:]).String(),
		SessionID:     sid,
		Channel:       opened,
		BeforeBridge:  preparation,
		Handoff:       preparation,
	})
	if err != nil {
		preparation.abandon(call)
		s.logger.Warn("pppoe: L2TP relay failed; terminating locally",
			"sid", sid, "service", pkt.ServiceNameString(), "error", err.Error())
		return false
	}
	if !accepted {
		preparation.abandon(call)
		return false
	}
	preparation.accept(call)
	return true
}

func vendorTagsFromPacket(pkt *Packet) []byte {
	tag := pkt.FindTag(TagVendorSpecific)
	if tag == nil {
		return nil
	}
	return append([]byte(nil), tag.Value...)
}

func relayIDFromPacket(pkt *Packet) []byte {
	tag := pkt.FindTag(TagRelaySessionID)
	if tag == nil {
		return nil
	}
	return tag.Value
}
