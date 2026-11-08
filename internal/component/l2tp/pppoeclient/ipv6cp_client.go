// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- IPv6CP in the network phase
// Related: network_phase.go -- keepaliveLoop feeds IPv6CP frames and restart ticks here
// Related: connected_session.go -- publishes the link-local address with the session result
//
// RFC: rfc/short/rfc5072.md

package pppoeclient

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// ipv6cpRestartInterval is the IPv6CP Restart timer, the RFC 1661 Section 4.6
// default of three seconds that IPCP negotiation uses too.
const ipv6cpRestartInterval = 3 * time.Second

// ipv6cpConfigureMax bounds the Configure-Requests sent before IPv6CP gives
// up: the RFC 1661 Section 4.6 Max-Configure default of ten. IPCP is not
// touched when the count runs out (RFC 7084 WLL-3).
const ipv6cpConfigureMax = 10

// ipv6cpEvent says what one IPv6CP input did to the negotiated link. Zero is
// invalid, so a caller never reads a forgotten result as "unchanged".
type ipv6cpEvent uint8

const (
	ipv6cpEventUnspecified ipv6cpEvent = iota
	ipv6cpUnchanged
	ipv6cpOpened
	ipv6cpClosed
)

// clientIPv6CP is the PPPoE client's IPv6CP automaton (RFC 5072 over the RFC
// 1661 option negotiation). It runs beside IPCP and ends the session only
// when IPCP is down too (connectedSession.ncpsRemaining): a peer that rejects
// or ignores IPv6CP leaves IPv4 up.
//
// RFC 7084 Section 4.2 WLL-3: "If the WAN interface supports PPP
// encapsulation, in a dual-stack environment with IPCP and IPV6CP running
// over one PPP logical channel, the Network Control Protocols (NCPs) MUST be
// treated as independent of each other and start and terminate
// independently."
//
// One worker owns it at a time: negotiateIPCP while IPCP negotiates (IPv6CP
// starts there, beside IPCP), then keepaliveLoop. It is not safe for
// concurrent use.
// newClientIPv6CP is its only producer.
type clientIPv6CP struct {
	state ppp.LCPState
	// identifier is the Identifier of the last Configure-Request sent;
	// codeRejectID the Identifier of the last Code-Reject sent.
	identifier   uint8
	codeRejectID uint8
	// local is the interface identifier of the last Configure-Request sent.
	local ppp.IPv6CPInterfaceID
	// nakSent is the suggestion of the last Configure-Nak sent to the peer,
	// valid only while hasNakSent.
	nakSent    ppp.IPv6CPInterfaceID
	hasNakSent bool
	// requests counts Configure-Requests sent since the last restart of the
	// counter, bounded by ipv6cpConfigureMax.
	requests int
	// mac seeds the tentative identifier: the source interface's EUI-48.
	mac     []byte
	peerMRU uint16
}

// newClientIPv6CP builds the automaton for one session. mac is the hardware
// address of the PPPoE source interface; a nil or non-EUI-48 value falls back
// to a random identifier (ppp.TentativeIPv6CPInterfaceID).
func newClientIPv6CP(mac []byte, peerMRU uint16) *clientIPv6CP {
	return &clientIPv6CP{state: ppp.LCPStateInitial, mac: append([]byte(nil), mac...), peerMRU: peerMRU}
}

// opened reports whether IPv6CP is in the Opened state.
func (c *clientIPv6CP) opened() bool { return c.state == ppp.LCPStateOpened }

// linkLocal is the link-local address of the local end while Opened, and the
// invalid address otherwise.
func (c *clientIPv6CP) linkLocal() netip.Addr {
	if c.state != ppp.LCPStateOpened {
		return netip.Addr{}
	}
	return ppp.IPv6LinkLocal(c.local)
}

