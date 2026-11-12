// Design: docs/architecture/testing/ci-format.md -- compiled fixtures a .ci drives through `le test fixture`.
// Related: register_ipsec_malformed_response.go -- the registration
// Related: test/ipsec/ike-peer-malformed-doctor.ci -- the run that starts this peer
// Related: tunnel_fixture_ipsec_corrupt_response.go -- the relay sibling; corruptRelayListen binds the same address
//
// The IKEv2 peer of test/ipsec/ike-peer-malformed-doctor.ci. It initiates a real IKE SA
// to a Ze responder: IKE_SA_INIT, then IKE_AUTH with a pre-shared key, with Ze's own
// key derivation (internal/component/ike/crypto) and payload codec
// (internal/component/ike/wire), so its keys are the keys Ze derives. Once one
// INFORMATIONAL exchange has crossed the established SA, it sends one INFORMATIONAL
// response that is correctly sealed under the SA's keys and whose inner payload chain
// does not parse. Ze's handleOwnedInbound (internal/component/ike/engine/inbound.go)
// counts it on the SA, which the doctor check ipsec-peer-malformed-message reports.
//
// Only a peer holding the keys can produce that message, which is why no relay can:
// a relay that changes a sealed message makes its integrity check fail, the other
// doctor check's input. The peer then answers Ze's own INFORMATIONAL requests, so the
// SA stays up while the test reads it.
//
// The proposals are fixed to the ones the .ci configures: AES-CBC-256, HMAC-SHA2-256-128,
// PRF-HMAC-SHA2-256 and MODP-2048 for the IKE SA, AES-CBC-256 and HMAC-SHA2-256-128 for
// the Child SA.

package fixture

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"time"

	ikecrypto "github.com/ze-software/ze/internal/component/ike/crypto"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// malformedZeAddr is the Ze responder's local-address. The peer binds corruptRelayAddr
// (127.0.0.3), the Ze config's remote-address.
var malformedZeAddr = netip.MustParseAddr("127.0.0.2")

const (
	// malformedPSK is the pre-shared-secret of the .ci's peer block.
	malformedPSK = "test-key-1234"
	// malformedKeyPad is the pad string of the AUTH computation (RFC 7296 Section 2.15).
	malformedKeyPad = "Key Pad for IKEv2"
	// malformedNonceOctets is the nonce length: at least half the PRF key size, and
	// within the 16 to 256 octets RFC 7296 Section 3.9 allows.
	malformedNonceOctets = 32
	// malformedInitAttempts bounds the IKE_SA_INIT sends. The peer starts before the
	// daemon, so the first sends meet a closed port until the daemon binds.
	malformedInitAttempts = 25
	// malformedAttempts bounds the sends of every later request.
	malformedAttempts = 10
	// malformedReplyWait is how long one send waits for its answer before it is sent again.
	malformedReplyWait = 2 * time.Second
	// malformedLifetimeMax bounds the peer when nothing cancels it. The runner's
	// background timeout and its teardown end it first in every .ci run.
	malformedLifetimeMax = 5 * time.Minute
	// malformedReadSlice is how long one serve read waits before the loop rechecks the context.
	malformedReadSlice = 200 * time.Millisecond
	// malformedICVOctets is the Integrity Checksum Data length of AUTH_HMAC_SHA2_256_128.
	malformedICVOctets = 16
	// malformedSPIOctets is the length of the ESP SPI the Child SA proposal carries.
	malformedSPIOctets = 4
	// malformedLivenessMsgID is the Message ID of the INFORMATIONAL request that proves
	// the established SA answers: IKE_SA_INIT took 0 and IKE_AUTH took 1.
	malformedLivenessMsgID = 2
	// malformedResponseMsgID is the Message ID of the malformed response. Ze holds no
	// rekey under it, so classifyInbound (msgid.go) answers inboundInvalid and the
	// INFORMATIONAL response arm of handleOwnedInbound opens it.
	malformedResponseMsgID = 0
)

