// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP peer (client/initiator) side
// RFC: rfc/short/rfc3748.md -- EAP peer (client) side exchange
// RFC: rfc/short/rfc7296.md -- Section 2.16: EAP in IKE_AUTH (initiator is EAP peer)
// RFC: rfc/short/rfc5216.md -- EAP-TLS fragmentation (Section 2.1.5)

package eap

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
)

const maxEAPRounds = 20

var (
	ErrTooManyRounds = errors.New("eap: exceeded maximum exchange rounds")
	ErrEAPFailure    = errors.New("eap: authenticator sent Failure")

	// errSessionEnded refuses every packet that arrives after the peer ended the
	// session. An EAP-Success is the packet a rogue authenticator sends next, and
	// answering it with the MSK would undo the refusal that produced this state.
	errSessionEnded = errors.New("eap: the peer session ended and answers nothing further")

	// errNoPeerTrustAnchor refuses an EAP-TLS peer session that could not
	// path-validate the authenticator. RFC 5216 Section 5.3 makes that validation
	// a MUST, and the configured CA is the peer's only trust anchor: EAP carries
	// no server hostname, so there is nothing else to check the chain against.
	errNoPeerTrustAnchor = errors.New("eap-tls: no CA certificate configured: RFC 5216 Section 5.3 requires the peer to path-validate the authenticator chain, which needs a trust anchor")

	// errTLSClientStalled reports a TLS client that settled and wrote nothing
	// while its handshake was still running. The other answer is the bare flags
	// octet. RFC 5216 Section 2.1.5 permits that empty EAP-TLS message only in
	// answer to a message with the M flag. An authenticator refuses it and fails
	// the method. The stall then reaches the operator as the peer's silence.
	errTLSClientStalled = errors.New("eap-tls: the TLS client produced no handshake data and its handshake is not complete")
)

// PeerResult is the outcome of processing one EAP-Request from the authenticator.
//
// Four outcomes, and the caller reads them in this order (handleEAPResponse,
// internal/component/ike/engine/fsm.go):
//
//	Err        the exchange failed, and the IKE SA is put in StateDead
//	Done       the exchange succeeded and MSK carries the key
//	Response   the exchange continues with this EAP-Response
//	Discarded  the packet was dropped and the exchange waits for the next one
//
// Discarded is a field rather than the absence of the other three. RFC 3748
// Section 4.2 makes a peer drop several packets in silence, and the wire
// behavior of dropping one is indistinguishable from a result that fell out of
// a branch nobody wrote: both send nothing and both end no exchange. A zero
// value that reads as a valid answer is what ai/rules/principles.md forbids, so
// the drop says so, and the caller logs it because a forged EAP-Success is a
// thing an operator wants named.
type PeerResult struct {
	Response  *Packet
	MSK       [64]byte
	Done      bool
	Discarded bool
	Err       error
}

// PeerTLSConfig holds certificate material for the EAP-TLS peer (client).
type PeerTLSConfig struct {
	CertPEM   []byte
	KeyPEM    []byte
	CACertPEM []byte
}

// PeerSession manages the EAP peer (client/initiator) side of an exchange.
type PeerSession struct {
	tlsFragmenter
	identity string
	password string
	method   uint8
	rounds   int
	state    peerState
	msk      [64]byte

	// MSCHAPv2 state. The Authenticator Challenge and the NT-Response are held
	// from the Challenge round to the Success round, because RFC 2759 Section 5
	// makes the peer recompute the Authenticator Response over both of them.
	peerChallenge [16]byte
	authChallenge [16]byte
	ntResponse    [24]byte
	userName      string

	// EAP-TLS state.
	tlsCfg       *PeerTLSConfig
	tlsConn      *tls.Conn
	tlsTransport *eapTLSTransport
	tlsStarted   atomic.Bool
	tlsDone      atomic.Bool

	// tlsErr holds the TLS handshake goroutine's error, so a failure reports its
	// cause rather than only the "no MSK" consequence.
	tlsErr atomic.Pointer[error]

	// pendingErr holds a TLS failure whose EAP-Response has already gone out, so
	// the round that follows can report it.
	//
	// RFC 5216 Section 2.1.3 makes the authenticator WAIT: "To ensure that the peer
	// receives the TLS alert message, the EAP server MUST wait for the peer to
	// reply with an EAP-Response packet." A peer that reports its failure INSTEAD
	// of replying leaves that wait unsatisfied, and the authenticator then sits
	// until its stale-handshake reaper rather than sending EAP-Failure. So the
	// reply goes out first and the cause is parked here.
	//
	// Written and read on the session's own goroutine, like state and identity
	// beside it.
	pendingErr error
}

type peerState uint8

