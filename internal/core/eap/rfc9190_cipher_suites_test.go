// Design: docs/architecture/ike/ipsec-14-responder.md -- the authenticator's EAP-TLS method
// Detail: docs/architecture/ike/ipsec-11-interop-eap.md -- the peer's EAP-TLS method
// RFC: rfc/short/rfc9190.md -- parameter negotiation (Section 2.4)
//
// VALIDATES: every cipher suite ze's peer offers encrypts, and ze's authenticator
// refuses a client that offers only suites without confidentiality.
// PREVENTS: an EAP-TLS session whose records are authenticated but readable,
// which RFC 9190 Section 2.4 rules out for TLS 1.3.

package eap

import (
	"crypto/tls"
	"encoding/binary"
	"slices"
	"strings"
	"testing"
	"time"
)

// Cipher suites that authenticate records without encrypting them. crypto/tls
// implements none of them, so they are written out here.
const (
	// tlsSuiteRSAWithNullSHA256 is TLS_RSA_WITH_NULL_SHA256 (RFC 5246 Appendix A.5).
	tlsSuiteRSAWithNullSHA256 uint16 = 0x003b
	// tlsSuiteSHA256SHA256 is TLS_SHA256_SHA256, the TLS 1.3 integrity-only suite
	// of RFC 9150 Section 9.
	tlsSuiteSHA256SHA256 uint16 = 0xc0b4
	// tlsSuiteEmptyRenegotiationSCSV is TLS_EMPTY_RENEGOTIATION_INFO_SCSV (RFC 5746
	// Section 3.3), a signal and not a suite.
	tlsSuiteEmptyRenegotiationSCSV uint16 = 0x00ff
)

// encryptingSuiteName answers the name of an offered suite crypto/tls implements,
// and false for any suite it does not.
func encryptingSuiteName(id uint16) (string, bool) {
	for _, list := range [][]*tls.CipherSuite{tls.CipherSuites(), tls.InsecureCipherSuites()} {
		for _, suite := range list {
			if suite.ID == id {
				return suite.Name, true
			}
		}
	}
	return "", false
}

// TestEAPTLS13PeerOffersOnlySuitesWithConfidentiality decodes the ClientHello
// ze's peer puts on the wire.
//
// RFC requirement: RFC9190-2.4-2 positive -- "In EAP-TLS with TLS 1.3, only
// cipher suites with confidentiality SHALL be supported". Every suite the peer
// offers is one crypto/tls implements, and none of them is a NULL-encryption or
// integrity-only suite.
func TestEAPTLS13PeerOffersOnlySuitesWithConfidentiality(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(NewResumption(time.Now, true)))
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS13, eapTLS13Rounds)
	requireCompleted(t, fl, "a full EAP-TLS 1.3 exchange")

	hello := parseClientHello(t, fl)
	if len(hello.cipherSuites) == 0 {
		t.Fatal("the ClientHello offers no cipher suite")
	}
	for _, id := range hello.cipherSuites {
		if id == tlsSuiteEmptyRenegotiationSCSV {
			continue
		}
		name, ok := encryptingSuiteName(id)
		if !ok {
			t.Errorf("the peer offers suite %#04x, which crypto/tls does not implement", id)
			continue
		}
		if strings.Contains(name, "_NULL_") || id == tlsSuiteSHA256SHA256 {
			t.Errorf("the peer offers %s, which provides no confidentiality", name)
		}
	}
}

// withCipherSuites answers a copy of a ClientHello record whose cipher_suites
// field holds exactly the suites given, with the record and handshake lengths
// fixed to match. The walk follows RFC 8446 Section 4.1.2.
func withCipherSuites(t *testing.T, record []byte, suites ...uint16) []byte {
	t.Helper()
	const handshakeAt = tlsRecordHeaderLen + 4
	body := record[handshakeAt:]
	at := 2 + tlsClientHelloRandomOctets
	at += 1 + int(body[at])
	old := int(binary.BigEndian.Uint16(body[at:]))

	field := make([]byte, 2+2*len(suites))
	binary.BigEndian.PutUint16(field, uint16(2*len(suites)))
	for i, id := range suites {
		binary.BigEndian.PutUint16(field[2+2*i:], id)
	}
	rewritten := append(append(append([]byte{}, body[:at]...), field...), body[at+2+old:]...)

	msg := make([]byte, handshakeAt, handshakeAt+len(rewritten))
	copy(msg, record[:tlsRecordHeaderLen])
	binary.BigEndian.PutUint16(msg[3:5], uint16(4+len(rewritten)))
	msg[5] = tlsHSClientHello
	msg[6], msg[7], msg[8] = byte(len(rewritten)>>16), byte(len(rewritten)>>8), byte(len(rewritten))
	return append(msg, rewritten...)
}

