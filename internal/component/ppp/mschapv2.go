// Design: docs/research/l2tpv2-ze-integration.md -- PPP auth-phase MS-CHAPv2 codec
// Related: auth_events.go -- EventAuthRequest emitted from runMSCHAPv2AuthPhase
// Related: chap.go -- sibling CHAP-MD5 codec sharing Auth-Protocol 0xC223 and chapIdentifier
// Related: session.go -- pppSession.chapIdentifier counter shared with CHAP-MD5
// Related: session_run.go -- frame pool (getFrameBuf/putFrameBuf) and sendAuthEvent

// Implements PPP MS-CHAPv2 (Microsoft CHAP Extensions, Version 2).
// Reference: RFC 2759 -- Microsoft PPP CHAP Extensions, Version 2
// Spec: rfc/short/rfc2759.md

package ppp

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"strconv"
	"time"
)

// MS-CHAPv2 packet codes. Section 4 inherits the CHAP framing from
// RFC 1994 Section 4: Challenge / Response / Success / Failure at codes
// 1/2/3/4. Code 7 (Change-Password) is out of scope for Phase 6.
const (
	MSCHAPv2CodeChallenge uint8 = 1
	MSCHAPv2CodeResponse  uint8 = 2
	MSCHAPv2CodeSuccess   uint8 = 3
	MSCHAPv2CodeFailure   uint8 = 4
)

// mschapv2HeaderLen is the fixed header: Code + Identifier + Length.
const mschapv2HeaderLen = 4

// Response Value layout per RFC 2759 Section 4 (and Section 5 for the
// Peer-Challenge / NT-Response interpretation): Peer-Challenge (16) +
// Reserved (8) + NT-Response (24) + Flags (1) = 49 octets total.
const (
	mschapv2ChallengeValueLen = 16
	mschapv2ResponseValueLen  = 49
	mschapv2PeerChallengeLen  = 16
	mschapv2ReservedLen       = 8
	mschapv2NTResponseLen     = 24
	mschapv2FlagsLen          = 1
)

// mschapv2AuthenticatorResponseLen is the length of the raw 20-octet
// Authenticator-Response returned by the auth handler (l2tp-auth plugin
// in production, test stubs in unit tests). It is hex-encoded to 40
// uppercase ASCII characters for the `S=` field of the Success Message
// per RFC 2759 Section 5.
const mschapv2AuthenticatorResponseLen = 20

// mschapv2LNSName is the Name field ze emits in outgoing Challenge
// packets until Phase 7 wires an operator-configured hostname. Matches
// the Phase 5 chapLNSName default.
//
// TODO(phase-7): source this from config (proposed: a local-name leaf
// in the l2tp YANG grouping) and thread it into pppSession via
// StartSession.
const mschapv2LNSName = "ze"

var (
	errMSCHAPv2TooShort        = errors.New("ppp: MSCHAPv2 packet shorter than 4-byte header")
	errMSCHAPv2LengthMismatch  = errors.New("ppp: MSCHAPv2 Length field does not match buffer")
	errMSCHAPv2WrongCode       = errors.New("ppp: MSCHAPv2 packet is not Response")
	errMSCHAPv2ValueSizeWrong  = errors.New("ppp: MSCHAPv2 Value-Size is not 49")
	errMSCHAPv2ReservedNonZero = errors.New("ppp: MSCHAPv2 Reserved octets are non-zero")
	errMSCHAPv2FlagsNonZero    = errors.New("ppp: MSCHAPv2 Flags octet is non-zero")
)

// MSCHAPv2Response is a parsed MS-CHAPv2 Response packet (code 2).
// PeerChallenge and NTResponse are fresh sub-slices of the parser input:
// the caller may retain them past the read-buffer lifetime by copying.
// Reserved (8 octets) and Flags (1 octet) are validated as zero and
// dropped -- they are not exposed because the RFC 2759 Section 5 hash
// recipes do not take them as input, so passing them downstream would be
// noise.
type MSCHAPv2Response struct {
	Identifier    uint8
	PeerChallenge []byte
	NTResponse    []byte
	Name          string
}