const (
	peerStateIdentity peerState = iota
	peerStateMethod

	// peerStateMethodDone is the peer after its method conversation completed
	// SUCCESSFULLY and before the EAP-Success that concludes the exchange. It is
	// the only state in which an EAP-Success is answered, and the only one in
	// which an EAP-Failure is discarded, so the two obligations of RFC 3748
	// Section 4.2 that turn on "the method conversation has concluded" both read
	// this one value.
	//
	// Two sites reach it, and each is the moment its method indicates success to
	// the peer and the peer indicates success back:
	//
	//	handleMSCHAPv2Success  the Authenticator Response verified, and the
	//	                       acknowledgement RFC 2759 Section 5 owes going out
	//	handleTLSRequest       the TLS handshake complete and the MSK exported,
	//	                       with the peer's own final flight already sent
	//
	// The peer can still owe EAP-TLS fragments here, so a Request is dispatched
	// from this state exactly as it is from peerStateMethod (handleRequest).
	peerStateMethodDone

	peerStateDone
	peerStateFailed
)

// NewPeerSession creates an EAP peer session for the given method.
func NewPeerSession(method uint8, identity, password string) *PeerSession {
	return &PeerSession{
		identity: identity,
		password: password,
		method:   method,
		state:    peerStateIdentity,
		userName: stripDomain(identity),
	}
}

// NewPeerSessionTLS creates an EAP-TLS peer session with certificate material.
func NewPeerSessionTLS(identity string, cfg *PeerTLSConfig) *PeerSession {
	return &PeerSession{
		identity: identity,
		method:   TypeTLS,
		state:    peerStateIdentity,
		userName: stripDomain(identity),
		tlsCfg:   cfg,
	}
}

// Process handles an incoming EAP packet (Request or Success/Failure) from the authenticator
// and returns the peer's response. On EAP-Success, Done is true and MSK is set.
func (ps *PeerSession) Process(request *Packet) PeerResult {
	ps.rounds++
	if ps.rounds > maxEAPRounds {
		ps.state = peerStateFailed
		return PeerResult{Err: ErrTooManyRounds}
	}

	// RFC 3748 Section 4: "Since EAP only defines Codes 1-4, EAP packets with
	// other codes MUST be silently discarded by both authenticators and peers."
	// The discard comes before every other guard, including the parked cause
	// below, because a discarded packet is one the peer never read: it can decide
	// nothing about the session, not even that the session is over.
	if request.Code == 0 || request.Code > CodeFailure {
		return peerDiscard()
	}

	// A parked TLS failure outranks whatever arrives next. The reply the RFC asks
	// for has gone out, so this packet ends the exchange whatever it is, and the
	// cause the operator needs is the handshake's rather than the generic "the
	// authenticator sent Failure" that names only the symptom.
	if ps.pendingErr != nil {
		ps.state = peerStateFailed
		return PeerResult{Err: ps.pendingErr}
	}

	// A session the peer ended stays ended, whatever arrives next. RFC 2759
	// Section 5 makes a missing or incorrect authenticator response end the
	// session, and an EAP-Success is what a rogue authenticator sends after that
	// refusal: answering it would hand out the MSK the refusal denied.
	if ps.state == peerStateFailed {
		// RFC 3748 Section 4.2: "The peer MUST silently discard Success packets."
		// A discard rather than errSessionEnded, because the engine reads a
		// non-nil Err as a reason to kill the SA, and the session is already over
		// by the refusal that set this state. The MSK is denied either way; what
		// changes is that a rogue authenticator can no longer choose the moment
		// the SA dies by sending one packet after the refusal.
		return peerDiscard()
	}

	switch request.Code {
	case CodeSuccess:
		// RFC 3748 Section 4.2: "A peer EAP implementation receiving a Success or
		// Failure packet where sending one is not explicitly permitted MUST
		// silently discard it.  By default, an EAP peer MUST silently discard a
		// "canned" Success packet (a Success packet sent immediately upon
		// connection).  This ensures that a rogue authenticator will not be able to
		// bypass mutual authentication by sending a Success packet prior to
		// conclusion of the EAP method conversation."
		//
		// That conclusion is peerStateMethodDone, and nothing else here reaches the
		// MSK. Before it ps.msk is either all zero or a key no method has yet
		// authenticated the far end for, and the caller turns a Done result
		// straight into the IKEv2 AUTH payload (handleEAPResponse,
		// internal/component/ike/engine/fsm.go), so an authenticator that says
		// "Success" first and authenticates never would be believed.
		if ps.state != peerStateMethodDone {
			return peerDiscard()
		}
		ps.state = peerStateDone
		msk := ps.msk
		return PeerResult{Done: true, MSK: msk}

	case CodeFailure:
		// RFC 3748 Section 4.2: "On the peer, after success result indications have
		// been exchanged by both sides, a Failure packet MUST be silently
		// discarded."
		//
		// peerStateMethodDone is that state and only that state: the authenticator
		// indicated success through the method, and the peer indicated success back
		// (see the state's own comment). An EAP-Failure arriving there contradicts
		// an authentication both ends already agreed on, so it is dropped and the
		// peer keeps waiting for the EAP-Success. RFC 3748 Section 4.2 adds that
		// the peer "MAY, in the event that an EAP Success is not received, conclude
		// that the EAP Success packet was lost"; ze does not take that liberty and
		// waits for the packet.
		//
		// No path through the engine reaches this today, because ze's own
		// authenticator sends Success rather than Failure once the method
		// succeeded. The guard exists for the authenticator that does not: the
		// packet is unauthenticated, so any party on the path can forge one.
		if ps.state == peerStateMethodDone {
			return peerDiscard()
		}
		ps.state = peerStateFailed
		return PeerResult{Err: ErrEAPFailure}

	case CodeRequest:
		return ps.handleRequest(request)

	default:
		return PeerResult{Err: fmt.Errorf("eap: unexpected code %d", request.Code)}
	}
}

