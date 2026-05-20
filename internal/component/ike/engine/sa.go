// Design: plan/spec-ipsec-7-ikev2-engine.md -- IKE SA state
// RFC: rfc/short/rfc7296.md -- IKE SA SPIs and state (Section 2.6)
package engine

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/crypto"
	"codeberg.org/thomas-mangin/ze/internal/component/ipsec"
)

// SAState represents the FSM state of an IKE SA.
type SAState uint8

const (
	StateIdle           SAState = iota
	StateSAInitSent             // initiator sent IKE_SA_INIT, awaiting response
	StateSAInitReceived         // responder received IKE_SA_INIT, sent response
	StateAuthSent               // initiator sent IKE_AUTH, awaiting response
	StateAuthReceived           // responder received IKE_AUTH, sent response
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

	// Retransmission
	LastSentMsg     []byte
	RetransmitTime  time.Time
	RetransmitCount int

	// Lifecycle
	CreatedAt     time.Time
	EstablishedAt time.Time

	// Remote peer hash algorithms announced via SIGNATURE_HASH_ALGORITHMS notify
	RemoteHashAlgos []uint16
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
