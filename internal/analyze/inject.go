// Design: docs/architecture/mrt.md — MRT RIB injection

package analyze

import "os"

const injectUsage = `ze-analyze inject -- inject MRT RIB entries into a running Ze instance

Reads TABLE_DUMP_V2 entries from an MRT file and injects them into the
local Ze RIB via the management API.

Usage:
  ze-analyze inject <file.mrt[.gz|.bz2]> [--nexthop <ip>] [--peer-asn <asn>]

Status: not yet implemented (requires Ze management API).
`

func runInject(args []string) int {
	if len(args) == 0 {
		os.Stderr.WriteString(injectUsage) //nolint:errcheck // usage output
		return 1
	}
	os.Stderr.WriteString("inject: not yet implemented\n") //nolint:errcheck // status
	return 1
}
