// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-TLS method handler
// RFC: rfc/short/rfc5216.md -- EAP-TLS: TLS handshake in EAP, fragmentation, MSK derivation

package eap

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// EAP-TLS flags byte (RFC 5216 Section 3).
const (
	eapTLSFlagL uint8 = 0x80 // Length included
	eapTLSFlagM uint8 = 0x40 // More fragments
	eapTLSFlagS uint8 = 0x20 // Start
)

const eapTLSFragmentSize = 1024
const eapTLSMaxReassembly = 65536

// eapTLSMaxPeerBuffered bounds the octets the EAP layer may hand the TLS engine
// before the engine has read them.
//
// A live handshake leaves nothing buffered between rounds: Process feeds the
// reassembled message, then waits for the engine to settle, and the engine
// settles only once it has consumed that message and parked in Read. So two
// messages' worth is the smallest ceiling a conformant exchange cannot reach --
// it leaves room for one whole undrained message plus the next full-size one --
// while still bounding the growth at a fixed 128 KiB.
//
// Past that the engine has stopped reading, which is what a failed handshake
// looks like: runTLSServer returns, and every later feed is pure accumulation.
// EAP-TLS runs before the peer is authenticated, so those octets come from an
// unauthenticated party and the refusal must be an error rather than more memory
// (ai/rules/evidence.md).
const eapTLSMaxPeerBuffered = 2 * eapTLSMaxReassembly

// tlsFragmenter holds fragmentation and reassembly state shared by
// both the EAP-TLS server (tlsMethod) and peer (PeerSession).
type tlsFragmenter struct {
	outBuf      []byte
	outOffset   int
	inBuf       []byte
	inExpected  int
	waitFragAck bool
}

// reassemble accumulates inbound TLS data from an EAP-TLS message.
// RFC 5216 Section 2.1.5: L flag on first fragment carries total length.
func (f *tlsFragmenter) reassemble(typeData []byte) error {
	if len(typeData) == 0 {
		return nil
	}
	flags := typeData[0]
	off := 1

	if flags&eapTLSFlagL != 0 {
		if len(typeData) < 5 {
			return fmt.Errorf("eap-tls: L flag set but message too short (%d bytes)", len(typeData))
		}
		totalLen := int(binary.BigEndian.Uint32(typeData[1:5]))
		if totalLen > eapTLSMaxReassembly {
			return fmt.Errorf("eap-tls: TLS message too large (%d bytes, limit %d)", totalLen, eapTLSMaxReassembly)
		}
		off = 5

		// RFC 5216 Section 2.1.5 REQUIRES the L flag on the first fragment and
		// permits it on the others, so a conformant peer can repeat it on every
		// fragment. Only the first one starts a message. Resetting the buffer on
		// each L kept the last fragment alone, reported no error, and handed
		// crypto/tls a truncated record.
		switch {
		case len(f.inBuf) > 0 && totalLen != f.inExpected:
			return fmt.Errorf("eap-tls: fragment re-declares length %d, but this message declared %d", totalLen, f.inExpected)
		case len(f.inBuf) == 0:
			f.inExpected = totalLen
			// Reuse existing buffer if capacity is sufficient.
			if cap(f.inBuf) >= totalLen {
				f.inBuf = f.inBuf[:0]
			} else {
				f.inBuf = make([]byte, 0, totalLen)
			}
		}
	}

	if off < len(typeData) {
		// Bound the buffer BEFORE it grows, whatever the peer declared. This
		// ceiling used to live inside the L-flag branch above, so a peer that
		// never set L left inExpected at 0, skipped the check, and grew this
		// buffer without any limit. EAP-TLS runs before the peer is
		// authenticated, so the octets arrive from an unauthenticated party
		// (ai/rules/evidence.md). Checking after the append would
		// still let each fragment overshoot by its own length.
		grown := len(f.inBuf) + len(typeData) - off
		if grown > eapTLSMaxReassembly {
			return fmt.Errorf("eap-tls: TLS message too large (%d bytes, limit %d)", grown, eapTLSMaxReassembly)
		}
		if f.inExpected > 0 && grown > f.inExpected {
			return fmt.Errorf("eap-tls: reassembled data (%d bytes) exceeds declared length (%d bytes)", grown, f.inExpected)
		}
		f.inBuf = append(f.inBuf, typeData[off:]...)
	}

	return nil
}

