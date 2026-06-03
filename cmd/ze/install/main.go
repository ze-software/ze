// Design: plan/spec-install-0-umbrella.md — ze install provisioning subcommand

package install

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
)

func Run(args []string) int {
	if len(args) == 0 {
		usage()
		return 0
	}

	switch args[0] {
	case "local":
		return runLocal(args[1:])
	case "remote":
		return runRemote(args[1:])
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown install subcommand: %s\n", args[0])
		usage()
		return 1
	}
}

func usage() {
	p := helpfmt.Page{
		Command: "ze install",
		Summary: "Install ze locally or provision remote devices",
		Usage:   []string{"ze install <subcommand> [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Subcommands", Entries: []helpfmt.HelpEntry{
				{Name: "local", Desc: "Install ze binary, systemd unit, and config directory on this machine"},
				{Name: "remote", Desc: "Start DHCP+PXE, TFTP, and HTTP provisioning servers"},
			}},
		},
		Examples: []string{
			"ze install local",
			"ze install local --prefix /usr/local",
			"ze install remote --interface eth0 --network 10.0.0.0/24 --image /path/to/gokrazy.img --ssh-username admin --ssh-password secret",
		},
	}
	p.Write()
}
