// Register the diag entry points (ping, generate wireguard keypair)
// with the cmd/ze dispatcher. Blank-imported by
// internal/component/plugin/all for its side effects: the generated
// composition root names every package that registers a command.
// Traceroute is handled by the daemon path (show traceroute)
// as a pure Go ICMP implementation; no offline wrapper needed.

package diag

import (
	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	registry.RegisterRoot("generate", registry.Meta{
		ShortHelp: "Generate cryptographic artifacts (keypairs, bundles)",
		Mode:      "offline",
		Section:   registry.SectionSystem,
		Subs:      "wireguard keypair",
	})
	registry.MustRegisterLocalMeta("generate wireguard keypair", RunWgKeypair, registry.Meta{
		ShortHelp: "Generate a WireGuard keypair with the system wg binary.",
		Description: "The private key is written on the first line and the public key on the second. " +
			"The wg binary must be installed on this host.",
	})
}