// reassemblyComplete reports whether every declared octet has arrived.
//
// A peer signals the end of a fragmented message by clearing the M flag, but
// that says only "I have stopped sending", never "you have it all". Feeding a
// short buffer to crypto/tls produces the opaque "local error: tls: error
// decoding message" several layers away from the cause, so the caller checks
// this before it drains (ai/rules/evidence.md).
//
// A message that declared no length has nothing to check against and is
// reported complete: RFC 5216 Section 2.1.5 requires the L flag only when a
// message is fragmented.
func (f *tlsFragmenter) reassemblyComplete() bool {
	return f.inExpected == 0 || len(f.inBuf) == f.inExpected
}

// drainReassembled returns the reassembled inbound data and resets length tracking.
// The caller must consume the returned slice before the next reassemble call.
func (f *tlsFragmenter) drainReassembled() []byte {
	data := f.inBuf
	f.inBuf = f.inBuf[:0]
	f.inExpected = 0
	return data
}

// startSending begins outbound fragmentation of data.
func (f *tlsFragmenter) startSending(data []byte) {
	f.outBuf = data
	f.outOffset = 0
}

// nextFragment returns the next outbound fragment as EAP-TLS TypeData.
// RFC 5216 Section 2.1.5: first fragment has L+M, middle has M, last has neither.
func (f *tlsFragmenter) nextFragment() []byte {
	remaining := len(f.outBuf) - f.outOffset
	if remaining <= 0 {
		return []byte{0}
	}

	isFirst := f.outOffset == 0
	chunkSize := min(remaining, eapTLSFragmentSize)
	isLast := f.outOffset+chunkSize >= len(f.outBuf)

	var flags uint8
	headerSize := 1
	if isFirst {
		flags |= eapTLSFlagL
		headerSize = 5
	}
	if !isLast {
		flags |= eapTLSFlagM
		f.waitFragAck = true
	}

	td := make([]byte, headerSize+chunkSize)
	td[0] = flags
	if isFirst {
		binary.BigEndian.PutUint32(td[1:5], uint32(len(f.outBuf)))
	}
	copy(td[headerSize:], f.outBuf[f.outOffset:f.outOffset+chunkSize])
	f.outOffset += chunkSize

	if isLast {
		f.outBuf = nil
		f.outOffset = 0
	}

	return td
}

// EAP-TLS key derivation labels. The label and the exporter inputs depend on the
// negotiated TLS version, so a single label is wrong for one of them.
//
// RFC 5216 Section 2.3 (TLS 1.2 and below):
//
//	MSK = TLS-PRF(master_secret, "client EAP encryption",
//	              client.random || server.random)[0..63]
//
// RFC 9190 Section 2.3 (TLS 1.3) replaces it, because TLS 1.3 has no
// master_secret to run the old PRF over:
//
//	Key_Material = TLS-Exporter("EXPORTER_EAP_TLS_Key_Material", Type-Code, 128)
//	MSK          = Key_Material[0..63]
const (
	eapTLSLabelRFC5216 = "client EAP encryption"
	eapTLSLabelRFC9190 = "EXPORTER_EAP_TLS_Key_Material"

	// eapTLSKeyMaterialLen is the RFC 9190 export length: 64 octets of MSK
	// followed by 64 of EMSK. Ze uses the MSK half.
	eapTLSKeyMaterialLen = 128
)

// eapTLSTypeCode is the EAP-TLS method type, used verbatim as the RFC 9190
// exporter context.
var eapTLSTypeCode = []byte{TypeTLS}

// exportEAPTLSMSK derives the EAP-TLS MSK from a completed TLS connection,
// choosing the derivation the negotiated TLS version defines.
//
// It returns an error rather than a zero MSK when the key cannot be exported. An
// all-zero MSK is a valid-looking answer the caller cannot tell from a real key:
// ze would compute its IKEv2 AUTH payload (RFC 7296 Section 2.16) over 64 zero
// octets, and two ends that both failed this way would agree on zeros and
// authenticate nothing (ai/rules/evidence.md).
func exportEAPTLSMSK(cs tls.ConnectionState) ([64]byte, error) {
	var msk [64]byte
	if !cs.HandshakeComplete {
		return msk, errors.New("eap-tls: TLS handshake did not complete, so no MSK exists")
	}

	if cs.Version >= tls.VersionTLS13 {
		material, err := cs.ExportKeyingMaterial(eapTLSLabelRFC9190, eapTLSTypeCode, eapTLSKeyMaterialLen)
		if err != nil {
			return msk, fmt.Errorf("eap-tls: export key material (RFC 9190 Section 2.3): %w", err)
		}
		copy(msk[:], material[:64])
		return msk, nil
	}

	exported, err := cs.ExportKeyingMaterial(eapTLSLabelRFC5216, nil, 64)
	if err != nil {
		return msk, eapTLS12ExportRefused(cs, err)
	}
	copy(msk[:], exported)
	return msk, nil
}

