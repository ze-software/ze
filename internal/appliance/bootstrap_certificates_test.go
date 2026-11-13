// Design: docs/architecture/appliance/builder.md -- bootstrap keys carry no certificate authority.
// Related: bootstrap_keys_test.go -- actual named assembly and publication fixture.

package appliance

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io/fs"
	"os"
	"testing"

	gossh "golang.org/x/crypto/ssh"
)

func applianceBootstrapCertificate(t *testing.T, expired bool, critical map[string]string) []byte {
	t.Helper()
	key, _, _, _, err := gossh.ParseAuthorizedKey(applianceBootstrapAuthorizedKey(t))
	if err != nil {
		t.Fatal(err)
	}
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := gossh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	certificate := &gossh.Certificate{
		Key: key, Serial: 1, CertType: gossh.UserCert,
		KeyId: "appliance-certificate-test", ValidPrincipals: []string{"admin"},
		ValidBefore: gossh.CertTimeInfinity,
		Permissions: gossh.Permissions{CriticalOptions: critical},
	}
	if expired {
		certificate.ValidBefore = 1
	}
	if err := certificate.SignCert(rand.Reader, authority); err != nil {
		t.Fatal(err)
	}
	return gossh.MarshalAuthorizedKey(certificate)
}

// TestAssembleRefusesBootstrapCertificates requires complete refusal before a
// new credential database is published, including when a valid plain key came
// first. Assembly must not silently discard a certificate's restrictions.
func TestAssembleRefusesBootstrapCertificates(t *testing.T) {
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
			dir := assembleTestAppliance(t, "certificate-key", nil)
			content := append(applianceBootstrapAuthorizedKey(t), applianceBootstrapCertificate(t, tc.expired, tc.critical)...)
			if err := os.WriteFile(secretFilePath(dir, "certificate-key", "authorized_keys"), content, 0o600); err != nil {
				t.Fatal(err)
			}
			if code := runAssemble([]string{"--keep", "certificate-key"}); code == exitOK {
				t.Error("image assembly accepted a certificate as an unrestricted recovery key")
			}
			if _, err := os.Stat(databasePath(dir, "certificate-key")); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("refused certificate assembly published a database: %v", err)
			}
		})
	}
	t.Run("plain-key-control", func(t *testing.T) {
		dir := assembleTestAppliance(t, "plain-key-control", nil)
		if err := os.WriteFile(secretFilePath(dir, "plain-key-control", "authorized_keys"), applianceBootstrapAuthorizedKey(t), 0o600); err != nil {
			t.Fatal(err)
		}
		if code := runAssemble([]string{"--keep", "plain-key-control"}); code != exitOK {
			t.Fatalf("useful plain key assembly was refused: exit %d", code)
		}
	})
}
