// Design: plan/learned/744-ipsec-9-ikev2-eap-nat.md -- EAP-MSCHAPv2 method handler
// RFC: rfc/short/rfc2759.md -- MS-CHAPv2 exchange inside EAP (type 26)

package eap

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// MS-CHAPv2 op codes inside EAP type 26.
const (
	mschapv2OpChallenge uint8 = 1
	mschapv2OpResponse  uint8 = 2
	mschapv2OpSuccess   uint8 = 3
	// mschapv2OpFailure is 4 per RFC 2759; we use EAP-Failure instead of an MS-CHAPv2 Failure packet.
	_ uint8 = 4
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
	userName := StripDomain(peerName)

	if !VerifyNTResponse(m.authChallenge, peerChallenge, userName, m.password, ntResponse) {
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

func (m *mschapv2Method) sendFailure() MethodResult {
	return MethodResult{Err: ErrMethodFailed}
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
