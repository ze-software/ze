// Design: docs/architecture/appliance/builder.md -- appliance init wizard

package appliance

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/selfcert"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	updateTokenBytes = 32
	secretsDirPerm   = 0o700
	sshPasswordKey   = "ze.appliance.ssh.password" //nolint:gosec // env var key name
)

var _ = env.MustRegister(env.EnvEntry{
	Key: sshPasswordKey, Type: envTypeString,
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
	batchFile := fs.String("batch", "", "Batch init from JSON manifest (array of entries)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze appliance init [options] <name>\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  ze appliance init lab\n")
		fmt.Fprintf(os.Stderr, "  ze appliance init --config input.json lab\n")
		fmt.Fprintf(os.Stderr, "  ze appliance init --cert ca.pem --key ca.key lab\n")
		fmt.Fprintf(os.Stderr, "  ze appliance init --batch manifest.json\n")
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if *batchFile != "" {
		return runBatchInit(*batchFile)
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
	appDir := AppliancePath(dir, name)

	if _, err := os.Stat(appDir); err == nil {
		fmt.Fprintf(os.Stderr, "error: appliance %q already exists at %s\n", name, appDir)
		return exitError
	}

	pw, owned := openPromptWriter()
	if owned {
		defer pw.Close() //nolint:errcheck // prompt tty close
	}

	var cfg applianceConfig
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
		if err := promptConfig(&cfg, os.Stdin, pw); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	if err := os.MkdirAll(tLSDir(dir, name), secretsDirPerm); err != nil {
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

	password := readPasswordValue(&cfg, pw)
	if password == "" {
		fmt.Fprintf(os.Stderr, "error: password is required\n")
		return exitError
	}

	passphrase := readPassphraseForInit(pw)

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
		var sb textbuf.Buffer
		for _, k := range cfg.Credentials.SSHAuthorizedKeys {
			sb.Str(k).Byte('\n')
		}
		authKeysPath := secretFilePath(dir, name, "authorized_keys")
		if err := os.WriteFile(authKeysPath, sb.Bytes(), 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "error: write authorized_keys: %v\n", err)
			return exitError
		}
	}

	if len(passphrase) > 0 {
		if err := writeEncryptedMarker(dir, name); err != nil {
			fmt.Fprintf(os.Stderr, "error: write encryption marker: %v\n", err)
			return exitError
		}
	}

	if err := saveConfig(ConfigPath(dir, name), &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}

	ZeroBytes(passphrase)
	initFailed = false
	fmt.Printf("initialized appliance %q at %s\n", name, appDir)
	return exitOK
}

func secretFilePath(baseDir, name, file string) string {
	var tb textbuf.Buffer
	return tb.Str(SecretsDir(baseDir, name)).Byte('/').Str(file).String()
}

func promptConfig(cfg *applianceConfig, r io.Reader, w io.Writer) error {
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
		fmt.Fprint(w, text) //nolint:errcheck // interactive prompt
	}
	if scanner.Scan() {
		return scanner.Text()
	}
	return ""
}

func readPasswordValue(_ *applianceConfig, pw io.Writer) string {
	if envPass := env.Get(sshPasswordKey); envPass != "" {
		fmt.Fprintf(os.Stderr, "WARNING: password from environment variable\n")
		return envPass
	}
	if !isTerminal(os.Stdin) {
		return ""
	}
	fmt.Fprint(pw, "SSH password: ") //nolint:errcheck // interactive prompt
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(pw) //nolint:errcheck // newline after hidden input
	if err != nil {
		return ""
	}
	return string(pass)
}

func readPassphraseForInit(pw io.Writer) []byte {
	if !isTerminal(os.Stdin) {
		if envPass := env.Get(passphraseKey); envPass != "" {
			fmt.Fprintf(os.Stderr, "WARNING: passphrase from environment variable (not recommended for production)\n")
			return []byte(envPass)
		}
		return nil
	}
	fmt.Fprint(pw, "Encryption passphrase (empty for no encryption): ") //nolint:errcheck // interactive prompt
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(pw) //nolint:errcheck // newline after hidden input
	if err != nil || len(pass) == 0 {
		return nil
	}
	return pass
}

