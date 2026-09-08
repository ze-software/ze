// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP-TLS in the IKEv2 responder and initiator seats
// RFC: rfc/short/rfc9190.md -- EAP-TLS 1.3, Section 5.10 "Discovered Vulnerabilities"
// Related: eap_tls_handshake_test.go (the PKI and the exchange harness), rfc5216_success_flight_test.go (driveTunedEAPTLSFlight)
//
// RFC 9190 Section 5.10: "[RFC7457] summarizes the attacks that were known at
// the time of publishing, and BCP 195 [RFC7525] [RFC8996] provides
// recommendations and requirements for improving the security of deployed
// services that use TLS.  However, many of the attacks are less serious for
// EAP-TLS as EAP-TLS only uses the TLS handshake and does not protect any
// application data.  EAP-TLS implementations MUST mitigate known attacks."
//
// The requirement points at a list, so the list is what this file tests. RFC
// 7457 Section 2 enumerates fifteen attacks. Nine of them name a property an
// EAP-TLS implementation can hold, and each of the nine has a positive and a
// negative, here or in the sibling test named beside it.
//
// | # | RFC 7457 | Positive | Negative |
// |---|----------|----------|----------|
// | 1 | 2.3 BEAST, 2.4 padding oracles | TestRFC9190MitigationNegotiatesTLS12OrAbove | TestRFC9190MitigationRefusesTLS11 |
// | 2 | 2.11 Triple Handshake | TestRFC9190MitigationOffersExtendedMasterSecret | TestEAPTLSExportRefusalNamesTheCause (eap_tls_export_refusal_test.go) |
// | 3 | 2.10 Renegotiation | TestRFC9190MitigationOffersSecureRenegotiationInfo | TestRFC9190MitigationOffersNoPostHandshakeAuthentication |
// | 4 | 2.7 Certificate and RSA | TestRFC9190MitigationNegotiatesEphemeralKeyExchange | TestRFC9190MitigationOffersNoStaticRSAKeyExchange |
// | 5 | 2.5 RC4 | TestRFC9190MitigationNegotiatesAnAEADSuite | TestRFC9190MitigationOffersNoRC4Suite |
// | 6 | 2.6 CRIME, TIME, BREACH | TestRFC9190MitigationOffersOnlyNullCompression | TestRFC9190MitigationRefusesACompressingClientHello |
// | 7 | 2.9 Diffie-Hellman parameters | TestRFC9190MitigationUsesANamedGroup | TestRFC9190MitigationOffersNoFiniteFieldDHGroup |
// | 8 | 2.1 SSL stripping, 2.2 STARTTLS injection, 2.12 virtual host confusion | TestRFC9190MitigationCarriesNoServerName | TestRFC9190MitigationRefusesAMethodDowngrade, TestRFC9190MitigationDropsBytesPipelinedBehindTheStart |
// | 9 | 2.15 Usability | TestEAPTLSMutualAuthHandshakeSucceeds (eap_tls_handshake_test.go) | TestEAPTLSPeerWithoutCARefusesToStart (eap_tls_handshake_test.go) |
//
// The certificate path validation half of row 4 is proven in
// eap_tls_handshake_test.go (TestEAPTLSServerRejectsUntrustedClientChain,
// TestEAPTLSPeerRejectsUntrustedServerChain) and its revocation half in
// rfc9190_revocation_test.go. A polarity already proven is CITED, never copied:
// a second assertion over the same behavior is not more coverage.
//
// Three of the fifteen are named here so no reader has to work out why they are
// absent. RFC 7457 Section 2.8 (theft of private keys) is a key storage
// property, Section 2.13 (denial of service) is a capacity property, and
// Section 2.14 (implementation issues) is a property of the TLS library rather
// than of the EAP-TLS code that drives it. None of the three is a behavior an
// EAP-TLS exchange exhibits, so none has an assertion to make.
//
// VALIDATES: the version floor, the key exchange, the cipher suite, the
// compression method, the absence of a server name, the refusal of a weaker EAP
// method, and the fate of octets pipelined behind an EAP-TLS Start, each read
// off a real exchange between the real authenticator and the real peer.
// PREVENTS: a later change that lowers the floor, offers a broken suite, adds a
// server name, or feeds unauthenticated pipelined octets to crypto/tls, landing
// with every existing test still green.

package eap

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"io"
	"net"
	"slices"
	"strings"
	"testing"
	"time"
)

// TLS ClientHello extension identifiers this file reads (IANA TLS ExtensionType
// Values). crypto/tls declares each of them unexported, so they are written out
// here rather than reached for.
const (
	tlsExtServerName           uint16 = 0
	tlsExtSupportedGroups      uint16 = 10
	tlsExtExtendedMasterSecret uint16 = 23
	tlsExtSupportedVersions    uint16 = 43
	tlsExtPostHandshakeAuth    uint16 = 49
	tlsExtRenegotiationInfo    uint16 = 0xff01
)

// The finite-field Diffie-Hellman named groups of RFC 7919 Section 2, which are
// the ffdhe2048 to ffdhe8192 codepoints. A group from this range in the offer
// would put finite-field parameters back on the wire, which RFC 7457 Section
// 2.9 says a client has to verify.
const (
	tlsGroupFFDHELow  uint16 = 0x0100
	tlsGroupFFDHEHigh uint16 = 0x01ff
)

// tlsFiniteFieldDHSuites are the finite-field Diffie-Hellman cipher suites of
// RFC 5246 Appendix A.5. crypto/tls implements them for neither role and
// therefore declares them nowhere. RFC 7457 Section 2.9 is about the parameters
// these suites carry, so their absence from the offer is what puts that attack
// out of reach.
var tlsFiniteFieldDHSuites = []uint16{
	0x0016, // TLS_DHE_RSA_WITH_3DES_EDE_CBC_SHA
	0x0033, // TLS_DHE_RSA_WITH_AES_128_CBC_SHA
	0x0039, // TLS_DHE_RSA_WITH_AES_256_CBC_SHA
	0x0067, // TLS_DHE_RSA_WITH_AES_128_CBC_SHA256
	0x006b, // TLS_DHE_RSA_WITH_AES_256_CBC_SHA256
	0x009e, // TLS_DHE_RSA_WITH_AES_128_GCM_SHA256
	0x009f, // TLS_DHE_RSA_WITH_AES_256_GCM_SHA384
}

