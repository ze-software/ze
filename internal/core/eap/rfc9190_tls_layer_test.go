// Design: docs/architecture/ike/ipsec-14-responder.md -- the authenticator's EAP-TLS method
// Detail: docs/architecture/ike/ipsec-11-interop-eap.md -- the peer's TLS client
// RFC: rfc/short/rfc9190.md -- early data, post-handshake authentication and PSK use (Sections 2.1, 2.1.1, 2.1.2, 2.1.8)
//
// VALIDATES: the configuration ze hands to Go crypto/tls produces no early_data
// in EncryptedExtensions or NewSessionTicket and no post-handshake
// CertificateRequest, read off the wire by decrypting the authenticator's records
// with the test client's own key log; the authenticator refuses a ClientHello
// that offers early data; ze's peer offers no early data from a ticket that
// permits it; a PSK is used only to resume a ticket the authenticator issued;
// and a peer that holds a certificate sends it.
// PREVENTS: a configuration change (a QUIC transport, an ALPN list, a raised
// version ceiling, a dropped client-certificate demand) that would let crypto/tls
// take the early-data or post-handshake paths RFC 9190 forbids, unseen because
// TLS 1.3 encrypts every message those paths change.
//
// Go crypto/tls performs these behaviors: it has no API for 0-RTT outside QUIC
// and none for post-handshake authentication at all. What ze owns is the
// tls.Config it builds (newTLSMethod, eap_tls.go; tlsClientConfig, peer.go) and
// the transport it runs crypto/tls over, so that is what these tests drive.

package eap

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// TLS 1.3 handshake message types the decrypted flight is read for (RFC 8446
// Section 4, "enum HandshakeType").
const (
	tlsHSNewSessionTicket    byte = 4
	tlsHSEncryptedExtensions byte = 8
	tlsHSFinished            byte = 20
	tlsAEADNonceOctets            = 12
)

// recordingConn keeps a copy of every byte the test's TLS client reads, which is
// every record the server under test wrote.
//
// Not safe for concurrent use: only the client goroutine reads, and the test
// reads the copy after that goroutine has ended.
type recordingConn struct {
	net.Conn
	received bytes.Buffer
}

func (c *recordingConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	c.received.Write(b[:n])
	return n, err
}

// tls13Exchange is what one recorded handshake leaves for the assertions.
type tls13Exchange struct {
	serverRecords []byte
	keyLog        string
	state         tls.ConnectionState
	serverErr     error
}

// runRecordedTLS13Handshake runs one handshake between the server config under
// test and a test client that logs its secrets, then lets the client read until
// the server closes, so the post-handshake messages (NewSessionTicket, and any
// post-handshake CertificateRequest) are on the recording too.
//
// The server closes as soon as its handshake returns, and its close_notify is
// what ends the client's read. The deadline is the backstop.
func runRecordedTLS13Handshake(t *testing.T, serverCfg, clientCfg *tls.Config) *tls13Exchange {
	t.Helper()

	serverConn, clientConn := net.Pipe()
	deadline := time.Now().Add(10 * time.Second)
	if err := serverConn.SetDeadline(deadline); err != nil {
		t.Fatalf("server deadline: %v", err)
	}
	if err := clientConn.SetDeadline(deadline); err != nil {
		t.Fatalf("client deadline: %v", err)
	}

	var keyLog bytes.Buffer
	clientCfg.KeyLogWriter = &keyLog
	recorded := &recordingConn{Conn: clientConn}
	ex := &tls13Exchange{}

	ctx := t.Context()
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		client := tls.Client(recorded, clientCfg)
		if err := client.HandshakeContext(ctx); err == nil {
			ex.state = client.ConnectionState()
			io.Copy(io.Discard, client) //nolint:errcheck // the read only pulls the post-handshake records onto the recording; it ends on the server's close
		}
		clientConn.Close() //nolint:errcheck // closing releases a server parked on the pipe
	}()

	server := tls.Server(serverConn, serverCfg)
	ex.serverErr = server.HandshakeContext(ctx)
	server.Close() //nolint:errcheck // the close_notify is what ends the client's read

	<-clientDone
	ex.serverRecords = recorded.received.Bytes()
	ex.keyLog = keyLog.String()
	return ex
}