// malformedPeerSA is the peer's side of one IKE SA. Not safe for concurrent use: one
// goroutine, the driver's, runs every exchange.
type malformedPeerSA struct {
	conn     *net.UDPConn
	ze       netip.AddrPort
	spiI     [8]byte
	spiR     [8]byte
	keys     *ikecrypto.SKKeys
	integ    ikecrypto.IntegrityTransform
	datagram []byte
}

// tunnelIPsecMalformedResponsePeer runs the peer until the context is canceled (the
// runner's SIGTERM at a stop or at teardown) or malformedLifetimeMax passes.
//
// Arguments: the test's IKE port ($PORT2).
func tunnelIPsecMalformedResponsePeer(ctx context.Context, args []string) error {
	port, err := malformedPeerArgs(args)
	if err != nil {
		return err
	}
	conn, err := corruptRelayListen(ctx, port)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // the peer is done with the socket; a close error changes no verdict

	// The ready= text of the .ci: printed once the socket is bound.
	fmt.Println("PEER: listening on", conn.LocalAddr())

	sa := &malformedPeerSA{conn: conn, ze: netip.AddrPortFrom(malformedZeAddr, port), datagram: make([]byte, corruptDatagramMax)}
	if err := sa.handshake(ctx); err != nil {
		return err
	}
	fmt.Println("PEER-ESTABLISHED: IKE SA up, initiator SPI", hex.EncodeToString(sa.spiI[:]), "responder SPI", hex.EncodeToString(sa.spiR[:]))

	if err := sa.liveness(ctx); err != nil {
		return err
	}
	fmt.Println("PEER-LIVE: the responder answered an INFORMATIONAL request on the established SA")

	if err := sa.sendMalformedResponse(); err != nil {
		return err
	}
	fmt.Println("PEER-MALFORMED: sent one sealed INFORMATIONAL response whose inner payload chain does not parse, message ID", malformedResponseMsgID)

	return sa.serve(ctx)
}

// malformedPeerArgs reads the IKE port.
func malformedPeerArgs(args []string) (uint16, error) {
	if len(args) != 1 {
		return 0, fmt.Errorf("ipsec/malformed-response-peer takes the test's IKE port only: le test fixture ipsec/malformed-response-peer $PORT2, got %q", args)
	}
	port, err := strconv.ParseUint(args[0], 10, 16)
	if err != nil {
		return 0, fmt.Errorf("ipsec/malformed-response-peer: IKE port %q: %w", args[0], err)
	}
	if port == 0 {
		return 0, fmt.Errorf("ipsec/malformed-response-peer: IKE port %q must be above 0", args[0])
	}
	return uint16(port), nil
}

