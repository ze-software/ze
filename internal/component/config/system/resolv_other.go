//go:build !linux

// Design: docs/architecture/config/syntax.md -- resolv.conf stub (non-Linux)
// Overview: resolv.go -- Resolvers, the per-source record

package system

// writeResolvFile does nothing off Linux: ze manages resolv.conf only on the
// appliance and Linux hosts. On Linux it writes servers to path.
func writeResolvFile(_ string, _ []string) error {
	return nil
}
