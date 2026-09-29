// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-TLS (RFC 5216)
// RFC: rfc/short/rfc5216.md -- Sections 2.1.1, 2.1.3, 2.4, 3, 5.3 and 5.4: what each flight carries
// Related: rfc5216_success_flight_test.go -- driveEAPTLSFlight and the record decoder
// Related: rfc9190_resumption_test.go -- the resumption stores and their peer config
//
// The older units for these rows asserted the tls.Config ze installs. These
// tests read the handshake messages that crossed: each EAP-TLS message is
// reassembled from its fragments, its plaintext handshake messages are listed
// in order, and the records after change_cipher_spec are counted, so a flight
// carrying a message too many or too few fails whatever the config says.
//
// VALIDATES: the TLS 1.2 full-handshake flights of Section 2.1.1; the peer's
// TLS 1.2 resumed flight of Section 2.1.1 and its ciphersuite; the
// authenticator ending the conversation after its EAP-Failure; null compression
// both ways; the reserved flag bits a sender writes; the server chain without
// its root; the peer refusing a revoked server certificate over TLS 1.2.
// PREVENTS: a server flight without its certificate or with a message after
// server_hello_done; a peer flight missing certificate, certificate_verify or
// client_key_exchange; a resumed peer flight carrying a full handshake; an
// authenticator that keeps answering after EAP-Failure; negotiated compression.

package eap

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"encoding/pem"
	"testing"
	"time"
)

// TLS handshake message types this file reads (RFC 5246 Section 7.4).
const (
	tlsHSClientHello        byte = 1
	tlsHSServerHello        byte = 2
	tlsHSCertificate        byte = 11
	tlsHSCertificateRequest byte = 13
	tlsHSServerHelloDone    byte = 14
	tlsHSCertificateVerify  byte = 15
	tlsHSClientKeyExchange  byte = 16
	tlsContentCCS           byte = 20
	tlsContentHandshake     byte = 22
)

// tlsFlight is one reassembled EAP-TLS message, read record by record.
type tlsFlight struct {
	types    []byte          // plaintext handshake message types, in order
	bodies   map[byte][]byte // the first body of each plaintext handshake type
	ccs      bool            // a change_cipher_spec record was seen
	afterCCS []byte          // content types of the records after change_cipher_spec
}

// eapTLSMessages reassembles the EAP-TLS messages one side sent. A bare packet
// (flags only, no data) is an ACK or a no-data Response and carries no message.
func eapTLSMessages(t *testing.T, packets []*Packet) [][]byte {
	t.Helper()
	var messages [][]byte
	var current []byte
	for _, p := range packets {
		if p == nil || p.Type != TypeTLS || len(p.TypeData) < 2 {
			continue
		}
		flags := p.TypeData[0]
		data := p.TypeData[1:]
		if flags&eapTLSFlagL != 0 {
			if len(p.TypeData) < 5 {
				t.Fatalf("an L-flagged packet of %d octets", len(p.TypeData))
			}
			data = p.TypeData[5:]
		}
		current = append(current, data...)
		if flags&eapTLSFlagM == 0 {
			messages = append(messages, current)
			current = nil
		}
	}
	return messages
}

