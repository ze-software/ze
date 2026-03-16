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

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/sshclient"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// SSH credential keys stored in zefs.
const (
	keyUsername = "ssh/username"
	keyPassword = "ssh/password"
	keyHost     = "ssh/host"
	keyPort     = "ssh/port"

	defaultHost = "127.0.0.1"
	defaultPort = "2222"
)

// Run executes ze init from CLI arguments.
// Returns exit code.
func Run(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze init [options]

Bootstrap the ze database with SSH credentials.
Must be run before any other ze command.

Reads from stdin (piped) or prompts interactively:
  Line 1: username
  Line 2: password
  Line 3: host (default: 127.0.0.1)
  Line 4: port (default: 2222)

Options:
`)
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, `
Examples:
  echo -e "admin\nsecret\n127.0.0.1\n2222" | ze init
  ze init  (interactive prompts)
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
		return RunInteractive(os.Stdin, os.Stderr, dbPath)
	}
	return RunWithReader(os.Stdin, dbPath)
}

// RunWithReader creates a zefs database with SSH credentials read from r.
// Format: one line each for username, password, host, port.
// Empty host defaults to 127.0.0.1, empty port defaults to 2222.
// When w is non-nil, interactive prompts are written to w.
// Returns exit code.
func RunWithReader(r io.Reader, dbPath string) int {
	return runInit(r, nil, dbPath)
}

// RunInteractive creates a zefs database with interactive prompts.
// Prompts are written to w (typically os.Stderr).
func RunInteractive(r io.Reader, w io.Writer, dbPath string) int {
	return runInit(r, w, dbPath)
}

func runInit(r io.Reader, promptW io.Writer, dbPath string) int {
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

	// Write SSH credentials
	entries := map[string]string{
		keyUsername: username,
		keyPassword: password,
		keyHost:     host,
		keyPort:     port,
	}

	for key, value := range entries {
		if err := store.WriteFile(key, []byte(value), 0); err != nil {
			store.Close() //nolint:errcheck // best-effort cleanup after write failure
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
