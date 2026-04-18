// Design: docs/architecture/core-design.md -- FIB noop backend
// Overview: fibkernel.go -- FIB kernel plugin
// Related: backend.go -- backend abstraction
// Related: backend_linux.go -- Linux netlink backend
//
// Noop backend for non-Linux platforms. Logs route operations but does not
// program the OS routing table. Darwin route socket support is defined in
// the spec but requires platform-specific route table ID conventions.

//go:build !linux

package fibkernel

// newBackend returns a noop backend on non-Linux platforms.
func newBackend() routeBackend {
	return &noopBackend{}
}

// noopBackend logs route operations without programming the OS routing table.
type noopBackend struct{}

func (n *noopBackend) addRoute(prefix, nextHop string) error {
	logger().Debug("fib-kernel: add route (noop)", "prefix", prefix, "next-hop", nextHop)
	return nil
}

func (n *noopBackend) delRoute(prefix string) error {
	logger().Debug("fib-kernel: del route (noop)", "prefix", prefix)
	return nil
}

func (n *noopBackend) replaceRoute(prefix, nextHop string) error {
	logger().Debug("fib-kernel: replace route (noop)", "prefix", prefix, "next-hop", nextHop)
	return nil
}

func (n *noopBackend) listZeRoutes() ([]installedRoute, error) {
	return nil, nil
}

func (n *noopBackend) close() error { return nil }
