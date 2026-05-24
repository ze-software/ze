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
	case "serve":
		return runServe(args[1:])
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
		Summary: "Zero-touch provisioning server",
		Usage:   []string{"ze install serve [options]"},
		Sections: []helpfmt.HelpSection{
			{Title: "Subcommands", Entries: []helpfmt.HelpEntry{
				{Name: "serve", Desc: "Start DHCP+PXE, TFTP, and HTTP provisioning servers"},
			}},
		},
		Examples: []string{
			"ze install serve --interface eth0 --network 10.0.0.0/24 --image /path/to/gokrazy.img --ssh-username admin --ssh-password secret",
		},
	}
	p.Write()
}
