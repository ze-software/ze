// Design: docs/guide/command-catalogue.md -- diagnostics (wireguard keygen)
// Related: ../main.go -- registers RunWgKeypair as a local command
//
// Package diag is the offline home for diagnostic commands that wrap OS
// tools with validated argv (no shell). The daemon is not required:
// subcommands are local shell-outs from the `ze` binary. Per
// rules/cli-patterns.md each subcommand uses its own flag.NewFlagSet with
// a custom Usage printer.
//
// The `ze ping` offline root and the daemon-side ping handlers now live in
// the dedicated ping feature module (internal/component/ping/cmd).

package diag

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// RunWgKeypair generates a WireGuard keypair by invoking `wg genkey`
// and `wg pubkey`. Prints two lines to stdout:
//
//	private: <base64>
//	public:  <base64>
//
// Usage: ze generate wireguard keypair
//
// Returns 1 if `wg` is not installed. No arguments accepted.
func RunWgKeypair(args []string) int {
	fs := flag.NewFlagSet("generate wireguard keypair", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		if _, err := fmt.Fprintln(os.Stderr, "Usage: ze generate wireguard keypair\n\nGenerate a WireGuard keypair by invoking `wg genkey` and `wg pubkey`.\nThe system must have the `wg` binary installed."); err != nil {
			return // writing to stderr; nothing to recover
		}
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 1
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "generate wireguard keypair: no arguments accepted")
		fs.Usage()
		return 1
	}
	ctx := context.Background()
	priv, err := exec.CommandContext(ctx, "wg", "genkey").Output() //nolint:gosec // no user input
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate wireguard keypair: wg genkey failed: %v\n", err)
		return 1
	}
	privStr := strings.TrimSpace(string(priv))
	pubCmd := exec.CommandContext(ctx, "wg", "pubkey") //nolint:gosec // no user input
	pubCmd.Stdin = strings.NewReader(privStr + "\n")
	pub, err := pubCmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "generate wireguard keypair: wg pubkey failed: %v\n", err)
		return 1
	}
	if _, err := fmt.Fprintf(os.Stdout, "private: %s\npublic:  %s\n", privStr, strings.TrimSpace(string(pub))); err != nil { //nolint:errcheck // output
		return 1
	}
	return 0
}