// parseTLSFlight reads the records of one message. Handshake records before
// change_cipher_spec are concatenated and split into handshake messages; the
// records after it are sealed, so only their content types are kept.
func parseTLSFlight(t *testing.T, msg []byte) tlsFlight {
	t.Helper()
	fl := tlsFlight{bodies: map[byte][]byte{}}
	var handshake []byte
	for off := 0; off < len(msg); {
		if off+tlsRecordHeaderLen > len(msg) {
			t.Fatalf("a truncated TLS record header at offset %d", off)
		}
		end := off + tlsRecordHeaderLen + int(binary.BigEndian.Uint16(msg[off+3:off+5]))
		if end > len(msg) {
			t.Fatalf("a TLS record overruns its message at offset %d", off)
		}
		switch {
		case fl.ccs:
			fl.afterCCS = append(fl.afterCCS, msg[off])
		case msg[off] == tlsContentCCS:
			fl.ccs = true
		case msg[off] == tlsContentHandshake:
			handshake = append(handshake, msg[off+tlsRecordHeaderLen:end]...)
		}
		off = end
	}
	for off := 0; off < len(handshake); {
		if off+4 > len(handshake) {
			t.Fatalf("a truncated handshake header at offset %d", off)
		}
		size := int(handshake[off+1])<<16 | int(handshake[off+2])<<8 | int(handshake[off+3])
		if off+4+size > len(handshake) {
			t.Fatalf("a handshake message overruns its records at offset %d", off)
		}
		kind := handshake[off]
		fl.types = append(fl.types, kind)
		if _, seen := fl.bodies[kind]; !seen {
			fl.bodies[kind] = handshake[off+4 : off+4+size]
		}
		off += 4 + size
	}
	return fl
}

// serverHelloSuite reads past the session_id to the ciphersuite and the
// compression octet of a ServerHello body (RFC 5246 Section 7.4.1.3).
func serverHelloSuite(t *testing.T, body []byte) (suite uint16, compression byte) {
	t.Helper()
	if len(body) < 35 {
		t.Fatalf("a ServerHello body of %d octets", len(body))
	}
	at := 35 + int(body[34])
	if at+3 > len(body) {
		t.Fatalf("a ServerHello body of %d octets ends before its compression method", len(body))
	}
	return binary.BigEndian.Uint16(body[at : at+2]), body[at+2]
}

// clientHelloCompression answers the offset of the compression_methods vector
// of a ClientHello body and the methods it lists (RFC 5246 Section 7.4.1.2).
func clientHelloCompression(t *testing.T, body []byte) (int, []byte) {
	t.Helper()
	at := 34
	if at >= len(body) {
		t.Fatalf("a ClientHello body of %d octets", len(body))
	}
	at += 1 + int(body[at])
	if at+2 > len(body) {
		t.Fatalf("a ClientHello body ends before its cipher suites")
	}
	at += 2 + int(binary.BigEndian.Uint16(body[at:at+2]))
	if at >= len(body) || at+1+int(body[at]) > len(body) {
		t.Fatalf("a ClientHello body ends before its compression methods")
	}
	return at, body[at+1 : at+1+int(body[at])]
}

// tls12Exchange drives one TLS 1.2 EAP-TLS conversation over the two stores
// and answers the flight and the reassembled messages of each side.
func tls12Exchange(t *testing.T, pki *eapTLSPKI, server, peerStore *Resumption) (*eapTLSFlight, *PeerSession, [][]byte, [][]byte) {
	t.Helper()
	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(peerStore))
	fl := driveEAPTLSFlight(t, pki.serverConfigResuming(server), peer, tls.VersionTLS12, eapTLS13Rounds)
	requireCompleted(t, fl, "the TLS 1.2 exchange")
	return fl, peer, eapTLSMessages(t, fl.serverSent), eapTLSMessages(t, fl.peerSent)
}

// fullTLS12Flights answers the ClientHello, the server's first flight and the
// peer's second flight of one full TLS 1.2 handshake.
func fullTLS12Flights(t *testing.T, pki *eapTLSPKI) (clientHello, serverFlight, peerFlight tlsFlight, fl *eapTLSFlight) {
	t.Helper()
	fl, _, server, peer := tls12Exchange(t, pki, NewResumption(time.Now, true), NewResumption(time.Now, true))
	if len(server) < 1 || len(peer) < 2 {
		t.Fatalf("the exchange carried %d server and %d peer messages", len(server), len(peer))
	}
	clientHello = parseTLSFlight(t, peer[0])
	serverFlight = parseTLSFlight(t, server[0])
	peerFlight = parseTLSFlight(t, peer[1])
	if len(clientHello.types) == 0 || clientHello.types[0] != tlsHSClientHello {
		t.Fatalf("the peer's first message carries handshake types %v, want a ClientHello", clientHello.types)
	}
	if len(serverFlight.types) == 0 || serverFlight.types[0] != tlsHSServerHello {
		t.Fatalf("the server's first message carries handshake types %v, want a ServerHello first", serverFlight.types)
	}
	return clientHello, serverFlight, peerFlight, fl
}