// eapTLS12ExportRefused explains a refused TLS 1.2 key material export to the
// operator who reads the log.
//
// crypto/tls refuses the export whenever the session is neither TLS 1.3 nor
// carries the RFC 7627 extended master secret (Conn.connectionStateLocked selects
// noEKMBecauseNoEMS on `c.vers != VersionTLS13 && !c.extMasterSecret`). RFC 5216
// Section 2.3 defines the EAP-TLS MSK as that export, so the peer cannot
// authenticate. strongSwan 5.9.14 lands here by DEFAULT rather than by
// limitation: charon ships `version_max = 1.2` and negotiates no RFC 7627, but
// `charon.tls.version_max = 1.3` on the same build reaches an established SA
// (test/interop-ipsec/scenarios/06-eap-tls13). The first remedy in the message
// below is therefore a peer config edit, not a peer upgrade.
//
// The message carries the remedies because there is no longer a way to override
// the refusal. Go 1.27 removed the tlsunsafeekm setting, and the runtime raises a
// fatal error before main() when that key is set to its old value, so an operator
// who reaches for it stops the daemon instead of starting a session
// (cmd/ze/main.go states the same thing for a reader of the source).
//
// The wrapped error keeps the crypto/tls sentence, which names the refusal ze
// actually met rather than the one ze expects.
func eapTLS12ExportRefused(cs tls.ConnectionState, err error) error {
	return fmt.Errorf(
		"eap-tls: cannot export the RFC 5216 Section 2.3 MSK for peer %s on %s. "+
			"The export needs TLS 1.3, or a TLS 1.2 session that negotiated the RFC 7627 extended master secret. "+
			"Move the peer to TLS 1.3 (RFC 9190), add RFC 7627 to its TLS 1.2 stack, or configure another EAP method: %w",
		eapTLSPeerName(cs), tls.VersionName(cs.Version), err)
}

// eapTLSPeerName names the other end of the EAP-TLS exchange for a log line.
//
// Both roles require a certificate from the other end, so a completed handshake
// has one: the authenticator sets RequireAndVerifyClientCert (newTLSMethod) and
// the peer verifies the authenticator chain (PeerSession). The empty case is
// therefore unreachable today, and it answers a placeholder rather than an empty
// string so a log line never reads as if ze knew the peer and found no name.
func eapTLSPeerName(cs tls.ConnectionState) string {
	if len(cs.PeerCertificates) == 0 {
		return "(no certificate)"
	}
	return cs.PeerCertificates[0].Subject.String()
}

type tlsState uint8

const (
	tlsStateStart tlsState = iota
	tlsStateHandshake
	tlsStateDone
)

type tlsMethod struct {
	tlsFragmenter
	tlsConfig  *tls.Config
	state      tlsState
	conn       *tls.Conn
	transport  *eapTLSTransport
	handshaked atomic.Bool

	// alertSent holds the handshake failure whose fatal TLS alert has ALREADY gone
	// out as an EAP-Request, so the round that follows can report it.
	//
	// RFC 5216 Section 2.1.3 ends a rejected handshake over two rounds, and one
	// MethodResult cannot hold both halves (see the handshakeError branch in
	// Process). It is written and read on the dispatch goroutine alone, exactly
	// like state and the fragmenter fields beside it, so it needs no atomic.
	alertSent error

	// successIndicated records that the RFC 9190 Section 2.5 protected success
	// result indication has already gone out. Step 3 of that procedure forbids
	// any further EAP-Request once it has been sent, so the write happens on
	// exactly one round. Written and read on the dispatch goroutine alone, like
	// state, alertSent and the fragmenter fields beside it.
	successIndicated bool

	// started publishes the transport to Close, which runs on the session's
	// owner goroutine while Start and Process run on the dispatch goroutine.
	// Start assigns transport BEFORE it stores this, so a Close that reads true
	// is guaranteed to see the assignment.
	started atomic.Bool
}

// eapTLSSuccessIndication is the payload RFC 9190 Section 2.5 defines as the
// protected success result indication: one octet of application data, 0x00,
// carried in an encrypted TLS record.
var eapTLSSuccessIndication = []byte{0x00}

