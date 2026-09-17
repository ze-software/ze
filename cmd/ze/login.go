// Design: docs/architecture/appliance-serial-login.md -- serial console login gate
//
// Gokrazy serial console authentication. When ze is invoked with argv[0]
// basename "ash" or "sh" (via /tmp/serial-busybox/ash symlink), this handler
// prompts for credentials before exec'ing into the real shell binary.
// Fail closed when the store or the credentials cannot be read (owner decision,
// 2026-09-17): the error is named on stderr and no shell is started. The
// appliance seeds the credentials at install, so an unreadable store is a fault.

//go:build ze_core

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/pkg/zefs"
)

const (
	shellBinaryPath = "/usr/local/bin/ze-recovery-shell"
	defaultZeFSDir  = "/perm/ze"
	maxLoginRetries = 3
	maxInputLen     = 256
)

var (
	execShellFn    = defaultExecShell
	readPasswordFn = defaultReadPassword
	isTerminalFn   = defaultIsTerminal
	retryDelay     = 2 * time.Second
)

func isShellInvocation(basename string) bool {
	return basename == "ash" || basename == "sh"
}

func loginMain() int {
	if !isTerminalFn(int(os.Stdin.Fd())) {
		return 1
	}

	dir := env.Get("ze.config.dir")
	if dir == "" {
		dir = defaultZeFSDir
	}

	db, err := storage.OpenReadOnly(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "login refused: cannot open store folder %s: %v\n", dir, err) //nolint:errcheck // serial console output
		return 1
	}
	defer db.Close() //nolint:errcheck // read-only access

	// Only an absent flag means "not disabled". A corrupt or unreadable flag is
	// refused by name: reading it as "not disabled" would let a disabled admin in.
	disabled, err := db.ReadFile(zefs.KeyInstanceAdminDisabled.Pattern)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "login refused: cannot read the local admin status: %v\n", err) //nolint:errcheck // serial console output
		return 1
	}
	if string(disabled) == booleanTextTrue {
		fmt.Fprintln(os.Stderr, "local admin login disabled") //nolint:errcheck // serial console output
		return 1
	}

	username, err := db.ReadFile(zefs.KeyLocalAdminUsername.Pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "login refused: cannot read the local admin username: %v\n", err) //nolint:errcheck // serial console output
		return 1
	}
	if len(username) == 0 {
		fmt.Fprintln(os.Stderr, "login refused: local admin username is empty") //nolint:errcheck // serial console output
		return 1
	}

	hash, err := db.ReadFile(zefs.KeyLocalAdminPassword.Pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "login refused: cannot read the local admin password: %v\n", err) //nolint:errcheck // serial console output
		return 1
	}
	if len(hash) == 0 {
		fmt.Fprintln(os.Stderr, "login refused: local admin password is empty") //nolint:errcheck // serial console output
		return 1
	}

	scanner := bufio.NewScanner(os.Stdin)

	// Scan returns false on EOF, on a read error, and on a line above
	// bufio.MaxScanTokenSize alike. Every one of them denies the login, so the
	// error needs no separate branch: this reads the console shut, not open.
	for attempt := range maxLoginRetries {
		fmt.Fprint(os.Stdout, "login: ") //nolint:errcheck // serial console output
		if !scanner.Scan() {
			return 1
		}
		inputUser := strings.TrimRight(scanner.Text(), "\r")
		if len(inputUser) > maxInputLen {
			inputUser = inputUser[:maxInputLen]
		}

		fmt.Fprint(os.Stdout, "password: ") //nolint:errcheck // serial console output
		inputPass, err := readPasswordFn(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stdout) //nolint:errcheck // serial console output
		if err != nil {
			return 1
		}
		if len(inputPass) > maxInputLen {
			inputPass = inputPass[:maxInputLen]
		}

		// Always run bcrypt regardless of username match to avoid timing side-channel.
		passErr := bcrypt.CompareHashAndPassword(hash, inputPass)
		if inputUser != string(username) || passErr != nil {
			fmt.Fprintln(os.Stderr, "login incorrect") //nolint:errcheck // serial console output
			if attempt < maxLoginRetries-1 {
				time.Sleep(retryDelay)
			}
			continue
		}

		fmt.Fprintf(os.Stderr, "authenticated as %s on serial console\n", inputUser) //nolint:errcheck // serial console output
		return execShellFn()
	}

	fmt.Fprintln(os.Stderr, "too many failed attempts") //nolint:errcheck // serial console output
	return 1
}

func defaultExecShell() int {
	err := crashlog.Exec(shellBinaryPath, []string{"ash"}, os.Environ())
	fmt.Fprintf(os.Stderr, "exec %s: %v\n", shellBinaryPath, err) //nolint:errcheck // serial console output
	return 1
}

func defaultReadPassword(fd int) ([]byte, error) {
	return term.ReadPassword(fd)
}

func defaultIsTerminal(fd int) bool {
	return term.IsTerminal(fd)
}
