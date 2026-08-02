// Design: plan/learned/740-ipsec-7-ikev2-engine.md -- IKE SA state
// RFC: rfc/short/rfc7296.md -- IKE SA SPIs and state (Section 2.6)
package engine

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/eap"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// SAState represents the FSM state of an IKE SA.
type SAState uint8

const (
	StateIdle           SAState = iota
	StateSAInitSent             // initiator sent IKE_SA_INIT, awaiting response
	StateSAInitReceived         // responder received IKE_SA_INIT, sent response
	StateAuthSent               // initiator sent IKE_AUTH, awaiting response
	StateAuthReceived           // responder received IKE_AUTH, sent response
	StateEAPInProgress          // initiator EAP exchange in IKE_AUTH (RFC 7296 Section 2.16)
	StateEstablished            // IKE SA fully established
	StateDead                   // SA is being torn down
)

func (s SAState) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateSAInitSent:
		return "sa-init-sent"
	case StateSAInitReceived:
		return "sa-init-responded"
	case StateAuthSent:
		return "auth-sent"
	case StateAuthReceived:
		return "auth-responded"
	case StateEAPInProgress:
		return "eap-in-progress"
	case StateEstablished:
		return "established"
	case StateDead:
		return "dead"
	}
	return "unknown"
}

// SA holds the state for a single IKE Security Association.
type SA struct {
	PeerName string
	PeerCfg  ipsec.SiteToSitePeer
	IKEGroup ipsec.IKEGroup
	ESPGroup ipsec.ESPGroup

	InitiatorSPI [8]byte
	ResponderSPI [8]byte
	IsInitiator  bool
	State        SAState

	// IKE_SA_INIT exchange data
	LocalNonce  []byte
	RemoteNonce []byte
	LocalDH     *crypto.DHExchange
	RemoteDHPub []byte

	// Negotiated proposal
	Proposal crypto.IKEProposal

	// Key material
	SKKeys *crypto.SKKeys

	// IKE_SA_INIT message bytes for AUTH computation
	// RFC 7296 Section 2.15: signed octets include the first IKE_SA_INIT message
	InitiatorSAInitMsg []byte
	ResponderSAInitMsg []byte

	// Message ID counters
	// RFC 7296 Section 2.2: separate counters for initiator and responder
	NextMsgID     uint32
	ExpectedMsgID uint32

	// msgIDExhausted records that one of the two counters reached the 32-bit ceiling.
	// RFC 7296 Section 2.2 calls the Message ID replay protection. It requires the SA
	// to be closed or rekeyed rather than wrapped. The counter therefore freezes and
	// this flag is raised instead. reserveRequestWindow then refuses every later
	// request, and the maintainSA ticker closes the SA. The methods live in msgid.go,
	// and only the maintainSA owner loop touches the flag.
	msgIDExhausted bool

	// PeerWindowSize is the number of outstanding requests the peer promised to keep,
	// read from its SET_WINDOW_SIZE notify during IKE_AUTH (RFC 7296 Section 2.3).
	// Zero means the peer sent none, which the same section reads as a window of one.
	//
	// It bounds what Ze MAY SEND. It never widens what Ze ACCEPTS: the accept width is
	// set by the SET_WINDOW_SIZE that Ze itself sends, and Ze sends none. Wiring this
	// field into classifyInbound would make every id in the range a replay candidate
	// against the counter Section 2.2 calls replay protection.
	PeerWindowSize uint32

	// Established-SA request/response window (RFC 7296 Section 2.3, size 1).
	// Cached so a retransmitted peer request is answered without reprocessing.
	// Accessed only by the maintainSA owner loop after establishment.
	lastResponse    []byte
	lastResponseID  uint32
	lastResponseSet bool

	// cachedReplayLimiter bounds how often this SA replays lastResponse to an address
	// it observed rather than to the configured peer.
	// RFC 7296 Section 2.21.4 requires a rate limit on messages sent in answer to
	// unprotected traffic. The cached response is the largest thing an
	// unauthenticated datagram can draw out of Ze (notify_error.go).
	cachedReplayLimiter *outboundNotifyLimiter

	// invalidMsgIDLimiter bounds how many INVALID_MESSAGE_ID notifications this SA
	// raises. RFC 7296 Section 2.3 MUST: "notifications of this type MUST be rate
	// limited". The emitter lives in notify_invalid_msgid.go.
	invalidMsgIDLimiter *outboundNotifyLimiter

	// The one self-initiated request this SA awaits an answer for. RFC 7296
	// Section 2.3 calls it the window for requests we send. Its size is one,
	// because Ze never sends a SET_WINDOW_SIZE notify and never reads one. Every
	// path that raises a request reserves this window before it reads NextMsgID,
	// and classifyInbound frees it when the answer arrives. The window methods
	// live in msgid.go, and only the maintainSA owner loop touches them.
	requestOutstanding bool
	requestMsgID       uint32
	requestSentAt      time.Time

	// requestMsg is the datagram of the request holding the window above, kept so the
	// owner loop can repeat it under its OWN Message ID (RFC 7296 Section 2.1).
	//
	// A request that spends a Message ID and keeps no copy of itself cannot be repeated.
	// If that datagram is lost, NextMsgID has moved and the peer still expects the id it
	// never saw. Every later request then falls outside the peer's window of one, and the
	// SA stalls until the liveness budget tears it down.
	//
	// The rekey and the DPD probe keep their own copies (pendingRekey.sentMsg,
	// dpdState.probeMsg) and never arm this slot. It serves the requests that have no
	// machine of their own, which today is the INVALID_MESSAGE_ID notify. A Delete
	// deliberately arms nothing: it ends the SA, so a lost one desynchronizes a
	// counter no later request reads.
	//
	// requestAttempts counts the repeats already made and requestLastSent is when the
	// last one went out. Together they drive the same backoff every other retransmit
	// in this package uses.
	requestMsg      []byte
	requestAttempts int
	requestLastSent time.Time

	// Retransmission
	LastSentMsg     []byte
	RetransmitTime  time.Time
	RetransmitCount int

	// Cookie is the COOKIE notification data a responder challenged this initiation
	// with, echoed as the FIRST payload of the next IKE_SA_INIT request.
	//
	// RFC 7296 Section 2.6 MUST: "If the IKE_SA_INIT response includes the COOKIE
	// notification, the initiator MUST then retry the IKE_SA_INIT request, and include
	// the COOKIE notification containing the received data as the first payload, and
	// all other payloads unchanged." Nil means the request carries no cookie, which is
	// what every first attempt does.
	//
	// It survives a later INVALID_KE_PAYLOAD retry, because Section 2.6.1's shorter
	// exchange carries the retained cookie beside a corrected KE payload.
	Cookie []byte

	// SAInitRetries counts the IKE_SA_INIT retries of this cycle, across BOTH causes.
	//
	// One shared counter, never one per cause. RFC 7296 Section 2.6.1 describes a
	// responder that folds KEi into its cookie calculation and therefore answers a
	// corrected retry with a NEW cookie; a per-cause counter would let that alternate
	// COOKIE / INVALID_KE_PAYLOAD without bound. The exchange must converge or give up,
	// never oscillate.
	SAInitRetries int

	// Lifecycle
	CreatedAt     time.Time
	EstablishedAt time.Time

	// hardExpiry is the instant this IKE SA's negotiated lifetime ends, held as unix
	// nanoseconds. Zero means no lifetime was configured, so the SA never expires.
	//
	// RFC 7296 Section 2.8: "When the lifetime of a Security Association expires, the
	// Security Association MUST NOT be used." The owner loop notices expiry once a
	// second and tears the SA down, but a send can be reached between two ticks, so
	// the deadline has to be readable from the send path itself rather than only from
	// the loop's stack.
	//
	// It is reached through atomic.LoadInt64 and atomic.StoreInt64, and is not declared
	// as an atomic.Int64. A test builds a twin SA of the opposite role from a copy of
	// this struct. atomic.Int64 carries noCopy, so go vet rejects that copy. A plain
	// int64 gives the same atomicity and permits the copy. The twin then inherits the
	// deadline by value, which is correct.
	hardExpiry int64

	// Remote peer hash algorithms announced via SIGNATURE_HASH_ALGORITHMS notify
	RemoteHashAlgos []uint16

	// NAT Traversal state (RFC 7296 Section 2.23).
	NATDetected bool
	BehindNAT   bool // true if we are the side behind NAT

	// peerEndpoint is the source address and port of the last message this SA
	// AUTHENTICATED. It is the destination of every message the SA sends on its own
	// initiative, and of every response the owner loop builds.
	//
	// RFC 7296 Section 2.11 MUST: an implementation
	// "MUST respond to the address and port from which the request was received".
	// RFC 7296 Section 2.23 repeats it, and adds that the port matters because a NAT
	// translates it.
	//
	// It is written by adoptAuthenticatedEndpoint alone, and only after a decrypt and
	// a Message ID window check. An unauthenticated datagram never moves it. Nil
	// means no authenticated message has arrived yet, and remoteUDPAddr then falls
	// back to the CONFIGURED remote rather than to whatever last arrived
	// (ai/rules/fail-closed-guards.md).
	//
	// Owner-loop state after establishment, like lastResponse, so it needs no lock.
	peerEndpoint *net.UDPAddr

	// localPort names the UDP port this SA sends FROM.
	//
	// RFC 7296 Section 2.23 MUST: an endpoint
	// "that discovers a NAT between it and its correspondent (as described below) MUST send all subsequent traffic from port 4500".
	//
	// Zero means the float decision has not been taken, and it reads as the IKE port.
	// A zero that read as 4500 would be the zero-value trap, so the single read site
	// in sendPath compares against transport.NATTPort rather than against zero.
	//
	// It answers a different question from NATDetected, which selects UDP
	// encapsulation for the Child SA and starts the keepalive. Merging the two is the
	// defect plan/spec-fixit-ike-responder-natt-port-float.md records.
	localPort uint16

	// ikeSocket and nattSocket are the two sockets this SA can send from. The session
	// binds both when it creates the SA.
	//
	// A nil nattSocket with a floated localPort sends nothing. A floated peer reads a
	// message from port 500 with no non-ESP marker as ESP, and drops it with no log.
	ikeSocket  *transport.UDPTransport
	nattSocket *transport.UDPTransport

	// EAP state (RFC 7296 Section 2.16).
	// EAPSession holds *eap.Session on the responder (the authenticator) and
	// *eap.PeerSession on the initiator (the peer). closeEAPSession recovers
	// whichever shape is present. Typed any so neither role's concrete type
	// appears in the SA declaration.
	EAPSession any
	EAPMSK     [64]byte

	// InitialContact records that the peer's first IKE_AUTH carried an INITIAL_CONTACT
	// notify (RFC 7296 Section 2.4): it asserts this is the only IKE SA to the peer
	// identity, authorizing us to delete any stale SA to it without waiting for a
	// timeout. Set on the responder in handleAuthRequest.
	InitialContact bool

	// Remote peer identity from IKE_AUTH response IDr payload.
	RemoteIDPayload *wire.PayloadID

	// Remote peer certificate from the FIRST IKE_AUTH CERT payload (DER-encoded X.509).
	// RFC 7296 Section 3.6 states the rule.
	// "If multiple certificates are sent, the first certificate MUST contain the public
	// key associated with the private key used to sign the AUTH payload."
	// The first payload is the peer certificate, and no later one is.
	RemoteCertRaw []byte

	// Every CERT payload after the first, in wire order (DER-encoded X.509). RFC 7296
	// Section 3.6 makes them the path from the peer certificate toward a trust anchor, so
	// getRemoteCert offers them to x509 as intermediates. A two-level authority cannot
	// chain without them: ca-certificate holds one anchor, not a path.
	RemoteCertChainRaw [][]byte

	// ChildProposalNum is the Proposal Num the peer put on the ESP proposal we
	// accepted from its SAi2. RFC 7296 Section 3.3.1 makes the response carry that
	// number, so the peer can tell which of its own proposals we took. Set by
	// selectResponderESP on the first IKE_AUTH, and read again on the final EAP
	// IKE_AUTH, which carries no SAi2 of its own. Zero means no selection ran.
	ChildProposalNum uint8

	// Negotiated Child SA parameters from IKE_AUTH piggybacked exchange.
	ChildInboundSPI  uint32     // our ESP SPI included in SAi2
	ChildOutboundSPI uint32     // responder's ESP SPI from AUTH response SA
	NegotiatedTSi    *net.IPNet // narrowed initiator TS from AUTH response
	NegotiatedTSr    *net.IPNet // narrowed responder TS from AUTH response

	// NegotiatedPairs is the full narrowed selector set, in TSi/TSr orientation.
	//
	// NegotiatedTSi/TSr above carry only the FIRST pair's prefixes, because the Child SA
	// and the dataplane policy have historically been single-selector. This slice keeps
	// the whole answer, including the port and protocol of each selector, so the payload
	// Ze puts on the wire and the policy Ze programs come from ONE source. RFC 7296
	// Section 2.9 permits several selectors, so a single prefix pair cannot represent
	// every conformant answer.
	NegotiatedPairs []tsPair

	// ProposedChildPairs is the selector set Ze put in its OWN TSi/TSr.
	//
	// proposeChildTSPayloads (rekey.go) records it. That function is the single producer of
	// an initiator's Child SA proposal.
	//
	// RFC 7296 Section 2.9 lets a responder NARROW the proposal, and never widen it. This is
	// therefore the ceiling for the answer (recordInitiatorSelectors, ts_narrow.go).
	// Without it the initiator installed whatever selectors came back. A hostile or buggy
	// responder then chose the traffic Ze forwards into the tunnel.
	//
	// An EMPTY slice means Ze proposed the wildcard. That is what a peer with no configured
	// traffic-selector list does. Everything is then within the proposal, and the configured
	// policy is the only constraint left. It is itself empty in that case.
	ProposedChildPairs []tsPair

	// UseTransportMode records that this SA's Child SAs run in transport mode.
	//
	// RFC 7296 Section 1.3.1: transport mode is negotiated with the USE_TRANSPORT_MODE
	// notification, and "Except when using this option to negotiate transport mode, all
	// Child SAs will use tunnel mode." The field is false until the notification is both
	// sent and echoed, so an unanswered request leaves tunnel mode, which is the RFC's
	// own outcome for a declined request.
	UseTransportMode bool

	// PeerRequestedTransport records that the peer asked for transport mode on this
	// exchange. The responder reads it to decide whether to echo USE_TRANSPORT_MODE.
	PeerRequestedTransport bool
}