func newTLSMethod(config MethodConfig) (*tlsMethod, error) {
	cert, err := tls.X509KeyPair(config.ServerCertPEM, config.ServerKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("eap-tls: load server cert: %w", err)
	}

	pool := x509.NewCertPool()
	if len(config.CACertPEM) > 0 {
		if !pool.AppendCertsFromPEM(config.CACertPEM) {
			return nil, errors.New("eap-tls: failed to parse CA certificate")
		}
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
		MinVersion:   tls.VersionTLS12,

		// ISSUE NO SESSION TICKET, BECAUSE NOTHING COULD EVER REDEEM ONE.
		//
		// This function builds a fresh tls.Config for every EAP session, and Go
		// keys ticket encryption on the Config instance: Config.ticketKeys
		// (crypto/tls/common.go) reads c.sessionTicketKeys, then falls back to
		// c.autoSessionTicketKeys, which it fills with 32 random octets the first
		// time a ticket is issued. Neither SessionTicketKey nor
		// SetSessionTicketKeys is set here, so each session gets its own random
		// key and a ticket minted under one is undecryptable under every other.
		//
		// Left on, ze would emit a NewSessionTicket the peer stores, offers on its
		// next EAP-TLS exchange, and is always refused for. Turning it off makes
		// that dead end EXPLICIT: six RFC 9190 MUSTs are conditional on resumption
		// (5.6-2, 5.7-1, 5.7-2, 5.7-3, 5.7-4, 5.7-6) and are unreachable while
		// this line stands. Should a shared ticket key ever be introduced, this
		// line must be removed deliberately, and removing it arms those six
		// obligations in the same commit rather than silently.
		SessionTicketsDisabled: true,
	}

	return &tlsMethod{
		tlsConfig: tlsCfg,
		state:     tlsStateStart,
	}, nil
}

func (m *tlsMethod) Type() uint8 { return TypeTLS }

// Start sends the EAP-TLS Start request (S flag set, no TLS data).
// RFC 5216 Section 2.1: server initiates with EAP-Request/EAP-TLS with S flag.
func (m *tlsMethod) Start(identifier uint8) *Packet {
	m.transport = newEAPTLSTransport()
	// Publish the transport BEFORE the goroutine exists. Close reads this flag
	// from the session's owner goroutine, and a Close that observed a started
	// method with no transport yet would return without releasing the goroutine
	// this line is about to start.
	m.started.Store(true)
	m.state = tlsStateHandshake

	go m.runTLSServer()

	return &Packet{
		Code:       CodeRequest,
		Identifier: identifier,
		Type:       TypeTLS,
		TypeData:   []byte{eapTLSFlagS},
	}
}

