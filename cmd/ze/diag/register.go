// Register the diag entry points (ping, generate wireguard keypair)
// with the cmd/ze dispatcher. Imported by cmd/ze/main.go for its side
// effects. Traceroute is handled by the daemon path (show traceroute)
// as a pure Go ICMP implementation; no offline wrapper needed.

package diag

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("ping", cmdregistry.Meta{
		Description: "Ping a target from this box (offline, uses OS ping)",
		Mode:        "offline",
		Section:     cmdregistry.SectionSystem,
		Subs:        "--count N, --interface IF",
	})
	cmdregistry.RegisterRoot("generate", cmdregistry.Meta{
		Description: "Generate cryptographic artifacts (keypairs, bundles)",
		Mode:        "offline",
		Section:     cmdregistry.SectionSystem,
		Subs:        "wireguard keypair",
	})
	cmdregistry.MustRegisterLocalMeta("ping", RunPing, cmdregistry.Meta{
		Description: "Ping a target using the OS ping command. Use --count N and --interface IF to control the test.",
	})
	cmdregistry.MustRegisterLocalMeta("generate wireguard keypair", RunWgKeypair, cmdregistry.Meta{
		Description: "Generate a WireGuard keypair. Prints private and public keys to stdout for use in your config.",
	})
}
