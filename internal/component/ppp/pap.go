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
	"strconv"
	"time"
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

// PAPRequest is a parsed PAP Authenticate-Request. Username and
// Password are fresh strings owned by the caller; the parser does not
// retain the input buffer.
type PAPRequest struct {
	Identifier uint8
	Username   string
	Password   string //nolint:gosec // RFC 1334 Section 2.2 cleartext field name; wire-parsed, not stored at rest.
}

// ParsePAPRequest decodes a PAP Authenticate-Request payload (after
// the 2-byte PPP protocol field has been stripped by ParseFrame).
//
// RFC 1334 Section 2.2 packet format:
//
//	Code (1) | Identifier (1) | Length (2) | Peer-ID Length (1)
//	| Peer-ID (variable) | Passwd-Length (1) | Password (variable)
func ParsePAPRequest(buf []byte) (PAPRequest, error) {
	if len(buf) < papHeaderLen {
		return PAPRequest{}, errPAPTooShort
	}
	if buf[0] != PAPAuthenticateRequest {
		return PAPRequest{}, errPAPWrongCode
	}
	identifier := buf[1]
	length := int(binary.BigEndian.Uint16(buf[2:4]))
	if length < papHeaderLen {
		return PAPRequest{}, errPAPLengthMismatch
	}
	if length > len(buf) {
		return PAPRequest{}, errPAPLengthMismatch
	}
	if length > MaxFrameLen-2 {
		return PAPRequest{}, errPAPLengthMismatch
	}
	body := buf[papHeaderLen:length]

	// Peer-ID Length + Peer-ID.
	if len(body) < 1 {
		return PAPRequest{}, errPAPPeerIDOverflow
	}
	pidLen := int(body[0])
	if 1+pidLen > len(body) {
		return PAPRequest{}, errPAPPeerIDOverflow
	}
	peerID := string(body[1 : 1+pidLen])
	body = body[1+pidLen:]

	// Passwd-Length + Password.
	if len(body) < 1 {
		return PAPRequest{}, errPAPPasswdOverflow
	}
	pwLen := int(body[0])
	if 1+pwLen > len(body) {
		return PAPRequest{}, errPAPPasswdOverflow
	}
	password := string(body[1 : 1+pwLen])

	return PAPRequest{
		Identifier: identifier,
		Username:   peerID,
		Password:   password,
	}, nil
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
// emits EventAuthRequest on the auth events channel, waits for the
// external handler's AuthResponse, and writes Authenticate-Ack (on
// accept) or Authenticate-Nak (on reject) back to the peer. Returns
// false if the session should tear down (reject, timeout, stop).
//
// Phase 7 replaces runAuthPhase's AuthMethodNone fallthrough with a
// method switch that calls this function when LCP negotiated
// Auth-Protocol = 0xC023 (PAP).
//
// RFC 1334 Section 2 flow: the peer is the sole initiator, so the
// handler's first action is a blocking Read rather than a write.
//
// TODO(phase-7): the emit-event / wait-authRespCh / fail-on-stop
// triplet here duplicates the same sequence in runAuthPhase
// (session_run.go). Phase 7 introduces auth.go as the canonical auth
// state machine; it should factor the shared shape into a helper
// (e.g. awaitAuthDecision) that runAuthPhase, runPAPAuthPhase,
// runCHAPAuthPhase, and runMSCHAPv2AuthPhase all call. Deferring the
// consolidation to Phase 7 per design-principles.md "No premature
// abstraction" (three call sites will exist after Phase 5 / Phase 6
// land CHAP and MS-CHAPv2).
func (s *pppSession) runPAPAuthPhase() bool {
	readBuf := getFrameBuf()
	defer putFrameBuf(readBuf)
	n, err := s.chanFile.Read(readBuf)
	if err != nil {
		s.fail("pap: chan fd read: " + err.Error())
		return false
	}
	proto, payload, _, perr := ParseFrame(readBuf[:n])
	if perr != nil {
		s.fail("pap: malformed frame: " + perr.Error())
		return false
	}
	if proto != ProtoPAP {
		s.fail("pap: unexpected protocol 0x" +
			strconv.FormatUint(uint64(proto), 16))
		return false
	}
	req, perr := ParsePAPRequest(payload)
	if perr != nil {
		s.fail("pap: malformed request: " + perr.Error())
		return false
	}

	evt := EventAuthRequest{
		TunnelID:   s.tunnelID,
		SessionID:  s.sessionID,
		Method:     AuthMethodPAP,
		Identifier: req.Identifier,
		Username:   req.Username,
		Response:   []byte(req.Password),
	}
	select {
	case s.authEventsOut <- evt:
	case <-s.stopCh:
		return false
	case <-s.sessStop:
		return false
	}

	timeout := s.authTimeout
	if timeout <= 0 {
		timeout = defaultAuthTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	var resp authResponseMsg
	select {
	case resp = <-s.authRespCh:
	case <-s.stopCh:
		return false
	case <-s.sessStop:
		return false
	case <-timer.C:
		s.fail("pap: auth timeout after " + timeout.String())
		s.sendAuthEvent(EventAuthFailure{
			TunnelID:  s.tunnelID,
			SessionID: s.sessionID,
			Reason:    "timeout",
		})
		return false
	}

	writeBuf := getFrameBuf()
	defer putFrameBuf(writeBuf)
	off := WriteFrame(writeBuf, 0, ProtoPAP, nil)
	msg := []byte(resp.message)
	if resp.accept {
		off += WritePAPAck(writeBuf, off, req.Identifier, msg)
	} else {
		off += WritePAPNak(writeBuf, off, req.Identifier, msg)
	}
	if !s.writeFrame(writeBuf[:off]) {
		return false
	}

	if resp.accept {
		s.sendAuthEvent(EventAuthSuccess{
			TunnelID:  s.tunnelID,
			SessionID: s.sessionID,
		})
		return true
	}
	// Mirror runAuthPhase reject path: emit EventSessionDown on the
	// lifecycle channel before the auth-channel failure event so the
	// transport can tear the session down. Without s.fail here the
	// session goroutine returns false from afterLCPOpen but nothing
	// on the lifecycle channel signals the reject.
	s.fail("pap: auth rejected: " + resp.message)
	s.sendAuthEvent(EventAuthFailure{
		TunnelID:  s.tunnelID,
		SessionID: s.sessionID,
		Reason:    resp.message,
	})
	return false
}