// Process handles an EAP-Response/EAP-TLS from the peer.
// RFC 5216 Section 2.1.5: handles fragment reassembly and outbound fragmentation.
func (m *tlsMethod) Process(response *Packet) MethodResult {
	if response.Type != TypeTLS {
		// A rejected peer must not be able to replace the reported cause by changing
		// one octet. Once the alert is on the wire the exchange is over, so the
		// failure to report is the handshake's, not this packet's type: answering
		// ErrMethodFailed here would hand the operator "eap: method authentication
		// failed" in place of the certificate error, which is the substitution
		// m.alertSent exists to prevent.
		if m.alertSent != nil {
			return MethodResult{Err: m.alertSent}
		}
		return MethodResult{Err: ErrMethodFailed}
	}

	// If we sent a fragment with M flag and are waiting for an ACK,
	// the peer's response is a fragment ACK (empty TLS data). Send next fragment.
	if m.waitFragAck {
		m.waitFragAck = false
		return MethodResult{
			Response: &Packet{Code: CodeRequest, Type: TypeTLS, TypeData: m.nextFragment()},
		}
	}

	// The fatal alert went out on an earlier round, and this response is the reply
	// RFC 5216 Section 2.1.3 makes the server wait for: "To ensure that the peer
	// receives the TLS alert message, the EAP server MUST wait for the peer to
	// reply with an EAP-Response packet." Reporting the failure here is what turns
	// into the EAP-Failure the same paragraph then makes mandatory
	// (Session.handleMethod, eap.go).
	//
	// It sits AFTER the fragment-ACK branch above so a fragmented alert finishes
	// going out first, and BEFORE the reassembly below so nothing more is fed to an
	// engine that has already stopped reading.
	//
	// Ze does not implement the OPTIONAL restart. The section leaves that to the
	// server -- "It is up to the EAP server whether to allow restarts" -- so a reply
	// carrying a fresh client_hello ends the conversation exactly as an empty one
	// does.
	if m.alertSent != nil {
		return MethodResult{Err: m.alertSent}
	}

	// Reassemble inbound fragments from peer.
	if err := m.reassemble(response.TypeData); err != nil {
		return MethodResult{Err: err}
	}

	// If peer set M flag, more fragments coming. Send ACK (empty EAP-TLS).
	if len(response.TypeData) > 0 && response.TypeData[0]&eapTLSFlagM != 0 {
		return MethodResult{
			Response: &Packet{Code: CodeRequest, Type: TypeTLS, TypeData: []byte{0}},
		}
	}

	// The peer cleared the M flag, so it has stopped sending. Refuse a message
	// that is shorter than the length it declared rather than passing the short
	// buffer to crypto/tls.
	if !m.reassemblyComplete() {
		return MethodResult{Err: fmt.Errorf("eap-tls: peer ended a TLS message after %d of %d declared bytes", len(m.inBuf), m.inExpected)}
	}

	// All fragments received. Feed reassembled data to TLS.
	if data := m.drainReassembled(); len(data) > 0 {
		if err := m.transport.feedPeerData(data); err != nil {
			return MethodResult{Err: err}
		}
	}

	// Read TLS engine output, waiting for the engine to settle so the whole
	// flight goes out in one EAP message.
	serverData := m.transport.waitServerData()

	// Report a rejected handshake as the failure it is, and report it with its
	// CAUSE. runTLSServer records the engine's error and returns, so from here on
	// the engine produces nothing: without this check waitServerData keeps
	// answering empty, the branch below keeps sending a bare fragment ACK, and a
	// peer whose certificate was refused is ACKed until the session reaper fires
	// 30s later while feedPeerData accumulates everything it sends. Placed before
	// the handshaked branch because a failed handshake never sets that flag, so
	// the alternative report is the "no MSK exists" consequence rather than the
	// certificate error that caused it.
	if err := m.transport.handshakeError(); err != nil {
		failure := fmt.Errorf("eap-tls: TLS handshake failed: %w", err)

		// SEND THE ALERT NOW, REPORT THE FAILURE ON THE NEXT ROUND. serverData holds
		// the fatal TLS alert the engine produced in this same round.
		//
		// Returning the alert BESIDE the error puts it nowhere. MethodResult carries
		// both fields, but Session.handleMethod (eap.go) tests Err first and answers
		// with s.failure(), so the Response is discarded and the alert octets never
		// reach the wire. The two are therefore mutually exclusive here, and the
		// cause is parked on m.alertSent until the round that may report it.
		//
		// RFC 5216 Section 2.1.3 spends two rounds on this deliberately. The server
		// SHOULD send the alert "so as to allow the peer to inform the user or log
		// the cause of the failure", it "MUST wait for the peer to reply with an
		// EAP-Response packet", and only then "MUST send an EAP-Failure packet and
		// terminate the conversation". Collapsing that into one round drops the
		// diagnostic the whole section exists to deliver.
		if len(serverData) > 0 {
			m.alertSent = failure
			m.startSending(serverData)
			return MethodResult{
				Response: &Packet{Code: CodeRequest, Type: TypeTLS, TypeData: m.nextFragment()},
			}
		}

		// The engine failed without producing an alert, so there is nothing for the
		// peer to receive and nothing for it to answer. Waiting would leave the
		// exchange to the stale-handshake reaper, which is the regression this
		// handshake-error read removed, so the failure is reported at once.
		return MethodResult{Err: failure}
	}

	if m.handshaked.Load() {
		// Send the engine's closing flight before concluding. Go's TLS 1.2
		// server writes ChangeCipherSpec and Finished and only THEN returns from
		// HandshakeContext, so those octets are produced in the very round that
		// sets handshaked. Returning Done here dropped the server Finished, the
		// peer waited for it forever, and its MSK stayed zero while ours did
		// not: the two IKEv2 AUTH payloads (RFC 7296 Section 2.16) were then
		// computed over keys the ends did not share. The peer side documents the
		// same ordering in handleTLSRequest; this is its mirror.
		//
		// On TLS 1.3 the protected success result indication rides in that same
		// flight, so the exchange keeps the round count it had before.
		indication, err := m.indicateSuccess()
		if err != nil {
			return MethodResult{Err: err}
		}
		serverData = append(serverData, indication...)

		if len(serverData) > 0 {
			m.startSending(serverData)
			return MethodResult{
				Response: &Packet{Code: CodeRequest, Type: TypeTLS, TypeData: m.nextFragment()},
			}
		}
		m.state = tlsStateDone
		msk, err := m.deriveMSK()
		if err != nil {
			return MethodResult{Err: err}
		}
		return MethodResult{MSK: msk, Done: true}
	}

	// The peer half reports a stall here instead of an ACK (readAndSendTLS,
	// peer.go, errTLSClientStalled). This half MUST NOT. The two branches above
	// already turn every state runTLSServer leaves into an error, so what remains
	// is a defensive path. The ACK also keeps the eapTLSMaxPeerBuffered ceiling
	// reachable. feedPeerData runs BEFORE waitServerData, so an error here ends
	// the exchange on the first message. No second message reaches the ceiling
	// (TestEAPTLSProcessRefusesUnboundedPeerBuffer).
	if len(serverData) == 0 {
		return MethodResult{
			Response: &Packet{Code: CodeRequest, Type: TypeTLS, TypeData: []byte{0}},
		}
	}

	// Start sending (possibly fragmented) server TLS data.
	m.startSending(serverData)
	return MethodResult{
		Response: &Packet{Code: CodeRequest, Type: TypeTLS, TypeData: m.nextFragment()},
	}
}

