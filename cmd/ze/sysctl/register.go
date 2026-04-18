// Register the sysctl root command with the cmd/ze dispatcher.

package sysctl

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("sysctl", cmdregistry.Meta{
		Description: "Kernel sysctl helpers",
		Mode:        "offline",
		Subs:        "",
	})
}
