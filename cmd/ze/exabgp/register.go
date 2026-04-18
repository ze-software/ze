// Register the exabgp root command with the cmd/ze dispatcher.

package exabgp

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("exabgp", cmdregistry.Meta{
		Description: "ExaBGP bridge tools",
		Mode:        "offline",
		Subs:        "plugin, migrate",
	})
}