// peerDiscard is the result of a silent discard: the packet is dropped and the
// session waits for the next one.
//
// Silence is owed to the AUTHENTICATOR, never to the operator. No EAP-Response
// goes out, no error ends the exchange, and handleEAPResponse
// (internal/component/ike/engine/fsm.go) leaves the SA in StateEAPInProgress
// with its retransmit timer armed, which is the state a peer that never
// received the packet would be in. A discard that reported an error instead
// would end the exchange, and an authenticator that can end an exchange with
// one forged packet is the denial of service this guard would have introduced.
//
// The session is not left open forever: maxEAPRounds counts a discarded packet
// like any other, so a flood of them ends in ErrTooManyRounds.
func peerDiscard() PeerResult { return PeerResult{Discarded: true} }

// Succeeded reports whether the exchange completed successfully.
func (ps *PeerSession) Succeeded() bool { return ps.state == peerStateDone }

// Close releases every resource the peer session holds.
//
// The caller MUST call it once the exchange has ended, for ANY reason: an
// EAP-Success, an EAP-Failure, a refused method, or an authenticator that
// stopped answering. The EAP-TLS client runs its TLS engine on a goroutine
// parked in eapTLSTransport.Read, and only closing the transport releases it,
// so an exchange that ends without this call strands that goroutine together
// with the tls.Conn and the handshake secrets it holds.
//
// The guard reads tlsStarted rather than the pointer, because startTLSClient
// runs on the engine's dispatch goroutine while this runs on the session's
// owner goroutine. startTLSClient assigns tlsTransport BEFORE it stores that
// flag, so a load that sees true also sees the assignment.
//
// Idempotent, and safe on an MS-CHAPv2 session or on one whose TLS client never
// started.
func (ps *PeerSession) Close() {
	if ps == nil || !ps.tlsStarted.Load() {
		return
	}
	ps.tlsTransport.shutdown()
}

func (ps *PeerSession) handleRequest(req *Packet) PeerResult {
	switch ps.state {
	case peerStateIdentity:
		if req.Type == TypeIdentity {
			ps.state = peerStateMethod
			return PeerResult{
				Response: &Packet{
					Code:       CodeResponse,
					Identifier: req.Identifier,
					Type:       TypeIdentity,
					TypeData:   []byte(ps.identity),
				},
			}
		}
		if req.Type == ps.method {
			ps.state = peerStateMethod
			return ps.handleMethodRequest(req)
		}
		return PeerResult{Err: fmt.Errorf("eap: unexpected type %d in identity state", req.Type)}

	// A Request is dispatched the same way once the method conversation has
	// concluded, because the peer can still owe packets there: an EAP-TLS final
	// flight longer than one fragment is answered with a fragment ACK, and that
	// ACK is an EAP-Request (handleTLSRequest).
	case peerStateMethod, peerStateMethodDone:
		return ps.handleMethodRequest(req)

	default:
		return PeerResult{Err: fmt.Errorf("eap: request in terminal state")}
	}
}

func (ps *PeerSession) handleMethodRequest(req *Packet) PeerResult {
	switch ps.method {
	case TypeMSCHAPv2:
		return ps.handleMSCHAPv2Request(req)
	case TypeTLS:
		return ps.handleTLSRequest(req)
	default:
		return PeerResult{Err: fmt.Errorf("%w: peer type %d", ErrUnsupportedMethod, ps.method)}
	}
}

func (ps *PeerSession) handleMSCHAPv2Request(req *Packet) PeerResult {
	// RFC 3748 Section 2.1: "The peer MUST silently discard a Request of a Type
	// other than the one under way." One method runs per conversation, so a
	// Request naming another Type answers no state this peer holds. An error here
	// killed the SA, which handed a packet nobody authenticated the power to end
	// the exchange.
	if req.Type != TypeMSCHAPv2 {
		return peerDiscard()
	}
	td := req.TypeData
	if len(td) < 4 {
		return PeerResult{Err: fmt.Errorf("eap-mschapv2: request too short")}
	}

	opCode := td[0]
	switch opCode {
	case mschapv2OpChallenge:
		return ps.handleMSCHAPv2Challenge(req.Identifier, td)
	case mschapv2OpSuccess:
		return ps.handleMSCHAPv2Success(req.Identifier, td)
	case mschapv2OpFailure:
		return ps.handleMSCHAPv2Failure(req.Identifier, td)
	default:
		return PeerResult{Err: fmt.Errorf("eap-mschapv2: unexpected opcode %d", opCode)}
	}
}

