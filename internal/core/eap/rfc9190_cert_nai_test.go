// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- the anonymous NAI the EAP-TLS peer sends
// RFC: rfc/short/rfc9190.md -- Section 2.1.7, Identity
// Detail: rfc/full/rfc7542.txt -- Section 2.2, the NAI grammar
// Related: internal/core/eap/rfc9190_nai_test.go -- the same derivation from the configured identity alone
//
// RFC 9190 Section 2.1.7 names the peer's own certificate as a source of the
// realm the Identity Response routes on:
//
//	"Many client certificates contain an identity such as an email address,
//	which is already in NAI format.  When the client certificate contains an NAI
//	as subject name or alternative subject name, an anonymous NAI SHOULD be
//	derived from the NAI in the certificate; see Section 2.1.8."
//
// VALIDATES: ze's EAP-TLS peer reads the realm out of its own certificate when
// the configured identity carries none, over each certificate field that can
// hold an NAI, and the username in that certificate reaches no packet.
// PREVENTS: a deployment whose local-id is not an NAI sending the non-routable
// bare "anonymous" while its certificate names a realm, which RFC 9190
// Section 2.1.3 says makes resumption likely impossible.

package eap

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"
)

// naiCertSerial is the serial number every certificate this file issues carries.
// The harness CRL revokes nothing (newEAPTLSPKI, eap_tls_handshake_test.go), so
// the number only has to miss the two the PKI already uses.
const naiCertSerial int64 = 30

// naiCertNames is the set of certificate fields RFC 9190 Section 2.1.7 can find
// an NAI in, plus the one it cannot.
//
// dnsNames is here to be REFUSED: a dNSName carries no "@", so the RFC 7542
// Section 2.2 grammar reads it as a utf8-username rather than as a realm.
type naiCertNames struct {
	commonName string
	emails     []string
	upns       []string
	dnsNames   []string
}

// naiSANExtension marshals one subjectAltName extension carrying every name in
// names, in the order rfc822Name, dNSName, otherName.
//
// It is built by hand because crypto/x509 has no field for an otherName:
// CreateCertificate generates the extension from EmailAddresses and DNSNames and
// skips that generation when ExtraExtensions already carries the same OID, so a
// certificate carrying both forms has to carry one hand-built extension.
func naiSANExtension(t *testing.T, names naiCertNames) pkix.Extension {
	t.Helper()

	var generalNames []asn1.RawValue
	for _, email := range names.emails {
		generalNames = append(generalNames, asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 1, Bytes: []byte(email)})
	}
	for _, dnsName := range names.dnsNames {
		generalNames = append(generalNames, asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 2, Bytes: []byte(dnsName)})
	}
	for _, upn := range names.upns {
		// Built from the three TLVs RFC 5280 Section 4.2.1.6 names, rather than
		// from the otherName struct: encoding/asn1 writes a RawValue that carries
		// FullBytes as it stands, so the "explicit" struct tag would not wrap the
		// value and the extension would carry an OtherName the parser refuses.
		value, err := asn1.MarshalWithParams(upn, "utf8")
		if err != nil {
			t.Fatalf("marshal userPrincipalName %q: %v", upn, err)
		}
		explicit, err := asn1.Marshal(asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 0, IsCompound: true, Bytes: value})
		if err != nil {
			t.Fatalf("marshal otherName value %q: %v", upn, err)
		}
		typeID, err := asn1.Marshal(oidUserPrincipalName)
		if err != nil {
			t.Fatalf("marshal otherName type-id: %v", err)
		}
		generalNames = append(generalNames, asn1.RawValue{
			Class:      asn1.ClassContextSpecific,
			Tag:        0,
			IsCompound: true,
			Bytes:      append(typeID, explicit...),
		})
	}

	der, err := asn1.Marshal(generalNames)
	if err != nil {
		t.Fatalf("marshal subjectAltName: %v", err)
	}
	return pkix.Extension{Id: oidSubjectAltName, Value: der}
}

