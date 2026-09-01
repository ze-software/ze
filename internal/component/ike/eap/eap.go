// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP framework and method dispatch
// RFC: rfc/short/rfc3748.md -- EAP packet format, exchange model (Sections 2, 4)

package eap

import (
	"errors"
	"fmt"
)

// EAP type codes.
const (
	TypeIdentity    uint8 = 1
	TypeNAK         uint8 = 3
	TypeTLS         uint8 = 13
	TypeMSCHAPv2    uint8 = 26
	TypeExpandedEAP uint8 = 254
)

// EAP codes.
const (
	CodeRequest  uint8 = 1
	CodeResponse uint8 = 2
	CodeSuccess  uint8 = 3
	CodeFailure  uint8 = 4
)

var (
	ErrUnsupportedMethod = errors.New("eap: unsupported method")
	ErrIdentityRequired  = errors.New("eap: identity response required first")
	ErrMethodFailed      = errors.New("eap: method authentication failed")
)

// Packet is a parsed EAP packet (Code, Identifier, Type, TypeData).
// RFC 3748 Section 4: Success/Failure packets have no Type field.
type Packet struct {
	Code       uint8
	Identifier uint8
	Type       uint8
	TypeData   []byte
}

// Encode serializes an EAP packet to wire format.
func (p *Packet) Encode() []byte {
	if p.Code == CodeSuccess || p.Code == CodeFailure {
		buf := make([]byte, 4)
		buf[0] = p.Code
		buf[1] = p.Identifier
		buf[2] = 0
		buf[3] = 4
		return buf
	}
	totalLen := 5 + len(p.TypeData)
	buf := make([]byte, totalLen)
	buf[0] = p.Code
	buf[1] = p.Identifier
	buf[2] = byte(totalLen >> 8)
	buf[3] = byte(totalLen)
	buf[4] = p.Type
	copy(buf[5:], p.TypeData)
	return buf
}

// DecodePacket parses raw EAP bytes into a Packet.
func DecodePacket(data []byte) (*Packet, error) {
	if len(data) < 4 {
		return nil, errors.New("eap: packet too short")
	}
	p := &Packet{
		Code:       data[0],
		Identifier: data[1],
	}
	eapLen := int(data[2])<<8 | int(data[3])
	if eapLen < 4 || eapLen > len(data) {
		return nil, errors.New("eap: invalid length")
	}
	if p.Code == CodeSuccess || p.Code == CodeFailure {
		return p, nil
	}
	if eapLen < 5 {
		return nil, errors.New("eap: request/response too short for type field")
	}
	p.Type = data[4]
	if eapLen > 5 {
		p.TypeData = make([]byte, eapLen-5)
		copy(p.TypeData, data[5:eapLen])
	}
	return p, nil
}

// MethodResult is the outcome of processing one EAP exchange round.
//
// Session.handleMethod reads the fields in this order, and the four outcomes
// they name are mutually exclusive:
//
//	FinalRequest  send it as the method's last word, then fail with Err
//	Done          the exchange succeeded and MSK carries the key
//	Err           the exchange failed, and an EAP-Failure answers the peer
//	Response      the exchange continues with this EAP-Request
//
// FinalRequest is the method's last word: a packet the method owes the peer on
// a refusal, sent by an exchange that has already failed. The EAP-Failure that
// RFC 3748 Section 4.2 obliges follows it on the next round, and nothing else
// can. A method that sets FinalRequest MUST set Err too, because the packet
// carries the protocol's reason and Err carries the operator's.
//
// Setting Response BESIDE Err puts the packet nowhere: the Err branch answers
// with an EAP-Failure and the Response is discarded. That is why the last word
// has a field of its own rather than a flag over Response, and it is a defect
// this package has already paid for once (see the EAP-TLS alert in
// tlsMethod.Process, eap_tls.go).
type MethodResult struct {
	Response     *Packet
	FinalRequest *Packet
	MSK          [64]byte
	Done         bool
	Err          error
}

// Method is the interface for an EAP authentication method (server/authenticator side).
type Method interface {
	// Type returns the EAP type code for this method.
	Type() uint8

	// Start generates the first EAP-Request for this method.
	Start(identifier uint8) *Packet

	// Process handles an EAP-Response from the peer and returns the next action.
	Process(response *Packet) MethodResult

	// Close releases every resource the method holds. The caller MUST call it
	// once the exchange has ended, for ANY reason: success, failure, refusal or
	// abandonment. A method that starts a goroutine leaks it otherwise.
	//
	// Close is idempotent and safe on a method that was never started.
	Close()
}

