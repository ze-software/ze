// Design: docs/architecture/ike/ipsec-14-responder.md -- the authenticator's EAP-TLS method
// Detail: docs/architecture/ike/ipsec-11-interop-eap.md -- the peer's EAP-TLS method
// RFC: rfc/short/rfc9190.md -- EAP-TLS 1.3 handshake rules (Sections 2.1, 2.1.1, 2.1.4, 2.3)
//
// VALIDATES: the TLS 1.3 handshake ze runs inside EAP-TLS keeps to the rules
// RFC 9190 Section 2.1 adds to it (no early data, no post-handshake
// authentication), the authenticator always authenticates with a certificate
// (Section 2.1.1), each end answers with records of the negotiated version and
// signals a fatal error with a TLS alert (Section 2.1.4), and the key material is
// exported at the full length Section 2.3 names (Section 2.3).
// PREVENTS: a ClientHello that offers 0-RTT data or post-handshake
// authentication, an authenticator that runs without a certificate, a refusal
// that ends the conversation without telling the other end why, and an MSK cut
// from a shorter export than the one the RFC defines.

package eap

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

// TLS extension code points the ClientHello assertions read (RFC 8446 Section
// 4.2, "enum ExtensionType").
const (
	tlsExtPreSharedKey         uint16 = 41
	tlsExtEarlyData            uint16 = 42
	tlsExtPSKKeyExchangeModes  uint16 = 45
	tlsHandshakeClientHello    byte   = 1
	tlsLegacyRecordVersion     uint16 = 0x0303
	tlsSupportedVersionTLS13   uint16 = 0x0304
	tlsRecordHeaderOctets             = 5
	tlsHandshakeHeaderOctets          = 4
	tlsClientHelloRandomOctets        = 32
)

// tlsRecord is one TLS record read off the EAP-TLS wire.
type tlsRecord struct {
	contentType byte
	version     uint16
	fragment    []byte
}

// tlsRecords splits one TLS message into its records, failing the test on a
// record whose declared length overruns the message.
func tlsRecords(t *testing.T, message []byte) []tlsRecord {
	t.Helper()
	var records []tlsRecord
	for len(message) > 0 {
		if len(message) < tlsRecordHeaderOctets {
			t.Fatalf("a TLS record header is cut short at %d octets", len(message))
		}
		length := int(binary.BigEndian.Uint16(message[3:5]))
		if len(message) < tlsRecordHeaderOctets+length {
			t.Fatalf("a TLS record declares %d octets and carries %d", length, len(message)-tlsRecordHeaderOctets)
		}
		records = append(records, tlsRecord{
			contentType: message[0],
			version:     binary.BigEndian.Uint16(message[1:3]),
			fragment:    message[tlsRecordHeaderOctets : tlsRecordHeaderOctets+length],
		})
		message = message[tlsRecordHeaderOctets+length:]
	}
	return records
}

// clientHelloExtensions decodes the ClientHello a peer's first TLS message
// carries and answers its extensions by type.
//
// The walk follows RFC 8446 Section 4.1.2, "struct ClientHello": legacy_version,
// random, legacy_session_id, cipher_suites, legacy_compression_methods, then the
// extensions block.
func clientHelloExtensions(t *testing.T, message []byte) map[uint16][]byte {
	t.Helper()
	records := tlsRecords(t, message)
	if len(records) == 0 || records[0].contentType != tlsRecordHandshake {
		t.Fatal("the peer's first TLS message is not a handshake record")
	}
	hs := records[0].fragment
	if len(hs) < tlsHandshakeHeaderOctets || hs[0] != tlsHandshakeClientHello {
		t.Fatal("the peer's first handshake message is not a ClientHello")
	}
	body := hs[tlsHandshakeHeaderOctets:]

	take := func(n int, what string) []byte {
		if len(body) < n {
			t.Fatalf("the ClientHello is cut short in %s", what)
		}
		field := body[:n]
		body = body[n:]
		return field
	}
	take(2+tlsClientHelloRandomOctets, "legacy_version and random")
	take(int(take(1, "the session id length")[0]), "legacy_session_id")
	take(int(binary.BigEndian.Uint16(take(2, "the cipher suite length"))), "cipher_suites")
	take(int(take(1, "the compression length")[0]), "legacy_compression_methods")
	block := take(int(binary.BigEndian.Uint16(take(2, "the extensions length"))), "extensions")

	extensions := make(map[uint16][]byte)
	for len(block) > 0 {
		if len(block) < 4 {
			t.Fatal("an extension header is cut short")
		}
		kind := binary.BigEndian.Uint16(block[0:2])
		length := int(binary.BigEndian.Uint16(block[2:4]))
		if len(block) < 4+length {
			t.Fatalf("extension %d declares %d octets past the end of the block", kind, length)
		}
		extensions[kind] = block[4 : 4+length]
		block = block[4+length:]
	}
	return extensions
}

