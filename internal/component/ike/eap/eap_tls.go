// Design: plan/learned/744-ipsec-9-ikev2-eap-nat.md -- EAP-TLS method handler
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
const eapTLSMaxReassembly = 65536

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
			return fmt.Errorf("eap-tls: TLS message too large (%d bytes)", totalLen)
		}
		f.inExpected = totalLen
		// Reuse existing buffer if capacity is sufficient.
		if cap(f.inBuf) >= totalLen {
			f.inBuf = f.inBuf[:0]
		} else {
			f.inBuf = make([]byte, 0, totalLen)
		}
		off = 5
	}

	if off < len(typeData) {
		f.inBuf = append(f.inBuf, typeData[off:]...)
	}

	if f.inExpected > 0 && len(f.inBuf) > f.inExpected {
		return fmt.Errorf("eap-tls: reassembled data (%d bytes) exceeds declared length (%d bytes)", len(f.inBuf), f.inExpected)
	}

	return nil
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
// RFC 5216 Section 2.1.5: handles fragment reassembly and outbound fragmentation.
func (m *tlsMethod) Process(response *Packet) MethodResult {
	if response.Type != TypeTLS {
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

	// All fragments received. Feed reassembled data to TLS.
	if data := m.drainReassembled(); len(data) > 0 {
		m.transport.feedPeerData(data)
	}

	// Read TLS engine output.
	serverData := m.transport.readServerData()

	if m.handshaked.Load() {
		m.state = tlsStateDone
		msk := m.deriveMSK()
		return MethodResult{MSK: msk, Done: true}
	}

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
