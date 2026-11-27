// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP-TLS certificate identity extraction and authorization
// Detail: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP-TLS peer server-identity verification
// RFC: rfc/short/rfc5216.md -- Sections 5.2 and 5.3 (Peer-Id/Server-Id from SAN, authorization, RFC 2818 Section 3.1)
//
// VALIDATES: EAP-TLS extracts Peer-Id and Server-Id from subjectAltName in wire
// order (never falling back to Subject CN when a SAN extension is present),
// exposes them on Session/PeerSession and eapTLSPeerName, authorizes client
// identities against AllowedIdentities and server identities against
// ServerIdentity per RFC 2818 Section 3.1 on both full and resumed TLS 1.3
// handshakes, and fails closed when identity policy is unset.
// PREVENTS: a trusted CA signing an unintended peer or server certificate from
// authenticating an EAP-TLS session, a Subject CN bypassing a SAN constraint,
// wildcard/IP class confusion, or a resumed session bypassing narrowed policy.

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
	"errors"
	"math/big"
	"net"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	testClientSAN = "client.example.com"
	testServerSAN = "server.example.com"
	testClientNAI = "client@example.com"
)

// newLeafWithSAN issues an end-entity leaf certificate signed by caCert/caKey
// with the requested Subject CommonName and DNS subjectAltName entries.
func newLeafWithSAN(
	t *testing.T,
	caCert *x509.Certificate,
	caKey *ecdsa.PrivateKey,
	cn string,
	serial int64,
	eku []x509.ExtKeyUsage,
	dnsNames []string,
) (certPEM, keyPEM []byte) {
	t.Helper()
	return newCustomLeaf(t, caCert, caKey, serial, pkix.Name{CommonName: cn}, eku, func(tmpl *x509.Certificate) {
		tmpl.DNSNames = dnsNames
	})
}

// newCustomLeaf issues an end-entity leaf certificate signed by caCert/caKey
// with full control over Subject and extensions.
func newCustomLeaf(
	t *testing.T,
	caCert *x509.Certificate,
	caKey *ecdsa.PrivateKey,
	serial int64,
	subject pkix.Name,
	eku []x509.ExtKeyUsage,
	mutate func(*x509.Certificate),
) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("leaf key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      subject,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  eku,
	}
	if mutate != nil {
		mutate(tmpl)
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("leaf cert: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

// marshalSANExtension builds a DER-encoded subjectAltName extension carrying
// the supplied GeneralName raw values in exact caller-specified wire order.
func marshalSANExtension(t *testing.T, names ...asn1.RawValue) pkix.Extension {
	t.Helper()
	var body []byte
	for _, gn := range names {
		encoded, err := asn1.Marshal(gn)
		if err != nil {
			t.Fatalf("marshal GeneralName: %v", err)
		}
		body = append(body, encoded...)
	}
	seq, err := asn1.Marshal(asn1.RawValue{
		Class:      asn1.ClassUniversal,
		Tag:        asn1.TagSequence,
		IsCompound: true,
		Bytes:      body,
	})
	if err != nil {
		t.Fatalf("marshal GeneralNames sequence: %v", err)
	}
	return pkix.Extension{Id: oidSubjectAltName, Value: seq}
}

func sanGeneralString(tag int, val string) asn1.RawValue {
	return asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: tag, Bytes: []byte(val)}
}

func sanGeneralIP(ip net.IP) asn1.RawValue {
	raw := ip.To4()
	if raw == nil {
		raw = ip.To16()
	}
	return asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 7, Bytes: raw}
}

func sanGeneralUPN(t *testing.T, upn string) asn1.RawValue {
	t.Helper()
	utf8Val, err := asn1.MarshalWithParams(upn, "utf8")
	if err != nil {
		t.Fatalf("marshal UPN UTF8String: %v", err)
	}
	explicitVal, err := asn1.Marshal(asn1.RawValue{
		Class:      asn1.ClassContextSpecific,
		Tag:        0,
		IsCompound: true,
		Bytes:      utf8Val,
	})
	if err != nil {
		t.Fatalf("marshal UPN explicit [0]: %v", err)
	}
	oidBytes, err := asn1.Marshal(oidUserPrincipalName)
	if err != nil {
		t.Fatalf("marshal UPN OID: %v", err)
	}
	return asn1.RawValue{
		Class:      asn1.ClassContextSpecific,
		Tag:        0,
		IsCompound: true,
		Bytes:      append(oidBytes, explicitVal...),
	}
}

func sanGeneralRegisteredID(t *testing.T, oid asn1.ObjectIdentifier) asn1.RawValue {
	t.Helper()
	oidDER, err := asn1.Marshal(oid)
	if err != nil {
		t.Fatalf("marshal registeredID OID: %v", err)
	}
	var raw asn1.RawValue
	if _, err := asn1.Unmarshal(oidDER, &raw); err != nil {
		t.Fatalf("unmarshal OID raw: %v", err)
	}
	return asn1.RawValue{Class: asn1.ClassContextSpecific, Tag: 8, Bytes: raw.Bytes}
}

func makeValidDNSOfLen(target int) string {
	var b strings.Builder
	for b.Len() < target {
		rem := target - b.Len()
		if b.Len() > 0 {
			b.WriteByte('.')
			rem--
		}
		seg := min(rem, 63)
		if rem-seg == 1 {
			seg--
		}
		b.WriteString(strings.Repeat("a", seg))
	}
	return b.String()
}

