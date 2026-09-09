// Design: docs/architecture/l2tp/bng-5-pppoe.md -- per-interface PPPoE server
// RFC: rfc/short/rfc2516.md -- Sections 5.1 through 5.4, discovery admission
// Related: discovery.go -- ParseDiscovery, Build* frame constructors
// Related: cookie.go -- GenerateCookie, VerifyCookie
// Related: session.go -- SessionTable, Session
// Related: ratelimit.go -- PADILimiter

package pppoe

import (
	"bytes"
	"log/slog"
	"net"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/core/callsink"
)

// InterfaceServer owns the PPPoE state for one access interface.
// The shared discovery reader goroutine dispatches parsed packets
// here by ifindex.
type InterfaceServer struct {
	ifName    string
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

	discFD    int
	pppDriver *ppp.Driver
	logger    *slog.Logger

	// sendFrameFn, when set, replaces the real discovery-socket write.
	// sendFrame calls it instead of sendDiscoveryFrame. Production code
	// never sets it, so the zero value (nil) keeps the real send path.
	// A test sets it to capture the frame a handler attempted to send,
	// because the non-Linux stub in socket_other.go always returns an
	// error and so cannot tell a test whether a send was attempted.
	sendFrameFn func(frame []byte)
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
// including one mid-teardown (State == StateTeardown): its kernel resources
// (AF_PPPOX socket, /dev/ppp channel and unit) are not released until Remove
// runs, so excluding it here would let the MAC hold one more session's worth
// of descriptors than the cap is meant to bound, for the length of that
// window. A cap that resolves to zero refuses every PADR rather than
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
		SID:         sid,
		MAC:         net.HardwareAddr(append([]byte(nil), pkt.SrcMAC[:]...)),
		IfName:      s.ifName,
		ServiceName: pkt.ServiceNameString(),
		Cookie:      append([]byte(nil), cookieTag.Value...),
		State:       StateDiscovery,
		CreatedAt:   time.Now(),
	}
	if hostUniq := pkt.FindTag(TagHostUniq); hostUniq != nil {
		sess.HostUniq = append([]byte(nil), hostUniq.Value...)
	}
	if err := s.sessions.Add(sess); err != nil {
		s.sessions.freeSID(sid)
		s.logger.Warn("pppoe: session add failed", "sid", sid, "error", err)
		return
	}

	if s.pppDriver == nil {
		s.logger.Warn("pppoe: no PPP driver, session will not start", "sid", sid)
		closePPPoxFD(s.sessions.Remove(sid))
		return
	}

	pppoxFD, err := pppoeCreate(s.ifName, sid, pkt.SrcMAC)
	if err != nil {
		s.logger.Error("pppoe: kernel socket failed", "sid", sid, "error", err)
		closePPPoxFD(s.sessions.Remove(sid))
		return
	}
	sess.PppoxFD = pppoxFD

	chanFD, unitFD, unitNum, err := ppp.DevPPPSetup(pppoxFD)
	if err != nil {
		s.logger.Error("pppoe: devPPPSetup failed", "sid", sid, "error", err)
		closePPPoxFD(s.sessions.Remove(sid))
		return
	}
	sess.UnitNum = unitNum

	// Send PADS only after kernel setup succeeds. If we sent it
	// earlier and kernel setup failed, the subscriber would wait
	// for LCP that never comes.
	var buf [EthMaxLen]byte
	frame := BuildPADS(buf[:], s.hwAddr, pkt, s.acName, sid)
	if frame == nil {
		closePPPoxFD(chanFD)
		closePPPoxFD(unitFD)
		closePPPoxFD(s.sessions.Remove(sid))
		return
	}
	s.sendFrame(frame)

	// Through the table's lock, not as a bare field write: handleSessionDown
	// marks StateTeardown from the event-consumer goroutine and
	// matchLiveCookie reads State from this one, both under that lock, so a
	// third accessor outside it would leave the field unsynchronized.
	s.sessions.markSession(sid)

	// spec-followup-l2tp-call AC-3: if a relay binding matches this service,
	// hand the subscriber to the L2TP subsystem (LAC incoming call) instead
	// of terminating PPP locally. The relay decision crosses the pppoe -> l2tp
	// boundary through the neutral callsink registry (pppoe never imports
	// l2tp). The subscriber's pppox socket fd travels with the request so the
	// L2TP side can bridge its PPP channel to the pppol2tp channel (A-4). The
	// unused local PPP unit is closed; the channel + pppox fds stay open (the
	// bridge needs them) and are released when the PPPoE session ends.
	if s.relayToL2TP(pkt, sid, pppoxFD) {
		closePPPoxFD(unitFD)
		s.logger.Info("pppoe: subscriber relayed to L2TP; local PPP not started", "sid", sid)
		return
	}

	// AuthMethod is what the AC advertises in its own LCP Configure-Request:
	// without it every subscriber reaches the accounting-only no-auth phase,
	// where an auth handler that holds credentials sees an empty username and
	// refuses. AuthRequired disconnects a client that rejects the method
	// rather than admitting it unauthenticated.
	start := ppp.StartSession{
		TunnelID:        uint16(s.ifIndex),
		SessionID:       sid,
		ChanFD:          chanFD,
		UnitFD:          unitFD,
		UnitNum:         unitNum,
		LNSMode:         true,
		AuthMethod:      s.authMethod,
		AuthRequired:    s.authRequired,
		MaxMRU:          PPPoEMaxMTU,
		AccessInterface: s.ifName,
		SubscriberMAC:   net.HardwareAddr(append([]byte(nil), pkt.SrcMAC[:]...)),
		ServiceName:     pkt.ServiceNameString(),
		VendorTags:      vendorTagsFromPacket(pkt),
	}

	s.pppDriver.SessionsIn() <- start
}