// start sends the first Configure-Request of an IPv6CP epoch (RFC 1661
// Section 4.1, the Up and Open events: irc, scr, Req-Sent). It chooses the
// tentative identifier afresh, so an LCP restart begins a new negotiation.
func (c *clientIPv6CP) start(w io.Writer, buf []byte) error {
	id, err := ppp.TentativeIPv6CPInterfaceID(c.mac)
	if err != nil {
		return fmt.Errorf("pppoe-client: IPv6CP interface identifier: %w", err)
	}
	c.local = id
	c.hasNakSent = false
	c.requests = 0
	c.state = ppp.LCPStateReqSent
	return c.sendRequest(w, buf)
}

// lowerDown takes IPv6CP down for an LCP restart (RFC 1661 Section 4.1, the
// Down event). It reports ipv6cpClosed when the link was Opened.
func (c *clientIPv6CP) lowerDown() ipv6cpEvent {
	wasOpened := c.opened()
	c.state = ppp.LCPStateInitial
	if wasOpened {
		return ipv6cpClosed
	}
	return ipv6cpUnchanged
}

// running reports whether IPv6CP is Opened or still on its way there or out:
// every RFC 1661 Section 4.1 state before This-Layer-Finished. Initial,
// Starting, Closed and Stopped have finished, so IPv6CP then keeps no link up
// (connectedSession.ncpsRemaining).
func (c *clientIPv6CP) running() bool {
	switch c.state {
	case ppp.LCPStateReqSent, ppp.LCPStateAckRcvd, ppp.LCPStateAckSent, ppp.LCPStateOpened,
		ppp.LCPStateClosing, ppp.LCPStateStopping:
		return true
	case ppp.LCPStateInitial, ppp.LCPStateStarting, ppp.LCPStateClosed, ppp.LCPStateStopped:
		return false
	}
	panic("BUG: invalid IPv6CP automaton state")
}

// protocolRejected stops IPv6CP after an LCP Protocol-Reject that names it.
// RFC 1661 Section 5.7: "Upon reception of a Protocol-Reject, the
// implementation MUST stop sending packets of the indicated protocol at the
// earliest opportunity." A later IPv6CP Configure-Request from the peer
// restarts the automaton, because the peer then speaks the protocol.
func (c *clientIPv6CP) protocolRejected() ipv6cpEvent {
	wasOpened := c.opened()
	c.state = ppp.LCPStateStopped
	if wasOpened {
		return ipv6cpClosed
	}
	return ipv6cpUnchanged
}

// tick runs the Restart timer. RFC 1661 Section 4.1: TO+ resends the
// Configure-Request (Ack-Rcvd returns to Req-Sent), and TO- in Req-Sent,
// Ack-Rcvd or Ack-Sent moves to Stopped with no further request. An ignored
// IPv6CP therefore stops after ipv6cpConfigureMax requests and IPCP stays.
func (c *clientIPv6CP) tick(w io.Writer, buf []byte, logger *slog.Logger) error {
	// Only the states this input changes are listed; the rest keep their state.
	//exhaustive:ignore
	switch c.state {
	case ppp.LCPStateReqSent, ppp.LCPStateAckRcvd, ppp.LCPStateAckSent:
	default:
		// Only the three negotiating states run the Restart timer.
		return nil
	}
	if c.requests >= ipv6cpConfigureMax {
		c.state = ppp.LCPStateStopped
		logger.Warn("pppoe-client: the access concentrator did not complete IPv6CP, so the session carries IPv4 only; enable IPv6 for PPPoE sessions on the access concentrator",
			"requests", c.requests)
		return nil
	}
	if c.state == ppp.LCPStateAckRcvd {
		c.state = ppp.LCPStateReqSent
	}
	return c.sendRequest(w, buf)
}