func (ps *PeerSession) handleMSCHAPv2Challenge(identifier uint8, td []byte) PeerResult {
	if len(td) < 21 {
		return PeerResult{Err: fmt.Errorf("eap-mschapv2: challenge too short: %d", len(td))}
	}
	valueSize := td[4]
	if valueSize != 16 {
		return PeerResult{Err: fmt.Errorf("eap-mschapv2: unexpected challenge size %d", valueSize)}
	}
	var authChallenge [16]byte
	copy(authChallenge[:], td[5:21])

	if _, err := rand.Read(ps.peerChallenge[:]); err != nil {
		return PeerResult{Err: fmt.Errorf("eap-mschapv2: generate peer challenge: %w", err)}
	}

	ntResponse := GenerateNTResponse(authChallenge, ps.peerChallenge, ps.userName, ps.password)
	ps.authChallenge = authChallenge
	ps.ntResponse = ntResponse
	ps.msk = DeriveMSK(ps.password, ntResponse)

	// MS-CHAPv2 Response: OpCode(1) + MS-ID(1) + MS-Length(2) + ValueSize(1) + Response(49) + Name.
	msID := td[1]
	nameBytes := []byte(ps.identity)
	msLen := 5 + 49 + len(nameBytes)
	resp := make([]byte, msLen)
	resp[0] = mschapv2OpResponse
	resp[1] = msID
	resp[2] = byte(msLen >> 8)
	resp[3] = byte(msLen)
	resp[4] = 49
	copy(resp[5:21], ps.peerChallenge[:])
	copy(resp[29:53], ntResponse[:])
	copy(resp[54:], nameBytes)

	return PeerResult{
		Response: &Packet{
			Code:       CodeResponse,
			Identifier: identifier,
			Type:       TypeMSCHAPv2,
			TypeData:   resp,
		},
	}
}

// authenticatorResponseDigits is the size of the S= value in the Success
// Message: a 20-octet number written as 40 hexadecimal digits.
const authenticatorResponseDigits = 40

// handleMSCHAPv2Success verifies that the authenticator knows the password
// before it acknowledges the Success packet, which is the second half of
// MS-CHAPv2 mutual authentication.
//
// RFC 2759 Section 5: "The authenticating peer MUST verify the authenticator
// response when a Success packet is received.  The method for verifying the
// authenticator is described in section 8.8, below.  If the authenticator
// response is either missing or incorrect, the peer MUST end the session."
//
// Ending the session has two parts here. The state goes to peerStateFailed, so
// no later packet is answered, and the error reaches startEAPExchange
// (internal/component/ike/engine/fsm.go), which puts the IKE SA in StateDead.
func (ps *PeerSession) handleMSCHAPv2Success(identifier uint8, td []byte) PeerResult {
	received, err := parseAuthenticatorResponse(td)
	if err != nil {
		ps.state = peerStateFailed
		return PeerResult{Err: err}
	}

	expected := GenerateAuthenticatorResponse(ps.password, ps.ntResponse, ps.peerChallenge, ps.authChallenge, ps.userName)
	if !constantTimeEqual(expected[:], received[:]) {
		ps.state = peerStateFailed
		return PeerResult{Err: errors.New("eap-mschapv2: the authenticator response does not match the expected value, so the authenticator does not know the password")}
	}

	// The method conversation has concluded successfully: the authenticator
	// indicated success with this packet, and the acknowledgement below is the
	// peer's own success result indication. RFC 3748 Section 4.2 turns two
	// obligations on that fact, and Process reads this state for both: an
	// EAP-Success is answered only from here, and an EAP-Failure is discarded only
	// from here.
	ps.state = peerStateMethodDone

	// RFC 2759: peer acknowledges Success with an MSCHAPv2 Success packet (OpCode=3).
	msID := td[1]
	ack := []byte{mschapv2OpSuccess, msID, 0, 4}
	return PeerResult{
		Response: &Packet{
			Code:       CodeResponse,
			Identifier: identifier,
			Type:       TypeMSCHAPv2,
			TypeData:   ack,
		},
	}
}

// parseAuthenticatorResponse reads the 20-octet authenticator response out of
// the Message field of an MS-CHAPv2 Success packet.
//
// RFC 2759 Section 5 gives the Message field as "S=<auth_string> M=<message>",
// where "The <auth_string> quantity is a 20 octet number encoded in ASCII as 40
// hexadecimal digits."  The Message starts after the four-octet header, and it
// carries no length of its own: it runs to the end of the packet.
//
//	 0        1        2        3        4                        len(td)
//	+--------+--------+--------+--------+-------------------------+
//	| OpCode | MS-ID  |    MS-Length    | S=<40 hex> M=<message>  |
//	+--------+--------+--------+--------+-------------------------+
//
// Every shape that carries no readable authenticator response is an error, the
// absent Message included. RFC 2759 Section 5 treats a missing response and an
// incorrect one the same way, so neither may reach the acknowledgement.
func parseAuthenticatorResponse(td []byte) ([20]byte, error) {
	var response [20]byte
	if len(td) <= 4 {
		return response, errors.New("eap-mschapv2: the Success packet carries no Message, so it carries no authenticator response")
	}

	message := string(td[4:])
	if !strings.HasPrefix(message, "S=") {
		return response, fmt.Errorf("eap-mschapv2: the Success Message carries no S= authenticator response: %q", message)
	}

	end := 2 + authenticatorResponseDigits
	if len(message) < end {
		return response, fmt.Errorf("eap-mschapv2: the authenticator response is %d characters, want %d hexadecimal digits", len(message)-2, authenticatorResponseDigits)
	}

	if _, err := hex.Decode(response[:], []byte(message[2:end])); err != nil {
		return response, fmt.Errorf("eap-mschapv2: the authenticator response is not hexadecimal: %w", err)
	}
	return response, nil
}