// GenerateSPI generates a random 8-byte SPI value using crypto/rand.
// RFC 7296 Section 2.6: SPI values MUST NOT be zero.
func GenerateSPI() ([8]byte, error) {
	var spi [8]byte
	for {
		if _, err := rand.Read(spi[:]); err != nil {
			return spi, err
		}
		if spi != [8]byte{} {
			return spi, nil
		}
	}
}

// GenerateNonce generates a random nonce of the given length.
// RFC 7296 Section 2.10: nonces MUST be at least 16 bytes and no more than 256 bytes.
func GenerateNonce(length int) ([]byte, error) {
	nonce := make([]byte, length)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return nonce, nil
}

// SPIPairKey returns a string key for the SPI pair, used as a map key.
func SPIPairKey(initiator, responder [8]byte) string {
	return hex.EncodeToString(initiator[:]) + ":" + hex.EncodeToString(responder[:])
}

// SPIHex returns the hex string of an SPI.
func SPIHex(spi [8]byte) string {
	return hex.EncodeToString(spi[:])
}

// setHardExpiry records the instant this IKE SA's lifetime ends. A zero instant
// clears the deadline, which is what an unconfigured lifetime means.
func (sa *SA) setHardExpiry(at time.Time) {
	if at.IsZero() {
		atomic.StoreInt64(&sa.hardExpiry, 0)
		return
	}
	atomic.StoreInt64(&sa.hardExpiry, at.UnixNano())
}