// naiClientCert issues a client certificate from the harness trusted CA
// carrying the names given, and returns it with its key as PEM.
//
// The certificate is a real one signed by the CA the authenticator trusts, so a
// test can drive a complete exchange with it rather than only reading the
// derivation.
func naiClientCert(t *testing.T, pki *eapTLSPKI, names naiCertNames) (certPEM, keyPEM []byte) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("client key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(naiCertSerial),
		Subject:      pkix.Name{CommonName: names.commonName},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	if len(names.emails)+len(names.upns)+len(names.dnsNames) > 0 {
		tmpl.ExtraExtensions = []pkix.Extension{naiSANExtension(t, names)}
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, pki.trustedCA, &key.PublicKey, pki.trustedCAKey)
	if err != nil {
		t.Fatalf("client cert: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

// peerConfigWithCert is the harness peer configuration with the client
// certificate replaced, so a test can drive an exchange with a certificate whose
// names it chose.
func (p *eapTLSPKI) peerConfigWithCert(certPEM, keyPEM []byte) *PeerTLSConfig {
	cfg := p.peerConfigWithCRL(p.trustedCRLPEM)
	cfg.CertPEM = certPEM
	cfg.KeyPEM = keyPEM
	return cfg
}

// certDerivedNAI answers the NAI an EAP-TLS peer built from this identity and
// this certificate would put in its Identity Response.
//
// It reads the value off a real PeerSession rather than calling the derivation,
// because NewPeerSessionTLS is where RFC 9190 Section 2.1.7 is applied and a
// constructor that stopped consulting the certificate has to fail these tests.
func certDerivedNAI(t *testing.T, identity string, certPEM []byte) string {
	t.Helper()

	peer := NewPeerSessionTLS(identity, &PeerTLSConfig{CertPEM: certPEM})
	t.Cleanup(peer.Close)
	return peer.identity
}

// TestEAPTLS13PeerDerivesItsRealmFromTheCertificateNAI drives a complete EAP-TLS
// 1.3 exchange for an operator whose local-id carries no realm and whose client
// certificate names one, and reads the Identity Response off the wire.
//
// RFC requirement: RFC9190-2.1.7-1 positive -- the certificate carries the NAI
// "alice@example.com" as an rfc822Name subject alternative name, the configured
// identity "ze-test-client" carries no realm, and the Identity Response on the
// wire is "@example.com". No packet the peer sent carries "alice", so the
// anonymous NAI Section 2.1.8 requires is what was derived from the certificate
// rather than the certificate's own NAI. The exchange completes with one MSK on
// both ends, so the derived identity costs the peer no authentication.
func TestEAPTLS13PeerDerivesItsRealmFromTheCertificateNAI(t *testing.T) {
	pki := newEAPTLSPKI(t)
	certPEM, keyPEM := naiClientCert(t, pki, naiCertNames{
		commonName: "eap-tls-client",
		emails:     []string{"alice@example.com"},
	})

	peer := NewPeerSessionTLS("ze-test-client", pki.peerConfigWithCert(certPEM, keyPEM))
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS13, naiExchangeRounds)

	if len(fl.peerSent) == 0 {
		t.Fatal("the peer answered nothing, so the exchange carried no Identity Response")
	}
	first := fl.peerSent[0]
	if first.Type != TypeIdentity {
		t.Fatalf("the peer's first packet is Type %d, want the Identity Response (Type %d)", first.Type, TypeIdentity)
	}
	if got := string(first.TypeData); got != "@example.com" {
		t.Fatalf("Identity Response carries %q, want %q derived from the certificate NAI", got, "@example.com")
	}
	for i, p := range fl.peerSent {
		if strings.Contains(string(p.TypeData), "alice") {
			t.Fatalf("peer packet %d carries the certificate's username in cleartext: RFC 9190 Section 2.1.8 forbids it", i)
		}
	}

	if !fl.peerDone {
		t.Fatalf("the exchange did not complete: %v", fl.peerErr)
	}
	if fl.peerMSK != fl.serverMSK {
		t.Fatal("the two ends derived different MSKs from an exchange the derived NAI opened")
	}
}

// TestEAPTLSPeerDerivesTheRealmFromEveryNAIBearingCertificateField reads the
// derivation over each certificate field that can hold an NAI, and over the
// order between them.
//
// RFC requirement: RFC9190-2.1.7-1 positive -- "When the client certificate
// contains an NAI as subject name or alternative subject name, an anonymous NAI
// SHOULD be derived from the NAI in the certificate". The alternative subject
// name is read in both forms that carry an NAI, the rfc822Name the section names
// itself and the userPrincipalName otherName an enterprise CA writes, and the
// subject name is read from the common name. The subject alternative name is
// preferred over the subject, and a name the grammar refuses is passed over for
// the next one rather than ending the search.
func TestEAPTLSPeerDerivesTheRealmFromEveryNAIBearingCertificateField(t *testing.T) {
	pki := newEAPTLSPKI(t)

	cases := []struct {
		what  string
		names naiCertNames
		nai   string
	}{
		{
			what:  "an rfc822Name subject alternative name",
			names: naiCertNames{commonName: "eap-tls-client", emails: []string{"alice@example.com"}},
			nai:   "@example.com",
		},
		{
			what:  "a userPrincipalName otherName",
			names: naiCertNames{commonName: "eap-tls-client", upns: []string{"alice@corp.example.com"}},
			nai:   "@corp.example.com",
		},
		{
			what:  "the subject common name",
			names: naiCertNames{commonName: "alice@cn.example.com"},
			nai:   "@cn.example.com",
		},
		{
			what:  "an rfc822Name before the common name",
			names: naiCertNames{commonName: "bob@cn.example.com", emails: []string{"alice@san.example.com"}},
			nai:   "@san.example.com",
		},
		{
			what:  "a userPrincipalName before the common name",
			names: naiCertNames{commonName: "bob@cn.example.com", upns: []string{"alice@upn.example.com"}},
			nai:   "@upn.example.com",
		},
		{
			what:  "the first rfc822Name the grammar accepts",
			names: naiCertNames{commonName: "eap-tls-client", emails: []string{"alice@localhost", "alice@example.com"}},
			nai:   "@example.com",
		},
		{
			what:  "an rfc822Name beside a dNSName",
			names: naiCertNames{commonName: "eap-tls-client", emails: []string{"alice@example.com"}, dnsNames: []string{"host.other.example"}},
			nai:   "@example.com",
		},
	}

	for _, tc := range cases {
		certPEM, _ := naiClientCert(t, pki, tc.names)
		got := certDerivedNAI(t, "ze-test-client", certPEM)
		if got != tc.nai {
			t.Errorf("with %s the peer sends %q, want %q", tc.what, got, tc.nai)
			continue
		}
		if !validNAI(got) {
			t.Errorf("with %s the peer sends %q, which the RFC 7542 Section 2.2 grammar refuses", tc.what, got)
		}
	}
}

// TestEAPTLSPeerKeepsTheAnonymousFallbackWhenNoCertificateNAIIsUsable reads the
// derivation over certificates that name no realm the exchange can route on, and
// over certificate material that does not parse at all.
//
// RFC requirement: RFC9190-2.1.7-1 negative -- the SHOULD is conditional on the
// certificate containing an NAI, so a certificate that contains none leaves the
// peer with the fixed username Section 2.1.8 allows. A dNSName is the sharp case:
// it is a subject alternative name and it is not an NAI, because it carries no
// "@" and RFC 7542 Section 2.2 reads it as a utf8-username rather than a realm.
// A peer that derived a realm from anything in a certificate would pass the
// positive rows above and fail every row here.
//
// Certificate material is attacker-influenced in the general case, so the rows
// that do not parse are here too: each answers the fixed username, and none of
// them ends the constructor.
func TestEAPTLSPeerKeepsTheAnonymousFallbackWhenNoCertificateNAIIsUsable(t *testing.T) {
	pki := newEAPTLSPKI(t)

	dnsOnly, _ := naiClientCert(t, pki, naiCertNames{commonName: "eap-tls-client", dnsNames: []string{"host.example.com"}})
	noNames, _ := naiClientCert(t, pki, naiCertNames{commonName: "eap-tls-client"})
	badRealm, _ := naiClientCert(t, pki, naiCertNames{commonName: "eap-tls-client", emails: []string{"alice@localhost"}})
	badUPN, _ := naiClientCert(t, pki, naiCertNames{commonName: "eap-tls-client", upns: []string{"alice@-example.com"}})
	_, keyPEM := naiClientCert(t, pki, naiCertNames{commonName: "eap-tls-client"})

	cases := []struct {
		what    string
		certPEM []byte
	}{
		{"a dNSName, which is no NAI", dnsOnly},
		{"a certificate naming no realm at all", noNames},
		{"an rfc822Name whose realm the grammar refuses", badRealm},
		{"a userPrincipalName whose realm the grammar refuses", badUPN},
		{"no certificate material", nil},
		{"empty certificate material", []byte{}},
		{"bytes that are not PEM", []byte("this is not a certificate")},
		{"a PEM block that is not a certificate", keyPEM},
		{"a CERTIFICATE block holding no certificate", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte{0x30, 0x03, 0x02, 0x01, 0x00}})},
		{"a truncated certificate", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pemBody(t, dnsOnly)[:20]})},
	}

	for _, tc := range cases {
		if got := certDerivedNAI(t, "ze-test-client", tc.certPEM); got != anonymousUser {
			t.Errorf("with %s the peer sends %q, want the fixed username %q", tc.what, got, anonymousUser)
		}
	}

	// A session built with no configuration at all takes the same answer. It
	// reaches no handshake either, and it still owes a valid NAI.
	peer := NewPeerSessionTLS("ze-test-client", nil)
	t.Cleanup(peer.Close)
	if peer.identity != anonymousUser {
		t.Errorf("with no TLS configuration the peer sends %q, want the fixed username %q", peer.identity, anonymousUser)
	}
}