// handleMSCHAPv2Failure answers the authenticator's Failure packet and ends the
// conversation behind it.
//
// RFC 2759 Section 6: "The Failure packet is identical in format to the standard
// CHAP Failure packet.  There is, however, formatted text stored in the Message
// field which, contrary to the standard CHAP rules, does affect the operation of
// the protocol."
//
// ACKNOWLEDGE NOW, REPORT ON THE ROUND AFTER. The authenticator still owes an
// EAP-Failure: RFC 3748 Section 4.2 says that after a failure result indication,
// "regardless of the response from the peer, it MUST subsequently send a Failure
// packet", and RFC 7296 Section 2.16 gives each IKE_AUTH message one EAP payload,
// so that packet needs a round of its own. A peer that answered nothing here
// would leave the authenticator with no round to send it in.
//
// The cause is parked rather than returned, for the reason pendingErr exists:
// PeerResult carries a response and an error, and the IKE engine reads the error
// first (handleEAPResponse, internal/component/ike/engine/fsm.go), so a cause
// returned beside the acknowledgement discards the acknowledgement. Parking it
// also ends the exchange whatever arrives next, which is what RFC 3748 Section
// 4.2 asks of a peer whose method completed unsuccessfully: "The peer MUST
// silently discard Success packets".
//
// Ze does not retry behind the R= flag. Taking a retry needs new credentials, and
// RFC 2759 Section 6 says where they come from: the authenticator expects "the
// peer to prompt the user for new credentials and resubmit the response". An
// IKEv2 EAP exchange in a router has no user at the keyboard, and Section 6
// obliges no peer to retry.
func (ps *PeerSession) handleMSCHAPv2Failure(identifier uint8, td []byte) PeerResult {
	failure, err := parseMSCHAPv2Failure(td)
	if err != nil {
		// A Failure packet Ze cannot read is still a refusal, and it is one with
		// nothing to acknowledge: the conversation ends here rather than sending
		// an acknowledgement of a packet whose contents were not understood.
		ps.state = peerStateFailed
		return PeerResult{Err: err}
	}

	// ErrEAPFailure stays the identity of "the authenticator refused us", which
	// is what the IKE engine and every test of that path read. The MS-CHAPv2
	// reason rides with it, because the EAP-Failure to come carries no field for
	// one (RFC 3748 Section 4.2: "Success and Failure packets MUST NOT contain
	// additional data").
	ps.pendingErr = fmt.Errorf("%w: %w", ErrEAPFailure, failure)

	// RFC 2759 Section 6 over the draft-kamath EAP encapsulation: the peer
	// answers the Failure packet with the OpCode alone, framed by the same
	// four-octet header the Success acknowledgement uses.
	// RFC 3748 Section 4.1: "The Identifier field of the Response MUST match
	// that of the currently outstanding Request."
	ack := []byte{mschapv2OpFailure, td[1], 0, 4}
	return PeerResult{
		Response: &Packet{
			Code:       CodeResponse,
			Identifier: identifier,
			Type:       TypeMSCHAPv2,
			TypeData:   ack,
		},
	}
}

// parseMSCHAPv2Failure reads the Message field of an MS-CHAPv2 Failure packet.
//
// RFC 2759 Section 6 gives the format as
// "E=eeeeeeeeee R=r C=cccccccccccccccccccccccccccccccc V=vvvvvvvvvv M=<msg>".
// The Message starts after the four-octet header and carries no length of its
// own: it runs to the end of the packet, and M= runs to the end of the Message.
//
//	 0        1        2        3        4                        len(td)
//	+--------+--------+--------+--------+-------------------------+
//	| OpCode | MS-ID  |    MS-Length    | E= R= C= V= M=<message> |
//	+--------+--------+--------+--------+-------------------------+
//
// Two fields are checked, and they are the two the RFC states an obligation
// about. E= carries the error code the caller logs. C= "MUST be exactly 32
// octets long and MUST be present", so a Message without one is named as the
// malformed packet it is rather than reported as an ordinary refusal. R= and V=
// are read past: R= offers a retry Ze does not take, and V= names a
// password-changing protocol Ze does not speak.
//
// Case is not part of the C= mandate, so a lowercase challenge is accepted here
// while sendFailure writes uppercase (eap_mschapv2.go).
func parseMSCHAPv2Failure(td []byte) (*mschapv2FailureError, error) {
	if len(td) <= 4 {
		return nil, fmt.Errorf("%w: the Failure packet carries no Message", errFailureChallenge)
	}
	message := string(td[4:])

	challenge, ok := mschapv2FailureField(message, "C=")
	if !ok {
		return nil, fmt.Errorf("%w: the Message carries no C= field: %q", errFailureChallenge, message)
	}
	if len(challenge) != mschapv2ChallengeDigits {
		return nil, fmt.Errorf("%w: the C= field is %d octets, want exactly %d", errFailureChallenge, len(challenge), mschapv2ChallengeDigits)
	}
	if _, err := hex.DecodeString(challenge); err != nil {
		return nil, fmt.Errorf("%w: the C= field is not hexadecimal: %w", errFailureChallenge, err)
	}

	digits, ok := mschapv2FailureField(message, "E=")
	if !ok {
		return nil, fmt.Errorf("eap-mschapv2: the Failure Message carries no E= error code: %q", message)
	}
	code, err := strconv.Atoi(digits)
	if err != nil {
		return nil, fmt.Errorf("eap-mschapv2: the E= error code %q is not a decimal number", digits)
	}

	// M= is the last field, and RFC 2759 Section 6 makes it "human-readable text
	// in the appropriate charset and language", so it runs to the end of the
	// Message rather than to the next space.
	text := ""
	if _, after, found := strings.Cut(message, "M="); found {
		text = after
	}

	return &mschapv2FailureError{code: code, message: text}, nil
}

