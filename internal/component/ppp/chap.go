// Design: docs/research/l2tpv2-ze-integration.md -- PPP auth-phase CHAP-MD5 codec
// Related: auth_events.go -- EventAuthRequest emitted from runCHAPAuthPhase
// Related: pap.go -- sibling auth-phase codec with the same skip-and-backfill shape
// Related: session.go -- pppSession.chapIdentifier counter the handler increments
// Related: session_run.go -- frame pool (getFrameBuf/putFrameBuf) and sendAuthEvent

// Implements PPP CHAP (Challenge Handshake Authentication Protocol) with MD5.
// Reference: RFC 1994 -- PPP Challenge Handshake Authentication Protocol (CHAP)
// Spec: rfc/short/rfc1994.md

package ppp

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"strconv"
)

// CHAP packet codes from RFC 1994 Section 4. The authenticator drives
// the exchange: it sends Challenge, the peer returns Response, and the
// authenticator replies Success or Failure.
const (
	CHAPCodeChallenge uint8 = 1
	CHAPCodeResponse  uint8 = 2
	CHAPCodeSuccess   uint8 = 3
	CHAPCodeFailure   uint8 = 4
)

// chapHeaderLen is the fixed header: Code + Identifier + Length.
const chapHeaderLen = 4

// chapMD5DigestLen is the response Value length for Algorithm 5 (MD5).
// RFC 1994 Section 4.1: the Response Value length depends on the hash
// algorithm used; for MD5 it is 16 octets.
//
// Declarative only in Phase 5: ParseCHAPResponse accepts any
// Value-Size in [1, 255] because the negotiated Algorithm is not yet
// threaded through LCP (Phase 7) and ze never computes the digest
// (Phase 8 l2tp-auth plugin validates via RADIUS). A bad Value-Size
// surfaces as a digest-mismatch reject out of RADIUS rather than a
// parser error.
const chapMD5DigestLen = 16

// chapChallengeValueLen is the Value length ze uses when generating
// Challenge packets. RFC 1994 Section 2.3 recommends a value "at least
// the length of the hash value"; 16 matches the MD5 digest size and
// every extant PPP stack.
const chapChallengeValueLen = 16

// chapLNSName is the Name field ze emits in outgoing Challenge packets
// until Phase 7 wires an operator-configured hostname.
//
// TODO(phase-7): source this from config (proposed: a local-name leaf
// in the l2tp YANG grouping) and thread it into pppSession via
// StartSession.
const chapLNSName = "ze"

var (
	errCHAPTooShort       = errors.New("ppp: CHAP packet shorter than 4-byte header")
	errCHAPLengthMismatch = errors.New("ppp: CHAP Length field does not match buffer")
	errCHAPWrongCode      = errors.New("ppp: CHAP packet is not Response")
	errCHAPValueSizeZero  = errors.New("ppp: CHAP Value-Size is zero")
	errCHAPValueOverflow  = errors.New("ppp: CHAP Value-Size exceeds packet")
)

// CHAPResponse is a parsed CHAP Response packet (code 2). Value is a
// fresh sub-slice of the parser input: the caller may retain it past
// the read-buffer lifetime by copying or converting it.
type CHAPResponse struct {
	Identifier uint8
	Value      []byte
	Name       string
}

