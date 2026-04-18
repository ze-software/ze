// Register the plugin root command with the cmd/ze dispatcher.

package plugin

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("plugin", cmdregistry.Meta{
		Description: "Plugin system",
		Mode:        "offline",
		Subs:        "<plugin-name> for plugin CLI, test for debugging",
	})
}
