// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-MSCHAPv2 method handler
// RFC: rfc/short/rfc2759.md -- MS-CHAPv2 exchange inside EAP (type 26)

package eap

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// MS-CHAPv2 op codes inside EAP type 26.
const (
	mschapv2OpChallenge uint8 = 1
	mschapv2OpResponse  uint8 = 2
	mschapv2OpSuccess   uint8 = 3
	mschapv2OpFailure   uint8 = 4
)

type mschapv2State uint8

const (
	mschapv2StateChallenge mschapv2State = iota
	mschapv2StateResponse
	mschapv2StateDone
)

type mschapv2Method struct {
	password      string
	authChallenge [16]byte
	msID          uint8
	state         mschapv2State
	msk           [64]byte
}

func newMSCHAPv2Method(config MethodConfig) *mschapv2Method {
	return &mschapv2Method{
		password: config.Password,
	}
}

func (m *mschapv2Method) Type() uint8 { return TypeMSCHAPv2 }

// DerivesKey answers true: handleResponse fills m.msk from DeriveMSK, and
// handleSuccessAck hands it to the exchange. TypeDerivesKey holds the single
// declaration.
func (m *mschapv2Method) DerivesKey() bool { return TypeDerivesKey(TypeMSCHAPv2) }

// Close releases the method's resources. MS-CHAPv2 runs entirely inside
// Process, so it holds no goroutine and no connection, and this does nothing.
// The method still declares it, because Method requires every implementation to
// answer the question rather than leave the caller to know which ones need it.
func (m *mschapv2Method) Close() {}

// Start sends the MS-CHAPv2 Challenge wrapped in EAP type 26.
// RFC 2759 Section 4: Challenge packet (Code 1).
func (m *mschapv2Method) Start(identifier uint8) *Packet {
	if _, err := rand.Read(m.authChallenge[:]); err != nil {
		return nil
	}
	m.msID = identifier
	m.state = mschapv2StateChallenge

	// MS-CHAPv2 Challenge: OpCode(1) + MS-ID(1) + MS-Length(2) + ValueSize(1) + Challenge(16) + Name.
	name := []byte("ze")
	msLen := 5 + 16 + len(name)
	td := make([]byte, msLen)
	td[0] = mschapv2OpChallenge
	td[1] = m.msID
	td[2] = byte(msLen >> 8)
	td[3] = byte(msLen)
	td[4] = 16
	copy(td[5:21], m.authChallenge[:])
	copy(td[21:], name)

	return &Packet{
		Code:       CodeRequest,
		Identifier: identifier,
		Type:       TypeMSCHAPv2,
		TypeData:   td,
	}
}

// Process handles an EAP-Response/MSCHAPv2 from the peer.
func (m *mschapv2Method) Process(response *Packet) MethodResult {
	if response.Type != TypeMSCHAPv2 {
		return MethodResult{Err: ErrMethodFailed}
	}
	// One octet is the floor for every shape: the OpCode. A per-shape length is
	// checked where that shape is parsed. A blanket four-octet floor here refused
	// the Success Response, which carries no other field (see below).
	if len(response.TypeData) == 0 {
		return MethodResult{Err: fmt.Errorf("eap-mschapv2: empty response")}
	}

	opCode := response.TypeData[0]

	switch m.state {
	case mschapv2StateChallenge:
		if opCode != mschapv2OpResponse {
			return MethodResult{Err: ErrMethodFailed}
		}
		return m.handleResponse(response.TypeData)
	case mschapv2StateResponse:
		// RFC 2759 Section 5: the peer's Success Response packet is a SINGLE
		// octet, the OpCode 3. It has no MS-ID, no MS-Length and no Message
		// field, so the four-octet floor this used to sit behind rejected a
		// conformant peer outright.
		//
		// Ze's own peer sends four octets (handleMSCHAPv2Success in peer.go
		// appends an MS-ID and a length), which is why every Ze-to-Ze test passed
		// over this. strongSwan sends the one octet the RFC describes, so the
		// authenticator answered its successful authentication with an
		// EAP-Failure.
		if opCode != mschapv2OpSuccess {
			return MethodResult{Err: fmt.Errorf("eap-mschapv2: expected Success opcode %d in the acknowledgement, got %d", mschapv2OpSuccess, opCode)}
		}
		return m.handleSuccessAck()
	default:
		return MethodResult{Err: ErrMethodFailed}
	}
}

func (m *mschapv2Method) handleResponse(td []byte) MethodResult {
	// MS-CHAPv2 Response: OpCode(1) + MS-ID(1) + MS-Length(2) + ValueSize(1) + Response(49) + Name.
	if len(td) < 54 {
		return MethodResult{Err: fmt.Errorf("eap-mschapv2: response too short: %d", len(td))}
	}
	valueSize := td[4]
	// RFC 2759: Value-Size in Response MUST be 49.
	if valueSize != 49 {
		return MethodResult{Err: fmt.Errorf("eap-mschapv2: invalid value-size %d", valueSize)}
	}

	var peerChallenge [16]byte
	copy(peerChallenge[:], td[5:21])

	// Reserved (8 octets) MUST be zero.
	for _, b := range td[21:29] {
		if b != 0 {
			return MethodResult{Err: fmt.Errorf("eap-mschapv2: non-zero reserved field")}
		}
	}

	var ntResponse [24]byte
	copy(ntResponse[:], td[29:53])

	// Flags octet MUST be zero.
	if td[53] != 0 {
		return MethodResult{Err: fmt.Errorf("eap-mschapv2: non-zero flags")}
	}

	// Extract peer name.
	peerName := ""
	if len(td) > 54 {
		peerName = string(td[54:])
	}
	userName := stripDomain(peerName)

	if !verifyNTResponse(m.authChallenge, peerChallenge, userName, m.password, ntResponse) {
		return m.sendFailure()
	}

	m.msk = DeriveMSK(m.password, ntResponse)

	authResp := GenerateAuthenticatorResponse(m.password, ntResponse, peerChallenge, m.authChallenge, userName)
	m.state = mschapv2StateResponse

	return m.sendSuccess(authResp)
}

