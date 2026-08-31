// Register the diag entry points (ping, generate wireguard keypair)
// with the cmd/ze dispatcher. Imported by cmd/ze/main.go for its side
// effects. Traceroute is handled by the daemon path (show traceroute)
// as a pure Go ICMP implementation; no offline wrapper needed.

// codegen:skip -- CLI command wired via cmd/ze/main.go, not a runtime plugin.

package diag

import (
	"github.com/ze-software/ze/internal/component/command/registry"
)

func init() {
	registry.RegisterRoot("generate", registry.Meta{
		Description: "Generate cryptographic artifacts (keypairs, bundles)",
		Mode:        "offline",
		Section:     registry.SectionSystem,
		Subs:        "wireguard keypair",
	})
	registry.MustRegisterLocalMeta("generate wireguard keypair", RunWgKeypair, registry.Meta{
		Description: "Generate a WireGuard keypair with the system wg binary.",
		LongHelp: "The private key is written on the first line and the public key on the second. " +
			"The wg binary must be installed on this host.",
	})
}
