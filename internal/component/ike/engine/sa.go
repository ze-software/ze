// Design: plan/learned/740-ipsec-7-ikev2-engine.md -- IKE SA state
// RFC: rfc/short/rfc7296.md -- IKE SA SPIs and state (Section 2.6)
package engine

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"time"

	"github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
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

	// Established-SA request/response window (RFC 7296 Section 2.3, size 1).
	// Cached so a retransmitted peer request is answered without reprocessing.
	// Accessed only by the maintainSA owner loop after establishment.
	lastResponse    []byte
	lastResponseID  uint32
	lastResponseSet bool

	// Retransmission
	LastSentMsg     []byte
	RetransmitTime  time.Time
	RetransmitCount int

	// Lifecycle
	CreatedAt     time.Time
	EstablishedAt time.Time

	// Remote peer hash algorithms announced via SIGNATURE_HASH_ALGORITHMS notify
	RemoteHashAlgos []uint16

	// NAT Traversal state (RFC 7296 Section 2.23).
	NATDetected bool
	BehindNAT   bool // true if we are the side behind NAT

	// EAP state (RFC 7296 Section 2.16).
	EAPSession any // *eap.Session, stored as any to avoid import cycle
	EAPMSK     [64]byte

	// InitialContact records that the peer's first IKE_AUTH carried an INITIAL_CONTACT
	// notify (RFC 7296 Section 2.4): it asserts this is the only IKE SA to the peer
	// identity, authorizing us to delete any stale SA to it without waiting for a
	// timeout. Set on the responder in handleAuthRequest.
	InitialContact bool

	// Remote peer identity from IKE_AUTH response IDr payload.
	RemoteIDPayload *wire.PayloadID

	// Remote peer certificate from IKE_AUTH CERT payload (DER-encoded X.509).
	RemoteCertRaw []byte

	// Negotiated Child SA parameters from IKE_AUTH piggybacked exchange.
	ChildInboundSPI  uint32     // our ESP SPI included in SAi2
	ChildOutboundSPI uint32     // responder's ESP SPI from AUTH response SA
	NegotiatedTSi    *net.IPNet // narrowed initiator TS from AUTH response
	NegotiatedTSr    *net.IPNet // narrowed responder TS from AUTH response
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

func (sa *SA) remoteUDPAddr() *net.UDPAddr {
	addr, err := net.ResolveUDPAddr("udp4", ikeAddr(sa.PeerCfg.RemoteAddress))
	if err != nil {
		return nil
	}
	return addr
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