// mschapv2FailureField returns the value of one space-delimited field of a
// Failure Message, and reports whether the field is present.
func mschapv2FailureField(message, prefix string) (string, bool) {
	for field := range strings.SplitSeq(message, " ") {
		if value, found := strings.CutPrefix(field, prefix); found {
			return value, true
		}
	}
	return "", false
}

// errFailureChallenge reports an MS-CHAPv2 Failure packet whose C= field is
// absent or is not 32 hexadecimal digits.
//
// RFC 2759 Section 6, on the C= field: "This field MUST be exactly 32 octets
// long and MUST be present." A Failure packet that breaks either half of that
// mandate is refused with this error rather than read as an ordinary refusal.
var errFailureChallenge = errors.New("eap-mschapv2: the Failure packet carries no conformant C= challenge")

// mschapv2FailureError is the authenticator's refusal, read as an error so the
// caller logs the code the authenticator sent rather than the bare fact that
// the exchange failed.
type mschapv2FailureError struct {
	code    int
	message string
}

func (e *mschapv2FailureError) Error() string {
	return "eap-mschapv2: the authenticator refused the credentials with error code " + strconv.Itoa(e.code) + ": " + e.message
}

// handleTLSRequest processes EAP-TLS requests from the authenticator.
// RFC 5216 Section 2.1.5: handles Start, fragmented data, and fragment ACKs.
func (ps *PeerSession) handleTLSRequest(req *Packet) PeerResult {
	if req.Type != TypeTLS {
		return PeerResult{Err: fmt.Errorf("eap-tls: expected type %d, got %d", TypeTLS, req.Type)}
	}

	// If we're waiting for a fragment ACK, the server's response is an ACK. Send next fragment.
	if ps.waitFragAck {
		ps.waitFragAck = false
		return PeerResult{
			Response: &Packet{
				Code:       CodeResponse,
				Identifier: req.Identifier,
				Type:       TypeTLS,
				TypeData:   ps.nextFragment(),
			},
		}
	}

	td := req.TypeData
	if len(td) == 0 {
		return PeerResult{Err: fmt.Errorf("eap-tls: empty request")}
	}
	flags := td[0]

	// EAP-TLS Start: initialize the TLS client handshake.
	if flags&eapTLSFlagS != 0 {
		if err := ps.startTLSClient(); err != nil {
			return PeerResult{Err: err}
		}
		return ps.readAndSendTLS(req.Identifier)
	}

	// Reassemble inbound TLS data.
	if err := ps.reassemble(td); err != nil {
		return PeerResult{Err: err}
	}

	// If M flag set, more fragments coming. Send fragment ACK (empty EAP-TLS).
	if flags&eapTLSFlagM != 0 {
		return PeerResult{
			Response: &Packet{
				Code:       CodeResponse,
				Identifier: req.Identifier,
				Type:       TypeTLS,
				TypeData:   []byte{0},
			},
		}
	}

	// The authenticator cleared the M flag, so it has stopped sending. Refuse a
	// message shorter than the length it declared rather than passing the short
	// buffer to crypto/tls, which reports it as the opaque "local error: tls:
	// error decoding message" with no mention of the missing octets.
	if !ps.reassemblyComplete() {
		return PeerResult{Err: fmt.Errorf("eap-tls: authenticator ended a TLS message after %d of %d declared bytes", len(ps.inBuf), ps.inExpected)}
	}

	// All fragments received. Feed to TLS engine.
	//
	// A refusal here means the authenticator kept sending while this side's TLS
	// engine had stopped reading. The exchange cannot recover, so it ends with
	// that error rather than queueing more (ai/rules/evidence.md).
	if data := ps.drainReassembled(); len(data) > 0 {
		if err := ps.tlsTransport.feedPeerData(data); err != nil {
			ps.state = peerStateFailed
			return PeerResult{Err: err}
		}
	}

	result := ps.readAndSendTLS(req.Identifier)

	// Capture the MSK once the handshake has completed, so it is ready when the
	// authenticator sends EAP-Success.
	//
	// This is read AFTER readAndSendTLS, never before: the data just fed is what
	// completes the handshake, and readAndSendTLS waits for the engine to settle,
	// so the round that finishes the handshake is the round whose wait observes
	// it. Reading tlsDone before the wait sees the previous round's value, and
	// the MSK is then still zero when EAP-Success arrives.
	//
	// Completion does NOT end the EAP exchange here: with TLS 1.3 the peer
	// produces its final flight (client Certificate/CertificateVerify/Finished)
	// at the same instant the handshake completes, and readAndSendTLS above has
	// just sent that flight. The EAP layer only concludes on the EAP-Success the
	// authenticator sends after it has verified the flight (handled in Process,
	// CodeSuccess). Returning Done here would drop the unsent flight and stall
	// the authenticator forever.
	if ps.tlsDone.Load() {
		msk, err := ps.deriveTLSMSK()
		if err != nil {
			// REPLY FIRST, REPORT ON THE NEXT ROUND. readAndSendTLS has already
			// built the EAP-Response for this round, and RFC 5216 Section 2.1.3
			// makes the authenticator wait for exactly that packet before it may
			// send EAP-Failure. Returning the error here instead discards the
			// response (the engine drops PeerResult.Response whenever Err is set),
			// so the authenticator waits out its stale-handshake reaper rather than
			// terminating in the round it would otherwise terminate in.
			//
			// Reachable whenever the authenticator's fatal alert lands before this
			// side's handshake completes: a TLS 1.2 client-certificate rejection, or
			// a rejection at ClientHello in any version.
			if result.Err == nil && result.Response != nil {
				ps.pendingErr = err
				return result
			}
			ps.state = peerStateFailed
			return PeerResult{Err: err}
		}
		ps.msk = msk

		// The method conversation has concluded successfully. Both ends have shown
		// the other a Finished the other verified, and readAndSendTLS above has
		// already sent the peer's half. RFC 3748 Section 4.2 lets an EAP-Success be
		// answered only from here (Process), and this is the assignment that puts a
		// real key behind the Done result it produces.
		ps.state = peerStateMethodDone
	}

	return result
}

