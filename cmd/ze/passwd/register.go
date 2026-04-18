// Register the passwd root command with the cmd/ze dispatcher.

package passwd

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("passwd", cmdregistry.Meta{
		Description: "Change stored SSH/HTTP passwords",
		Mode:        "setup",
		Subs:        "",
	})
}
