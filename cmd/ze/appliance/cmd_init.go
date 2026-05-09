// Design: plan/spec-appliance-1-builder.md — appliance init wizard

package appliance

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

	zeweb "codeberg.org/thomas-mangin/ze/internal/component/web"
	"codeberg.org/thomas-mangin/ze/internal/core/env"
)

const (
	updateTokenBytes = 32
	secretsDirPerm   = 0o700
	sshPasswordKey   = "ze.appliance.ssh.password" //nolint:gosec // env var key name
)

var _ = env.MustRegister(env.EnvEntry{
	Key: sshPasswordKey, Type: "string",
	Description: "SSH password for appliance init/passwd (CI only)",
})

func init() {
	cmdInit = runInit
}

func runInit(args []string) int {
	fs := flag.NewFlagSet("appliance init", flag.ContinueOnError)
	configFile := fs.String("config", "", "Read pre-filled JSON config instead of prompting")
	certFile := fs.String("cert", "", "Path to CA-signed certificate (PEM)")
	keyFile := fs.String("key", "", "Path to CA-signed private key (PEM)")
	managedFlag := fs.Bool("managed", false, "Enable managed (fleet) mode")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze appliance init [options] <name>\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  ze appliance init lab\n")
		fmt.Fprintf(os.Stderr, "  ze appliance init --config input.json lab\n")
		fmt.Fprintf(os.Stderr, "  ze appliance init --cert ca.pem --key ca.key lab\n")
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
	appDir := AppliancePath(dir, name)

	if _, err := os.Stat(appDir); err == nil {
		fmt.Fprintf(os.Stderr, "error: appliance %q already exists at %s\n", name, appDir)
		return exitError
	}

	var cfg ApplianceConfig
	if *configFile != "" {
		loaded, err := LoadConfig(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
		cfg = *loaded
		cfg.Identity.Name = name
		if cfg.Identity.Hostname == "" {
			cfg.Identity.Hostname = name
		}
	} else {
		cfg = DefaultConfig(name)
		cfg.Managed = *managedFlag
		if err := promptConfig(&cfg, os.Stdin, os.Stderr); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	if err := os.MkdirAll(TLSDir(dir, name), secretsDirPerm); err != nil {
		fmt.Fprintf(os.Stderr, "error: create directories: %v\n", err)
		return exitError
	}
	if err := os.Chmod(SecretsDir(dir, name), secretsDirPerm); err != nil {
		fmt.Fprintf(os.Stderr, "error: chmod secrets: %v\n", err)
		return exitError
	}

	initFailed := true
	defer func() {
		if initFailed {
			os.RemoveAll(appDir) //nolint:errcheck // best-effort cleanup of partial init
		}
	}()

	password := readPasswordValue(&cfg)
	if password == "" {
		fmt.Fprintf(os.Stderr, "error: password is required\n")
		return exitError
	}

	passphrase := readPassphraseForInit()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: hash password: %v\n", err)
		return exitError
	}

	if err := WriteSecret(
		secretFilePath(dir, name, "password.hash"),
		hashedPassword, passphrase,
	); err != nil {
		fmt.Fprintf(os.Stderr, "error: write password hash: %v\n", err)
		return exitError
	}

	if err := writeTLSSecrets(dir, name, &cfg, *certFile, *keyFile, passphrase); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	token := make([]byte, updateTokenBytes)
	if _, err := io.ReadFull(rand.Reader, token); err != nil {
		fmt.Fprintf(os.Stderr, "error: generate update token: %v\n", err)
		return exitError
	}
	encoded := []byte(base64.StdEncoding.EncodeToString(token))
	if err := WriteSecret(
		secretFilePath(dir, name, "update.token"),
		encoded, passphrase,
	); err != nil {
		fmt.Fprintf(os.Stderr, "error: write update token: %v\n", err)
		return exitError
	}

	if len(cfg.Credentials.SSHAuthorizedKeys) > 0 {
		var sb strings.Builder
		for _, k := range cfg.Credentials.SSHAuthorizedKeys {
			sb.WriteString(k)
			sb.WriteByte('\n')
		}
		authKeysPath := secretFilePath(dir, name, "authorized_keys")
		if err := os.WriteFile(authKeysPath, []byte(sb.String()), 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "error: write authorized_keys: %v\n", err)
			return exitError
		}
	}

	if len(passphrase) > 0 {
		if err := WriteEncryptedMarker(dir, name); err != nil {
			fmt.Fprintf(os.Stderr, "error: write encryption marker: %v\n", err)
			return exitError
		}
	}

	if err := SaveConfig(ConfigPath(dir, name), &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	ZeroBytes(passphrase)
	initFailed = false
	fmt.Printf("initialized appliance %q at %s\n", name, appDir)
	return exitOK
}

func secretFilePath(baseDir, name, file string) string {
	return SecretsDir(baseDir, name) + "/" + file
}

func promptConfig(cfg *ApplianceConfig, r io.Reader, w io.Writer) error {
	if !isTerminal(os.Stdin) {
		return nil
	}
	scanner := bufio.NewScanner(r)

	if v := prompt(scanner, w, fmt.Sprintf("SSH username [%s]: ", cfg.Credentials.Username)); v != "" {
		cfg.Credentials.Username = v
	}
	if v := prompt(scanner, w, fmt.Sprintf("SSH listen address [%s:%s]: ", cfg.SSH.Host, cfg.SSH.Port)); v != "" {
		cfg.SSH.Host = v
	}
	if v := prompt(scanner, w, fmt.Sprintf("TLS certificate hostname (e.g. router.local) [%s]: ", cfg.TLS.CertName)); v != "" {
		cfg.TLS.CertName = v
	}
	if v := prompt(scanner, w, fmt.Sprintf("Hostname [%s]: ", cfg.Identity.Hostname)); v != "" {
		cfg.Identity.Hostname = v
	}

	return scanner.Err()
}

func prompt(scanner *bufio.Scanner, w io.Writer, text string) string {
	if w != nil {
		fmt.Fprint(w, text) //nolint:errcheck // prompt output to stderr
	}
	if scanner.Scan() {
		return scanner.Text()
	}
	return ""
}

func readPasswordValue(_ *ApplianceConfig) string {
	if envPass := env.Get(sshPasswordKey); envPass != "" {
		fmt.Fprintf(os.Stderr, "WARNING: password from environment variable\n")
		return envPass
	}
	if !isTerminal(os.Stdin) {
		return ""
	}
	fmt.Fprint(os.Stderr, "SSH password: ")
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return ""
	}
	return string(pass)
}