// verifyServerChain returns a tls.Config.VerifyPeerCertificate callback that
// validates the authenticator's presented certificate chain against roots
// without any DNS/hostname check (EAP-TLS has no server hostname).
// RFC 5216 Section 5.3: the peer validates the authenticator's certificate.
func verifyServerChain(roots *x509.CertPool) func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return fmt.Errorf("eap-tls: authenticator presented no certificate")
		}
		certs := make([]*x509.Certificate, 0, len(rawCerts))
		for _, raw := range rawCerts {
			c, err := x509.ParseCertificate(raw)
			if err != nil {
				return fmt.Errorf("eap-tls: parse authenticator certificate: %w", err)
			}
			certs = append(certs, c)
		}
		opts := x509.VerifyOptions{Roots: roots, Intermediates: x509.NewCertPool()}
		for _, c := range certs[1:] {
			opts.Intermediates.AddCert(c)
		}
		if _, err := certs[0].Verify(opts); err != nil {
			return fmt.Errorf("eap-tls: authenticator certificate chain verification failed: %w", err)
		}
		return nil
	}
}

func (ps *PeerSession) startTLSClient() error {
	if ps.tlsCfg == nil {
		return fmt.Errorf("eap-tls: no TLS config")
	}

	cert, err := tls.X509KeyPair(ps.tlsCfg.CertPEM, ps.tlsCfg.KeyPEM)
	if err != nil {
		return fmt.Errorf("eap-tls: load client cert: %w", err)
	}

	// RFC 5216 Section 5.3: "Both sides MUST perform certificate path validation."
	// The configured trust anchor is the peer's only means of doing so, so a
	// session with none is refused rather than started: there is no weaker but
	// still conformant mode to fall back to, and a peer that skips validation
	// authenticates nothing.
	if len(ps.tlsCfg.CACertPEM) == 0 {
		return errNoPeerTrustAnchor
	}
	rootCAs := x509.NewCertPool()
	if !rootCAs.AppendCertsFromPEM(ps.tlsCfg.CACertPEM) {
		return fmt.Errorf("eap-tls: failed to parse peer CA certificate")
	}

	// EAP-TLS carries no server hostname, so Go's default (SNI/hostname-based)
	// verification cannot be used: with a CA set but no ServerName, tls.Client
	// rejects the config outright ("either ServerName or InsecureSkipVerify must
	// be specified") and never sends a ClientHello. InsecureSkipVerify disables
	// only that default hostname check; the chain itself is always verified
	// against the trust anchor in VerifyPeerCertificate, which the guard above
	// guarantees is present.
	tlsCfg := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true, //nolint:gosec // EAP has no server hostname; the chain is always verified in VerifyPeerCertificate
		MinVersion:         tls.VersionTLS12,
		RootCAs:            rootCAs,
		// Go does not call VerifyPeerCertificate for a resumed session, and
		// that callback is the only certificate check this config has. A
		// client with no ClientSessionCache resumes nothing already; refusing
		// tickets here states the requirement instead of leaving it to that
		// default, so adding a cache later cannot skip the chain check.
		SessionTicketsDisabled: true,
		VerifyPeerCertificate:  verifyServerChain(rootCAs),
	}

	ps.tlsTransport = newEAPTLSTransport()
	ps.tlsConn = tls.Client(ps.tlsTransport, tlsCfg)
	ps.tlsStarted.Store(true)

	go func() {
		// Keep the handshake error. Discarding it left every TLS failure
		// (rejected certificate, unsupported version, bad record) reported as
		// the same downstream symptom, "no MSK exists", which names the
		// consequence and not the cause.
		if err := ps.tlsConn.HandshakeContext(context.Background()); err != nil {
			ps.tlsErr.Store(&err)
		}
		// Publish the outcome BEFORE the wakeup: handshakeFinished releases a
		// waiter in readAndSendTLS, and handleTLSRequest reads tlsDone straight
		// after that wait to decide whether to capture the MSK.
		ps.tlsDone.Store(true)
		ps.tlsTransport.handshakeFinished()
	}()

	return nil
}

