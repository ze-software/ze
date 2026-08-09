//go:build !linux

// Design: docs/architecture/mpls/mpls-kernel.md -- non-Linux `show mpls forwarding` stub
//
// The kernel MPLS FIB is Linux-only (the appliance target is Linux). On other
// platforms the command returns an empty table rather than erroring, so the CLI
// grammar still resolves and scripts get a well-formed (empty) response.
package mpls

func dumpMPLSRoutes(_ int) ([]forwardingEntry, error) {
	return nil, nil
}
