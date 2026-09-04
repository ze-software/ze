// Design: docs/research/l2tpv2-ze-integration.md -- NCP coordinator
// RFC: rfc/short/rfc1661.md -- RFC 1661 Section 5.4 (Configure-Reject), Section 6 (Configuration Option Length and Data handling)
// Related: ip_events.go -- EventIPRequest + ipResponseMsg (handler boundary)
// Related: session.go -- pppSession NCP state fields
// Related: session_run.go -- runAuthPhase hands off to runNCPPhase
// Related: ipcp.go -- IPCP option codec
// Related: ipv6cp.go -- IPv6CP option codec + interface-id generator
// Related: ppp_fsm.go -- RFC 1661 §4 FSM reused by every NCP

package ppp

import (
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/core/rtproto"
	"github.com/ze-software/ze/internal/core/textbuf"

	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
)

// defaultIPTimeout bounds the time between emitting EventIPRequest and
// observing both (a) the handler's IPResponse and (b) the FSM reaching
// Opened. Spec-l2tp-6c-ncp AC-17 mandates a default.
const defaultIPTimeout = 30 * time.Second

// runNCPPhase drives every enabled NCP to Opened, programs the pppN
// interface (for IPv4), and emits EventSessionIPAssigned per family.
// Returns false when the session must tear down: handler rejection,
// peer Configure-Reject of IP-Address, timeout, chan fd closed,
// driver stop.
//
// AC-15: when both NCPs are disabled the session reaches EventSessionUp
// immediately after this helper returns true (logged as a config
// observation, not a crash).
//
// Deviation from AC-2 (spec text): AC-2 states the initial IPCP
// CONFREQ carries IP-Address=0.0.0.0, which matches LAC-client
// behavior. In LNS role ze assigns addresses, so ze emits
// EventIPRequest on NCP start and the FIRST CONFREQ already carries
// the assigned local address. The Deviations section of the spec
// records this choice.
func (s *pppSession) runNCPPhase() bool {
	ipcpEnabled := !s.disableIPCP
	ipv6cpEnabled := !s.disableIPv6CP

	if !ipcpEnabled && !ipv6cpEnabled {
		s.logger.Warn("ppp: both NCPs disabled, skipping NCP phase")
		return true
	}

	if ipcpEnabled {
		if !s.requestIPCPAddresses() {
			return false
		}
		s.logger.Info("ppp: IPCP addresses obtained, starting NCP FSM")
		if !s.startNCP(AddressFamilyIPv4) {
			return false
		}
		s.logger.Info("ppp: IPCP FSM started, entering loop", "state", s.ipcpState.String())
	}
	if ipv6cpEnabled {
		ok, declined := s.requestIPv6CPInterfaceID()
		if !ok {
			return false
		}
		if declined {
			// The handler has no IPv6 for this session (e.g. an
			// IPv4-only pool). Drop IPv6CP and continue with IPv4 only;
			// an independent NCP must not tear down a good session.
			s.disableIPv6CP = true
			ipv6cpEnabled = false
		} else if !s.startNCP(AddressFamilyIPv6) {
			return false
		}
	}

	timeout := s.ipTimeout
	if timeout <= 0 {
		timeout = defaultIPTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	if len(s.earlyNCPFrames) > 0 {
		s.logger.Info("ppp: draining early NCP frames", "count", len(s.earlyNCPFrames))
	}
	for _, early := range s.earlyNCPFrames {
		s.handleFrame(early)
	}
	s.earlyNCPFrames = nil

	for {
		if s.ncpsComplete(ipcpEnabled, ipv6cpEnabled) {
			return true
		}
		select {
		case <-s.stopCh:
			s.sendEvent(EventSessionDown{
				TunnelID:  s.tunnelID,
				SessionID: s.sessionID,
				Reason:    "driver stopped during NCP phase",
				Cause:     l2tpevents.TerminateCauseNASRequest,
			})
			return false
		case <-s.sessStop:
			s.sendEvent(EventSessionDown{
				TunnelID:  s.tunnelID,
				SessionID: s.sessionID,
				Reason:    "session stopped during NCP phase",
				Cause:     l2tpevents.TerminateCauseNASRequest,
			})
			return false
		case <-timer.C:
			var tb textbuf.Buffer
			s.fail(tb.Str("ncp: timeout after ").Str(timeout.String()).String())
			return false
		case frame, ok := <-s.framesIn:
			if !ok {
				s.sendEvent(EventSessionDown{
					TunnelID:  s.tunnelID,
					SessionID: s.sessionID,
					Reason:    "chan fd closed during NCP phase",
					Cause:     l2tpevents.TerminateCauseNASError,
				})
				return false
			}
			s.logger.Debug("ppp: NCP loop got frame", "len", len(frame))
			term := s.handleFrame(frame)
			putFrameBuf(frame)
			if term {
				return false
			}
		}
	}
}

// ncpsComplete reports whether every enabled NCP has reached Opened.
func (s *pppSession) ncpsComplete(ipcpEnabled, ipv6cpEnabled bool) bool {
	if ipcpEnabled && s.ipcpState != LCPStateOpened {
		return false
	}
	if ipv6cpEnabled && s.ipv6cpState != LCPStateOpened {
		return false
	}
	return true
}

// requestIPCPAddresses emits EventIPRequest for IPv4 and waits for the
// handler's IPResponse, storing local/peer/DNS into per-session state.
// Returns false on reject, timeout, or stop.
func (s *pppSession) requestIPCPAddresses() bool {
	req := EventIPRequest{
		TunnelID:  s.tunnelID,
		SessionID: s.sessionID,
		Family:    AddressFamilyIPv4,
	}
	msg, ok := s.awaitIPDecision(req, AddressFamilyIPv4)
	if !ok {
		return false
	}
	if !msg.accept {
		var tb textbuf.Buffer
		s.fail(tb.Str("ipcp: handler rejected: ").Str(msg.reason).String())
		return false
	}
	if !msg.local.Is4() || !msg.peer.Is4() {
		s.fail("ipcp: handler response missing IPv4 addresses")
		return false
	}
	s.localIPv4 = msg.local
	s.peerIPv4 = msg.peer
	s.dnsPrimary = msg.dnsPrimary
	s.dnsSecondary = msg.dnsSecondary
	return true
}

// requestIPv6CPInterfaceID generates a local 8-byte identifier (crypto/
// rand), emits EventIPRequest for IPv6, and honors a non-zero
// PeerInterfaceID override from the handler.
//
// Returns (ok, declined):
//   - ok=false: hard failure (interface-id generation, IP-decision
//     timeout, or stop); the caller tears the session down.
//   - declined=true: the handler has no IPv6 to offer this session (e.g.
//     an IPv4-only pool). IPv6CP is an independent NCP (RFC 5072; RFC
//     1661 §2 runs each Network-Layer Protocol separately), so the
//     caller drops IPv6CP and keeps an otherwise-good IPv4 session
//     rather than failing the whole session.
func (s *pppSession) requestIPv6CPInterfaceID() (ok, declined bool) {
	id, err := generateIPv6CPInterfaceID()
	if err != nil {
		var tb textbuf.Buffer
		s.fail(tb.Str("ipv6cp: interface-id generation: ").Err(err).String())
		return false, false
	}
	s.localInterfaceID = id
	req := EventIPRequest{
		TunnelID:  s.tunnelID,
		SessionID: s.sessionID,
		Family:    AddressFamilyIPv6,
	}
	msg, ok := s.awaitIPDecision(req, AddressFamilyIPv6)
	if !ok {
		return false, false
	}
	if !msg.accept {
		s.logger.Info("ppp: IPv6CP declined by handler, continuing IPv4-only",
			"reason", msg.reason)
		return true, true
	}
	if msg.hasPeerInterface {
		s.peerInterfaceID = msg.peerInterfaceID
	}
	return true, false
}

// awaitIPDecision emits req on ipEventsOut and waits for a matching-
// family response on ipRespCh, bounded by s.ipTimeout. A response
// carrying a different family is silently dropped (the handler
// responded to a future request we have not sent yet, or reordered).
func (s *pppSession) awaitIPDecision(req EventIPRequest, family AddressFamily) (ipResponseMsg, bool) {
	select {
	case s.ipEventsOut <- req:
	case <-s.stopCh:
		return ipResponseMsg{}, false
	case <-s.sessStop:
		return ipResponseMsg{}, false
	}
	timeout := s.ipTimeout
	if timeout <= 0 {
		timeout = defaultIPTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case decision := <-s.ipRespCh:
			if decision.family != family {
				s.logger.Warn("ppp: IP response for wrong family dropped",
					"want", family.String(), "got", decision.family.String())
				continue
			}
			return decision, true
		case <-s.stopCh:
			return ipResponseMsg{}, false
		case <-s.sessStop:
			return ipResponseMsg{}, false
		case <-timer.C:
			var tb textbuf.Buffer
			s.fail(tb.Str(family.String()).Str(": ip-response timeout after ").Str(timeout.String()).String())
			return ipResponseMsg{}, false
		}
	}
}