// readAndSendTLS reads TLS engine output and sends it (possibly fragmented).
//
// It WAITS for the engine to settle. A snapshot taken before the engine writes
// its flight yields an empty slice. That empty slice reads as the bare fragment
// ACK below. RFC 5216 Section 2.1.5 permits that ACK only in answer to a message
// with the M flag. An authenticator refuses it, and the method fails before the
// ClientHello crosses.
//
// The wait is BOUNDED by eapTLSSettleBackstop, so it also returns empty for an
// engine that never answered. That case is reported, never sent.
func (ps *PeerSession) readAndSendTLS(identifier uint8) PeerResult {
	clientData := ps.tlsTransport.waitServerData()

	if len(clientData) == 0 {
		// An empty answer is owed in two states, and they are the same shape from
		// here: the engine is finished and has nothing left to send.
		//
		// RFC 5216 Section 2.1.3, on TLS 1.2: "If the EAP server authenticates
		// successfully, the peer MUST send an EAP-Response packet of
		// EAP-Type=EAP-TLS, and no data." The closing round reaches here with the
		// engine finished and its output drained, and the authenticator waits for
		// that packet before EAP-Success.
		//
		// RFC 9190 Section 2.5 step 4, on TLS 1.3: the authenticator's last
		// EAP-Request carries the protected success result indication, an
		// encrypted TLS record holding application data 0x00, and the peer
		// answers it with "an EAP-Response of EAP-Type=EAP-TLS and no data".
		// THAT IS THE SAME PACKET THIS BRANCH ALREADY BUILDS, and it is right by
		// arithmetic rather than by design: handleTLSRequest feeds the record to
		// the transport, no goroutine is left reading the tls.Conn once
		// HandshakeContext has returned, so the engine produces nothing and
		// tlsDone is already set. This peer therefore ANSWERS the indication
		// correctly without ever DECRYPTING it, and so cannot tell an
		// authenticator that sent one from an authenticator that did not.
		// Requiring it (spec-ipsec-rfc9190 AC-2) is a separate change and needs a
		// reader on the connection; the published RFC states no peer-side
		// obligation, and errata 7577, which proposes one, is still Reported
		// rather than Verified (rfc/short/rfc9190.md).
		//
		// A handshake that is still running means the engine settled and wrote
		// nothing. The bounded wait's backstop fired on a wedged client, or the
		// transport closed under it. The flags octet would then answer a Start, or
		// a data message, with a fragment ACK. That is the form that fails the
		// method, so the stall is reported by name. An empty buffer alone cannot
		// tell "finished with nothing left" from "never produced anything"
		// (ai/rules/evidence.md).
		if !ps.tlsDone.Load() {
			ps.state = peerStateFailed
			return PeerResult{Err: errTLSClientStalled}
		}
		return PeerResult{
			Response: &Packet{
				Code:       CodeResponse,
				Identifier: identifier,
				Type:       TypeTLS,
				TypeData:   []byte{0},
			},
		}
	}

	ps.startSending(clientData)
	return PeerResult{
		Response: &Packet{
			Code:       CodeResponse,
			Identifier: identifier,
			Type:       TypeTLS,
			TypeData:   ps.nextFragment(),
		},
	}
}

// deriveTLSMSK derives the peer's EAP-TLS MSK from the completed TLS connection.
//
// tlsDone is also set when the TLS handshake FAILED (e.g. the authenticator's
// certificate was rejected), and ExportKeyingMaterial panics on a connection
// whose handshake did not complete, so exportEAPTLSMSK guards on it and reports
// an error rather than returning a zero MSK that reads as a real key.
func (ps *PeerSession) deriveTLSMSK() ([64]byte, error) {
	if ps.tlsConn == nil {
		return [64]byte{}, errors.New("eap-tls: no TLS connection to derive the MSK from")
	}
	// Report the handshake's own error when it has one. It names the cause; the
	// missing MSK is only the consequence.
	if err := ps.tlsErr.Load(); err != nil {
		return [64]byte{}, fmt.Errorf("eap-tls: TLS handshake failed: %w", *err)
	}
	return exportEAPTLSMSK(ps.tlsConn.ConnectionState())
}
