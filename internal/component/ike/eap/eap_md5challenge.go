// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP MD5-Challenge method handler
// RFC: rfc/short/rfc3748.md -- Section 5.4: MD5-Challenge (type 4)
// RFC: rfc/short/rfc1994.md -- PPP CHAP: the challenge, the response and their fields
// Related: peer.go -- handleMD5ChallengeRequest, the peer half of this exchange
//
// MD5-Challenge is CHAP carried inside EAP. RFC 3748 Section 5.4: "The
// MD5-Challenge Type is analogous to the PPP CHAP protocol [RFC1994] (with MD5
// as the specified algorithm)." The authenticator sends a challenge, the peer
// answers with MD5 over the Identifier, the shared secret and the challenge, and
// the authenticator compares that value against its own computation.
//
// The method derives NO key and protects nothing. RFC 3748 Section 5.4 Security
// Claims: "Mutual authentication:     No", "Integrity protection:      No",
// "Replay protection:         No", "Key derivation:            No". So a
// successful exchange sets no MSK, DerivesKey answers false, and RFC 7296
// Section 2.16 obliges the IKEv2 carrier to compute its AUTH payloads from SK_pi
// and SK_pr instead. Nothing here may be read as authenticating the
// AUTHENTICATOR to the peer: only the peer is authenticated.

package eap

import (
	"crypto/md5" //nolint:gosec // RFC 3748 Section 5.4 prescribes the CHAP response of RFC 1994 with MD5 as the specified algorithm; no wire-compatible alternative exists.
	"crypto/rand"
	"crypto/subtle"
	"fmt"
)

// md5ChallengeValueSize is the length of both values this method handles: the
// challenge it issues, and the response MD5 produces.
//
// RFC 1994 Section 4.1 sizes the response: "The length of the Response Value
// depends upon the hash algorithm used (16 octets for MD5)." The challenge has
// no mandated length, because "The Challenge Value is a variable stream of
// octets", so ze issues 16 of them, which is the size Section 2.3 prefers for a
// secret under MD5 and the size the response itself has.
const md5ChallengeValueSize = 16

// md5ChallengeName is the Name field ze writes into its Request.
//
// RFC 1994 Section 4.1: "The Name field is one or more octets representing the
// identification of the system transmitting the packet." It indexes a secret in
// an authenticator that holds many. Ze holds the one secret the operator
// configured, so the field identifies rather than selects, and it carries the
// same value mschapv2Method.Start writes.
const md5ChallengeName = "ze"

type md5ChallengeState uint8

const (
	md5ChallengeStateChallenge md5ChallengeState = iota
	md5ChallengeStateDone
)

// md5ChallengeMethod is the authenticator half of MD5-Challenge.
type md5ChallengeMethod struct {
	secret string

	// challenge is the Value this method issued, held from Start to Process
	// because the response is computed over it.
	challenge [md5ChallengeValueSize]byte

	// identifier is the EAP Identifier the Request went out with, and it is the
	// first octet of the hashed stream. RFC 1994 Section 4.1: "The Response
	// Identifier MUST be copied from the Identifier field of the Challenge which
	// caused the Response", so the value the Request carried is the value the
	// response was computed under.
	identifier uint8

	state md5ChallengeState
}

func newMD5ChallengeMethod(config MethodConfig) *md5ChallengeMethod {
	return &md5ChallengeMethod{
		secret: config.Password,
	}
}

func (m *md5ChallengeMethod) Type() uint8 { return TypeMD5Challenge }

// DerivesKey answers false. RFC 3748 Section 5.4 Security Claims: "Key
// derivation:            No". TypeDerivesKey holds the single declaration.
func (m *md5ChallengeMethod) DerivesKey() bool { return TypeDerivesKey(TypeMD5Challenge) }

// Close releases the method's resources. MD5-Challenge runs entirely inside
// Start and Process, so it holds no goroutine and no connection, and this does
// nothing. The method still declares it, because Method requires every
// implementation to answer the question rather than leave the caller to know
// which ones need it.
func (m *md5ChallengeMethod) Close() {}

// Start issues the MD5-Challenge Request.
//
// RFC 3748 Section 5.4: "The Request contains a "challenge" message to the
// peer." The Type-Data is the CHAP Challenge body of RFC 1994 Section 4.1:
//
//	 0        1                            1+Value-Size        len(TypeData)
//	+--------+----------------------------+-------------------+
//	| Value- |          Value             |       Name        |
//	|  Size  |  (Value-Size octets)       |  (rest of packet) |
//	+--------+----------------------------+-------------------+
//	    16     the 16-octet challenge        "ze"
//
// The Name has no terminator and no length octet of its own. RFC 1994 Section
// 4.1: "The Name should not be NUL or CR/LF terminated.  The size is determined
// from the Length field." DecodePacket sizes TypeData from the EAP Length field,
// so the reader on the other side gets that size and never scans for a byte.
//
// The challenge is fresh for each Request. RFC 1994 Section 2.3: "Each challenge
// value SHOULD be unique, since repetition of a challenge value in conjunction
// with the same secret would permit an attacker to reply with a previously
// intercepted response." Section 2.3 also asks for unpredictability, which is
// why the octets come from crypto/rand rather than from a counter.
//
// The Identifier is the one the exchange handed down, and it is not changed
// afterwards. RFC 3748 Section 5.4 differs from CHAP here and says so: "Note
// that the use of the Identifier field in the MD5-Challenge Type is different
// from that described in [RFC1994].  EAP allows for retransmission of
// MD5-Challenge Request packets, while [RFC1994] states that both the Identifier
// and Challenge fields MUST change each time a Challenge (the CHAP equivalent of
// the MD5-Challenge Request packet) is sent." A retransmission of this Request is
// therefore the same octets, and the response the peer computed over them stays
// valid.
func (m *md5ChallengeMethod) Start(identifier uint8) *Packet {
	// crypto/rand.Read "never returns an error, and always fills b entirely", and
	// it crashes the program irrecoverably rather than returning a short read. So
	// there is no failure to report, which matters because Start has no way to
	// report one and returning a nil Packet would leave the exchange silent.
	rand.Read(m.challenge[:]) //nolint:errcheck // crypto/rand.Read never returns an error, and Start has no error return to carry one.

	m.identifier = identifier
	m.state = md5ChallengeStateChallenge

	td := make([]byte, 1+md5ChallengeValueSize+len(md5ChallengeName))
	td[0] = md5ChallengeValueSize
	copy(td[1:], m.challenge[:])
	copy(td[1+md5ChallengeValueSize:], md5ChallengeName)

	return &Packet{
		Code:       CodeRequest,
		Identifier: identifier,
		Type:       TypeMD5Challenge,
		TypeData:   td,
	}
}

