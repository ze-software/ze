// Design: docs/architecture/system-architecture.md -- daemon privilege checking
// Overview: drop.go -- privilege package
// Related: check_linux.go -- Linux capability-aware variant

//go:build !linux

package privilege

import (
	"errors"
	"os"
	"runtime"
)

// CheckPrivileges answers the generic capability inventory where no capability
// set exists to read: nothing missing for root, and for any other user the
// error that says the set cannot be determined. Each feature that needs a
// privilege still reports its own impact.
func CheckPrivileges() ([]Capability, error) {
	if os.Getuid() == 0 {
		return nil, nil
	}
	return nil, errors.New("capabilities exist on Linux only, and ze runs without root on " + runtime.GOOS)
}