// pemBody returns the DER inside the first PEM block of the material given.
func pemBody(t *testing.T, material []byte) []byte {
	t.Helper()

	block, _ := pem.Decode(material)
	if block == nil {
		t.Fatal("the material carries no PEM block")
	}
	return block.Bytes
}

// TestEAPTLSPeerPrefersTheConfiguredRealmOverTheCertificate reads the
// derivation for an operator who configured a realm AND holds a certificate
// naming a different one.
//
// RFC requirement: RFC9190-2.1.7-1 negative -- the certificate is the source
// only where the configured identity gives no realm. RFC 9190 Section 2.1.3 asks
// for "NAIs with the same realm during resumption and the original full
// handshake" and warns that "If this recommendation is not followed, resumption
// is likely impossible", so the realm the operator configured is the one the
// deployment routes on and the certificate does not displace it. A peer that
// always read the certificate would pass the positive rows and fail this one.
func TestEAPTLSPeerPrefersTheConfiguredRealmOverTheCertificate(t *testing.T) {
	pki := newEAPTLSPKI(t)
	certPEM, _ := naiClientCert(t, pki, naiCertNames{
		commonName: "carol@cn.example.com",
		emails:     []string{"alice@san.example.com"},
		upns:       []string{"dave@upn.example.com"},
	})

	if got := certDerivedNAI(t, "bob@operator.example", certPEM); got != "@operator.example" {
		t.Fatalf("the peer sends %q, want the configured realm %q", got, "@operator.example")
	}
}