// offersTLS13 reports whether a supported_versions extension body lists TLS 1.3.
func offersTLS13(body []byte) bool {
	if len(body) < 1 {
		return false
	}
	list := body[1:]
	for i := 0; i+1 < len(list); i += 2 {
		if binary.BigEndian.Uint16(list[i:]) == tlsSupportedVersionTLS13 {
			return true
		}
	}
	return false
}

// TestEAPTLS13PeerClientHelloOffersNoEarlyDataOrPostHandshakeAuth drives an
// initial and a resumed EAP-TLS 1.3 exchange and decodes the ClientHello ze's
// peer put on the wire in each.
//
// The resumed one is the ClientHello that matters for early data: RFC 8446
// Section 4.2.10 lets a client send 0-RTT data only alongside a pre_shared_key,
// so the test requires that extension to be present before it reads the absence
// of early_data. The supported_versions check holds the scenario to TLS 1.3,
// where Section 2.1 applies.
//
// RFC requirement: RFC9190-2.1-2 positive -- "Early Data MUST NOT be used in
// EAP-TLS." Ze's peer offers no early_data extension in either ClientHello,
// including the one that offers a PSK, so no 0-RTT data can follow it.
//
// RFC requirement: RFC9190-2.1-4 positive -- "clients MUST NOT send an
// EndOfEarlyData message". RFC 8446 Section 4.5 sends that message only after a
// server accepted early_data, and ze's peer offered none, so no exchange can
// reach it.
//
// RFC requirement: RFC9190-2.1-6 positive -- "Clients MUST NOT send a
// post_handshake_auth extension". Neither ClientHello carries extension 49,
// although the peer holds a client certificate it could offer.
//
// RFC requirement: RFC9190-2.1.3-2 positive -- "The "psk_dh_ke" key exchange
// mode MUST be used for resumption". The resumed ClientHello's
// psk_key_exchange_modes extension lists psk_dhe_ke (1) and nothing else, so the
// PSK-only mode psk_ke (0) is never on offer.
func TestEAPTLS13PeerClientHelloOffersNoEarlyDataOrPostHandshakeAuth(t *testing.T) {
	pki := newEAPTLSPKI(t)
	server := NewResumption(time.Now, true)
	peerStore := NewResumption(time.Now, true)

	first, _ := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, first, "the initial authentication")
	second, secondPeer := driveResumingExchange(t, pki, server, peerStore)
	requireCompleted(t, second, "the resumed authentication")
	if !secondPeer.Resumed() {
		t.Fatal("the second exchange did not resume, so its ClientHello cannot show the PSK offer this test reads")
	}

	for name, fl := range map[string]*eapTLSFlight{"initial": first, "resumed": second} {
		messages := eapTLSMessages(t, fl.peerSent)
		if len(messages) == 0 {
			t.Fatalf("%s: the peer sent no TLS data", name)
		}
		extensions := clientHelloExtensions(t, messages[0])
		if !offersTLS13(extensions[tlsExtSupportedVersions]) {
			t.Fatalf("%s: the ClientHello does not offer TLS 1.3, so RFC 9190 Section 2.1 does not apply to it", name)
		}
		if _, ok := extensions[tlsExtEarlyData]; ok {
			t.Errorf("%s: the ClientHello carries an early_data extension, which RFC 9190 Section 2.1 forbids", name)
		}
		if _, ok := extensions[tlsExtPostHandshakeAuth]; ok {
			t.Errorf("%s: the ClientHello carries a post_handshake_auth extension, which RFC 9190 Section 2.1 forbids", name)
		}
	}

	resumedHello := clientHelloExtensions(t, eapTLSMessages(t, second.peerSent)[0])
	if _, ok := resumedHello[tlsExtPreSharedKey]; !ok {
		t.Fatal("the resumed ClientHello offers no pre_shared_key, so the absence of early_data above proves nothing about a PSK offer")
	}
	// RFC 8446 Section 4.2.9: "PskKeyExchangeMode ke_modes<1..255>", one octet of
	// length then the modes.
	if modes := resumedHello[tlsExtPSKKeyExchangeModes]; !bytes.Equal(modes, []byte{1, 1}) {
		t.Fatalf("the resumed ClientHello offers PSK modes %x, want psk_dhe_ke alone (01 01)", modes)
	}
}