// handshake runs IKE_SA_INIT and IKE_AUTH, derives the SA's keys, and answers an error
// unless the responder's IKE_AUTH response carries its AUTH payload.
func (sa *malformedPeerSA) handshake(ctx context.Context) error {
	if _, err := rand.Read(sa.spiI[:]); err != nil {
		return fmt.Errorf("initiator SPI: %w", err)
	}
	dh, err := ikecrypto.NewDHExchange(ikecrypto.DH_MODP_2048)
	if err != nil {
		return fmt.Errorf("diffie-hellman: %w", err)
	}
	defer dh.Clear()
	ni := make([]byte, malformedNonceOctets)
	if _, err := rand.Read(ni); err != nil {
		return fmt.Errorf("nonce: %w", err)
	}

	initRequest := malformedSAInitRequest(sa.spiI, dh.PublicKey, ni, nil)
	initReply, err := sa.exchange(ctx, initRequest, wire.ExchangeIKESAInit, 0, malformedInitAttempts)
	if err != nil {
		return fmt.Errorf("IKE_SA_INIT: %w", err)
	}
	var header wire.Header
	if err := header.ReadFrom(initReply); err != nil {
		return fmt.Errorf("IKE_SA_INIT response header: %w", err)
	}
	// RFC 7296 Section 2.6: "If the IKE_SA_INIT response includes the COOKIE
	// notification, the initiator MUST then retry the IKE_SA_INIT request, and include
	// the COOKIE notification containing the received data as the first payload, and all
	// other payloads unchanged." Ze challenges while it counts half-open SAs at or above
	// its cookie-threshold (cookieRequired, cookie.go).
	cookie, err := malformedCookie(initReply[wire.HeaderLen:], header.NextPayload)
	if err != nil {
		return err
	}
	if cookie != nil {
		// The retried request is the RealMessage1 the AUTH payload signs.
		initRequest = malformedSAInitRequest(sa.spiI, dh.PublicKey, ni, cookie)
		initReply, err = sa.exchange(ctx, initRequest, wire.ExchangeIKESAInit, 0, malformedAttempts)
		if err != nil {
			return fmt.Errorf("IKE_SA_INIT with the COOKIE: %w", err)
		}
		if err := header.ReadFrom(initReply); err != nil {
			return fmt.Errorf("IKE_SA_INIT response header: %w", err)
		}
	}
	sa.spiR = header.ResponderSPI
	kePublic, nr, err := malformedSAInitAnswer(initReply[wire.HeaderLen:], header.NextPayload)
	if err != nil {
		return err
	}

	if err := sa.deriveKeys(dh, kePublic, ni, nr); err != nil {
		return err
	}

	authRequest, err := sa.authRequest(initRequest, nr)
	if err != nil {
		return err
	}
	authReply, err := sa.exchange(ctx, authRequest, wire.ExchangeIKEAuth, 1, malformedAttempts)
	if err != nil {
		return fmt.Errorf("IKE_AUTH: %w", err)
	}
	payloads, err := sa.open(authReply)
	if err != nil {
		return fmt.Errorf("IKE_AUTH response: %w", err)
	}
	return malformedAuthAnswer(payloads)
}

// malformedSAInitRequest builds the IKE_SA_INIT request: SA, KE, Ni.
//
// RFC 7296 Section 1.2: "The first pair of messages (IKE_SA_INIT) negotiate
// cryptographic algorithms, exchange nonces, and do a Diffie-Hellman
// exchange [DH]."
//
// A non-nil cookie is the responder's COOKIE data, carried as the first payload.
func malformedSAInitRequest(spiI [8]byte, kePublic, ni, cookie []byte) []byte {
	payloads := make([]wire.PayloadEntry, 0, 4)
	if cookie != nil {
		payloads = append(payloads, wire.PayloadEntry{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyCookie, NotificationData: cookie}})
	}
	payloads = append(payloads,
		wire.PayloadEntry{Payload: malformedIKEProposal()},
		wire.PayloadEntry{Payload: &wire.PayloadKE{DHGroup: uint16(ikecrypto.DH_MODP_2048), KeyExchangeData: kePublic}},
		wire.PayloadEntry{Payload: &wire.PayloadNonce{NonceData: ni}},
	)
	message := wire.Message{
		Header: wire.Header{
			InitiatorSPI: spiI,
			MajorVersion: 2,
			ExchangeType: wire.ExchangeIKESAInit,
			Flags:        wire.FlagInitiator,
			MessageID:    0,
		},
		Payloads: payloads,
	}
	out := make([]byte, message.Len())
	message.WriteTo(out, 0)
	return out
}

// malformedIKEProposal is the one IKE SA proposal the peer offers.
func malformedIKEProposal() *wire.PayloadSA {
	return &wire.PayloadSA{Proposals: []wire.Proposal{{
		IsLast:     true,
		Number:     1,
		ProtocolID: wire.ProtocolIKE,
		Transforms: []wire.Transform{
			{Type: wire.TransformTypeENCR, ID: uint16(ikecrypto.ENCR_AES_CBC), Attrs: []wire.TransformAttr{{Type: wire.AttrTypeKeyLength, Value: 256}}},
			{Type: wire.TransformTypePRF, ID: uint16(ikecrypto.PRF_HMAC_SHA2_256)},
			{Type: wire.TransformTypeINTG, ID: uint16(ikecrypto.AUTH_HMAC_SHA2_256_128)},
			{IsLast: true, Type: wire.TransformTypeDH, ID: uint16(ikecrypto.DH_MODP_2048)},
		},
	}}}
}