// lifetimeExpired reports whether this IKE SA's negotiated lifetime has run out.
// RFC 7296 Section 2.8 forbids using the SA once it has.
//
// An SA with no configured lifetime never expires, so a zero deadline reports
// false rather than "expired at the epoch".
func (sa *SA) lifetimeExpired(now time.Time) bool {
	deadline := atomic.LoadInt64(&sa.hardExpiry)
	if deadline == 0 {
		return false
	}
	return !now.Before(time.Unix(0, deadline))
}

// remoteUDPAddr returns the destination for a message this SA sends on its own
// initiative, and for every response the owner loop builds.
//
// RFC 7296 Section 2.11 MUST: an implementation
// "MUST respond to the address and port from which the request was received".
// The authenticated observation is therefore preferred over the configured remote.
//
// The fallback is the CONFIGURED remote, never the datagram in hand. An
// unauthenticated source address is attacker-chosen, and a fallback that used it
// would let one forged packet aim the SA at a victim
// (ai/rules/fail-closed-guards.md). The port of that fallback follows localPort, so
// a floated SA reaches the peer's port 4500 before its first authenticated message.
//
// It returns a COPY. established.go rewrites the port of the value it gets back, and
// a shared pointer would let that rewrite corrupt the SA's own state.
func (sa *SA) remoteUDPAddr() *net.UDPAddr {
	if sa.peerEndpoint != nil {
		return &net.UDPAddr{
			IP:   append(net.IP(nil), sa.peerEndpoint.IP...),
			Port: sa.peerEndpoint.Port,
			Zone: sa.peerEndpoint.Zone,
		}
	}
	addr, err := net.ResolveUDPAddr("udp4", ikeAddr(sa.PeerCfg.RemoteAddress))
	if err != nil {
		return nil
	}
	if sa.localPort == transport.NATTPort {
		addr.Port = transport.NATTPort
	}
	return addr
}