// startNCP drives the NCP FSM from Closed to ReqSent (synthetic Open)
// and sends the first CONFREQ.
func (s *pppSession) startNCP(family AddressFamily) bool {
	tr := LCPDoTransition(LCPStateClosed, LCPEventOpen)
	s.setNCPState(family, tr.NewState)
	for _, act := range tr.Actions {
		if !s.performNCPAction(family, act, LCPPacket{}) {
			return false
		}
	}
	return true
}

// setNCPState writes the per-family FSM state back into the session.
func (s *pppSession) setNCPState(family AddressFamily, st LCPState) {
	switch family {
	case AddressFamilyIPv4:
		s.ipcpState = st
	case AddressFamilyIPv6:
		s.ipv6cpState = st
	}
}

// ncpState reads the per-family FSM state.
func (s *pppSession) ncpState(family AddressFamily) LCPState {
	switch family {
	case AddressFamilyIPv4:
		return s.ipcpState
	case AddressFamilyIPv6:
		return s.ipv6cpState
	}
	return LCPStateInitial
}

// nextNCPIdentifier bumps the per-family identifier counter and
// returns the new value. RFC 1661 §5.1 requires a distinct Identifier
// per outgoing Configure-Request.
func (s *pppSession) nextNCPIdentifier(family AddressFamily) uint8 {
	switch family {
	case AddressFamilyIPv4:
		s.ipcpIdentifier++
		return s.ipcpIdentifier
	case AddressFamilyIPv6:
		s.ipv6cpIdentifier++
		return s.ipv6cpIdentifier
	}
	return 0
}