// TestEAPTLS13AuthenticatorAuthenticatesWithItsCertificate drives a complete
// EAP-TLS 1.3 exchange and reads the chain the peer verified.
//
// RFC requirement: RFC9190-2.1.1-1 positive -- "The EAP-TLS server MUST
// authenticate with a certificate". The peer completes the exchange holding a
// verified chain whose leaf is the certificate the authenticator was configured
// with, and the exchange was not a resumption, so the certificate was presented
// in this handshake.
func TestEAPTLS13AuthenticatorAuthenticatesWithItsCertificate(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(NewResumption(time.Now, true)))
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS13, eapTLS13Rounds)

	requireCompleted(t, fl, "a full EAP-TLS 1.3 exchange")
	if peer.Resumed() {
		t.Fatal("the first exchange over a fresh store resumed")
	}
	chains := peer.ServerChains()
	if len(chains) == 0 || len(chains[0]) == 0 {
		t.Fatal("the peer completed without a verified authenticator chain")
	}
	if got := chains[0][0].Subject.CommonName; got != "eap-tls-server" {
		t.Fatalf("the authenticator authenticated as %q, want its configured certificate eap-tls-server", got)
	}
}

// TestEAPTLSAuthenticatorWithoutACertificateIsRefused builds an EAP-TLS
// authenticator from a configuration that carries no server certificate.
//
// RFC requirement: RFC9190-2.1.1-1 negative -- an authenticator that cannot
// authenticate with a certificate does not run EAP-TLS at all: the session is
// refused when it is built, before any peer could be offered a certificate-less
// handshake.
func TestEAPTLSAuthenticatorWithoutACertificateIsRefused(t *testing.T) {
	pki := newEAPTLSPKI(t)
	for name, cfg := range map[string]MethodConfig{
		"no certificate or key": {
			CACertPEM:  pki.trustedCAPEM,
			CRLPEM:     pki.trustedCRLPEM,
			Resumption: NewResumption(time.Now, true),
		},
		"a key with no certificate": {
			ServerKeyPEM: pki.serverKeyPEM,
			CACertPEM:    pki.trustedCAPEM,
			CRLPEM:       pki.trustedCRLPEM,
			Resumption:   NewResumption(time.Now, true),
		},
	} {
		sess, err := NewSession(TypeTLS, cfg)
		if err == nil {
			sess.Close()
			t.Errorf("%s: the authenticator was built and would run EAP-TLS without a certificate", name)
			continue
		}
		if !strings.Contains(err.Error(), "server cert") {
			t.Errorf("%s: refused for another reason than the missing certificate: %v", name, err)
		}
	}
}

// requireRecordTypes fails the test unless every record of message has one of
// the content types allowed and the legacy record version every TLS 1.2 and 1.3
// record carries, and at least one record has the type required.
func requireRecordTypes(t *testing.T, what string, message []byte, required byte, allowed ...byte) {
	t.Helper()
	records := tlsRecords(t, message)
	seen := false
	for i, r := range records {
		if r.version != tlsLegacyRecordVersion {
			t.Errorf("%s: record %d carries version %#04x, want %#04x", what, i, r.version, tlsLegacyRecordVersion)
		}
		if r.contentType == required {
			seen = true
			continue
		}
		if !bytes.Contains(allowed, []byte{r.contentType}) {
			t.Errorf("%s: record %d has content type %#02x, outside %x", what, i, r.contentType, append([]byte{required}, allowed...))
		}
	}
	if !seen {
		t.Errorf("%s: no record of content type %#02x", what, required)
	}
}