// TestRFC5216IdentityIsTakenFromTheSANAndAuthorized is the RFC 5216 Sections
// 5.2 and 5.3 core test: SAN wire-order extraction for PeerID/PeerIDs and
// ServerID/ServerIDs, eapTLSPeerName SAN-first behavior, SAN-over-CN no-fallback
// invariant, and rejection of unauthorized CA-signed certificates on both roles.
//
// RFC requirement: RFC5216-5.2-1 positive -- when subjectAltName is present,
// Peer-Id and Server-Id are set to the contents of subjectAltName in
// certificate wire order rather than Subject CN.
// RFC requirement: RFC5216-5.2-1 negative -- when subjectAltName is present
// (even with zero supported GeneralNames), Subject CN is never consulted as
// Peer-Id or Server-Id for authorization.
// RFC requirement: RFC5216-5.3-2 positive -- an authorized client SAN and
// server SAN complete authentication.
// RFC requirement: RFC5216-5.3-2 negative -- a valid CA-signed client cert
// whose SAN is not in AllowedIdentities is refused even when Subject CN matches.
// RFC requirement: RFC5216-5.3-3 negative -- a valid CA-signed server cert
// whose SAN does not match ServerIdentity is refused even when Subject CN matches.
func TestRFC5216IdentityIsTakenFromTheSANAndAuthorized(t *testing.T) {
	pki := newEAPTLSPKI(t)
	serverAuth := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	clientAuth := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}

	serverCert, serverKey := newLeafWithSAN(
		t, pki.trustedCA, pki.trustedCAKey,
		"server-cn-ignored", 20, serverAuth,
		[]string{testServerSAN},
	)
	// Build a client certificate with multiple SAN types in an explicit ASN.1
	// wire order so we prove wire-order preservation across all five SAN kinds.
	clientSANOrder := []string{
		"spiffe://corp.example/node/1",
		"192.0.2.10",
		testClientNAI,
		testClientSAN,
		"upn-user@corp.example.com",
	}
	clientCert, clientKey := newCustomLeaf(
		t, pki.trustedCA, pki.trustedCAKey, 21,
		pkix.Name{CommonName: "client-cn-ignored"},
		clientAuth,
		func(tmpl *x509.Certificate) {
			tmpl.ExtraExtensions = []pkix.Extension{
				marshalSANExtension(t,
					sanGeneralString(6, clientSANOrder[0]),
					sanGeneralIP(net.ParseIP(clientSANOrder[1])),
					sanGeneralString(1, clientSANOrder[2]),
					sanGeneralString(2, clientSANOrder[3]),
					sanGeneralUPN(t, clientSANOrder[4]),
				),
			}
		},
	)

	t.Run("authorized SANs extracted in wire order and exported on both roles", func(t *testing.T) {
		serverCfg := MethodConfig{
			ServerCertPEM:     serverCert,
			ServerKeyPEM:      serverKey,
			CACertPEM:         pki.trustedCAPEM,
			CRLPEM:            pki.trustedCRLPEM,
			Resumption:        NewResumption(time.Now, true),
			AllowedIdentities: []string{testClientSAN},
		}
		peer := NewPeerSessionTLS(testClientNAI, &PeerTLSConfig{
			CertPEM:        clientCert,
			KeyPEM:         clientKey,
			CACertPEM:      pki.trustedCAPEM,
			CRLPEM:         pki.trustedCRLPEM,
			ServerIdentity: testServerSAN,
		})

		res := runEAPTLSHandshake(t, serverCfg, peer)
		if res.peerErr != nil {
			t.Fatalf("authorized handshake failed on peer: %v", res.peerErr)
		}
		if !res.serverEAPSuccess || !res.peerDone {
			t.Fatalf("authorized handshake did not complete: serverSuccess=%v peerDone=%v serverErr=%v",
				res.serverEAPSuccess, res.peerDone, res.sess.Err())
		}
		if got := res.sess.PeerID(); got != clientSANOrder[0] {
			t.Errorf("Session.PeerID() = %q, want first wire-order SAN %q", got, clientSANOrder[0])
		}
		if got := res.sess.PeerIDs(); !slices.Equal(got, clientSANOrder) {
			t.Errorf("Session.PeerIDs() = %v, want wire order %v", got, clientSANOrder)
		}
		if got := res.peer.ServerID(); got != testServerSAN {
			t.Errorf("PeerSession.ServerID() = %q, want %q", got, testServerSAN)
		}
		if got := res.peer.ServerIDs(); !slices.Equal(got, []string{testServerSAN}) {
			t.Errorf("PeerSession.ServerIDs() = %v, want [%s]", got, testServerSAN)
		}
		if got := eapTLSPeerName(res.serverState()); got != clientSANOrder[0] {
			t.Errorf("eapTLSPeerName(server) = %q, want SAN %q", got, clientSANOrder[0])
		}
		if got := eapTLSPeerName(res.peerState()); got != testServerSAN {
			t.Errorf("eapTLSPeerName(peer) = %q, want SAN %q", got, testServerSAN)
		}
	})

	t.Run("authenticator rejects valid CA-signed client with unauthorized SAN despite matching CN", func(t *testing.T) {
		rogueClientCert, rogueClientKey := newLeafWithSAN(
			t, pki.trustedCA, pki.trustedCAKey,
			testClientSAN, 22, clientAuth,
			[]string{"rogue.example.com"},
		)
		serverCfg := MethodConfig{
			ServerCertPEM:     serverCert,
			ServerKeyPEM:      serverKey,
			CACertPEM:         pki.trustedCAPEM,
			CRLPEM:            pki.trustedCRLPEM,
			Resumption:        NewResumption(time.Now, true),
			AllowedIdentities: []string{testClientSAN},
		}
		peer := NewPeerSessionTLS(testClientNAI, &PeerTLSConfig{
			CertPEM:        rogueClientCert,
			KeyPEM:         rogueClientKey,
			CACertPEM:      pki.trustedCAPEM,
			CRLPEM:         pki.trustedCRLPEM,
			ServerIdentity: testServerSAN,
		})

		res := runEAPTLSHandshake(t, serverCfg, peer)
		if res.serverEAPSuccess {
			t.Fatal("authenticator sent EAP-Success for a client certificate whose SAN is not in AllowedIdentities")
		}
		if res.sess.Err() == nil {
			t.Fatal("authenticator recorded no error for rejected client identity")
		}
		if !strings.Contains(res.sess.Err().Error(), "rogue.example.com") {
			t.Errorf("authenticator error %q does not name extracted SAN Peer-Id", res.sess.Err())
		}
	})

	t.Run("peer rejects valid CA-signed server with mismatched SAN despite matching CN", func(t *testing.T) {
		rogueServerCert, rogueServerKey := newLeafWithSAN(
			t, pki.trustedCA, pki.trustedCAKey,
			testServerSAN, 23, serverAuth,
			[]string{"rogue-server.example.com"},
		)
		serverCfg := MethodConfig{
			ServerCertPEM:     rogueServerCert,
			ServerKeyPEM:      rogueServerKey,
			CACertPEM:         pki.trustedCAPEM,
			CRLPEM:            pki.trustedCRLPEM,
			Resumption:        NewResumption(time.Now, true),
			AllowedIdentities: []string{testClientSAN},
		}
		peer := NewPeerSessionTLS(testClientNAI, &PeerTLSConfig{
			CertPEM:        clientCert,
			KeyPEM:         clientKey,
			CACertPEM:      pki.trustedCAPEM,
			CRLPEM:         pki.trustedCRLPEM,
			ServerIdentity: testServerSAN,
		})

		res := runEAPTLSHandshake(t, serverCfg, peer)
		if res.peerDone {
			t.Fatal("peer completed EAP-TLS against a server certificate whose SAN does not match ServerIdentity")
		}
		if res.peerErr == nil {
			t.Fatal("peer recorded no error for mismatched server identity")
		}
		if !strings.Contains(res.peerErr.Error(), "rogue-server.example.com") || !strings.Contains(res.peerErr.Error(), testServerSAN) {
			t.Errorf("peer error %q should name both presented and expected Server-Id", res.peerErr)
		}
	})

	t.Run("SAN extension with zero supported GeneralNames never falls back to CN even under wildcard", func(t *testing.T) {
		unsupportedSANClient, unsupportedSANClientKey := newCustomLeaf(
			t, pki.trustedCA, pki.trustedCAKey, 24,
			pkix.Name{CommonName: testClientSAN},
			clientAuth,
			func(tmpl *x509.Certificate) {
				tmpl.ExtraExtensions = []pkix.Extension{
					marshalSANExtension(t, sanGeneralRegisteredID(t, asn1.ObjectIdentifier{1, 2, 3, 4})),
				}
			},
		)
		serverCfg := MethodConfig{
			ServerCertPEM:     serverCert,
			ServerKeyPEM:      serverKey,
			CACertPEM:         pki.trustedCAPEM,
			CRLPEM:            pki.trustedCRLPEM,
			Resumption:        NewResumption(time.Now, true),
			AllowedIdentities: []string{"*"},
		}
		peer := NewPeerSessionTLS(testClientNAI, &PeerTLSConfig{
			CertPEM:        unsupportedSANClient,
			KeyPEM:         unsupportedSANClientKey,
			CACertPEM:      pki.trustedCAPEM,
			CRLPEM:         pki.trustedCRLPEM,
			ServerIdentity: testServerSAN,
		})

		res := runEAPTLSHandshake(t, serverCfg, peer)
		if res.serverEAPSuccess {
			t.Fatal("authenticator accepted a client cert whose SAN extension has zero supported entries")
		}
		if res.sess.Err() == nil {
			t.Fatal("authenticator recorded no error for client cert with empty usable SANs")
		}
	})
}

