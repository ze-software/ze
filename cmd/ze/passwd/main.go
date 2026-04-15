// Design: docs/architecture/system-architecture.md -- ze passwd helper

// Package passwd implements the `ze passwd` subcommand.
//
// Reads a plaintext password from stdin (piped) or the controlling terminal
// (interactive) and prints a bcrypt hash to stdout suitable for pasting into
// the `password` leaf of `system.authentication.user`. Uses the same cost
// (bcrypt.DefaultCost) as `ze init` and the config commit hook so all three
// produce interchangeable hashes.
package passwd

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
)

// Run executes the passwd subcommand. Returns an exit code.
func Run(args []string) int {
	fs := flag.NewFlagSet("passwd", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		p := helpfmt.Page{
			Command: "ze passwd",
			Summary: "Bcrypt-hash a plaintext password for system.authentication.user",
			Usage: []string{
				"ze passwd                  Interactive prompt (twice for confirmation)",
				"echo <plain> | ze passwd   Hash piped plaintext, print on stdout",
			},
			Examples: []string{
				`echo "secret" | ze passwd`,
				`ze config set ze.conf system authentication user alice password "$(echo secret | ze passwd)"`,
			},
		}
		p.Write()
	}
	if err := fs.Parse(args); err != nil {
		return 1
	}
	return runImpl(os.Stdin, os.Stdout, os.Stderr)
}

// scannerMaxBytes bounds the stdin scanner just above the bcrypt 72-byte
// rejection point. Oversize input is read in full so runImpl can surface
// the bcrypt.ErrPasswordTooLong with a clear message rather than letting
// the scanner return a truncated buffer that looks like "empty password".
const scannerMaxBytes = 1024

// runImpl is the testable core. Reads plaintext from r, writes the hash to w,
// and uses errOut for prompts and error messages. Detects whether r is a TTY
// to decide between interactive double-prompt and single-line piped input.
// Exit codes follow rules/cli-patterns.md: 0 success, 1 any error.
func runImpl(r io.Reader, w, errOut io.Writer) int {
	plain, err := readPlaintext(r, errOut)
	if err != nil {
		fmt.Fprintf(errOut, "error: %v\n", err) //nolint:errcheck // best-effort error output
		return 1
	}
	if plain == "" {
		fmt.Fprintln(errOut, "error: empty password") //nolint:errcheck // best-effort error output
		return 1
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		if errors.Is(err, bcrypt.ErrPasswordTooLong) {
			fmt.Fprintf(errOut, //nolint:errcheck // best-effort error output
				"error: password too long (%d bytes; bcrypt limit is 72)\n", len(plain))
		} else {
			fmt.Fprintf(errOut, "error: bcrypt: %v\n", err) //nolint:errcheck // best-effort error output
		}
		return 1
	}
	fmt.Fprintln(w, string(hash)) //nolint:errcheck // best-effort hash output
	return 0
}

// readPlaintext returns the plaintext password.
//   - If r is a TTY: prompt twice and require both entries to match.
//   - Otherwise (piped/file): read the first line.
//
// Scanner buffer is sized above bcrypt's 72-byte limit so an oversize input
// is fully captured and rejected by bcrypt with a clear message rather than
// silently truncated to a misleading "empty password" error.
func readPlaintext(r io.Reader, errOut io.Writer) (string, error) {
	if f, ok := r.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		return readPlaintextTTY(f, errOut)
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 128), scannerMaxBytes)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return "", nil
	}
	return strings.TrimRight(scanner.Text(), "\r\n"), nil
}

// readPlaintextTTY prompts twice (Password / Confirm) without echo and
// requires the two entries to match.
func readPlaintextTTY(tty *os.File, errOut io.Writer) (string, error) {
	fmt.Fprint(errOut, "Password: ") //nolint:errcheck // terminal prompt
	first, err := term.ReadPassword(int(tty.Fd()))
	fmt.Fprintln(errOut) //nolint:errcheck // terminal newline
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	fmt.Fprint(errOut, "Confirm:  ") //nolint:errcheck // terminal prompt
	second, err := term.ReadPassword(int(tty.Fd()))
	fmt.Fprintln(errOut) //nolint:errcheck // terminal newline
	if err != nil {
		return "", fmt.Errorf("read confirmation: %w", err)
	}
	if !bytes.Equal(first, second) {
		return "", fmt.Errorf("passwords do not match")
	}
	return string(first), nil
}
