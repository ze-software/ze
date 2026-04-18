// Register the cli root command with the cmd/ze dispatcher.

package cli

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("cli", cmdregistry.Meta{
		Description: "Interactive CLI for the running daemon",
		Mode:        "daemon",
		Subs:        "-c <cmd> for single command",
	})
}
