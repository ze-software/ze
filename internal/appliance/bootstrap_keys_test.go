// Design: docs/architecture/appliance/builder.md -- bootstrap keys are inbound local credentials.

package appliance

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	gossh "golang.org/x/crypto/ssh"

	"github.com/ze-software/ze/pkg/zefs"
)

func applianceBootstrapAuthorizedKey(t *testing.T) []byte {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	key, err := gossh.NewPublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	return gossh.MarshalAuthorizedKey(key)
}

func TestAssembleStoresBootstrapKeysInLocalCredentialNamespace(t *testing.T) {
	dir := assembleTestAppliance(t, "bootstrap-key", nil)
	expected := applianceBootstrapAuthorizedKey(t)
	path := filepath.Join(dir, "bootstrap-key", "secrets", "authorized_keys")
	if err := os.WriteFile(path, expected, 0o600); err != nil {
		t.Fatal(err)
	}
	if code := runAssemble([]string{"--keep", "bootstrap-key"}); code != exitOK {
		t.Fatalf("assemble returned %d", code)
	}
	store, err := zefs.Open(databasePath(dir, "bootstrap-key"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close() //nolint:errcheck // Owned test artifact cleanup.
	actual, err := store.ReadFile("meta/auth/local/authorized-keys")
	if err != nil {
		t.Fatalf("assembled public keys are not in the local credential namespace: %v", err)
	}
	if !bytes.Equal(actual, expected) {
		t.Fatal("assembled public keys differ from supplied complete payload")
	}
	if _, err := store.ReadFile("meta/ssh/authorized-keys"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("inbound keys leaked into the outbound namespace: %v", err)
	}
}

func TestAssembleRefusesMalformedOrRestrictedBootstrapKeys(t *testing.T) {
	for _, content := range [][]byte{
		[]byte("ssh-ed25519 invalid-data\n"),
		append([]byte("restrict "), applianceBootstrapAuthorizedKey(t)...),
	} {
		dir := assembleTestAppliance(t, "refused-key", nil)
		path := filepath.Join(dir, "refused-key", "secrets", "authorized_keys")
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
		if code := runAssemble([]string{"--keep", "refused-key"}); code == exitOK {
			t.Fatal("image assembly accepted a malformed or unenforceable restricted public key")
		}
		if _, err := os.Stat(databasePath(dir, "refused-key")); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("refused key assembly published a database: %v", err)
		}
	}
}