// tlsRC4Suites are the three RC4 cipher suites crypto/tls implements. They sit
// in its disabledCipherSuites set, so defaultCipherSuites drops them and a
// tls.Config naming no CipherSuites offers none of them (defaultCipherSuites,
// crypto/tls/defaults.go). Ze names none, and the assertions below are over the
// offer that reached the wire rather than over that source.
var tlsRC4Suites = []uint16{
	tls.TLS_RSA_WITH_RC4_128_SHA,
	tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
	tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,
}

// tlsStaticRSASuites are the cipher suites whose key exchange is RSA key
// transport rather than an ephemeral exchange. RFC 7457 Section 2.7 names
// Bleichenbacher and Klima, and both attack that key transport. Section 2.8
// names forward secrecy as the remedy, which only an ephemeral exchange gives.
var tlsStaticRSASuites = []uint16{
	tls.TLS_RSA_WITH_RC4_128_SHA,
	tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	tls.TLS_RSA_WITH_AES_256_CBC_SHA,
	tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
	tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
}

// tlsAEADSuites are the authenticated-encryption cipher suites a conformant
// exchange can land on. RFC 7457 Section 2.4 names AES-GCM as the mitigation for
// the padding oracle attacks that follow from MAC-then-encrypt, and the TLS 1.3
// suites are AEAD by construction (RFC 8446 Appendix B.4).
var tlsAEADSuites = []uint16{
	tls.TLS_AES_128_GCM_SHA256,
	tls.TLS_AES_256_GCM_SHA384,
	tls.TLS_CHACHA20_POLY1305_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
}

// tlsNamedGroups are the key exchange groups crypto/tls offers, in the order
// curvePreferenceOrder declares them (crypto/tls/defaults.go). Each one is a
// predefined group, which is what RFC 7457 Section 2.9 asks for.
var tlsNamedGroups = []tls.CurveID{
	tls.X25519MLKEM768,
	tls.SecP256r1MLKEM768,
	tls.SecP384r1MLKEM1024,
	tls.MLKEM1024,
	tls.X25519,
	tls.CurveP256,
	tls.CurveP384,
	tls.CurveP521,
}

// attackExchange is one completed EAP-TLS exchange with everything the
// mitigation tests read off it: what the peer put on the wire, what the
// authenticator parsed out of it, and what the two ends settled on.
type attackExchange struct {
	// hello is the peer's ClientHello, reassembled from the EAP-TLS fragments
	// the peer sent and decoded field by field.
	hello *rawClientHello

	// offer is the ClientHelloInfo crypto/tls handed the authenticator for that
	// same ClientHello. It carries the parsed supported versions, cipher suites
	// and supported groups, which the raw message carries only as extension
	// bodies this file does not decode.
	offer *tls.ClientHelloInfo

	peerState tls.ConnectionState
	authState tls.ConnectionState
}

// driveAttackExchange runs one full EAP-TLS exchange between the real
// authenticator and the real peer, and returns what both ends produced.
//
// maxTLSVersion caps the AUTHENTICATOR's tls.Config, so a caller can hold the
// exchange at TLS 1.2 and read the same properties there. Zero leaves the
// authenticator unconstrained, which lands on TLS 1.3.
//
// It fails the test when the exchange does not complete. Every assertion in this
// file is about a SUCCESSFUL exchange, and a property read off a handshake that
// never finished says nothing about the one an operator gets.
func driveAttackExchange(t *testing.T, pki *eapTLSPKI, maxTLSVersion uint16) *attackExchange {
	t.Helper()

	peer := newAttackPeer(pki)

	var offer *tls.ClientHelloInfo
	flight := driveTunedEAPTLSFlight(t, pki.serverConfig(), peer, maxTLSVersion, 40, func(cfg *tls.Config) {
		// GetConfigForClient is the only hook crypto/tls offers onto the
		// ClientHello as it parsed it. Returning nil keeps the config the
		// authenticator built, so nothing about the exchange changes: this reads
		// the offer, it does not shape it (Config.GetConfigForClient,
		// crypto/tls/common.go).
		cfg.GetConfigForClient = func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
			seen := *hello
			offer = &seen
			return nil, nil //nolint:nilnil // a nil config with a nil error is how crypto/tls is told to keep the one the authenticator built
		}
	})

	if !flight.peerDone {
		t.Fatalf("the EAP-TLS exchange did not complete: peerErr=%v successAt=%d failureAt=%d", flight.peerErr, flight.successAt, flight.failureAt)
	}
	if offer == nil {
		t.Fatal("the authenticator saw no ClientHello, so there is nothing to read")
	}

	method, ok := flight.sess.method.(*tlsMethod)
	if !ok {
		t.Fatalf("authenticator method is %T, want *tlsMethod", flight.sess.method)
	}

	return &attackExchange{
		hello:     parseClientHello(t, flight),
		offer:     offer,
		peerState: peer.tlsConn.ConnectionState(),
		authState: method.conn.ConnectionState(),
	}
}

// newAttackPeer builds the EAP-TLS peer these tests drive, over the harness PKI.
// The caller owns its lifetime: driveTunedEAPTLSFlight closes the peer it is
// given, and a test that drives the peer itself closes it in a cleanup.
func newAttackPeer(pki *eapTLSPKI) *PeerSession {
	return NewPeerSessionTLS("eap-tls-client", &PeerTLSConfig{
		CertPEM:    pki.clientCertPEM,
		KeyPEM:     pki.clientKeyPEM,
		CACertPEM:  pki.trustedCAPEM,
		CRLPEM:     pki.trustedCRLPEM,
		Resumption: NewResumption(time.Now, true),
	})
}

