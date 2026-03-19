// Design: docs/architecture/system-architecture.md — ze init bootstrap command

// Package init provides the `ze init` command that bootstraps the zefs database
// with SSH credentials before any other ze command can work.
package init

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	sshclient "codeberg.org/thomas-mangin/ze/cmd/ze/internal/ssh/client"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// SSH credential keys stored in zefs under meta/ namespace.
const (
	keyUsername = "meta/ssh/username"
	keyPassword = "meta/ssh/password" //nolint:gosec // key name, not a credential
	keyHost     = "meta/ssh/host"
	keyPort     = "meta/ssh/port"

	keyIdentityName = "meta/instance/name"
	keyManaged      = "meta/instance/managed"

	defaultHost = "127.0.0.1"
	defaultPort = "2222"
)

// Run executes ze init from CLI arguments.
// Returns exit code.
func Run(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	managedFlag := fs.Bool("managed", false, "Enable managed (fleet) mode")
	forceFlag := fs.Bool("force", false, "Replace existing database (moves old to .replaced-<date>)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze init [options]

Bootstrap the ze database with SSH credentials.
Must be run before any other ze command.

Reads from stdin (piped) or prompts interactively:
  Line 1: username
  Line 2: password
  Line 3: host (default: 127.0.0.1)
  Line 4: port (default: 2222)
  Line 5: name (default: hostname)

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  echo -e "admin\nsecret\n127.0.0.1\n2222\nmy-router" | ze init
  ze init --managed  (interactive prompts, managed mode)
  ze init --force    (replace existing database)
`)
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	dbPath := sshclient.ResolveDBPath()
	if dbPath == "" {
		fmt.Fprintf(os.Stderr, "error: cannot determine database location\n")
		return 1
	}

	// Handle --force: move existing database aside after confirmation.
	if *forceFlag {
		if _, err := os.Stat(dbPath); err == nil {
			if daemonRunning(dbPath) {
				fmt.Fprintf(os.Stderr, "error: daemon is running -- stop it before replacing the database\n")
				return 1
			}
			if !confirmForceReplace(dbPath) {
				fmt.Fprintf(os.Stderr, "aborted\n")
				return 1
			}
			if err := moveAsideDB(dbPath); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return 1
			}
		}
	}

	// Use interactive prompts when stdin is a terminal
	if isTerminal(os.Stdin) {
		return runInit(os.Stdin, os.Stderr, dbPath, *managedFlag)
	}
	return runInit(os.Stdin, nil, dbPath, *managedFlag)
}

// RunWithReader creates a zefs database with SSH credentials read from r.
// Format: one line each for username, password, host, port, name.
// Empty host defaults to 127.0.0.1, empty port defaults to 2222.
func RunWithReader(r io.Reader, dbPath string, managed bool) int {
	return runInit(r, nil, dbPath, managed)
}

// RunWithReaderForce is like RunWithReader but moves an existing database aside first.
// Used by tests and non-interactive callers where confirmation is handled externally.
func RunWithReaderForce(r io.Reader, dbPath string, managed bool) (int, error) {
	if _, err := os.Stat(dbPath); err == nil {
		if err := moveAsideDB(dbPath); err != nil {
			return 1, err
		}
	}
	return runInit(r, nil, dbPath, managed), nil
}

// RunInteractive creates a zefs database with interactive prompts.
// Prompts are written to w (typically os.Stderr).
func RunInteractive(r io.Reader, w io.Writer, dbPath string) int {
	return runInit(r, w, dbPath, false)
}

func runInit(r io.Reader, promptW io.Writer, dbPath string, managed bool) int {
	// Check if database already exists
	if _, err := os.Stat(dbPath); err == nil {
		fmt.Fprintf(os.Stderr, "error: database already exists: %s\n", dbPath)
		fmt.Fprintf(os.Stderr, "hint: remove it first if you want to reinitialize\n")
		return 1
	}

	// Read credentials (with optional prompts)
	scanner := bufio.NewScanner(r)

	username := promptAndRead(scanner, promptW, "username: ")
	password := promptAndRead(scanner, promptW, "password: ")
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

// promptAndRead optionally writes a prompt to w, then reads a line.
func promptAndRead(scanner *bufio.Scanner, w io.Writer, prompt string) string {
	if w != nil {
		fmt.Fprint(w, prompt) //nolint:errcheck // terminal prompt
	}
	return readLine(scanner)
}

// confirmForceReplace prompts the user for confirmation before replacing an existing database.
// Returns true only if the user types "yes" (case-insensitive). Non-interactive stdin aborts.
func confirmForceReplace(dbPath string) bool {
	if !isTerminal(os.Stdin) {
		fmt.Fprintf(os.Stderr, "error: --force requires interactive confirmation (stdin is not a terminal)\n")
		return false
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

	scanner := bufio.NewScanner(os.Stdin)
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
