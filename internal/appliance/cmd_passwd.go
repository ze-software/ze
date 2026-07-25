// Design: plan/learned/675-appliance-1-builder.md — password rotation

package appliance

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

	"github.com/ze-software/ze/internal/core/env"
)

func init() {
	cmdPasswd = runPasswd
}

func runPasswd(args []string) int {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "usage: ze appliance passwd <name>\n")
		return exitError
	}
	name := args[0]
	dir := getBaseDir()

	if _, err := os.Stat(ConfigPath(dir, name)); err != nil {
		fmt.Fprintf(os.Stderr, "error: appliance %q not found at %s\n", name, AppliancePath(dir, name))
		return exitError
	}

	var passphrase []byte
	if IsEncrypted(dir, name) {
		var err error
		passphrase, _, err = ResolvePassphrase(nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
		defer ZeroBytes(passphrase)
	}

	password := readNewPassword()
	if password == "" {
		fmt.Fprintf(os.Stderr, "error: password is required\n")
		return exitError
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: hash password: %v\n", err)
		return exitError
	}

	if err := WriteSecret(secretFilePath(dir, name, "password.hash"), hashed, passphrase); err != nil {
		fmt.Fprintf(os.Stderr, "error: write password hash: %v\n", err)
		return exitError
	}

	fmt.Printf("password updated for appliance %q\n", name)
	return exitOK
}

func readNewPassword() string {
	if envPass := env.Get(sshPasswordKey); envPass != "" {
		fmt.Fprintf(os.Stderr, "WARNING: password from environment variable\n")
		return envPass
	}
	if !isTerminal(os.Stdin) {
		return ""
	}
	fmt.Fprint(os.Stderr, "New SSH password: ") //nolint:errcheck // prompt
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return ""
	}
	return string(pass)
}