// bindSockets gives the SA the two sockets its session owns. The session calls it
// once, when it creates the SA.
func (sa *SA) bindSockets(ike, natt *transport.UDPTransport) {
	sa.ikeSocket = ike
	sa.nattSocket = natt
}

// inheritSendPath copies the send path of the SA this one replaces: both sockets,
// the float verdict, and the authenticated peer endpoint.
//
// RFC 7296 Section 2.18 makes a rekeyed IKE SA the same conversation, with the same
// peer, over the same path. None of the four is renegotiated.
//
// A replacement that started at port 500 with no endpoint drops every NAT-traversing
// tunnel on its first rekey. That failure appears an hour after establishment.
func (sa *SA) inheritSendPath(old *SA) {
	sa.ikeSocket = old.ikeSocket
	sa.nattSocket = old.nattSocket
	sa.localPort = old.localPort
	sa.peerEndpoint = old.peerEndpoint
}

// floatToNATTPort records that every later message this SA sends leaves from port
// 4500. RFC 7296 Section 2.23 MUST: an endpoint
// "that discovers a NAT between it and its correspondent (as described below) MUST send all subsequent traffic from port 4500".
//
// The float is sticky. The same sentence says ALL subsequent traffic, so nothing
// moves the SA back to port 500 once a NAT is known.
func (sa *SA) floatToNATTPort() {
	sa.localPort = transport.NATTPort
}