// Session manages a single EAP exchange (authenticator side).
type Session struct {
	method     Method
	identifier uint8
	identity   string
	msk        [64]byte
	state      sessionState

	// err is why the method refused, kept because the EAP-Failure packet cannot
	// carry a reason: RFC 3748 Section 4.2 gives Failure a Code, an Identifier and
	// a Length and no Type field at all, which `Packet.Encode` implements and
	// RFC3748-4.2-2 records. Without this the authenticator half of every method
	// discards its own diagnosis and the operator reads "authentication failed"
	// with nothing to act on. The EAP-TLS MSK export refusal is the case that
	// forced it (exportEAPTLSMSK, eap_tls.go), because the whole point of that
	// message is telling an operator what to change on the peer.
	err error
}

type sessionState uint8

const (
	stateIdentity sessionState = iota
	stateMethod
	stateSuccess
	stateFailure

	// stateLastWord is the exchange after a method's last word has gone out and
	// before the EAP-Failure that RFC 3748 Section 4.2 obliges: "After the
	// authenticator sends a failure result indication to the peer, regardless of
	// the response from the peer, it MUST subsequently send a Failure packet."
	// The exchange has already failed here, and Err already carries the cause.
	stateLastWord
)

// NewSession creates an EAP session for the given method type.
func NewSession(methodType uint8, config MethodConfig) (*Session, error) {
	var m Method
	switch methodType {
	case TypeMSCHAPv2:
		m = newMSCHAPv2Method(config)
	case TypeTLS:
		var err error
		m, err = newTLSMethod(config)
		if err != nil {
			return nil, fmt.Errorf("eap: create EAP-TLS method: %w", err)
		}
	default:
		return nil, fmt.Errorf("%w: type %d", ErrUnsupportedMethod, methodType)
	}
	return &Session{
		method:     m,
		identifier: 0,
		state:      stateIdentity,
	}, nil
}

// MethodConfig holds configuration needed by EAP methods.
type MethodConfig struct {
	// For EAP-MSCHAPv2.
	Password string `json:"-"` //nolint:gosec // EAP credential, never serialized

	// For EAP-TLS.
	CACertPEM     []byte
	ServerCertPEM []byte
	ServerKeyPEM  []byte
}

// Begin returns the initial EAP-Request/Identity packet.
// RFC 3748 Section 5.1: authenticator starts with Identity request.
func (s *Session) Begin() *Packet {
	s.identifier = 1
	return &Packet{
		Code:       CodeRequest,
		Identifier: s.identifier,
		Type:       TypeIdentity,
	}
}

// Process handles an incoming EAP-Response and returns the next EAP-Request
// (or Success/Failure). Returns nil when the exchange is complete and the
// final packet has already been returned.
func (s *Session) Process(response *Packet) *Packet {
	// RFC 3748 Section 4: "Since EAP only defines Codes 1-4, EAP packets with
	// other codes MUST be silently discarded by both authenticators and peers."
	//
	// A nil packet is the discard: handleResponderEAP
	// (internal/component/ike/engine/responder_eap.go) sends nothing for it, and
	// the exchange keeps the state it had, so the peer's next packet is read as if
	// this one had never arrived. Answering an undefined Code with an EAP-Failure,
	// which this did until 2026-09-01, ends a conversation on a packet the RFC
	// says not to read at all, and lets anybody on the path kill an EAP exchange
	// with one unauthenticated octet.
	if response.Code == 0 || response.Code > CodeFailure {
		return nil
	}
	if response.Code != CodeResponse {
		return s.failure(response)
	}

	// RFC 3748 Section 4.1: "An authenticator receiving a Response whose
	// Identifier value does not match that of the currently outstanding Request
	// MUST silently discard the Response."
	//
	// s.identifier IS that outstanding value while the exchange waits: every
	// Request leaves carrying it, and it is reassigned only when a terminal
	// packet answers a Response's own Identifier. So an answer to a Request this
	// exchange never sent, or a replay of one already answered, is dropped here
	// rather than fed to the method. The peer's duty to echo the Identifier is
	// RFC3748-4.1-4, a different row: this one is the authenticator's duty to
	// check that it did.
	if response.Identifier != s.identifier {
		return nil
	}

	switch s.state {
	case stateIdentity:
		return s.handleIdentity(response)
	case stateMethod:
		return s.handleMethod(response)
	case stateLastWord:
		// RFC 3748 Section 4.2: "After the authenticator sends a failure result
		// indication to the peer, regardless of the response from the peer, it
		// MUST subsequently send a Failure packet." The response is not read:
		// whatever the peer answered the last word with, this exchange owes it
		// an EAP-Failure and nothing else.
		return s.failure(response)
	default:
		return nil
	}
}