// rawClientHello is the peer's ClientHello as it left ze, decoded from the
// handshake record the EAP-TLS fragments carried.
//
// The decoded fields answer what tls.ClientHelloInfo cannot: the legacy version
// octets, the compression methods, and which extensions are present. RFC 8996
// Section 4 and Section 5 are written about the first, RFC 7457 Section 2.6
// about the second, and Section 2.10 about the third.
type rawClientHello struct {
	// record is the whole first TLS flight, its record header included, ready to
	// be replayed at an authenticator.
	record []byte

	legacyVersion      uint16
	cipherSuites       []uint16
	compressionMethods []byte
	extensions         []uint16

	// compressionOffset is where the first compression method octet sits inside
	// record, so a test can replay the same ClientHello with that one octet
	// changed and nothing else.
	compressionOffset int
}

// hasExtension reports whether the ClientHello carried the extension named.
func (h *rawClientHello) hasExtension(ext uint16) bool {
	return slices.Contains(h.extensions, ext)
}

// helloCursor reads a ClientHello left to right, failing the test at the first
// field that runs off the end.
//
// A length in this message is attacker-chosen in general. Here it is ze's own,
// so a short read is a defect in this decoder or in the encoder rather than a
// case to handle.
type helloCursor struct {
	t   *testing.T
	buf []byte
	off int
}

func (c *helloCursor) take(octets int, field string) []byte {
	c.t.Helper()
	if c.off+octets > len(c.buf) {
		c.t.Fatalf("the ClientHello is short: %s wants %d octets at offset %d of %d", field, octets, c.off, len(c.buf))
	}
	out := c.buf[c.off : c.off+octets]
	c.off += octets
	return out
}

func (c *helloCursor) takeUint8(field string) int {
	c.t.Helper()
	return int(c.take(1, field)[0])
}

func (c *helloCursor) takeUint16(field string) int {
	c.t.Helper()
	return int(binary.BigEndian.Uint16(c.take(2, field)))
}

// parseClientHello reassembles the peer's first EAP-TLS flight and decodes the
// ClientHello inside it.
//
// The flight is fragmented. RFC 5216 Section 2.1.5 puts the L and M flags on the
// first fragment and clears M on the last, eapTLSFragmentSize is 1024, and a TLS
// 1.3 ClientHello carrying an ML-KEM key share is over 1500 octets. So the
// payloads are concatenated until the M flag clears, which is what the
// authenticator's own tlsFragmenter.reassemble does with the same packets.
func parseClientHello(t *testing.T, flight *eapTLSFlight) *rawClientHello {
	t.Helper()

	var record []byte
	started := false
	for _, pkt := range flight.peerSent {
		// The peer's first response is the EAP-Response/Identity, which carries
		// no EAP-TLS header at all. The TLS flight starts at the first
		// EAP-Response/EAP-TLS carrying a payload behind its flags octet.
		if pkt == nil || pkt.Type != TypeTLS || len(pkt.TypeData) < 2 {
			if started {
				break
			}
			continue
		}
		payload := 1
		if pkt.TypeData[0]&eapTLSFlagL != 0 {
			payload = 5
		}
		started = true
		record = append(record, pkt.TypeData[payload:]...)
		if pkt.TypeData[0]&eapTLSFlagM == 0 {
			break
		}
	}
	if len(record) == 0 {
		t.Fatal("the peer sent no EAP-TLS message carrying TLS data")
	}

	cur := &helloCursor{t: t, buf: record}
	if contentType := cur.takeUint8("record content type"); contentType != int(tlsRecordHandshake) {
		t.Fatalf("the first record content type is %#02x, want %#02x (handshake)", contentType, tlsRecordHandshake)
	}
	cur.take(2, "record version")
	cur.take(2, "record length")
	if msgType := cur.takeUint8("handshake message type"); msgType != int(tlsHandshakeCliHlo) {
		t.Fatalf("the first handshake message type is %#02x, want %#02x (ClientHello)", msgType, tlsHandshakeCliHlo)
	}
	cur.take(3, "handshake message length")

	hello := &rawClientHello{record: record}
	hello.legacyVersion = uint16(cur.takeUint16("legacy_version"))
	cur.take(32, "random")
	cur.take(cur.takeUint8("legacy_session_id length"), "legacy_session_id")

	suites := cur.take(cur.takeUint16("cipher_suites length"), "cipher_suites")
	for off := 0; off+2 <= len(suites); off += 2 {
		hello.cipherSuites = append(hello.cipherSuites, binary.BigEndian.Uint16(suites[off:off+2]))
	}

	compressionOctets := cur.takeUint8("legacy_compression_methods length")
	hello.compressionOffset = cur.off
	hello.compressionMethods = cur.take(compressionOctets, "legacy_compression_methods")

	end := cur.off + 2 + cur.takeUint16("extensions length")
	for cur.off+4 <= end {
		ext := uint16(cur.takeUint16("extension type"))
		cur.take(cur.takeUint16("extension length"), "extension body")
		hello.extensions = append(hello.extensions, ext)
	}
	return hello
}