// ncpProto returns the PPP protocol value for a family.
func ncpProto(family AddressFamily) uint16 {
	switch family {
	case AddressFamilyIPv4:
		return ProtoIPCP
	case AddressFamilyIPv6:
		return ProtoIPv6CP
	}
	return 0
}

// handleIPCPPacket advances the IPCP FSM for one received packet.
// RFC 1332 §3 reuses the LCP transition table; only the option codec
// and the Protocol field differ.
func (s *pppSession) handleIPCPPacket(pkt LCPPacket) bool {
	s.logger.Debug("ppp: IPCP rx", "code", LCPCodeName(pkt.Code), "id", pkt.Identifier, "len", len(pkt.Data))
	return s.handleNCPPacket(AddressFamilyIPv4, pkt, isKnownIPCPOption,
		s.evalIPCPRequest, s.absorbIPCPNak, s.absorbIPCPReject)
}

// handleIPv6CPPacket advances the IPv6CP FSM for one received packet.
// RFC 5072 §4 reuses the LCP transition table.
func (s *pppSession) handleIPv6CPPacket(pkt LCPPacket) bool {
	return s.handleNCPPacket(AddressFamilyIPv6, pkt, isKnownIPv6CPOption,
		s.evalIPv6CPRequest, s.absorbIPv6CPNak, s.absorbIPv6CPReject)
}

// ncpRequestVerdict is what a received Configure-Request earns. A bool
// carries only the first two: RFC 1661 Section 6 has a third answer
// for a packet whose options do not fit inside it, and that answer is
// no answer at all.
type ncpRequestVerdict uint8

const (
	// ncpRequestAcceptable feeds the FSM an RCR+, which Acks.
	ncpRequestAcceptable ncpRequestVerdict = iota
	// ncpRequestUnacceptable feeds the FSM an RCR-, which Naks or Rejects.
	ncpRequestUnacceptable
	// ncpRequestDiscard feeds the FSM nothing: the packet is invalid and
	// is dropped without a reply and without a state change.
	ncpRequestDiscard
)

