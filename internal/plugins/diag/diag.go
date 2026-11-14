// Design: docs/guide/command-catalogue.md -- diagnostics (wireguard keygen)
// Related: register.go -- registers runWgKeypair as a local-data command
//
// Package diag is the offline home of `request wireguard keypair`. The daemon
// is not required: the command is answered in the `ze` process that runs it.
// Per rules/cli-patterns.md each subcommand uses its own flag.NewFlagSet with
// a custom Usage printer.
//
// The `ze ping` offline root and the daemon-side ping handlers now live in
// the dedicated ping feature module (internal/component/ping/cmd).

package diag

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// keyWireguardKeypair is the envelope key of the keypair answer, the key its
// drawing is registered under.
const keyWireguardKeypair = "wireguard-keypair"

const wgKeypairUsage = "Usage: ze request wireguard keypair\n\nGenerate a WireGuard keypair inside ze. No other program is needed.\n"

// wgKeypair is a generated WireGuard keypair, each key in the standard base64
// form that `wg genkey` and `wg pubkey` print and a WireGuard config reads.
type wgKeypair struct {
	PrivateKey string `json:"private-key"`
	PublicKey  string `json:"public-key"`
}

// wgKeypairAnswer is the answer of `request wireguard keypair`.
type wgKeypairAnswer struct {
	Keypair wgKeypair `json:"wireguard-keypair"`
}

// runWgKeypair generates a WireGuard keypair in process and answers both
// keys. The private key is a Curve25519 scalar read from crypto/rand and
// clamped as https://cr.yp.to/ecdh.html requires; the public key is that
// scalar times the base point (wgtypes.GeneratePrivateKey, Key.PublicKey).
// That is what `wg genkey` and `wg pubkey` compute, so no external program
// runs and a host without wireguard-tools, the appliance image included,
// answers the same. It writes nothing and logs nothing: the private key
// leaves only in the answer, which the local route draws as two lines,
//
//	private: <base64>
//	public:  <base64>
//
// Usage: ze request wireguard keypair
//
// No arguments accepted.
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
		refusal := opererr.New("request wireguard keypair: no arguments accepted",
			"Usage: ze request wireguard keypair (the keys are made from random bytes alone, so nothing typed after the command is read)",
			"ze request wireguard keypair")
		return command.LocalRefusal(1, refusal.Error()+"\n")
	}
	private, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return wgRandomRefusal(err)
	}
	return command.LocalAnswer{Payload: wgKeypairAnswer{Keypair: wgKeypair{
		PrivateKey: private.String(),
		PublicKey:  private.PublicKey().String(),
	}}}
}

// wgRandomRefusal refuses the command when the system random source could not
// supply the private key's bytes, so no partial keypair is ever answered.
func wgRandomRefusal(err error) command.LocalAnswer {
	refusal := opererr.Wrap(err,
		"request wireguard keypair: no keypair was made",
		"the system random source did not supply the bytes a private key needs",
		"ze request wireguard keypair")
	return command.LocalRefusal(1, refusal.Error()+"\n")
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
