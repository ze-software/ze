// Design: docs/architecture/system-architecture.md — ze init bootstrap command

// Package init provides the `ze init` command that bootstraps the zefs database
// with SSH credentials before any other ze command can work.
package init

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	sshclient "codeberg.org/thomas-mangin/ze/cmd/ze/internal/ssh/client"
	"codeberg.org/thomas-mangin/ze/internal/component/iface"
	zeweb "codeberg.org/thomas-mangin/ze/internal/component/web"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"

	// Register the netlink backend so iface.LoadBackend("netlink")
	// below resolves. Without this blank import, DiscoverInterfaces
	// returns "no backend loaded" and every detected interface
	// (ethernet, dummy, veth, bridge, tunnel, wireguard) is silently
	// dropped from the initial ze.conf.
	_ "codeberg.org/thomas-mangin/ze/internal/plugins/ifacenetlink"
)

// Key aliases for readability (from zefs key registry).
var (
	keyUsername     = zefs.KeySSHUsername.Pattern
	keyPassword     = zefs.KeySSHPassword.Pattern
	keyHost         = zefs.KeySSHHost.Pattern
	keyPort         = zefs.KeySSHPort.Pattern
	keyIdentityName = zefs.KeyInstanceName.Pattern
	keyManaged      = zefs.KeyInstanceManaged.Pattern
)

const (
	defaultHost = "127.0.0.1"
	defaultPort = "2222"
)

// Run executes ze init from CLI arguments.
// Returns exit code.
func Run(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	managedFlag := fs.Bool("managed", false, "Enable managed (fleet) mode")
	forceFlag := fs.Bool("force", false, "Replace existing database (moves old to .replaced-<date>)")
	yesFlag := fs.Bool("yes", false, "Skip confirmation prompt (use with --force)")
	webCertFlag := fs.String("web-cert", "", "Generate TLS certificate for web server (listen address, e.g. 0.0.0.0:8080)")

	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze init",
			Summary: "Bootstrap the ze database with SSH credentials",
			Usage:   []string{"ze init [options]"},
			Sections: []helpfmt.HelpSection{
				{Title: "Input (stdin or interactive prompts)", Entries: []helpfmt.HelpEntry{
					{Name: "Line 1: username", Desc: ""},
					{Name: "Line 2: password", Desc: ""},
					{Name: "Line 3: host", Desc: "(default: 127.0.0.1)"},
					{Name: "Line 4: port", Desc: "(default: 2222)"},
					{Name: "Line 5: name", Desc: "(default: hostname)"},
				}},
				{Title: "Options", Entries: []helpfmt.HelpEntry{
					{Name: "--managed", Desc: "Enable managed (fleet) mode"},
					{Name: "--force", Desc: "Replace existing database (moves old to .replaced-<date>)"},
					{Name: "--yes", Desc: "Skip confirmation prompt (use with --force)"},
					{Name: "--web-cert <addr>", Desc: "Generate TLS certificate for web server (e.g. 0.0.0.0:8080)"},
				}},
			},
			Examples: []string{
				`echo -e "admin\nsecret\n127.0.0.1\n2222\nmy-router" | ze init`,
				"ze init --managed  (interactive prompts, managed mode)",
				"ze init --force         (replace existing database)",
				"ze init --force --yes   (replace without confirmation)",
			},
		}
		p.Write()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	dbPath := sshclient.ResolveDBPath()
	if dbPath == "" {
		fmt.Fprintf(os.Stderr, "error: cannot determine database location\n")
		return 1
	}

	// When piped, read all data first so --force can prompt on /dev/tty.
	var inputReader io.Reader = os.Stdin
	var promptWriter io.Writer
	interactive := isTerminal(os.Stdin)
	if interactive {
		promptWriter = os.Stderr
	} else {
		data, readErr := io.ReadAll(os.Stdin)
		if readErr != nil {
			fmt.Fprintf(os.Stderr, "error: reading stdin: %v\n", readErr)
			return 1
		}
		inputReader = bytes.NewReader(data)
	}

	// Handle --force: move existing database aside after confirmation.
	if *forceFlag {
		if _, err := os.Stat(dbPath); err == nil {
			if daemonRunning(dbPath) {
				fmt.Fprintf(os.Stderr, "error: daemon is running -- stop it before replacing the database\n")
				return 1
			}
			if !*yesFlag && !confirmForceReplace(dbPath) {
				fmt.Fprintf(os.Stderr, "aborted\n")
				return 1
			}
			if err := moveAsideDB(dbPath); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
		}
	}

	return runInit(inputReader, promptWriter, dbPath, *managedFlag, *webCertFlag)
}

// RunWithReader creates a zefs database with SSH credentials read from r.
// Format: one line each for username, password, host, port, name.
// Empty host defaults to 127.0.0.1, empty port defaults to 2222.
func RunWithReader(r io.Reader, dbPath string, managed bool) int {
	return runInit(r, nil, dbPath, managed, "")
}