// TestRFC5216ServerFlightCarriesItsCertificateAndEndsWithServerHelloDone reads
// the server's first flight of a full TLS 1.2 handshake.
//
// RFC requirement: RFC5216-2.1.1-6 positive -- on a full handshake the EAP-Request
// carrying server_hello carries a certificate message, and server_hello_done is
// its last handshake message.
func TestRFC5216ServerFlightCarriesItsCertificateAndEndsWithServerHelloDone(t *testing.T) {
	_, server, _, _ := fullTLS12Flights(t, newEAPTLSPKI(t))
	if !bytes.Contains(server.types, []byte{tlsHSCertificate}) {
		t.Fatalf("the server flight carries handshake types %v, with no certificate", server.types)
	}
	if last := server.types[len(server.types)-1]; last != tlsHSServerHelloDone {
		t.Fatalf("the server flight carries handshake types %v, want server_hello_done last", server.types)
	}
}

// TestRFC5216ServerSendsItsChainWithoutTheRoot reads the certificate list of
// the server's certificate message.
//
// RFC requirement: RFC5216-5.3-1 positive -- the server sends its certificate
// chain minus the root: the list is the server leaf alone, and the trusted CA
// certificate is not in it.
func TestRFC5216ServerSendsItsChainWithoutTheRoot(t *testing.T) {
	pki := newEAPTLSPKI(t)
	_, server, _, _ := fullTLS12Flights(t, pki)
	body := server.bodies[tlsHSCertificate]
	if len(body) < 3 {
		t.Fatalf("a certificate message of %d octets", len(body))
	}
	var chain [][]byte
	for off := 3; off < len(body); {
		if off+3 > len(body) {
			t.Fatal("a truncated certificate length")
		}
		size := int(body[off])<<16 | int(body[off+1])<<8 | int(body[off+2])
		if off+3+size > len(body) {
			t.Fatal("a certificate overruns its message")
		}
		chain = append(chain, body[off+3:off+3+size])
		off += 3 + size
	}
	leaf, _ := pem.Decode(pki.serverCertPEM)
	if leaf == nil {
		t.Fatal("the server certificate PEM does not decode")
	}
	if len(chain) != 1 || !bytes.Equal(chain[0], leaf.Bytes) {
		t.Fatalf("the server sent %d certificates, want exactly its leaf", len(chain))
	}
	if bytes.Equal(chain[0], pki.trustedCA.Raw) {
		t.Fatal("the server sent the root certificate")
	}
}

// TestRFC5216PeerFlightAnswersTheCertificateRequest reads the peer's second
// flight of a full TLS 1.2 handshake.
//
// RFC requirement: RFC5216-2.1.1-1 positive -- the server flight carried a
// certificate_request, and the peer's answer carries certificate and
// certificate_verify.
//
// RFC requirement: RFC5216-2.1.1-9 positive -- the server_hello did not resume,
// and the peer's answer carries client_key_exchange, then change_cipher_spec,
// then one sealed handshake record, the finished message.
func TestRFC5216PeerFlightAnswersTheCertificateRequest(t *testing.T) {
	_, server, peer, _ := fullTLS12Flights(t, newEAPTLSPKI(t))
	if !bytes.Contains(server.types, []byte{tlsHSCertificateRequest}) {
		t.Fatalf("the server flight carries %v, with no certificate_request", server.types)
	}
	want := []byte{tlsHSCertificate, tlsHSClientKeyExchange, tlsHSCertificateVerify}
	if !bytes.Equal(peer.types, want) {
		t.Fatalf("the peer flight carries handshake types %v, want %v", peer.types, want)
	}
	if !peer.ccs || !bytes.Equal(peer.afterCCS, []byte{tlsContentHandshake}) {
		t.Fatalf("the peer flight: change_cipher_spec %v, then records %v; want one sealed finished", peer.ccs, peer.afterCCS)
	}
}

