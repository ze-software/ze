// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-TLS over TLS 1.3
// RFC: rfc/short/rfc9190.md -- EAP-TLS 1.3, protected success result indication (Section 2.5)
//
// RFC 9190 Section 2.5 adds one packet to a successful EAP-TLS 1.3 conversation
// that RFC 5216 never had:
//
//	"When an EAP-TLS server has successfully processed the TLS client Finished
//	and sent its last handshake message (Finished or a post-handshake message),
//	it sends an encrypted TLS record with application data 0x00.  The encrypted
//	TLS record with application data 0x00 is a protected success result
//	indication, as defined in [RFC3748].  After sending an EAP-Request that
//	contains the protected success result indication, the EAP-TLS server must
//	not send any more EAP-Requests and may only send an EAP-Success.  The
//	EAP-TLS server MUST NOT send an encrypted TLS record with application data
//	0x00 before it has successfully processed the client Finished and sent its
//	last handshake message."
//
// rfc5216_success_flight_test.go pins the TLS 1.2 closing flight, which this
// section replaces on TLS 1.3. This file pins the TLS 1.3 one.
//
// MEASURED DISCRIMINATION, 2026-08-12: with tlsMethod.indicateSuccess forced to
// return nothing, TestEAPTLS13SendsProtectedSuccessIndication and
// TestEAPTLSIssuesNoUnredeemableSessionTicket both FAIL. The two
// negative-polarity tests stay green, which is the shape of every assertion
// about an ABSENCE and is stated on each of them rather than papered over
// (ai/rules/interop-and-goal-validation.md, "Prove the test discriminates").
//
// VALIDATES: the authenticator's last EAP-Request carries one encrypted record
// whose plaintext is the single octet 0x00, the peer answers it with a no-data
// EAP-Response, EAP-Success follows, and the exchange still yields one identical
// non-zero MSK on both sides.
// PREVENTS: the authenticator concluding with a bare EAP-Success (which
// strongSwan's EAP-TLS client refuses: eap_tls.c get_msk returns FAILED and logs
// "missing protected success indication for EAP-TLS with TLS 1.3"), sending the
// indication on a round before the client Finished was accepted, sending it at
// all on TLS 1.2, and issuing a NewSessionTicket no session could ever redeem.

package eap

import (
	"crypto/tls"
	"testing"
	"time"
)

// eapTLS13Rounds bounds a conversation whose length RFC 9190 Section 2.5 fixes.
//
// MEASURED 2026-08-12 with this file's PKI: the authenticator sends 7 packets on
// TLS 1.3 (Start, two handshake flights, the indication, EAP-Success) and 6 on
// TLS 1.2. The remaining slack absorbs the extra fragments a larger certificate
// chain would add, and nothing more: a regression that adds a ROUND trips this
// cap and reddens the test, where the 40 the RFC 5216 flight tests use would
// absorb it silently.
const eapTLS13Rounds = 12

// eapTLSAppDataRecords reports how many of a packet's TLS records carry the
// application_data content type. It answers -1 when the packet is not a
// complete, unfragmented TLS message, so "no records" and "not a message this
// helper can read" never look the same.
func eapTLSAppDataRecords(p *Packet) int {
	if p == nil || p.Code != CodeRequest || p.Type != TypeTLS {
		return 0
	}
	types := tlsRecordContentTypes(p.TypeData)
	if types == nil {
		return -1
	}
	n := 0
	for _, ct := range types {
		if ct == tlsRecordApplicationData {
			n++
		}
	}
	return n
}

// readPeerPlaintext decrypts whatever the authenticator's last EAP-Request left
// in the peer's transport, using the peer's own tls.Conn and therefore the
// session's real traffic keys.
//
// The read runs on its own goroutine behind a deadline. eapTLSTransport.Read
// parks forever on an empty buffer, so a regression that sends no record at all
// would hang the test rather than fail it, and a hung test reports nothing.
func readPeerPlaintext(t *testing.T, peer *PeerSession) []byte {
	t.Helper()

	type readResult struct {
		data []byte
		err  error
	}
	done := make(chan readResult, 1)
	go func() {
		buf := make([]byte, 16)
		n, err := peer.tlsConn.Read(buf)
		done <- readResult{data: buf[:n], err: err}
	}()

	select {
	case r := <-done:
		if r.err != nil {
			t.Fatalf("the peer could not decrypt the authenticator's record: %v", r.err)
		}
		return r.data
	case <-time.After(5 * time.Second):
		t.Fatal("the peer's TLS engine produced no plaintext: the authenticator sent no application data record")
		return nil
	}
}