// tls13Message is one handshake message decrypted off the server's records.
type tls13Message struct {
	kind          byte
	body          []byte
	afterFinished bool
}

// tls13Suite answers the hash and key length of a TLS 1.3 AES-GCM suite (RFC 8446
// Appendix B.4). The harness client offers no other, so another one is a broken
// harness rather than a finding.
func tls13Suite(t *testing.T, suite uint16) (func() hash.Hash, int) {
	t.Helper()
	switch suite {
	case tls.TLS_AES_128_GCM_SHA256:
		return sha256.New, 16
	case tls.TLS_AES_256_GCM_SHA384:
		return sha512.New384, 32
	}
	t.Fatalf("the handshake negotiated suite %#04x, which this decoder does not decrypt", suite)
	return nil, 0
}

// tls13ExpandLabel is HKDF-Expand-Label (RFC 8446 Section 7.1) with an empty
// context.
func tls13ExpandLabel(t *testing.T, digest func() hash.Hash, secret []byte, label string, length int) []byte {
	t.Helper()
	info := binary.BigEndian.AppendUint16(nil, uint16(length))
	info = append(info, byte(len("tls13 ")+len(label)))
	info = append(info, "tls13 "...)
	info = append(info, label...)
	info = append(info, 0)
	out, err := hkdf.Expand(digest, secret, string(info), length)
	if err != nil {
		t.Fatalf("HKDF-Expand-Label %q: %v", label, err)
	}
	return out
}

// tls13Keys is the AEAD and IV for one traffic secret (RFC 8446 Section 7.3).
type tls13Keys struct {
	aead cipher.AEAD
	iv   []byte
	seq  uint64
}