// malformedESPProposal is the one Child SA proposal the IKE_AUTH request offers.
func malformedESPProposal(spi []byte) *wire.PayloadSA {
	return &wire.PayloadSA{Proposals: []wire.Proposal{{
		IsLast:     true,
		Number:     1,
		ProtocolID: wire.ProtocolESP,
		SPISize:    malformedSPIOctets,
		SPI:        spi,
		Transforms: []wire.Transform{
			{Type: wire.TransformTypeENCR, ID: uint16(ikecrypto.ENCR_AES_CBC), Attrs: []wire.TransformAttr{{Type: wire.AttrTypeKeyLength, Value: 256}}},
			{Type: wire.TransformTypeINTG, ID: uint16(ikecrypto.AUTH_HMAC_SHA2_256_128)},
			{IsLast: true, Type: wire.TransformTypeESN, ID: 0},
		},
	}}}
}

// malformedCookie answers the COOKIE data of an IKE_SA_INIT response, or nil when the
// response carries none.
func malformedCookie(chain []byte, firstType uint8) ([]byte, error) {
	payloads, err := wire.ParsePayloadChain(chain, firstType)
	if err != nil {
		return nil, fmt.Errorf("IKE_SA_INIT response payloads: %w", err)
	}
	for _, entry := range payloads {
		notify, ok := entry.Payload.(*wire.PayloadNotify)
		if !ok {
			continue
		}
		if notify.NotifyMsgType == wire.NotifyCookie {
			return append([]byte(nil), notify.NotificationData...), nil
		}
	}
	return nil, nil
}

// malformedSAInitAnswer reads KEr and Nr from the IKE_SA_INIT response payloads, and
// answers an error for an error notification or a missing payload.
func malformedSAInitAnswer(chain []byte, firstType uint8) ([]byte, []byte, error) {
	payloads, err := wire.ParsePayloadChain(chain, firstType)
	if err != nil {
		return nil, nil, fmt.Errorf("IKE_SA_INIT response payloads: %w", err)
	}
	var kePublic, nr []byte
	for _, entry := range payloads {
		switch payload := entry.Payload.(type) {
		case *wire.PayloadKE:
			kePublic = payload.KeyExchangeData
		case *wire.PayloadNonce:
			nr = payload.NonceData
		case *wire.PayloadNotify:
			if wire.NotifyIsError(payload.NotifyMsgType) {
				return nil, nil, fmt.Errorf("IKE_SA_INIT refused by the responder with %s: its ike-group does not match the peer's proposal", wire.NotifyTypeName(payload.NotifyMsgType))
			}
		}
	}
	if kePublic == nil {
		return nil, nil, errors.New("IKE_SA_INIT response carries no KE payload")
	}
	if nr == nil {
		return nil, nil, errors.New("IKE_SA_INIT response carries no Nonce payload")
	}
	return kePublic, nr, nil
}