func readPassphraseForInit() []byte {
	if !isTerminal(os.Stdin) {
		if envPass := env.Get(passphraseKey); envPass != "" {
			fmt.Fprintf(os.Stderr, "WARNING: passphrase from environment variable (not recommended for production)\n")
			return []byte(envPass)
		}
		return nil
	}
	fmt.Fprint(os.Stderr, "Encryption passphrase (empty for no encryption): ")
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil || len(pass) == 0 {
		return nil
	}
	return pass
}

func writeTLSSecrets(baseDir, name string, cfg *ApplianceConfig, certFile, keyFile string, passphrase []byte) error {
	certPath := TLSDir(baseDir, name) + "/cert.pem"
	keyPath := TLSDir(baseDir, name) + "/key.pem"

	if certFile != "" && keyFile != "" {
		certData, err := os.ReadFile(certFile) //nolint:gosec // user-provided path
		if err != nil {
			return fmt.Errorf("read cert %s: %w", certFile, err)
		}
		keyData, err := os.ReadFile(keyFile) //nolint:gosec // user-provided path
		if err != nil {
			return fmt.Errorf("read key %s: %w", keyFile, err)
		}
		if err := os.WriteFile(certPath, certData, 0o600); err != nil {
			return fmt.Errorf("write cert: %w", err)
		}
		return WriteSecret(keyPath, keyData, passphrase)
	}

	validity := time.Duration(cfg.TLS.ValidityYears) * 365 * 24 * time.Hour
	var extraNames []string
	if cfg.TLS.CertName != "" {
		extraNames = []string{cfg.TLS.CertName}
	}
	listenAddr := cfg.SSH.Host
	certPEM, keyPEM, err := zeweb.GenerateWebCertWithNames(listenAddr, extraNames, validity)
	if err != nil {
		return fmt.Errorf("generate TLS certificate: %w", err)
	}
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		return fmt.Errorf("write cert: %w", err)
	}
	return WriteSecret(keyPath, keyPEM, passphrase)
}

func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}