// ParseCHAPResponse decodes a CHAP Response payload (after the 2-byte
// PPP protocol field has been stripped by ParseFrame).
//
// RFC 1994 Section 4.1 packet format:
//
//	Code (1) | Identifier (1) | Length (2) | Value-Size (1)
//	| Value (Value-Size) | Name (Length - 5 - Value-Size)
//
// RFC 1994 Section 4.1 requires Value-Size >= 1 ("The length of the
// Challenge Value depends upon the method of generation ... The Value
// field is one or more octets").
func ParseCHAPResponse(buf []byte) (CHAPResponse, error) {
	if len(buf) < chapHeaderLen {
		return CHAPResponse{}, errCHAPTooShort
	}
	if buf[0] != CHAPCodeResponse {
		return CHAPResponse{}, errCHAPWrongCode
	}
	identifier := buf[1]
	length := int(binary.BigEndian.Uint16(buf[2:4]))
	if length < chapHeaderLen+1 {
		return CHAPResponse{}, errCHAPLengthMismatch
	}
	if length > len(buf) {
		return CHAPResponse{}, errCHAPLengthMismatch
	}
	if length > MaxFrameLen-2 {
		return CHAPResponse{}, errCHAPLengthMismatch
	}
	body := buf[chapHeaderLen:length]

	valueSize := int(body[0])
	if valueSize == 0 {
		return CHAPResponse{}, errCHAPValueSizeZero
	}
	if 1+valueSize > len(body) {
		return CHAPResponse{}, errCHAPValueOverflow
	}
	value := make([]byte, valueSize)
	copy(value, body[1:1+valueSize])
	name := string(body[1+valueSize:])

	return CHAPResponse{
		Identifier: identifier,
		Value:      value,
		Name:       name,
	}, nil
}

// WriteCHAPChallenge encodes a CHAP Challenge (code 1) into buf at
// offset off and returns the number of bytes written. The caller
// SHOULD pass buf[off:] with cap >= MaxFrameLen - 2 (the PPP-payload
// room after WriteFrame); smaller buffers clamp Name safely but the
// emitted packet may not carry the caller's full Name.
//
// Value is clamped to 255 octets (Value-Size is a single octet per
// RFC 1994 Section 4.1); Name is clamped to the smaller of the
// single-frame room and the actual buf[off:] capacity. Both clamps
// are silent and should not occur for normal callers -- the Phase 5
// caller passes a 16-byte Value and a 2-byte Name ("ze"), and the
// Phase 7 config-driven Name MUST be validated at the caller before
// this function sees it.
//
// RFC 1994 Section 4.1: Challenge shares the Response layout. The
// Identifier MUST change each time a Challenge is sent; the caller
// (runCHAPAuthPhase) owns that counter.
func WriteCHAPChallenge(buf []byte, off int, identifier uint8, value, name []byte) int {
	return writeCHAPValued(buf, off, CHAPCodeChallenge, identifier, value, name)
}

func writeCHAPValued(buf []byte, off int, code, identifier uint8, value, name []byte) int {
	valueSize := min(len(value), 0xFF)
	buf[off] = code
	buf[off+1] = identifier
	// Skip Length; backfill once the body is written. Same pattern as
	// writePAPReply and WriteLCPPacket (rules/buffer-first.md).
	buf[off+4] = byte(valueSize)
	copy(buf[off+5:], value[:valueSize])
	nameOff := 5 + valueSize
	// Clamp Name to the smaller of single-frame room and actual buffer
	// room at off. Frame cap = MaxFrameLen - 2 (minus PPP protocol
	// field). Buffer cap = len(buf) - off. The min() lets a
	// legitimately short buffer (misuse, or a non-standard caller that
	// writes at off != 2) clamp safely without overrunning. The max(,0)
	// guard handles buffers so small they cannot hold even the header +
	// Value: we still need a non-negative slice bound for name[:nameLen].
	frameRoom := MaxFrameLen - 2 - chapHeaderLen - 1 - valueSize
	bufRoom := len(buf) - off - chapHeaderLen - 1 - valueSize
	maxName := max(min(frameRoom, bufRoom), 0)
	nameLen := min(len(name), maxName)
	copy(buf[off+nameOff:], name[:nameLen])
	total := nameOff + nameLen
	binary.BigEndian.PutUint16(buf[off+2:off+4], uint16(total))
	return total
}

