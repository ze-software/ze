// Design: plan/learned/878-appliance-login-shell.md -- serial console login gate
//
// Gokrazy serial console authentication. When ze is invoked with argv[0]
// basename "ash" or "sh" (via /tmp/serial-busybox/ash symlink), this handler
// prompts for credentials before exec'ing into the real shell binary.
// Fail-open when ZeFS is missing: serial console is the last-resort recovery path.

//go:build ze_core

package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

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
	dbPath := filepath.Join(dir, "database.zefs")

	db, err := zefs.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot open %s: %v (granting access without authentication)\n", dbPath, err) //nolint:errcheck // serial console output
		return execShellFn()
	}
	defer db.Close() //nolint:errcheck // read-only access

	if disabled, disErr := db.ReadFile(zefs.KeyInstanceAdminDisabled.Pattern); disErr == nil && string(disabled) == "true" {
		fmt.Fprintln(os.Stderr, "local admin login disabled") //nolint:errcheck // serial console output
		return 1
	}

	username, err := db.ReadFile(zefs.KeyLocalAdminUsername.Pattern)
	if err != nil || len(username) == 0 {
		fmt.Fprintf(os.Stderr, "warning: local admin credentials not configured (granting access without authentication)\n") //nolint:errcheck // serial console output
		return execShellFn()
	}

	hash, err := db.ReadFile(zefs.KeyLocalAdminPassword.Pattern)
	if err != nil || len(hash) == 0 {
		fmt.Fprintf(os.Stderr, "warning: local admin password not configured (granting access without authentication)\n") //nolint:errcheck // serial console output
		return execShellFn()
	}

	scanner := bufio.NewScanner(os.Stdin)

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
	err := syscall.Exec(shellBinaryPath, []string{"ash"}, os.Environ()) //nolint:gosec // hardcoded constant path, not user input
	fmt.Fprintf(os.Stderr, "exec %s: %v\n", shellBinaryPath, err)       //nolint:errcheck // serial console output
	return 1
}

func defaultReadPassword(fd int) ([]byte, error) {
	return term.ReadPassword(fd)
}

func defaultIsTerminal(fd int) bool {
	return term.IsTerminal(fd)
}
