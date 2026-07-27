// Design: (none -- research/analysis tool)
//
// Dumps MRT records as BGP UPDATE hex, one per line.
// Useful for piping into ze bgp decode or other analysis tools.
package analyze

import (
	"encoding/hex"
	"fmt"
	"os"
)

func runMRTDump(args []string) int {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, `ze-analyze mrt-dump -- dump MRT records as BGP UPDATE hex

Reads MRT files (RIB dumps or BGP4MP updates) and outputs each route as a
BGP UPDATE message body in hex encoding, one per line. This is useful for
piping into 'ze bgp decode' or other tools that process raw UPDATE hex.

For RIB dumps: each RIB entry is wrapped in an UPDATE (withdrawn=0 + attrs + NLRI).
For BGP4MP: the UPDATE body is extracted directly.

Usage:
  ze-analyze mrt-dump <file.gz> [file2.gz ...]

Examples:
  ze-analyze mrt-dump test/internet/latest-bview.gz | head -10
  ze-analyze mrt-dump test/internet/ripe-updates.*.gz | ze bgp decode -
`)
		return 1
	}

	var damaged malformedCounter
	defer damaged.report(os.Stderr)

	for _, fname := range args {
		if err := processMRTFile(fname, mrtHandler{
			OnRIB: func(data []byte, subtype uint16) {
				nlri := getRIBPrefix(data)
				if nlri == nil {
					return
				}
				damaged.note(forEachRIBEntry(data, subtype, func(_ uint16, attrs []byte) {
					update := buildUpdate(nil, attrs, nlri)
					fmt.Println(hex.EncodeToString(update))
				}))
			},
			OnBGP4MP: func(data []byte, subtype uint16, _ uint32) {
				body, _ := extractBGP4MPUpdate(subtype, data)
				if body != nil {
					fmt.Println(hex.EncodeToString(body))
				}
			},
		}); err != nil {
			fmt.Fprintf(os.Stderr, "error processing %s: %v\n", fname, err)
			return 1
		}
	}

	return 0
}