// TestRFC5216NeitherSideNegotiatesCompression reads both hellos.
//
// RFC requirement: RFC5216-2.4-3 positive -- the peer's ClientHello offers the
// null compression method alone, and the ServerHello selects it.
func TestRFC5216NeitherSideNegotiatesCompression(t *testing.T) {
	client, server, _, _ := fullTLS12Flights(t, newEAPTLSPKI(t))
	if _, offered := clientHelloCompression(t, client.bodies[tlsHSClientHello]); !bytes.Equal(offered, []byte{0}) {
		t.Fatalf("the peer offers compression methods %v, want [0]", offered)
	}
	if _, chosen := serverHelloSuite(t, server.bodies[tlsHSServerHello]); chosen != 0 {
		t.Fatalf("the server selected compression method %d, want 0", chosen)
	}
}

// TestRFC5216ServerRefusesTheCompressionAPeerOffers injects a ClientHello
// offering DEFLATE before null into ze's authenticator.
//
// RFC requirement: RFC5216-2.4-3 negative -- a peer that requests compression
// (methods [1, 0]) is answered with a ServerHello selecting null compression.
func TestRFC5216ServerRefusesTheCompressionAPeerOffers(t *testing.T) {
	pki := newEAPTLSPKI(t)
	sess, err := NewSession(TypeTLS, pki.serverConfig())
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(NewResumption(time.Now, true)))
	t.Cleanup(func() {
		sess.Close()
		peer.Close()
	})
	method, ok := sess.method.(*tlsMethod)
	if !ok {
		t.Fatalf("authenticator method is %T", sess.method)
	}
	method.tlsConfig.MaxVersion = tls.VersionTLS12

	// Run to the EAP-TLS Start, then collect the peer's whole ClientHello. A
	// hybrid key share makes it longer than one fragment, so the fragments are
	// acknowledged here, to the peer alone: the authenticator sees only the
	// rewritten message.
	identity := peer.Process(sess.Begin())
	if identity.Response == nil {
		t.Fatalf("the peer did not answer the Identity Request: %v", identity.Err)
	}
	start := sess.Process(identity.Response)
	if start == nil || start.Type != TypeTLS {
		t.Fatalf("the Identity Response drew %v, want the EAP-TLS Start", start)
	}
	res := peer.Process(start)
	var fragments []*Packet
	for range 8 {
		if res.Response == nil {
			t.Fatalf("the peer stopped inside its ClientHello: %v", res.Err)
		}
		fragments = append(fragments, res.Response)
		if res.Response.TypeData[0]&eapTLSFlagM == 0 {
			break
		}
		ack := &Packet{Code: CodeRequest, Identifier: res.Response.Identifier + 1, Type: TypeTLS, TypeData: []byte{0}}
		res = peer.Process(ack)
	}
	whole := eapTLSMessages(t, fragments)
	if len(whole) != 1 {
		t.Fatalf("the peer's first flight reassembles into %d messages, want one ClientHello", len(whole))
	}

	// Rewrite its compression_methods from [0] to [1, 0], fixing both lengths.
	record := whole[0]
	if len(record) < tlsRecordHeaderLen+4 || record[tlsRecordHeaderLen] != tlsHSClientHello {
		t.Fatal("the peer's first message is not a ClientHello record")
	}
	body := record[tlsRecordHeaderLen+4:]
	at, methods := clientHelloCompression(t, body)
	if !bytes.Equal(methods, []byte{0}) {
		t.Fatalf("the peer offered %v, want [0] before the rewrite", methods)
	}
	rewritten := append(append(append([]byte{}, body[:at]...), 2, 1, 0), body[at+2:]...)
	msg := make([]byte, tlsRecordHeaderLen+4, tlsRecordHeaderLen+4+len(rewritten))
	copy(msg, record[:tlsRecordHeaderLen])
	binary.BigEndian.PutUint16(msg[3:5], uint16(4+len(rewritten)))
	msg[5] = tlsHSClientHello
	msg[6], msg[7], msg[8] = byte(len(rewritten)>>16), byte(len(rewritten)>>8), byte(len(rewritten))
	msg = append(msg, rewritten...)

	out := sess.Process(&Packet{Code: CodeResponse, Identifier: start.Identifier, Type: TypeTLS, TypeData: append([]byte{0}, msg...)})
	if out == nil || out.Type != TypeTLS || len(out.TypeData) < 2 {
		t.Fatalf("the ClientHello offering DEFLATE drew %v, want the server flight", out)
	}
	data := out.TypeData[1:]
	if out.TypeData[0]&eapTLSFlagL != 0 {
		data = out.TypeData[5:]
	}
	if len(data) < tlsRecordHeaderLen+4 || data[0] != tlsContentHandshake || data[tlsRecordHeaderLen] != tlsHSServerHello {
		t.Fatal("the server flight does not open with a ServerHello")
	}
	if _, chosen := serverHelloSuite(t, data[tlsRecordHeaderLen+4:]); chosen != 0 {
		t.Fatalf("the server selected compression method %d for a peer offering [1 0], want 0", chosen)
	}
}

