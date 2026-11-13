//go:build ze_ssh

// Design: docs/guide/authentication.md -- bootstrap recovery accepts plain public keys.
// Related: bootstrap_key_login_test.go -- actual accepted-identity SSH fixture.
// Upstream: golang.org/x/crypto v0.58.0, ssh/certs.go, Certificate and NewCertSigner;
// https://github.com/golang/crypto/blob/v0.58.0/ssh/certs.go.

package hub

import (
	"crypto/rand"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

func bootstrapCertificate(t *testing.T, expired bool, critical map[string]string) gossh.Signer {
	t.Helper()
	subject, authority := bootstrapSigner(t), bootstrapSigner(t)
	certificate := &gossh.Certificate{
		Key: subject.PublicKey(), Serial: 1, CertType: gossh.UserCert,
		KeyId: "bootstrap-certificate-test", ValidPrincipals: []string{"admin"},
		ValidBefore: gossh.CertTimeInfinity,
		Permissions: gossh.Permissions{CriticalOptions: critical},
	}
	if expired {
		certificate.ValidBefore = 1
	}
	if err := certificate.SignCert(rand.Reader, authority); err != nil {
		t.Fatal(err)
	}
	signer, err := gossh.NewCertSigner(certificate, subject)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

// TestBootstrapCertificatesCannotBecomeRecoveryKeys reaches the real SSH
// listener with a signed certificate whose restrictions the plain-key login
// mechanism cannot enforce. No certificate, including an otherwise valid one,
// may become an unrestricted local recovery credential.
func TestBootstrapCertificatesCannotBecomeRecoveryKeys(t *testing.T) {
	for _, tc := range []struct {
		name     string
		expired  bool
		critical map[string]string
	}{
		{name: "expired", expired: true},
		{name: "force-command", critical: map[string]string{"force-command": "show version"}},
		{name: "source-address", critical: map[string]string{"source-address": "192.0.2.0/24"}},
		{name: "otherwise-valid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			signer := bootstrapCertificate(t, tc.expired, tc.critical)
			store := bootstrapStore(t, string(gossh.MarshalAuthorizedKey(signer.PublicKey())), "", false)
			address, host := bootstrapSSHServer(t, store, nil)
			if err := bootstrapSSHDial(address, "admin", host, gossh.PublicKeys(signer)); err == nil {
				t.Error("SSH certificate authenticated as unrestricted recovery authority")
			}
			users, err := usersFromStore(store)
			if err == nil {
				t.Error("canonical bootstrap reader accepted an unsupported certificate")
			}
			if users != nil {
				t.Error("refused certificate exposed a partial recovery identity")
			}
		})
	}
	t.Run("plain-key-control", func(t *testing.T) {
		signer := bootstrapSigner(t)
		store := bootstrapStore(t, string(gossh.MarshalAuthorizedKey(signer.PublicKey())), "", false)
		address, host := bootstrapSSHServer(t, store, nil)
		if err := bootstrapSSHDial(address, "admin", host, gossh.PublicKeys(signer)); err != nil {
			t.Fatalf("useful plain public key was refused: %v", err)
		}
	})
}