func newTLS13Keys(t *testing.T, suite uint16, secret []byte) *tls13Keys {
	t.Helper()
	digest, keyOctets := tls13Suite(t, suite)
	block, err := aes.NewCipher(tls13ExpandLabel(t, digest, secret, "key", keyOctets))
	if err != nil {
		t.Fatalf("AES key: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("GCM: %v", err)
	}
	return &tls13Keys{aead: aead, iv: tls13ExpandLabel(t, digest, secret, "iv", tlsAEADNonceOctets)}
}

// open decrypts one protected record (RFC 8446 Section 5.2 and 5.3) and answers
// its inner content type and content.
func (k *tls13Keys) open(t *testing.T, header, sealed []byte) (byte, []byte) {
	t.Helper()
	nonce := make([]byte, tlsAEADNonceOctets)
	binary.BigEndian.PutUint64(nonce[tlsAEADNonceOctets-8:], k.seq)
	for i := range nonce {
		nonce[i] ^= k.iv[i]
	}
	k.seq++
	plain, err := k.aead.Open(nil, nonce, sealed, header)
	if err != nil {
		t.Fatalf("record %d did not decrypt: %v", k.seq-1, err)
	}
	end := len(plain)
	for end > 0 && plain[end-1] == 0 {
		end--
	}
	if end == 0 {
		t.Fatal("a protected record carries no content type")
	}
	return plain[end-1], plain[:end-1]
}

// keyLogSecret reads one secret from an NSS key log, failing the test when the
// label is absent.
func keyLogSecret(t *testing.T, keyLog, label string) []byte {
	t.Helper()
	for line := range strings.SplitSeq(keyLog, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[0] != label {
			continue
		}
		secret, err := hex.DecodeString(fields[2])
		if err != nil {
			t.Fatalf("key log %s: %v", label, err)
		}
		return secret
	}
	t.Fatalf("the key log carries no %s", label)
	return nil
}

// decryptServerFlight answers every handshake message the server wrote, in
// order, marking the ones that came after its Finished. The handshake traffic
// secret protects the records up to and including the Finished, and the first
// application traffic secret protects every record after it (RFC 8446 Section
// 7.1).
func decryptServerFlight(t *testing.T, ex *tls13Exchange) []tls13Message {
	t.Helper()
	suite := ex.state.CipherSuite
	keys := newTLS13Keys(t, suite, keyLogSecret(t, ex.keyLog, "SERVER_HANDSHAKE_TRAFFIC_SECRET"))
	application := keyLogSecret(t, ex.keyLog, "SERVER_TRAFFIC_SECRET_0")

	var messages []tls13Message
	var stream []byte
	afterFinished := false
	raw := ex.serverRecords
	for len(raw) >= tlsRecordHeaderLen {
		length := int(binary.BigEndian.Uint16(raw[3:5]))
		if len(raw) < tlsRecordHeaderLen+length {
			t.Fatalf("a record of %d octets overruns the %d recorded", length, len(raw)-tlsRecordHeaderLen)
		}
		header, fragment := raw[:tlsRecordHeaderLen], raw[tlsRecordHeaderLen:tlsRecordHeaderLen+length]
		raw = raw[tlsRecordHeaderLen+length:]
		if header[0] != tlsRecordApplicationData {
			continue // The ServerHello and the compatibility ChangeCipherSpec travel in clear.
		}
		inner, content := keys.open(t, header, fragment)
		if inner != tlsRecordHandshake {
			continue // An alert: the close_notify that ends the exchange.
		}
		stream = append(stream, content...)
		for len(stream) >= tlsHandshakeHeaderOctets {
			bodyOctets := int(stream[1])<<16 | int(stream[2])<<8 | int(stream[3])
			if len(stream) < tlsHandshakeHeaderOctets+bodyOctets {
				break
			}
			kind := stream[0]
			messages = append(messages, tls13Message{kind: kind, body: stream[tlsHandshakeHeaderOctets : tlsHandshakeHeaderOctets+bodyOctets], afterFinished: afterFinished})
			stream = stream[tlsHandshakeHeaderOctets+bodyOctets:]
			if kind == tlsHSFinished && !afterFinished {
				afterFinished = true
				keys = newTLS13Keys(t, suite, application)
			}
		}
	}
	return messages
}

// extensionTypes lists the extension types of a uint16-length-prefixed
// extension block.
func extensionTypes(t *testing.T, block []byte) []uint16 {
	t.Helper()
	if len(block) < 2 || int(binary.BigEndian.Uint16(block))+2 != len(block) {
		t.Fatalf("an extension block of %d octets does not match its length prefix", len(block))
	}
	var types []uint16
	for off := 2; off+4 <= len(block); {
		types = append(types, binary.BigEndian.Uint16(block[off:]))
		off += 4 + int(binary.BigEndian.Uint16(block[off+2:]))
	}
	return types
}

// newSessionTicketExtensions answers the extensions of a NewSessionTicket body
// (RFC 8446 Section 4.6.1): ticket_lifetime(4), ticket_age_add(4),
// ticket_nonce<0..255>, ticket<1..2^16-1>, extensions<0..2^16-2>.
func newSessionTicketExtensions(t *testing.T, body []byte) []uint16 {
	t.Helper()
	at := 8
	if len(body) <= at {
		t.Fatalf("a NewSessionTicket of %d octets is truncated", len(body))
	}
	at += 1 + int(body[at])
	if len(body) < at+2 {
		t.Fatalf("a NewSessionTicket of %d octets is truncated before its ticket", len(body))
	}
	at += 2 + int(binary.BigEndian.Uint16(body[at:]))
	if len(body) < at {
		t.Fatalf("a NewSessionTicket of %d octets is truncated before its extensions", len(body))
	}
	return extensionTypes(t, body[at:])
}

// TestEAPTLS13AuthenticatorConfigSendsNoEarlyDataOrPostHandshakeRequest runs the
// tls.Config newTLSMethod builds against a TLS 1.3 client holding a certificate,
// decrypts every record the authenticator wrote with the client's own key log,
// and reads the encrypted messages early data and post-handshake authentication
// would change.
//
// Go crypto/tls performs these behaviors: it accepts early data only on a QUIC
// connection and has no post-handshake authentication API. Ze's part is the
// configuration and the transport it hands crypto/tls, and that is what this
// test holds: a TLS 1.3 ceiling, the client-certificate demand that puts the
// CertificateRequest inside the handshake, and a plain stream transport.
//
// RFC requirement: RFC9190-2.1-3 positive -- "EAP-TLS servers MUST NOT send an
// early_data extension". The decrypted EncryptedExtensions carries no extension
// 42.
//
// RFC requirement: RFC9190-2.1-5 positive -- "Post-handshake authentication
// MUST NOT be used in EAP-TLS". The client is authenticated by the
// CertificateRequest inside the handshake, and nothing after the server's
// Finished asks for a certificate again.
//
// RFC requirement: RFC9190-2.1-7 positive -- "Servers MUST NOT request
// post-handshake client authentication". No CertificateRequest follows the
// server's Finished, while exactly one precedes it.
//
// RFC requirement: RFC9190-2.1.2-3 positive -- "The NewSessionTicket message
// MUST NOT include an "early_data" extension". Every decrypted NewSessionTicket
// carries no extension 42.
func TestEAPTLS13AuthenticatorConfigSendsNoEarlyDataOrPostHandshakeRequest(t *testing.T) {
	pki := newEAPTLSPKI(t)
	method, err := newTLSMethod(pki.serverConfig())
	if err != nil {
		t.Fatalf("newTLSMethod: %v", err)
	}
	t.Cleanup(method.Close)
	if method.tlsConfig.MaxVersion != tls.VersionTLS13 {
		t.Fatalf("the authenticator's ceiling is %#04x, want TLS 1.3", method.tlsConfig.MaxVersion)
	}
	if method.tlsConfig.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Fatalf("the authenticator's ClientAuth is %v, want RequireAndVerifyClientCert", method.tlsConfig.ClientAuth)
	}

	// A client session cache is what makes crypto/tls offer psk_dhe_ke, and the
	// authenticator issues a NewSessionTicket only to a client offering it.
	client := naiClientConfig(t, pki, pki.clientCertPEM, pki.clientKeyPEM)
	client.ClientSessionCache = tls.NewLRUClientSessionCache(1)
	ex := runRecordedTLS13Handshake(t, method.tlsConfig, client)
	if ex.serverErr != nil {
		t.Fatalf("the authenticator refused the client: %v", ex.serverErr)
	}
	if ex.state.Version != tls.VersionTLS13 {
		t.Fatalf("the handshake negotiated %#04x, so RFC 9190 Section 2.1 does not apply to it", ex.state.Version)
	}

	var extensions, requestsBefore, requestsAfter, tickets int
	for _, m := range decryptServerFlight(t, ex) {
		switch {
		case m.kind == tlsHSEncryptedExtensions:
			extensions++
			for _, ext := range extensionTypes(t, m.body) {
				if ext == tlsExtEarlyData {
					t.Error("the EncryptedExtensions carries an early_data extension, which RFC 9190 Section 2.1 forbids a server to send")
				}
			}
		case m.kind == tlsHSCertificateRequest && m.afterFinished:
			requestsAfter++
		case m.kind == tlsHSCertificateRequest:
			requestsBefore++
		case m.kind == tlsHSNewSessionTicket:
			tickets++
			for _, ext := range newSessionTicketExtensions(t, m.body) {
				if ext == tlsExtEarlyData {
					t.Error("a NewSessionTicket carries an early_data extension, which RFC 9190 Section 2.1.2 forbids")
				}
			}
		}
	}
	if extensions != 1 {
		t.Fatalf("decrypted %d EncryptedExtensions, want 1: the decoder did not read the handshake", extensions)
	}
	if requestsBefore != 1 {
		t.Fatalf("decrypted %d CertificateRequest messages inside the handshake, want 1", requestsBefore)
	}
	if requestsAfter != 0 {
		t.Errorf("the authenticator sent %d CertificateRequest messages after its Finished: post-handshake authentication", requestsAfter)
	}
	if tickets == 0 {
		t.Fatal("decrypted no NewSessionTicket, so the post-handshake records were never read")
	}
}