// sendRequest writes a Configure-Request carrying the one Interface-Identifier
// option. RFC 5072 Section 4.1: "A Configure-Request MUST contain exactly one
// instance of the interface-identifier option [1]."
//
// RFC 5072 Section 4 packet layout after the 2-octet PPP Protocol 0x8057:
//
//	0 Code | 1 Identifier | 2..3 Length | 4 Type 1 | 5 Length 10 | 6..13 Interface-Identifier
func (c *clientIPv6CP) sendRequest(w io.Writer, buf []byte) error {
	// RFC 1661 Section 5.1: "On transmission, the Identifier field MUST be
	// changed whenever the content of the Data field changes, and whenever a
	// valid reply has been received for a previous request."
	c.identifier++
	c.requests++
	var option [ppp.IPv6CPInterfaceIDOptionLen]byte
	ppp.WriteIPv6CPInterfaceID(option[:], 0, c.local)
	off := ppp.WriteFrame(buf, 0, ppp.ProtoIPv6CP, nil)
	off += ppp.WriteLCPPacket(buf, off, ppp.LCPConfigureRequest, c.identifier, option[:])
	if err := writeClientPacket(w, buf[:off]); err != nil {
		return fmt.Errorf("pppoe-client: IPv6CP Configure-Request: %w", err)
	}
	return nil
}

// requestOptions is the option list of the outstanding Configure-Request,
// which a Configure-Ack or Configure-Reject must match (ValidateLCPReply).
func (c *clientIPv6CP) requestOptions() [ppp.IPv6CPInterfaceIDOptionLen]byte {
	var option [ppp.IPv6CPInterfaceIDOptionLen]byte
	ppp.WriteIPv6CPInterfaceID(option[:], 0, c.local)
	return option
}

// handle answers one IPv6CP packet and reports what it did to the link. An
// error means a write failed, and the session ends as for any failed write.
func (c *clientIPv6CP) handle(w io.Writer, buf []byte, pkt ppp.LCPPacket, logger *slog.Logger) (ipv6cpEvent, error) {
	// RFC 5072 Section 3: "Other Codes should be treated as unrecognized and
	// should result in Code-Rejects."
	if ipcpCodeUnrecognized(pkt.Code) {
		// RFC 1661 Section 5.6: "The Identifier field MUST be changed for
		// each Code-Reject sent."
		c.codeRejectID++
		if err := sendNCPCodeReject(w, buf, ppp.ProtoIPv6CP, pkt, c.codeRejectID, c.peerMRU); err != nil {
			return ipv6cpEventUnspecified, err
		}
		return ipv6cpUnchanged, nil
	}
	switch pkt.Code {
	case ppp.LCPConfigureRequest:
		return c.receiveRequest(w, buf, pkt)
	case ppp.LCPConfigureAck:
		return c.receiveAck(w, buf, pkt)
	case ppp.LCPConfigureNak:
		return c.receiveNak(w, buf, pkt)
	case ppp.LCPConfigureReject:
		return c.receiveReject(pkt, logger)
	case ppp.LCPTerminateRequest:
		// RFC 1661 Section 5.5: "Upon reception of a Terminate-Request, a
		// Terminate-Ack MUST be transmitted."
		off := ppp.WriteFrame(buf, 0, ppp.ProtoIPv6CP, nil)
		off += ppp.WriteLCPPacket(buf, off, ppp.LCPTerminateAck, pkt.Identifier, nil)
		if err := writeClientPacket(w, buf[:off]); err != nil {
			return ipv6cpEventUnspecified, fmt.Errorf("pppoe-client: IPv6CP Terminate-Ack: %w", err)
		}
		wasOpened := c.opened()
		// RFC 1661 Section 4.1 RTR: Opened goes to Stopping and then Stopped
		// once the peer's Terminate exchange ends; the negotiating states
		// return to Req-Sent. The peer's next Configure-Request restarts.
		if wasOpened {
			c.state = ppp.LCPStateStopped
			logger.Info("pppoe-client: the access concentrator closed IPv6CP; IPv4 continues")
			return ipv6cpClosed, nil
		}
		if c.state == ppp.LCPStateAckRcvd || c.state == ppp.LCPStateAckSent {
			c.state = ppp.LCPStateReqSent
		}
		return ipv6cpUnchanged, nil
	case ppp.LCPTerminateAck:
		// RFC 1661 Section 4.1 RTA: Opened restarts negotiation (tld, scr).
		if c.opened() {
			c.state = ppp.LCPStateReqSent
			return ipv6cpClosed, c.sendRequest(w, buf)
		}
		if c.state == ppp.LCPStateAckRcvd {
			c.state = ppp.LCPStateReqSent
		}
		return ipv6cpUnchanged, nil
	case ppp.LCPCodeReject:
		// RFC 1661 Section 4.1 RXJ+: a Code-Reject of a code this client
		// sends only optionally is permitted and changes no state; IPv6CP
		// sends no optional code.
		return ipv6cpUnchanged, nil
	}
	return ipv6cpEventUnspecified, fmt.Errorf("pppoe-client: IPv6CP code %d passed the code check unhandled", pkt.Code)
}

