package engine

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/eap"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: the revocation lists an operator wrote under `pki ca` reach BOTH
// EAP-TLS roles. eapTLSServerConfig (responder_eap.go) and buildPeerTLSConfig
// (fsm.go) each copy them onto the config the EAP method reads, and each answers
// nil for a CA that holds none.
// PREVENTS: a CRL that parses at config load and never arrives at the check,
// which would leave every TLS 1.3 EAP-TLS session refused for want of a source
// the operator did configure (eap.checkChainRevocation, RFC 9190 Section 5.4).

// crlWiringCA returns a CA that can sign revocation lists, plus its DER and key.
func crlWiringCA(t *testing.T, cn string) (*x509.Certificate, []byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate the CA key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create the CA certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse the CA certificate: %v", err)
	}
	return cert, der, key
}

// crlWiringLeaf returns an end-entity certificate and its key, signed by the CA.
func crlWiringLeaf(t *testing.T, cn string, serial int64, ca *x509.Certificate, caKey *ecdsa.PrivateKey) (*x509.Certificate, []byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate the leaf key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
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

// crlWiringList issues a revocation list naming one serial number.
func crlWiringList(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, revoked int64) []byte {
	t.Helper()
	der, err := x509.CreateRevocationList(rand.Reader, &x509.RevocationList{
		Number:     big.NewInt(1),
		ThisUpdate: time.Now().Add(-time.Hour),
		NextUpdate: time.Now().Add(time.Hour),
		RevokedCertificateEntries: []x509.RevocationListEntry{{
			SerialNumber:   big.NewInt(revoked),
			RevocationTime: time.Now().Add(-time.Minute),
		}},
	}, ca, caKey)
	if err != nil {
		t.Fatalf("create the revocation list: %v", err)
	}
	return der
}

// loadCRLWiringStore puts one CA and one device certificate in the PKI store,
// giving the CA the revocation lists named, and clears the store afterwards.
func loadCRLWiringStore(t *testing.T, ca *x509.Certificate, caDER []byte, leaf *x509.Certificate, leafDER []byte, leafKey *ecdsa.PrivateKey, crls [][]byte) {
	t.Helper()
	if err := pki.Load(&pki.PKIConfig{
		CACerts: map[string]*pki.CACertEntry{
			"crl-ca": {Name: "crl-ca", Certificate: ca, Raw: caDER, RawCRLs: crls},
		},
		Certificates: map[string]*pki.CertificateEntry{
			"crl-cert": {Name: "crl-cert", Certificate: leaf, Raw: leafDER, PrivateKey: leafKey},
		},
	}); err != nil {
		t.Fatalf("load the PKI store: %v", err)
	}
	t.Cleanup(func() {
		if err := pki.Load(nil); err != nil {
			t.Errorf("clear the PKI store: %v", err)
		}
	})
}

// crlSerials re-parses concatenated PEM revocation lists and returns every
// serial number they revoke, in order.
func crlSerials(t *testing.T, crlPEM []byte) []int64 {
	t.Helper()
	var serials []int64
	rest := crlPEM
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return serials
		}
		list, err := x509.ParseRevocationList(block.Bytes)
		if err != nil {
			t.Fatalf("re-parse the delivered revocation list: %v", err)
		}
		for _, entry := range list.RevokedCertificateEntries {
			serials = append(serials, entry.SerialNumber.Int64())
		}
	}
}

func TestEAPTLSConfigsCarryTheCARevocationLists(t *testing.T) {
	ca, caDER, caKey := crlWiringCA(t, "crl-wiring-ca")
	leaf, leafDER, leafKey := crlWiringLeaf(t, "crl-wiring-node", 2, ca, caKey)
	const revokedSerial int64 = 4242
	loadCRLWiringStore(t, ca, caDER, leaf, leafDER, leafKey, [][]byte{crlWiringList(t, ca, caKey, revokedSerial)})

	sa := &SA{PeerName: "branch", PeerCfg: ipsec.SiteToSitePeer{Auth: ipsec.AuthConfig{
		Mode:          ipsec.AuthEAPTLS,
		Certificate:   "crl-cert",
		CACertificate: "crl-ca",
	}}, Resumption: eap.NewResumption(time.Now, true)}

	serverCfg, err := eapTLSServerConfig(sa)
	if err != nil {
		t.Fatalf("eapTLSServerConfig: %v", err)
	}
	if got := crlSerials(t, serverCfg.CRLPEM); len(got) != 1 || got[0] != revokedSerial {
		t.Fatalf("the authenticator config carries revoked serials %v, want [%d]", got, revokedSerial)
	}

	peerCfg := buildPeerTLSConfig(sa, slogutil.DiscardLogger())
	if peerCfg == nil {
		t.Fatal("buildPeerTLSConfig returned nil for a peer with a certificate and a CA")
	}
	if got := crlSerials(t, peerCfg.CRLPEM); len(got) != 1 || got[0] != revokedSerial {
		t.Fatalf("the peer config carries revoked serials %v, want [%d]", got, revokedSerial)
	}
}

// TestEAPTLSConfigsCarryNoListWhenTheCAHasNone keeps the "nothing configured"
// state distinguishable at the boundary: it must arrive as nil, because that is
// what makes a TLS 1.3 session refuse rather than complete unchecked.
func TestEAPTLSConfigsCarryNoListWhenTheCAHasNone(t *testing.T) {
	ca, caDER, caKey := crlWiringCA(t, "crl-wiring-ca")
	leaf, leafDER, leafKey := crlWiringLeaf(t, "crl-wiring-node", 2, ca, caKey)
	loadCRLWiringStore(t, ca, caDER, leaf, leafDER, leafKey, nil)

	sa := &SA{PeerName: "branch", PeerCfg: ipsec.SiteToSitePeer{Auth: ipsec.AuthConfig{
		Mode:          ipsec.AuthEAPTLS,
		Certificate:   "crl-cert",
		CACertificate: "crl-ca",
	}}, Resumption: eap.NewResumption(time.Now, true)}

	serverCfg, err := eapTLSServerConfig(sa)
	if err != nil {
		t.Fatalf("eapTLSServerConfig: %v", err)
	}
	if serverCfg.CRLPEM != nil {
		t.Fatalf("the authenticator config carries %d octets of CRL PEM for a CA with none", len(serverCfg.CRLPEM))
	}

	peerCfg := buildPeerTLSConfig(sa, slogutil.DiscardLogger())
	if peerCfg == nil {
		t.Fatal("buildPeerTLSConfig returned nil for a peer with a certificate and a CA")
	}
	if peerCfg.CRLPEM != nil {
		t.Fatalf("the peer config carries %d octets of CRL PEM for a CA with none", len(peerCfg.CRLPEM))
	}
}
