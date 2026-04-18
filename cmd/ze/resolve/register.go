// Register the resolve root command with the cmd/ze dispatcher.

package resolve

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("resolve", cmdregistry.Meta{
		Description: "DNS resolver tools",
		Mode:        "offline",
		Subs:        "",
	})
}
