// Design: docs/architecture/container/runtime.md -- the system-call filter
// Related: table_x86_64.go -- the x86-64 call numbers
// Related: table_aarch64.go -- the AArch64 call numbers

//go:build linux

package seccomp

import (
	"fmt"
	"runtime"

	"golang.org/x/sys/unix"
)

// syscallTable is the system-call numbering of one architecture: its name,
// the audit constant the kernel reports for a call made under it, and the
// number of every call by its lower-case name.
//
// The two tables are plain files with no architecture suffix, so a host of
// either architecture compiles and tests both.
type syscallTable struct {
	// goarch is the runtime.GOARCH value of the architecture. It is also the
	// name the runtime library's profile gives the architecture
	// (configs.Seccomp.Architectures holds "amd64" and "arm64").
	goarch string
	// audit is the AUDIT_ARCH value seccomp_data.arch carries.
	audit uint32
	// numbers maps a call name to its number.
	numbers map[string]uint32
}

// The GOARCH names of the two shipped architectures, which the runtime
// library's profile also uses.
const (
	goarchAMD64 = "amd64"
	goarchARM64 = "arm64"
)

// tables are the architectures Ze ships for, one table each.
var tables = []*syscallTable{&amd64Table, &arm64Table}

// amd64Table is the x86-64 numbering.
var amd64Table = syscallTable{
	goarch:  goarchAMD64,
	audit:   unix.AUDIT_ARCH_X86_64,
	numbers: amd64Numbers,
}

// arm64Table is the AArch64 numbering.
var arm64Table = syscallTable{
	goarch:  goarchARM64,
	audit:   unix.AUDIT_ARCH_AARCH64,
	numbers: arm64Numbers,
}

// tableFor answers the table of goarch, and refuses an architecture Ze holds
// no table for: a filter built from a guessed numbering would allow and refuse
// the wrong calls.
func tableFor(goarch string) (*syscallTable, error) {
	for _, table := range tables {
		if table.goarch == goarch {
			return table, nil
		}
	}
	return nil, fmt.Errorf("container system-call filter: ze holds no system-call table for architecture %q", goarch)
}

// nativeTable answers the table of the architecture this binary runs on.
func nativeTable() (*syscallTable, error) {
	return tableFor(runtime.GOARCH)
}