// TestRFC5216AllowedIdentitiesMatchingMatrix verifies every authenticator
// allowed-identities entry form, negative boundary, class separation, client
// EKU enforcement, and independence from the unauthenticated Identity Response.
//
// RFC requirement: RFC5216-5.2-1 positive -- SAN entries of type dNSName,
// rfc822Name, iPAddress, uniformResourceIdentifier, and UPN otherName are
// extracted as Peer-Id; CN and Subject DN are used only when SAN is absent.
// RFC requirement: RFC5216-5.3-2 positive -- exact DNS, single-label DNS
// wildcard, exact NAI, NAI realm, exact IP, exact URI, and explicit lone "*"
// authorize matching valid CA-signed client certificates.
// RFC requirement: RFC5216-5.3-2 negative -- multi-label DNS wildcard, apex
// DNS, bare @realm, IP-in-dNSName/CN, URI-in-CN, Unicode Kelvin-sign folding,
// empty Subject, mismatched EKU, and spoofed EAP-Response/Identity are refused.
func TestRFC5216AllowedIdentitiesMatchingMatrix(t *testing.T) {
	pki := newEAPTLSPKI(t)
	serverAuth := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	clientAuth := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}

	serverCert, serverKey := newLeafWithSAN(
		t, pki.trustedCA, pki.trustedCAKey,
		"server-cn", 30, serverAuth,
		[]string{testServerSAN},
	)

	makeURI := func(raw string) []*url.URL {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatalf("parse URI %q: %v", raw, err)
		}
		return []*url.URL{u}
	}

	var nextSerial int64 = 100
	runCase := func(
		t *testing.T,
		eapIdentity string,
		allowed []string,
		subject pkix.Name,
		eku []x509.ExtKeyUsage,
		mutate func(*x509.Certificate),
		wantAccept bool,
	) {
		t.Helper()
		nextSerial++
		certPEM, keyPEM := newCustomLeaf(t, pki.trustedCA, pki.trustedCAKey, nextSerial, subject, eku, mutate)
		serverCfg := MethodConfig{
			ServerCertPEM:     serverCert,
			ServerKeyPEM:      serverKey,
			CACertPEM:         pki.trustedCAPEM,
			CRLPEM:            pki.trustedCRLPEM,
			Resumption:        NewResumption(time.Now, true),
			AllowedIdentities: allowed,
		}
		peer := NewPeerSessionTLS(eapIdentity, &PeerTLSConfig{
			CertPEM:        certPEM,
			KeyPEM:         keyPEM,
			CACertPEM:      pki.trustedCAPEM,
			CRLPEM:         pki.trustedCRLPEM,
			ServerIdentity: testServerSAN,
		})
		res := runEAPTLSHandshake(t, serverCfg, peer)
		if wantAccept {
			if !res.serverEAPSuccess || !res.peerDone {
				t.Fatalf("expected acceptance for allowed=%v, got serverSuccess=%v peerDone=%v serverErr=%v peerErr=%v",
					allowed, res.serverEAPSuccess, res.peerDone, res.sess.Err(), res.peerErr)
			}
			return
		}
		if res.serverEAPSuccess {
			t.Fatalf("expected refusal for allowed=%v, but authenticator sent EAP-Success (PeerID=%q)", allowed, res.sess.PeerID())
		}
		if res.sess.Err() == nil {
			t.Fatalf("expected non-nil Session.Err() on refusal for allowed=%v", allowed)
		}
	}

	t.Run("exact DNS ASCII case-insensitive match", func(t *testing.T) {
		runCase(t, testClientNAI, []string{"gw1.branch.example.com"},
			pkix.Name{CommonName: "ignored"}, clientAuth,
			func(c *x509.Certificate) { c.DNSNames = []string{"GW1.BRANCH.EXAMPLE.COM"} }, true)
	})

	t.Run("ASCII-only folding rejects Unicode Kelvin sign U+212A for k", func(t *testing.T) {
		runCase(t, testClientNAI, []string{"key.example.com"},
			pkix.Name{CommonName: "\u212aey.example.com"}, clientAuth, nil, false)
	})

	t.Run("single-label DNS wildcard matches single label", func(t *testing.T) {
		runCase(t, testClientNAI, []string{"*.branch.example.com"},
			pkix.Name{CommonName: "ignored"}, clientAuth,
			func(c *x509.Certificate) { c.DNSNames = []string{"gw1.branch.example.com"} }, true)
	})

	t.Run("single-label DNS wildcard refuses multi-label", func(t *testing.T) {
		runCase(t, testClientNAI, []string{"*.branch.example.com"},
			pkix.Name{CommonName: "ignored"}, clientAuth,
			func(c *x509.Certificate) { c.DNSNames = []string{"a.b.branch.example.com"} }, false)
	})

	t.Run("single-label DNS wildcard refuses apex", func(t *testing.T) {
		runCase(t, testClientNAI, []string{"*.branch.example.com"},
			pkix.Name{CommonName: "ignored"}, clientAuth,
			func(c *x509.Certificate) { c.DNSNames = []string{"branch.example.com"} }, false)
	})

	t.Run("exact NAI matches rfc822Name SAN", func(t *testing.T) {
		runCase(t, testClientNAI, []string{"alice@example.com"},
			pkix.Name{CommonName: "ignored"}, clientAuth,
			func(c *x509.Certificate) { c.EmailAddresses = []string{"ALICE@EXAMPLE.COM"} }, true)
	})

	t.Run("exact NAI matches UPN otherName SAN", func(t *testing.T) {
		runCase(t, testClientNAI, []string{"alice@example.com"},
			pkix.Name{CommonName: "ignored"}, clientAuth,
			func(c *x509.Certificate) {
				c.ExtraExtensions = []pkix.Extension{marshalSANExtension(t, sanGeneralUPN(t, "Alice@Example.com"))}
			}, true)
	})

	t.Run("NAI realm matches rfc822Name and UPN otherName", func(t *testing.T) {
		runCase(t, testClientNAI, []string{"@example.com"},
			pkix.Name{CommonName: "ignored"}, clientAuth,
			func(c *x509.Certificate) { c.EmailAddresses = []string{"bob@EXAMPLE.COM"} }, true)
		runCase(t, testClientNAI, []string{"@example.com"},
			pkix.Name{CommonName: "ignored"}, clientAuth,
			func(c *x509.Certificate) {
				c.ExtraExtensions = []pkix.Extension{marshalSANExtension(t, sanGeneralUPN(t, "bob@example.com"))}
			}, true)
	})

	t.Run("NAI realm refuses bare @example.com", func(t *testing.T) {
		runCase(t, testClientNAI, []string{"@example.com"},
			pkix.Name{CommonName: "ignored"}, clientAuth,
			func(c *x509.Certificate) {
				c.ExtraExtensions = []pkix.Extension{marshalSANExtension(t, sanGeneralString(1, "@example.com"))}
			}, false)
	})

	t.Run("exact IPv4 and IPv6 match iPAddress SAN only", func(t *testing.T) {
		runCase(t, testClientNAI, []string{"192.0.2.10"},
			pkix.Name{CommonName: "ignored"}, clientAuth,
			func(c *x509.Certificate) { c.IPAddresses = []net.IP{net.ParseIP("192.0.2.10")} }, true)
		runCase(t, testClientNAI, []string{"2001:db8::10"},
			pkix.Name{CommonName: "ignored"}, clientAuth,
			func(c *x509.Certificate) { c.IPAddresses = []net.IP{net.ParseIP("2001:db8::10")} }, true)
	})

	t.Run("exact IP refuses dNSName SAN and refuses CN fallback", func(t *testing.T) {
		runCase(t, testClientNAI, []string{"192.0.2.10"},
			pkix.Name{CommonName: "192.0.2.10"}, clientAuth,
			func(c *x509.Certificate) { c.DNSNames = []string{"192.0.2.10"} }, false)
		runCase(t, testClientNAI, []string{"192.0.2.10"},
			pkix.Name{CommonName: "192.0.2.10"}, clientAuth, nil, false)
	})

	t.Run("exact URI matches uniformResourceIdentifier SAN octet-for-octet only", func(t *testing.T) {
		runCase(t, testClientNAI, []string{"spiffe://corp.example/node/1"},
			pkix.Name{CommonName: "ignored"}, clientAuth,
			func(c *x509.Certificate) { c.URIs = makeURI("spiffe://corp.example/node/1") }, true)
		runCase(t, testClientNAI, []string{"urn:ze:client:1"},
			pkix.Name{CommonName: "ignored"}, clientAuth,
			func(c *x509.Certificate) { c.URIs = makeURI("urn:ze:client:1") }, true)
		// Case difference must fail (octet-for-octet).
		runCase(t, testClientNAI, []string{"spiffe://corp.example/node/1"},
			pkix.Name{CommonName: "ignored"}, clientAuth,
			func(c *x509.Certificate) { c.URIs = makeURI("spiffe://corp.example/NODE/1") }, false)
		// Never falls back to CN.
		runCase(t, testClientNAI, []string{"spiffe://corp.example/node/1"},
			pkix.Name{CommonName: "spiffe://corp.example/node/1"}, clientAuth, nil, false)
	})

	t.Run("explicit lone wildcard accepts Subject DN without CN and refuses empty Subject", func(t *testing.T) {
		runCase(t, testClientNAI, []string{"*"},
			pkix.Name{Organization: []string{"Example Corp"}}, clientAuth, nil, true)
		runCase(t, testClientNAI, []string{"*"},
			pkix.Name{}, clientAuth, nil, false)
	})

	t.Run("unauthenticated EAP-Response/Identity cannot satisfy AllowedIdentities", func(t *testing.T) {
		runCase(t, "alice@example.com", []string{"alice@example.com"},
			pkix.Name{CommonName: "ignored"}, clientAuth,
			func(c *x509.Certificate) { c.EmailAddresses = []string{"mallory@example.com"} }, false)
	})

	t.Run("client certificate with ServerAuth-only EKU is refused", func(t *testing.T) {
		runCase(t, testClientNAI, []string{testClientSAN},
			pkix.Name{CommonName: "ignored"}, serverAuth,
			func(c *x509.Certificate) { c.DNSNames = []string{testClientSAN} }, false)
	})
}