// RunWithReaderForce is like RunWithReader but moves an existing database aside first.
// Used by tests and non-interactive callers where confirmation is handled externally.
func RunWithReaderForce(r io.Reader, dbPath string, managed bool) (int, error) {
	if _, err := os.Stat(dbPath); err == nil {
		if err := moveAsideDB(dbPath); err != nil {
			return 1, err
		}
	}
	return runInit(r, nil, dbPath, managed, ""), nil
}

// RunInteractive creates a zefs database with interactive prompts.
// Prompts are written to w (typically os.Stderr).
func RunInteractive(r io.Reader, w io.Writer, dbPath string) int {
	return runInit(r, w, dbPath, false, "")
}

func runInit(r io.Reader, promptW io.Writer, dbPath string, managed bool, webCertAddr string) int {
	// Check if database already exists
	if _, err := os.Stat(dbPath); err == nil {
		fmt.Fprintf(os.Stderr, "error: database already exists: %s\n", dbPath)
		fmt.Fprintf(os.Stderr, "hint: remove it first if you want to reinitialize\n")
		return 1
	}

	// Read credentials (with optional prompts)
	scanner := bufio.NewScanner(r)

	username := promptAndRead(scanner, promptW, "username: ")

	var password string
	if promptW != nil && r == os.Stdin && isTerminal(os.Stdin) {
		password = readPassword(promptW, "password: ")
	} else {
		password = promptAndRead(scanner, promptW, "password: ")
	}
	host := promptAndRead(scanner, promptW, "host [127.0.0.1]: ")
	port := promptAndRead(scanner, promptW, "port [2222]: ")
	defaultName, _ := os.Hostname()
	name := promptAndRead(scanner, promptW, fmt.Sprintf("name [%s]: ", defaultName))

	// Check for I/O errors during reading
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error: read input: %v\n", err)
		return 1
	}

	// Validate required fields
	if username == "" {
		fmt.Fprintf(os.Stderr, "error: username is required\n")
		return 1
	}
	if password == "" {
		fmt.Fprintf(os.Stderr, "error: password is required\n")
		return 1
	}

	// Hash password with bcrypt before storing -- zefs holds the hash,
	// which the CLI sends as an opaque auth token over SSH.
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: hash password: %v\n", err)
		return 1
	}

	// Apply defaults
	if host == "" {
		host = defaultHost
	}
	if port == "" {
		port = defaultPort
	}
	if name == "" {
		name = defaultName
	}

	// Create parent directory if needed
	if dir := filepath.Dir(dbPath); dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			fmt.Fprintf(os.Stderr, "error: create directory: %v\n", err)
			return 1
		}
	}

	// Create the database
	store, err := zefs.Create(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: create database: %v\n", err)
		return 1
	}

	// Write SSH credentials in deterministic order.
	type entry struct {
		key, value string
	}
	managedValue := "false"
	if managed {
		managedValue = "true"
	}

	entries := []entry{
		{keyUsername, username},
		{keyPassword, string(hashedPassword)},
		{keyHost, host},
		{keyPort, port},
		{keyManaged, managedValue},
	}
	if name != "" {
		entries = append(entries, entry{keyIdentityName, name})
	}

	for _, e := range entries {
		if err := store.WriteFile(e.key, []byte(e.value), 0); err != nil {
			fmt.Fprintf(os.Stderr, "error: write %s: %v\n", e.key, err)
			store.Close()     //nolint:errcheck // best-effort cleanup after write failure
			os.Remove(dbPath) //nolint:errcheck // best-effort cleanup of partial database
			return 1
		}
	}

	// Discover OS interfaces and generate initial config. LoadBackend
	// activates the netlink backend registered via the blank import
	// above; without it DiscoverInterfaces returns "no backend loaded"
	// and every detected netdev is silently dropped. Backend load
	// failures (e.g., non-Linux platforms with only the stub backend)
	// are non-fatal -- init still completes, the user just gets an
	// empty interface config.
	if loadErr := iface.LoadBackend("netlink"); loadErr != nil {
		fmt.Fprintf(os.Stderr, "warning: load netlink backend: %v\n", loadErr)
	} else {
		if discovered, discErr := iface.DiscoverInterfaces(); discErr != nil {
			fmt.Fprintf(os.Stderr, "warning: interface discovery: %v\n", discErr)
		} else if len(discovered) > 0 {
			if config := iface.EmitConfig(discovered); config != "" {
				configKey := zefs.KeyFileActive.Key("ze.conf")
				if wErr := store.WriteFile(configKey, []byte(config), 0); wErr != nil {
					fmt.Fprintf(os.Stderr, "warning: write initial config: %v\n", wErr)
				} else {
					fmt.Printf("discovered %d interface(s), wrote initial config\n", len(discovered))
				}
			}
		}
		if closeErr := iface.CloseBackend(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "warning: close netlink backend: %v\n", closeErr)
		}
	}

	// Generate and store TLS certificate if requested.
	if webCertAddr != "" {
		certPEM, keyPEM, certErr := zeweb.GenerateWebCertWithAddr(webCertAddr)
		if certErr != nil {
			fmt.Fprintf(os.Stderr, "error: generate TLS certificate: %v\n", certErr)
			store.Close()     //nolint:errcheck // best-effort cleanup
			os.Remove(dbPath) //nolint:errcheck // best-effort cleanup
			return 1
		}
		if err := store.WriteFile(zefs.KeyWebCert.Pattern, certPEM, 0); err != nil {
			fmt.Fprintf(os.Stderr, "error: write TLS cert: %v\n", err)
			store.Close()     //nolint:errcheck // best-effort cleanup
			os.Remove(dbPath) //nolint:errcheck // best-effort cleanup
			return 1
		}
		if err := store.WriteFile(zefs.KeyWebKey.Pattern, keyPEM, 0); err != nil {
			fmt.Fprintf(os.Stderr, "error: write TLS key: %v\n", err)
			store.Close()     //nolint:errcheck // best-effort cleanup
			os.Remove(dbPath) //nolint:errcheck // best-effort cleanup
			return 1
		}
		fmt.Printf("generated TLS certificate for %s\n", webCertAddr)
	}

	if err := store.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "error: close database: %v\n", err)
		return 1
	}

	fmt.Printf("initialized %s\n", dbPath)
	return 0
}

