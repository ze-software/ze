// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP framework and method dispatch
// RFC: rfc/short/rfc3748.md -- EAP packet format, exchange model (Sections 2, 4)

package eap

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// EAP type codes.
//
// RFC 3748 Section 5: "All EAP implementations MUST support Types 1-4, which
// are defined in this document, and SHOULD support Type 254." ze supports
// Types 1, 2, 3 and 4. Type 254 is answered with the legacy Nak that Section
// 5.7 prescribes for a peer not equipped to interpret it.
//
// Type 4 (MD5-Challenge) was left unimplemented under an owner-authorized
// deviation of 2026-08-30 and implemented on 2026-09-01, which reverses it. The
// deviation rested on RFC 7296 Section 2.16, "EAP methods that do not establish
// a shared key SHOULD NOT be used", and that sentence governs which method an
// IKEv2 operator MAY select. It discharges no obligation to SUPPORT the Type,
// and RFC 3748 Section 5.4 addresses the requirement to an authenticator that
// authenticates peers locally, which is what ze's does.
const (
	TypeIdentity     uint8 = 1
	TypeNotification uint8 = 2
	TypeNAK          uint8 = 3
	TypeMD5Challenge uint8 = 4
	TypeTLS          uint8 = 13
	TypeMSCHAPv2     uint8 = 26
	TypeExpandedEAP  uint8 = 254
)

// TypeDerivesKey reports whether an EAP method Type produces a Master Session
// Key as a side effect of authentication.
//
// It is the ONE declaration of that fact. Every Method.DerivesKey answers from
// it, and PeerSession.DerivesKey reads it directly, because the peer half runs
// its methods inline and holds no Method instance to ask. A second copy of the
// list would be a future disagreement with nothing to arbitrate it
// (ai/rules/principles.md).
//
// It is exported for one caller outside this package: warnKeylessEAPModes
// (internal/component/ike/engine/eap_auth.go) writes the operator's warning for
// a configured mode, before any session exists to ask.
//
// RFC 7296 Section 2.16 is why the question is asked at all: "For EAP methods
// that create a shared key as a side effect of authentication, that shared key
// MUST be used by both the initiator and responder to generate AUTH payloads in
// messages 7 and 8 using the syntax for shared secrets specified in Section
// 2.15.  The shared key from EAP is the field from the EAP specification named
// MSK." and "If EAP methods that do not generate a shared key are used, the AUTH
// payloads in messages 7 and 8 MUST be generated using SK_pi and SK_pr,
// respectively."
//
// So the carrier MUST ASK which of the two rules applies. Inferring it from an
// all-zero MSK cannot work: an all-zero MSK is exactly what a method that FAILED
// leaves behind, so the zero would be read as a valid answer to a question it
// never answered (ai/rules/principles.md).
//
// An unknown Type answers false, and the direction is deliberate. False routes
// the carrier to SK_pi/SK_pr, which authenticates; true would route it to an MSK
// no method filled. NewSession refuses an unknown Type before a session exists,
// so no exchange reaches this with one.
func TypeDerivesKey(methodType uint8) bool {
	switch methodType {
	case TypeTLS:
		// RFC 5216 Section 2.3 derives the MSK from the TLS master secret.
		return true
	case TypeMSCHAPv2:
		// RFC 3748 Section 5 lists no MSK for MS-CHAPv2, but draft-kamath does,
		// and DeriveMSK (mschapv2.go) implements it.
		return true
	default:
		// RFC 3748 Section 5.4 Security Claims for MD5-Challenge: "Key
		// derivation:         No".
		return false
	}
}