// replayClientHello writes one ClientHello record at a fresh authenticator built
// from the production MethodConfig, and returns the content type of the first
// TLS record the authenticator answers with.
//
// A handshake record (0x16) means the ClientHello was accepted far enough to
// produce a ServerHello. An alert record (0x15) means it was refused. Those two
// answers are what let a one-octet edit to the same message discriminate, rather
// than an assertion over an error string.
func replayClientHello(t *testing.T, pki *eapTLSPKI, record []byte) byte {
	t.Helper()

	sess, err := NewSession(TypeTLS, pki.serverConfig())
	if err != nil {
		t.Fatalf("create the authenticator session: %v", err)
	}
	t.Cleanup(sess.Close)
	method, ok := sess.method.(*tlsMethod)
	if !ok {
		t.Fatalf("authenticator method is %T, want *tlsMethod", sess.method)
	}

	clientSide, serverSide := net.Pipe()
	t.Cleanup(func() {
		clientSide.Close() //nolint:errcheck // the pipe is torn down at the end of the test; its close error changes no assertion
	})

	// net.Pipe is unbuffered and synchronous, so the authenticator's first write
	// blocks until this side reads it. The reader therefore starts BEFORE the
	// ClientHello goes out, and it hands the answer back on a channel.
	answer := make(chan byte, 1)
	go func() {
		header := make([]byte, tlsRecordHeaderLen)
		if _, err := io.ReadFull(clientSide, header); err != nil {
			// The authenticator closed without writing a record. Zero is no TLS
			// content type, so the assertion below names it as neither the
			// handshake record nor the alert.
			answer <- 0
			return
		}
		answer <- header[0]
		io.Copy(io.Discard, clientSide) //nolint:errcheck // the pipe is drained so the authenticator never blocks writing the rest of its flight
	}()

	go func() {
		conn := tls.Server(serverSide, method.tlsConfig)
		conn.HandshakeContext(context.Background()) //nolint:errcheck // the assertion is the record the authenticator wrote, captured by the reader above
		conn.Close()                                //nolint:errcheck // the pipe is torn down with the test
	}()

	if _, err := clientSide.Write(record); err != nil {
		t.Fatalf("write the ClientHello to the authenticator: %v", err)
	}

	select {
	case contentType := <-answer:
		return contentType
	case <-time.After(10 * time.Second):
		t.Fatal("the authenticator answered no TLS record within 10s")
		return 0
	}
}

// TestRFC9190MitigationNegotiatesTLS12OrAbove drives two complete EAP-TLS
// exchanges, one with the authenticator unconstrained and one capped at TLS 1.2,
// and asserts both ends land on TLS 1.2 or above in each.
//
// RFC requirement: RFC9190-5.10-1 positive -- a completed EAP-TLS exchange
// negotiates TLS 1.2 or above on the authenticator and on the peer, both with
// the authenticator unconstrained and with it capped at TLS 1.2, and the peer's
// ClientHello offers no version below TLS 1.2. RFC 8996 Section 4: "TLS 1.0 MUST
// NOT be used.  Negotiation of TLS 1.0 from any version of TLS MUST NOT be
// permitted." Section 5 says the same of TLS 1.1. That floor is the mitigation
// RFC 7457 Section 2.3 (BEAST, a TLS 1.0 CBC weakness) and Section 2.4 (padding
// oracles) call for.
func TestRFC9190MitigationNegotiatesTLS12OrAbove(t *testing.T) {
	pki := newEAPTLSPKI(t)

	for _, tc := range []struct {
		name       string
		maxVersion uint16
	}{
		{name: "unconstrained", maxVersion: 0},
		{name: "capped at TLS 1.2", maxVersion: tls.VersionTLS12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exchange := driveAttackExchange(t, pki, tc.maxVersion)

			if exchange.authState.Version < tls.VersionTLS12 {
				t.Errorf("the authenticator negotiated %s, want TLS 1.2 or above", tls.VersionName(exchange.authState.Version))
			}
			if exchange.peerState.Version < tls.VersionTLS12 {
				t.Errorf("the peer negotiated %s, want TLS 1.2 or above", tls.VersionName(exchange.peerState.Version))
			}

			if !exchange.hello.hasExtension(tlsExtSupportedVersions) {
				t.Fatal("the ClientHello carries no supported_versions extension, so the offer cannot be read")
			}
			for _, version := range exchange.offer.SupportedVersions {
				if version < tls.VersionTLS12 {
					t.Errorf("the ClientHello offers %s, which RFC 8996 forbids", tls.VersionName(version))
				}
			}
		})
	}
}

// TestRFC9190MitigationRefusesTLS11 puts a TLS 1.1 party on each side of the
// exchange in turn, and asserts neither reaches a session.
//
// RFC requirement: RFC9190-5.10-1 negative -- the authenticator's own tls.Config
// refuses a client offering only TLS 1.0 and TLS 1.1, at the handshake, with an
// error naming the unsupported versions; and an authenticator offering only TLS
// 1.0 and TLS 1.1 completes no exchange with ze's peer, which reports a handshake
// error, receives no EAP-Success and derives no MSK. RFC 8996 Section 5: "TLS 1.1
// MUST NOT be used.  Negotiation of TLS 1.1 from any version of TLS MUST NOT be
// permitted."
//
// The floor is MinVersion on both roles: newTLSMethod (eap_tls.go) sets it for
// the authenticator and startTLSClient (peer.go) for the peer.
func TestRFC9190MitigationRefusesTLS11(t *testing.T) {
	pki := newEAPTLSPKI(t)

	t.Run("the authenticator refuses a TLS 1.1 client", func(t *testing.T) {
		sess, err := NewSession(TypeTLS, pki.serverConfig())
		if err != nil {
			t.Fatalf("create the authenticator session: %v", err)
		}
		t.Cleanup(sess.Close)
		method, ok := sess.method.(*tlsMethod)
		if !ok {
			t.Fatalf("authenticator method is %T, want *tlsMethod", sess.method)
		}

		clientSide, serverSide := net.Pipe()

		refusal := make(chan error, 1)
		go func() {
			conn := tls.Server(serverSide, method.tlsConfig)
			refusal <- conn.HandshakeContext(context.Background())
			conn.Close() //nolint:errcheck // the pipe is torn down with the test
		}()

		// InsecureSkipVerify because this client never reaches a certificate:
		// the authenticator refuses its ClientHello on the version alone, which
		// is what this subtest asserts.
		client := tls.Client(clientSide, &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // the refusal under test happens at the ClientHello, before any certificate crosses
			MinVersion:         tls.VersionTLS10,
			MaxVersion:         tls.VersionTLS11,
		})
		client.HandshakeContext(context.Background()) //nolint:errcheck // the assertion is the authenticator's refusal below, not the alert this side receives
		client.Close()                                //nolint:errcheck // the pipe is torn down with the test

		err = <-refusal
		if err == nil {
			t.Fatal("the authenticator completed a handshake with a client offering only TLS 1.0 and TLS 1.1")
		}
		if !strings.Contains(err.Error(), "unsupported versions") {
			t.Errorf("the authenticator refused with %q, want a refusal naming the unsupported versions", err)
		}
	})

	t.Run("the peer reaches no session with a TLS 1.1 authenticator", func(t *testing.T) {
		flight := driveTunedEAPTLSFlight(t, pki.serverConfig(), newAttackPeer(pki), 0, 40, func(cfg *tls.Config) {
			cfg.MinVersion = tls.VersionTLS10
			cfg.MaxVersion = tls.VersionTLS11
		})

		if flight.peerDone {
			t.Error("the peer completed an exchange with an authenticator offering only TLS 1.0 and TLS 1.1")
		}
		if flight.peerErr == nil {
			t.Error("the peer reported no error against a TLS 1.1 authenticator")
		}
		if flight.successAt >= 0 {
			t.Errorf("the authenticator sent EAP-Success at packet %d of a TLS 1.1 exchange", flight.successAt)
		}
		if flight.peerMSK != [64]byte{} {
			t.Error("the peer derived an MSK from an exchange that negotiated no TLS version")
		}
	})
}