// deriveKeys derives the SA's key hierarchy.
//
// RFC 7296 Section 2.14: "SKEYSEED = prf(Ni | Nr, g^ir)" and
// "{SK_d | SK_ai | SK_ar | SK_ei | SK_er | SK_pi | SK_pr}
// = prf+ (SKEYSEED, Ni | Nr | SPIi | SPIr)".
func (sa *malformedPeerSA) deriveKeys(dh *ikecrypto.DHExchange, kePublic, ni, nr []byte) error {
	shared, err := dh.SharedSecret(kePublic)
	if err != nil {
		return fmt.Errorf("diffie-hellman shared secret: %w", err)
	}
	defer clear(shared)
	skeyseed, err := ikecrypto.DeriveSKEYSEED(ikecrypto.PRF_HMAC_SHA2_256, ni, nr, shared)
	if err != nil {
		return fmt.Errorf("SKEYSEED: %w", err)
	}
	defer clear(skeyseed)
	encryption, err := ikecrypto.LookupEncryption("aes256")
	if err != nil {
		return fmt.Errorf("encryption transform: %w", err)
	}
	sa.integ, err = ikecrypto.LookupIntegrity("sha256")
	if err != nil {
		return fmt.Errorf("integrity transform: %w", err)
	}
	sa.keys, err = ikecrypto.DeriveSKKeys(ikecrypto.PRF_HMAC_SHA2_256, skeyseed, ni, nr, sa.spiI[:], sa.spiR[:], encryption, sa.integ)
	if err != nil {
		return fmt.Errorf("SK keys: %w", err)
	}
	return nil
}

// authRequest builds the sealed IKE_AUTH request: IDi, AUTH, SA, TSi, TSr.
//
// RFC 7296 Section 2.15: "InitiatorSignedOctets = RealMessage1 | NonceRData | MACedIDForI"
// and, for a shared secret, "AUTH = prf( prf(Shared Secret, "Key Pad for IKEv2"),
// <InitiatorSignedOctets>)".
func (sa *malformedPeerSA) authRequest(initRequest, nr []byte) ([]byte, error) {
	identity := &wire.PayloadID{IDPayloadType: wire.PayloadTypeIDi, IDType: wire.IDTypeIPv4Addr, IDData: corruptRelayAddr.AsSlice()}
	identityBody := make([]byte, identity.Len())
	identity.WriteTo(identityBody, 0)
	macedID, err := ikecrypto.PRF(ikecrypto.PRF_HMAC_SHA2_256, sa.keys.SK_pi, identityBody)
	if err != nil {
		return nil, fmt.Errorf("MACedIDForI: %w", err)
	}
	signed := make([]byte, 0, len(initRequest)+len(nr)+len(macedID))
	signed = append(signed, initRequest...)
	signed = append(signed, nr...)
	signed = append(signed, macedID...)
	padKey, err := ikecrypto.PRF(ikecrypto.PRF_HMAC_SHA2_256, []byte(malformedPSK), []byte(malformedKeyPad))
	if err != nil {
		return nil, fmt.Errorf("AUTH pad key: %w", err)
	}
	authData, err := ikecrypto.PRF(ikecrypto.PRF_HMAC_SHA2_256, padKey, signed)
	if err != nil {
		return nil, fmt.Errorf("AUTH: %w", err)
	}

	espSPI := make([]byte, malformedSPIOctets)
	if _, err := rand.Read(espSPI); err != nil {
		return nil, fmt.Errorf("ESP SPI: %w", err)
	}
	inner := malformedInnerChain([]wire.Payload{
		identity,
		&wire.PayloadAUTH{AuthMethod: wire.AuthMethodPSK, AuthData: authData},
		malformedESPProposal(espSPI),
		malformedHostSelector(wire.PayloadTypeTSi, corruptRelayAddr),
		malformedHostSelector(wire.PayloadTypeTSr, malformedZeAddr),
	})
	return sa.seal(wire.ExchangeIKEAuth, wire.FlagInitiator, 1, wire.PayloadTypeIDi, inner)
}

// malformedHostSelector is a traffic selector payload naming one IPv4 host, every
// protocol and every port: the endpoints themselves, as a peer with no tunnel prefixes
// protects.
func malformedHostSelector(payloadType uint8, host netip.Addr) *wire.PayloadTS {
	address := host.AsSlice()
	return &wire.PayloadTS{TSPayloadType: payloadType, TrafficSelectors: []wire.TrafficSelector{{
		TSType:       wire.TSTypeIPv4AddrRange,
		StartPort:    0,
		EndPort:      65535,
		StartAddress: address,
		EndAddress:   address,
	}}}
}