// WriteCHAPSuccess encodes a CHAP Success (code 3) into buf at offset
// off and returns the number of bytes written. The caller MUST ensure
// buf[off:] has cap >= 4 + len(message).
//
// RFC 1994 Section 4.2 packet format:
//
//	Code (1) | Identifier (1) | Length (2) | Message (Length - 4)
//
// Unlike PAP Ack/Nak, CHAP Success/Failure has NO Msg-Length octet:
// the Message runs from byte 4 to Length. The encoder clamps Message
// to MaxFrameLen - 2 (frame header) - 4 (CHAP header) so the declared
// Length always fits a single PPP frame.
func WriteCHAPSuccess(buf []byte, off int, identifier uint8, message []byte) int {
	return writeCHAPReply(buf, off, CHAPCodeSuccess, identifier, message)
}

// WriteCHAPFailure encodes a CHAP Failure (code 4). Identical shape to
// Success except for the Code byte; see RFC 1994 Section 4.2.
func WriteCHAPFailure(buf []byte, off int, identifier uint8, message []byte) int {
	return writeCHAPReply(buf, off, CHAPCodeFailure, identifier, message)
}

func writeCHAPReply(buf []byte, off int, code, identifier uint8, message []byte) int {
	// Clamp Message to the smaller of single-frame room and actual
	// buffer room at off; same reasoning as writeCHAPValued's Name
	// clamp (see that function's comment), including the max(,0) guard
	// against a buffer too small to hold even the CHAP header.
	frameRoom := MaxFrameLen - 2 - chapHeaderLen
	bufRoom := len(buf) - off - chapHeaderLen
	maxMessage := max(min(frameRoom, bufRoom), 0)
	msgLen := min(len(message), maxMessage)
	buf[off] = code
	buf[off+1] = identifier
	copy(buf[off+chapHeaderLen:], message[:msgLen])
	total := chapHeaderLen + msgLen
	binary.BigEndian.PutUint16(buf[off+2:off+4], uint16(total))
	return total
}

// drawCHAPChallenge fills the first chapChallengeValueLen octets of
// dst with random bytes from crypto/rand. RFC 1994 Section 2.3: "Each
// challenge value SHOULD be unique ... SHOULD also be unpredictable."
// Unlike generateMagic, zero is a legal Challenge Value, so no retry
// loop is needed. Panics if dst is shorter than chapChallengeValueLen
// -- that is a programmer error, not a runtime failure.
func drawCHAPChallenge(dst []byte) error {
	if len(dst) < chapChallengeValueLen {
		panic("BUG: drawCHAPChallenge dst shorter than chapChallengeValueLen")
	}
	_, err := rand.Read(dst[:chapChallengeValueLen])
	return err
}

