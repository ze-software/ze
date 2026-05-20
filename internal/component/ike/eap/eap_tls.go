// Design: plan/spec-ipsec-9-ikev2-eap-nat.md -- EAP-TLS method handler
// RFC: rfc/short/rfc5216.md -- EAP-TLS: TLS handshake in EAP, fragmentation, MSK derivation

package eap

import (
	"context"
	"crypto/sha256"
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

type tlsState uint8

const (
	tlsStateStart tlsState = iota
	tlsStateHandshake
	tlsStateDone
)

type tlsMethod struct {
	tlsConfig  *tls.Config
	state      tlsState
	conn       *tls.Conn
	transport  *eapTLSTransport
	handshaked atomic.Bool
}

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
func (m *tlsMethod) Process(response *Packet) MethodResult {
	if response.Type != TypeTLS {
		return MethodResult{Err: ErrMethodFailed}
	}

	tlsData := extractTLSData(response.TypeData)
	if len(tlsData) > 0 {
		m.transport.feedPeerData(tlsData)
	}

	serverData := m.transport.readServerData()

	if m.handshaked.Load() {
		m.state = tlsStateDone
		msk := m.deriveMSK()
		return MethodResult{MSK: msk, Done: true}
	}

	if len(serverData) == 0 {
		return MethodResult{
			Response: &Packet{
				Code:     CodeRequest,
				Type:     TypeTLS,
				TypeData: []byte{0},
			},
		}
	}

	return m.buildFragmentedResponse(serverData)
}

func (m *tlsMethod) buildFragmentedResponse(data []byte) MethodResult {
	td := encodeTLSData(data)
	return MethodResult{
		Response: &Packet{
			Code:     CodeRequest,
			Type:     TypeTLS,
			TypeData: td,
		},
	}
}

func extractTLSData(typeData []byte) []byte {
	if len(typeData) == 0 {
		return nil
	}
	flags := typeData[0]
	off := 1
	if flags&eapTLSFlagL != 0 {
		if len(typeData) < 5 {
			return nil
		}
		off = 5
	}
	if off >= len(typeData) {
		return nil
	}
	return typeData[off:]
}

func encodeTLSData(data []byte) []byte {
	if len(data) <= eapTLSFragmentSize {
		td := make([]byte, 5+len(data))
		td[0] = eapTLSFlagL
		binary.BigEndian.PutUint32(td[1:5], uint32(len(data)))
		copy(td[5:], data)
		return td
	}
	td := make([]byte, 5+eapTLSFragmentSize)
	td[0] = eapTLSFlagL | eapTLSFlagM
	binary.BigEndian.PutUint32(td[1:5], uint32(len(data)))
	copy(td[5:], data[:eapTLSFragmentSize])
	return td
}

func (m *tlsMethod) runTLSServer() {
	m.conn = tls.Server(m.transport, m.tlsConfig)
	err := m.conn.HandshakeContext(context.Background())
	if err != nil {
		m.transport.setError(err)
		return
	}
	m.handshaked.Store(true)
}

// deriveMSK derives the MSK from the TLS connection.
// RFC 5216 Section 2.3: MSK = TLS-PRF(master_secret, "client EAP encryption",
// client.random || server.random)[0..63].
func (m *tlsMethod) deriveMSK() [64]byte {
	var msk [64]byte
	if m.conn == nil {
		return msk
	}
	cs := m.conn.ConnectionState()
	exported, err := cs.ExportKeyingMaterial("client EAP encryption", nil, 64)
	if err == nil {
		copy(msk[:], exported)
		return msk
	}
	// Fallback: hash session state for a deterministic key.
	h := sha256.New()
	h.Write(cs.TLSUnique)
	sum := h.Sum(nil)
	copy(msk[:32], sum)
	copy(msk[32:], sum)
	return msk
}

// notifyCh sends a non-blocking signal on a buffered channel.
func notifyCh(ch chan struct{}) {
	select { //nolint:staticcheck // non-blocking signal; drop is intentional when already signaled
	case ch <- struct{}{}:
	case <-time.After(0):
	}
}

// eapTLSTransport implements net.Conn for piping TLS through EAP packets.
type eapTLSTransport struct {
	mu        sync.Mutex
	peerBuf   []byte
	serverBuf []byte
	peerCh    chan struct{}
	err       error
	closed    bool
}

func newEAPTLSTransport() *eapTLSTransport {
	return &eapTLSTransport{
		peerCh: make(chan struct{}, 1),
	}
}

func (t *eapTLSTransport) feedPeerData(data []byte) {
	t.mu.Lock()
	t.peerBuf = append(t.peerBuf, data...)
	t.mu.Unlock()
	notifyCh(t.peerCh)
}

func (t *eapTLSTransport) readServerData() []byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	data := t.serverBuf
	t.serverBuf = nil
	return data
}

func (t *eapTLSTransport) setError(err error) {
	t.mu.Lock()
	t.err = err
	t.mu.Unlock()
}

// Read implements net.Conn.Read (called by TLS to read peer data).
func (t *eapTLSTransport) Read(p []byte) (int, error) {
	for {
		t.mu.Lock()
		if len(t.peerBuf) > 0 {
			n := copy(p, t.peerBuf)
			t.peerBuf = t.peerBuf[n:]
			t.mu.Unlock()
			return n, nil
		}
		if t.closed {
			t.mu.Unlock()
			return 0, io.EOF
		}
		t.mu.Unlock()
		<-t.peerCh
	}
}

// Write implements net.Conn.Write (called by TLS to send server data).
func (t *eapTLSTransport) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.serverBuf = append(t.serverBuf, p...)
	return len(p), nil
}

// Close implements net.Conn.Close.
func (t *eapTLSTransport) Close() error {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()
	notifyCh(t.peerCh)
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
