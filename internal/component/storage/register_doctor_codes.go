// Design: docs/architecture/doctor-and-health-checks.md -- doctor diagnostic codes owned by internal/component/storage.

package storage

import "github.com/ze-software/ze/internal/core/diagnostic"

// doctorDiagnosticCodes is the explanation metadata for the doctor codes this package owns.
var doctorDiagnosticCodes = []diagnostic.CodeMeta{
	{
		Code:        codeSmartSysfs,
		Title:       "SMART block devices cannot be enumerated",
		Description: "SMART management is enabled but /sys/class/block cannot be read, so the storage manager will find no device to monitor.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-smart-sysfs"},
	},
	{
		Code:        codeSmartAccess,
		Title:       "No block device answers SMART",
		Description: "SMART management is enabled and the host holds block devices, but none of them answers a SMART query, which is what a process with insufficient privileges for the SMART ioctls sees. Grant ze the privilege or disable storage/smart.",
		Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-smart-access"},
	},
}

func init() {
	for _, meta := range doctorDiagnosticCodes {
		if err := diagnostic.Register(meta); err != nil {
			panic("BUG: storage: doctor code registration refused: " + err.Error())
		}
	}
}
