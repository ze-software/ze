// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP-TLS configured intermediate certificates
// RFC: rfc/short/rfc5216.md -- Section 5.3, Certificate Validation
//
// VALIDATES: ValidateIntermediateRefs accepts an eap-tls peer whose
// intermediate-certificate names resolve to CA certificates, and refuses a name
// the PKI ca entries lack, an entry that is not a CA, and the list on a peer
// whose mode never reads it. Each refusal names the peer, the entry and the fix.
// PREVENTS: an intermediate list that reads as a working RFC 5216 Section 5.3
// pre-configuration while completing no path.

package ipsec

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"strings"
	"testing"
)

// intermediateLookup answers a PKI ca entry set: corp-sub-ca holds a CA
// certificate and device-leaf holds a certificate that cannot sign another.
func intermediateLookup(name string) *x509.Certificate {
	switch name {
	case "corp-sub-ca":
		return &x509.Certificate{Subject: pkix.Name{CommonName: "Corp Sub CA"}, BasicConstraintsValid: true, IsCA: true}
	case "device-leaf":
		return &x509.Certificate{Subject: pkix.Name{CommonName: "device"}, BasicConstraintsValid: true}
	}
	return nil
}

func intermediatePeer(mode AuthMode, refs ...string) *IPsecConfig {
	return &IPsecConfig{
		Peers: map[string]SiteToSitePeer{
			"branch": {Auth: AuthConfig{
				Mode:                     mode,
				Certificate:              "server",
				CACertificate:            "corp-root",
				IntermediateCertificates: &CertificateRefs{Names: refs},
			}},
		},
	}
}

func requireRefusal(t *testing.T, err error, wants ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("the config was accepted")
	}
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestValidateIntermediateRefsAcceptsCACertificates(t *testing.T) {
	if err := intermediatePeer(AuthEAPTLS, "corp-sub-ca").ValidateIntermediateRefs(intermediateLookup); err != nil {
		t.Fatalf("an eap-tls peer naming a CA intermediate was refused: %v", err)
	}
	if err := intermediatePeer(AuthPreSharedSecret).ValidateIntermediateRefs(intermediateLookup); err != nil {
		t.Fatalf("a peer with no intermediate list was refused: %v", err)
	}
}

func TestValidateIntermediateRefsRefusesAMissingEntry(t *testing.T) {
	err := intermediatePeer(AuthEAPTLS, "corp-sub-ca", "typo-ca").ValidateIntermediateRefs(intermediateLookup)
	requireRefusal(t, err, "branch", `"typo-ca"`, "not found", "pki ca typo-ca")
}

func TestValidateIntermediateRefsRefusesANonCACertificate(t *testing.T) {
	err := intermediatePeer(AuthEAPTLS, "device-leaf").ValidateIntermediateRefs(intermediateLookup)
	requireRefusal(t, err, "branch", `"device-leaf"`, "not a CA certificate")
}

func TestValidateIntermediateRefsRefusesTheListOutsideEAPTLS(t *testing.T) {
	for _, mode := range []AuthMode{AuthPreSharedSecret, AuthX509, AuthEAPMSCHAPv2} {
		err := intermediatePeer(mode, "corp-sub-ca").ValidateIntermediateRefs(intermediateLookup)
		requireRefusal(t, err, "branch", "read only in eap-tls mode", mode.String())
	}
}