// handleNCPPacket is the family-generic FSM driver. known and the three
// callbacks encode the per-family option semantics (which option Types the
// family recognizes, which Reqs are acceptable, which Nak suggestions to
// absorb, which Rejects are fatal).
// absorbReject returns fatal=true to tear the session down (spec AC-16).
func (s *pppSession) handleNCPPacket(
	family AddressFamily,
	pkt LCPPacket,
	known func(uint8) bool,
	evalRequest func(LCPPacket) ncpRequestVerdict,
	absorbNak func(LCPPacket),
	absorbReject func(LCPPacket) bool,
) bool {
	cur := s.ncpState(family)

	// RFC 1661 Section 6: "When the Data field is indicated by the Length
	// to extend beyond the end of the Information field, the entire packet
	// is silently discarded without affecting the automaton."
	//
	// The sentence sits under the Data field of the Configuration Option
	// format and names no Code, where the Configure-Nak sentence one
	// paragraph above it is scoped to "received in a Configure-Request". So
	// it binds every packet whose Data field carries Configuration Options,
	// not the request alone. RFC 1332 Section 3 and RFC 5072 Section 4 both
	// adopt that option format, so it binds IPCP and IPv6CP as it binds LCP.
	//
	// The Configure-Request is held to it by evalRequest below, which has to
	// tell the fault apart from an unacceptable value to pick its verdict.
	// The three reply codes need no verdict, so they are held to it here.
	switch pkt.Code {
	case LCPConfigureAck, LCPConfigureNak, LCPConfigureReject:
		if scanNCPOptions(pkt.Data, known) == ncpOptionsTruncated {
			s.logger.Debug("ppp: NCP reply silently discarded, options do not fit the packet",
				"family", family.String(),
				"state", cur.String(),
				"code", LCPCodeName(pkt.Code),
				"id", pkt.Identifier,
				"len", len(pkt.Data))
			return false
		}
	}

	optsBad := false
	if pkt.Code == LCPConfigureRequest {
		switch evalRequest(pkt) {
		case ncpRequestDiscard:
			// RFC 1661 Section 6: "When the Data field is indicated by
			// the Length to extend beyond the end of the Information
			// field, the entire packet is silently discarded without
			// affecting the automaton."
			//
			// The silence is owed twice. Any reply to a packet an
			// off-path sender can forge makes ze a reflector, and this
			// packet is one no conforming peer emits.
			s.logger.Debug("ppp: NCP Configure-Request silently discarded",
				"family", family.String(),
				"state", cur.String(),
				"id", pkt.Identifier,
				"len", len(pkt.Data))
			return false
		case ncpRequestUnacceptable:
			optsBad = true
		case ncpRequestAcceptable:
		}
	}

	if pkt.Code == LCPConfigureNak {
		absorbNak(pkt)
	}
	if pkt.Code == LCPConfigureReject {
		if absorbReject(pkt) {
			var tb textbuf.Buffer
			s.fail(tb.Str(family.String()).Str(": peer Configure-Reject of mandatory option").String())
			return true
		}
	}

	ev := codeToEvent(pkt.Code, optsBad)
	tr := LCPDoTransition(cur, ev)
	if tr.NewState == cur && len(tr.Actions) == 0 {
		s.logger.Debug("ppp: NCP no-op transition",
			"family", family.String(),
			"state", cur.String(),
			"code", LCPCodeName(pkt.Code))
		return false
	}

	for _, act := range tr.Actions {
		if !s.performNCPAction(family, act, pkt) {
			return true
		}
	}

	s.setNCPState(family, tr.NewState)

	if tr.NewState == LCPStateOpened && cur != LCPStateOpened {
		if !s.onNCPOpened(family) {
			return true
		}
	}

	if tr.NewState == LCPStateClosed || tr.NewState == LCPStateStopped {
		var tb textbuf.Buffer
		s.fail(tb.Str(family.String()).Str(": state=").Str(tr.NewState.String()).String())
		return true
	}
	return false
}

// evalIPCPRequest judges the peer's IPCP Configure-Request.
//
// Ze in LNS role assigns peer's IP-Address. Peer's CONFREQ IP-Address
// option carries peer's own requested IP. Accept only if equal to
// s.peerIPv4; else Nak with s.peerIPv4.
func (s *pppSession) evalIPCPRequest(pkt LCPPacket) ncpRequestVerdict {
	switch scanNCPOptions(pkt.Data, isKnownIPCPOption) {
	case ncpOptionsTruncated:
		// RFC 1661 Section 6: "When the Data field is indicated by the
		// Length to extend beyond the end of the Information field, the
		// entire packet is silently discarded without affecting the
		// automaton."
		return ncpRequestDiscard
	case ncpOptionsUnknownType, ncpOptionsBadLength:
		return ncpRequestUnacceptable
	case ncpOptionsOK:
	}
	opts, err := ParseIPCPOptions(pkt.Data)
	if err != nil {
		// The list frames cleanly, so the fault is one recognized
		// option's own Length or payload width. RFC 1661 Section 6:
		// "If a negotiable Configuration Option is received in a
		// Configure-Request, but with an invalid or unrecognized
		// Length, a Configure-Nak SHOULD be transmitted which includes
		// the desired Configuration Option with an appropriate Length
		// and Data."
		return ncpRequestUnacceptable
	}
	if opts.HasIPAddress && opts.IPAddress != s.peerIPv4 {
		return ncpRequestUnacceptable
	}
	// DNS options in peer's CR: peer is requesting DNS values. Accept
	// if they match our configured values; else Nak.
	if opts.HasPrimary && opts.PrimaryDNS != s.dnsPrimary {
		return ncpRequestUnacceptable
	}
	if opts.HasSecondary && opts.SecondaryDNS != s.dnsSecondary {
		return ncpRequestUnacceptable
	}
	return ncpRequestAcceptable
}

// absorbIPCPNak applies peer's Nak suggestions to per-session state so
// the next CONFREQ reflects them.
func (s *pppSession) absorbIPCPNak(pkt LCPPacket) {
	opts, err := ParseIPCPOptions(pkt.Data)
	if err != nil {
		return
	}
	if opts.HasIPAddress {
		s.localIPv4 = opts.IPAddress
	}
	if opts.HasPrimary {
		s.dnsPrimary = opts.PrimaryDNS
	}
	if opts.HasSecondary {
		s.dnsSecondary = opts.SecondaryDNS
	}
}