// TestRFC5216ServerIdentityRFC2818Rules verifies RFC 2818 Section 3.1 server
// certificate identity matching on the EAP-TLS peer role.
//
// RFC requirement: RFC5216-5.3-3 positive -- peer matches exact DNS (ASCII
// case-insensitive), leftmost single-label cert wildcard *.a.b, exact IP
// against iPAddress SAN, exact NAI against rfc822Name/UPN SAN, and most-specific
// Subject CN only when no SAN extension exists.
// RFC requirement: RFC5216-5.3-3 negative -- peer refuses multi-label wildcard,
// apex against wildcard, single-label *.com wildcard, partial/inner wildcards,
// IP literal against dNSName or CN, CN when any SAN extension exists, earlier
// non-most-specific CN, and ServerAuth-less EKU.
func TestRFC5216ServerIdentityRFC2818Rules(t *testing.T) {
	pki := newEAPTLSPKI(t)
	serverAuth := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	clientAuth := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}

	clientCert, clientKey := newLeafWithSAN(
		t, pki.trustedCA, pki.trustedCAKey,
		"client-cn", 40, clientAuth,
		[]string{testClientSAN},
	)

	var nextSerial int64 = 200
	runCase := func(
		t *testing.T,
		serverIdentity string,
		subject pkix.Name,
		eku []x509.ExtKeyUsage,
		mutate func(*x509.Certificate),
		wantAccept bool,
	) {
		t.Helper()
		nextSerial++
		serverCert, serverKey := newCustomLeaf(t, pki.trustedCA, pki.trustedCAKey, nextSerial, subject, eku, mutate)
		serverCfg := MethodConfig{
			ServerCertPEM:     serverCert,
			ServerKeyPEM:      serverKey,
			CACertPEM:         pki.trustedCAPEM,
			CRLPEM:            pki.trustedCRLPEM,
			Resumption:        NewResumption(time.Now, true),
			AllowedIdentities: []string{testClientSAN},
		}
		peer := NewPeerSessionTLS(testClientNAI, &PeerTLSConfig{
			CertPEM:        clientCert,
			KeyPEM:         clientKey,
			CACertPEM:      pki.trustedCAPEM,
			CRLPEM:         pki.trustedCRLPEM,
			ServerIdentity: serverIdentity,
		})
		res := runEAPTLSHandshake(t, serverCfg, peer)
		if wantAccept {
			if !res.peerDone || !res.serverEAPSuccess {
				t.Fatalf("expected acceptance for server-identity=%q, got peerDone=%v serverSuccess=%v peerErr=%v serverErr=%v",
					serverIdentity, res.peerDone, res.serverEAPSuccess, res.peerErr, res.sess.Err())
			}
			return
		}
		if res.peerDone {
			t.Fatalf("expected refusal for server-identity=%q, but peer completed (ServerID=%q)", serverIdentity, res.peer.ServerID())
		}
		if res.peerErr == nil {
			t.Fatalf("expected non-nil peerErr on refusal for server-identity=%q", serverIdentity)
		}
	}

	t.Run("exact DNS ASCII case-insensitive", func(t *testing.T) {
		runCase(t, "vpn.example.com", pkix.Name{CommonName: "ignored"}, serverAuth,
			func(c *x509.Certificate) { c.DNSNames = []string{"VPN.EXAMPLE.COM"} }, true)
	})

	t.Run("cert leftmost single-label wildcard matches single label", func(t *testing.T) {
		runCase(t, "vpn.example.com", pkix.Name{CommonName: "ignored"}, serverAuth,
			func(c *x509.Certificate) { c.DNSNames = []string{"*.example.com"} }, true)
	})

	t.Run("cert wildcard refuses multi-label and apex", func(t *testing.T) {
		runCase(t, "a.b.example.com", pkix.Name{CommonName: "ignored"}, serverAuth,
			func(c *x509.Certificate) { c.DNSNames = []string{"*.example.com"} }, false)
		runCase(t, "example.com", pkix.Name{CommonName: "ignored"}, serverAuth,
			func(c *x509.Certificate) { c.DNSNames = []string{"*.example.com"} }, false)
	})

	t.Run("cert single-label *.com and partial or inner wildcards are refused", func(t *testing.T) {
		runCase(t, "example.com", pkix.Name{CommonName: "ignored"}, serverAuth,
			func(c *x509.Certificate) { c.DNSNames = []string{"*.com"} }, false)
		runCase(t, "vpn.example.com", pkix.Name{CommonName: "ignored"}, serverAuth,
			func(c *x509.Certificate) { c.DNSNames = []string{"v*.example.com"} }, false)
		runCase(t, "a.b.example.com", pkix.Name{CommonName: "ignored"}, serverAuth,
			func(c *x509.Certificate) { c.DNSNames = []string{"a.*.example.com"} }, false)
	})

	t.Run("IP literal matches iPAddress SAN only and refuses dNSName or CN", func(t *testing.T) {
		runCase(t, "192.0.2.10", pkix.Name{CommonName: "ignored"}, serverAuth,
			func(c *x509.Certificate) { c.IPAddresses = []net.IP{net.ParseIP("192.0.2.10")} }, true)
		runCase(t, "2001:db8::10", pkix.Name{CommonName: "ignored"}, serverAuth,
			func(c *x509.Certificate) { c.IPAddresses = []net.IP{net.ParseIP("2001:db8::10")} }, true)
		runCase(t, "192.0.2.10", pkix.Name{CommonName: "192.0.2.10"}, serverAuth,
			func(c *x509.Certificate) { c.DNSNames = []string{"192.0.2.10"} }, false)
		runCase(t, "192.0.2.10", pkix.Name{CommonName: "192.0.2.10"}, serverAuth, nil, false)
	})

	t.Run("exact NAI matches rfc822Name SAN", func(t *testing.T) {
		runCase(t, "server@example.com", pkix.Name{CommonName: "ignored"}, serverAuth,
			func(c *x509.Certificate) { c.EmailAddresses = []string{"SERVER@EXAMPLE.COM"} }, true)
	})

	t.Run("CN fallback runs only when SAN extension is absent and uses most specific CN", func(t *testing.T) {
		multiCN := pkix.Name{
			ExtraNames: []pkix.AttributeTypeAndValue{
				{Type: oidCommonName, Value: "first.example.com"},
				{Type: oidCommonName, Value: "second.example.com"},
			},
		}
		runCase(t, "second.example.com", multiCN, serverAuth, nil, true)
		runCase(t, "first.example.com", multiCN, serverAuth, nil, false)
		// When ANY SAN extension exists (even IP-only), CN is never consulted.
		runCase(t, "vpn.example.com", pkix.Name{CommonName: "vpn.example.com"}, serverAuth,
			func(c *x509.Certificate) { c.IPAddresses = []net.IP{net.ParseIP("192.0.2.99")} }, false)
	})

	t.Run("server certificate with ClientAuth-only EKU is refused", func(t *testing.T) {
		runCase(t, testServerSAN, pkix.Name{CommonName: "ignored"}, clientAuth,
			func(c *x509.Certificate) { c.DNSNames = []string{testServerSAN} }, false)
	})
}

