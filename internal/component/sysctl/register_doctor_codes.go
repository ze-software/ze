// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/sysctl.

package sysctl

import "github.com/ze-software/ze/internal/core/diagnostic"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        doctorSysctlProcfsCode,
		Title:       "Sysctl key not writable or undetermined",
		Description: "Sysctl settings are configured, and this process cannot write one or more configured /proc/sys key files (net.* keys need CAP_NET_ADMIN or root; other keys need root; /proc/sys may be read-only) or cannot determine whether they are writable.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-sysctl-procfs"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: sysctl: doctor code registration refused: " + err.Error())
		}
	}
}