// absorbIPCPReject returns fatal=true if the peer rejected IP-Address
// (mandatory per AC-16). DNS rejects are absorbed by clearing the
// option from future CONFREQs.
func (s *pppSession) absorbIPCPReject(pkt LCPPacket) bool {
	opts, err := ParseIPCPOptions(pkt.Data)
	if err != nil {
		return true
	}
	if opts.HasIPAddress {
		return true
	}
	if opts.HasPrimary {
		s.dnsPrimary = netip.Addr{}
	}
	if opts.HasSecondary {
		s.dnsSecondary = netip.Addr{}
	}
	return false
}

// evalIPv6CPRequest judges the peer's IPv6CP Configure-Request. The
// only option is Interface-Identifier; an all-zero value is always
// rejected (RFC 5072 §3.2), a collision with ze's own identifier
// triggers a Nak so the peer picks a different value.
//
// RFC 5072 Section 4: "IPV6CP uses the same Configuration Option
// format defined for LCP", so the RFC 1661 Section 6 framing rules
// bind this path exactly as they bind evalIPCPRequest.
func (s *pppSession) evalIPv6CPRequest(pkt LCPPacket) ncpRequestVerdict {
	switch scanNCPOptions(pkt.Data, isKnownIPv6CPOption) {
	case ncpOptionsTruncated:
		// RFC 1661 Section 6: "When the Data field is indicated by the
		// Length to extend beyond the end of the Information field, the
		// entire packet is silently discarded without affecting the
		// automaton."
		return ncpRequestDiscard
	case ncpOptionsUnknownType, ncpOptionsBadLength:
		return ncpRequestUnacceptable
	case ncpOptionsOK:
	}
	opts, err := parseIPv6CPOptions(pkt.Data)
	if err != nil {
		// RFC 1661 Section 6: "If a negotiable Configuration Option is
		// received in a Configure-Request, but with an invalid or
		// unrecognized Length, a Configure-Nak SHOULD be transmitted
		// which includes the desired Configuration Option with an
		// appropriate Length and Data."
		return ncpRequestUnacceptable
	}
	if !opts.HasInterfaceID {
		return ncpRequestAcceptable
	}
	if !isValidIPv6CPInterfaceID(opts.InterfaceID) {
		return ncpRequestUnacceptable
	}
	if opts.InterfaceID == s.localInterfaceID {
		return ncpRequestUnacceptable
	}
	s.peerInterfaceID = opts.InterfaceID
	return ncpRequestAcceptable
}

// absorbIPv6CPNak applies peer's Nak-suggested Interface-Identifier.
func (s *pppSession) absorbIPv6CPNak(pkt LCPPacket) {
	opts, err := parseIPv6CPOptions(pkt.Data)
	if err != nil {
		return
	}
	if opts.HasInterfaceID && isValidIPv6CPInterfaceID(opts.InterfaceID) {
		s.localInterfaceID = opts.InterfaceID
	}
}

// absorbIPv6CPReject: Interface-Identifier is mandatory; peer rejecting
// it is fatal.
func (s *pppSession) absorbIPv6CPReject(pkt LCPPacket) bool {
	opts, err := parseIPv6CPOptions(pkt.Data)
	if err != nil {
		return true
	}
	return opts.HasInterfaceID
}

// performNCPAction translates one FSM action into wire I/O on the
// chan fd for the given family.
func (s *pppSession) performNCPAction(family AddressFamily, act LCPAction, current LCPPacket) bool {
	switch act {
	case LCPActSCR:
		return s.sendNCPConfigureRequest(family)
	case LCPActSCA:
		return s.sendNCPConfigureAck(family, current)
	case LCPActSCN:
		return s.sendNCPConfigureNakOrReject(family, current)
	case LCPActSTR:
		return s.sendNCPTerminateRequest(family)
	case LCPActSTA:
		return s.sendNCPTerminateAck(family, current)
	case LCPActSCJ:
		return s.sendNCPCodeReject(family, current)
	case LCPActSER:
		// NCPs never emit Echo-Request (RFC 5072 §3 restricts Codes
		// 8-11); SER should never fire, but stay defensive.
		return true
	case LCPActIRC, LCPActZRC, LCPActTLU, LCPActTLD, LCPActTLS, LCPActTLF:
		return true
	}
	return true
}