// TestEAPTLSPeerDropsTheUsernameTheCertificateCarries reads what survives the
// derivation, over every certificate field an NAI can be read from.
//
// RFC requirement: RFC9190-2.1.8-2 positive -- "A client supporting TLS 1.3 MUST
// NOT send its username (or any other permanent identifiers) in cleartext in the
// Identity Response". A certificate NAI carries a username exactly as a
// configured identity does, and Section 2.1.7 asks for an anonymous NAI to be
// derived from it rather than for it to be sent. Every row here names a distinct
// username, and none of them survives: the derived NAI is "@" and the realm, and
// nothing else.
func TestEAPTLSPeerDropsTheUsernameTheCertificateCarries(t *testing.T) {
	pki := newEAPTLSPKI(t)

	cases := []struct {
		what     string
		names    naiCertNames
		username string
	}{
		{"an rfc822Name", naiCertNames{commonName: "eap-tls-client", emails: []string{"rfc822user@example.com"}}, "rfc822user"},
		{"a userPrincipalName", naiCertNames{commonName: "eap-tls-client", upns: []string{"upnuser@example.com"}}, "upnuser"},
		{"a common name", naiCertNames{commonName: "cnuser@example.com"}, "cnuser"},
	}

	for _, tc := range cases {
		certPEM, _ := naiClientCert(t, pki, tc.names)
		got := certDerivedNAI(t, "ze-test-client", certPEM)

		if strings.Contains(got, tc.username) {
			t.Errorf("with %s the peer sends %q, which still carries the certificate's username", tc.what, got)
			continue
		}
		if got != "@example.com" {
			t.Errorf("with %s the peer sends %q, want %q", tc.what, got, "@example.com")
		}
	}
}