// indicateSuccess writes the RFC 9190 Section 2.5 protected success result
// indication and returns the ciphertext the TLS record layer produced for it.
// It answers nil on every round but one.
//
// RFC 9190 Section 2.5: "When an EAP-TLS server has successfully processed the
// TLS client Finished and sent its last handshake message (Finished or a
// post-handshake message), it sends an encrypted TLS record with application
// data 0x00. The encrypted TLS record with application data 0x00 is a protected
// success result indication."
//
// THE VERSION TEST READS THE NEGOTIATED VERSION, NOT THE CONFIGURED ONE. The
// section is new against RFC 5216 and "only applies to TLS 1.3", so a TLS 1.2
// exchange must conclude with the bare EAP-Success it concluded with before.
// TestEAPTLS12SendsNoProtectedSuccessIndication (rfc9190_test.go) is that proof:
// it drives a TLS 1.2 flight and asserts the authenticator sends no
// application_data record at all. The interop lab cannot prove it, because ze is
// the EAP PEER in every TLS 1.2 scenario and this function runs on the
// authenticator side. tlsConfig.MinVersion is TLS 1.2 and says nothing about
// what this session settled on.
//
// THE CALLER MUST HAVE OBSERVED m.handshaked, which is what satisfies
// RFC9190-2.5-2, the MUST NOT against sending the indication early. runTLSServer
// stores that flag only after HandshakeContext returns, and Go's TLS 1.3 server
// writes its whole post-handshake flight inside that call: sendSessionTickets is
// invoked from readClientCertificate (crypto/tls/handshake_server_tls13.go),
// which runs before readClientFinished. So a round that sees handshaked has both
// processed the client Finished and sent the last handshake message.
//
// It is written ONCE. Step 3 of the procedure says the server "must not send any
// more EAP-Requests and may only send an EAP-Success" after the request carrying
// the indication, so a second write would be a violation rather than a retry.
func (m *tlsMethod) indicateSuccess() ([]byte, error) {
	if m.successIndicated || m.conn == nil {
		return nil, nil
	}
	if m.conn.ConnectionState().Version < tls.VersionTLS13 {
		return nil, nil
	}

	// tls.Conn.Write applies the record layer, so what reaches the transport is
	// the ENCRYPTED record the section asks for. Writing the octet to the
	// transport directly would put a bare 0x00 on the wire and protect nothing.
	if _, err := m.conn.Write(eapTLSSuccessIndication); err != nil {
		return nil, fmt.Errorf("eap-tls: write the RFC 9190 Section 2.5 protected success indication: %w", err)
	}
	m.successIndicated = true

	// The engine goroutine has already returned, so the transport is settled and
	// this collects the record Write just produced without waiting for one.
	return m.transport.waitServerData(), nil
}

// Close releases the TLS engine goroutine Start launched.
//
// runTLSServer parks in eapTLSTransport.Read whenever it has consumed every
// octet the peer sent, and Read returns from that park only after the transport
// is closed. Every exchange that ends before the handshake completes therefore
// strands that goroutine, along with the tls.Conn and the handshake secrets it
// holds. The peer chooses when to stop answering, and it is unauthenticated
// until the exchange finishes, so the count is the peer's to pick.
//
// Idempotent: eapTLSTransport.Close only sets a flag under its mutex. Safe
// before Start, when there is no transport and no goroutine to release.
func (m *tlsMethod) Close() {
	if !m.started.Load() {
		return
	}
	m.transport.shutdown()
}

func (m *tlsMethod) runTLSServer() {
	m.conn = tls.Server(m.transport, m.tlsConfig)
	err := m.conn.HandshakeContext(context.Background())
	if err != nil {
		m.transport.setError(err)
		return
	}
	// Publish the outcome BEFORE the wakeup: handshakeFinished releases a
	// waiter in Process, which reads handshaked to decide the method is done.
	m.handshaked.Store(true)
	m.transport.handshakeFinished()
}