// ParseMSCHAPv2Response decodes an MS-CHAPv2 Response payload (after the
// 2-byte PPP protocol field has been stripped by ParseFrame).
//
// RFC 2759 Section 4 packet format:
//
//	Code (1) | Identifier (1) | Length (2) | Value-Size (1)
//	| Peer-Challenge (16) | Reserved (8) | NT-Response (24) | Flags (1)
//	| Name (Length - 5 - 49)
//
// RFC 2759 Section 4: Value-Size MUST be exactly 49 on Response;
// Reserved MUST be zero on transmit and MUST be rejected if non-zero on
// receive; Flags MUST be zero in this version.
func ParseMSCHAPv2Response(buf []byte) (MSCHAPv2Response, error) {
	if len(buf) < mschapv2HeaderLen {
		return MSCHAPv2Response{}, errMSCHAPv2TooShort
	}
	if buf[0] != MSCHAPv2CodeResponse {
		// RFC 2759 Section 8 defines Code 7 (Change-Password) which a
		// peer MAY send after receiving Failure with E=648
		// (ERROR_PASSWD_EXPIRED). ze does not implement
		// Change-Password; such packets fall into this path and the
		// caller (runMSCHAPv2AuthPhase) fails the session. Honoring
		// Code 7 is deferred to a future spec.
		return MSCHAPv2Response{}, errMSCHAPv2WrongCode
	}
	identifier := buf[1]
	length := int(binary.BigEndian.Uint16(buf[2:4]))
	if length < mschapv2HeaderLen+1+mschapv2ResponseValueLen {
		return MSCHAPv2Response{}, errMSCHAPv2LengthMismatch
	}
	if length > len(buf) {
		return MSCHAPv2Response{}, errMSCHAPv2LengthMismatch
	}
	if length > MaxFrameLen-2 {
		return MSCHAPv2Response{}, errMSCHAPv2LengthMismatch
	}
	body := buf[mschapv2HeaderLen:length]

	valueSize := int(body[0])
	if valueSize != mschapv2ResponseValueLen {
		return MSCHAPv2Response{}, errMSCHAPv2ValueSizeWrong
	}
	// Value field layout within body:
	//   [1 .. 1+16)                Peer-Challenge
	//   [17 .. 17+8)               Reserved (MUST be zero)
	//   [25 .. 25+24)              NT-Response
	//   [49]                       Flags (MUST be zero)
	peerChallengeOff := 1
	reservedOff := peerChallengeOff + mschapv2PeerChallengeLen
	ntResponseOff := reservedOff + mschapv2ReservedLen
	flagsOff := ntResponseOff + mschapv2NTResponseLen
	nameOff := flagsOff + mschapv2FlagsLen

	// RFC 2759 Section 4: Reserved MUST be zero. A non-zero Reserved
	// block is a protocol violation and MUST be rejected so a peer
	// that smuggles bits in the Reserved space cannot slip past.
	for i := range mschapv2ReservedLen {
		if body[reservedOff+i] != 0 {
			return MSCHAPv2Response{}, errMSCHAPv2ReservedNonZero
		}
	}
	// RFC 2759 Section 4: Flags MUST be zero in this version.
	if body[flagsOff] != 0 {
		return MSCHAPv2Response{}, errMSCHAPv2FlagsNonZero
	}

	peerChallenge := make([]byte, mschapv2PeerChallengeLen)
	copy(peerChallenge, body[peerChallengeOff:peerChallengeOff+mschapv2PeerChallengeLen])
	ntResponse := make([]byte, mschapv2NTResponseLen)
	copy(ntResponse, body[ntResponseOff:ntResponseOff+mschapv2NTResponseLen])
	name := string(body[nameOff:])

	return MSCHAPv2Response{
		Identifier:    identifier,
		PeerChallenge: peerChallenge,
		NTResponse:    ntResponse,
		Name:          name,
	}, nil
}

