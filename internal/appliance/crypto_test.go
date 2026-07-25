package appliance

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

func TestEncryptDecryptRoundtrip(t *testing.T) {
	plaintext := []byte("secret data for testing roundtrip")
	passphrase := []byte("test-passphrase-2026")

	encrypted, err := Encrypt(plaintext, passphrase)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	if bytes.Equal(encrypted, plaintext) {
		t.Fatal("encrypted should differ from plaintext")
	}

	decrypted, err := Decrypt(encrypted, passphrase)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("roundtrip mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptWrongPassphrase(t *testing.T) {
	plaintext := []byte("secret data")
	encrypted, err := Encrypt(plaintext, []byte("correct"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	_, err = Decrypt(encrypted, []byte("wrong"))
	if err == nil {
		t.Fatal("expected error with wrong passphrase")
	}
	if !strings.Contains(err.Error(), "decryption failed") {
		t.Errorf("error = %q, want containing 'decryption failed'", err.Error())
	}
}

func TestDecryptCorruptCiphertext(t *testing.T) {
	plaintext := []byte("secret data")
	encrypted, err := Encrypt(plaintext, []byte("pass"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	encrypted[len(encrypted)-1] ^= 0xFF
	_, err = Decrypt(encrypted, []byte("pass"))
	if err == nil {
		t.Fatal("expected error with corrupt ciphertext")
	}
}

func TestDecryptTooShort(t *testing.T) {
	_, err := Decrypt([]byte("short"), []byte("pass"))
	if err == nil {
		t.Fatal("expected error with short input")
	}
	if !strings.Contains(err.Error(), "too short") {
		t.Errorf("error = %q, want containing 'too short'", err.Error())
	}
}

func TestWriteReadSecretPlaintext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	plaintext := []byte("plain-secret")

	if err := WriteSecret(path, plaintext, nil); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := ReadSecret(path, nil)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if !bytes.Equal(got, plaintext) {
		t.Errorf("got %q, want %q", got, plaintext)
	}
}

func TestWriteReadSecretEncrypted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	plaintext := []byte("encrypted-secret")
	passphrase := []byte("my-pass")

	if err := WriteSecret(path, plaintext, passphrase); err != nil {
		t.Fatalf("write: %v", err)
	}

	raw, err := os.ReadFile(path) //nolint:gosec // test
	if err != nil {
		t.Fatalf("read raw: %v", err)
	}
	if bytes.Equal(raw, plaintext) {
		t.Fatal("file on disk should be encrypted, not plaintext")
	}

	got, err := ReadSecret(path, passphrase)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if !bytes.Equal(got, plaintext) {
		t.Errorf("got %q, want %q", got, plaintext)
	}
}

func TestWriteSecretAtomicRename(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")

	if err := WriteSecret(path, []byte("data"), nil); err != nil {
		t.Fatalf("write: %v", err)
	}

	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should not remain after successful write")
	}
}

func TestZeroBytes(t *testing.T) {
	data := []byte("sensitive")
	ZeroBytes(data)
	for i, b := range data {
		if b != 0 {
			t.Errorf("byte %d = %d, want 0", i, b)
		}
	}
}

func TestIsEncrypted(t *testing.T) {
	dir := t.TempDir()
	name := "test-app"
	secretsDir := SecretsDir(dir, name)
	if err := os.MkdirAll(secretsDir, 0o700); err != nil {
		t.Fatal(err)
	}

	if IsEncrypted(dir, name) {
		t.Error("should not be encrypted without marker")
	}

	if err := WriteEncryptedMarker(dir, name); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	if !IsEncrypted(dir, name) {
		t.Error("should be encrypted with marker")
	}
}

func TestPassphraseEnvVarWarning(t *testing.T) {
	t.Setenv("ZE_APPLIANCE_PASSPHRASE", "test-pass")
	env.ResetCache()

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	pass, source, err := ResolvePassphrase(nil)
	w.Close() //nolint:errcheck // test pipe cleanup
	os.Stderr = oldStderr

	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if string(pass) != "test-pass" {
		t.Errorf("pass = %q, want test-pass", pass)
	}
	if source != "env" {
		t.Errorf("source = %q, want env", source)
	}

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])
	if !strings.Contains(output, "WARNING") {
		t.Errorf("stderr should contain WARNING, got %q", output)
	}
}