// TestEAPTLS13EachEndAnswersWithRecordsOfTheNegotiatedVersion drives a complete
// EAP-TLS 1.3 exchange and decodes the last TLS message each end sent.
//
// On TLS 1.3 everything after the ServerHello is protected, so a conforming
// record is an application_data record (RFC 8446 Section 5.2, "opaque_type ...
// MUST be set to the value 23 (application_data)"); the client may also send the
// one compatibility change_cipher_spec record Appendix D.4 allows.
//
// RFC requirement: RFC9190-2.1.4-1 positive -- "If the EAP-TLS peer
// authenticates successfully, the EAP-TLS server MUST send an EAP-Request packet
// with EAP-Type=EAP-TLS containing TLS records conforming to the version of TLS
// used". The request after the peer's Finished is EAP-TLS and carries only TLS
// 1.3 protected records.
//
// RFC requirement: RFC9190-2.1.4-2 positive -- "If the EAP-TLS server
// authenticates successfully, the EAP-TLS peer MUST send an EAP-Response message
// with EAP-Type=EAP-TLS containing TLS records conforming to the version of TLS
// used". The peer's answer to the authenticator's Finished is EAP-TLS and
// carries TLS 1.3 protected records.
//
// RFC requirement: RFC9190-2.1-8 positive -- "When EAP-TLS is used with TLS
// version 1.3, the formatting and processing of the TLS handshake SHALL be done
// as specified in version 1.3 of TLS." Both ends complete a TLS 1.3 handshake
// whose flights after the ServerHello are TLS 1.3 protected records, with only
// the one compatibility change_cipher_spec RFC 8446 Appendix D.4 allows.
func TestEAPTLS13EachEndAnswersWithRecordsOfTheNegotiatedVersion(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(NewResumption(time.Now, true)))
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS13, eapTLS13Rounds)
	requireCompleted(t, fl, "a full EAP-TLS 1.3 exchange")

	closing := fl.serverSent[fl.successAt-1]
	if closing.Code != CodeRequest || closing.Type != TypeTLS {
		t.Fatalf("the authenticator's answer to the peer's Finished is code %d type %d, want an EAP-Request/EAP-TLS", closing.Code, closing.Type)
	}
	server := eapTLSMessages(t, fl.serverSent)
	requireRecordTypes(t, "the authenticator's closing message", server[len(server)-1], tlsRecordApplicationData)

	peerMessages := eapTLSMessages(t, fl.peerSent)
	if len(peerMessages) < 2 {
		t.Fatalf("the peer sent %d TLS messages, want its ClientHello and its Finished flight", len(peerMessages))
	}
	requireRecordTypes(t, "the peer's Finished flight", peerMessages[len(peerMessages)-1], tlsRecordApplicationData, tlsRecordChangeCipherSpec)
}

// TestEAPTLS12EachEndAnswersWithRecordsOfTheNegotiatedVersion is the same
// exchange with the authenticator held at TLS 1.2.
//
// RFC requirement: RFC9190-2.1.4-1 negative -- the records conform to the
// version USED, not to TLS 1.3 whatever was negotiated: at TLS 1.2 the
// authenticator's closing message is the RFC 5246 change_cipher_spec and
// Finished pair, a change_cipher_spec record followed by a handshake record. An
// end that always wrote TLS 1.3 shaped records would fail here.
//
// RFC requirement: RFC9190-2.1.4-2 negative -- the same on the peer: at TLS 1.2
// its Finished flight carries its Certificate in a plaintext handshake record,
// which no TLS 1.3 flight does.
//
// RFC requirement: RFC9190-2.1-8 negative -- the TLS 1.3 formatting is tied to
// TLS 1.3 being used: with the authenticator held at TLS 1.2 the same code
// formats and processes the RFC 5246 handshake, plaintext Certificate and the
// change_cipher_spec and Finished pair included.
func TestEAPTLS12EachEndAnswersWithRecordsOfTheNegotiatedVersion(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(NewResumption(time.Now, true)))
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS12, eapTLS13Rounds)
	requireCompleted(t, fl, "a full EAP-TLS 1.2 exchange")

	server := eapTLSMessages(t, fl.serverSent)
	closing := tlsRecords(t, server[len(server)-1])
	ccs := slices.IndexFunc(closing, func(r tlsRecord) bool { return r.contentType == tlsRecordChangeCipherSpec })
	if ccs < 0 || ccs+1 >= len(closing) || closing[ccs+1].contentType != tlsRecordHandshake {
		t.Fatalf("the authenticator's TLS 1.2 closing message has no change_cipher_spec followed by its handshake-typed Finished (%d records)", len(closing))
	}

	peerMessages := eapTLSMessages(t, fl.peerSent)
	flight := tlsRecords(t, peerMessages[len(peerMessages)-1])
	if len(flight) == 0 || flight[0].contentType != tlsRecordHandshake {
		t.Fatal("the peer's TLS 1.2 Finished flight does not open with a plaintext handshake record")
	}
}