// withEmptyExtension answers a copy of a ClientHello record with one empty
// extension appended to its extension list and every length fixed to match.
// The ClientHello must carry no pre_shared_key, which RFC 8446 Section 4.2.11
// requires to stay last.
func withEmptyExtension(t *testing.T, record []byte, add uint16) []byte {
	t.Helper()
	const handshakeAt = tlsRecordHeaderLen + tlsHandshakeHeaderOctets
	body := record[handshakeAt:]
	at := 2 + tlsClientHelloRandomOctets
	at += 1 + int(body[at])
	at += 2 + int(binary.BigEndian.Uint16(body[at:]))
	at += 1 + int(body[at])
	extOctets := int(binary.BigEndian.Uint16(body[at:]))
	extEnd := at + 2 + extOctets

	rewritten := append([]byte{}, body[:at]...)
	rewritten = binary.BigEndian.AppendUint16(rewritten, uint16(extOctets+4))
	rewritten = append(rewritten, body[at+2:extEnd]...)
	rewritten = binary.BigEndian.AppendUint16(rewritten, add)
	rewritten = binary.BigEndian.AppendUint16(rewritten, 0)
	rewritten = append(rewritten, body[extEnd:]...)

	msg := make([]byte, handshakeAt, handshakeAt+len(rewritten))
	copy(msg, record[:tlsRecordHeaderLen])
	binary.BigEndian.PutUint16(msg[3:5], uint16(tlsHandshakeHeaderOctets+len(rewritten)))
	msg[5] = tlsHSClientHello
	msg[6], msg[7], msg[8] = byte(len(rewritten)>>16), byte(len(rewritten)>>8), byte(len(rewritten))
	return append(msg, rewritten...)
}

