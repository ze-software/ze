// Design: docs/architecture/wire/isis.md -- offline `ze isis decode` subcommand entry
//
// Package cli provides the offline IS-IS tooling that ships with the
// internal/plugins/isis codec. In the isis-2 (wire codec) slice the only verb
// is decode: it reads a hex IS-IS PDU from stdin and prints a JSON view. The
// running-daemon CLI surface (show isis neighbor/database/route, clear isis) is
// owned by isis-13 and lives separately; this command is a thin offline caller
// that proves the codec wires end-to-end (test/isis-wire/isis-pdu-1.ci).

package cli

import "slices"

// Run executes the isis decode subcommand. With a help flag it prints usage;
// otherwise it decodes a hex IS-IS PDU from stdin to JSON. There is a single
// verb (no sub-command switch). Returns the process exit code.
func Run(args []string) int {
	if slices.ContainsFunc(args, isHelpArg) {
		usage()
		return 0
	}
	return cmdDecode(args)
}

// isHelpArg reports whether a is a help flag.
func isHelpArg(a string) bool { return a == "-h" || a == "--help" || a == "help" }

// usage prints the isis decode usage to stderr.
func usage() {
	errln("usage: ze isis decode [--pretty] < hex")
	errln("  decode a hex IS-IS PDU from stdin to JSON")
}
