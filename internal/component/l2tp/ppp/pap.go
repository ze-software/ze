// Design: docs/research/l2tpv2-ze-integration.md -- PPP auth-phase PAP codec
// Related: auth_events.go -- EventAuthRequest emitted from runPAPAuthPhase
// Related: session.go -- pppSession fields the handler reads and writes
// Related: session_run.go -- frame pool (getFrameBuf/putFrameBuf) and sendAuthEvent
// Related: frame.go -- ProtoPAP (0xC023) framing constants

// Implements PPP PAP (Password Authentication Protocol).
// Reference: RFC 1334 -- PPP Authentication Protocols
// Spec: rfc/short/rfc1334.md

package ppp

import (
	"encoding/binary"
	"errors"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// PAP packet codes from RFC 1334 Section 2.1. Unlike CHAP there are
// only three: the peer sends Authenticate-Request, the authenticator
// replies Ack or Nak.
const (
	PAPAuthenticateRequest uint8 = 1
	PAPAuthenticateAck     uint8 = 2
	PAPAuthenticateNak     uint8 = 3
)

// papHeaderLen is the fixed header: Code + Identifier + Length. The
// RFC 1334 Section 2.1 Length field covers this header and the Data
// area; octets past Length are Data Link Layer padding.
const papHeaderLen = 4

// papFirstRequestTimeout bounds the wait for the peer's first PAP
// Authenticate-Request. Matches the session-level defaultAuthTimeout.
const papFirstRequestTimeout = 30 * time.Second

// proxyPAPRequestWait bounds the same wait on a proxied LCP lifetime, where the
// LAC may already have finished PAP with the peer. It covers two Restart periods:
// a peer still in the Authentication phase repeats its request, RFC 1334 Section
// 2.2.1: "The Authenticate-Request packet MUST be repeated until a valid reply
// packet is received, or an optional retry counter expires."
const proxyPAPRequestWait = 2 * defaultRestartTimer

// papMaxFieldLen is the maximum length of a Peer-ID, Password or
// Message field. RFC 1334 prefixes each field with a one-octet length,
// capping the encoded content at 255 octets.
const papMaxFieldLen = 0xFF

var (
	errPAPTooShort       = errors.New("ppp: PAP packet shorter than 4-byte header")
	errPAPLengthMismatch = errors.New("ppp: PAP Length field does not match buffer")
	errPAPWrongCode      = errors.New("ppp: PAP packet is not Authenticate-Request")
	errPAPPeerIDOverflow = errors.New("ppp: PAP Peer-ID Length exceeds packet")
	errPAPPasswdOverflow = errors.New("ppp: PAP Passwd-Length exceeds packet")
)

// pAPRequest is a parsed PAP Authenticate-Request. Username and
// Password are fresh strings owned by the caller; the parser does not
// retain the input buffer.
type pAPRequest struct {
	Identifier uint8
	Username   string
	Password   string //nolint:gosec // RFC 1334 Section 2.2 cleartext field name; wire-parsed, not stored at rest.
}

// parsePAPRequest decodes a PAP Authenticate-Request payload (after
// the 2-byte PPP protocol field has been stripped by ParseFrame).
//
// RFC 1334 Section 2.2 packet format:
//
//	Code (1) | Identifier (1) | Length (2) | Peer-ID Length (1)
//	| Peer-ID (variable) | Passwd-Length (1) | Password (variable)
func parsePAPRequest(buf []byte) (pAPRequest, error) {
	if err := validatePAPRequest(buf); err != nil {
		return pAPRequest{}, err
	}
	peerEnd := 5 + int(buf[4])
	passwordEnd := peerEnd + 1 + int(buf[peerEnd])
	return pAPRequest{
		Identifier: buf[1],
		Username:   string(buf[5:peerEnd]),
		Password:   string(buf[peerEnd+1 : passwordEnd]),
	}, nil
}

// validatePAPRequest checks the wire bounds without copying credentials.
// Reanswers need only the Identifier and MUST NOT retain the incoming frame.
func validatePAPRequest(buf []byte) error {
	if len(buf) < papHeaderLen {
		return errPAPTooShort
	}
	if buf[0] != PAPAuthenticateRequest {
		return errPAPWrongCode
	}
	length := int(binary.BigEndian.Uint16(buf[2:4]))
	if length < papHeaderLen {
		return errPAPLengthMismatch
	}
	if length > len(buf) {
		return errPAPLengthMismatch
	}
	if length > MaxFrameLen {
		return errPAPLengthMismatch
	}
	body := buf[papHeaderLen:length]
	if len(body) == 0 {
		return errPAPPeerIDOverflow
	}
	peerEnd := 1 + int(body[0])
	if peerEnd > len(body) {
		return errPAPPeerIDOverflow
	}
	if peerEnd == len(body) {
		return errPAPPasswdOverflow
	}
	if peerEnd+1+int(body[peerEnd]) > len(body) {
		return errPAPPasswdOverflow
	}
	return nil
}

// WritePAPAck encodes a PAP Authenticate-Ack into buf at offset off
// and returns the number of bytes written. The caller MUST ensure
// buf[off:] has cap >= 5 + min(len(message), 255).
//
// RFC 1334 Section 2.3 packet format:
//
//	Code (1) | Identifier (1) | Length (2) | Msg-Length (1) | Message
func WritePAPAck(buf []byte, off int, identifier uint8, message []byte) int {
	return writePAPReply(buf, off, PAPAuthenticateAck, identifier, message)
}

// WritePAPNak encodes a PAP Authenticate-Nak. Identical shape to Ack
// except for the Code byte; see RFC 1334 Section 2.3.
func WritePAPNak(buf []byte, off int, identifier uint8, message []byte) int {
	return writePAPReply(buf, off, PAPAuthenticateNak, identifier, message)
}

func writePAPReply(buf []byte, off int, code, identifier uint8, message []byte) int {
	// RFC 1334 Section 2.3: Msg-Length is one octet; clamp to 255
	// rather than truncating via byte() which would wrap.
	msgLen := min(len(message), papMaxFieldLen)
	buf[off] = code
	buf[off+1] = identifier
	// Skip Length; backfill once the body is written. This follows
	// the skip-and-backfill pattern from rules/buffer-first.md that
	// WriteLCPPacket also uses.
	buf[off+4] = byte(msgLen)
	copy(buf[off+5:], message[:msgLen])
	total := 5 + msgLen
	binary.BigEndian.PutUint16(buf[off+2:off+4], uint16(total))
	return total
}

// runPAPAuthPhase reads one PAP Authenticate-Request from chanFile,
// emits EventAuthRequest via awaitAuthDecision, and writes
// Authenticate-Ack (on accept) or Authenticate-Nak (on reject) back
// to the peer. Returns false if the session should tear down
// (reject, timeout, stop).
//
// Called from runAuthPhase when LCP negotiated Auth-Protocol = 0xC023
// (PAP); see session_run.go method switch.
//
// RFC 1334 Section 2 flow: the peer is the sole initiator, so the
// handler's first action is a blocking Read rather than a write.
//
// Non-PAP frames arriving during the wait go through s.handleFrame, as in
// waitCHAPLike, so LCP stays responsive. A change of LCP state ends the wait and
// the caller resumes the session loop.
//
// A proxied lifetime that sees no request renegotiates LCP instead of failing
// (renegotiateProxiedLCP), which also returns false.
func (s *pppSession) runPAPAuthPhase() bool {
	lcpState := s.currentState()
	wait := papFirstRequestTimeout
	if s.lcpProxied {
		wait = proxyPAPRequestWait
	}
	deadline := time.NewTimer(wait)
	defer deadline.Stop()

	var req pAPRequest
	for {
		var frame []byte
		select {
		case f, ok := <-s.framesIn:
			if !ok {
				s.fail("pap: frames channel closed")
				return false
			}
			frame = f
		case <-deadline.C:
			if s.lcpProxied {
				s.renegotiateProxiedLCP()
				return false
			}
			s.fail("pap: timed out waiting for Authenticate-Request")
			return false
		case <-s.stopCh:
			return false
		case <-s.sessStop:
			return false
		}
		proto, payload, _, perr := ParseFrame(frame)
		if perr != nil {
			putFrameBuf(frame)
			var tb textbuf.Buffer
			s.fail(tb.Str("pap: malformed frame: ").Err(perr).String())
			return false
		}
		if proto != ProtoPAP {
			// RFC 1661 Section 5.5: "Upon reception of a Terminate-Request, a
			// Terminate-Ack MUST be transmitted."
			term := s.handleFrame(frame)
			putFrameBuf(frame)
			if term || s.currentState() != lcpState {
				return false
			}
			continue
		}
		parsed, perr := parsePAPRequest(payload)
		putFrameBuf(frame)
		if perr != nil {
			var tb textbuf.Buffer
			s.fail(tb.Str("pap: malformed request: ").Err(perr).String())
			return false
		}
		req = parsed
		break
	}

	evt := EventAuthRequest{
		TunnelID:   s.tunnelID,
		SessionID:  s.sessionID,
		Method:     AuthMethodPAP,
		Identifier: req.Identifier,
		Username:   req.Username,
		Response:   []byte(req.Password),
	}
	resp, ok := s.awaitAuthDecision(evt, "pap")
	if !ok {
		return false
	}

	writeBuf := getFrameBuf()
	defer putFrameBuf(writeBuf)
	off := WriteFrame(writeBuf, 0, ProtoPAP, nil)
	// RFC 1334 Section 2.2.2: "The Message field is zero or more octets,
	// and its contents are implementation dependent." Keep only the text
	// that fits after Code, Identifier, Length and Msg-Length. The auth
	// decision and the cached reply Code do not depend on this display text.
	messageLen := min(len(resp.message), papMaxFieldLen, max(int(s.transmitMRU())-papHeaderLen-1, 0))
	message := []byte(resp.message[:messageLen])
	code := PAPAuthenticateAck
	if resp.accept {
		off += WritePAPAck(writeBuf, off, req.Identifier, message)
	} else {
		code = PAPAuthenticateNak
		off += WritePAPNak(writeBuf, off, req.Identifier, message)
	}
	if !s.writeFrame(writeBuf[:off]) {
		return false
	}
	// RFC 1334 Section 2.2.1: "Because the Authenticate-Ack might be lost,
	// the authenticator MUST allow repeated Authenticate-Request packets
	// after completing the Authentication phase."
	s.papReplyCode = code

	if resp.accept {
		s.sendAuthEvent(eventAuthSuccess{
			TunnelID:  s.tunnelID,
			SessionID: s.sessionID,
		})
		return true
	}
	s.closeLinkAfterRefusal(resp)
	var tb textbuf.Buffer
	s.fail(tb.Str("pap: auth rejected: ").Str(resp.message).String())
	s.sendAuthEvent(eventAuthFailure{
		TunnelID:  s.tunnelID,
		SessionID: s.sessionID,
		Reason:    resp.message,
	})
	return false
}

// renegotiateProxiedLCP restarts LCP on a proxied lifetime whose peer sent no
// Authenticate-Request. PAP is peer-initiated, so Ze cannot ask for one, and a
// peer the LAC already authenticated is in the Network phase and will not send
// one. Proxy LCP is optional for the LNS, and Ze never authenticates from the
// Proxy Authen AVPs: the Proxy Authen Name only seeds the displayed username
// until an authenticated identity replaces it (l2tp/reactor_kernel.go
// handleSessionIPAssigned). The peer authenticates to Ze in the new round, so
// nothing rests on the LAC.
//
// RFC 2661 Section 4.4.5: "Note that the LNS policy may be to enter an additional
// round of LCP negotiation and/or authentication if the LAC is not trusted."
// RFC 1661 Section 4.3: "The Down followed by an Up will cause an orderly
// renegotiation of the link, by progressing through the Starting to the
// Request-Sent state."
//
// The Down and Up run as one transition, so a Configure-Request that cannot be
// sent leaves LCP in Opened with the session failed, where the caller ends it.
func (s *pppSession) renegotiateProxiedLCP() {
	s.logger.Info("ppp: no PAP Authenticate-Request on proxied LCP, renegotiating LCP",
		"waited", proxyPAPRequestWait.String())
	s.resetAuthNegotiation()
	down := LCPDoTransition(LCPStateOpened, LCPEventDown)
	up := LCPDoTransition(down.NewState, LCPEventUp)
	// The caller reads the outcome from the LCP state, as it does after every
	// other LCP interruption of the authentication phase.
	s.applyTransition(LCPStateOpened, lCPTransition{
		NewState: up.NewState,
		Actions:  slices.Concat(down.Actions, up.Actions),
	}, LCPPacket{})
}

// reanswerPAP preserves the authentication decision for later requests.
// The session goroutine MUST call this only after PAP authentication completed.
// It MUST discard PAP requests in other phases and clear the decision when LCP
// leaves the network phase. This returns false on a failed reply write.
// RFC 1334 Section 2.2.2 reply offsets:
//
//	0 Code | 1 Identifier | 2..3 Length | 4 Msg-Length | 5.. Message
func (s *pppSession) reanswerPAP(payload []byte) bool {
	if s.papReplyCode == 0 {
		return true
	}
	if err := validatePAPRequest(payload); err != nil {
		return true
	}
	buf := getFrameBuf()
	defer putFrameBuf(buf)
	off := WriteFrame(buf, 0, ProtoPAP, nil)
	// RFC 1334 Section 2.2.1: "Protocol phase MUST return the same reply
	// Code returned when the Authentication phase completed (the message
	// portion MAY be different)."
	// RFC 1334 Section 2.2.2: "The Identifier field MUST be copied from the
	// Identifier field of the Authenticate-Request which caused this reply."
	off += writePAPReply(buf, off, s.papReplyCode, payload[1], nil)
	return s.writeFrame(buf[:off])
}