// TestRFC5216ResumedSessionReverifiesPeerAndServerIdentity proves that TLS 1.3
// resumed sessions re-run identity authorization on both roles against cached
// certificates and export verified PeerID/ServerID (RFC 9190 Section 5.7).
//
// RFC requirement: RFC5216-5.3-2 positive -- a resumed TLS 1.3 session whose
// cached leaf certificate still matches AllowedIdentities and ServerIdentity
// succeeds and exports PeerID/ServerID.
// RFC requirement: RFC5216-5.3-2 negative -- a resumed TLS 1.3 session whose
// cached client leaf no longer matches narrowed AllowedIdentities is refused.
// RFC requirement: RFC5216-5.3-3 negative -- a resumed TLS 1.3 session whose
// cached server leaf no longer matches updated ServerIdentity is refused.
func TestRFC5216ResumedSessionReverifiesPeerAndServerIdentity(t *testing.T) {
	pki := newEAPTLSPKI(t)
	serverAuth := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	clientAuth := []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}

	serverCert, serverKey := newLeafWithSAN(
		t, pki.trustedCA, pki.trustedCAKey,
		"server-cn", 50, serverAuth,
		[]string{testServerSAN},
	)
	clientCert, clientKey := newLeafWithSAN(
		t, pki.trustedCA, pki.trustedCAKey,
		"client-cn", 51, clientAuth,
		[]string{testClientSAN},
	)

	serverStore := NewResumption(time.Now, true)
	peerStore := NewResumption(time.Now, true)

	makeServerCfg := func(allowed []string) MethodConfig {
		return MethodConfig{
			ServerCertPEM:     serverCert,
			ServerKeyPEM:      serverKey,
			CACertPEM:         pki.trustedCAPEM,
			CRLPEM:            pki.trustedCRLPEM,
			Resumption:        serverStore,
			AllowedIdentities: allowed,
		}
	}
	makePeer := func(serverID string) *PeerSession {
		return NewPeerSessionTLS(testClientNAI, &PeerTLSConfig{
			CertPEM:        clientCert,
			KeyPEM:         clientKey,
			CACertPEM:      pki.trustedCAPEM,
			CRLPEM:         pki.trustedCRLPEM,
			Resumption:     peerStore,
			ServerIdentity: serverID,
		})
	}

	// 1. Initial full TLS 1.3 handshake populates the ticket cache.
	firstPeer := makePeer(testServerSAN)
	first := driveEAPTLSFlight(t, makeServerCfg([]string{testClientSAN}), firstPeer, tls.VersionTLS13, eapTLS13Rounds)
	requireCompleted(t, first, "initial full TLS 1.3 handshake")
	if first.sess.Resumed() || firstPeer.Resumed() {
		t.Fatal("initial handshake unexpectedly reported resumption")
	}

	// 2. Resumed TLS 1.3 handshake with matching policy succeeds and exports IDs.
	resumedPeer := makePeer(testServerSAN)
	resumed := driveEAPTLSFlight(t, makeServerCfg([]string{testClientSAN}), resumedPeer, tls.VersionTLS13, eapTLS13Rounds)
	requireCompleted(t, resumed, "authorized resumed TLS 1.3 handshake")
	if !resumed.sess.Resumed() || !resumedPeer.Resumed() {
		t.Fatalf("expected resumed handshake: server=%v peer=%v", resumed.sess.Resumed(), resumedPeer.Resumed())
	}
	if got := resumed.sess.PeerID(); got != testClientSAN {
		t.Errorf("resumed Session.PeerID() = %q, want %q", got, testClientSAN)
	}
	if got := resumedPeer.ServerID(); got != testServerSAN {
		t.Errorf("resumed PeerSession.ServerID() = %q, want %q", got, testServerSAN)
	}

	// 3. Authenticator narrows AllowedIdentities while keeping the same ticket store:
	// resumed handshake MUST fail closed on the cached client leaf.
	narrowedAuthPeer := makePeer(testServerSAN)
	narrowedAuth := driveEAPTLSFlight(t, makeServerCfg([]string{"other-client.example.com"}), narrowedAuthPeer, tls.VersionTLS13, eapTLS13Rounds)
	if narrowedAuth.successAt >= 0 {
		t.Fatal("authenticator sent EAP-Success on a resumed session whose cached client cert is no longer in AllowedIdentities")
	}
	if !narrowedAuth.sess.Resumed() {
		t.Fatal("authenticator refusal came from a full handshake rather than the resumed handshake under test")
	}
	if narrowedAuth.sess.Err() == nil {
		t.Fatal("authenticator recorded no error when rejecting resumed client identity")
	}

	// 4. Peer changes expected ServerIdentity while reusing its ticket cache:
	// resumed handshake MUST fail closed on the cached server leaf.
	// Re-prime the peer cache first because TLS 1.3 tickets are single-use.
	reprimePeer := makePeer(testServerSAN)
	reprime := driveEAPTLSFlight(t, makeServerCfg([]string{testClientSAN}), reprimePeer, tls.VersionTLS13, eapTLS13Rounds)
	requireCompleted(t, reprime, "re-prime ticket cache")

	changedServerPeer := makePeer("other-server.example.com")
	changedServer := driveEAPTLSFlight(t, makeServerCfg([]string{testClientSAN}), changedServerPeer, tls.VersionTLS13, eapTLS13Rounds)
	if changedServer.peerDone || changedServer.successAt >= 0 {
		t.Fatal("peer completed a resumed session whose cached server cert does not match updated ServerIdentity")
	}
	if !changedServerPeer.Resumed() {
		t.Fatal("peer refusal came from a full handshake rather than the resumed handshake under test")
	}
	if changedServer.peerErr == nil {
		t.Fatal("peer recorded no error when rejecting resumed server identity")
	}
}

