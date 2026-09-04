// Design: docs/architecture/appliance/builder.md -- TLS certificate replacement

package appliance

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"
)

// errTLSFlagPair refuses half of a replacement. Accepting one flag alone would
// silently fall through to issuing a new certificate and destroy the material
// the operator meant to keep.
var errTLSFlagPair = errors.New("--cert and --key must be given together")

// checkTLSFlags refuses half a replacement. Both commands call it as soon as
// they have parsed their flags: a later refusal would come after the passphrase
// prompt, and the operator would read a passphrase error for a flag mistake.
func checkTLSFlags(certFile, keyFile string) error {
	if (certFile == "") != (keyFile == "") {
		return errTLSFlagPair
	}
	return nil
}

func init() {
	cmdReplaceCert = runReplaceCert
}

func runReplaceCert(args []string) int {
	fs := flag.NewFlagSet("appliance replace-cert", flag.ContinueOnError)
	certFile := fs.String("cert", "", "Path to CA-signed certificate (PEM)")
	keyFile := fs.String("key", "", "Path to CA-signed private key (PEM)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze appliance replace-cert [options] <name>\n\n")
		fmt.Fprintf(os.Stderr, "Without --cert/--key, issues a new certificate from the appliance CA.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}
	if fs.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "error: requires <name>\n")
		fs.Usage()
		return exitError
	}

	if err := checkTLSFlags(*certFile, *keyFile); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	name := fs.Arg(0)
	dir := getBaseDir()

	cfg, err := LoadConfig(ConfigPath(dir, name))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	var passphrase []byte
	if isEncrypted(dir, name) {
		var resolveErr error
		passphrase, _, resolveErr = ResolvePassphrase(nil)
		if resolveErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", resolveErr)
			return exitError
		}
		defer ZeroBytes(passphrase)
	}

	// writeTLSSecrets is the appliance's one TLS write path: it validates the
	// material before it touches either file, and restores the previous
	// certificate when the key write fails. Initialization uses the same call.
	if writeErr := writeTLSSecrets(dir, name, cfg, *certFile, *keyFile, passphrase); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", writeErr)
		return exitError
	}

	if *certFile != "" {
		fmt.Printf("certificate replaced for appliance %q (CA-signed)\n", name)
		return exitOK
	}
	fmt.Printf("certificate reissued for appliance %q (appliance CA, %d years)\n", name, cfg.TLS.ValidityYears)
	return exitOK
}

// validateTLSPair refuses material the appliance could not serve. The pair must
// parse and belong together, which is the check selfcert.NewTLSConfig makes at
// boot, so material that passes here is material the web listener will accept.
// An expired certificate is refused as well: replacing a certificate with one
// that is already past its not-after date leaves the listener unusable.
//
// A certificate whose not-before is in the future is ACCEPTED. Staging a
// renewal ahead of its start date is a supported workflow, and the material is
// copied into an image that boots later.
//
// certSource and keySource name the material in the error, so the operator
// reads which of the two was refused. Both entry points pass the path the
// operator typed; the issuing path passes a phrase, because no file the
// operator can open holds the material.
func validateTLSPair(certPEM, keyPEM []byte, certSource, keySource string) error {
	if block, _ := pem.Decode(certPEM); block == nil {
		return fmt.Errorf("%s holds no PEM data", certSource)
	}
	if block, _ := pem.Decode(keyPEM); block == nil {
		return fmt.Errorf("%s holds no PEM data", keySource)
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("%s and %s are not a pair: %w", certSource, keySource, err)
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return fmt.Errorf("%s does not parse: %w", certSource, err)
	}
	if time.Now().After(leaf.NotAfter) {
		return fmt.Errorf("%s expired on %s (valid from %s)",
			certSource,
			leaf.NotAfter.Format(time.RFC3339),
			leaf.NotBefore.Format(time.RFC3339))
	}
	return nil
}

// writeTLSPair replaces cert.pem and key.pem together.
//
// Both halves go through WriteSecret, which writes a temp file and renames it,
// so neither file is ever left truncated. WriteSecret with no passphrase writes
// the bytes unchanged, which is what a certificate needs: it is public material
// the assemble step and the booting daemon read back as PEM.
//
// The certificate is written first and its previous content is held until the
// key write succeeds. A failed key write puts the certificate back, so the
// appliance keeps the pair it was already serving. The key needs no backup: the
// rename is the only step that replaces it, so a failure leaves it untouched.
func writeTLSPair(certPath, keyPath string, certPEM, keyPEM, passphrase []byte) error {
	previousCert, hadCert, err := readForRestore(certPath)
	if err != nil {
		return err
	}

	if writeErr := WriteSecret(certPath, certPEM, nil); writeErr != nil {
		return fmt.Errorf("write certificate %s: %w", certPath, writeErr)
	}

	if writeErr := WriteSecret(keyPath, keyPEM, passphrase); writeErr != nil {
		failure := fmt.Errorf("write key %s: %w", keyPath, writeErr)
		if restoreErr := restoreTLSFile(certPath, previousCert, hadCert); restoreErr != nil {
			return fmt.Errorf("%w; the previous certificate was not restored: %w", failure, restoreErr)
		}
		return fmt.Errorf("%w; the previous certificate and key are unchanged", failure)
	}
	return nil
}

// readForRestore returns the file's current bytes, and whether it was there at
// all. A missing file is not an error: initialization writes into an empty
// directory, and the restore for that case is a delete.
func readForRestore(path string) (previous []byte, existed bool, err error) {
	data, readErr := os.ReadFile(path) //nolint:gosec // path built from the appliance directory
	if readErr == nil {
		return data, true, nil
	}
	if errors.Is(readErr, os.ErrNotExist) {
		return nil, false, nil
	}
	return nil, false, fmt.Errorf("read %s: %w", path, readErr)
}

// restoreTLSFile puts back what readForRestore captured. The backup holds the
// file's bytes exactly as they were on disk, already encrypted when the store
// is encrypted, so it is written back with no passphrase.
func restoreTLSFile(path string, previous []byte, existed bool) error {
	if !existed {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
		return nil
	}
	return WriteSecret(path, previous, nil)
}