// TestEAPTLS13FatalErrorSendsATLSAlert drives two refusals, one from each end,
// and asks the OTHER end what it received.
//
// On TLS 1.3 an alert sent after the ServerHello is protected, so the wire shows
// an application_data record either way (RFC 8446 Section 5.2). The receiving
// end's TLS engine decrypts it and reports it as a remote alert, which is what
// this test reads.
//
// RFC requirement: RFC9190-2.1.4-3 positive -- "Whenever an implementation
// encounters a fatal error condition, it MUST send an appropriate TLS Error
// alert". The authenticator that refuses an untrusted client certificate sends
// an alert the peer receives, and the peer that refuses an untrusted
// authenticator certificate sends an alert the authenticator receives.
func TestEAPTLS13FatalErrorSendsATLSAlert(t *testing.T) {
	pki := newEAPTLSPKI(t)

	t.Run("authenticator refuses the client", func(t *testing.T) {
		peer := NewPeerSessionTLS("rogue-client", &PeerTLSConfig{
			CertPEM:   pki.untrustedClientCertPEM,
			KeyPEM:    pki.untrustedClientKeyPEM,
			CACertPEM: pki.trustedCAPEM,
			CRLPEM:    pki.trustedCRLPEM,
		})
		_, refusal := driveRejectedEAPTLS13(t, pki.serverConfig(), peer)
		if refusal.FinalRequest == nil || refusal.FinalRequest.Type != TypeTLS {
			t.Fatal("the authenticator refused the client and sent no EAP-TLS request carrying an alert")
		}
		requireRemoteAlert(t, "the peer", peerAlertError(t, peer, refusal.FinalRequest))
	})

	t.Run("peer refuses the authenticator", func(t *testing.T) {
		serverCfg := pki.serverConfig()
		serverCfg.ServerCertPEM = pki.untrustedServerCertPEM
		serverCfg.ServerKeyPEM = pki.untrustedServerKeyPEM
		peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(NewResumption(time.Now, true)))
		fl := driveEAPTLSFlight(t, serverCfg, peer, tls.VersionTLS13, eapTLS13Rounds)
		if fl.peerDone {
			t.Fatal("the peer completed against an untrusted authenticator")
		}
		requireRemoteAlert(t, "the authenticator", fl.sess.Err())
	})
}

// peerAlertError hands the authenticator's final request to the peer and answers
// the error the peer's TLS engine recorded for it.
func peerAlertError(t *testing.T, peer *PeerSession, final *Packet) error {
	t.Helper()
	res := peer.Process(final)
	if res.Done {
		t.Fatal("the peer completed after the authenticator refused it")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		// On TLS 1.3 the client's handshake is complete once it has sent its
		// Finished, so the authenticator's alert reaches the post-handshake read
		// rather than the handshake: either field can hold it.
		if p := peer.tlsErr.Load(); p != nil {
			return *p
		}
		if p := peer.indicationErr.Load(); p != nil {
			return *p
		}
		time.Sleep(10 * time.Millisecond)
	}
	return res.Err
}

// requireRemoteAlert fails the test unless err is the TLS engine's report of an
// alert the other end sent. crypto/tls reports a received alert as a
// tls.AlertError, and wraps it as "remote error: tls: ..." in its message.
func requireRemoteAlert(t *testing.T, who string, err error) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s recorded no error, so no alert reached it", who)
	}
	var alert tls.AlertError
	if errors.As(err, &alert) {
		return
	}
	if strings.Contains(err.Error(), "remote error: tls:") {
		return
	}
	t.Fatalf("%s failed without receiving a TLS alert: %v", who, err)
}

// TestEAPTLS13CompletedExchangeCarriesNoAlert is the negative of the test above.
//
// RFC requirement: RFC9190-2.1.4-3 negative -- the alert follows a fatal error
// and nothing else: over a complete exchange neither end's TLS engine records a
// received alert, and both derive the same MSK. An end that closed every
// conversation with an alert, or sent one on the success path, fails here.
func TestEAPTLS13CompletedExchangeCarriesNoAlert(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(NewResumption(time.Now, true)))
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS13, eapTLS13Rounds)
	requireCompleted(t, fl, "a full EAP-TLS 1.3 exchange")
	if err := fl.sess.Err(); err != nil {
		t.Fatalf("the authenticator recorded an error on a completed exchange: %v", err)
	}
	if p := peer.tlsErr.Load(); p != nil {
		t.Fatalf("the peer's TLS engine recorded an error on a completed exchange: %v", *p)
	}
	for i, m := range eapTLSMessages(t, fl.serverSent) {
		for _, r := range tlsRecords(t, m) {
			if r.contentType == tlsRecordAlert {
				t.Errorf("authenticator message %d carries a plaintext alert record on a completed exchange", i)
			}
		}
	}
}

