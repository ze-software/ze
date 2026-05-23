// Register the init root command with the cmd/ze dispatcher.

package init

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("init", cmdregistry.Meta{
		Description: "Bootstrap database with SSH credentials",
		Mode:        "setup",
		Section:     cmdregistry.SectionSystem,
		Subs:        "--managed for fleet mode, --force to replace",
	})
}