// TestRFC5216FailClosedWhenIdentityPolicyUnset verifies fail-closed guards in
// both EAP-TLS constructors when identity policy is missing or invalid.
//
// RFC requirement: RFC5216-5.3-2 negative -- newTLSMethod refuses empty
// AllowedIdentities and refuses mixing "*" with specific entries.
// RFC requirement: RFC5216-5.3-3 negative -- startTLSClient refuses empty or
// invalid ServerIdentity.
func TestRFC5216FailClosedWhenIdentityPolicyUnset(t *testing.T) {
	pki := newEAPTLSPKI(t)

	t.Run("authenticator refuses empty AllowedIdentities", func(t *testing.T) {
		cfg := pki.serverConfig()
		cfg.AllowedIdentities = nil
		if _, err := NewSession(TypeTLS, cfg); err == nil {
			t.Fatal("NewSession(TypeTLS) succeeded with empty AllowedIdentities")
		}
	})

	t.Run("authenticator refuses wildcard mixed with specific entry", func(t *testing.T) {
		cfg := pki.serverConfig()
		cfg.AllowedIdentities = []string{"*", testClientSAN}
		if _, err := NewSession(TypeTLS, cfg); err == nil {
			t.Fatal("NewSession(TypeTLS) succeeded with \"*\" mixed alongside another entry")
		}
	})

	t.Run("peer refuses empty ServerIdentity", func(t *testing.T) {
		peer := NewPeerSessionTLS(testClientNAI, &PeerTLSConfig{
			CertPEM:   pki.clientCertPEM,
			KeyPEM:    pki.clientKeyPEM,
			CACertPEM: pki.trustedCAPEM,
			CRLPEM:    pki.trustedCRLPEM,
		})
		if err := peer.startTLSClient(); !errors.Is(err, errNoPeerServerIdentity) {
			t.Fatalf("startTLSClient() = %v, want errNoPeerServerIdentity", err)
		}
	})

	t.Run("peer refuses wildcard in ServerIdentity", func(t *testing.T) {
		peer := NewPeerSessionTLS(testClientNAI, &PeerTLSConfig{
			CertPEM:        pki.clientCertPEM,
			KeyPEM:         pki.clientKeyPEM,
			CACertPEM:      pki.trustedCAPEM,
			CRLPEM:         pki.trustedCRLPEM,
			ServerIdentity: "*.example.com",
		})
		if err := peer.startTLSClient(); err == nil {
			t.Fatal("startTLSClient() accepted wildcard ServerIdentity")
		}
	})
}