// sendNCPConfigureRequest encodes a CONFREQ for the given family using
// the per-session negotiated state and sends it. Increments the
// per-family identifier.
func (s *pppSession) sendNCPConfigureRequest(family AddressFamily) bool {
	id := s.nextNCPIdentifier(family)
	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ncpProto(family), nil)
	dataOff := off + lcpHeaderLen
	dataLen := s.writeNCPOptions(family, buf, dataOff)
	off += WriteLCPPacket(buf, off, LCPConfigureRequest, id, buf[dataOff:dataOff+dataLen])
	return s.writeFrame(buf[:off])
}

// writeNCPOptions serializes the family's outbound options into buf at
// off and returns bytes written.
func (s *pppSession) writeNCPOptions(family AddressFamily, buf []byte, off int) int {
	switch family {
	case AddressFamilyIPv4:
		// RFC 1332: ConfReq carries only our own IP-Address.
		// DNS is communicated via Nak to the peer's ConfReq (RFC 1877).
		opts := iPCPOptions{
			IPAddress:    s.localIPv4,
			HasIPAddress: s.localIPv4.IsValid(),
		}
		return WriteIPCPOptions(buf, off, opts)
	case AddressFamilyIPv6:
		opts := iPv6CPOptions{
			InterfaceID:    s.localInterfaceID,
			HasInterfaceID: true,
		}
		return writeIPv6CPOptions(buf, off, opts)
	}
	return 0
}

// sendNCPConfigureAck echoes the peer's Configure-Request option Data
// back verbatim.
func (s *pppSession) sendNCPConfigureAck(family AddressFamily, req LCPPacket) bool {
	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ncpProto(family), nil)
	off += WriteLCPPacket(buf, off, LCPConfigureAck, req.Identifier, req.Data)
	return s.writeFrame(buf[:off])
}

// sendNCPConfigureNakOrReject inspects the peer's Request and replies
// Nak (when we can suggest a better value) or Reject (for unknown
// option types).
func (s *pppSession) sendNCPConfigureNakOrReject(family AddressFamily, req LCPPacket) bool {
	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ncpProto(family), nil)
	dataOff := off + lcpHeaderLen
	code, dataLen := s.buildNakOrReject(family, req, buf, dataOff)
	off += WriteLCPPacket(buf, off, code, req.Identifier, buf[dataOff:dataOff+dataLen])
	return s.writeFrame(buf[:off])
}

// buildNakOrReject picks Nak vs Reject and fills the response Data.
// Returns (code, dataLen).
func (s *pppSession) buildNakOrReject(family AddressFamily, req LCPPacket, buf []byte, off int) (uint8, int) {
	switch family {
	case AddressFamilyIPv4:
		// RFC 1661 Section 5.4: "If some Configuration Options received
		// in a Configure-Request are not recognizable or are not
		// acceptable for negotiation (as configured by a network
		// administrator), then the implementation MUST transmit a
		// Configure-Reject."
		//
		// Only an unrecognized Type earns that. An invalid Length is
		// Section 6's case and falls through to the Nak below.
		if scanNCPOptions(req.Data, isKnownIPCPOption) == ncpOptionsUnknownType {
			dataLen := copyUnknownOptions(req.Data, isKnownIPCPOption, buf, off)
			return LCPConfigureReject, dataLen
		}
		// RFC 1661 Section 6: "If a negotiable Configuration Option is
		// received in a Configure-Request, but with an invalid or
		// unrecognized Length, a Configure-Nak SHOULD be transmitted
		// which includes the desired Configuration Option with an
		// appropriate Length and Data." The Nak below carries ze's
		// assigned address and DNS servers, each at its own correct
		// Length, which is that desired option set.
		nak := iPCPOptions{
			IPAddress:    s.peerIPv4,
			HasIPAddress: s.peerIPv4.IsValid(),
			PrimaryDNS:   s.dnsPrimary,
			HasPrimary:   s.dnsPrimary.IsValid(),
			SecondaryDNS: s.dnsSecondary,
			HasSecondary: s.dnsSecondary.IsValid(),
		}
		return LCPConfigureNak, WriteIPCPOptions(buf, off, nak)
	case AddressFamilyIPv6:
		// RFC 1661 Section 5.4: "If some Configuration Options received
		// in a Configure-Request are not recognizable or are not
		// acceptable for negotiation (as configured by a network
		// administrator), then the implementation MUST transmit a
		// Configure-Reject."
		if scanNCPOptions(req.Data, isKnownIPv6CPOption) == ncpOptionsUnknownType {
			dataLen := copyUnknownOptions(req.Data, isKnownIPv6CPOption, buf, off)
			return LCPConfigureReject, dataLen
		}
		// RFC 1661 Section 6: "If a negotiable Configuration Option is
		// received in a Configure-Request, but with an invalid or
		// unrecognized Length, a Configure-Nak SHOULD be transmitted
		// which includes the desired Configuration Option with an
		// appropriate Length and Data."
		nak := iPv6CPOptions{
			InterfaceID:    s.peerInterfaceID,
			HasInterfaceID: true,
		}
		return LCPConfigureNak, writeIPv6CPOptions(buf, off, nak)
	}
	return LCPConfigureReject, 0
}