// TestRFC9190MitigationOffersExtendedMasterSecret asserts the peer's ClientHello
// offers the RFC 7627 extended master secret.
//
// RFC requirement: RFC9190-5.10-1 positive -- the peer's ClientHello carries the
// extended_master_secret extension (type 23), which is the RFC 7627 session hash
// binding the master secret to the handshake it came from. RFC 7457 Section 2.11:
// "The triple handshake attack [BhargavanDFPS14] enables the attacker to cause
// two TLS connections to share keying material."
//
// The negative is already proven and is not copied here: a TLS 1.2 session that
// did NOT negotiate the extended master secret exports no EAP-TLS MSK at all
// (exportEAPTLSMSK and eapTLS12ExportRefused, eap_tls.go), which
// TestEAPTLSExportRefusalNamesTheCause asserts in eap_tls_export_refusal_test.go.
func TestRFC9190MitigationOffersExtendedMasterSecret(t *testing.T) {
	pki := newEAPTLSPKI(t)
	exchange := driveAttackExchange(t, pki, tls.VersionTLS12)

	if !exchange.hello.hasExtension(tlsExtExtendedMasterSecret) {
		t.Errorf("the ClientHello offers extensions %#04x, which carry no extended_master_secret (%#04x)", exchange.hello.extensions, tlsExtExtendedMasterSecret)
	}
}

// TestRFC9190MitigationOffersSecureRenegotiationInfo asserts the peer's
// ClientHello carries the RFC 5746 renegotiation_info extension.
//
// RFC requirement: RFC9190-5.10-1 positive -- the peer's ClientHello carries the
// renegotiation_info extension (type 0xff01), which is the extension RFC 5746
// defines to bind a renegotiation to the connection under it. RFC 7457 Section
// 2.10: "A major attack on the TLS renegotiation mechanism applies to all current
// versions of the protocol.  The attack and the TLS extension that resolves it
// are described in [RFC5746]."
//
// crypto/tls sets the extension on the ClientHello it builds (makeClientHello,
// crypto/tls/handshake_client.go).
func TestRFC9190MitigationOffersSecureRenegotiationInfo(t *testing.T) {
	pki := newEAPTLSPKI(t)
	exchange := driveAttackExchange(t, pki, tls.VersionTLS12)

	if !exchange.hello.hasExtension(tlsExtRenegotiationInfo) {
		t.Errorf("the ClientHello offers extensions %#04x, which carry no renegotiation_info (%#04x)", exchange.hello.extensions, tlsExtRenegotiationInfo)
	}
}

// TestRFC9190MitigationOffersNoPostHandshakeAuthentication asserts the peer never
// offers to be authenticated again after the handshake.
//
// RFC requirement: RFC9190-5.10-1 negative -- the peer's ClientHello carries no
// post_handshake_auth extension (type 49), which RFC 8446 Section 4.6.2 makes the
// precondition for a post-handshake CertificateRequest, so an authenticator
// cannot ask this peer to authenticate again on the same connection. RFC 9190
// Section 2.1: "Note that the EAP-TLS server must not request post-handshake
// client authentication."
//
// crypto/tls implements post-handshake authentication for neither role
// (handlePostHandshakeMessage, crypto/tls/conn.go).
func TestRFC9190MitigationOffersNoPostHandshakeAuthentication(t *testing.T) {
	pki := newEAPTLSPKI(t)
	exchange := driveAttackExchange(t, pki, 0)

	if exchange.hello.hasExtension(tlsExtPostHandshakeAuth) {
		t.Errorf("the ClientHello offers post_handshake_auth (%#04x), which RFC 9190 Section 2.1 rules out", tlsExtPostHandshakeAuth)
	}
}

// TestRFC9190MitigationNegotiatesEphemeralKeyExchange asserts the completed
// exchange ran an ephemeral key exchange rather than RSA key transport.
//
// RFC requirement: RFC9190-5.10-1 positive -- the cipher suite a completed
// EAP-TLS exchange negotiates is none of the seven RSA key transport suites, and
// both ends report the same non-zero key exchange group, at TLS 1.3 and again at
// TLS 1.2. RFC 7457 Section 2.7: "There have been several practical attacks on
// TLS when used with RSA certificates (the most common use case).  These include
// [Bleichenbacher98] and [Klima03]." Section 2.8 names forward secrecy, which an
// ephemeral exchange gives and key transport does not.
//
// The certificate path validation half of RFC 7457 Section 2.7 is proven
// elsewhere and is not copied here: TestEAPTLSMutualAuthHandshakeSucceeds,
// TestEAPTLSServerRejectsUntrustedClientChain and
// TestEAPTLSPeerRejectsUntrustedServerChain (eap_tls_handshake_test.go), and the
// revocation tests in rfc9190_revocation_test.go.
func TestRFC9190MitigationNegotiatesEphemeralKeyExchange(t *testing.T) {
	pki := newEAPTLSPKI(t)

	for _, tc := range []struct {
		name       string
		maxVersion uint16
	}{
		{name: "unconstrained", maxVersion: 0},
		{name: "capped at TLS 1.2", maxVersion: tls.VersionTLS12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exchange := driveAttackExchange(t, pki, tc.maxVersion)

			if slices.Contains(tlsStaticRSASuites, exchange.authState.CipherSuite) {
				t.Errorf("the exchange negotiated %s, which is RSA key transport", tls.CipherSuiteName(exchange.authState.CipherSuite))
			}
			if exchange.authState.CurveID == 0 {
				t.Error("the authenticator names no key exchange group, so no ephemeral exchange was run")
			}
			if exchange.peerState.CurveID != exchange.authState.CurveID {
				t.Errorf("the peer ran key exchange %v and the authenticator %v", exchange.peerState.CurveID, exchange.authState.CurveID)
			}
		})
	}
}

