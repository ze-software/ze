package engine

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/eap"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: the certificate an operator wrote under `pki certificate` reaches
// the realm the EAP-TLS peer puts in its Identity Response. buildPeerTLSConfig
// (fsm.go) copies it onto eap.PeerTLSConfig.CertPEM, and NewPeerSessionTLS reads
// the NAI out of it when the configured local-id carries no realm.
// PREVENTS: a deployment whose local-id is not an NAI sending the bare
// "anonymous", which no realm routes and which RFC 9190 Section 2.1.3 says makes
// resumption likely impossible, while its own certificate names a realm.

// naiWiringLeaf returns an end-entity certificate carrying one rfc822Name
// subject alternative name, signed by the CA given.
func naiWiringLeaf(t *testing.T, cn, email string, ca *x509.Certificate, caKey *ecdsa.PrivateKey) (*x509.Certificate, []byte, *ecdsa.PrivateKey) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate the leaf key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:   big.NewInt(7),
		Subject:        pkix.Name{CommonName: cn},
		NotBefore:      time.Now().Add(-time.Hour),
		NotAfter:       time.Now().Add(time.Hour),
		KeyUsage:       x509.KeyUsageDigitalSignature,
		ExtKeyUsage:    []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		EmailAddresses: []string{email},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("create the leaf certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse the leaf certificate: %v", err)
	}
	return cert, der, key
}

// TestEAPTLSPeerConfigCarriesTheCertificateTheNAIIsDerivedFrom builds the peer
// configuration the engine builds, hands it to the constructor the engine calls,
// and reads the Identity Response the peer answers with.
//
// The identity is the one startEAPExchange (fsm.go) would pass: the operator's
// local-id, or the peer name when there is none. "branch" carries no realm, so
// the certificate is what the realm can come from.
//
// RFC requirement: RFC9190-2.1.7-1 positive -- "When the client certificate
// contains an NAI as subject name or alternative subject name, an anonymous NAI
// SHOULD be derived from the NAI in the certificate". The certificate under
// `pki certificate` names "operator@branch.example.com", and the Identity
// Response the peer produces is "@branch.example.com": the operator's own
// configuration is what supplies the realm, and the username in it is absent.
func TestEAPTLSPeerConfigCarriesTheCertificateTheNAIIsDerivedFrom(t *testing.T) {
	ca, caDER, caKey := crlWiringCA(t, "nai-wiring-ca")
	leaf, leafDER, leafKey := naiWiringLeaf(t, "nai-wiring-node", "operator@branch.example.com", ca, caKey)
	loadCRLWiringStore(t, ca, caDER, leaf, leafDER, leafKey, nil)

	sa := &SA{PeerName: "branch", PeerCfg: ipsec.SiteToSitePeer{Auth: ipsec.AuthConfig{
		Mode:          ipsec.AuthEAPTLS,
		Certificate:   "crl-cert",
		CACertificate: "crl-ca",
	}}, Resumption: eap.NewResumption(time.Now, true)}

	peerCfg := buildPeerTLSConfig(sa, slogutil.DiscardLogger())
	if peerCfg == nil {
		t.Fatal("buildPeerTLSConfig returned nil for a peer with a certificate and a CA")
	}
	if len(peerCfg.CertPEM) == 0 {
		t.Fatal("the peer config carries no certificate, so no NAI can be derived from one")
	}

	ps := eap.NewPeerSessionTLS(sa.PeerName, peerCfg)
	t.Cleanup(ps.Close)

	res := ps.Process(&eap.Packet{Code: eap.CodeRequest, Identifier: 1, Type: eap.TypeIdentity})
	if res.Response == nil {
		t.Fatalf("the peer answered no Identity Response: %v", res.Err)
	}
	if got := string(res.Response.TypeData); got != "@branch.example.com" {
		t.Fatalf("Identity Response carries %q, want %q derived from the configured certificate", got, "@branch.example.com")
	}
}