// ncpOptionScan is the verdict of one walk over the option list of a
// received Configure-Request.
//
// RFC 1661 Section 6 answers three malformed shapes three different
// ways -- Configure-Reject an unrecognized Type, Configure-Nak an
// invalid Length, discard the whole packet when an option is not
// contained in it -- so a bool cannot carry the answer. A bool also
// has to spend one of its two values on "I could not parse this",
// which every caller reads as "every option was recognized"
// (ai/rules/principles.md: a value that is silently wrong must not be
// reachable).
type ncpOptionScan uint8

const (
	// ncpOptionsOK: the list walks cleanly and every Type is known.
	ncpOptionsOK ncpOptionScan = iota
	// ncpOptionsUnknownType: the list walks cleanly and carries at
	// least one Type this NCP does not recognize.
	ncpOptionsUnknownType
	// ncpOptionsBadLength: an option declares a Length smaller than the
	// two octets its own Type and Length fields occupy.
	ncpOptionsBadLength
	// ncpOptionsTruncated: an option is not contained in the packet --
	// its Data runs past the end, or its header does not fit.
	ncpOptionsTruncated
)

// scanNCPOptions classifies the option list of a received NCP
// Configure-Request. buf is the LCP Data field, which ParseLCPPacket
// has already cut to the packet's own Length field, so "past the end
// of buf" is "past the end of the Information field".
//
// One walker serves both NCPs: RFC 1332 Section 3 says "IPCP uses the
// same Configuration Option format defined for LCP", and RFC 5072
// Section 4 says the same for IPV6CP. Only the known() predicate
// differs.
//
// Truncation outranks an unrecognized Type. The packet is invalid as a
// whole, and RFC 1661 Section 6 discards it instead of answering it,
// so the walk completes its framing checks before it reports a Type it
// does not know.
//
// An unrecognized Type in turn outranks an invalid Length, because RFC
// 1661 Section 5.4 says the implementation "MUST transmit a
// Configure-Reject" for it while Section 6 only says an invalid Length
// SHOULD draw a Configure-Nak. The walk stops at the invalid Length
// either way: a Length below the header cannot advance the walk, so an
// unrecognized Type after the fault is one this scan never sees.
func scanNCPOptions(buf []byte, known func(uint8) bool) ncpOptionScan {
	unknown := false
	off := 0
	for off < len(buf) {
		// RFC 1661 Section 6: "When the Data field is indicated by the
		// Length to extend beyond the end of the Information field, the
		// entire packet is silently discarded without affecting the
		// automaton." One remaining octet cannot hold the Type and the
		// Length, so the option's own Length field lies past that end.
		if len(buf)-off < 2 {
			return ncpOptionsTruncated
		}
		optLen := int(buf[off+1])
		// RFC 1661 Section 6: "The Length field is one octet, and
		// indicates the length of this Configuration Option including
		// the Type, Length and Data fields." A Length below 2 counts
		// fewer octets than the option's own header holds.
		if optLen < 2 {
			// RFC 1661 Section 5.4: "If some Configuration Options
			// received in a Configure-Request are not recognizable or
			// are not acceptable for negotiation (as configured by a
			// network administrator), then the implementation MUST
			// transmit a Configure-Reject." A Type already read and not
			// recognized owes that Reject, and the MUST outranks the
			// Nak Section 6 asks for here.
			if unknown {
				return ncpOptionsUnknownType
			}
			return ncpOptionsBadLength
		}
		// RFC 1661 Section 6: "When the Data field is indicated by the
		// Length to extend beyond the end of the Information field, the
		// entire packet is silently discarded without affecting the
		// automaton."
		if off+optLen > len(buf) {
			return ncpOptionsTruncated
		}
		if !known(buf[off]) {
			unknown = true
		}
		off += optLen
	}
	if unknown {
		return ncpOptionsUnknownType
	}
	return ncpOptionsOK
}

// copyUnknownOptions walks src and copies every option whose type is
// not known (predicate returns false) into buf at off. Used to build
// the Data field of a Configure-Reject per RFC 1661 §5.4.
//
// Callers reach this only after scanNCPOptions has answered
// ncpOptionsUnknownType for the same src, so the framing guards below
// never fire; they keep the walk inside src if that ever changes.
func copyUnknownOptions(src []byte, known func(uint8) bool, buf []byte, off int) int {
	start := off
	i := 0
	for i < len(src) {
		if len(src)-i < 2 {
			break
		}
		l := int(src[i+1])
		if l < 2 || i+l > len(src) {
			break
		}
		if !known(src[i]) {
			n := copy(buf[off:], src[i:i+l])
			off += n
		}
		i += l
	}
	return off - start
}

