package engine

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/x509"
	"math/big"
	"testing"
	"time"

	"golang.org/x/crypto/ocsp"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/pki"
	"github.com/ze-software/ze/internal/core/eap"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// VALIDATES: the OCSP response an operator wrote under `pki certificate` reaches
// the EAP-TLS authenticator, and the certificate-status-request leaf an operator
// wrote under an ipsec peer reaches the EAP-TLS peer. eapTLSServerConfig
// (responder_eap.go) and buildPeerTLSConfig (fsm.go) are the two builders.
// PREVENTS: a stapled response that parses at config load and never reaches the
// certificate message, which would leave RFC 9190 Section 5.4's stapling
// obligation unimplementable by an operator; and a status leaf that commits and
// changes nothing.

// ocspWiringResponse signs an OCSP response about the leaf, with the CA's key.
func ocspWiringResponse(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, serial int64) []byte {
	t.Helper()
	der, err := ocsp.CreateResponse(ca, ca, ocsp.Response{
		Status:       ocsp.Good,
		SerialNumber: big.NewInt(serial),
		ThisUpdate:   time.Now().Add(-time.Hour),
		NextUpdate:   time.Now().Add(time.Hour),
	}, caKey)
	if err != nil {
		t.Fatalf("create the OCSP response: %v", err)
	}
	return der
}

// loadOCSPWiringStore puts one CA and one device certificate in the PKI store,
// giving the certificate the stapled response named, and clears the store
// afterwards.
func loadOCSPWiringStore(
	t *testing.T,
	ca *x509.Certificate, caDER []byte,
	leaf *x509.Certificate, leafDER []byte, leafKey *ecdsa.PrivateKey,
	staple []byte,
) {
	t.Helper()
	if err := pki.Load(&pki.PKIConfig{
		CACerts: map[string]*pki.CACertEntry{
			"ocsp-ca": {Name: "ocsp-ca", Certificate: ca, Raw: caDER},
		},
		Certificates: map[string]*pki.CertificateEntry{
			"ocsp-cert": {
				Name: "ocsp-cert", Certificate: leaf, Raw: leafDER,
				PrivateKey: leafKey, RawOCSPResponse: staple,
			},
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

// ocspWiringSA builds an EAP-TLS SA naming the store entries above.
func ocspWiringSA(statusRequest bool) *SA {
	return &SA{PeerName: "branch", PeerCfg: ipsec.SiteToSitePeer{Auth: ipsec.AuthConfig{
		Mode:                     ipsec.AuthEAPTLS,
		Certificate:              "ocsp-cert",
		CACertificate:            "ocsp-ca",
		CertificateStatusRequest: statusRequest,
	}}, Resumption: eap.NewResumption(time.Now, true)}
}

// TestEAPTLSConfigsCarryTheStapledOCSPResponse checks the authenticator half.
func TestEAPTLSConfigsCarryTheStapledOCSPResponse(t *testing.T) {
	ca, caDER, caKey := crlWiringCA(t, "ocsp-wiring-ca")
	const leafSerial int64 = 77
	leaf, leafDER, leafKey := crlWiringLeaf(t, "ocsp-wiring-node", leafSerial, ca, caKey)
	staple := ocspWiringResponse(t, ca, caKey, leafSerial)
	loadOCSPWiringStore(t, ca, caDER, leaf, leafDER, leafKey, staple)

	serverCfg, err := eapTLSServerConfig(ocspWiringSA(false))
	if err != nil {
		t.Fatalf("eapTLSServerConfig: %v", err)
	}
	if !bytes.Equal(serverCfg.OCSPStaple, staple) {
		t.Fatalf("the authenticator config carries %d octets of stapled status, and the operator configured %d",
			len(serverCfg.OCSPStaple), len(staple))
	}
}

// TestEAPTLSConfigsCarryNoStapleWhenTheCertificateHasNone keeps the "nothing
// configured" state distinguishable at the boundary: it must arrive as nil,
// because that is what makes the authenticator answer a Certificate Status
// Request with no status rather than with something invented here.
func TestEAPTLSConfigsCarryNoStapleWhenTheCertificateHasNone(t *testing.T) {
	ca, caDER, caKey := crlWiringCA(t, "ocsp-wiring-ca")
	leaf, leafDER, leafKey := crlWiringLeaf(t, "ocsp-wiring-node", 77, ca, caKey)
	loadOCSPWiringStore(t, ca, caDER, leaf, leafDER, leafKey, nil)

	serverCfg, err := eapTLSServerConfig(ocspWiringSA(false))
	if err != nil {
		t.Fatalf("eapTLSServerConfig: %v", err)
	}
	if serverCfg.OCSPStaple != nil {
		t.Fatalf("the authenticator config carries %d octets of stapled status for a certificate with none",
			len(serverCfg.OCSPStaple))
	}
}

// TestEAPTLSPeerConfigCarriesTheCertificateStatusRequestLeaf checks the peer
// half, in both polarities: a leaf that arrived as false would silently disable
// the Section 5.4 status rule for every peer that turned it on.
func TestEAPTLSPeerConfigCarriesTheCertificateStatusRequestLeaf(t *testing.T) {
	ca, caDER, caKey := crlWiringCA(t, "ocsp-wiring-ca")
	leaf, leafDER, leafKey := crlWiringLeaf(t, "ocsp-wiring-node", 77, ca, caKey)
	loadOCSPWiringStore(t, ca, caDER, leaf, leafDER, leafKey, nil)

	for _, want := range []bool{true, false} {
		peerCfg := buildPeerTLSConfig(ocspWiringSA(want), slogutil.DiscardLogger())
		if peerCfg == nil {
			t.Fatal("buildPeerTLSConfig returned nil for a peer with a certificate and a CA")
		}
		if peerCfg.CertificateStatusRequest != want {
			t.Fatalf("the peer config carries certificate-status-request %v, and the operator wrote %v",
				peerCfg.CertificateStatusRequest, want)
		}
	}
}