// writeTLSSecrets resolves the appliance's TLS material and stores it. It is
// the one write path for cert.pem and key.pem: initialization and
// `ze appliance replace-cert` both reach the files through here, so both get
// the same validation and the same restore on failure (cmd_cert.go).
//
// Operator-supplied material is validated before either file is touched. The
// key buffer is zeroed once the write is done, as the passphrase already is.
func writeTLSSecrets(baseDir, name string, cfg *applianceConfig, certFile, keyFile string, passphrase []byte) error {
	var tb textbuf.Buffer
	certPath := tb.Str(tLSDir(baseDir, name)).Str("/cert.pem").String()
	keyPath := tb.Reset().Str(tLSDir(baseDir, name)).Str("/key.pem").String()

	if err := checkTLSFlags(certFile, keyFile); err != nil {
		return err
	}

	if certFile != "" {
		certData, err := cliio.ReadFile(certFile) // "-" reads stdin
		if err != nil {
			return fmt.Errorf("read cert %s: %w", certFile, err)
		}
		keyData, err := cliio.ReadFile(keyFile) // "-" reads stdin (once; a second "-" fails closed)
		if err != nil {
			return fmt.Errorf("read key %s: %w", keyFile, err)
		}
		defer ZeroBytes(keyData)
		if err := validateTLSPair(certData, keyData, certFile, keyFile); err != nil {
			return err
		}
		return writeTLSPair(certPath, keyPath, certData, keyData, passphrase)
	}

	validity := time.Duration(cfg.TLS.ValidityYears) * 365 * 24 * time.Hour
	var extraNames []string
	if cfg.TLS.CertName != "" {
		extraNames = []string{cfg.TLS.CertName}
	}
	listenAddr := cfg.SSH.Host
	certPEM, keyPEM, err := selfcert.GenerateWebCertWithNames(listenAddr, extraNames, validity)
	if err != nil {
		return fmt.Errorf("generate TLS certificate: %w", err)
	}
	defer ZeroBytes(keyPEM)
	if err := validateTLSPair(certPEM, keyPEM, "the generated certificate", "the generated key"); err != nil {
		return err
	}
	return writeTLSPair(certPath, keyPath, certPEM, keyPEM, passphrase)
}

func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// openPromptWriter opens /dev/tty for prompt output, bypassing any
// stdout/stderr redirection. Falls back to stderr when unavailable.
func openPromptWriter() (io.WriteCloser, bool) {
	f, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return os.Stderr, false
	}
	return f, true
}

type batchEntry struct {
	Name              string   `json:"name"`
	Hostname          string   `json:"hostname"`
	Password          string   `json:"password"` //nolint:gosec // manifest input, not stored
	DeviceAddress     string   `json:"device.address"`
	SSHAuthorizedKeys []string `json:"ssh-authorized-keys"`
}

func runBatchInit(manifestPath string) int {
	data, err := cliio.ReadFile(manifestPath) // "-" reads stdin
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read manifest: %v\n", err)
		return exitError
	}

	var entries []batchEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		fmt.Fprintf(os.Stderr, "error: parse manifest: %v\n", err)
		return exitError
	}

	if len(entries) == 0 {
		fmt.Fprintf(os.Stderr, "error: manifest is empty\n")
		return exitError
	}

	dir := getBaseDir()

	pw, owned := openPromptWriter()
	if owned {
		defer pw.Close() //nolint:errcheck // prompt tty close
	}
	passphrase := readPassphraseForInit(pw)

	succeeded, failed := 0, 0
	for _, entry := range entries {
		if entry.Name == "" {
			fmt.Fprintf(os.Stderr, "error: manifest entry missing name\n")
			failed++
			continue
		}

		password := entry.Password
		if password == "generate" {
			genBytes := make([]byte, 24)
			if _, randErr := io.ReadFull(rand.Reader, genBytes); randErr != nil {
				fmt.Fprintf(os.Stderr, "error: generate password for %s: %v\n", entry.Name, randErr)
				failed++
				continue
			}
			password = base64.StdEncoding.EncodeToString(genBytes)
			fmt.Printf("%s: %s\n", entry.Name, password)
		}

		if password == "" {
			envPass := env.Get(sshPasswordKey)
			if envPass == "" {
				fmt.Fprintf(os.Stderr, "error: no password for %s (set password field or %s env var)\n", entry.Name, sshPasswordKey)
				failed++
				continue
			}
			password = envPass
		}

		if code := initOneFromBatch(dir, entry, password, passphrase); code != exitOK {
			fmt.Fprintf(os.Stderr, "FAILED: %s\n", entry.Name)
			failed++
		} else {
			succeeded++
		}
	}

	ZeroBytes(passphrase)
	fmt.Printf("%d succeeded, %d failed\n", succeeded, failed)
	if failed > 0 {
		return exitError
	}
	return exitOK
}

