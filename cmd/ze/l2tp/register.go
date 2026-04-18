// Register the l2tp root command with the cmd/ze dispatcher.

package l2tp

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("l2tp", cmdregistry.Meta{
		Description: "L2TP tools",
		Mode:        "offline",
		Subs:        "",
	})
}