// Close releases every resource the exchange holds.
//
// The caller MUST call it once the exchange has ended, for ANY reason: an
// EAP-Success, an EAP-Failure, a refused method, or a peer that stopped
// answering. EAP-TLS runs its TLS engine on a goroutine parked in a read that
// only this call can release, so an exchange that ends without it leaks that
// goroutine and the TLS keys it holds. The peer decides how many exchanges
// start and how many of them it abandons, and it is unauthenticated while it
// does so, which is what makes the omission reachable from the network.
//
// Close is idempotent and safe on a session whose method never started.
func (s *Session) Close() {
	if s == nil || s.method == nil {
		return
	}
	s.method.Close()
}

// Identity returns the peer identity extracted from the EAP-Response/Identity.
func (s *Session) Identity() string { return s.identity }

// State returns whether the session completed successfully.
func (s *Session) Succeeded() bool { return s.state == stateSuccess }

// Err returns why the method refused the peer, or nil when the exchange failed
// for a reason the method never saw: a peer that answered the Identity request
// with something else, or a NAK of the offered method. The caller MUST log it
// beside its own failure line, because RFC 3748 Section 4.2 leaves an EAP-Failure
// packet no field to carry a reason in and this is the only place one exists.
func (s *Session) Err() error { return s.err }

// MSK returns the Master Session Key after successful authentication.
func (s *Session) MSK() [64]byte {
	return s.msk
}

// msk stores the derived MSK.
var _ = (*Session)(nil) // compile check

func (s *Session) handleIdentity(response *Packet) *Packet {
	if response.Type != TypeIdentity {
		if response.Type == TypeNAK {
			return s.failure(response)
		}
		return s.failure(response)
	}

	s.identity = string(response.TypeData)
	s.state = stateMethod
	s.identifier++
	return s.method.Start(s.identifier)
}

func (s *Session) handleMethod(response *Packet) *Packet {
	if response.Type == TypeNAK {
		return s.failure(response)
	}

	result := s.method.Process(response)
	if result.FinalRequest != nil {
		return s.finalRequest(result)
	}
	if result.Err != nil {
		s.err = result.Err
		s.state = stateFailure
		return s.failure(response)
	}
	if result.Done {
		s.state = stateSuccess
		s.msk = result.MSK
		// RFC 3748 Section 4.2: "The Identifier field MUST match the Identifier
		// field of the Response packet that it is sent in response to." Success
		// ends the exchange rather than opening one, so it answers the Response's
		// Identifier and does not advance the counter. Same rule, and same former
		// off-by-one, as failure below.
		s.identifier = response.Identifier
		return &Packet{
			Code:       CodeSuccess,
			Identifier: s.identifier,
		}
	}
	s.identifier++
	if result.Response != nil {
		result.Response.Identifier = s.identifier
	}
	return result.Response
}

// finalRequest sends a method's last word: the packet a method owes the peer on
// a refusal, sent by an exchange that has already failed.
//
// The decision is taken here and now. Err is recorded, no MSK is kept, and the
// only packet this exchange can produce afterwards is the EAP-Failure. That is
// what separates a last word from the parked cause EAP-TLS uses, where the
// Session does not learn of the failure until the round after.
//
// One round still follows, and the lower layer is why. RFC 3748 Section 4.2:
// "After the authenticator sends a failure result indication to the peer,
// regardless of the response from the peer, it MUST subsequently send a Failure
// packet." RFC 7296 Section 2.16 carries one EAP payload for each IKE_AUTH
// message, so the last word and the EAP-Failure cannot share a round.
//
// The packet is an EAP-Request, so it takes a new Identifier exactly as the
// continuing round below does.
func (s *Session) finalRequest(result MethodResult) *Packet {
	if result.Err == nil {
		panic("BUG: MethodResult.FinalRequest was set with no Err, so the exchange would fail with no cause")
	}
	s.err = result.Err
	s.state = stateLastWord
	s.identifier++
	result.FinalRequest.Identifier = s.identifier
	return result.FinalRequest
}

// failure ends the exchange, answering the packet the caller was given.
//
// RFC 3748 Section 4.2: "The Identifier field MUST match the Identifier field of
// the Response packet that it is sent in response to." A Failure does not open a
// new exchange, so it does not advance s.identifier: it addresses the one it is
// ending. Incrementing first, which this did until 2026-08-05, made every
// EAP-Failure carry the answered Identifier plus one, and a peer enforcing
// Section 4.2 discards it.
//
// answered may be nil only if a future caller has no packet in hand. There is no
// such caller today, and the fallback keeps the old numbering rather than
// stamping a zero that would look like a valid Identifier.
func (s *Session) failure(answered *Packet) *Packet {
	s.state = stateFailure
	id := s.identifier + 1
	if answered != nil {
		id = answered.Identifier
	}
	s.identifier = id
	return &Packet{
		Code:       CodeFailure,
		Identifier: id,
	}
}