// driveRejectedEAPTLS13 drives the real authenticator against the real peer over
// a session pinned to TLS 1.3, until the authenticator refuses the peer, and
// returns the authenticator.
//
// It drives tlsMethod directly rather than through driveEAPTLSFlight, because
// the refusal direction of Section 2.5 is NOT readable from the wire: on TLS 1.3
// the fatal alert is sealed under the handshake keys and travels under the same
// application_data content type the indication does, so counting record content
// types cannot tell the two apart. The authenticator's own one-shot flag can,
// and nothing but indicateSuccess writes it.
//
// The version pin touches only this method's own tls.Config copy. No production
// default moves, and MinVersion stays TLS 1.2 everywhere else.
func driveRejectedEAPTLS13(t *testing.T, cfg MethodConfig, peer *PeerSession) *tlsMethod {
	t.Helper()

	method, err := newTLSMethod(cfg)
	if err != nil {
		t.Fatalf("newTLSMethod: %v", err)
	}
	method.tlsConfig.MinVersion = tls.VersionTLS13
	method.tlsConfig.MaxVersion = tls.VersionTLS13
	t.Cleanup(func() {
		method.Close()
		peer.Close()
	})

	req := method.Start(1)
	for i := range eapTLS13Rounds {
		pres := peer.Process(req)
		if pres.Err != nil {
			t.Fatalf("round %d: the peer failed before the authenticator refused it (%v); this test needs the AUTHENTICATOR to be the side that refuses", i+1, pres.Err)
		}
		if pres.Response == nil {
			t.Fatalf("round %d: the peer stopped answering before the refusal", i+1)
		}

		mres := method.Process(pres.Response)
		if mres.Done {
			t.Fatalf("round %d: the authenticator completed a handshake it was meant to refuse", i+1)
		}
		if mres.Err != nil || method.alertSent != nil {
			return method
		}
		if mres.Response == nil {
			t.Fatalf("round %d: the authenticator sent nothing at all", i+1)
		}
		req = mres.Response
	}
	t.Fatalf("the authenticator never refused the peer within %d rounds", eapTLS13Rounds)
	return nil
}

