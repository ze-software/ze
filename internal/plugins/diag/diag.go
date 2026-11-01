// Design: docs/guide/command-catalogue.md -- diagnostics (wireguard keygen)
// Related: register.go -- registers runWgKeypair as a local-data command
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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os/exec"
	"strings"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// keyWireguardKeypair is the envelope key of the keypair answer, the key its
// drawing is registered under.
const keyWireguardKeypair = "wireguard-keypair"

const wgKeypairUsage = "Usage: ze request wireguard keypair\n\nGenerate a WireGuard keypair by invoking `wg genkey` and `wg pubkey`.\nThe system must have the `wg` binary installed.\n"

// wgKeypair is a generated WireGuard keypair, base64 as `wg` prints it.
type wgKeypair struct {
	PrivateKey string `json:"private-key"`
	PublicKey  string `json:"public-key"`
}

// wgKeypairAnswer is the answer of `request wireguard keypair`.
type wgKeypairAnswer struct {
	Keypair wgKeypair `json:"wireguard-keypair"`
}

// runWgKeypair generates a WireGuard keypair by invoking `wg genkey`
// and `wg pubkey`, and answers both keys. It writes nothing: the local
// route draws the answer as two lines,
//
//	private: <base64>
//	public:  <base64>
//
// Usage: ze request wireguard keypair
//
// Refuses with exit code 1 if `wg` is not installed. No arguments accepted.
func runWgKeypair(validated command.ValidatedArgs) command.LocalAnswer {
	args := validated.Tokens()
	var diag bytes.Buffer
	fs := flag.NewFlagSet("request wireguard keypair", flag.ContinueOnError)
	fs.SetOutput(&diag)
	fs.Usage = func() { diag.WriteString(wgKeypairUsage) }
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return command.LocalRefusal(0, diag.String())
		}
		return command.LocalRefusal(1, diag.String())
	}
	if fs.NArg() > 0 {
		return command.LocalRefusal(1, "request wireguard keypair: no arguments accepted\n"+wgKeypairUsage)
	}
	ctx := context.Background()
	priv, err := exec.CommandContext(ctx, "wg", "genkey").Output() //nolint:gosec // no user input
	if err != nil {
		return wgRefusal("wg genkey", err)
	}
	privStr := strings.TrimSpace(string(priv))
	pubCmd := exec.CommandContext(ctx, "wg", "pubkey") //nolint:gosec // no user input
	pubCmd.Stdin = strings.NewReader(privStr + "\n")
	pub, err := pubCmd.Output()
	if err != nil {
		return wgRefusal("wg pubkey", err)
	}
	return command.LocalAnswer{Payload: wgKeypairAnswer{Keypair: wgKeypair{
		PrivateKey: privStr,
		PublicKey:  strings.TrimSpace(string(pub)),
	}}}
}

// wgRefusal refuses the command after the named `wg` step failed.
func wgRefusal(step string, err error) command.LocalAnswer {
	var tb textbuf.Buffer
	tb.Str("request wireguard keypair: ").Str(step).Str(" failed: ").Err(err).Byte('\n')
	return command.LocalRefusal(1, tb.String())
}

// drawWgKeypair draws a keypair as the two lines the command printed.
func drawWgKeypair(raw json.RawMessage) (string, error) {
	var keypair wgKeypair
	if err := json.Unmarshal(raw, &keypair); err != nil {
		return "", err
	}
	var tb textbuf.Buffer
	tb.Str("private: ").Str(keypair.PrivateKey).Str("\npublic:  ").Str(keypair.PublicKey).Byte('\n')
	return tb.String(), nil
}
