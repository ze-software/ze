package service

import "codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"

func init() {
	cmdregistry.RegisterRoot("service", cmdregistry.Meta{
		Description: "Manage ze as a systemd service",
		Mode:        "setup",
		Section:     cmdregistry.SectionSystem,
		Subs:        "install [--config] [--start] [--force] [--dry-run], uninstall [--purge], status",
	})
}