// deriveMSK derives the authenticator's MSK from the completed TLS connection.
//
// A failed export is an error, never a substitute key. This used to fall back to
// sha256(TLSUnique), which no other implementation computes: the peer derives the
// RFC 5216 MSK, the two keys disagree, and the IKEv2 AUTH payload built from them
// (RFC 7296 Section 2.16) fails to verify with no usable reason. A locally
// invented key is indistinguishable from a real one at the call site, which is
// what makes it dangerous (ai/rules/evidence.md).
func (m *tlsMethod) deriveMSK() ([64]byte, error) {
	if m.conn == nil {
		return [64]byte{}, errors.New("eap-tls: no TLS connection to derive the MSK from")
	}
	return exportEAPTLSMSK(m.conn.ConnectionState())
}

// notifyCh sends a non-blocking signal on a buffered channel.
// The channel is a level-style wakeup for a waiter that re-reads all available
// buffered data on each loop, so at most one pending token is ever needed.
//
// This MUST use default (skip only when a token is already queued), NOT
// case <-time.After(0): time.After(0) is essentially always ready, so the
// select would choose at random and drop roughly half of all wakeups even when
// the buffer has space -- permanently parking a Read blocked on <-ch and
// deadlocking the EAP-TLS handshake.
func notifyCh(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default: // A wakeup is already queued; the waiter will observe the latest data.
	}
}

// eapTLSSettleBackstop bounds waitServerData against a TLS engine that neither
// reads, writes, nor returns. It is a BACKSTOP, never the primary signal: the
// engine settles in microseconds through readIdle, finished, closed or err, so a
// fire here means the engine is wedged and the caller is better served by the
// empty answer than by a blocked IKE exchange.
const eapTLSSettleBackstop = 2 * time.Second

// eapTLSTransport implements net.Conn for piping TLS through EAP packets.
//
// It carries a wakeup in each direction. peerCh wakes the TLS engine when EAP
// delivers peer bytes. serverCh wakes the EAP side when the engine produces
// bytes or stops producing them. Both are needed: the EAP side must send the
// engine's whole flight in one EAP message, and it cannot know the flight is
// complete from a buffer snapshot, because an empty buffer reads the same
// whether the engine has finished writing or has not started.
type eapTLSTransport struct {
	mu        sync.Mutex
	peerBuf   []byte
	serverBuf []byte
	peerCh    chan struct{}
	serverCh  chan struct{}
	err       error
	closed    bool

	// readIdle records that the TLS engine is parked in Read with no input
	// available. The engine writes its whole flight before it reads again, so
	// readIdle is the signal that the flight is complete.
	readIdle bool

	// finished records that the TLS engine goroutine has returned. It produces
	// no further output, so a waiter must stop waiting for any.
	finished bool
}

func newEAPTLSTransport() *eapTLSTransport {
	return &eapTLSTransport{
		peerCh:   make(chan struct{}, 1),
		serverCh: make(chan struct{}, 1),
	}
}

// feedPeerData hands the TLS engine octets the peer sent.
//
// It REFUSES data that would grow the unread backlog past
// eapTLSMaxPeerBuffered and returns an error saying so, rather than appending.
// The caller MUST report that error and end the exchange: an engine that has
// stopped reading never drains what is already queued, so continuing would
// accumulate whatever an unauthenticated peer chooses to send
// (ai/rules/evidence.md).
func (t *eapTLSTransport) feedPeerData(data []byte) error {
	t.mu.Lock()
	// Check BEFORE the append, so the refused message is never held at all.
	if grown := len(t.peerBuf) + len(data); grown > eapTLSMaxPeerBuffered {
		buffered := len(t.peerBuf)
		t.mu.Unlock()
		return fmt.Errorf("eap-tls: peer sent %d more bytes with %d still unread by the TLS engine, over the %d byte limit", len(data), buffered, eapTLSMaxPeerBuffered)
	}
	t.peerBuf = append(t.peerBuf, data...)
	// Input is available, so the engine is no longer idle. Clearing this here
	// rather than in Read closes a race: the engine clears it only once it is
	// scheduled, and a waiter that ran first would read the stale idle set by
	// the previous park and return before the engine answered.
	t.readIdle = false
	t.mu.Unlock()
	notifyCh(t.peerCh)
	return nil
}

// handshakeError returns the error the TLS engine goroutine recorded, or nil.
//
// waitServerData settles on a FAILED handshake exactly as readily as on a
// finished flight, and its return value cannot tell the two apart: both can be
// empty. A caller that does not read this therefore treats "the handshake was
// rejected" as "the engine produced nothing this round" and answers with a bare
// fragment ACK forever (ai/rules/evidence.md). This mirrors the peer
// side, which keeps the same error in PeerSession.tlsErr and reports it from
// deriveTLSMSK.
func (t *eapTLSTransport) handshakeError() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.err
}