// sendPath returns the socket this SA sends from, and whether that socket frames its
// messages with the non-ESP marker of RFC 3948 Section 2.2.
//
// The fallback covers an SA the session never bound, which is every SA a unit test
// builds by hand. It reports the role of the socket it was handed.
//
// It fails closed on the one case that matters. A floated SA with no NAT-T socket
// returns nil, so nothing is sent.
//
// A floated peer reads a message from port 500 with no marker as ESP, and drops it
// with no log. A silent fallback would therefore look like a working tunnel and
// carry nothing (ai/rules/fail-closed-guards.md).
func (sa *SA) sendPath(fallback *transport.UDPTransport) (*transport.UDPTransport, bool) {
	if sa.localPort == transport.NATTPort {
		if sa.nattSocket == nil {
			return nil, false
		}
		return sa.nattSocket, true
	}
	if sa.ikeSocket != nil {
		return sa.ikeSocket, sa.ikeSocket.IsNATT()
	}
	natT := fallback.IsNATT()
	return fallback, natT
}

// adoptAuthenticatedEndpoint records where an AUTHENTICATED message came from, and
// floats the SA when that message arrived on the NAT-T socket.
//
// CALLER OBLIGATIONS. Both are RFC conditions, and neither can be checked here.
//
//   - The message MUST have decrypted and its integrity check MUST have passed.
//     RFC 7296 Section 2.23 names the trigger as a packet
//     "whose integrity protection validates".
//   - The Message ID window check MUST have run first. The same section states that
//     "dynamic updates can only be done safely if replay protection is enabled",
//     and the Message ID counter of Section 2.2 IS that replay protection.
//
// The caller therefore calls this AFTER decryptAndParse returns a nil error, and
// never from a branch that only compared a Message ID.
//
// The FIRST observation always lands. It associates an endpoint with the SA rather
// than changing one, and RFC 7296 Section 2.11 requires the response to reach the
// port the request came from.
//
// A LATER observation that DIFFERS is the dynamic address update of RFC 7296
// Section 2.23. It is refused while this node is behind a NAT. "A host behind a NAT
// SHOULD NOT do this type of dynamic address update if a validated packet has
// different port and/or address values because it opens a possible DoS attack".
func (sa *SA) adoptAuthenticatedEndpoint(observed *net.UDPAddr, arrivedOnNATT bool, log *slog.Logger) {
	if arrivedOnNATT {
		sa.floatToNATTPort()
	}
	if observed == nil {
		return
	}
	if sa.peerEndpoint == nil {
		sa.peerEndpoint = copyUDPAddr(observed)
		return
	}
	if sameUDPEndpoint(sa.peerEndpoint, observed) {
		return
	}
	if sa.BehindNAT {
		log.Debug("ike: authenticated packet came from a new endpoint, not adopted because this node is behind a NAT",
			"peer", sa.PeerName, "stored", sa.peerEndpoint, "observed", observed)
		return
	}
	log.Info("ike: peer endpoint moved, adopting the address of the validated packet",
		"peer", sa.PeerName, "old", sa.peerEndpoint, "new", observed)
	sa.peerEndpoint = copyUDPAddr(observed)
}

