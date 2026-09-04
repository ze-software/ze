// Design: docs/architecture/appliance/builder.md -- passphrase change (rekey)

package appliance

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"golang.org/x/term"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const newPassphraseKey = "ze.appliance.new.passphrase" //nolint:gosec // env var key name

var _ = env.MustRegister(env.EnvEntry{
	Key: newPassphraseKey, Type: envTypeString,
	Description: "New encryption passphrase for rekey (CI only)",
})

func init() {
	cmdRekey = runRekey
}

func runRekey(args []string) int {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: ze appliance rekey <name>\n")
		return exitError
	}
	name := args[0]
	dir := getBaseDir()

	if _, err := os.Stat(ConfigPath(dir, name)); err != nil {
		fmt.Fprintf(os.Stderr, "error: appliance %q not found\n", name)
		return exitError
	}

	var oldPassphrase []byte
	if isEncrypted(dir, name) {
		var err error
		oldPassphrase, _, err = ResolvePassphrase(nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
		defer ZeroBytes(oldPassphrase)
	}

	newPassphrase := readNewPassphrase()
	defer ZeroBytes(newPassphrase)

	secretFiles := []string{"password.hash", "update.token"}

	// Every private key under tls/ is re-encrypted. A key left out survives the
	// rekey under the OLD passphrase, and the command that reads it next fails
	// with a decryption error the operator cannot place. ca-key.pem is the
	// appliance's certificate authority (ca.go); it is absent on an appliance
	// initialized with operator-supplied material, which issues nothing.
	keyPaths := []string{
		filepath.Join(tLSDir(dir, name), "key.pem"),
		filepath.Join(tLSDir(dir, name), caKeyFileName),
	}

	for _, f := range secretFiles {
		path := secretFilePath(dir, name, f)
		data, err := readSecret(path, oldPassphrase)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: read %s: %v\n", f, err)
			return exitError
		}
		if err := WriteSecret(path, data, newPassphrase); err != nil {
			fmt.Fprintf(os.Stderr, "error: write %s: %v\n", f, err)
			return exitError
		}
		ZeroBytes(data)
	}

	if err := rekeyKeyFiles(keyPaths, oldPassphrase, newPassphrase); err != nil {
		var tb textbuf.Buffer
		tb.Str("error: ").Err(err).Byte('\n')
		tb.StdErr() //nolint:errcheck // the command is returning its error exit
		return exitError
	}

	markerPath := filepath.Join(SecretsDir(dir, name), encryptedMarker)
	if len(newPassphrase) > 0 {
		if writeErr := writeEncryptedMarker(dir, name); writeErr != nil {
			fmt.Fprintf(os.Stderr, "error: write marker: %v\n", writeErr)
			return exitError
		}
	} else {
		os.Remove(markerPath) //nolint:errcheck // may not exist
	}

	fmt.Printf("encryption rekeyed for appliance %q\n", name)
	return exitOK
}

func readNewPassphrase() []byte {
	if envPass := env.Get(newPassphraseKey); envPass != "" {
		fmt.Fprintf(os.Stderr, "WARNING: new passphrase from environment variable\n")
		return []byte(envPass)
	}
	if !isTerminal(os.Stdin) {
		return nil
	}
	fmt.Fprint(os.Stderr, "New encryption passphrase (empty to remove encryption): ") //nolint:errcheck // prompt
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil || len(pass) == 0 {
		return nil
	}
	return pass
}

// rekeyKeyFiles re-encrypts each private key file under the new passphrase. A
// file that is not there is skipped: an appliance initialized with
// operator-supplied material has no certificate authority of its own, and that
// absence is a state rather than a failure.
//
// An empty newPassphrase writes the keys back in plaintext, which is how the
// command removes encryption.
func rekeyKeyFiles(paths []string, oldPassphrase, newPassphrase []byte) error {
	for _, path := range paths {
		if _, statErr := os.Stat(path); errors.Is(statErr, fs.ErrNotExist) {
			continue
		}

		keyData, err := readSecret(path, oldPassphrase)
		if err != nil {
			return fmt.Errorf("read %s: %w", filepath.Base(path), err)
		}
		if err := WriteSecret(path, keyData, newPassphrase); err != nil {
			ZeroBytes(keyData)
			return fmt.Errorf("write %s: %w", filepath.Base(path), err)
		}
		ZeroBytes(keyData)
	}
	return nil
}
