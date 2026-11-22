// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/mtu/cmd.

package cmd

import "github.com/ze-software/ze/internal/core/diagnostic"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeMTULocalState,
		Title:       "show mtu cannot read a local-state source",
		Description: "One of the two kernel sources show mtu reads beside the wire cannot be read: the fragmentation counters of /proc/net/snmp (through procfs) or net.ipv4.tcp_mtu_probing (through the sysctl component). The measurement and the tunnel sizing still run; the payload reports the finding that source feeds as unreadable instead of a value. The message names the source and its error. Mount procfs at /proc, or run on a host whose sysctl backend the daemon can read.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-mtu-local-state"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: cmd: doctor code registration refused: " + err.Error())
		}
	}
}