// copyUDPAddr detaches an address from the packet buffer that carried it, so a later
// read of the SA cannot see a reused slice.
func copyUDPAddr(a *net.UDPAddr) *net.UDPAddr {
	return &net.UDPAddr{IP: append(net.IP(nil), a.IP...), Port: a.Port, Zone: a.Zone}
}

// sameUDPEndpoint reports whether two addresses name one endpoint. It compares the
// address and the port, because RFC 7296 Section 2.23 makes a changed PORT alone a
// reason to move the SA.
func sameUDPEndpoint(a, b *net.UDPAddr) bool {
	return a.Port == b.Port && a.IP.Equal(b.IP)
}

// initiatorNonce and responderNonce return the two IKE_SA_INIT nonces in the
// absolute RFC 7296 order (Ni = the IKE-SA initiator's nonce, Nr = the
// responder's), independent of which side this SA is. Key derivation (SKEYSEED,
// SK_*, and Child SA KEYMAT) is defined over the absolute pair Ni|Nr, so
// responder-side derivation MUST NOT feed Local/Remote directly — for a responder
// SA LocalNonce is Nr and RemoteNonce is Ni. For an initiator SA these return
// LocalNonce/RemoteNonce respectively, so existing initiator call sites are
// unchanged. RFC 7296 Section 2.14, Section 2.17.
func (sa *SA) initiatorNonce() []byte {
	if sa.IsInitiator {
		return sa.LocalNonce
	}
	return sa.RemoteNonce
}