// TestEAPTLSAuthenticatorRefusesSuitesWithoutConfidentiality replays ze's own
// ClientHello at a fresh authenticator, once as sent and once with its
// cipher_suites replaced by suites that do not encrypt.
//
// RFC requirement: RFC9190-2.4-2 negative -- a client that offers only
// TLS_SHA256_SHA256 (RFC 9150) or TLS_RSA_WITH_NULL_SHA256 is answered with an
// alert and no ServerHello, while the same ClientHello with its own suites is
// answered with the handshake. The authenticator supports no suite without
// confidentiality.
func TestEAPTLSAuthenticatorRefusesSuitesWithoutConfidentiality(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(NewResumption(time.Now, true)))
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS13, eapTLS13Rounds)
	requireCompleted(t, fl, "a full EAP-TLS 1.3 exchange")
	record := parseClientHello(t, fl).record

	if got := replayClientHello(t, pki, record); got != tlsRecordHandshake {
		t.Fatalf("the unmodified ClientHello drew content type %#02x, want the handshake: the replay itself is broken", got)
	}
	for name, suite := range map[string]uint16{
		"TLS_SHA256_SHA256":        tlsSuiteSHA256SHA256,
		"TLS_RSA_WITH_NULL_SHA256": tlsSuiteRSAWithNullSHA256,
	} {
		if got := replayClientHello(t, pki, withCipherSuites(t, record, suite)); got != tlsRecordAlert {
			t.Errorf("a ClientHello offering only %s drew content type %#02x, want an alert", name, got)
		}
	}
}

// RFC 8446 Section 9 code points the compliance test reads.
const (
	tlsSuiteAES128GCMSHA256    uint16 = 0x1301 // TLS_AES_128_GCM_SHA256, Section 9.1 MUST
	tlsSigECDSASecp256r1SHA256 uint16 = 0x0403 // ecdsa_secp256r1_sha256, Section 9.1 MUST
	tlsSigRSAPSSRSAESHA256     uint16 = 0x0804 // rsa_pss_rsae_sha256, Section 9.1 MUST
	tlsSigRSAPKCS1SHA256       uint16 = 0x0401 // rsa_pkcs1_sha256, Section 9.1 MUST for certificates
	tlsGroupSecp256r1          uint16 = 0x0017 // secp256r1, Section 9.1 MUST
	tlsExtSignatureAlgorithms  uint16 = 13
	tlsExtSupportedGroupsMTI   uint16 = 10
	tlsExtKeyShare             uint16 = 51
)

// uint16List decodes an extension body that is one uint16-length-prefixed list
// of uint16 code points.
func uint16List(t *testing.T, body []byte) []uint16 {
	t.Helper()
	if len(body) < 2 || int(binary.BigEndian.Uint16(body)) != len(body)-2 {
		t.Fatalf("an extension list of %d octets does not match its length prefix", len(body))
	}
	var out []uint16
	for i := 2; i+1 < len(body); i += 2 {
		out = append(out, binary.BigEndian.Uint16(body[i:]))
	}
	return out
}

// withoutExtension answers a copy of a ClientHello record with one extension
// removed and every length fixed to match.
func withoutExtension(t *testing.T, record []byte, drop uint16) []byte {
	t.Helper()
	const handshakeAt = tlsRecordHeaderLen + 4
	body := record[handshakeAt:]
	at := 2 + tlsClientHelloRandomOctets
	at += 1 + int(body[at])
	at += 2 + int(binary.BigEndian.Uint16(body[at:]))
	at += 1 + int(body[at])
	extEnd := at + 2 + int(binary.BigEndian.Uint16(body[at:]))

	var kept []byte
	removed := false
	for off := at + 2; off+4 <= extEnd; {
		kind := binary.BigEndian.Uint16(body[off:])
		end := off + 4 + int(binary.BigEndian.Uint16(body[off+2:]))
		if kind == drop {
			removed = true
		} else {
			kept = append(kept, body[off:end]...)
		}
		off = end
	}
	if !removed {
		t.Fatalf("the ClientHello carries no extension %d to remove", drop)
	}
	rewritten := append([]byte{}, body[:at]...)
	rewritten = binary.BigEndian.AppendUint16(rewritten, uint16(len(kept)))
	rewritten = append(append(rewritten, kept...), body[extEnd:]...)

	msg := make([]byte, handshakeAt, handshakeAt+len(rewritten))
	copy(msg, record[:tlsRecordHeaderLen])
	binary.BigEndian.PutUint16(msg[3:5], uint16(4+len(rewritten)))
	msg[5] = tlsHSClientHello
	msg[6], msg[7], msg[8] = byte(len(rewritten)>>16), byte(len(rewritten)>>8), byte(len(rewritten))
	return append(msg, rewritten...)
}