// TestRFC9190MitigationOffersNoStaticRSAKeyExchange asserts the peer offers no
// RSA key transport suite, so no authenticator can select one.
//
// RFC requirement: RFC9190-5.10-1 negative -- the peer's ClientHello offers none
// of the seven RSA key transport cipher suites crypto/tls implements, neither in
// its raw cipher_suites field nor in the offer the authenticator parsed, so the
// Bleichenbacher and Klima attacks of RFC 7457 Section 2.7 have no suite to run
// against.
func TestRFC9190MitigationOffersNoStaticRSAKeyExchange(t *testing.T) {
	pki := newEAPTLSPKI(t)
	exchange := driveAttackExchange(t, pki, tls.VersionTLS12)

	for _, suite := range tlsStaticRSASuites {
		if slices.Contains(exchange.hello.cipherSuites, suite) {
			t.Errorf("the ClientHello offers %s, which is RSA key transport", tls.CipherSuiteName(suite))
		}
		if slices.Contains(exchange.offer.CipherSuites, suite) {
			t.Errorf("the authenticator was offered %s, which is RSA key transport", tls.CipherSuiteName(suite))
		}
	}
}

// TestRFC9190MitigationNegotiatesAnAEADSuite asserts the completed exchange is
// protected by authenticated encryption.
//
// RFC requirement: RFC9190-5.10-1 positive -- the cipher suite a completed
// EAP-TLS exchange negotiates is one of the nine AEAD suites this file names, on
// the authenticator and on the peer, at TLS 1.3 and again at TLS 1.2, so it is
// neither RC4 nor a MAC-then-encrypt suite. RFC 7457 Section 2.4: "The Lucky
// Thirteen attack can be mitigated by using authenticated encryption like AES-GCM
// [RFC5288] or encrypt-then-MAC [RFC7366] instead of the TLS default of
// MAC-then-encrypt."
//
// Ze names no CipherSuites on either role, so the offer is Go's default suite
// set and the selection is Go's preference order.
func TestRFC9190MitigationNegotiatesAnAEADSuite(t *testing.T) {
	pki := newEAPTLSPKI(t)

	for _, tc := range []struct {
		name       string
		maxVersion uint16
	}{
		{name: "unconstrained", maxVersion: 0},
		{name: "capped at TLS 1.2", maxVersion: tls.VersionTLS12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exchange := driveAttackExchange(t, pki, tc.maxVersion)

			if !slices.Contains(tlsAEADSuites, exchange.authState.CipherSuite) {
				t.Errorf("the authenticator negotiated %s, which is not one of the AEAD suites", tls.CipherSuiteName(exchange.authState.CipherSuite))
			}
			if !slices.Contains(tlsAEADSuites, exchange.peerState.CipherSuite) {
				t.Errorf("the peer negotiated %s, which is not one of the AEAD suites", tls.CipherSuiteName(exchange.peerState.CipherSuite))
			}
		})
	}
}

// TestRFC9190MitigationOffersNoRC4Suite asserts RC4 is never on the table.
//
// RFC requirement: RFC9190-5.10-1 negative -- the peer's ClientHello offers none
// of the three RC4 cipher suites crypto/tls implements, neither in its raw
// cipher_suites field nor in the offer the authenticator parsed, and the
// negotiated suite is none of them either. RFC 7457 Section 2.5: "As a result,
// RC4 can no longer be seen as providing a sufficient level of security for TLS
// sessions."
//
// The three suites stay in Go's disabled set, which the default offer drops
// (defaultCipherSuites, crypto/tls/defaults.go).
func TestRFC9190MitigationOffersNoRC4Suite(t *testing.T) {
	pki := newEAPTLSPKI(t)
	exchange := driveAttackExchange(t, pki, tls.VersionTLS12)

	for _, suite := range tlsRC4Suites {
		if slices.Contains(exchange.hello.cipherSuites, suite) {
			t.Errorf("the ClientHello offers %s", tls.CipherSuiteName(suite))
		}
		if slices.Contains(exchange.offer.CipherSuites, suite) {
			t.Errorf("the authenticator was offered %s", tls.CipherSuiteName(suite))
		}
	}
	if slices.Contains(tlsRC4Suites, exchange.authState.CipherSuite) {
		t.Errorf("the exchange negotiated %s", tls.CipherSuiteName(exchange.authState.CipherSuite))
	}
}

// TestRFC9190MitigationOffersOnlyNullCompression asserts the peer offers TLS
// record compression to nobody.
//
// RFC requirement: RFC9190-5.10-1 positive -- the peer's ClientHello carries
// exactly one legacy_compression_methods octet and its value is null (0), so no
// TLS-level compression can be negotiated. RFC 7457 Section 2.6: "The CRIME
// attack [CRIME] (CVE-2012-4929) allows an active attacker to decrypt ciphertext
// (specifically, cookies) when TLS is used with TLS-level compression."
//
// crypto/tls implements no TLS record compression for either role, and ze adds
// none of its own.
func TestRFC9190MitigationOffersOnlyNullCompression(t *testing.T) {
	pki := newEAPTLSPKI(t)
	exchange := driveAttackExchange(t, pki, 0)

	if len(exchange.hello.compressionMethods) != 1 || exchange.hello.compressionMethods[0] != 0 {
		t.Errorf("the ClientHello offers compression methods %#02x, want the single null method 0x00", exchange.hello.compressionMethods)
	}
}

