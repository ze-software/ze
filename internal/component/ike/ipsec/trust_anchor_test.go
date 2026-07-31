package ipsec

import (
	"strings"
	"testing"
)

// VALIDATES: every mode that authenticates a remote certificate needs a trust
// anchor, and the config is refused without one.
// PREVENTS: a certificate with no trust anchor passing for authentication. With no
// ca-certificate the runtime accepted any self-signed certificate that carried a
// valid signature, so an attacker answering on UDP 500 authenticated as the peer.
// For eap-mschapv2 Ze then sent the user's challenge and response to that attacker.
//
// RFC 7296 Section 2.16 requires EAP to be used with a public-key-signature-based
// authentication of the responder to the initiator. A signature the initiator cannot
// chain to a trust anchor authenticates nobody.
func TestTanCertificateModesNeedATrustAnchor(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode AuthMode
	}{
		{"eap-mschapv2", AuthEAPMSCHAPv2},
		{"eap-tls", AuthEAPTLS},
		{"x509", AuthX509},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Negative. A certificate with no trust anchor is refused.
			bare := &IPsecConfig{
				Peers: map[string]SiteToSitePeer{
					"branch": {Auth: AuthConfig{Mode: tc.mode, Certificate: "device"}},
				},
			}
			err := bare.ValidatePKIRefs(alwaysPresent, alwaysPresent, noCN)
			if err == nil {
				t.Fatalf("a %s peer with no ca-certificate was accepted", tc.name)
			}
			for _, want := range []string{"branch", "ca-certificate"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal %q does not name %q", err, want)
				}
			}

			// Positive. The same peer with a trust anchor passes, so the check
			// hardens the mode rather than taking it offline.
			anchored := &IPsecConfig{
				Peers: map[string]SiteToSitePeer{
					"branch": {Auth: AuthConfig{Mode: tc.mode, Certificate: "device", CACertificate: "corp-ca"}},
				},
			}
			if err := anchored.ValidatePKIRefs(alwaysPresent, alwaysPresent, noCN); err != nil {
				t.Errorf("a complete %s peer was rejected: %v", tc.name, err)
			}
		})
	}
}

// VALIDATES: a pre-shared-secret peer needs neither a certificate nor a trust
// anchor.
// PREVENTS: the widened check demanding certificate material from a mode that runs
// no public-key exchange, which would refuse every working PSK config.
func TestTanPreSharedSecretNeedsNoCertificateMaterial(t *testing.T) {
	cfg := &IPsecConfig{
		Peers: map[string]SiteToSitePeer{
			"branch": {Auth: AuthConfig{Mode: AuthPreSharedSecret, PSK: "secret"}},
		},
	}
	if err := cfg.ValidatePKIRefs(alwaysPresent, alwaysPresent, noCN); err != nil {
		t.Fatalf("a pre-shared-secret peer was rejected: %v", err)
	}
}