// TestRFC5216IdentityParsingBoundaries exercises every numeric length and
// label-count boundary for ParseAllowedIdentity, ParseServerIdentity, and
// IsUnambiguousFQDN.
func TestRFC5216IdentityParsingBoundaries(t *testing.T) {
	valid255 := makeValidDNSOfLen(255)
	invalid256 := makeValidDNSOfLen(256)
	if len(valid255) != 255 || len(invalid256) != 256 {
		t.Fatalf("boundary generator lengths: %d, %d", len(valid255), len(invalid256))
	}

	allowedValid := []string{
		"*",
		"a",
		"swan-eap-client",
		"vpn.example.com",
		valid255,
		"*.example.com",
		"*.branch.example.com",
		"alice@example.com",
		"@example.com",
		"192.0.2.10",
		"2001:db8::10",
		"spiffe://corp.example/node/1",
		"urn:ze:client:1",
	}
	for _, in := range allowedValid {
		if _, err := ParseAllowedIdentity(in); err != nil {
			t.Errorf("ParseAllowedIdentity(%q) failed: %v", in, err)
		}
	}

	allowedInvalid := []string{
		"",
		invalid256,
		"*.com",
		"*.",
		"*.*.example.com",
		"foo*.example.com",
		"a.*.example.com",
		".example.com",
		"vpn.example.com.",
		"-bad.example.com",
		"bad-.example.com",
		"@",
		"@localhost",
		"*@example.com",
		"alice@localhost",
		"192.0.2.0/24",
		"fe80::1%eth0",
		"spiffe://corp.example/*",
		"://missing-scheme",
		"https://",
		"urn:",
		"bad\x00.example.com",
		"bad\r.example.com",
		"bad\n.example.com",
		"bad\x7f.example.com",
		"bad space.example.com",
	}
	for _, in := range allowedInvalid {
		if got, err := ParseAllowedIdentity(in); err == nil {
			t.Errorf("ParseAllowedIdentity(%q) unexpectedly succeeded: %+v", in, got)
		}
	}

	serverValid := []string{
		"a",
		"eap-tls-server",
		"vpn.example.com",
		valid255,
		"172.28.0.3",
		"2001:db8::1",
		"server@example.com",
	}
	for _, in := range serverValid {
		if _, err := ParseServerIdentity(in); err != nil {
			t.Errorf("ParseServerIdentity(%q) failed: %v", in, err)
		}
	}

	serverInvalid := []string{
		"",
		invalid256,
		"*",
		"*.example.com",
		".example.com",
		"@example.com",
		"172.28.0.0/24",
		"fe80::1%eth0",
		"vpn.example.com.",
		"bad\x00.example.com",
		"bad space.example.com",
	}
	for _, in := range serverInvalid {
		if got, err := ParseServerIdentity(in); err == nil {
			t.Errorf("ParseServerIdentity(%q) unexpectedly succeeded: %+v", in, got)
		}
	}

	fqdnValid := []string{"vpn.example", "vpn.example.com", valid255}
	for _, in := range fqdnValid {
		if !IsUnambiguousFQDN(in) {
			t.Errorf("IsUnambiguousFQDN(%q) = false, want true", in)
		}
	}

	fqdnInvalid := []string{
		"",
		"vpn",
		"172.28.0.3",
		"127.0.0.2",
		"2001:db8::1",
		"vpn.example.",
		"*.example.com",
		".example.com",
		"vpn.example\x00.com",
		"vpn example.com",
		invalid256,
	}
	for _, in := range fqdnInvalid {
		if IsUnambiguousFQDN(in) {
			t.Errorf("IsUnambiguousFQDN(%q) = true, want false", in)
		}
	}
}