// malformedAuthAnswer answers an error unless the IKE_AUTH response carries the
// responder's AUTH payload, naming any error notification it carries instead.
func malformedAuthAnswer(payloads []wire.PayloadEntry) error {
	for _, entry := range payloads {
		if notify, ok := entry.Payload.(*wire.PayloadNotify); ok && wire.NotifyIsError(notify.NotifyMsgType) {
			return fmt.Errorf("IKE_AUTH refused by the responder with %s: check its pre-shared-secret, esp-group and peer addresses against the peer's", wire.NotifyTypeName(notify.NotifyMsgType))
		}
	}
	for _, entry := range payloads {
		if entry.Payload.Type() == wire.PayloadTypeAUTH {
			return nil
		}
	}
	return fmt.Errorf("IKE_AUTH response carries %d payloads and no AUTH payload: the responder did not authenticate", len(payloads))
}

// liveness sends one empty INFORMATIONAL request and waits for its sealed answer: the
// barrier that shows the responder handles inbound messages on the established SA
// before the malformed response is sent.
//
// RFC 7296 Section 1: "An INFORMATIONAL request with no payloads (other than the
// empty Encrypted payload required by the syntax) is commonly used as a check for
// liveness."
func (sa *malformedPeerSA) liveness(ctx context.Context) error {
	request, err := sa.seal(wire.ExchangeInformational, wire.FlagInitiator, malformedLivenessMsgID, 0, nil)
	if err != nil {
		return err
	}
	reply, err := sa.exchange(ctx, request, wire.ExchangeInformational, malformedLivenessMsgID, malformedAttempts)
	if err != nil {
		return fmt.Errorf("INFORMATIONAL liveness check: %w", err)
	}
	if _, err := sa.open(reply); err != nil {
		return fmt.Errorf("INFORMATIONAL liveness response: %w", err)
	}
	return nil
}

// sendMalformedResponse sends one INFORMATIONAL response, sealed under the SA's keys,
// whose inner chain opens with a Notify payload header that claims 255 octets and is
// the last four octets of the plaintext. The integrity check verifies and
// wire.ParsePayloadChain refuses the chain, which is the errInnerParse outcome of
// openProtected (internal/component/ike/engine/protected_input.go).
func (sa *malformedPeerSA) sendMalformedResponse() error {
	truncated := []byte{0, 0, 0, 0xff}
	response, err := sa.seal(wire.ExchangeInformational, wire.FlagInitiator|wire.FlagResponse, malformedResponseMsgID, wire.PayloadTypeNotify, truncated)
	if err != nil {
		return err
	}
	if _, err := sa.conn.WriteToUDPAddrPort(response, sa.ze); err != nil {
		return fmt.Errorf("send the malformed response to %s: %w", sa.ze, err)
	}
	return nil
}

// serve answers every INFORMATIONAL request the responder sends on the SA with an empty
// sealed response, so its liveness checks find the peer alive, until the context ends or
// malformedLifetimeMax passes. A datagram that is not such a request is ignored.
func (sa *malformedPeerSA) serve(ctx context.Context) error {
	deadline := time.Now().Add(malformedLifetimeMax)
	answered := 0
	// Bounded by the context and by deadline: each pass waits at most malformedReadSlice.
	for ctx.Err() == nil && time.Now().Before(deadline) {
		datagram, got, err := sa.read(malformedReadSlice)
		if err != nil {
			return err
		}
		if !got {
			continue
		}
		var header wire.Header
		if err := header.ReadFrom(datagram); err != nil {
			continue
		}
		if header.Flags&wire.FlagResponse != 0 {
			continue
		}
		if header.ExchangeType != wire.ExchangeInformational {
			continue
		}
		if header.InitiatorSPI != sa.spiI {
			continue
		}
		// RFC 7296 Section 2.4: a message that fails its integrity check draws nothing.
		if _, err := sa.open(datagram); err != nil {
			continue
		}
		response, err := sa.seal(wire.ExchangeInformational, wire.FlagInitiator|wire.FlagResponse, header.MessageID, 0, nil)
		if err != nil {
			return err
		}
		if _, err := sa.conn.WriteToUDPAddrPort(response, sa.ze); err != nil {
			return fmt.Errorf("answer the responder's INFORMATIONAL request %d: %w", header.MessageID, err)
		}
		answered++
	}
	fmt.Println("PEER-SUMMARY: answered", answered, "INFORMATIONAL requests from the responder")
	return nil
}

