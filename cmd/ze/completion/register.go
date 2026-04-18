// Register the completion root command with the cmd/ze dispatcher.

package completion

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("completion", cmdregistry.Meta{
		Description: "Shell completion scripts",
		Mode:        "offline",
		Subs:        "bash, zsh, fish, nushell",
	})
}
