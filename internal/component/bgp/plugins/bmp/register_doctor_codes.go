// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/bgp/plugins/bmp.

package bmp

import "github.com/ze-software/ze/internal/core/diagnostic"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeBMPUnreachable,
		Title:       "BMP collectors unreachable",
		Description: "None of the configured BMP sender collectors accepted a TCP connection within the probe timeout.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-bmp-unreachable"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: bmp: doctor code registration refused: " + err.Error())
		}
	}
}