// receiveRequest answers the peer's Configure-Request (RFC 1661 Section 4.1
// RCR+ and RCR-), comparing its identifier with local as RFC 5072 Section 4.1
// orders.
func (c *clientIPv6CP) receiveRequest(w io.Writer, buf []byte, pkt ppp.LCPPacket) (ipv6cpEvent, error) {
	peerID, present, err := ppp.ParseIPv6CPInterfaceID(pkt.Data)
	if err != nil {
		// RFC 1661 Section 6: an option list whose lengths overrun the packet,
		// or a second Interface-Identifier, is discarded unanswered.
		return ipv6cpUnchanged, nil
	}
	event := ipv6cpUnchanged
	// Only the states this input changes are listed; the rest keep their state.
	//exhaustive:ignore
	switch c.state {
	case ppp.LCPStateOpened:
		// RFC 1661 Section 4.1 RCR in Opened: tld, scr, then sca or scn.
		event = ipv6cpClosed
		c.state = ppp.LCPStateReqSent
		if err := c.sendRequest(w, buf); err != nil {
			return ipv6cpEventUnspecified, err
		}
	case ppp.LCPStateStopped, ppp.LCPStateInitial, ppp.LCPStateClosed:
		// RFC 1661 Section 4.1 RCR in Stopped: irc, scr, then sca or scn.
		// The peer speaks IPv6CP again, so negotiation restarts.
		if err := c.start(w, buf); err != nil {
			return ipv6cpEventUnspecified, err
		}
	}

	// Options other than the Interface-Identifier are rejected verbatim
	// (RFC 1661 Section 5.4). ParseIPv6CPInterfaceID checked each boundary.
	rejected := 0
	room := buf[6:]
	for data := pkt.Data; len(data) != 0; {
		length := int(data[1])
		if data[0] != ppp.IPv6CPOptInterfaceID {
			rejected += copy(room[rejected:], data[:length])
		}
		data = data[length:]
	}
	if rejected != 0 {
		return event, c.answerRequest(w, buf, ppp.LCPConfigureReject, pkt.Identifier, room[:rejected])
	}
	if !present {
		// The peer negotiates no identifier of its own. Section 4.1 lets a
		// side that requires one Nak it in; this client does not require the
		// peer's identifier, so the request is acceptable as it stands.
		return c.ackRequest(w, buf, pkt, event)
	}
	var option [ppp.IPv6CPInterfaceIDOptionLen]byte
	zero := ppp.IPv6CPInterfaceID{}
	if peerID == zero {
		if c.local == zero {
			// RFC 5072 Section 4.1: "If the two interface identifiers are
			// equal to zero, the interface identifier's negotiation MUST be
			// terminated by transmitting the Configure-Reject with the
			// interface-identifier value set to zero."
			ppp.WriteIPv6CPInterfaceID(option[:], 0, zero)
			return event, c.answerRequest(w, buf, ppp.LCPConfigureReject, pkt.Identifier, option[:])
		}
		// RFC 5072 Section 4.1: "If the two interface identifiers are
		// different but the received interface identifier is zero, a
		// Configure-Nak is sent with a non-zero interface-identifier value
		// suggested for use by the remote peer."
		return event, c.nakRequest(w, buf, pkt.Identifier)
	}
	if peerID == c.local {
		// RFC 5072 Section 4.1: "If the two interface identifiers are equal
		// and are not zero, Configure-Nak MUST be sent specifying a different
		// non-zero interface-identifier value suggested for use by the remote
		// peer."
		return event, c.nakRequest(w, buf, pkt.Identifier)
	}
	// RFC 5072 Section 4.1: "If the two interface identifiers are different
	// and the received interface identifier is not zero, the interface
	// identifier MUST be acknowledged, i.e., a Configure-Ack is sent with the
	// requested interface identifier, meaning that the responding peer agrees
	// with the interface identifier requested."
	return c.ackRequest(w, buf, pkt, event)
}