// TestEAPTLS13AuthenticatorRefusesAClientHelloOfferingEarlyData replays ze's own
// initial ClientHello at a fresh authenticator, once as sent and once with an
// early_data extension added.
//
// RFC requirement: RFC9190-2.1-2 negative -- "Early Data MUST NOT be used in
// EAP-TLS." A client that offers early data is answered with an alert and no
// ServerHello, while the same ClientHello without the offer is answered with the
// handshake, so no 0-RTT data is ever read. Go crypto/tls sends the alert
// (unsupported_extension, serverHandshakeStateTLS13.processClientHello) because
// ze runs it over a stream transport rather than QUIC under a TLS 1.3 ceiling.
func TestEAPTLS13AuthenticatorRefusesAClientHelloOfferingEarlyData(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(NewResumption(time.Now, true)))
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS13, eapTLS13Rounds)
	requireCompleted(t, fl, "a full EAP-TLS 1.3 exchange")
	record := parseClientHello(t, fl).record
	if _, ok := clientHelloExtensions(t, eapTLSMessages(t, fl.peerSent)[0])[tlsExtPreSharedKey]; ok {
		t.Fatal("the initial ClientHello offers a pre_shared_key, so an extension appended after it would be refused for its position")
	}

	if got := replayClientHello(t, pki, record); got != tlsRecordHandshake {
		t.Fatalf("the unmodified ClientHello drew content type %#02x, want the handshake: the replay itself is broken", got)
	}
	if got := replayClientHello(t, pki, withEmptyExtension(t, record, tlsExtEarlyData)); got != tlsRecordAlert {
		t.Errorf("a ClientHello offering early_data drew content type %#02x, want an alert", got)
	}
}

