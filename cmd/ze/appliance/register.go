package appliance

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("appliance", cmdregistry.Meta{
		Description: "Manage gokrazy-based Ze appliance images",
		Mode:        "offline",
		Subs:        "init, build, assemble, list, show, export, import",
	})
}