// exchange sends request and answers a copy of the first response from the responder
// that carries this SA's initiator SPI, exchange and Message ID. It sends again after
// each malformedReplyWait with no answer, attempts times at most. A datagram that is not
// the answer is ignored.
func (sa *malformedPeerSA) exchange(ctx context.Context, request []byte, exchange uint8, messageID uint32, attempts int) ([]byte, error) {
	for range attempts {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, err := sa.conn.WriteToUDPAddrPort(request, sa.ze); err != nil {
			return nil, fmt.Errorf("send to %s: %w", sa.ze, err)
		}
		waitEnd := time.Now().Add(malformedReplyWait)
		// Bounded by waitEnd: each read waits at most until it.
		for time.Now().Before(waitEnd) {
			datagram, got, err := sa.read(time.Until(waitEnd))
			if err != nil {
				return nil, err
			}
			if !got {
				continue
			}
			if malformedIsAnswer(datagram, sa.spiI, exchange, messageID) {
				return append([]byte(nil), datagram...), nil
			}
		}
	}
	return nil, fmt.Errorf("no answer from %s after %d sends %s apart: the responder is not running, or it dropped the request (its log names why)", sa.ze, attempts, malformedReplyWait)
}

// malformedIsAnswer reports whether datagram is the response to the request with this
// exchange and Message ID on the SA whose initiator SPI is spiI.
func malformedIsAnswer(datagram []byte, spiI [8]byte, exchange uint8, messageID uint32) bool {
	var header wire.Header
	if err := header.ReadFrom(datagram); err != nil {
		return false
	}
	if header.InitiatorSPI != spiI {
		return false
	}
	if header.Flags&wire.FlagResponse == 0 {
		return false
	}
	if header.ExchangeType != exchange {
		return false
	}
	return header.MessageID == messageID
}

// read answers the next datagram from the responder within wait, and false when none
// came: the wait ran out, or the datagram came from another source. The answer aliases
// sa.datagram, so it is valid until the next read. The socket is not connected, so an
// ICMP port-unreachable for a send that preceded the daemon's bind is never reported
// here (Linux reports one only on a connected UDP socket).
func (sa *malformedPeerSA) read(wait time.Duration) ([]byte, bool, error) {
	if err := sa.conn.SetReadDeadline(time.Now().Add(wait)); err != nil {
		return nil, false, fmt.Errorf("set read deadline: %w", err)
	}
	octets, source, err := sa.conn.ReadFromUDPAddrPort(sa.datagram)
	if err != nil {
		var timeout net.Error
		if errors.As(err, &timeout) && timeout.Timeout() {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read: %w", err)
	}
	if source != sa.ze {
		return nil, false, nil
	}
	return sa.datagram[:octets], true, nil
}

// seal builds one message whose only payload is the Encrypted payload, carrying inner
// (a payload chain whose first payload is of firstType), sealed with the initiator's
// keys.
//
// RFC 7296 Section 3.14: "Pad Length is the length of the Padding field." The padding
// makes the plaintext a multiple of the AES block, and the Integrity Checksum Data
// covers the message from the first octet of the header to the last octet of the
// ciphertext.
func (sa *malformedPeerSA) seal(exchange, flags uint8, messageID uint32, firstType uint8, inner []byte) ([]byte, error) {
	padOctets := (aes.BlockSize - (len(inner)+1)%aes.BlockSize) % aes.BlockSize
	plaintext := make([]byte, len(inner)+padOctets+1)
	copy(plaintext, inner)
	plaintext[len(plaintext)-1] = byte(padOctets)

	prefix := wire.HeaderLen + wire.GenericHeaderLen
	total := prefix + aes.BlockSize + len(plaintext) + malformedICVOctets
	out := make([]byte, total)
	header := wire.Header{
		InitiatorSPI: sa.spiI,
		ResponderSPI: sa.spiR,
		NextPayload:  wire.PayloadTypeSK,
		MajorVersion: 2,
		ExchangeType: exchange,
		Flags:        flags,
		MessageID:    messageID,
		Length:       uint32(total),
	}
	header.WriteTo(out, 0)
	encrypted := wire.GenericHeader{NextPayload: firstType, Length: uint16(total - wire.HeaderLen)}
	encrypted.WriteTo(out, wire.HeaderLen)

	iv := out[prefix : prefix+aes.BlockSize]
	if _, err := rand.Read(iv); err != nil {
		return nil, fmt.Errorf("IV: %w", err)
	}
	block, err := aes.NewCipher(sa.keys.SK_ei)
	if err != nil {
		return nil, fmt.Errorf("SK_ei cipher: %w", err)
	}
	ciphertext := out[prefix+aes.BlockSize : total-malformedICVOctets]
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, plaintext)

	icv, err := ikecrypto.ComputeIntegrity(sa.integ.ID, sa.keys.SK_ai, out[:total-malformedICVOctets])
	if err != nil {
		return nil, fmt.Errorf("integrity checksum: %w", err)
	}
	copy(out[total-malformedICVOctets:], icv)
	return out, nil
}

