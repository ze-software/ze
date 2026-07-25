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

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/helpfmt"
	"github.com/ze-software/ze/internal/core/selfcert"
	sshclient "github.com/ze-software/ze/internal/core/ssh/client"
	"github.com/ze-software/ze/pkg/zefs"

	// Register the netlink backend so iface.LoadBackend("netlink")
	// below resolves. Without this blank import, DiscoverInterfaces
	// returns "no backend loaded" and every detected interface
	// (ethernet, dummy, veth, bridge, tunnel, wireguard) is silently
	// dropped from the initial ze.conf.
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
)

// Key aliases for readability (from zefs key registry).
var (
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
	webCertNameFlag := fs.String("web-cert-name", "", "Extra DNS name for the TLS certificate SAN (e.g. router.example.com)")
	seedFlag := fs.Bool("seed", false, "Seed database for an appliance image: skip baking this host's interface discovery into the active config (the appliance builds its config at first boot from the template plus on-device discovery)")

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
					{Name: "--web-cert-name <host>", Desc: "Extra DNS name for TLS certificate SAN (e.g. router.example.com)"},
					{Name: "--seed", Desc: "Appliance seed DB: skip on-host interface discovery (appliance builds its config at first boot from template + on-device discovery)"},
				}},
			},
			Examples: []string{
				`echo -e "admin\nsecret\n127.0.0.1\n2222\nmy-router" | ze init`,
				"ze init --managed  (interactive prompts, managed mode)",
				"ze init --force         (replace existing database)",
				"ze init --force --yes   (replace without confirmation)",
			},
		}
		p.WriteErr()
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

	return runInit(inputReader, promptWriter, dbPath, *managedFlag, *webCertFlag, *webCertNameFlag, *seedFlag)
}

// RunWithReader creates a zefs database with SSH credentials read from r.
// Format: one line each for username, password, host, port, name.
// Empty host defaults to 127.0.0.1, empty port defaults to 2222.
func RunWithReader(r io.Reader, dbPath string, managed bool) int {
	return runInit(r, nil, dbPath, managed, "", "", false)
}

// RunWithReaderForce is like RunWithReader but moves an existing database aside first.
// Used by tests and non-interactive callers where confirmation is handled externally.
func RunWithReaderForce(r io.Reader, dbPath string, managed bool) (int, error) {
	if _, err := os.Stat(dbPath); err == nil {
		if err := moveAsideDB(dbPath); err != nil {
			return 1, err
		}
	}
	return runInit(r, nil, dbPath, managed, "", "", false), nil
}

// RunInteractive creates a zefs database with interactive prompts.
// Prompts are written to w (typically os.Stderr).
func RunInteractive(r io.Reader, w io.Writer, dbPath string) int {
	return runInit(r, w, dbPath, false, "", "", false)
}

