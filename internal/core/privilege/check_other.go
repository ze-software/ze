// Design: docs/architecture/system-architecture.md -- daemon privilege checking
// Overview: drop.go -- privilege package
// Related: check_linux.go -- Linux capability-aware variant

//go:build !linux

package privilege

import "os"

// CheckPrivileges returns warnings when not running as root.
// On non-Linux platforms there is no capability system to inspect.
func CheckPrivileges() []string {
	if os.Getuid() == 0 {
		return nil
	}
	return []string{"running without root; privileged operations (port 179, raw sockets, FIB) will fail"}
}
