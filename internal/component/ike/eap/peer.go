// Design: plan/learned/805-ipsec-11-interop-eap.md -- EAP peer (client/initiator) side
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
		userName: StripDomain(identity),
	}
}

// NewPeerSessionTLS creates an EAP-TLS peer session with certificate material.
func NewPeerSessionTLS(identity string, cfg *PeerTLSConfig) *PeerSession {
	return &PeerSession{
		identity: identity,
		method:   TypeTLS,
		state:    peerStateIdentity,
		userName: StripDomain(identity),
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

	// All fragments received. Feed to TLS engine.
	if data := ps.drainReassembled(); len(data) > 0 {
		ps.tlsTransport.feedPeerData(data)
	}

	// If the TLS handshake has completed, capture the MSK now so it is ready
	// when the authenticator sends EAP-Success. Completion does NOT end the EAP
	// exchange here: with TLS 1.3 the peer produces its final flight (client
	// Certificate/CertificateVerify/Finished) at the same instant the handshake
	// completes, and that flight must still be sent to the authenticator via
	// readAndSendTLS below. The EAP layer only concludes on the EAP-Success the
	// authenticator sends after it has verified that flight (handled in Process,
	// CodeSuccess). Returning Done here would drop the unsent flight and stall
	// the authenticator forever.
	if ps.tlsDone.Load() {
		ps.msk = ps.deriveTLSMSK()
	}

	return ps.readAndSendTLS(req.Identifier)
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

	var rootCAs *x509.CertPool
	if len(ps.tlsCfg.CACertPEM) > 0 {
		rootCAs = x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM(ps.tlsCfg.CACertPEM) {
			return fmt.Errorf("eap-tls: failed to parse peer CA certificate")
		}
	}

	// RFC 5216 Section 5.3: the EAP peer validates the authenticator's
	// certificate chain against its configured trust anchor. EAP-TLS carries no
	// server hostname, so Go's default (SNI/hostname-based) verification cannot
	// be used -- with a CA set but no ServerName, tls.Client rejects the config
	// outright ("either ServerName or InsecureSkipVerify must be specified") and
	// never sends a ClientHello. InsecureSkipVerify disables only that default
	// hostname check; when a trust anchor is configured the chain is instead
	// verified explicitly against it in VerifyPeerCertificate below. With no
	// trust anchor the peer performs no server-certificate validation.
	tlsCfg := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true, //nolint:gosec // EAP has no server hostname; chain verified in VerifyPeerCertificate when a CA is configured
		MinVersion:         tls.VersionTLS12,
	}
	if rootCAs != nil {
		tlsCfg.RootCAs = rootCAs
		tlsCfg.VerifyPeerCertificate = verifyServerChain(rootCAs)
	}

	ps.tlsTransport = newEAPTLSTransport()
	ps.tlsConn = tls.Client(ps.tlsTransport, tlsCfg)
	ps.tlsStarted.Store(true)

	go func() {
		_ = ps.tlsConn.HandshakeContext(context.Background())
		ps.tlsDone.Store(true)
	}()

	return nil
}

// readAndSendTLS reads TLS engine output and sends it (possibly fragmented).
func (ps *PeerSession) readAndSendTLS(identifier uint8) PeerResult {
	clientData := ps.tlsTransport.readServerData()

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

func (ps *PeerSession) deriveTLSMSK() [64]byte {
	var msk [64]byte
	if ps.tlsConn == nil {
		return msk
	}
	cs := ps.tlsConn.ConnectionState()
	// Fail closed: tlsDone is also set when the TLS handshake FAILED (e.g. the
	// authenticator's certificate was rejected). ExportKeyingMaterial panics on
	// a connection whose handshake did not complete, so guard on it and return
	// an all-zero MSK -- an invalid key that cannot yield a passing EAP-Success.
	if !cs.HandshakeComplete {
		return msk
	}
	exported, err := cs.ExportKeyingMaterial("client EAP encryption", nil, 64)
	if err == nil {
		copy(msk[:], exported)
	}
	return msk
}
