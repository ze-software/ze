//go:build !linux

// Design: plan/spec-mpls-1-kernel.md -- non-Linux `show mpls forwarding` stub
// Related: mpls_forwarding.go -- handler and MPLSForwardingEntry type
// Related: mpls_forwarding_linux.go -- the real kernel reader
//
// The kernel MPLS FIB is Linux-only (the appliance target is Linux). On other
// platforms the command returns an empty table rather than erroring, so the CLI
// grammar still resolves and scripts get a well-formed (empty) response.
package show

func dumpMPLSRoutes(_ int) ([]MPLSForwardingEntry, error) {
	return nil, nil
}