// keyMaterialExchange drives a complete EAP-TLS 1.3 exchange and answers the
// authenticator's MSK, its EMSK and its TLS connection state, read before the
// session's Close erases the EMSK.
func keyMaterialExchange(t *testing.T) (msk, emsk [64]byte, cs tls.ConnectionState) {
	t.Helper()
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(NewResumption(time.Now, true)))
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS13, eapTLS13Rounds)
	requireCompleted(t, fl, "a full EAP-TLS 1.3 exchange")
	method, ok := fl.sess.method.(*tlsMethod)
	if !ok {
		t.Fatalf("authenticator method is %T, want *tlsMethod", fl.sess.method)
	}
	cs = method.conn.ConnectionState()
	if cs.Version != tls.VersionTLS13 {
		t.Fatalf("negotiated %#04x, want TLS 1.3", cs.Version)
	}
	return fl.sess.msk, fl.sess.emsk, cs
}

// TestRFC9190KeyMaterialIsOneFullLengthExport compares the authenticator's MSK
// and EMSK with one 128-octet export under the RFC 9190 label.
//
// RFC requirement: RFC9190-2.3-2 positive -- "Note that the key derivation MUST
// use the length values given above": Key_Material is exported at 128 octets, the
// MSK is Key_Material(0, 63) and the EMSK is Key_Material(64, 127), each 64
// octets.
//
// RFC requirement: RFC9190-2.3-3 positive -- "If an implementation intends to
// use only a part of the output of the TLS-Exporter function, then it MUST ask
// for the full output and then only use the desired part." Ze needs both halves,
// and the two keys together are exactly the full 128-octet output.
func TestRFC9190KeyMaterialIsOneFullLengthExport(t *testing.T) {
	msk, emsk, cs := keyMaterialExchange(t)
	full, err := cs.ExportKeyingMaterial("EXPORTER_EAP_TLS_Key_Material", []byte{TypeTLS}, 128)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if [64]byte(full[:64]) != msk {
		t.Fatalf("MSK = %x\nwant Key_Material(0, 63) = %x", msk, full[:64])
	}
	if [64]byte(full[64:]) != emsk {
		t.Fatalf("EMSK = %x\nwant Key_Material(64, 127) = %x", emsk, full[64:])
	}
}

// TestRFC9190KeysAreNotAShorterExport is the negative of the test above. The TLS
// 1.3 exporter feeds the requested length into HKDF-Expand-Label (RFC 8446
// Section 7.5), so a shorter request yields different octets rather than a
// prefix, and an MSK taken from a 64-octet request is visible.
//
// RFC requirement: RFC9190-2.3-2 negative -- neither key is cut from an export
// at another length: the MSK is not a 64-octet export, and the EMSK is neither a
// 64-octet export nor the second half of a 256-octet one.
//
// RFC requirement: RFC9190-2.3-3 negative -- the MSK, the part ze uses first, is
// not the answer to a request for only that part.
func TestRFC9190KeysAreNotAShorterExport(t *testing.T) {
	msk, emsk, cs := keyMaterialExchange(t)
	export := func(length int) []byte {
		t.Helper()
		out, err := cs.ExportKeyingMaterial("EXPORTER_EAP_TLS_Key_Material", []byte{TypeTLS}, length)
		if err != nil {
			t.Fatalf("export of %d octets: %v", length, err)
		}
		return out
	}
	full, part, double := export(128), export(64), export(256)
	if bytes.Equal(full[:64], part) {
		t.Fatal("the exporter answered a 64-octet request with a prefix of the 128-octet one, so this test cannot see a short request")
	}
	if [64]byte(part) == msk {
		t.Fatal("the MSK is a 64-octet export: the implementation asked for only the part it uses")
	}
	if [64]byte(part) == emsk {
		t.Fatal("the EMSK is a 64-octet export, not Key_Material(64, 127)")
	}
	if [64]byte(double[64:128]) == emsk {
		t.Fatal("the EMSK is cut from a 256-octet export, not the 128 octets RFC 9190 Section 2.3 names")
	}
}