// TestEAPTLS13PeerOffersNoEarlyDataFromATicketThatPermitsIt marks the ticket
// ze's peer cached as one that permits early data, which is what an
// authenticator's NewSessionTicket early_data extension would record, then
// drives the resumption and decodes the resumed ClientHello.
//
// RFC requirement: RFC9190-2.1.2-4 positive -- "If the "early_data" extension
// is received, then it MUST be ignored". The resumed ClientHello offers the
// ticket as a pre_shared_key and no early_data. Go crypto/tls offers 0-RTT from
// such a ticket only when the client also offers the ALPN protocol the ticket
// was issued for (handshake_client.go, loadSession); ze's tlsClientConfig
// offers no ALPN, so the permission is ignored, and the test holds that
// configuration too.
func TestEAPTLS13PeerOffersNoEarlyDataFromATicketThatPermitsIt(t *testing.T) {
	pki := newEAPTLSPKI(t)
	server := NewResumption(time.Now, true)
	peerStore := NewResumption(time.Now, true)

	first, firstPeer := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, first, "the initial authentication")
	if protocols := firstPeer.tlsClientConfig(nil, &serverChainCheck{}).NextProtos; len(protocols) != 0 {
		t.Fatalf("ze's peer offers ALPN %q, which lets crypto/tls send early data from a permitting ticket", protocols)
	}

	marked := 0
	peerStore.mu.Lock()
	for key, entry := range peerStore.cached {
		ticket, state, err := entry.state.ResumptionState()
		if err != nil {
			peerStore.mu.Unlock()
			t.Fatalf("read the cached ticket: %v", err)
		}
		state.EarlyData = true
		permitting, err := tls.NewResumptionState(ticket, state)
		if err != nil {
			peerStore.mu.Unlock()
			t.Fatalf("rebuild the cached ticket: %v", err)
		}
		entry.state = permitting
		peerStore.cached[key] = entry
		marked++
	}
	peerStore.mu.Unlock()
	if marked == 0 {
		t.Fatal("the peer cached no ticket, so there is nothing to mark")
	}

	second, secondPeer := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, second, "the resumed authentication")
	if !secondPeer.Resumed() {
		t.Fatal("the second exchange did not resume, so it cannot show what the peer does with the ticket")
	}
	hello := clientHelloExtensions(t, eapTLSMessages(t, second.peerSent)[0])
	if _, ok := hello[tlsExtPreSharedKey]; !ok {
		t.Fatal("the resumed ClientHello offers no pre_shared_key, so it never read the marked ticket")
	}
	if _, ok := hello[tlsExtEarlyData]; ok {
		t.Error("the resumed ClientHello offers early_data from a ticket that permitted it, which RFC 9190 Section 2.1.2 says to ignore")
	}
}

// TestEAPTLS13PeerHoldingACertificateSendsIt drives a complete EAP-TLS 1.3
// exchange with ze's peer holding a client certificate, and reads the chain the
// authenticator verified.
//
// RFC requirement: RFC9190-2.1.8-4 negative -- "This means that the EAP-TLS peer
// only sends an empty certificate_list if it does not have an appropriate
// certificate to send". A peer that has one sends it: the authenticator, which
// treats an empty list as terminal, concludes with success holding the peer's
// own leaf.
func TestEAPTLS13PeerHoldingACertificateSendsIt(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(NewResumption(time.Now, true)))
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS13, eapTLS13Rounds)
	requireCompleted(t, fl, "a full EAP-TLS 1.3 exchange")
	if !fl.sess.Succeeded() {
		t.Fatal("the authenticator did not conclude with success")
	}
	if got := serverPeerLeafName(t, fl); got != "eap-tls-client" {
		t.Fatalf("the authenticator verified %q, want the peer's own certificate eap-tls-client", got)
	}
}

// replayForServerHello writes one ClientHello record at an authenticator built
// from cfg and answers the extension types of the ServerHello it sends back.
func replayForServerHello(t *testing.T, cfg MethodConfig, record []byte) []uint16 {
	t.Helper()

	method, err := newTLSMethod(cfg)
	if err != nil {
		t.Fatalf("newTLSMethod: %v", err)
	}
	t.Cleanup(method.Close)

	clientSide, serverSide := net.Pipe()
	if err := clientSide.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatalf("deadline: %v", err)
	}
	t.Cleanup(func() {
		clientSide.Close() //nolint:errcheck // the pipe is torn down with the test
	})
	go func() {
		conn := tls.Server(serverSide, method.tlsConfig)
		conn.HandshakeContext(t.Context()) //nolint:errcheck // the assertion is the ServerHello, read below
		conn.Close()                       //nolint:errcheck // the pipe is torn down with the test
	}()

	answer := make(chan []byte, 1)
	go func() {
		header := make([]byte, tlsRecordHeaderLen)
		if _, err := io.ReadFull(clientSide, header); err != nil {
			answer <- nil
			return
		}
		fragment := make([]byte, binary.BigEndian.Uint16(header[3:5]))
		if _, err := io.ReadFull(clientSide, fragment); err != nil {
			answer <- nil
			return
		}
		answer <- append(header, fragment...)
		io.Copy(io.Discard, clientSide) //nolint:errcheck // drained so the authenticator never blocks writing the rest of its flight
	}()
	if _, err := clientSide.Write(record); err != nil {
		t.Fatalf("write the ClientHello: %v", err)
	}

	first := <-answer
	if len(first) < tlsRecordHeaderLen+tlsHandshakeHeaderOctets || first[0] != tlsRecordHandshake || first[tlsRecordHeaderLen] != tlsHSServerHello {
		t.Fatalf("the authenticator answered %x, not a ServerHello", first)
	}
	// RFC 8446 Section 4.1.3: legacy_version(2), random(32),
	// legacy_session_id_echo<0..32>, cipher_suite(2),
	// legacy_compression_method(1), extensions.
	body := first[tlsRecordHeaderLen+tlsHandshakeHeaderOctets:]
	at := 2 + tlsClientHelloRandomOctets
	at += 1 + int(body[at])
	at += 3
	return extensionTypes(t, body[at:])
}

