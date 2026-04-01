// Design: plan/spec-iface-2-manage.md — Interface delete subcommand

package iface

import (
	"fmt"
	"os"

	"codeberg.org/thomas-mangin/ze/cmd/ze/internal/helpfmt"
	mgr "codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// cmdDelete removes an interface by name.
// Returns exit code.
func cmdDelete(args []string) int {
	if len(args) < 1 {
		deleteUsage()
		return 1
	}

	switch args[0] {
	case "help", "-h", "--help": //nolint:goconst // consistent pattern across cmd files
		deleteUsage()
		return 0
	}

	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "error: delete requires exactly one argument: <name>\n")
		deleteUsage()
		return 1
	}

	name := args[0]
	if err := mgr.DeleteInterface(name); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	fmt.Printf("deleted interface %s\n", name)
	return 0
}

func deleteUsage() {
	p := helpfmt.Page{
		Command: "ze interface delete",
		Summary: "Delete a network interface",
		Usage:   []string{"ze interface delete <name>"},
		Examples: []string{
			"ze interface delete lo1",
			"ze interface delete ze0",
		},
	}
	p.Write()
}
