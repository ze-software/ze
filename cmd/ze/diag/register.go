// Register the diag entry points (ping, traceroute, generate wireguard
// keypair) with the cmd/ze dispatcher. Imported by cmd/ze/main.go for
// its side effects.

package diag

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("ping", cmdregistry.Meta{
		Description: "Send ICMP echo-request (OS `ping` wrapper)",
		Mode:        "offline",
		Subs:        "--count N, --interface IF",
	})
	cmdregistry.RegisterRoot("traceroute", cmdregistry.Meta{
		Description: "Trace path to target (OS `traceroute` wrapper)",
		Mode:        "offline",
		Subs:        "--probes N, --interface IF",
	})
	cmdregistry.RegisterRoot("generate", cmdregistry.Meta{
		Description: "Generate artifacts (keypairs, bundles)",
		Mode:        "offline",
		Subs:        "wireguard keypair",
	})
	cmdregistry.MustRegisterLocal("ping", RunPing)
	cmdregistry.MustRegisterLocal("traceroute", RunTraceroute)
	cmdregistry.MustRegisterLocal("generate wireguard keypair", RunWgKeypair)
}