// TestEAPTLS13SendsProtectedSuccessIndication drives a mutually authenticated
// EAP-TLS conversation over TLS 1.3 and asserts the three closing packets RFC
// 9190 Section 2.5 owes, in order.
//
// RFC requirement: RFC9190-2.5-1 positive -- the EAP-Request immediately before
// EAP-Success carries exactly one TLS record, that record's content type is
// application_data, and the peer's own tls.Conn decrypts its plaintext to the
// single octet 0x00. The peer answers it with an EAP-Response of
// EAP-Type=EAP-TLS carrying no data (step 4), and the authenticator sends no
// further EAP-Request: the next and last packet it sends is EAP-Success
// (step 3).
//
// RFC requirement: RFC9190-2.5-2 positive -- no EAP-Request before that one
// carries an application_data record, so the indication was not sent before the
// client Finished had been processed. The handshake completing, and the two MSKs
// agreeing, is what proves the client Finished WAS processed on that round.
func TestEAPTLS13SendsProtectedSuccessIndication(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
	})

	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS13, eapTLS13Rounds)

	if fl.peerErr != nil {
		t.Fatalf("the peer failed a handshake both sides should accept: %v", fl.peerErr)
	}
	if fl.successAt < 1 {
		t.Fatalf("no EAP-Success after a data packet: successAt=%d over %d packets", fl.successAt, len(fl.serverSent))
	}

	// Step 3: EAP-Success is the LAST thing the authenticator sends. Anything
	// after it would break the "no more EAP-Requests" clause.
	if fl.successAt != len(fl.serverSent)-1 {
		t.Fatalf("the authenticator sent %d more packet(s) after EAP-Success", len(fl.serverSent)-1-fl.successAt)
	}

	// Step 2: the packet before EAP-Success is the indication.
	indication := fl.serverSent[fl.successAt-1]
	switch n := eapTLSAppDataRecords(indication); n {
	case -1:
		// A fragment, or a packet with no length prefix at all. With the
		// indication missing, the packet in this position is the fragmented
		// handshake flight that completed the exchange, so this is the branch a
		// bare-EAP-Success regression lands in.
		t.Fatalf("the packet before EAP-Success is not a complete unfragmented TLS message (%d octets, flags %#02x): "+
			"the authenticator concluded without the RFC 9190 Section 2.5 indication",
			len(indication.TypeData), indication.TypeData[0])
	case 1:
		// Exactly one record, which is what RFC9190-2.5-1 asks for and what
		// SessionTicketsDisabled in newTLSMethod keeps true.
	default:
		t.Fatalf("the packet before EAP-Success carries %d application_data records, want 1", n)
	}

	// RFC9190-2.5-2: nothing earlier carried one.
	for i := range fl.successAt - 1 {
		if n := eapTLSAppDataRecords(fl.serverSent[i]); n > 0 {
			t.Fatalf("server packet %d carries %d application_data record(s) before the client Finished round (%d)", i, n, fl.successAt-1)
		}
	}

	// Step 4: the peer answers the indication with the no-data EAP-Response.
	// peerSent[i] answers serverSent[i]: driveEAPTLSFlight appends one packet
	// from each side per round.
	if len(fl.peerSent) <= fl.successAt-1 {
		t.Fatalf("the peer sent %d packets, so it never answered the indication at index %d", len(fl.peerSent), fl.successAt-1)
	}
	if answer := fl.peerSent[fl.successAt-1]; !bareEAPTLSResponse(answer) {
		t.Fatalf("the peer answered the indication with TypeData=% x, want the bare flags octet", answer.TypeData)
	}

	// The record really carries application data 0x00, decrypted under the
	// session's traffic keys. The content type alone does not prove this: every
	// TLS 1.3 post-handshake message travels under the same outer type.
	if plain := readPeerPlaintext(t, peer); len(plain) != 1 || plain[0] != 0x00 {
		t.Fatalf("the indication's plaintext is % x, want the single octet 00", plain)
	}

	// The exchange still authenticates. A "success indication" that broke the
	// keys would be worse than none.
	var zero [64]byte
	if fl.peerMSK == zero || fl.serverMSK == zero {
		t.Fatalf("an MSK is all zero: peer=%v server=%v", fl.peerMSK == zero, fl.serverMSK == zero)
	}
	if fl.peerMSK != fl.serverMSK {
		t.Fatalf("MSK mismatch:\n peer=  %x\n server=%x", fl.peerMSK, fl.serverMSK)
	}
}

// TestEAPTLS13RefusedClientGetsNoSuccessIndication drives a TLS 1.3 exchange the
// authenticator refuses, and asserts no protected success indication is sent.
//
// RFC requirement: RFC9190-2.5-2 negative -- the client's chain does not verify,
// so the authenticator never successfully processes the client Finished. It
// sends its fatal TLS alert and reports the failure, and it never sends the
// encrypted record carrying application data 0x00. A server that sent it anyway
// would be telling a peer it had just refused that it had succeeded.
//
// RFC requirement: RFC9190-2.5-1 negative -- the procedure's precondition
// ("successfully processed the TLS client Finished") does not hold, so the
// procedure does not run.
//
// THIS TEST ASSERTS AN ABSENCE, so deleting the indication leaves it green. That
// is inherent, not an oversight (ai/rules/interop-and-goal-validation.md). What
// it discriminates against is the opposite defect: a write not gated on
// m.handshaked, which would set the flag on the alert round. The two assertions
// before the absence keep it from being vacuous about the SCENARIO -- they
// require the exchange to have reached the refusal at all.
func TestEAPTLS13RefusedClientGetsNoSuccessIndication(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("rogue-client", &PeerTLSConfig{
		CertPEM:   pki.untrustedClientCertPEM,
		KeyPEM:    pki.untrustedClientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
	})

	method := driveRejectedEAPTLS13(t, pki.serverConfig(), peer)

	if method.alertSent == nil {
		t.Fatal("the exchange ended without the authenticator's fatal TLS alert, so it never reached the round a broken server would have indicated success on")
	}
	if method.handshaked.Load() {
		t.Fatal("the authenticator marked the handshake complete for a client chain it refused")
	}
	if method.successIndicated {
		t.Fatal("the authenticator sent the RFC 9190 Section 2.5 protected success indication to a client it refused")
	}
}