// TestRFC9190MitigationRefusesACompressingClientHello replays ze's own
// ClientHello at ze's own authenticator twice, once unchanged and once with its
// single compression method changed from null to DEFLATE, and asserts the
// authenticator answers a ServerHello to the first and an alert to the second.
//
// The two replays differ in ONE octet, so nothing else can account for the alert.
//
// RFC requirement: RFC9190-5.10-1 negative -- the authenticator answers a
// ClientHello offering DEFLATE compression with a TLS alert record rather than a
// handshake record, while the same ClientHello offering the null method gets a
// handshake record. RFC 7457 Section 2.6: "The TIME attack can be mitigated by
// disabling TLS compression."
//
// The refusal is made over the tls.Config newTLSMethod built (eap_tls.go), which
// is the config every EAP-TLS authenticator session runs.
func TestRFC9190MitigationRefusesACompressingClientHello(t *testing.T) {
	pki := newEAPTLSPKI(t)
	exchange := driveAttackExchange(t, pki, 0)

	accepted := replayClientHello(t, pki, exchange.hello.record)
	if accepted != tlsRecordHandshake {
		t.Fatalf("the authenticator answered ze's own ClientHello with record type %#02x, want %#02x (handshake)", accepted, tlsRecordHandshake)
	}

	compressing := slices.Clone(exchange.hello.record)
	compressing[exchange.hello.compressionOffset] = 1 // DEFLATE, RFC 3749 Section 2.

	refused := replayClientHello(t, pki, compressing)
	if refused != tlsRecordAlert {
		t.Errorf("the authenticator answered a DEFLATE ClientHello with record type %#02x, want %#02x (alert)", refused, tlsRecordAlert)
	}
}

// TestRFC9190MitigationUsesANamedGroup asserts the key exchange ran over a
// predefined group rather than over parameters the other end chose.
//
// RFC requirement: RFC9190-5.10-1 positive -- the key exchange group of a
// completed EAP-TLS exchange, at TLS 1.3 and again at TLS 1.2, is one of the
// eight named groups crypto/tls offers, and the peer and the authenticator agree
// on which one it was. RFC 7457 Section 2.9: "Using predefined DH groups, as
// proposed in [FFDHE-TLS], would mitigate this attack."
//
// Ze names no CurvePreferences on either role, so the offer is Go's own named
// group order (curvePreferenceOrder, crypto/tls/defaults.go).
func TestRFC9190MitigationUsesANamedGroup(t *testing.T) {
	pki := newEAPTLSPKI(t)

	for _, tc := range []struct {
		name       string
		maxVersion uint16
	}{
		{name: "unconstrained", maxVersion: 0},
		{name: "capped at TLS 1.2", maxVersion: tls.VersionTLS12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			exchange := driveAttackExchange(t, pki, tc.maxVersion)

			if !slices.Contains(tlsNamedGroups, exchange.authState.CurveID) {
				t.Errorf("the exchange used key exchange group %v, which is not one of the named groups", exchange.authState.CurveID)
			}
			if exchange.peerState.CurveID != exchange.authState.CurveID {
				t.Errorf("the peer used group %v and the authenticator %v", exchange.peerState.CurveID, exchange.authState.CurveID)
			}
		})
	}
}

// TestRFC9190MitigationOffersNoFiniteFieldDHGroup asserts no finite-field
// Diffie-Hellman parameters can reach the peer.
//
// RFC requirement: RFC9190-5.10-1 negative -- the peer's ClientHello offers no
// finite-field Diffie-Hellman cipher suite and no ffdhe named group, so an
// authenticator has no way to send it Diffie-Hellman parameters to verify. RFC
// 7457 Section 2.9: "In addition, clients that do not properly verify the
// received parameters are exposed to man-in-the-middle (MITM) attacks."
//
// crypto/tls implements no finite-field Diffie-Hellman key exchange, so the
// absence holds for both roles (curvePreferenceOrder, crypto/tls/defaults.go).
func TestRFC9190MitigationOffersNoFiniteFieldDHGroup(t *testing.T) {
	pki := newEAPTLSPKI(t)
	exchange := driveAttackExchange(t, pki, tls.VersionTLS12)

	for _, suite := range tlsFiniteFieldDHSuites {
		if slices.Contains(exchange.hello.cipherSuites, suite) {
			t.Errorf("the ClientHello offers finite-field Diffie-Hellman suite %#04x", suite)
		}
	}
	if !exchange.hello.hasExtension(tlsExtSupportedGroups) {
		t.Fatal("the ClientHello carries no supported_groups extension, so the offered groups cannot be read")
	}
	for _, group := range exchange.offer.SupportedCurves {
		if uint16(group) >= tlsGroupFFDHELow && uint16(group) <= tlsGroupFFDHEHigh {
			t.Errorf("the ClientHello offers ffdhe group %v", group)
		}
	}
}

// TestRFC9190MitigationCarriesNoServerName asserts an EAP-TLS exchange carries no
// host name for a virtual host to be confused about.
//
// RFC requirement: RFC9190-5.10-1 positive -- the peer's ClientHello carries no
// server_name extension, the ClientHelloInfo the authenticator parsed carries an
// empty ServerName, and the completed ConnectionState carries an empty ServerName
// on both roles, so neither a certificate nor a session is selected by host name.
// RFC 7457 Section 2.12: "A recent article [Delignat14] describes a security
// issue whereby SSLv3 fallback and improper handling of session caches on the
// server side can be abused by an attacker to establish a malicious connection to
// a virtual host other than the one originally intended and approved by the
// server."
//
// EAP carries no server hostname, which is why startTLSClient (peer.go) sets no
// ServerName and verifies the chain in VerifyPeerCertificate instead.
func TestRFC9190MitigationCarriesNoServerName(t *testing.T) {
	pki := newEAPTLSPKI(t)
	exchange := driveAttackExchange(t, pki, 0)

	if exchange.hello.hasExtension(tlsExtServerName) {
		t.Errorf("the ClientHello carries a server_name extension (%#04x)", tlsExtServerName)
	}
	if exchange.offer.ServerName != "" {
		t.Errorf("the authenticator was offered server name %q, want none", exchange.offer.ServerName)
	}
	if exchange.authState.ServerName != "" {
		t.Errorf("the authenticator's connection carries server name %q, want none", exchange.authState.ServerName)
	}
	if exchange.peerState.ServerName != "" {
		t.Errorf("the peer's connection carries server name %q, want none", exchange.peerState.ServerName)
	}
}