// Process verifies the peer's MD5-Challenge Response.
//
// The packet arrives unauthenticated from the network, so every length is read
// against the Type-Data actually present before it is used as an index. A
// malformed Response ends the exchange with an error and never a panic.
//
// RFC 1994 Section 4.2: "If the Value received in a Response is equal to the
// expected value, then the implementation MUST transmit a CHAP packet with the
// Code field set to 3 (Success).  If the Value received in a Response is not
// equal to the expected value, then the implementation MUST transmit a CHAP
// packet with the Code field set to 4 (Failure)." Inside EAP those two packets
// are the EAP-Success and the EAP-Failure that Session.handleMethod sends for a
// Done result and an Err result respectively, so this returns the verdict and
// composes neither.
//
// No MSK is ever set, on either arm. RFC 3748 Section 5.4 Security Claims: "Key
// derivation:            No".
func (m *md5ChallengeMethod) Process(response *Packet) MethodResult {
	if m.state != md5ChallengeStateChallenge {
		return MethodResult{Err: fmt.Errorf("eap-md5: a second Response arrived after the exchange finished")}
	}

	td := response.TypeData
	if len(td) == 0 {
		return MethodResult{Err: fmt.Errorf("eap-md5: the Response carries no Type-Data, so it has no Value-Size octet")}
	}

	// RFC 1994 Section 4.1: "The length of the Response Value depends upon the
	// hash algorithm used (16 octets for MD5)." MD5 has one output length, so a
	// Response claiming any other size did not compute the response this method
	// asked for.
	valueSize := int(td[0])
	if valueSize != md5ChallengeValueSize {
		return MethodResult{Err: fmt.Errorf("eap-md5: the Response declares Value-Size %d, and MD5 produces %d octets", valueSize, md5ChallengeValueSize)}
	}
	if len(td) < 1+valueSize {
		return MethodResult{Err: fmt.Errorf("eap-md5: the Response declares Value-Size %d but carries %d octets after it", valueSize, len(td)-1)}
	}

	// The Name follows the Value and runs to the end of the packet. Ze holds the
	// one secret the operator configured, so the Name identifies the peer rather
	// than selecting a secret, and Session.identity already carries the identity
	// the exchange authenticates. Nothing here reads it.
	expected := md5ChallengeResponse(m.identifier, m.secret, m.challenge[:])

	// Constant time, because the alternative leaks the expected value one octet
	// at a time to a peer that can retry: an early-exit compare answers faster
	// the sooner it disagrees, and the peer chooses every octet it sends.
	matched := subtle.ConstantTimeCompare(expected[:], td[1:1+valueSize]) == 1

	// The method is finished either way, so the credential goes now rather than
	// at a Close the caller might skip.
	m.state = md5ChallengeStateDone
	m.secret = ""

	if !matched {
		return MethodResult{Err: fmt.Errorf("%w: the MD5-Challenge Response does not match the shared secret", ErrMethodFailed)}
	}
	return MethodResult{Done: true}
}

// md5ChallengeResponse computes the CHAP Response Value both halves of this
// method need: the authenticator to check one, the peer to send one.
//
// RFC 1994 Section 4.1: "The Response Value is the one-way hash calculated over
// a stream of octets consisting of the Identifier, followed by (concatenated
// with) the "secret", followed by (concatenated with) the Challenge Value.  The
// length of the Response Value depends upon the hash algorithm used (16 octets
// for MD5)."
//
// The identifier is the EAP Identifier of the Request, which RFC 1994 Section
// 4.1 also makes the Response's: "The Response Identifier MUST be copied from
// the Identifier field of the Challenge which caused the Response."
//
// The stream is built in one stack-sized buffer for the usual case, because the
// three parts have to be hashed as one. A secret and a challenge that together
// exceed it fall back to the heap, which costs one allocation per exchange on a
// control-plane path. The bound on the challenge is the EAP packet: a Value-Size
// octet cannot name more than 255 octets.
//
// internal/component/l2tp/auth.go computes the same CHAP response for RFC 2661
// tunnel authentication, over a Message Type octet rather than an EAP
// Identifier. The two are separate components and neither imports the other.
func md5ChallengeResponse(identifier uint8, secret string, challenge []byte) [md5ChallengeValueSize]byte {
	total := 1 + len(secret) + len(challenge)

	var stack [128]byte
	var stream []byte
	if total <= len(stack) {
		stream = stack[:total]
	} else {
		stream = make([]byte, total)
	}

	stream[0] = identifier
	copy(stream[1:], secret)
	copy(stream[1+len(secret):], challenge)

	return md5.Sum(stream) //nolint:gosec // RFC 3748 Section 5.4 prescribes the CHAP response of RFC 1994 with MD5 as the specified algorithm.
}
