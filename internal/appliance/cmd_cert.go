// Design: docs/architecture/appliance/builder.md -- TLS certificate replacement

package appliance

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/internal/core/textbuf"
)

func init() {
	cmdReplaceCert = runReplaceCert
}

func runReplaceCert(args []string) int {
	fs := flag.NewFlagSet("appliance replace-cert", flag.ContinueOnError)
	certFile := fs.String("cert", "", "Path to CA-signed certificate (PEM)")
	keyFile := fs.String("key", "", "Path to CA-signed private key (PEM)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze appliance replace-cert [options] <name>\n\n")
		fmt.Fprintf(os.Stderr, "Without --cert/--key, regenerates a self-signed certificate.\n\n")
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

	name := fs.Arg(0)
	dir := getBaseDir()

	cfg, err := LoadConfig(ConfigPath(dir, name))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	var passphrase []byte
	if IsEncrypted(dir, name) {
		var resolveErr error
		passphrase, _, resolveErr = ResolvePassphrase(nil)
		if resolveErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", resolveErr)
			return exitError
		}
		defer ZeroBytes(passphrase)
	}

	var tb textbuf.Buffer
	certPath := tb.Str(TLSDir(dir, name)).Str("/cert.pem").String()
	keyPath := tb.Reset().Str(TLSDir(dir, name)).Str("/key.pem").String()

	if *certFile != "" && *keyFile != "" {
		certData, readErr := cliio.ReadFile(*certFile) // "-" reads stdin
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "error: read cert: %v\n", readErr)
			return exitError
		}
		keyData, readErr := cliio.ReadFile(*keyFile) // "-" reads stdin (once; a second "-" fails closed)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "error: read key: %v\n", readErr)
			return exitError
		}
		if writeErr := os.WriteFile(certPath, certData, 0o600); writeErr != nil {
			fmt.Fprintf(os.Stderr, "error: write cert: %v\n", writeErr)
			return exitError
		}
		if writeErr := WriteSecret(keyPath, keyData, passphrase); writeErr != nil {
			fmt.Fprintf(os.Stderr, "error: write key: %v\n", writeErr)
			return exitError
		}
		fmt.Printf("certificate replaced for appliance %q (CA-signed)\n", name)
		return exitOK
	}

	validity := time.Duration(cfg.TLS.ValidityYears) * 365 * 24 * time.Hour
	var extraNames []string
	if cfg.TLS.CertName != "" {
		extraNames = []string{cfg.TLS.CertName}
	}
	certPEM, keyPEM, genErr := selfcert.GenerateWebCertWithNames(cfg.SSH.Host, extraNames, validity)
	if genErr != nil {
		fmt.Fprintf(os.Stderr, "error: generate certificate: %v\n", genErr)
		return exitError
	}
	if writeErr := os.WriteFile(certPath, certPEM, 0o600); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: write cert: %v\n", writeErr)
		return exitError
	}
	if writeErr := WriteSecret(keyPath, keyPEM, passphrase); writeErr != nil {
		fmt.Fprintf(os.Stderr, "error: write key: %v\n", writeErr)
		return exitError
	}

	fmt.Printf("certificate regenerated for appliance %q (self-signed, %d years)\n", name, cfg.TLS.ValidityYears)
	return exitOK
}