// isKnownIPv6CPOption is the scanNCPOptions predicate for IPv6CP.
func isKnownIPv6CPOption(t uint8) bool {
	return t == IPv6CPOptInterfaceID
}

// sendNCPTerminateRequest sends an NCP Terminate-Request.
func (s *pppSession) sendNCPTerminateRequest(family AddressFamily) bool {
	id := s.nextNCPIdentifier(family)
	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ncpProto(family), nil)
	off += WriteLCPPacket(buf, off, LCPTerminateRequest, id, nil)
	return s.writeFrame(buf[:off])
}

// sendNCPTerminateAck sends an NCP Terminate-Ack for the peer's Req.
func (s *pppSession) sendNCPTerminateAck(family AddressFamily, req LCPPacket) bool {
	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ncpProto(family), nil)
	off += WriteLCPPacket(buf, off, LCPTerminateAck, req.Identifier, nil)
	return s.writeFrame(buf[:off])
}

// sendNCPCodeReject replies with a Code-Reject carrying the rejected
// packet verbatim, matching sendCodeReject's format.
func (s *pppSession) sendNCPCodeReject(family AddressFamily, req LCPPacket) bool {
	body := make([]byte, lcpHeaderLen+len(req.Data))
	body[0] = req.Code
	body[1] = req.Identifier
	ncpSetLength(body, uint16(lcpHeaderLen+len(req.Data)))
	copy(body[lcpHeaderLen:], req.Data)

	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ncpProto(family), nil)
	off += WriteLCPPacket(buf, off, LCPCodeReject, req.Identifier, body)
	return s.writeFrame(buf[:off])
}

// ncpSetLength writes a big-endian uint16 at body[2:4].
func ncpSetLength(body []byte, length uint16) {
	body[2] = byte(length >> 8)
	body[3] = byte(length)
}

// onNCPOpened runs per-family post-Opened side effects. For IPv4 this
// programs pppN with the assigned address and peer route. For IPv6 no
// backend call is made (kernel auto-derives link-local). Both emit
// EventSessionIPAssigned.
func (s *pppSession) onNCPOpened(family AddressFamily) bool {
	ifname := textbuf.StrInt("ppp", int64(s.unitNum))
	switch family {
	case AddressFamilyIPv4:
		var tb textbuf.Buffer
		localCIDR := tb.Addr(s.localIPv4).Str("/32").String()
		peerCIDR := tb.Reset().Addr(s.peerIPv4).Str("/32").String()
		if err := s.backend.AddAddressP2P(ifname, localCIDR, peerCIDR); err != nil {
			s.fail(tb.Reset().Str("iface AddAddressP2P: ").Err(err).String())
			return false
		}
		s.sendEvent(EventSessionIPAssigned{
			TunnelID:     s.tunnelID,
			SessionID:    s.sessionID,
			Family:       AddressFamilyIPv4,
			Local:        s.localIPv4,
			Peer:         s.peerIPv4,
			DNSPrimary:   s.dnsPrimary,
			DNSSecondary: s.dnsSecondary,
		})
	case AddressFamilyIPv6:
		s.sendEvent(EventSessionIPAssigned{
			TunnelID:    s.tunnelID,
			SessionID:   s.sessionID,
			Family:      AddressFamilyIPv6,
			InterfaceID: s.peerInterfaceID,
		})
	}
	return true
}

// teardownNCPResources removes the IPCP-programmed address and route
// on session exit. Called from the run() defer chain when IPCP has
// reached Opened (s.ipcpState == LCPStateOpened). Best-effort: errors
// are logged but do not block teardown.
func (s *pppSession) teardownNCPResources() {
	if s.ipv6Svc != nil {
		s.ipv6Svc.Stop()
		s.ipv6Svc = nil
	}
	if s.ipcpState != LCPStateOpened {
		return
	}
	ifname := textbuf.StrInt("ppp", int64(s.unitNum))
	var tb textbuf.Buffer
	peerCIDR := tb.Addr(s.peerIPv4).Str("/32").String()
	localCIDR := tb.Reset().Addr(s.localIPv4).Str("/32").String()
	if err := s.backend.RemoveRoute(ifname, peerCIDR, "", 0, rtproto.Iface); err != nil {
		s.logger.Debug("ppp: RemoveRoute on teardown", "error", err.Error())
	}
	if err := s.backend.RemoveAddress(ifname, localCIDR); err != nil {
		s.logger.Debug("ppp: RemoveAddress on teardown", "error", err.Error())
	}
}
