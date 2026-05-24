package install

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("install", cmdregistry.Meta{
		Description: "Zero-touch provisioning server",
		Mode:        "setup",
		Section:     cmdregistry.SectionSystem,
		Subs:        "serve --interface --network --image --ssh-username --ssh-password",
	})
}