func (sa *SA) responderNonce() []byte {
	if sa.IsInitiator {
		return sa.RemoteNonce
	}
	return sa.LocalNonce
}

// forgetKeys erases every secret this IKE SA holds, and every input from which those
// secrets could be derived again.
//
// RFC 7296 Section 2.12 MUST: "Achieving perfect forward secrecy requires that when a
// connection is closed, each endpoint MUST forget not only the keys used by the
// connection but also any information that could be used to recompute those keys."
//
// The second clause is the wider one, and it decides what this erases beyond SKKeys:
//
//	SK_d        recomputes a rekeyed SA's whole key set (Section 2.18), so it is the
//	            load-bearing member of SKKeys rather than one of seven.
//	LocalDH     holds the private exponent. With the peer's public value it recomputes
//	            the shared secret g^ir, and SKEYSEED with it. This is the secret that
//	            makes the rest of the input useless on its own.
//	EAPMSK      generates the AUTH payloads of the EAP exchange (Section 2.16).
//	the nonces  SKEYSEED = prf(Ni | Nr, g^ir) (Section 2.14), so Ni and Nr complete
//	            that input.
//
// The three secrets are OVERWRITTEN. The two nonces are only DROPPED, and the
// difference is deliberate. Ni and Nr travel in cleartext in IKE_SA_INIT, so they are
// public and overwriting them buys no secrecy. Their buffers are also borrowed: a nonce
// is taken from a decoded payload or handed in by the caller that generated it, and
// this SA is not their only reader. Zeroing one corrupts whoever else holds it, which
// is a live defect rather than a hardening step. Releasing the reference is what
// "forget" means for a value that was never secret.
//
// Safe on a partly built SA and on a nil SA: every field is optional and a nil one is
// skipped. Called once per SA, on the path that closes it.
func (sa *SA) forgetKeys() {
	if sa == nil {
		return
	}
	if sa.SKKeys != nil {
		sa.SKKeys.Clear()
	}
	if sa.LocalDH != nil {
		sa.LocalDH.Clear()
	}
	sa.closeEAPSession()
	clear(sa.EAPMSK[:])
	sa.LocalNonce = nil
	sa.RemoteNonce = nil
}

// closeEAPSession ends whatever EAP exchange this SA carried.
//
// It belongs on the close path for two reasons. An EAP-TLS exchange runs its TLS
// engine on a goroutine parked in a read that only this call releases, so an SA
// that ends mid-exchange strands that goroutine; and the goroutine holds a live
// tls.Conn, so erasing EAPMSK beside a surviving connection would forget the
// derived key while leaving the material that derives it, which is the wider
// clause of Section 2.12 above.
//
// This runs only where the SA is already over. Calling it during a live exchange
// would fail the handshake, but every caller of forgetKeys has just erased
// SK_d and the DH private value, so no exchange can continue past this point.
//
// The field is typed any (see the SA declaration), so the two session shapes are
// recovered by assertion. Both Close methods are idempotent and nil-safe.
func (sa *SA) closeEAPSession() {
	switch s := sa.EAPSession.(type) {
	case *eap.Session:
		s.Close()
	case *eap.PeerSession:
		s.Close()
	}
}