// TestEAPTLS12SendsNoProtectedSuccessIndication caps the authenticator at TLS
// 1.2 and asserts the RFC 9190 procedure does not run there.
//
// RFC requirement: RFC9190-2.5-1 negative -- Section 2.5 "only applies to TLS
// 1.3". On a TLS 1.2 session the authenticator concludes with the RFC 5216
// Section 2.1.3 flight and EAP-Success, and sends no application data at all, so
// an RFC 5216 peer that would refuse an unexpected record still interoperates.
//
// This is the guard on the version test in tlsMethod.indicateSuccess reading the
// NEGOTIATED version rather than tlsConfig.MinVersion, which is TLS 1.2 in every
// session including the TLS 1.3 ones. Flip that test to MinVersion and this
// reddens; delete the indication entirely and it stays green, which is what an
// absence assertion does.
func TestEAPTLS12SendsNoProtectedSuccessIndication(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
	})

	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS12, eapTLS13Rounds)

	if fl.peerErr != nil {
		t.Fatalf("the peer failed a TLS 1.2 handshake both sides should accept: %v", fl.peerErr)
	}
	if fl.successAt < 0 {
		t.Fatalf("the authenticator never sent EAP-Success across %d packets", len(fl.serverSent))
	}
	for i, p := range fl.serverSent {
		if n := eapTLSAppDataRecords(p); n > 0 {
			t.Fatalf("server packet %d carries %d application_data record(s) on a TLS 1.2 session", i, n)
		}
	}

	var zero [64]byte
	if fl.peerMSK == zero || fl.peerMSK != fl.serverMSK {
		t.Fatalf("the TLS 1.2 MSK changed: peer=%x server=%x", fl.peerMSK, fl.serverMSK)
	}
}

// TestEAPTLSIssuesNoUnredeemableSessionTicket asserts the authenticator mints no
// TLS session ticket.
//
// newTLSMethod builds a fresh tls.Config for every EAP session, and Go keys
// ticket encryption on the Config instance (Config.ticketKeys, crypto/tls
// common.go, fills autoSessionTicketKeys with random octets per Config). A
// ticket issued in one EAP session is therefore undecryptable in every other, so
// resumption is unreachable by construction and six RFC 9190 MUSTs conditional
// on it (5.6-2, 5.7-1, 5.7-2, 5.7-3, 5.7-4, 5.7-6) are dead.
//
// VALIDATES: the closing EAP-Request carries ONE record. Go writes a
// NewSessionTicket as its own record from readClientCertificate, in the same
// HandshakeContext call, so a config that issued one would put two records in
// that packet.
// PREVENTS: SessionTicketsDisabled being dropped from newTLSMethod, which would
// silently arm those six obligations while nothing could still redeem a ticket.
func TestEAPTLSIssuesNoUnredeemableSessionTicket(t *testing.T) {
	pki := newEAPTLSPKI(t)
	method, err := newTLSMethod(pki.serverConfig())
	if err != nil {
		t.Fatalf("newTLSMethod: %v", err)
	}
	if !method.tlsConfig.SessionTicketsDisabled {
		t.Fatal("newTLSMethod leaves session tickets enabled, so it issues tickets no other session can redeem")
	}

	peer := NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:   pki.clientCertPEM,
		KeyPEM:    pki.clientKeyPEM,
		CACertPEM: pki.trustedCAPEM,
	})
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS13, eapTLS13Rounds)
	if fl.successAt < 1 {
		t.Fatalf("no EAP-Success after a data packet: successAt=%d", fl.successAt)
	}
	if types := tlsRecordContentTypes(fl.serverSent[fl.successAt-1].TypeData); len(types) != 1 {
		t.Fatalf("the closing EAP-Request carries %d TLS records (% x), want 1: a second one is a NewSessionTicket", len(types), types)
	}
}
