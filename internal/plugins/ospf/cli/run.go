// Design: docs/architecture/ospf/ospf-2-wire.md -- offline `ze ospf decode` subcommand entry

package cli

import "slices"

// Run executes the ospf decode command. With help it prints usage; otherwise it
// decodes a hex OSPFv2 packet from stdin to JSON.
func Run(args []string) int {
	if slices.ContainsFunc(args, isHelpArg) {
		usage()
		return 0
	}
	return cmdDecode(args)
}

func isHelpArg(a string) bool { return a == "-h" || a == "--help" || a == "help" }

func usage() {
	errln("usage: ze ospf decode [--pretty] < hex")
	errln("  decode a hex OSPFv2 packet from stdin to JSON")
}
