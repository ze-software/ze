// Design: docs/architecture/mrt.md — MRT BGP session replay

package analyze

import "os"

const replayUsage = `ze-analyze replay -- replay MRT over a BGP session

Reads BGP4MP records from an MRT file and replays them over a live BGP
session to a remote peer, preserving original timing where possible.

Usage:
  ze-analyze replay <file.mrt[.gz|.bz2]> <peer-ip:port>

Status: not yet implemented.
`

func runReplay(args []string) int {
	if len(args) == 0 {
		os.Stderr.WriteString(replayUsage) //nolint:errcheck // usage output
		return 1
	}
	os.Stderr.WriteString("replay: not yet implemented\n") //nolint:errcheck // status
	return 1
}