// TestRFC5216SendersWriteZeroReservedFlagBits reads the flags octet of every
// EAP-TLS packet both sides sent in a full TLS 1.2 conversation.
//
// RFC requirement: RFC5216-3-5 positive -- every flags octet ze writes, as
// authenticator and as peer, has the five reserved bits clear.
func TestRFC5216SendersWriteZeroReservedFlagBits(t *testing.T) {
	_, _, _, fl := fullTLS12Flights(t, newEAPTLSPKI(t))
	const reserved = 0x1f
	for side, packets := range map[string][]*Packet{"authenticator": fl.serverSent, "peer": fl.peerSent} {
		for i, p := range packets {
			if p == nil || p.Type != TypeTLS || len(p.TypeData) == 0 {
				continue
			}
			if p.TypeData[0]&reserved != 0 {
				t.Fatalf("%s packet %d carries flags %#02x, with reserved bits set", side, i, p.TypeData[0])
			}
		}
	}
}

// resumedTLS12Flights drives two TLS 1.2 conversations over one pair of stores
// and answers the first ServerHello body, the resuming server flight and the
// peer's answer to it.
func resumedTLS12Flights(t *testing.T) (firstHello []byte, resumed, answer tlsFlight) {
	t.Helper()
	pki := newEAPTLSPKI(t)
	server, peerStore := NewResumption(time.Now, true), NewResumption(time.Now, true)

	_, _, firstServer, _ := tls12Exchange(t, pki, server, peerStore)
	firstHello = parseTLSFlight(t, firstServer[0]).bodies[tlsHSServerHello]

	_, peer, secondServer, secondPeer := tls12Exchange(t, pki, server, peerStore)
	if !peer.Resumed() {
		t.Fatal("the second TLS 1.2 conversation did not resume")
	}
	if len(secondPeer) < 2 {
		t.Fatalf("the resumed conversation carried %d peer messages", len(secondPeer))
	}
	resumed = parseTLSFlight(t, secondServer[0])
	if len(resumed.types) == 0 || resumed.types[0] != tlsHSServerHello {
		t.Fatalf("the resuming server flight carries %v, want a ServerHello first", resumed.types)
	}
	return firstHello, resumed, parseTLSFlight(t, secondPeer[1])
}