// readLine reads a single line from the scanner, trimming whitespace.
func readLine(scanner *bufio.Scanner) string {
	if !scanner.Scan() {
		return ""
	}
	return strings.TrimSpace(scanner.Text())
}

// isTerminal returns true if f is a terminal (not a pipe or file).
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// readPassword prompts for a password without echoing input to the terminal.
// Prints "***" after reading to confirm input was received.
func readPassword(w io.Writer, prompt string) string {
	fmt.Fprint(w, prompt) //nolint:errcheck // terminal prompt
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(w, "***") //nolint:errcheck // visual confirmation
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(pw))
}

// promptAndRead optionally writes a prompt to w, then reads a line.
func promptAndRead(scanner *bufio.Scanner, w io.Writer, prompt string) string {
	if w != nil {
		fmt.Fprint(w, prompt) //nolint:errcheck // terminal prompt
	}
	return readLine(scanner)
}

// confirmForceReplace prompts the user for confirmation before replacing an existing database.
// Returns true only if the user types "yes" (case-insensitive).
// When stdin is piped, opens /dev/tty for the confirmation prompt.
func confirmForceReplace(dbPath string) bool {
	var ttyReader io.Reader
	if isTerminal(os.Stdin) {
		ttyReader = os.Stdin
	} else {
		tty, err := os.Open("/dev/tty")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: --force requires a terminal for confirmation\n")
			return false
		}
		defer tty.Close() //nolint:errcheck // read-only
		ttyReader = tty
	}

	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  +-------------------------------------------------------+\n")
	fmt.Fprintf(os.Stderr, "  |  WARNING: replacing the existing database              |\n")
	fmt.Fprintf(os.Stderr, "  +-------------------------------------------------------+\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  Database : %s\n", dbPath)
	fmt.Fprintf(os.Stderr, "  Backup to: %s.replaced-<date>\n", filepath.Base(dbPath))
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  SSH credentials, instance metadata, and config state\n")
	fmt.Fprintf(os.Stderr, "  in the current database will be replaced.\n")
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "  Type 'yes' to proceed: ")

	scanner := bufio.NewScanner(ttyReader)
	if !scanner.Scan() {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(scanner.Text()), "yes")
}

// daemonRunning checks if a ze daemon is reachable by reading host/port
// from the existing database and dialing the SSH port.
func daemonRunning(dbPath string) bool {
	store, err := zefs.Open(dbPath)
	if err != nil {
		return false
	}
	defer store.Close() //nolint:errcheck // probe only

	host := defaultHost
	if data, err := store.ReadFile(keyHost); err == nil && len(data) > 0 {
		host = string(data)
	}
	port := defaultPort
	if data, err := store.ReadFile(keyPort); err == nil && len(data) > 0 {
		port = string(data)
	}

	d := net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.Dial("tcp", net.JoinHostPort(host, port))
	if err != nil {
		return false
	}
	conn.Close() //nolint:errcheck // probe connection
	return true
}

// moveAsideDB renames the existing database to <path>.replaced-<date>.
func moveAsideDB(dbPath string) error {
	stamp := time.Now().Format("2006-01-02T150405")
	dest := dbPath + ".replaced-" + stamp
	if err := os.Rename(dbPath, dest); err != nil {
		return fmt.Errorf("move database: %w", err)
	}
	fmt.Fprintf(os.Stderr, "moved %s to %s\n", filepath.Base(dbPath), filepath.Base(dest))
	return nil
}
