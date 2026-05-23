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
		Description: "Send ICMP echo-request (OS `ping` wrapper)",
		Mode:        "offline",
		Section:     cmdregistry.SectionSystem,
		Subs:        "--count N, --interface IF",
	})
	cmdregistry.RegisterRoot("generate", cmdregistry.Meta{
		Description: "Generate artifacts (keypairs, bundles)",
		Mode:        "offline",
		Section:     cmdregistry.SectionSystem,
		Subs:        "wireguard keypair",
	})
	cmdregistry.MustRegisterLocal("ping", RunPing)
	cmdregistry.MustRegisterLocal("generate wireguard keypair", RunWgKeypair)
}