// TestRFC5216ResumedFlightsCarryOnlyChangeCipherSpecAndFinished drives two TLS
// 1.2 conversations over one pair of stores and reads the peer's resumed
// flight and the resumed ciphersuite.
//
// RFC requirement: RFC5216-2.1.1-10 positive -- the peer answers the resuming
// server_hello with change_cipher_spec and one sealed finished record, and no
// plaintext handshake message.
//
// RFC requirement: RFC5216-2.1.1-5 positive -- the resumed ServerHello selects
// the ciphersuite the first conversation established.
func TestRFC5216ResumedFlightsCarryOnlyChangeCipherSpecAndFinished(t *testing.T) {
	firstHello, resumed, answer := resumedTLS12Flights(t)
	firstSuite, _ := serverHelloSuite(t, firstHello)
	if len(answer.types) != 0 || !answer.ccs || !bytes.Equal(answer.afterCCS, []byte{tlsContentHandshake}) {
		t.Fatalf("the peer answered the resuming server_hello with %v, ccs %v, then %v", answer.types, answer.ccs, answer.afterCCS)
	}
	if suite, _ := serverHelloSuite(t, resumed.bodies[tlsHSServerHello]); suite != firstSuite {
		t.Fatalf("the resumed ServerHello selects suite %#04x, the session was established under %#04x", suite, firstSuite)
	}
}

// TestRFC5216AuthenticatorEndsTheConversationAfterItsFailure answers the
// authenticator's alert with the no-data Response and then keeps talking.
//
// RFC requirement: RFC5216-2.1.3-4 positive -- the no-data Response draws
// EAP-Failure, and every Response after it draws nothing: the conversation is
// over and did not succeed.
func TestRFC5216AuthenticatorEndsTheConversationAfterItsFailure(t *testing.T) {
	impostor := newImpostorPKI(t)
	sess, err := NewSession(TypeTLS, impostor.serverConfig())
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	peer := NewPeerSessionTLS("impostor-pki-client", impostor.peerConfig())
	t.Cleanup(func() {
		sess.Close()
		peer.Close()
	})

	alert := alertFlight(t, sess, peer, 40)
	noData := &Packet{Code: CodeResponse, Identifier: alert.Identifier, Type: TypeTLS, TypeData: []byte{0}}
	failure := sess.Process(noData)
	if failure == nil || failure.Code != CodeFailure {
		t.Fatalf("the no-data Response to the alert drew %v, want EAP-Failure", failure)
	}
	for _, late := range []*Packet{
		noData,
		{Code: CodeResponse, Identifier: failure.Identifier, Type: TypeTLS, TypeData: []byte{0}},
		{Code: CodeResponse, Identifier: failure.Identifier + 1, Type: TypeTLS, TypeData: []byte{0}},
	} {
		if out := sess.Process(late); out != nil {
			t.Fatalf("a Response after EAP-Failure drew %v, want nothing", out)
		}
	}
	if sess.Succeeded() {
		t.Fatal("the conversation succeeded after its EAP-Failure")
	}
}

// TestRFC5216PeerRefusesARevokedServerCertificateOverTLS12 gives the peer a
// CRL naming the server's certificate and runs TLS 1.2.
//
// RFC requirement: RFC5216-5.4-1 positive -- the peer half: a peer holding a
// CRL that revokes the authenticator's certificate refuses the TLS 1.2
// exchange, exports no MSK, and no EAP-Success concludes it.
func TestRFC5216PeerRefusesARevokedServerCertificateOverTLS12(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", pki.peerConfigWithCRL(pki.crlRevoking(t, eapTLSServerSerial)))
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS12, revocationRounds)

	if fl.peerErr == nil {
		t.Fatal("the peer accepted an authenticator certificate its CRL revokes")
	}
	if fl.peerDone || fl.successAt >= 0 {
		t.Fatalf("the exchange concluded: peerDone=%v successAt=%d", fl.peerDone, fl.successAt)
	}
	if fl.peerMSK != ([64]byte{}) {
		t.Fatal("the peer exported an MSK from a refused exchange")
	}
}