func (m *mschapv2Method) sendSuccess(authResp [20]byte) MethodResult {
	sHex := strings.ToUpper(hex.EncodeToString(authResp[:]))
	msg := "S=" + sHex + " M=Authentication successful"

	msLen := 4 + len(msg)
	td := make([]byte, msLen)
	td[0] = mschapv2OpSuccess
	td[1] = m.msID
	td[2] = byte(msLen >> 8)
	td[3] = byte(msLen)
	copy(td[4:], msg)

	return MethodResult{
		Response: &Packet{
			Code:     CodeRequest,
			Type:     TypeMSCHAPv2,
			TypeData: td,
		},
	}
}

// MS-CHAPv2 Failure Message fields, RFC 2759 Section 6.
const (
	// mschapv2ErrorAuthenticationFailure is the E= code for a Response the
	// authenticator could not verify. RFC 2759 Section 6 lists it among the
	// codes the field carries: "691 ERROR_AUTHENTICATION_FAILURE".
	mschapv2ErrorAuthenticationFailure = 691

	// mschapv2PasswordChangeVersion is the V= value. RFC 2759 Section 6: "The
	// "vvvvvvvvvv" is the ASCII representation of a decimal version code (need
	// not be 10 digits) indicating the password changing protocol version
	// supported on the server.  For MS-CHAP-V2, this value SHOULD always be 3."
	// sendFailure writes this value into every Failure Message.
	mschapv2PasswordChangeVersion = 3

	// mschapv2ChallengeDigits is the length of the C= field. RFC 2759 Section 6:
	// "This field MUST be exactly 32 octets long and MUST be present." The peer
	// half checks an arriving C= field against this length.
	mschapv2ChallengeDigits = 32
)

// sendFailure answers an NT-Response the authenticator could not verify with an
// MS-CHAPv2 Failure packet, and ends the exchange behind it.
//
// RFC 2759 Section 6: "The Failure packet is identical in format to the standard
// CHAP Failure packet.  There is, however, formatted text stored in the Message
// field which, contrary to the standard CHAP rules, does affect the operation of
// the protocol.  The Message field format is:
//
//	"E=eeeeeeeeee R=r C=cccccccccccccccccccccccccccccccc V=vvvvvvvvvv M=<msg>""
//
// Wire format, over the draft-kamath EAP encapsulation:
//
//	 0        1        2        3        4                        msLen
//	+--------+--------+--------+--------+-------------------------+
//	| OpCode | MS-ID  |    MS-Length    | E= R= C= V= M=<message> |
//	+--------+--------+--------+--------+-------------------------+
//	    4      answered  total octets     the formatted Message
//
// R is '0' because Ze offers no retry. RFC 2759 Section 6: "The "r" is an ASCII
// flag set to '1' if a retry is allowed, and '0' if not.  When the authenticator
// sets this flag to '1' it disables short timeouts, expecting the peer to prompt
// the user for new credentials and resubmit the response." An IKEv2 EAP exchange
// inside a router has no user to prompt, so the flag states the truth.
//
// The C= challenge is drawn fresh for each refusal and is not kept: it is the
// challenge a retry would answer, and this authenticator offers none. The
// mandate is on the field, not on the retry.
func (m *mschapv2Method) sendFailure() MethodResult {
	var challenge [16]byte
	if _, err := rand.Read(challenge[:]); err != nil {
		return MethodResult{Err: fmt.Errorf("eap-mschapv2: generate the Failure challenge: %w", err)}
	}

	// RFC 2759 Section 6: "This field MUST be exactly 32 octets long and MUST be
	// present." Sixteen octets of hexadecimal are exactly 32 digits, and Section
	// 5 sets the case this file writes hexadecimal in.
	msg := "E=" + strconv.Itoa(mschapv2ErrorAuthenticationFailure) +
		" R=0" +
		" C=" + strings.ToUpper(hex.EncodeToString(challenge[:])) +
		" V=" + strconv.Itoa(mschapv2PasswordChangeVersion) +
		" M=Authentication failure"

	msLen := 4 + len(msg)
	td := make([]byte, msLen)
	td[0] = mschapv2OpFailure
	td[1] = m.msID
	td[2] = byte(msLen >> 8)
	td[3] = byte(msLen)
	copy(td[4:], msg)

	// The method is finished, so the credential it was given goes now rather
	// than at a Close the caller might skip.
	m.state = mschapv2StateDone
	m.password = ""

	return MethodResult{
		FinalRequest: &Packet{
			Code:     CodeRequest,
			Type:     TypeMSCHAPv2,
			TypeData: td,
		},
		Err: fmt.Errorf("%w: the NT-Response does not match the password, error code %d", ErrMethodFailed, mschapv2ErrorAuthenticationFailure),
	}
}

func (m *mschapv2Method) handleSuccessAck() MethodResult {
	m.state = mschapv2StateDone
	msk := m.msk
	clear(m.msk[:])
	m.password = ""
	return MethodResult{
		MSK:  msk,
		Done: true,
	}
}