// WriteMSCHAPv2Challenge encodes an MS-CHAPv2 Challenge (code 1) into
// buf at offset off and returns the number of bytes written. Shape
// matches WriteCHAPChallenge (shared Auth-Protocol 0xC223 framing) with
// a fixed Value-Size of 16 per RFC 2759 Section 4.
//
// The caller SHOULD pass buf[off:] with cap >= MaxFrameLen - 2 (the
// PPP-payload room after WriteFrame); smaller buffers clamp Name safely
// but the emitted packet may not carry the caller's full Name.
//
// RFC 2759 Section 4 reuses the RFC 1994 Section 4.1 Identifier
// discipline: the Identifier MUST change each time a Challenge is sent.
// runMSCHAPv2AuthPhase owns that counter via s.chapIdentifier (shared
// with CHAP-MD5 because LCP negotiates exactly one Auth-Protocol per
// session).
func WriteMSCHAPv2Challenge(buf []byte, off int, identifier uint8, value, name []byte) int {
	return writeMSCHAPv2Valued(buf, off, MSCHAPv2CodeChallenge, identifier, value, name)
}

func writeMSCHAPv2Valued(buf []byte, off int, code, identifier uint8, value, name []byte) int {
	// Authenticator Challenge is fixed at 16 octets per RFC 2759 Section 4.
	// A caller that passes a longer value is clamped; a shorter value
	// leaves the Value tail UNDEFINED because buf comes from a sync.Pool
	// of reused MaxFrameLen buffers that are not zeroed on Get. The
	// Value-Size byte still reports 16 unconditionally. Production callers
	// (runMSCHAPv2AuthPhase) MUST pass exactly mschapv2ChallengeValueLen
	// bytes drawn by drawMSCHAPv2Challenge.
	valueSize := min(len(value), mschapv2ChallengeValueLen)
	buf[off] = code
	buf[off+1] = identifier
	// Skip Length; backfill once the body is written. Same pattern as
	// writeCHAPValued (rules/buffer-first.md).
	buf[off+4] = byte(mschapv2ChallengeValueLen)
	copy(buf[off+5:], value[:valueSize])
	nameOff := 5 + mschapv2ChallengeValueLen
	// Clamp Name to the smaller of single-frame room and actual buffer
	// room at off, with a max(, 0) guard for buffers too small to hold
	// the header + Value (see writeCHAPValued for the full rationale).
	frameRoom := MaxFrameLen - 2 - mschapv2HeaderLen - 1 - mschapv2ChallengeValueLen
	bufRoom := len(buf) - off - mschapv2HeaderLen - 1 - mschapv2ChallengeValueLen
	maxName := max(min(frameRoom, bufRoom), 0)
	nameLen := min(len(name), maxName)
	copy(buf[off+nameOff:], name[:nameLen])
	total := nameOff + nameLen
	binary.BigEndian.PutUint16(buf[off+2:off+4], uint16(total))
	return total
}

