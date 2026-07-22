package ipsec

import (
	"strings"
	"testing"
)

func alwaysPresent(string) bool { return true }
func noCN(string) string        { return "" }

// VALIDATES: AC-8 -- an EAP-TLS peer configured without a ca-certificate is
// rejected at config verification, naming the peer and the requirement.
// PREVENTS: The config being accepted and the failure surfacing only at session
// setup (or, before this, not at all): RFC 5216 Section 5.3 makes path validation
// a MUST, and the trust anchor is the peer's only means of performing it, because
// EAP carries no server hostname to check a certificate against.
func TestValidatePKIRefsRequiresCAForEAPTLS(t *testing.T) {
	cfg := &IPsecConfig{
		Peers: map[string]SiteToSitePeer{
			"branch": {Auth: AuthConfig{Mode: AuthEAPTLS, Certificate: "client"}},
		},
	}

	err := cfg.ValidatePKIRefs(alwaysPresent, alwaysPresent, noCN)

	if err == nil {
		t.Fatal("an eap-tls peer with no ca-certificate was accepted")
	}
	for _, want := range []string{"branch", "eap-tls", "ca-certificate", "RFC 5216"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// VALIDATES: AC-8 -- an EAP-TLS peer WITH a trust anchor passes.
// PREVENTS: The new check rejecting every EAP-TLS config, which would take the
// feature offline rather than harden it.
func TestValidatePKIRefsAcceptsEAPTLSWithCA(t *testing.T) {
	cfg := &IPsecConfig{
		Peers: map[string]SiteToSitePeer{
			"branch": {Auth: AuthConfig{Mode: AuthEAPTLS, Certificate: "client", CACertificate: "corp-ca"}},
		},
	}

	if err := cfg.ValidatePKIRefs(alwaysPresent, alwaysPresent, noCN); err != nil {
		t.Fatalf("a complete eap-tls peer was rejected: %v", err)
	}
}

// VALIDATES: AC-8 -- an EAP-TLS peer naming a CA the PKI store does not hold is
// rejected, not silently accepted.
// PREVENTS: A typo in ca-certificate reaching the runtime, where the CA lookup
// returns nil and the session would previously have run with no trust anchor.
func TestValidatePKIRefsRejectsUnknownCAForEAPTLS(t *testing.T) {
	cfg := &IPsecConfig{
		Peers: map[string]SiteToSitePeer{
			"branch": {Auth: AuthConfig{Mode: AuthEAPTLS, Certificate: "client", CACertificate: "typo-ca"}},
		},
	}
	absent := func(string) bool { return false }

	err := cfg.ValidatePKIRefs(absent, alwaysPresent, noCN)

	if err == nil {
		t.Fatal("an eap-tls peer naming an unknown CA was accepted")
	}
	if !strings.Contains(err.Error(), "typo-ca") {
		t.Errorf("error %q does not name the missing CA", err)
	}
}

// VALIDATES: AC-8 -- auth modes that carry no certificate material are untouched.
// PREVENTS: The widened mode check accidentally demanding a CA from PSK or
// EAP-MSCHAPv2 peers, which have no certificates at all.
func TestValidatePKIRefsIgnoresNonCertificateModes(t *testing.T) {
	for _, mode := range []AuthMode{AuthPreSharedSecret, AuthEAPMSCHAPv2} {
		cfg := &IPsecConfig{
			Peers: map[string]SiteToSitePeer{"branch": {Auth: AuthConfig{Mode: mode}}},
		}
		if err := cfg.ValidatePKIRefs(alwaysPresent, alwaysPresent, noCN); err != nil {
			t.Errorf("auth mode %v was rejected: %v", mode, err)
		}
	}
}