func (s *InterfaceServer) handlePADT(pkt *Packet) {
	if pkt.SID == 0 {
		return
	}

	sess := s.sessions.Lookup(pkt.SID)
	if sess == nil {
		return
	}

	if !bytes.Equal(sess.MAC, pkt.SrcMAC[:]) {
		s.logger.Warn("pppoe: PADT MAC mismatch", "sid", pkt.SID,
			"expected", sess.MAC, "got", net.HardwareAddr(pkt.SrcMAC[:]))
		return
	}

	s.logger.Info("pppoe: PADT received", "sid", pkt.SID)

	if s.pppDriver != nil {
		_ = s.pppDriver.StopSession(uint16(s.ifIndex), pkt.SID)
	}

	closePPPoxFD(s.sessions.Remove(pkt.SID))
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
func (s *InterfaceServer) handleSessionDown(sid uint16) {
	sess := s.sessions.markTeardown(sid)
	if sess == nil {
		return
	}

	var dstMAC [EthALen]byte
	copy(dstMAC[:], sess.MAC)

	var buf [EthMaxLen]byte
	frame := BuildPADT(buf[:], s.hwAddr, dstMAC, sid, s.acName)
	if frame != nil {
		s.sendFrame(frame)
	}

	closePPPoxFD(s.sessions.Remove(sid))
}

func (s *InterfaceServer) sendFrame(frame []byte) {
	if s.sendFrameFn != nil {
		s.sendFrameFn(frame)
		return
	}
	if err := sendDiscoveryFrame(s.discFD, s.ifIndex, frame); err != nil {
		s.logger.Debug("pppoe: send failed", "error", err)
	}
}

// relayToL2TP consults the registered call-sink to decide whether a
// PADS-completed subscriber should be relayed into an L2TP tunnel rather than
// terminated locally. pppoxFD is the subscriber's PPPoE pppox socket, passed
// so the L2TP side can derive its PPP channel number for the kernel bridge.
// Returns true when the subscriber was handed off to L2TP (an incoming call
// was originated). A nil sink (no L2TP subsystem) or an unmatched service
// returns false; a matched-but-failed relay logs and returns false so the
// subscriber falls back to local termination.
func (s *InterfaceServer) relayToL2TP(pkt *Packet, sid uint16, pppoxFD int) bool {
	sink := callsink.Lookup()
	if sink == nil {
		return false
	}
	accepted, err := sink.Relay(callsink.Request{
		Service:       pkt.ServiceNameString(),
		Interface:     s.ifName,
		SubscriberMAC: net.HardwareAddr(pkt.SrcMAC[:]).String(),
		SessionID:     sid,
		ChannelFD:     pppoxFD,
	})
	if err != nil {
		s.logger.Warn("pppoe: L2TP relay failed; terminating locally",
			"sid", sid, "service", pkt.ServiceNameString(), "error", err.Error())
		return false
	}
	return accepted
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