// ackRequest sends the Configure-Ack (sca) and moves the automaton: Req-Sent
// to Ack-Sent, Ack-Rcvd to Opened. prior is what the request already did to
// the link, which an Opened result replaces.
func (c *clientIPv6CP) ackRequest(w io.Writer, buf []byte, pkt ppp.LCPPacket, prior ipv6cpEvent) (ipv6cpEvent, error) {
	if err := c.answerRequest(w, buf, ppp.LCPConfigureAck, pkt.Identifier, pkt.Data); err != nil {
		return ipv6cpEventUnspecified, err
	}
	if c.state == ppp.LCPStateAckRcvd {
		c.state = ppp.LCPStateOpened
		return ipv6cpOpened, nil
	}
	c.state = ppp.LCPStateAckSent
	return prior, nil
}

// nakRequest sends a Configure-Nak suggesting an identifier for the peer
// (scn); Ack-Sent returns to Req-Sent (RFC 1661 Section 4.1 RCR-).
//
// RFC 5072 Section 4.1: "Such a suggested interface identifier MUST be
// different from the interface identifier of the last Configure-Request sent
// to the peer." ppp.SuggestIPv6CPInterfaceID draws it distinct from local,
// with the "u" bit cleared.
func (c *clientIPv6CP) nakRequest(w io.Writer, buf []byte, identifier uint8) error {
	suggestion, err := ppp.SuggestIPv6CPInterfaceID(c.local)
	if err != nil {
		return fmt.Errorf("pppoe-client: IPv6CP Nak suggestion: %w", err)
	}
	var option [ppp.IPv6CPInterfaceIDOptionLen]byte
	ppp.WriteIPv6CPInterfaceID(option[:], 0, suggestion)
	if err := c.answerRequest(w, buf, ppp.LCPConfigureNak, identifier, option[:]); err != nil {
		return err
	}
	c.nakSent = suggestion
	c.hasNakSent = true
	if c.state == ppp.LCPStateAckSent {
		c.state = ppp.LCPStateReqSent
	}
	return nil
}

// answerRequest writes one reply to a Configure-Request. data MAY alias buf
// past the 6-octet frame and packet header: WriteLCPPacket copies it onto
// itself there.
func (c *clientIPv6CP) answerRequest(w io.Writer, buf []byte, code, identifier uint8, data []byte) error {
	off := ppp.WriteFrame(buf, 0, ppp.ProtoIPv6CP, nil)
	off += ppp.WriteLCPPacket(buf, off, code, identifier, data)
	if err := writeClientPacket(w, buf[:off]); err != nil {
		return fmt.Errorf("pppoe-client: IPv6CP %s: %w", ppp.LCPCodeName(code), err)
	}
	return nil
}

