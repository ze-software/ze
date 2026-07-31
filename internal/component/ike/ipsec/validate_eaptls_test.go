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

// VALIDATES: AC-8 -- a pre-shared-secret peer carries no certificate material and
// is untouched. An eap-mschapv2 peer is not in that class, because RFC 7296
// Section 2.16 makes the responder sign its AUTH with a public key.
// PREVENTS: The widened mode check demanding a CA from a PSK peer, and the older
// belief that eap-mschapv2 needs no certificate at all. That belief was the
// violation: the responder fell back to a pre-shared-key AUTH, which is not a
// public-key signature. See engine.TestEapAuthResponderRefusesWithoutCertificate.
func TestValidatePKIRefsIgnoresNonCertificateModes(t *testing.T) {
	cases := []struct {
		mode      AuthMode
		wantError bool
	}{
		// No EAP exchange, so RFC 7296 Section 2.16 does not apply.
		{AuthPreSharedSecret, false},
		// An EAP exchange, so the responder needs a public-key credential.
		{AuthEAPMSCHAPv2, true},
	}
	for _, c := range cases {
		cfg := &IPsecConfig{
			Peers: map[string]SiteToSitePeer{"branch": {Auth: AuthConfig{Mode: c.mode}}},
		}
		err := cfg.ValidatePKIRefs(alwaysPresent, alwaysPresent, noCN)
		if c.wantError {
			if err == nil {
				t.Errorf("auth mode %v with no certificate was accepted", c.mode)
				continue
			}
			if !strings.Contains(err.Error(), "RFC 7296 Section 2.16") {
				t.Errorf("auth mode %v error %q does not cite the requirement", c.mode, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("auth mode %v was rejected: %v", c.mode, err)
		}
	}
}

// VALIDATES: the local-id/CN equality rule stays X.509-only.
// PREVENTS: Widening the loop to EAP-TLS silently importing an X.509 rule that
// has no basis for it. The EAP-TLS IKE AUTH is MSK-derived, not signed by this
// certificate, and local-id is the EAP identity: an NAI such as
// "alice@example.com" against a device certificate CN of "alice" is the ordinary
// deployment, and rejecting it would break working configs on upgrade.
func TestValidatePKIRefsAllowsEAPTLSIdentityUnlikeCertCN(t *testing.T) {
	cfg := &IPsecConfig{
		Peers: map[string]SiteToSitePeer{
			"branch": {Auth: AuthConfig{
				Mode:          AuthEAPTLS,
				Certificate:   "alice-cert",
				CACertificate: "corp-ca",
				LocalID:       "alice@example.com",
			}},
		},
	}
	certCN := func(string) string { return "alice" }

	if err := cfg.ValidatePKIRefs(alwaysPresent, alwaysPresent, certCN); err != nil {
		t.Fatalf("an eap-tls peer whose local-id differs from the cert CN was rejected: %v", err)
	}
}

// VALIDATES: the same rule still applies to X.509 peers.
// PREVENTS: Scoping the check so tightly that it stops running where it belongs.
func TestValidatePKIRefsStillChecksCNForX509(t *testing.T) {
	cfg := &IPsecConfig{
		Peers: map[string]SiteToSitePeer{
			// The trust anchor is present, so the peer reaches the CN rule this
			// test is about rather than stopping at the anchor requirement.
			"branch": {Auth: AuthConfig{
				Mode:          AuthX509,
				Certificate:   "alice-cert",
				CACertificate: "corp-ca",
				LocalID:       "alice@example.com",
			}},
		},
	}
	certCN := func(string) string { return "alice" }

	err := cfg.ValidatePKIRefs(alwaysPresent, alwaysPresent, certCN)
	if err == nil {
		t.Fatal("an x509 peer whose local-id differs from the cert CN was accepted")
	}
	if !strings.Contains(err.Error(), "does not match certificate CN") {
		t.Errorf("error %q is not the CN mismatch", err)
	}
}
