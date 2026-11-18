// Design: docs/architecture/ospf/ospf-2-wire.md -- offline `ze ospf decode` subcommand entry

package cli

import "slices"

// Run executes the ospf decode command. With help it prints usage; otherwise it
// decodes an OSPF packet or LSA from a file or stdin to JSON.
func Run(args []string) int {
	if slices.ContainsFunc(args, isHelpArg) {
		usage()
		return 0
	}
	return cmdDecode(args)
}

func isHelpArg(a string) bool { return a == "-h" || a == "--help" || a == "help" }

func usage() {
	errln("usage: ze ospf decode [--pretty] [--opaque | --v3] [<path> | -]")
	errln("  reads one OSPF packet/LSA from a file or stdin, emits JSON on stdout")
	errln("  omit the input only when piping a packet into the command")
	errln("  --opaque: interpret input as an IPv4 opaque LSA (RFC 5250)")
	errln("  --v3:     interpret input as an OSPFv3 LSA (RFC 5340)")
}