// receiveAck moves the automaton on a Configure-Ack that matches the
// outstanding request byte for byte (RFC 1661 Section 5.2).
func (c *clientIPv6CP) receiveAck(w io.Writer, buf []byte, pkt ppp.LCPPacket) (ipv6cpEvent, error) {
	options := c.requestOptions()
	if !ppp.ValidateLCPReply(pkt, c.identifier, options[:]) {
		return ipv6cpUnchanged, nil
	}
	// Only the states this input changes are listed; the rest keep their state.
	//exhaustive:ignore
	switch c.state {
	case ppp.LCPStateReqSent:
		c.requests = 0
		c.state = ppp.LCPStateAckRcvd
	case ppp.LCPStateAckSent:
		c.requests = 0
		c.state = ppp.LCPStateOpened
		return ipv6cpOpened, nil
	case ppp.LCPStateAckRcvd:
		// RFC 1661 Section 4.1 RCA in Ack-Rcvd: scr, Req-Sent (crossed).
		c.state = ppp.LCPStateReqSent
		return ipv6cpUnchanged, c.sendRequest(w, buf)
	case ppp.LCPStateOpened:
		// RFC 1661 Section 4.1 RCA in Opened: tld, scr, Req-Sent.
		c.state = ppp.LCPStateReqSent
		return ipv6cpClosed, c.sendRequest(w, buf)
	}
	return ipv6cpUnchanged, nil
}

// receiveNak adopts the identifier the peer suggests and sends a new request
// (RFC 1661 Section 4.1 RCN).
func (c *clientIPv6CP) receiveNak(w io.Writer, buf []byte, pkt ppp.LCPPacket) (ipv6cpEvent, error) {
	if pkt.Identifier != c.identifier {
		return ipv6cpUnchanged, nil
	}
	// Only the states this input changes are listed; the rest keep their state.
	//exhaustive:ignore
	switch c.state {
	case ppp.LCPStateReqSent, ppp.LCPStateAckRcvd, ppp.LCPStateAckSent, ppp.LCPStateOpened:
	default:
		// No request is outstanding in the other states.
		return ipv6cpUnchanged, nil
	}
	suggested, present, err := ppp.ParseIPv6CPInterfaceID(pkt.Data)
	if err != nil {
		return ipv6cpUnchanged, nil
	}
	if present && ppp.ValidIPv6CPInterfaceID(suggested) {
		if c.hasNakSent && suggested == c.nakSent {
			// RFC 5072 Section 4.1: "But if the received interface identifier
			// is equal to the one sent in the last Configure-Nak, a new
			// interface identifier MUST be chosen."
			fresh, err := ppp.TentativeIPv6CPInterfaceID(nil)
			if err != nil {
				return ipv6cpEventUnspecified, fmt.Errorf("pppoe-client: IPv6CP interface identifier: %w", err)
			}
			c.local = fresh
		} else {
			// RFC 5072 Section 4.1: "In this case, a new Configure-Request
			// MUST be sent with the identifier value suggested in the last
			// Configure-Nak from the peer."
			c.local = suggested
		}
	}
	event := ipv6cpUnchanged
	if c.state == ppp.LCPStateOpened {
		event = ipv6cpClosed
	}
	if c.state != ppp.LCPStateAckSent {
		c.state = ppp.LCPStateReqSent
	}
	return event, c.sendRequest(w, buf)
}

// errIPv6CPIdentifierRejected is logged when the peer Configure-Rejects the
// Interface-Identifier, which leaves no identifier to form the link-local.
var errIPv6CPIdentifierRejected = errors.New("pppoe-client: the access concentrator rejected the IPv6CP Interface-Identifier, so no IPv6 link-local address can be formed; IPv4 continues; enable IPv6 interface-identifier negotiation on the access concentrator")

// receiveReject stops IPv6CP when the peer rejects the Interface-Identifier.
//
// RFC 5072 Section 4.1: "A new Configure-Request MUST NOT contain the
// interface-identifier option if a valid Interface-Identifier
// Configure-Reject is received." The same Section requires "exactly one
// instance of the interface-identifier option" in every Configure-Request,
// and Section 5 forms the link-local from it, so no request without the
// option can bring IPv6 up: IPv6CP stops, as pppd's ipv6cp_up closes the
// protocol when no local identifier was negotiated, and IPCP stays.
func (c *clientIPv6CP) receiveReject(pkt ppp.LCPPacket, logger *slog.Logger) (ipv6cpEvent, error) {
	options := c.requestOptions()
	if !ppp.ValidateLCPReply(pkt, c.identifier, options[:]) {
		return ipv6cpUnchanged, nil
	}
	if len(pkt.Data) == 0 {
		return ipv6cpUnchanged, nil
	}
	wasOpened := c.opened()
	c.state = ppp.LCPStateStopped
	logger.Warn(errIPv6CPIdentifierRejected.Error())
	if wasOpened {
		return ipv6cpClosed, nil
	}
	return ipv6cpUnchanged, nil
}

