// Design: docs/architecture/system-architecture.md — ze init bootstrap command

// Package init provides the `ze init` command that bootstraps the zefs database
// with SSH credentials before any other ze command can work.
package init

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	sshclient "codeberg.org/thomas-mangin/ze/cmd/ze/internal/ssh/client"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// SSH credential keys stored in zefs under meta/ namespace.
const (
	keyUsername = "meta/ssh/username"
	keyPassword = "meta/ssh/password" //nolint:gosec // key name, not a credential
	keyHost     = "meta/ssh/host"
	keyPort     = "meta/ssh/port"

	keyIdentityName = "meta/identity/name"
	keyManaged      = "meta/managed"

	defaultHost = "127.0.0.1"
	defaultPort = "2222"
)

// Run executes ze init from CLI arguments.
// Returns exit code.
func Run(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	managedFlag := fs.Bool("managed", false, "Enable managed (fleet) mode")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze init [options]

Bootstrap the ze database with SSH credentials.
Must be run before any other ze command.

Reads from stdin (piped) or prompts interactively:
  Line 1: username
  Line 2: password
  Line 3: host (default: 127.0.0.1)
  Line 4: port (default: 2222)
  Line 5: name (optional instance name)

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  echo -e "admin\nsecret\n127.0.0.1\n2222\nmy-router" | ze init
  ze init --managed  (interactive prompts, managed mode)
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
	name := promptAndRead(scanner, promptW, "name []: ")

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

	// Apply defaults
	if host == "" {
		host = defaultHost
	}
	if port == "" {
		port = defaultPort
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
		{keyPassword, password},
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
