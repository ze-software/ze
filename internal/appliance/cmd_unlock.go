// Design: docs/architecture/appliance/builder.md -- passphrase agent lifecycle

package appliance

import (
	"crypto/rand"
	"flag"
	"fmt"
	"os"

	"golang.org/x/term"
)

func init() {
	cmdUnlock = runUnlock
}

func runUnlock(args []string) int {
	fs := flag.NewFlagSet("appliance unlock", flag.ContinueOnError)
	stopFlag := fs.Bool("stop", false, "Stop running passphrase agent")
	durationFlag := fs.Duration("duration", DefaultAgentDuration, "Agent expiry duration")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze appliance unlock [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return exitError
	}

	if *stopFlag {
		if err := StopAgent(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return exitError
		}
		return exitOK
	}

	fmt.Fprint(os.Stderr, "Passphrase: ") //nolint:errcheck // prompt
	passphrase, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read passphrase: %v\n", err)
		return exitError
	}
	if len(passphrase) == 0 {
		fmt.Fprintf(os.Stderr, "error: passphrase is required\n")
		return exitError
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		fmt.Fprintf(os.Stderr, "error: generate salt: %v\n", err)
		return exitError
	}
	key := DeriveKey(passphrase, salt)
	ZeroBytes(passphrase)

	if err := RunAgent(key, *durationFlag); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitError
	}
	return exitOK
}