// waitServerData blocks until the TLS engine settles, then returns everything it
// produced. The engine has settled when it parks in Read with no input, when its
// goroutine has returned, when the transport is closed, or when it failed.
//
// A snapshot of serverBuf cannot answer this question. Reading it without the
// wait returns an empty slice while the engine is still building the flight,
// which the EAP layer then sends as a bare fragment ACK. RFC 5216 Section 2.1.5
// permits that ACK only in answer to a message carrying the M flag, so a peer
// refuses it and the method fails before any TLS record crosses.
func (t *eapTLSTransport) waitServerData() []byte {
	backstop := time.NewTimer(eapTLSSettleBackstop)
	defer backstop.Stop()

	for {
		t.mu.Lock()
		if t.readIdle || t.finished || t.closed || t.err != nil {
			data := t.serverBuf
			t.serverBuf = nil
			t.mu.Unlock()
			return data
		}
		t.mu.Unlock()

		select {
		case <-t.serverCh:
		case <-backstop.C:
			t.mu.Lock()
			data := t.serverBuf
			t.serverBuf = nil
			t.mu.Unlock()
			return data
		}
	}
}

// setError records a failed TLS handshake. The engine goroutine returns on this
// path, so it also marks the engine finished.
func (t *eapTLSTransport) setError(err error) {
	t.mu.Lock()
	t.err = err
	t.finished = true
	t.mu.Unlock()
	notifyCh(t.serverCh)
}

// handshakeFinished marks the TLS engine goroutine as returned. The caller MUST
// publish the handshake outcome (the tlsDone / handshaked flag) BEFORE it calls
// this: the call is the wakeup that lets a waiter observe the outcome.
func (t *eapTLSTransport) handshakeFinished() {
	t.mu.Lock()
	t.finished = true
	t.mu.Unlock()
	notifyCh(t.serverCh)
}

// Read implements net.Conn.Read (called by TLS to read peer data).
func (t *eapTLSTransport) Read(p []byte) (int, error) {
	for {
		t.mu.Lock()
		if len(t.peerBuf) > 0 {
			n := copy(p, t.peerBuf)
			t.peerBuf = t.peerBuf[n:]
			t.readIdle = false
			t.mu.Unlock()
			return n, nil
		}
		if t.closed {
			t.readIdle = false
			t.mu.Unlock()
			return 0, io.EOF
		}
		// The engine is about to block with no input, so it has written
		// everything it can for this flight. Publish that before releasing the
		// lock, then wake the EAP side.
		t.readIdle = true
		t.mu.Unlock()
		notifyCh(t.serverCh)
		<-t.peerCh
	}
}

// Write implements net.Conn.Write (called by TLS to send server data).
func (t *eapTLSTransport) Write(p []byte) (int, error) {
	t.mu.Lock()
	t.serverBuf = append(t.serverBuf, p...)
	t.mu.Unlock()
	notifyCh(t.serverCh)
	return len(p), nil
}

// shutdown releases the TLS engine goroutine and every waiter on this
// transport. Read answers io.EOF once closed is set, and the two wakeups below
// are what lets a parked reader or waiter observe it.
//
// It cannot fail, so it returns nothing: the net.Conn Close below exists only
// to satisfy the interface, and an error return there would make every internal
// caller either check a value that is always nil or suppress it.
//
// Idempotent. Safe to call from any goroutine, and safe to call more than once.
func (t *eapTLSTransport) shutdown() {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()
	notifyCh(t.peerCh)
	notifyCh(t.serverCh)
}

// Close implements net.Conn.Close.
func (t *eapTLSTransport) Close() error {
	t.shutdown()
	return nil
}

func (t *eapTLSTransport) LocalAddr() net.Addr  { return eapAddr{} }
func (t *eapTLSTransport) RemoteAddr() net.Addr { return eapAddr{} }

func (t *eapTLSTransport) SetDeadline(deadline time.Time) error {
	return fmt.Errorf("eap-tls: deadlines not supported (got %v)", deadline)
}

func (t *eapTLSTransport) SetReadDeadline(deadline time.Time) error {
	return fmt.Errorf("eap-tls: deadlines not supported (got %v)", deadline)
}

func (t *eapTLSTransport) SetWriteDeadline(deadline time.Time) error {
	return fmt.Errorf("eap-tls: deadlines not supported (got %v)", deadline)
}

type eapAddr struct{}

func (eapAddr) Network() string { return "eap" }
func (eapAddr) String() string  { return "eap" }