// rejectsIPv6CP reports whether an LCP Protocol-Reject names IPv6CP.
// RFC 1661 Section 5.7: the Rejected-Protocol field is octets 0..1 of Data.
func rejectsIPv6CP(pkt ppp.LCPPacket) bool {
	if len(pkt.Data) < 2 {
		return false
	}
	return uint16(pkt.Data[0])<<8|uint16(pkt.Data[1]) == ppp.ProtoIPv6CP
}

// enableIPv6CP attaches the IPv6CP automaton that negotiation started (it ran
// beside IPCP in negotiateIPCP) to a connected session, and applies the state
// it reached: IPv6 passes when IPv6CP already opened, and is discarded
// otherwise. setIPv6Mode MUST be serialized against transport and unit
// closure, as setIPv4Mode is.
//
// RFC 1661 Section 3.6: "Any supported network-layer protocol packets
// received when the corresponding NCP is not in the Opened state MUST be
// silently discarded."
func (s *connectedSession) enableIPv6CP(ipv6 *clientIPv6CP, setIPv6Mode func(ppp.NPMode) error, logger *slog.Logger) error {
	if ipv6 == nil {
		return errors.New("pppoe-client: missing IPv6CP automaton")
	}
	if setIPv6Mode == nil {
		return errors.New("pppoe-client: missing connected IPv6 mode control")
	}
	s.ipv6 = ipv6
	s.setIPv6Mode = setIPv6Mode
	if ipv6.opened() {
		return s.applyIPv6CP(ipv6cpOpened, logger)
	}
	if err := setIPv6Mode(ppp.NPModeDrop); err != nil {
		return fmt.Errorf("pppoe-client: cannot hold IPv6 until IPv6CP opens: %w", err)
	}
	return nil
}

// applyIPv6CP carries an IPv6CP event to the unit and the session result.
// Opened passes IPv6 and publishes the link-local address; Closed discards
// IPv6 again and withdraws it. IPCP and the IPv4 result are untouched.
//
// RFC 5072 Section 2: "Before any IPv6 packets may be communicated, PPP MUST
// reach the network-layer protocol phase, and the IPv6 Control Protocol MUST
// reach the Opened state."
func (s *connectedSession) applyIPv6CP(event ipv6cpEvent, logger *slog.Logger) error {
	switch event {
	case ipv6cpUnchanged:
		return nil
	case ipv6cpOpened:
		if err := s.setIPv6Mode(ppp.NPModePass); err != nil {
			return fmt.Errorf("pppoe-client: cannot pass IPv6 after IPv6CP opened: %w", err)
		}
		s.linkLocal = s.ipv6.linkLocal()
		logger.Info("pppoe-client: IPv6CP opened", "link-local", s.linkLocal)
		s.publish()
		return nil
	case ipv6cpClosed:
		if err := s.setIPv6Mode(ppp.NPModeDrop); err != nil {
			return fmt.Errorf("pppoe-client: cannot discard IPv6 after IPv6CP closed: %w", err)
		}
		s.linkLocal = netip.Addr{}
		logger.Info("pppoe-client: IPv6CP closed")
		s.publish()
		return nil
	case ipv6cpEventUnspecified:
		return errors.New("pppoe-client: IPv6CP produced no event")
	}
	return fmt.Errorf("pppoe-client: unknown IPv6CP event %d", event)
}

// setIPv6Mode serializes the unit ioctl with transport closure, as
// setIPv4Mode does.
func (s *sessionLink) setIPv6Mode(unitFD int, mode ppp.NPMode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	return ppp.SetIPv6Mode(unitFD, mode)
}