// resumedClientHello drives an initial and a resumed exchange through ze's own
// peer and answers the resumed ClientHello record, which offers the ticket the
// first exchange issued as a pre_shared_key.
func resumedClientHello(t *testing.T, pki *eapTLSPKI, server *Resumption) (initial, resumed []byte) {
	t.Helper()
	peerStore := NewResumption(time.Now, true)
	first, _ := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, first, "the initial authentication")
	second, secondPeer := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, second, "the resumed authentication")
	if !secondPeer.Resumed() {
		t.Fatal("the second exchange did not resume, so its ClientHello carries no ticket")
	}
	return parseClientHello(t, first).record, parseClientHello(t, second).record
}

// TestEAPTLS13PSKIsUsedToResumeATicketTheAuthenticatorIssued replays ze's
// initial and resumed ClientHello at the authenticator that issued the ticket.
//
// RFC requirement: RFC9190-2.1.1-2 positive -- "Pre-Shared Key (PSK)
// authentication SHALL NOT be used except for resumption." The initial
// ClientHello offers no PSK and its ServerHello selects none, so the first
// authentication is the certificate one; the PSK appears only on the resumed
// ClientHello, and the issuing authenticator selects it there.
func TestEAPTLS13PSKIsUsedToResumeATicketTheAuthenticatorIssued(t *testing.T) {
	pki := newEAPTLSPKI(t)
	server := NewResumption(time.Now, true)
	initial, resumed := resumedClientHello(t, pki, server)

	for _, ext := range replayForServerHello(t, pki.serverConfigResuming(server), initial) {
		if ext == tlsExtPreSharedKey {
			t.Fatal("the ServerHello to the initial ClientHello selects a PSK: PSK authentication outside resumption")
		}
	}
	selected := false
	for _, ext := range replayForServerHello(t, pki.serverConfigResuming(server), resumed) {
		if ext == tlsExtPreSharedKey {
			selected = true
		}
	}
	if !selected {
		t.Fatal("the issuing authenticator did not select the resumption PSK, so the test shows no PSK use at all")
	}
}

// TestEAPTLS13PSKTheAuthenticatorDidNotIssueIsNotUsed replays ze's resumed
// ClientHello at an authenticator holding other ticket keys, which is what a
// PSK provisioned outside a resumption is to it: an identity it never issued.
//
// RFC requirement: RFC9190-2.1.1-2 negative -- "Pre-Shared Key (PSK)
// authentication SHALL NOT be used except for resumption." The ServerHello
// selects no PSK, so the exchange falls back to the full certificate handshake.
func TestEAPTLS13PSKTheAuthenticatorDidNotIssueIsNotUsed(t *testing.T) {
	pki := newEAPTLSPKI(t)
	_, resumed := resumedClientHello(t, pki, NewResumption(time.Now, true))

	for _, ext := range replayForServerHello(t, pki.serverConfig(), resumed) {
		if ext == tlsExtPreSharedKey {
			t.Fatal("an authenticator that never issued the ticket selected its PSK: PSK authentication outside resumption")
		}
	}
}