// TestRFC9190MitigationRefusesAMethodDowngrade offers a peer configured for
// EAP-TLS a weaker EAP method, and asserts it refuses and names EAP-TLS instead.
//
// This is the EAP shape of SSL stripping: something between the ends removes the
// request for the strong method and puts a weak one in its place. The peer
// answering a legacy Nak is what makes that substitution fail rather than succeed
// quietly.
//
// RFC requirement: RFC9190-5.10-1 negative -- a peer configured for EAP-TLS
// answers an EAP-Request of Type 4 (MD5-Challenge) with an EAP-Response of Type 3
// (legacy Nak) whose Type-Data is the single octet 13 (EAP-TLS), and derives no
// key from it. RFC 7457 Section 2.1: "Various attacks attempt to remove the use
// of Secure Socket Layer / Transport Layer Security (SSL/TLS) altogether by
// modifying unencrypted protocols that request the use of TLS, specifically
// modifying HTTP traffic and HTML pages as they pass on the wire."
//
// naks() answers true for every authentication Type other than the configured
// one until a method is committed (peer.go).
func TestRFC9190MitigationRefusesAMethodDowngrade(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := newAttackPeer(pki)
	t.Cleanup(peer.Close)

	identity := peer.Process(&Packet{Code: CodeRequest, Identifier: 1, Type: TypeIdentity})
	if identity.Err != nil {
		t.Fatalf("the peer refused the Identity Request: %v", identity.Err)
	}

	// A well-formed MD5-Challenge Request: one Value-Size octet and sixteen
	// challenge octets (RFC 1994 Section 4.1). The peer has to refuse it for what
	// it IS, rather than for being malformed.
	challenge := []byte{16, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	nak := peer.Process(&Packet{Code: CodeRequest, Identifier: 2, Type: TypeMD5Challenge, TypeData: challenge})

	if nak.Err != nil {
		t.Fatalf("the peer errored on an MD5-Challenge Request rather than refusing it: %v", nak.Err)
	}
	if nak.Response == nil {
		t.Fatal("the peer answered an MD5-Challenge Request with nothing")
	}
	if nak.Response.Type != TypeNAK {
		t.Fatalf("the peer answered an MD5-Challenge Request with Type %d, want %d (legacy Nak)", nak.Response.Type, TypeNAK)
	}
	if len(nak.Response.TypeData) != 1 || nak.Response.TypeData[0] != TypeTLS {
		t.Errorf("the Nak names %#02x, want the single octet %#02x (EAP-TLS)", nak.Response.TypeData, TypeTLS)
	}
	if peer.msk != [64]byte{} {
		t.Error("the peer derived a key from an exchange it refused")
	}
}

// TestRFC9190MitigationDropsBytesPipelinedBehindTheStart appends octets to the
// EAP-TLS Start request, and asserts the exchange completes exactly as it does
// without them.
//
// This is the EAP shape of the STARTTLS command injection attack: octets that
// arrive with the request to start TLS and are executed once TLS is running.
// handleTLSRequest (peer.go) starts the TLS client on the S flag and calls
// readAndSendTLS, reassembling and feeding nothing from that packet, so the
// appended octets reach crypto/tls never. Fed, they would be read as the first
// TLS record of the authenticator's flight and the handshake would fail.
//
// RFC requirement: RFC9190-5.10-1 negative -- an EAP-TLS Start request carrying
// ten trailing octets behind its flags octet completes the same mutually
// authenticated exchange as an unmodified Start does, with a non-zero MSK the
// authenticator agrees with, so those octets were never fed to the TLS engine.
// RFC 7457 Section 2.2: "Multiple implementations of STARTTLS had a flaw where an
// application-layer input buffer retained commands that were pipelined with the
// STARTTLS command, such that commands received prior to TLS negotiation are
// executed after TLS negotiation."
//
// The EAP-TLS Start is the only request whose payload the peer discards, and it
// discards all of it (handleTLSRequest, peer.go).
func TestRFC9190MitigationDropsBytesPipelinedBehindTheStart(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := newAttackPeer(pki)
	sess, err := NewSession(TypeTLS, pki.serverConfig())
	if err != nil {
		t.Fatalf("create the authenticator session: %v", err)
	}
	t.Cleanup(func() {
		sess.Close()
		peer.Close()
	})

	// A syntactically valid TLS record header over five octets of rubbish,
	// appended to the Start. crypto/tls accepts the header and fails on the body,
	// so feeding it is an outcome this test can tell apart from not feeding it.
	pipelined := []byte{tlsRecordHandshake, 0x03, 0x03, 0x00, 0x05, 'p', 'i', 'p', 'e', 'd'}

	req := sess.Begin()
	req.TypeData = append(slices.Clone(req.TypeData), pipelined...)

	var result PeerResult
	for range 40 {
		result = peer.Process(req)
		if result.Err != nil {
			t.Fatalf("the peer failed an exchange whose Start carried pipelined octets: %v", result.Err)
		}
		if result.Done {
			break
		}
		if result.Response == nil {
			t.Fatal("the peer answered with nothing before the exchange concluded")
		}
		next := sess.Process(result.Response)
		if next == nil {
			t.Fatal("the authenticator answered with nothing before the exchange concluded")
		}
		req = next
	}

	if !result.Done {
		t.Fatal("the exchange did not conclude within 40 rounds")
	}
	if result.MSK == [64]byte{} {
		t.Error("the peer concluded with an all-zero MSK")
	}
	if result.MSK != sess.MSK() {
		t.Error("the peer and the authenticator derived different MSKs")
	}
}
