// Design: plan/spec-iface-2-manage.md — Interface show subcommand

package iface

import (
	"flag"
	"fmt"
	"os"
)

// cmdShow lists interfaces or shows details for a specific one.
// Returns exit code.
func cmdShow(args []string) int {
	fs := flag.NewFlagSet("ze interface show", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "Output in JSON format")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze interface show [options] [name]

List all interfaces or show details for a specific interface.

Options:
  --json    Output in JSON format

Examples:
  ze interface show
  ze interface show eth0
  ze interface show --json
`)
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	remaining := fs.Args()

	switch len(remaining) {
	case 0:
		// List all interfaces
		if *jsonOutput {
			fmt.Println(`{"status":"ok","message":"interface show not yet implemented"}`)
		} else {
			fmt.Println("interface show: not yet implemented")
		}
		return 0
	case 1:
		// Show specific interface
		name := remaining[0]
		if *jsonOutput {
			fmt.Printf("{\"status\":\"ok\",\"name\":%q,\"message\":\"interface show not yet implemented\"}\n", name)
		} else {
			fmt.Printf("interface show %s: not yet implemented\n", name)
		}
		return 0
	default:
		fmt.Fprintf(os.Stderr, "error: too many arguments\n")
		fs.Usage()
		return 1
	}
}
