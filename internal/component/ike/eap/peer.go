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
	"strings"
	"sync/atomic"
)

const maxEAPRounds = 20

var (
	ErrTooManyRounds = errors.New("eap: exceeded maximum exchange rounds")
	ErrEAPFailure    = errors.New("eap: authenticator sent Failure")

	// errNoPeerTrustAnchor refuses an EAP-TLS peer session that could not
	// path-validate the authenticator. RFC 5216 Section 5.3 makes that validation
	// a MUST, and the configured CA is the peer's only trust anchor: EAP carries
	// no server hostname, so there is nothing else to check the chain against.
	errNoPeerTrustAnchor = errors.New("eap-tls: no CA certificate configured: RFC 5216 Section 5.3 requires the peer to path-validate the authenticator chain, which needs a trust anchor")
)

// PeerResult is the outcome of processing one EAP-Request from the authenticator.
type PeerResult struct {
	Response *Packet
	MSK      [64]byte
	Done     bool
	Err      error
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

	// MSCHAPv2 state.
	peerChallenge [16]byte
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

	// A parked TLS failure outranks whatever arrives next. The reply the RFC asks
	// for has gone out, so this packet ends the exchange whatever it is, and the
	// cause the operator needs is the handshake's rather than the generic "the
	// authenticator sent Failure" that names only the symptom.
	if ps.pendingErr != nil {
		ps.state = peerStateFailed
		return PeerResult{Err: ps.pendingErr}
	}

	switch request.Code {
	case CodeSuccess:
		ps.state = peerStateDone
		msk := ps.msk
		return PeerResult{Done: true, MSK: msk}

	case CodeFailure:
		ps.state = peerStateFailed
		return PeerResult{Err: ErrEAPFailure}

	case CodeRequest:
		return ps.handleRequest(request)

	default:
		return PeerResult{Err: fmt.Errorf("eap: unexpected code %d", request.Code)}
	}
}

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

	case peerStateMethod:
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
	if req.Type != TypeMSCHAPv2 {
		return PeerResult{Err: fmt.Errorf("eap: expected type %d, got %d", TypeMSCHAPv2, req.Type)}
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

func (ps *PeerSession) handleMSCHAPv2Success(identifier uint8, td []byte) PeerResult {
	if len(td) > 4 {
		msg := string(td[4:])
		if strings.HasPrefix(msg, "S=") && len(msg) >= 42 {
			if _, err := hex.DecodeString(msg[2:42]); err != nil {
				return PeerResult{Err: fmt.Errorf("eap-mschapv2: invalid authenticator response: %w", err)}
			}
		}
	}

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
		Certificates:          []tls.Certificate{cert},
		InsecureSkipVerify:    true, //nolint:gosec // EAP has no server hostname; the chain is always verified in VerifyPeerCertificate
		MinVersion:            tls.VersionTLS12,
		RootCAs:               rootCAs,
		VerifyPeerCertificate: verifyServerChain(rootCAs),
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
// It WAITS for the engine to settle. A snapshot taken before the engine has
// written its flight yields an empty slice, which the branch below sends as a
// bare fragment ACK: RFC 5216 Section 2.1.5 permits that only in answer to a
// message carrying the M flag, so an authenticator refuses it and the method
// fails before the ClientHello ever crosses.
func (ps *PeerSession) readAndSendTLS(identifier uint8) PeerResult {
	clientData := ps.tlsTransport.waitServerData()

	if len(clientData) == 0 {
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
