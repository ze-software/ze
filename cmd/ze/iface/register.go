// Register the interface root command and `show interface` offline
// shortcut with the cmd/ze dispatcher.

package iface

import (
	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/cmdregistry"
)

func init() {
	cmdregistry.RegisterRoot("interface", cmdregistry.Meta{
		Description: "Manage OS network interfaces",
		Mode:        "offline",
		Subs:        "show, scan, create, delete, up, down, mtu, mac, neighbors, routes, clear, unit, addr, migrate",
	})
	cmdregistry.MustRegisterLocal("show interface", func(args []string) int {
		return Run(append([]string{"show"}, args...))
	})
}
