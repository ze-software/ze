package install

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("install", cmdregistry.Meta{
		Description: "Install ze locally or provision remote devices",
		Mode:        "setup",
		Section:     cmdregistry.SectionSystem,
		Subs:        "local [--prefix] [--no-systemd], remote --interface --network --image --ssh-username --ssh-password, appliance <command>",
	})
}
