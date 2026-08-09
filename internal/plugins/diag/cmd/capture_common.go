// Design: docs/architecture/diagnostics/packet-capture.md -- shared capture constants and helpers

package cmd

import (
	"strconv"
)

const (
	argCount               = "count"
	msgSubsystemNotRunning = "subsystem not running"

	capL2TP = "l2tp"
	capBGP  = "bgp"
	capBFD  = "bfd"

	fmtPcap = "pcap"
)

func extractCountFilter(args []string) int {
	for i, a := range args {
		if a == argCount && i+1 < len(args) {
			n, err := strconv.Atoi(args[i+1])
			if err == nil && n > 0 {
				return n
			}
		}
	}
	return 0
}
