// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- AUTH payload of an EAP exchange
// RFC: rfc/short/rfc7296.md -- Section 2.15: AUTH = prf(prf(Shared Secret, "Key Pad for IKEv2"), signed octets)
// RFC: rfc/short/rfc7296.md -- Section 2.16: which secret an EAP exchange puts in that formula

package engine

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"slices"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/eap"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

var keyPadForIKEv2 = []byte("Key Pad for IKEv2")

// computeAuthFromSharedSecret computes an IKEv2 AUTH payload from a shared secret.
//
// RFC 7296 Section 2.15 gives ONE formula and one name for its key: "AUTH = prf(
// prf(Shared Secret, "Key Pad for IKEv2"), <InitiatorSignedOctets>)". It also says
// what that key may be: "The shared secret can be variable length."
//
// Section 2.15 then names the octets an EAP exchange substitutes for it: "If the
// EAP method is key-generating, substitute master session key (MSK) for the shared
// secret in the computation.  For non-key-generating methods, substitute SK_pi and
// SK_pr, respectively, for the shared secret in the two AUTH computations."
//
// Three secrets reach this formula, and they differ in nothing else:
//
//   - the configured pre-shared key (computePSKAuth, auth.go)
//   - the MSK of a key-deriving EAP method
//   - SK_pi or SK_pr for a method that derives none
//
// The secret is therefore an argument here. A copy of the formula for each would
// be three places for the pad string, the PRF order and the operand order to
// drift. Nothing would arbitrate a disagreement between them
// (ai/rules/no-layering.md).
//
// The secret is NOT cleared: SK_pi, SK_pr and the configured key all outlive this
// call. The intermediate prf output is this function's own, and is.
func computeAuthFromSharedSecret(prfID crypto.PRFID, secret, signedOctets []byte) ([]byte, error) {
	sk, err := crypto.PRF(prfID, secret, keyPadForIKEv2)
	if err != nil {
		return nil, err
	}
	auth, err := crypto.PRF(prfID, sk, signedOctets)
	clear(sk)
	if err != nil {
		return nil, err
	}
	return auth, nil
}

// verifyAuthFromSharedSecret verifies an AUTH payload against the value the shared
// secret produces over the same signed octets.
func verifyAuthFromSharedSecret(prfID crypto.PRFID, secret, signedOctets, receivedAuth []byte) error {
	expected, err := computeAuthFromSharedSecret(prfID, secret, signedOctets)
	if err != nil {
		return err
	}
	if !constantTimeEqualAuth(expected, receivedAuth) {
		return errAuthFailed
	}
	return nil
}