// WriteMSCHAPv2Success encodes an MS-CHAPv2 Success (code 3) into buf at
// offset off and returns the number of bytes written. The caller MUST
// ensure buf[off:] has cap >= 4 + 45 + len(message). Panics with
// "BUG: ..." if authResponseBlob is not exactly 20 octets.
//
// RFC 2759 Section 5 packet format:
//
//	Code (1) | Identifier (1) | Length (2) | Message (Length - 4)
//
// Message layout (structured ASCII):
//
//	"S=" + 40 uppercase hex chars of authResponseBlob + " M=" + message
//
// Like CHAP-MD5 Success/Failure there is NO Msg-Length octet between
// the header and the Message: the Message runs from byte 4 to Length.
// Hex digits A-F are emitted uppercase per RFC 2759 Section 5. The
// encoder clamps the combined preface + message to MaxFrameLen - 2 -
// mschapv2HeaderLen so the declared Length always fits a single PPP
// frame.
func WriteMSCHAPv2Success(buf []byte, off int, identifier uint8, authResponseBlob, message []byte) int {
	if len(authResponseBlob) != mschapv2AuthenticatorResponseLen {
		panic("BUG: WriteMSCHAPv2Success authResponseBlob length != " +
			strconv.Itoa(mschapv2AuthenticatorResponseLen))
	}
	// Preface: 'S' '=' + 40 hex + ' ' 'M' '=' = 2 + 40 + 3 = 45 bytes.
	const (
		prefaceLen = 2 + 2*mschapv2AuthenticatorResponseLen + 3
		hexdigits  = "0123456789ABCDEF"
	)

	frameRoom := MaxFrameLen - 2 - mschapv2HeaderLen
	bufRoom := len(buf) - off - mschapv2HeaderLen
	maxMessage := max(min(frameRoom, bufRoom), 0)

	buf[off] = MSCHAPv2CodeSuccess
	buf[off+1] = identifier

	// Stage the preface in a fixed-size stack array so clamping at the
	// frame boundary is a single bounded copy with no allocation.
	var preface [prefaceLen]byte
	preface[0] = 'S'
	preface[1] = '='
	for i, b := range authResponseBlob {
		preface[2+2*i] = hexdigits[b>>4]
		preface[2+2*i+1] = hexdigits[b&0x0F]
	}
	preface[2+2*mschapv2AuthenticatorResponseLen] = ' '
	preface[2+2*mschapv2AuthenticatorResponseLen+1] = 'M'
	preface[2+2*mschapv2AuthenticatorResponseLen+2] = '='

	msgStart := off + mschapv2HeaderLen
	prefaceWrite := min(prefaceLen, maxMessage)
	copy(buf[msgStart:], preface[:prefaceWrite])
	msgRoom := max(maxMessage-prefaceWrite, 0)
	msgLen := min(len(message), msgRoom)
	copy(buf[msgStart+prefaceWrite:], message[:msgLen])

	total := mschapv2HeaderLen + prefaceWrite + msgLen
	binary.BigEndian.PutUint16(buf[off+2:off+4], uint16(total))
	return total
}

// WriteMSCHAPv2Failure encodes an MS-CHAPv2 Failure (code 4). The
// Message is written verbatim. RFC 2759 Section 5 specifies a
// structured format `E=<code> R=<retry> C=<32-hex> V=<ver> M=<text>` --
// the auth handler is responsible for assembling that string. ze's
// encoder does not interpret the error code.
func WriteMSCHAPv2Failure(buf []byte, off int, identifier uint8, message []byte) int {
	return writeMSCHAPv2Reply(buf, off, MSCHAPv2CodeFailure, identifier, message)
}

func writeMSCHAPv2Reply(buf []byte, off int, code, identifier uint8, message []byte) int {
	// Clamp Message to the smaller of single-frame room and actual
	// buffer room at off. Same pattern as writeCHAPReply.
	frameRoom := MaxFrameLen - 2 - mschapv2HeaderLen
	bufRoom := len(buf) - off - mschapv2HeaderLen
	maxMessage := max(min(frameRoom, bufRoom), 0)
	msgLen := min(len(message), maxMessage)
	buf[off] = code
	buf[off+1] = identifier
	copy(buf[off+mschapv2HeaderLen:], message[:msgLen])
	total := mschapv2HeaderLen + msgLen
	binary.BigEndian.PutUint16(buf[off+2:off+4], uint16(total))
	return total
}

// drawMSCHAPv2Challenge fills the first mschapv2ChallengeValueLen
// octets of dst with random bytes from crypto/rand. RFC 2759 Section 4:
// the Authenticator Challenge MUST be a 16-octet random value drawn
// from a cryptographic source. Panics with "BUG: ..." if dst is shorter
// than mschapv2ChallengeValueLen -- that is a programmer error, not a
// runtime failure.
func drawMSCHAPv2Challenge(dst []byte) error {
	if len(dst) < mschapv2ChallengeValueLen {
		panic("BUG: drawMSCHAPv2Challenge dst shorter than mschapv2ChallengeValueLen")
	}
	_, err := rand.Read(dst[:mschapv2ChallengeValueLen])
	return err
}

