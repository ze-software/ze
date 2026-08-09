// Design: docs/architecture/appliance/builder.md -- passphrase change (rekey)

package appliance

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/term"

	"github.com/ze-software/ze/internal/core/env"
)

const newPassphraseKey = "ze.appliance.new.passphrase" //nolint:gosec // env var key name

var _ = env.MustRegister(env.EnvEntry{
	Key: newPassphraseKey, Type: "string",
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
	if IsEncrypted(dir, name) {
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
	tlsKeyPath := filepath.Join(TLSDir(dir, name), "key.pem")

	for _, f := range secretFiles {
		path := secretFilePath(dir, name, f)
		data, err := ReadSecret(path, oldPassphrase)
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

	keyData, err := ReadSecret(tlsKeyPath, oldPassphrase)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read TLS key: %v\n", err)
		return exitError
	}
	if err := WriteSecret(tlsKeyPath, keyData, newPassphrase); err != nil {
		fmt.Fprintf(os.Stderr, "error: write TLS key: %v\n", err)
		return exitError
	}
	ZeroBytes(keyData)

	markerPath := filepath.Join(SecretsDir(dir, name), encryptedMarker)
	if len(newPassphrase) > 0 {
		if writeErr := WriteEncryptedMarker(dir, name); writeErr != nil {
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