func runInit(r io.Reader, promptW io.Writer, dbPath string, managed bool, webCertAddr, webCertName string, seed bool) int {
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

	tmpPath := dbPath + ".init-tmp"
	store, err := zefs.Create(tmpPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: create database: %v\n", err)
		return 1
	}
	cleanupTmp := func() {
		store.Close()      //nolint:errcheck // best-effort cleanup
		os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup of partial database
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
		{zefs.KeySSHUsername.Key(host, port), username},
		{zefs.KeySSHPassword.Key(host, port), string(hashedPassword)},
		{zefs.KeyLocalAdminUsername.Pattern, username},
		{zefs.KeyLocalAdminPassword.Pattern, string(hashedPassword)},
		{zefs.KeySSHDefault.Pattern, host + "/" + port},
		{keyManaged, managedValue},
	}
	if name != "" {
		entries = append(entries, entry{keyIdentityName, name})
	}

	for _, e := range entries {
		if err := store.WriteFile(e.key, []byte(e.value), 0); err != nil {
			fmt.Fprintf(os.Stderr, "error: write %s: %v\n", e.key, err)
			cleanupTmp()
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
	//
	// --seed skips this entirely: an appliance-image seed DB must NOT bake
	// this build host's interfaces into file/active/ze.conf. That active
	// config would hold the wrong host's NICs and would shadow any
	// file/template/ze.conf so the appliance never applies it. Instead the
	// appliance boots with no active config and builds one at first boot from
	// the template merged with its own on-device discovery (see
	// cmd/ze/ze_core_start.go bootstrapConfigFromTemplate).
	if seed { //nolint:staticcheck // SA9003: intentional no-op; see comment above
		// appliance seed: nothing baked in; first boot discovers on-device.
	} else if loadErr := iface.LoadBackend("netlink"); loadErr != nil {
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
	// --web-cert-name generates a cert with the hostname as DNS SAN (no IP enumeration).
	// --web-cert generates a cert with IP SANs derived from the listen address.
	// Both can be combined.
	if webCertAddr != "" || webCertName != "" {
		var extraNames []string
		if webCertName != "" {
			extraNames = []string{webCertName}
		}
		certPEM, keyPEM, certErr := selfcert.GenerateWebCertWithNames(webCertAddr, extraNames, 0)
		if certErr != nil {
			fmt.Fprintf(os.Stderr, "error: generate TLS certificate: %v\n", certErr)
			cleanupTmp()
			return 1
		}
		if err := store.WriteFile(zefs.KeyWebCert.Pattern, certPEM, 0); err != nil {
			fmt.Fprintf(os.Stderr, "error: write TLS cert: %v\n", err)
			cleanupTmp()
			return 1
		}
		if err := store.WriteFile(zefs.KeyWebKey.Pattern, keyPEM, 0); err != nil {
			fmt.Fprintf(os.Stderr, "error: write TLS key: %v\n", err)
			cleanupTmp()
			return 1
		}
		switch {
		case webCertName != "" && webCertAddr != "":
			fmt.Printf("generated TLS certificate for %s (%s)\n", webCertName, webCertAddr)
		case webCertName != "":
			fmt.Printf("generated TLS certificate for %s\n", webCertName)
		default:
			fmt.Printf("generated TLS certificate for %s\n", webCertAddr)
		}
	}

	if err := store.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "error: close database: %v\n", err)
		os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup
		return 1
	}

	if err := os.Rename(tmpPath, dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: atomic rename %s -> %s: %v\n", tmpPath, dbPath, err)
		os.Remove(tmpPath) //nolint:errcheck // best-effort cleanup
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

// daemonRunning reports whether a live ze daemon is serving the database at
// dbPath. It reads the daemon's SSH host:port from the database, dials it, and
// reads the SSH identification banner the server sends on accept, returning
// true ONLY when that banner is ze's own "SSH-2.0-ze" marker.
//
// This is a positive-identification probe: a generic SSH server (e.g. the
// host's OpenSSH answering on 0.0.0.0:22, "SSH-2.0-OpenSSH_*"), a bare TCP
// listener, an unreachable port, or a slow/garbled response all yield false.
// The guard exists to stop `ze init --force` from clobbering a database a live
// ze is actively using; a live ze answers with its banner immediately on
// accept, so requiring that banner both fixes the non-ze false positive (which
// previously matched any TCP listener) and still protects a running daemon.
func daemonRunning(dbPath string) bool {
	store, err := zefs.Open(dbPath)
	if err != nil {
		return false
	}
	defer store.Close() //nolint:errcheck // probe only

	host, port := defaultHost, defaultPort
	if data, err := store.ReadFile(zefs.KeySSHDefault.Pattern); err == nil && len(data) > 0 {
		if parts := strings.SplitN(string(data), "/", 2); len(parts) == 2 {
			host, port = parts[0], parts[1]
		}
	}

	d := net.Dialer{Timeout: 2 * time.Second}
	conn, err := d.Dial("tcp", net.JoinHostPort(host, port))
	if err != nil {
		return false
	}
	defer conn.Close() //nolint:errcheck // probe connection

	return isZeSSHBanner(conn)
}

// isZeSSHBanner reads the SSH identification string the server sends on accept
// and reports whether it is ze's "SSH-2.0-ze" banner. The read is bounded by a
// deadline and the RFC 4253 §4.2 maximum (255 bytes) so a silent or flooding
// listener can neither hang the probe nor exhaust memory. Nothing from the
// untrusted peer is acted on beyond this classification.
func isZeSSHBanner(conn net.Conn) bool {
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second)) //nolint:errcheck // probe only
	r := bufio.NewReader(io.LimitReader(conn, 255))
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	return strings.HasPrefix(strings.TrimRight(line, "\r\n"), sshclient.ServerVersionBanner)
}

// moveAsideDB renames the existing database to <path>.replaced-<date>.
func moveAsideDB(dbPath string) error {
	dest, err := zefs.MoveAside(dbPath)
	if err != nil {
		return fmt.Errorf("move database: %w", err)
	}
	fmt.Fprintf(os.Stderr, "moved %s to %s\n", filepath.Base(dbPath), filepath.Base(dest))
	return nil
}