// runMSCHAPv2AuthPhase drives the MS-CHAPv2 mutual authentication as
// authenticator: draw a fresh 16-byte Authenticator Challenge, send
// Challenge, read the peer's Response, validate Value-Size (49) and
// zero-Reserved / zero-Flags, emit EventAuthRequest, wait for the
// external handler's AuthResponse carrying the 20-byte
// Authenticator-Response, and write Success (accept) or Failure
// (reject). Returns false if the session should tear down (reject,
// timeout, stop, wire error).
//
// The EventAuthRequest payload carries the 40-byte hash inputs the
// external handler needs to recompute GenerateAuthenticatorResponse:
// Challenge = the Authenticator Challenge ze sent; Response =
// Peer-Challenge (16) || NT-Response (24). Reserved and Flags are
// validated and dropped (RFC 2759 Section 5 hash recipes do not take
// them).
//
// Phase 7 replaces runAuthPhase's AuthMethodNone fallthrough with a
// method switch that calls this function when LCP negotiated
// Auth-Protocol = 0xC223 with Algorithm = 0x81 (MS-CHAPv2).
//
// RFC 2759 Section 4 inherits RFC 1994 Section 4.1's Identifier
// discipline: the Identifier MUST change each time a Challenge is sent.
// runMSCHAPv2AuthPhase increments s.chapIdentifier before drawing the
// next value and uses the result as the Challenge Identifier; the
// peer's Response MUST echo that same Identifier. The counter is shared
// with CHAP-MD5 because LCP negotiates exactly one Auth-Protocol method
// per session.
//
// TODO(phase-7): the draw-value / emit-event / wait-authRespCh /
// fail-on-stop triplet here duplicates the same sequence in
// runAuthPhase, runPAPAuthPhase, and runCHAPAuthPhase. Phase 7
// introduces auth.go as the canonical auth state machine; it should
// factor the shared emit/wait/fail shape into a helper (e.g.
// awaitAuthDecision) that all four runXxxAuthPhase functions call. This
// is the fourth call site -- the right moment to extract per
// design-principles.md "No premature abstraction".
func (s *pppSession) runMSCHAPv2AuthPhase() bool {
	var value [mschapv2ChallengeValueLen]byte
	if err := drawMSCHAPv2Challenge(value[:]); err != nil {
		s.fail("chap-v2: crypto/rand: " + err.Error())
		return false
	}
	s.chapIdentifier++
	identifier := s.chapIdentifier

	challengeBuf := getFrameBuf()
	defer putFrameBuf(challengeBuf)
	cOff := WriteFrame(challengeBuf, 0, ProtoCHAP, nil)
	cOff += WriteMSCHAPv2Challenge(challengeBuf, cOff, identifier, value[:], []byte(mschapv2LNSName))
	if !s.writeFrame(challengeBuf[:cOff]) {
		return false
	}

	readBuf := getFrameBuf()
	defer putFrameBuf(readBuf)
	n, err := s.chanFile.Read(readBuf)
	if err != nil {
		s.fail("chap-v2: chan fd read: " + err.Error())
		return false
	}
	proto, payload, _, perr := ParseFrame(readBuf[:n])
	if perr != nil {
		s.fail("chap-v2: malformed frame: " + perr.Error())
		return false
	}
	if proto != ProtoCHAP {
		s.fail("chap-v2: unexpected protocol 0x" +
			strconv.FormatUint(uint64(proto), 16))
		return false
	}
	resp, perr := ParseMSCHAPv2Response(payload)
	if perr != nil {
		s.fail("chap-v2: malformed response: " + perr.Error())
		return false
	}
	// RFC 2759 Section 4 (via RFC 1994 Section 4.1): Response
	// Identifier MUST match the outstanding Challenge Identifier.
	// Same fail-immediately behavior as runCHAPAuthPhase; retry loop
	// is deferred to Phase 9.
	if resp.Identifier != identifier {
		s.fail("chap-v2: response identifier 0x" +
			strconv.FormatUint(uint64(resp.Identifier), 16) +
			" does not match challenge 0x" +
			strconv.FormatUint(uint64(identifier), 16))
		return false
	}

	// Event payload: Challenge = the Authenticator Challenge we sent
	// (16 bytes, bytes.Clone so the caller may retain it past the read
	// buffer's lifetime); Response = Peer-Challenge || NT-Response
	// (40 bytes). Both are the full input set GenerateAuthenticatorResponse
	// needs alongside Username (= Name).
	responsePayload := make([]byte, mschapv2PeerChallengeLen+mschapv2NTResponseLen)
	copy(responsePayload, resp.PeerChallenge)
	copy(responsePayload[mschapv2PeerChallengeLen:], resp.NTResponse)

	evt := EventAuthRequest{
		TunnelID:   s.tunnelID,
		SessionID:  s.sessionID,
		Method:     AuthMethodMSCHAPv2,
		Identifier: identifier,
		Username:   resp.Name,
		Challenge:  bytes.Clone(value[:]),
		Response:   responsePayload,
		PeerName:   resp.Name,
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

	var decision authResponseMsg
	select {
	case decision = <-s.authRespCh:
	case <-s.stopCh:
		return false
	case <-s.sessStop:
		return false
	case <-timer.C:
		s.fail("chap-v2: auth timeout after " + timeout.String())
		s.sendAuthEvent(EventAuthFailure{
			TunnelID:  s.tunnelID,
			SessionID: s.sessionID,
			Reason:    "timeout",
		})
		return false
	}

	// Validate the plugin-supplied blob BEFORE acquiring the frame
	// buffer, so the rare bad-plugin path does not allocate work it
	// never uses.
	//
	// RFC 2759 Section 5: Success carries the 20-octet
	// Authenticator-Response which the peer verifies for mutual
	// authentication. The auth handler (l2tp-auth plugin in Phase 8)
	// computes it via GenerateAuthenticatorResponse and returns the
	// raw 20 bytes via decision.authResponseBlob; ze hex-encodes and
	// formats. A blob of any other length is a plugin protocol
	// violation -- fail the session cleanly rather than panic, because
	// the auth handler runs in an external process and the PPP
	// goroutine is not recovered at any higher level. A panic here
	// would crash the whole daemon for one misbehaving plugin.
	// WriteMSCHAPv2Success still panics on wrong-length blobs from
	// DIRECT callers (programmer error).
	if decision.accept && len(decision.authResponseBlob) != mschapv2AuthenticatorResponseLen {
		s.fail("chap-v2: auth handler returned authResponseBlob length " +
			strconv.Itoa(len(decision.authResponseBlob)) + ", want " +
			strconv.Itoa(mschapv2AuthenticatorResponseLen))
		s.sendAuthEvent(EventAuthFailure{
			TunnelID:  s.tunnelID,
			SessionID: s.sessionID,
			Reason:    "auth handler returned wrong authResponseBlob length",
		})
		return false
	}

	writeBuf := getFrameBuf()
	defer putFrameBuf(writeBuf)
	rOff := WriteFrame(writeBuf, 0, ProtoCHAP, nil)
	msg := []byte(decision.message)
	if decision.accept {
		rOff += WriteMSCHAPv2Success(writeBuf, rOff, identifier,
			decision.authResponseBlob, msg)
	} else {
		rOff += WriteMSCHAPv2Failure(writeBuf, rOff, identifier, msg)
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
	// Mirror runCHAPAuthPhase reject path: emit EventSessionDown on
	// the lifecycle channel via s.fail before EventAuthFailure on the
	// auth channel, so the transport can tear the session down.
	s.fail("chap-v2: auth rejected: " + decision.message)
	s.sendAuthEvent(EventAuthFailure{
		TunnelID:  s.tunnelID,
		SessionID: s.sessionID,
		Reason:    decision.message,
	})
	return false
}