// TestEAPTLS13BothRolesMeetTheRFC8446MandatoryToImplementSet decodes the
// ClientHello ze's peer sends, then replays it at ze's authenticator offering
// the mandatory cipher suite alone.
//
// RFC requirement: RFC9190-2.4-1 positive -- "the EAP-TLS peers and EAP- TLS
// servers MUST comply with the compliance requirements (mandatory- to-implement
// cipher suites, signature algorithms, key exchange algorithms, extensions,
// etc.) defined in Section 9 of [RFC8446]". The peer offers
// TLS_AES_128_GCM_SHA256, the signature algorithms ecdsa_secp256r1_sha256,
// rsa_pss_rsae_sha256 and rsa_pkcs1_sha256, the secp256r1 group, and the
// supported_versions, signature_algorithms, supported_groups and key_share
// extensions Section 9.2 requires of a TLS 1.3 ClientHello; the authenticator
// completes its first flight for a client offering TLS_AES_128_GCM_SHA256 and
// no other suite.
func TestEAPTLS13BothRolesMeetTheRFC8446MandatoryToImplementSet(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(NewResumption(time.Now, true)))
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS13, eapTLS13Rounds)
	requireCompleted(t, fl, "a full EAP-TLS 1.3 exchange")

	hello := parseClientHello(t, fl)
	if !slices.Contains(hello.cipherSuites, tlsSuiteAES128GCMSHA256) {
		t.Error("the peer does not offer TLS_AES_128_GCM_SHA256")
	}
	extensions := clientHelloExtensions(t, hello.record)
	for _, ext := range []uint16{tlsExtSupportedVersions, tlsExtSignatureAlgorithms, tlsExtSupportedGroupsMTI, tlsExtKeyShare} {
		if _, ok := extensions[ext]; !ok {
			t.Errorf("the TLS 1.3 ClientHello lacks extension %d, which RFC 8446 Section 9.2 requires", ext)
		}
	}
	signatures := uint16List(t, extensions[tlsExtSignatureAlgorithms])
	for _, alg := range []uint16{tlsSigECDSASecp256r1SHA256, tlsSigRSAPSSRSAESHA256, tlsSigRSAPKCS1SHA256} {
		if !slices.Contains(signatures, alg) {
			t.Errorf("the peer does not offer signature algorithm %#04x", alg)
		}
	}
	if !slices.Contains(uint16List(t, extensions[tlsExtSupportedGroupsMTI]), tlsGroupSecp256r1) {
		t.Error("the peer does not offer the secp256r1 group")
	}

	if got := replayClientHello(t, pki, withCipherSuites(t, hello.record, tlsSuiteAES128GCMSHA256)); got != tlsRecordHandshake {
		t.Fatalf("a ClientHello offering only TLS_AES_128_GCM_SHA256 drew content type %#02x, want the handshake", got)
	}
}

// TestEAPTLS13AuthenticatorAbortsAClientHelloWithoutSignatureAlgorithms replays
// ze's own ClientHello at ze's authenticator with its signature_algorithms
// extension removed.
//
// RFC requirement: RFC9190-2.4-1 negative -- RFC 8446 Section 9.2: a TLS 1.3
// ClientHello "If not containing a "pre_shared_key" extension, it MUST contain
// both a "signature_algorithms" extension and a "supported_groups" extension",
// and "Servers receiving a ClientHello which does not conform to these
// requirements MUST abort the handshake with a "missing_extension" alert." The
// authenticator answers that ClientHello with an alert and no ServerHello, so it
// does not negotiate TLS 1.3 outside the Section 9 requirements.
func TestEAPTLS13AuthenticatorAbortsAClientHelloWithoutSignatureAlgorithms(t *testing.T) {
	pki := newEAPTLSPKI(t)
	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(NewResumption(time.Now, true)))
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS13, eapTLS13Rounds)
	requireCompleted(t, fl, "a full EAP-TLS 1.3 exchange")
	record := parseClientHello(t, fl).record

	if got := replayClientHello(t, pki, record); got != tlsRecordHandshake {
		t.Fatalf("the unmodified ClientHello drew content type %#02x, want the handshake: the replay itself is broken", got)
	}
	if got := replayClientHello(t, pki, withoutExtension(t, record, tlsExtSignatureAlgorithms)); got != tlsRecordAlert {
		t.Fatalf("a TLS 1.3 ClientHello without signature_algorithms drew content type %#02x, want an alert", got)
	}
}
