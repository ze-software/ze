package engine

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/component/pki"
)

// rctCA builds a self-signed certificate authority.
func rctCA(t *testing.T, cn string) (*x509.Certificate, []byte, *ecdsa.PrivateKey) {
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
		KeyUsage:              x509.KeyUsageCertSign,
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

// rctLeaf builds an end-entity certificate with no subject alternative name. A nil
// issuer makes it self-signed, which is what an attacker with no access to a trust
// anchor can produce.
func rctLeaf(t *testing.T, cn string, issuer *x509.Certificate, issuerKey *ecdsa.PrivateKey) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()
	return rctLeafWithSAN(t, cn, nil, nil, issuer, issuerKey)
}

// rctLeafWithSAN builds an end-entity certificate that carries the given subject
// alternative names. gen-pki.sh mints server.pem with an address alternative name and
// client.pem with none, so both shapes need a fixture.
func rctLeafWithSAN(t *testing.T, cn string, ips []net.IP, dns []string, issuer *x509.Certificate, issuerKey *ecdsa.PrivateKey) ([]byte, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate the leaf key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  ips,
		DNSNames:     dns,
	}
	parent, parentKey := tmpl, key
	if issuer != nil {
		parent, parentKey = issuer, issuerKey
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, parentKey)
	if err != nil {
		t.Fatalf("create the leaf certificate: %v", err)
	}
	return der, key
}

// rctDigitalSigAuth builds a well-formed RFC 7427 method 14 AUTH payload over the
// octets this SA expects from the remote party, signed by the given key. The
// signature is valid, so anything that refuses the payload refuses it for the trust
// anchor and not for the arithmetic.
func rctDigitalSigAuth(t *testing.T, sa *SA, key *ecdsa.PrivateKey) *wire.PayloadAUTH {
	t.Helper()
	octets, err := computeSignedOctets(sa, !sa.IsInitiator)
	if err != nil {
		t.Fatalf("compute the signed octets: %v", err)
	}
	digest := sha256.Sum256(octets)
	sig, err := ecdsa.SignASN1(rand.Reader, key, digest[:])
	if err != nil {
		t.Fatalf("sign the digest: %v", err)
	}
	data := make([]byte, 0, 1+len(oidECDSASHA256)+len(sig))
	data = append(data, byte(len(oidECDSASHA256)))
	data = append(data, oidECDSASHA256...)
	data = append(data, sig...)
	return &wire.PayloadAUTH{AuthMethod: wire.AuthMethodDigitalSig, AuthData: data}
}

// VALIDATES: a certificate AUTH is refused when the peer has no configured trust
// anchor, whatever the configured authentication mode is.
// PREVENTS: the authentication bypass a missing ca-certificate opened. getRemoteCert
// returned the peer's own certificate unverified, so a self-signed certificate with
// a valid signature authenticated. For eap-mschapv2 Ze then sent the user challenge
// and response to whoever answered. A pre-shared-secret peer was reachable the same
// way, because the AUTH method on the wire selects the verifier, not the config.
func TestRctCertificateAuthNeedsATrustAnchor(t *testing.T) {
	for _, mode := range []ipsec.AuthMode{
		ipsec.AuthEAPMSCHAPv2,
		ipsec.AuthEAPTLS,
		ipsec.AuthX509,
		ipsec.AuthPreSharedSecret,
	} {
		t.Run(mode.String(), func(t *testing.T) {
			sa := testSAWithKeys(t)
			sa.PeerCfg.Auth.Mode = mode
			sa.PeerCfg.Auth.CACertificate = ""

			der, key := rctLeaf(t, "attacker", nil, nil)
			sa.RemoteCertRaw = der

			if err := verifyRemoteAuth(sa, rctDigitalSigAuth(t, sa, key)); err == nil {
				t.Fatal("a self-signed certificate authenticated with no trust anchor configured")
			}
		})
	}
}

// VALIDATES: with a trust anchor configured, a certificate the anchor signed still
// authenticates, and one it did not sign does not.
// PREVENTS: the refusal above turning into a blanket refusal of certificate
// authentication, which would make the negative case pass for the wrong reason.
func TestRctTrustedCertificateStillAuthenticates(t *testing.T) {
	caCert, caDER, caKey := rctCA(t, "rct-ca")
	if err := pki.Load(&pki.PKIConfig{
		CACerts: map[string]*pki.CACertEntry{
			"rct-ca": {Name: "rct-ca", Certificate: caCert, Raw: caDER},
		},
	}); err != nil {
		t.Fatalf("load the PKI store: %v", err)
	}
	t.Cleanup(func() {
		if err := pki.Load(nil); err != nil {
			t.Errorf("clear the PKI store: %v", err)
		}
	})

	// Positive. The anchor signed this certificate, so the AUTH verifies.
	trusted := testSAWithKeys(t)
	trusted.PeerCfg.Auth.Mode = ipsec.AuthX509
	trusted.PeerCfg.Auth.CACertificate = "rct-ca"
	der, key := rctLeaf(t, "rct-peer", caCert, caKey)
	trusted.RemoteCertRaw = der
	if err := verifyRemoteAuth(trusted, rctDigitalSigAuth(t, trusted, key)); err != nil {
		t.Fatalf("a certificate the configured anchor signed was refused: %v", err)
	}

	// Negative. A self-signed certificate does not chain to the same anchor.
	untrusted := testSAWithKeys(t)
	untrusted.PeerCfg.Auth.Mode = ipsec.AuthX509
	untrusted.PeerCfg.Auth.CACertificate = "rct-ca"
	otherDER, otherKey := rctLeaf(t, "attacker", nil, nil)
	untrusted.RemoteCertRaw = otherDER
	if err := verifyRemoteAuth(untrusted, rctDigitalSigAuth(t, untrusted, otherKey)); err == nil {
		t.Error("a self-signed certificate authenticated against a configured anchor")
	}
}