// runCHAPAuthPhase drives the CHAP-MD5 three-way handshake as
// authenticator: draw a fresh 16-byte Challenge value, send Challenge,
// read the peer's Response, emit EventAuthRequest, wait for the
// external handler's AuthResponse, and write Success (accept) or
// Failure (reject). Returns false if the session should tear down
// (reject, timeout, stop, wire error).
//
// Phase 7 replaces runAuthPhase's AuthMethodNone fallthrough with a
// method switch that calls this function when LCP negotiated
// Auth-Protocol = 0xC223 (CHAP) with Algorithm = 5 (MD5).
//
// RFC 1994 Section 4.1: the Identifier MUST change each time a
// Challenge is sent. runCHAPAuthPhase increments s.chapIdentifier
// before drawing the next value and uses the result as the Challenge
// Identifier; the peer's Response MUST echo that same Identifier.
//
// Called from runAuthPhase when LCP negotiated Auth-Protocol = 0xC223
// (CHAP) with Algorithm = 5 (MD5); see session_run.go method switch.
// The emit/wait/timeout triplet is factored into awaitAuthDecision
// (auth.go).
func (s *pppSession) runCHAPAuthPhase() bool {
	var value [chapChallengeValueLen]byte
	if err := drawCHAPChallenge(value[:]); err != nil {
		s.fail("chap: crypto/rand: " + err.Error())
		return false
	}
	s.chapIdentifier++
	identifier := s.chapIdentifier

	challengeBuf := getFrameBuf()
	defer putFrameBuf(challengeBuf)
	cOff := WriteFrame(challengeBuf, 0, ProtoCHAP, nil)
	cOff += WriteCHAPChallenge(challengeBuf, cOff, identifier, value[:], []byte(chapLNSName))
	if !s.writeFrame(challengeBuf[:cOff]) {
		return false
	}

	var frame []byte
	select {
	case f, ok := <-s.framesIn:
		if !ok {
			s.fail("chap: frames channel closed")
			return false
		}
		frame = f
	case <-s.stopCh:
		return false
	case <-s.sessStop:
		return false
	}
	defer putFrameBuf(frame)
	proto, payload, _, perr := ParseFrame(frame)
	if perr != nil {
		s.fail("chap: malformed frame: " + perr.Error())
		return false
	}
	if proto != ProtoCHAP {
		s.fail("chap: unexpected protocol 0x" +
			strconv.FormatUint(uint64(proto), 16))
		return false
	}
	resp, perr := ParseCHAPResponse(payload)
	if perr != nil {
		s.fail("chap: malformed response: " + perr.Error())
		return false
	}
	// RFC 1994 Section 4.1: the Response Identifier MUST match the
	// outstanding Challenge Identifier. A mismatched Identifier is a
	// silently-discarded stale or forged packet; we fail the session
	// rather than spin because Phase 5 does not implement the retry
	// loop (TODO phase-9: retransmit the Challenge until a matching
	// Response arrives or a retry counter expires).
	if resp.Identifier != identifier {
		s.fail("chap: response identifier 0x" +
			strconv.FormatUint(uint64(resp.Identifier), 16) +
			" does not match challenge 0x" +
			strconv.FormatUint(uint64(identifier), 16))
		return false
	}

	// RFC 1994 Section 4.1 Response packet: Name identifies the peer.
	// Populate both Username and PeerName so Phase 6 (MS-CHAPv2) can
	// reuse Username with a different semantic without overloading the
	// field. Challenge carries the Value *we sent*; Response carries
	// the MD5 digest the peer returned. ze never computes the digest
	// -- that happens in the l2tp-auth plugin in Phase 8.
	evt := EventAuthRequest{
		TunnelID:   s.tunnelID,
		SessionID:  s.sessionID,
		Method:     AuthMethodCHAPMD5,
		Identifier: identifier,
		Username:   resp.Name,
		Challenge:  bytes.Clone(value[:]),
		Response:   resp.Value,
		PeerName:   resp.Name,
	}
	decision, ok := s.awaitAuthDecision(evt, "chap")
	if !ok {
		return false
	}

	writeBuf := getFrameBuf()
	defer putFrameBuf(writeBuf)
	rOff := WriteFrame(writeBuf, 0, ProtoCHAP, nil)
	msg := []byte(decision.message)
	if decision.accept {
		rOff += WriteCHAPSuccess(writeBuf, rOff, identifier, msg)
	} else {
		rOff += WriteCHAPFailure(writeBuf, rOff, identifier, msg)
	}
	if !s.writeFrame(writeBuf[:rOff]) {
		return false
	}

	if decision.accept {
		s.sendAuthEvent(EventAuthSuccess{
			TunnelID:  s.tunnelID,
			SessionID: s.sessionID,
		})
		return true
	}
	// Mirror runPAPAuthPhase reject path: emit EventSessionDown on the
	// lifecycle channel via s.fail before EventAuthFailure on the auth
	// channel, so the transport can tear the session down.
	s.fail("chap: auth rejected: " + decision.message)
	s.sendAuthEvent(EventAuthFailure{
		TunnelID:  s.tunnelID,
		SessionID: s.sessionID,
		Reason:    decision.message,
	})
	return false
}