// open verifies and decrypts a message the responder sealed, and answers its inner
// payloads.
func (sa *malformedPeerSA) open(message []byte) ([]wire.PayloadEntry, error) {
	prefix := wire.HeaderLen + wire.GenericHeaderLen
	if len(message) < prefix+2*aes.BlockSize+malformedICVOctets {
		return nil, fmt.Errorf("message of %d octets is too short for an Encrypted payload", len(message))
	}
	var header wire.Header
	if err := header.ReadFrom(message); err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}
	if header.NextPayload != wire.PayloadTypeSK {
		return nil, fmt.Errorf("message's first payload is type %d, not the Encrypted payload", header.NextPayload)
	}
	body := len(message) - malformedICVOctets
	if err := ikecrypto.VerifyIntegrity(sa.integ.ID, sa.keys.SK_ar, message[:body], message[body:]); err != nil {
		return nil, fmt.Errorf("integrity check: %w", err)
	}
	plaintext, err := ikecrypto.DecryptAESCBCRaw(sa.keys.SK_er, message[prefix:body])
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	padOctets := int(plaintext[len(plaintext)-1])
	if padOctets+1 > len(plaintext) {
		return nil, fmt.Errorf("pad length %d exceeds the %d octet plaintext", padOctets, len(plaintext))
	}
	inner := plaintext[:len(plaintext)-1-padOctets]
	payloads, err := wire.ParsePayloadChain(inner, message[wire.HeaderLen])
	if err != nil {
		return nil, fmt.Errorf("inner payloads: %w", err)
	}
	return payloads, nil
}

// malformedInnerChain writes payloads as one payload chain, each behind its generic
// header (RFC 7296 Section 3.2), as an Encrypted payload carries it.
func malformedInnerChain(payloads []wire.Payload) []byte {
	octets := 0
	for _, payload := range payloads {
		octets += wire.GenericHeaderLen + payload.Len()
	}
	out := make([]byte, octets)
	off := 0
	for i, payload := range payloads {
		var generic wire.GenericHeader
		if i+1 < len(payloads) {
			generic.NextPayload = payloads[i+1].Type()
		}
		generic.Length = uint16(wire.GenericHeaderLen + payload.Len())
		generic.WriteTo(out, off)
		off += wire.GenericHeaderLen
		off += payload.WriteTo(out, off)
	}
	return out
}