// typeAuthenticationLow and typeAuthenticationHigh bound the authentication
// Types, which are the ones a peer may refuse with a legacy Nak.
//
// RFC 3748 Section 5.3.1: "Where a peer receives a Request for an unacceptable
// authentication Type (4-253,255), or a peer lacking support for Expanded Types
// receives a Request for Type 254, a Nak Response (Type 3) MUST be sent."
// Types 1, 2 and 3 sit below the range and are never Nak'd; 254 is named
// separately because Section 5.7 routes it here rather than the range doing so.
const (
	typeAuthenticationLow  uint8 = 4
	typeAuthenticationHigh uint8 = 253
	typeExperimental       uint8 = 255
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

	// DerivesKey reports whether this method fills MethodResult.MSK on success.
	//
	// The carrier has to ask, because RFC 7296 Section 2.16 gives the two kinds of
	// method two different AUTH payloads: a method that establishes a shared key
	// uses the MSK, and one that does not uses SK_pi and SK_pr. An all-zero MSK
	// cannot answer the question, since that is also what a method that failed
	// leaves behind. Every implementation answers from TypeDerivesKey, which is
	// the single declaration of the fact.
	DerivesKey() bool

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

	// methodAnswered records that the peer has answered a Request of the method's
	// own Type with a non-Nak Response. It is the authenticator's mirror of
	// PeerSession.methodCommitted, and it reads the same sentence from the other
	// end: RFC 3748 Section 2.1, "A peer MUST NOT send a Nak (legacy or expanded)
	// in reply to a Request after an initial non-Nak Response has been sent.
	// Since spoofed EAP Request packets may be sent by an attacker, an
	// authenticator receiving an unexpected Nak SHOULD discard it and log the
	// event."
	//
	// The Identity Response does NOT set it, for the reason the peer's field
	// carries: Section 5.4 describes a Nak sent in answer to the Request that
	// FOLLOWS the Identity Response, so counting the Identity Response would turn
	// every legitimate Nak into an unexpected one.
	methodAnswered bool

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
	case TypeMD5Challenge:
		m = newMD5ChallengeMethod(config)
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
	// Password is the shared secret. EAP-MSCHAPv2 hashes it into the NT password
	// hash, and MD5-Challenge uses it as the CHAP "secret" of RFC 1994 Section
	// 4.1. One field carries both, because both methods hold one credential the
	// operator configured and neither can hold the other's.
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

// Err returns the exchange's diagnosis, or nil when it has none: why the method
// refused the peer, why a Nak ended the exchange, or why a packet was discarded.
// The caller MUST log it beside its own failure line, because RFC 3748 Section
// 4.2 leaves an EAP-Failure packet no field to carry a reason in and this is the
// only place one exists.
//
// A non-nil value does NOT mean the exchange ended: an unexpected Nak is
// discarded and recorded here while the exchange continues (nakUnexpected), so a
// caller reads Succeeded rather than this to learn the outcome.
func (s *Session) Err() error { return s.err }

// MSK returns the Master Session Key after successful authentication.
//
// The value is meaningless unless DerivesKey reports true. A method that derives
// no key, MD5-Challenge among them, succeeds with this array still all zero.
func (s *Session) MSK() [64]byte {
	return s.msk
}

// DerivesKey reports whether the configured method fills MSK on success.
//
// RFC 7296 Section 2.16 makes the answer decide which AUTH payload the IKEv2
// carrier computes: "For EAP methods that create a shared key as a side effect
// of authentication, that shared key MUST be used by both the initiator and
// responder to generate AUTH payloads in messages 7 and 8", and "If EAP methods
// that do not generate a shared key are used, the AUTH payloads in messages 7
// and 8 MUST be generated using SK_pi and SK_pr, respectively."
//
// The carrier asks rather than reading MSK, because an all-zero MSK is
// indistinguishable from a method that failed, and a zero that looks like a
// valid answer is the defect ai/rules/principles.md forbids.
//
// A session with no method answers false, which routes the carrier to
// SK_pi/SK_pr. NewSession builds no such session; the guard exists because Close
// and Err tolerate one, so this answers rather than panicking.
func (s *Session) DerivesKey() bool {
	if s == nil || s.method == nil {
		return false
	}
	return s.method.DerivesKey()
}

// msk stores the derived MSK.
var _ = (*Session)(nil) // compile check

func (s *Session) handleIdentity(response *Packet) *Packet {
	if response.Type != TypeIdentity {
		if response.Type == TypeNAK {
			return s.nakRefused(response)
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
		if s.methodAnswered {
			return s.nakUnexpected(response)
		}
		return s.nakRefused(response)
	}

	// RFC 3748 Section 4.1: "An EAP server receiving a Response not meeting
	// these requirements MUST silently discard it." The requirement above it is
	// that a Response answers the Request's Type or is a legacy Nak, and a Nak
	// is the arm just handled. So a Response of any other Type is one this
	// exchange never asked for, and the RFC asks for silence rather than an
	// EAP-Failure: answering it lets anybody on the path end a conversation with
	// one unauthenticated packet, which is the shape RFC3748-4-5 already closed
	// for an undefined Code.
	if response.Type != s.method.Type() {
		return nil
	}

	// The peer has now sent the "initial non-Nak Response" of RFC 3748 Section
	// 2.1, so every Nak that follows is one the peer is forbidden to send. The
	// commitment is the Response ARRIVING rather than the method accepting its
	// contents: the sentence bounds what the peer sent, and a malformed payload
	// is still a non-Nak Response the peer chose to send.
	s.methodAnswered = true

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

// nakRefused ends the exchange on a legacy Nak and records the Types the peer
// asked for instead.
//
// RFC 3748 Section 5.3.1: "The Type-Data field of the Nak Response (Type 3)
// MUST contain one or more octets indicating the desired authentication
// Type(s), one octet per Type, or the value zero (0) to indicate no proposed
// alternative."
//
// ze's authenticator offers exactly one method, so a Nak the peer was entitled
// to send ends the exchange. Those octets are then the only word the peer gets
// to say about WHY, and an EAP-Failure has no field to carry it: RFC 3748
// Section 4.2 gives Failure a Code, an Identifier and a Length and nothing else.
// Discarding them, which this did until 2026-09-01, left an operator reading
// "authentication failed" with no way to learn that the far end wanted a method
// ze does not run.
//
// A Nak the peer was NOT entitled to send takes nakUnexpected instead.
func (s *Session) nakRefused(response *Packet) *Packet {
	s.err = nakRefusal(s.method.Type(), response.TypeData)
	return s.failure(response)
}

// nakUnexpected discards a Nak the peer is no longer allowed to send, and
// records why.
//
// RFC 3748 Section 2.1: "A peer MUST NOT send a Nak (legacy or expanded) in
// reply to a Request after an initial non-Nak Response has been sent.  Since
// spoofed EAP Request packets may be sent by an attacker, an authenticator
// receiving an unexpected Nak SHOULD discard it and log the event."
//
// Both halves of that sentence are here. The discard is the nil return
// Session.Process already uses for a packet it did not read: handleResponderEAP
// (internal/component/ike/engine/responder_eap.go) sends nothing for it and the
// exchange keeps the state it had, so the peer's next packet is read as if this
// one had never arrived. The log is Session.err, which is the only field this
// exchange has to carry a diagnosis to the operator.
//
// Ending the exchange instead, which this did until 2026-09-01, let one spoofed
// packet turn a live authentication into an EAP-Failure and a dead IKE SA.
func (s *Session) nakUnexpected(response *Packet) *Packet {
	s.err = fmt.Errorf(
		"eap: the peer sent a Nak asking for type %s after it had already answered a type %d Request, which RFC 3748 Section 2.1 forbids; the packet was discarded",
		desiredTypes(response.TypeData), s.method.Type())
	return nil
}

// nakRefusal names the method that was offered and the Types the Nak asked for.
func nakRefusal(offered uint8, desired []byte) error {
	if len(desired) == 0 {
		return fmt.Errorf("eap: the peer refused type %d with a Nak carrying no desired type", offered)
	}

	// RFC 3748 Section 5.3.1: "Type zero (0) is used to indicate that the sender
	// has no viable alternatives, and therefore the authenticator SHOULD NOT send
	// another Request after receiving a Nak Response containing a zero value."
	// ze sends no further Request in either case, because it has only one method
	// to offer, so the zero is reported rather than acted on.
	if len(desired) == 1 && desired[0] == 0 {
		return fmt.Errorf("eap: the peer refused type %d with a Nak proposing no alternative", offered)
	}
	return fmt.Errorf("eap: the peer refused type %d with a Nak asking for type %s", offered, desiredTypes(desired))
}

// desiredTypeMax bounds how many desired-Type octets a Nak is read for.
//
// RFC 3748 Section 5.3.1 states what the field holds: "Authentication Types are
// numbered 4 and above", and the Type-Data carries "one octet per Type". There
// are 252 such values, 4 through 255, so a Nak longer than that has repeated
// itself and no further octet can name a Type the list does not already hold.
// The bound is the RFC's own count rather than a number chosen here.
//
// A bound is owed because the packet is unauthenticated and arrives before any
// key exists. wireEAPToPacket (internal/component/ike/engine/fsm.go) slices
// Type-Data out of the whole EAP payload with no cap of its own, so one IKE_AUTH
// can carry tens of thousands of octets into the error string and from there
// into an operator's log line. The peer's Notification path is bounded for the
// same reason (notificationMax, peer.go).
const desiredTypeMax = 252

// desiredTypes renders the authentication Types a Nak asked for, reading at most
// desiredTypeMax octets and saying so when it stopped short.
func desiredTypes(desired []byte) string {
	if len(desired) == 0 {
		return "none"
	}

	truncated := len(desired) > desiredTypeMax
	if truncated {
		desired = desired[:desiredTypeMax]
	}

	var b textbuf.Buffer
	b.Reset()
	for i, t := range desired {
		if i > 0 {
			b.Str(", ")
		}
		b.Uint8(t)
	}
	if truncated {
		b.Str(" (truncated)")
	}
	return b.String()
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
