// Register the signal + status root commands with the cmd/ze dispatcher.

package signal

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("signal", cmdregistry.Meta{
		Description: "Send signals to the daemon via SSH",
		Mode:        "daemon",
		Section:     cmdregistry.SectionSystem,
		Subs:        "reload, stop, restart, quit",
	})
	cmdregistry.RegisterRoot("status", cmdregistry.Meta{
		Description: "Check if daemon is running",
		Mode:        "daemon",
		Section:     cmdregistry.SectionSystem,
		Subs:        "exit 0 = running, 1 = not",
	})
}
