// Register the diag entry points (ping, request wireguard keypair)
// with the cmd/ze dispatcher. Blank-imported by
// internal/component/plugin/all for its side effects: the generated
// composition root names every package that registers a command.
// Traceroute is handled by the daemon path (show traceroute)
// as a pure Go ICMP implementation; no offline wrapper needed.

package diag

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	// The keypair is answered as DATA, so it reaches the pipe layer; the
	// drawing gives the text route the two lines the command always printed.
	command.MustRegisterTextRenderer(keyWireguardKeypair, drawWgKeypair)
	command.MustRegisterLocalData("request wireguard keypair", runWgKeypair, registry.Meta{
		ShortHelp: "Generate a WireGuard keypair with the system wg binary.",
		Description: "The private key is written on the first line and the public key on the second. " +
			"The wg binary must be installed on this host.",
	}, command.RenderLocalAnswer)
}
