// Register the firewall root command with the cmd/ze dispatcher.

package firewall

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("firewall", cmdregistry.Meta{
		Description: "Firewall management",
		Mode:        "offline",
		Subs:        "show, apply",
	})
}