func initOneFromBatch(dir string, entry batchEntry, password string, passphrase []byte) int {
	name := entry.Name
	appDir := AppliancePath(dir, name)

	if _, err := os.Stat(appDir); err == nil {
		fmt.Fprintf(os.Stderr, "error: appliance %q already exists\n", name)
		return exitError
	}

	cfg := DefaultConfig(name)
	if entry.Hostname != "" {
		cfg.Identity.Hostname = entry.Hostname
	}
	if entry.DeviceAddress != "" {
		cfg.Device.Address = entry.DeviceAddress
	}
	if len(entry.SSHAuthorizedKeys) > 0 {
		cfg.Credentials.SSHAuthorizedKeys = entry.SSHAuthorizedKeys
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s: %v\n", name, err)
		return exitError
	}

	if err := os.MkdirAll(tLSDir(dir, name), secretsDirPerm); err != nil {
		fmt.Fprintf(os.Stderr, "error: create directories for %s: %v\n", name, err)
		return exitError
	}
	if err := os.Chmod(SecretsDir(dir, name), secretsDirPerm); err != nil {
		fmt.Fprintf(os.Stderr, "error: chmod secrets for %s: %v\n", name, err)
		return exitError
	}

	initFailed := true
	defer func() {
		if initFailed {
			os.RemoveAll(appDir) //nolint:errcheck // best-effort cleanup
		}
	}()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: hash password for %s: %v\n", name, err)
		return exitError
	}

	if err := WriteSecret(secretFilePath(dir, name, "password.hash"), hashedPassword, passphrase); err != nil {
		fmt.Fprintf(os.Stderr, "error: write password hash for %s: %v\n", name, err)
		return exitError
	}

	if err := writeTLSSecrets(dir, name, &cfg, "", "", passphrase); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s: %v\n", name, err)
		return exitError
	}

	token := make([]byte, updateTokenBytes)
	if _, err := io.ReadFull(rand.Reader, token); err != nil {
		fmt.Fprintf(os.Stderr, "error: generate update token for %s: %v\n", name, err)
		return exitError
	}
	encoded := []byte(base64.StdEncoding.EncodeToString(token))
	if err := WriteSecret(secretFilePath(dir, name, "update.token"), encoded, passphrase); err != nil {
		fmt.Fprintf(os.Stderr, "error: write update token for %s: %v\n", name, err)
		return exitError
	}

	if len(cfg.Credentials.SSHAuthorizedKeys) > 0 {
		var sb textbuf.Buffer
		for _, k := range cfg.Credentials.SSHAuthorizedKeys {
			sb.Str(k).Byte('\n')
		}
		authKeysPath := secretFilePath(dir, name, "authorized_keys")
		if err := os.WriteFile(authKeysPath, sb.Bytes(), 0o600); err != nil {
			fmt.Fprintf(os.Stderr, "error: write authorized_keys for %s: %v\n", name, err)
			return exitError
		}
	}

	if len(passphrase) > 0 {
		if err := writeEncryptedMarker(dir, name); err != nil {
			fmt.Fprintf(os.Stderr, "error: write encryption marker for %s: %v\n", name, err)
			return exitError
		}
	}

	if err := saveConfig(ConfigPath(dir, name), &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "error: %s: %v\n", name, err)
		return exitError
	}

	initFailed = false
	fmt.Fprintf(os.Stderr, "initialized appliance %q\n", name)
	return exitOK
}