// constantTimeEqualAuth compares two authenticators in constant time, so a caller
// leaks nothing about how far the comparison got. crypto/subtle carries the
// primitive. Its ConstantTimeCompare answers 0 for two slices of different
// lengths, so no separate length test is needed.
func constantTimeEqualAuth(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

// natHashEqual compares NAT detection hashes.
// RFC 7296 Section 2.23: SHA-1 hashes are 20 bytes.
func natHashEqual(a, b []byte) bool {
	return constantTimeEqualAuth(a, b)
}

// errNoReplyDestination reports that a response had no socket or no address to go to.
var errNoReplyDestination = errors.New("ike: reply has no destination")

// sendReply answers one request on the socket it ARRIVED on, addressed to the source
// it came FROM.
//
// RFC 7296 Section 2.11 MUST (rfc/full/rfc7296.txt:2591-2593): an implementation
// "MUST respond to the address and port from which the request was received. It MUST specify the address and port at which the request was received as the source address and port in the response".
//
// RFC 7296 Section 2.23 states the same obligation. It also gives the reason. A NAT
// reads the port numbers of inbound packets to select the internal node.
//
// Three things this deliberately does NOT do. Each one was a defect.
//
//   - It does not consult sa.NATDetected. That flag records what ZE sees.
//   - It does not rewrite the destination port to 4500. A NAT rarely maps a peer
//     there, and discarding the observed port violates Section 2.11.
//   - It does not rebuild the address from remote.IP alone.
//
// The marker follows the ARRIVAL socket. The role is read from the transport, never
// compared against a port number. Under the ze.test.ike.port override neither socket
// carries a well-known port, so a comparison picks the wrong framing in every
// functional test (ai/rules/evidence.md).
func sendReply(tr *transport.UDPTransport, data []byte, remote *net.UDPAddr) error {
	if tr == nil || remote == nil {
		return errNoReplyDestination
	}
	if tr.IsNATT() {
		// RFC 3948 Section 2.2: IKE on port 4500 carries the four-octet non-ESP marker.
		data = transport.AddNonESPMarker(data)
	}
	return tr.Send(data, remote)
}

// eapMethodType answers the EAP method Type an operator's authentication mode
// selects, and whether that mode runs EAP at all.
//
// It is the ONE declaration of that mapping. The authenticator reads it in
// newEAPSession below, the peer reads it in startEAPExchange (fsm.go), and the
// adoption warning reads it in warnKeylessEAPModes. Each of those had its own
// switch until 2026-09-01. A mode added to two of the three was then a mode the
// third silently refused (ai/rules/principles.md).
//
// ipsec.IsEAPMode answers the same question one layer up, for config validation,
// and the two must name the same modes. TestRFC3748IKEv2EAPModesSelectAKeyDeriving
// Method is where they are compared: it hands every mode IsEAPMode names to
// newEAPSession and fails on a refusal.
func eapMethodType(mode ipsec.AuthMode) (uint8, bool) {
	switch mode {
	case ipsec.AuthEAPMD5:
		return eap.TypeMD5Challenge, true
	case ipsec.AuthEAPMSCHAPv2:
		return eap.TypeMSCHAPv2, true
	case ipsec.AuthEAPTLS:
		return eap.TypeTLS, true
	default:
		return 0, false
	}
}

// newEAPSession creates an EAP session for a peer configured with EAP authentication.
func newEAPSession(authMode ipsec.AuthMode, config eap.MethodConfig) (*eap.Session, error) {
	methodType, isEAP := eapMethodType(authMode)
	if !isEAP {
		return nil, fmt.Errorf("ike: auth mode %s is not an EAP method", authMode)
	}
	return eap.NewSession(methodType, config)
}

// warnKeylessEAPModes writes one line for each configured peer whose EAP method
// establishes no shared key.
//
// RFC 7296 Section 2.16 (rfc/full/rfc7296.txt:2958): "EAP methods that do not
// establish a shared key SHOULD NOT be used, as they are subject to a number of
// man-in-the-middle attacks". That is a SHOULD NOT about USE, so the mode is an
// operator's to choose, and the choice is one the operator is told about.
//
// It runs where the configuration is ADOPTED (applyIPsecConfig, register.go),
// once for each delivery, and not on the session path. A line for each handshake
// repeats a fact about the configuration at the rate the peer reconnects. That
// buries the log rather than informing it.
//
// The peers are named in sorted order, so two adoptions of one configuration
// write the same lines in the same order.
func warnKeylessEAPModes(cfg *ipsec.IPsecConfig, log *slog.Logger) {
	if cfg == nil {
		return
	}
	for _, name := range slices.Sorted(maps.Keys(cfg.Peers)) {
		warnKeylessEAPMode(name, cfg.Peers[name].Auth.Mode, log)
	}
	if cfg.RemoteAccess != nil {
		warnKeylessEAPMode("remote-access", cfg.RemoteAccess.Auth.Mode, log)
	}
}

// warnKeylessEAPMode writes the line for one configured mode. It writes nothing
// for a mode whose method derives a key, and nothing for a mode without EAP.
//
// The keyless test asks the method Type, through the eap package's single
// declaration of which Types derive a key. A list of modes held here would be a
// second declaration of that fact. It would disagree with the first the day a
// method is added (ai/rules/principles.md).
func warnKeylessEAPMode(peerName string, mode ipsec.AuthMode, log *slog.Logger) {
	methodType, isEAP := eapMethodType(mode)
	if !isEAP {
		return
	}
	if eap.TypeDerivesKey(methodType) {
		return
	}
	log.Warn("ike: this peer runs an EAP method that establishes no shared key. "+
		"The IKEv2 AUTH payloads are keyed by SK_pi and SK_pr instead of by an EAP MSK. "+
		"RFC 7296 Section 2.16: EAP methods that do not establish a shared key SHOULD NOT "+
		"be used, as they are subject to a number of man-in-the-middle attacks",
		"peer", peerName, "mode", mode.String())
}

// eapMethodFacts is what the AUTH construction asks the EAP exchange this SA ran.
//
// sa.EAPSession holds *eap.Session on the responder (the authenticator) and
// *eap.PeerSession on the initiator (the peer), typed any so that neither concrete
// type appears in the SA declaration (sa.go). Both answer these two questions, so
// one assertion serves both roles.
//
// The two compile-time checks below are the guard on that assertion. Without them a
// method renamed in the eap package would turn every EAP AUTH into the "no EAP
// exchange" error at run time, and nothing would report which of the two types stopped
// matching (ai/rules/principles.md).
type eapMethodFacts interface {
	// Succeeded reports whether the EAP exchange completed successfully.
	Succeeded() bool

	// DerivesKey reports whether the method fills the MSK on success.
	DerivesKey() bool
}

var (
	_ eapMethodFacts = (*eap.Session)(nil)
	_ eapMethodFacts = (*eap.PeerSession)(nil)
)

// eapAuthSecret returns the octets that key the AUTH payload of an EAP exchange for
// the party named by isInitiator: true for message 7, which the initiator sends, and
// false for message 8, which the responder sends.
//
// RFC 7296 Section 2.16 writes one sentence for each kind of method. "For EAP methods
// that create a shared key as a side effect of authentication, that shared key MUST be
// used by both the initiator and responder to generate AUTH payloads in messages 7 and
// 8 using the syntax for shared secrets specified in Section 2.15." And, for the case
// this function exists for: "If EAP methods that do not generate a shared key are used,
// the AUTH payloads in messages 7 and 8 MUST be generated using SK_pi and SK_pr,
// respectively."
//
// The paragraph before that second sentence says such methods "SHOULD NOT be used", not
// MUST NOT, and this is the RFC saying what an implementation does when one is.
//
// The METHOD is asked which sentence applies. Reading sa.EAPMSK to decide cannot answer
// the question, because an all-zero MSK is the same value for a method that derives
// none, for one whose derivation failed, and for a field nobody has set yet. Deciding
// on it reads a zero as a valid answer (ai/rules/principles.md).
//
// The exchange MUST have succeeded first, and that guard is load-bearing rather than
// tidy. SK_pi and SK_pr are derived from SKEYSEED, which anybody who completed
// IKE_SA_INIT holds, so an AUTH keyed by them proves nothing by itself: what it
// authenticates is the EAP exchange that came before it. RFC 7296 Section 2.16:
// "Following such an extended exchange, the EAP AUTH payloads MUST be included in the
// two messages following the one containing the EAP Success message." Without this
// check a peer could send its AUTH on the first EAP round and skip authenticating at
// all. A key-deriving method was covered by the MSK being zero until success; a method
// that derives no key has no such accident to rely on.
func eapAuthSecret(sa *SA, isInitiator bool) ([]byte, error) {
	facts, ok := sa.EAPSession.(eapMethodFacts)
	if !ok {
		return nil, fmt.Errorf(
			"ike auth: peer %q ran no EAP exchange (session %T), so nothing can say which secret "+
				"RFC 7296 Section 2.16 keys its AUTH payload with", sa.PeerName, sa.EAPSession)
	}
	if !facts.Succeeded() {
		return nil, fmt.Errorf(
			"ike auth: the EAP exchange with peer %q has not succeeded, and RFC 7296 Section 2.16 "+
				"places the EAP AUTH payloads after the EAP Success message", sa.PeerName)
	}
	if facts.DerivesKey() {
		return sa.EAPMSK[:], nil
	}
	if sa.SKKeys == nil {
		return nil, fmt.Errorf(
			"ike auth: peer %q holds no SK_pi and SK_pr, so the EAP method it ran, which derives "+
				"no key, has no AUTH key at all", sa.PeerName)
	}
	if isInitiator {
		return sa.SKKeys.SK_pi, nil
	}
	return sa.SKKeys.SK_pr, nil
}

// computeEAPAuth computes this side's AUTH payload for an EAP exchange: message 7 on
// the initiator, message 8 on the responder (RFC 7296 Section 2.16).
//
// One role bool selects both halves, and it has to. RFC 7296 Section 2.15 pairs the
// initiator's signed octets with SK_pi and the responder's with SK_pr, so a secret
// chosen under one role and octets built under the other produce an AUTH the far side
// cannot verify.
func computeEAPAuth(sa *SA) (*wire.PayloadAUTH, error) {
	signedOctets, err := computeSignedOctets(sa, sa.IsInitiator)
	if err != nil {
		return nil, err
	}
	secret, err := eapAuthSecret(sa, sa.IsInitiator)
	if err != nil {
		return nil, err
	}
	authData, err := computeAuthFromSharedSecret(sa.Proposal.PRF.ID, secret, signedOctets)
	if err != nil {
		return nil, err
	}
	return &wire.PayloadAUTH{
		AuthMethod: wire.AuthMethodPSK,
		AuthData:   authData,
	}, nil
}
