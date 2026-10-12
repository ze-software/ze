//go:build !linux

// Design: docs/architecture/mpls/mpls-kernel.md -- non-Linux `show mpls forwarding` refusal
//
// Ze reads the kernel MPLS FIB on Linux alone (the appliance target is Linux).
// On any other host the reader refuses with an error naming the OS, so the CLI
// grammar still resolves and `show mpls forwarding` says the table cannot be
// read, instead of an empty table a caller would take for "no MPLS routes".
// OpenBSD's kernel has MPLS, but Ze does not read its table.
package mpls

import (
	"errors"
	"runtime"
)

func dumpMPLSRoutes(_ int) ([]forwardingEntry, error) {
	return nil, errors.New("cannot read the MPLS forwarding table: Ze reads the kernel table " +
		"on Linux only, and this host runs " + runtime.GOOS + ", so no label route " +
		"can be shown or checked here. Run ze on Linux to read the table")
}
