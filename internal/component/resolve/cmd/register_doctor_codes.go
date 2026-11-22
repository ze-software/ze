// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/resolve/cmd.

package cmd

import "github.com/ze-software/ze/internal/core/diagnostic"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeRIRSourceRefused,
		Title:       "RIR delegation source will not be read",
		Description: "A `system/rir/delegation-source` block names a URL that `update resolve rir` refuses, so that registry's file is never fetched and the ASN-to-registry table stays as it is. A source is read over HTTPS, or over plain HTTP only when its host is the router itself; the host is compared after parsing, so a name that merely starts with 127.0.0.1 is not loopback. The config editor refuses such a URL at commit time, so a running config carrying one came from an older binary, a restored backup, or an edit made outside the editor.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-rir-source-refused"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: cmd: doctor code registration refused: " + err.Error())
		}
	}
}
