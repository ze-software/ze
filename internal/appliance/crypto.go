// Design: plan/learned/675-appliance-1-builder.md — Argon2id KDF + ChaCha20-Poly1305 AEAD encryption

package appliance

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errCiphertextTooShort                 = errors.New("ciphertext too short")
	errDecryptionFailed                   = errors.New("decryption failed")
	errNoPassphraseSourceAvailableNoAgent = errors.New("no passphrase source available (no agent, no env var, no prompt)")
)

const (
	saltSize        = 16
	argonTime       = 3
	argonMemory     = 64 * 1024 // 64 MiB
	argonThreads    = 4
	argonKeyLen     = 32
	encryptedMarker = ".encrypted"
	passphraseKey   = "ze.appliance.passphrase" //nolint:gosec // env var key name, not a credential
)

var _ = env.MustRegister(env.EnvEntry{
	Key: passphraseKey, Type: "string",
	Description: "Appliance encryption passphrase (CI only, not recommended for production)",
	Secret:      true,
})

func DeriveKey(passphrase, salt []byte) []byte {
	return argon2.IDKey(passphrase, salt, argonTime, argonMemory, argonThreads, argonKeyLen)
}

func Encrypt(plaintext, passphrase []byte) ([]byte, error) {
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}

	key := DeriveKey(passphrase, salt)
	defer ZeroBytes(key)

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("create AEAD: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	ciphertext := aead.Seal(nil, nonce, plaintext, nil)

	// envelope: [salt][nonce][ciphertext+tag]
	envelope := make([]byte, 0, saltSize+len(nonce)+len(ciphertext))
	envelope = append(envelope, salt...)
	envelope = append(envelope, nonce...)
	envelope = append(envelope, ciphertext...)

	return envelope, nil
}

func Decrypt(envelope, passphrase []byte) ([]byte, error) {
	nonceSize := chacha20poly1305.NonceSizeX
	overhead := chacha20poly1305.Overhead
	minSize := saltSize + nonceSize + overhead

	if len(envelope) < minSize {
		return nil, errCiphertextTooShort
	}

	salt := envelope[:saltSize]
	nonce := envelope[saltSize : saltSize+nonceSize]
	ciphertext := envelope[saltSize+nonceSize:]

	key := DeriveKey(passphrase, salt)
	defer ZeroBytes(key)

	aead, err := chacha20poly1305.NewX(key)
	if err != nil {
		return nil, fmt.Errorf("create AEAD: %w", err)
	}

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errDecryptionFailed
	}

	return plaintext, nil
}

func IsEncrypted(baseDir, name string) bool {
	marker := filepath.Join(SecretsDir(baseDir, name), encryptedMarker)
	_, err := os.Stat(marker)
	return err == nil
}

func WriteEncryptedMarker(baseDir, name string) error {
	marker := filepath.Join(SecretsDir(baseDir, name), encryptedMarker)
	return os.WriteFile(marker, nil, 0o600)
}

func ReadSecret(path string, passphrase []byte) ([]byte, error) {
	data, err := os.ReadFile(path) //nolint:gosec // user-provided path
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(passphrase) == 0 {
		return data, nil
	}
	return Decrypt(data, passphrase)
}

func WriteSecret(path string, plaintext, passphrase []byte) error {
	var data []byte
	var err error

	if len(passphrase) > 0 {
		data, err = Encrypt(plaintext, passphrase)
		if err != nil {
			return fmt.Errorf("encrypt: %w", err)
		}
	} else {
		data = plaintext
	}

	var tb textbuf.Buffer
	tmpPath := tb.Str(path).Str(".tmp").String()
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup
		return fmt.Errorf("rename %s: %w", path, err)
	}
	return nil
}

func ZeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

const agentSocketName = "ze-appliance-agent.sock"

func AgentSocketPath() string {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, agentSocketName)
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("ze-appliance-agent-%d.sock", os.Getuid()))
}

func ResolvePassphrase(prompt func() ([]byte, error)) ([]byte, string, error) {
	key, err := requestKeyFromAgent()
	if err == nil {
		return key, "agent", nil
	}

	if envPass := env.Get(passphraseKey); envPass != "" {
		fmt.Fprintf(os.Stderr, "WARNING: passphrase from environment variable (not recommended for production)\n")
		return []byte(envPass), "env", nil
	}

	if prompt == nil {
		return nil, "", errNoPassphraseSourceAvailableNoAgent
	}
	pass, err := prompt()
	if err != nil {
		return nil, "", fmt.Errorf("passphrase prompt: %w", err)
	}
	return pass, "prompt", nil
}

func requestKeyFromAgent() ([]byte, error) {
	sockPath := AgentSocketPath()
	var d net.Dialer
	conn, err := d.Dial("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("connect to agent: %w", err)
	}
	defer conn.Close() //nolint:errcheck // best-effort close

	key := make([]byte, argonKeyLen)
	if _, err := io.ReadFull(conn, key); err != nil {
		return nil, fmt.Errorf("read key from agent: %w", err)
	}
	return key, nil
}
