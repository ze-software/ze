// Design: docs/architecture/core-design.md -- FIB P4 backend
// Overview: fibp4.go -- FIB P4 plugin
//
// P4 backend interface and noop implementation.
// The real P4Runtime gRPC backend requires adding google.golang.org/grpc
// and a P4Runtime protobuf dependency to go.mod. The noop backend logs
// operations for testing and development without a P4 switch.
package fibp4

// newBackend returns the P4 backend.
// Returns noop until gRPC/P4Runtime dependency is added to go.mod.
func newBackend(_ string, _ int) p4Backend {
	return &noopBackend{}
}

// noopBackend logs P4 operations without programming a real switch.
type noopBackend struct{}

func (n *noopBackend) addRoute(prefix, nextHop string) error {
	logger().Info("fib-p4: add route (noop)", "prefix", prefix, "next-hop", nextHop)
	return nil
}

func (n *noopBackend) delRoute(prefix string) error {
	logger().Info("fib-p4: del route (noop)", "prefix", prefix)
	return nil
}

func (n *noopBackend) replaceRoute(prefix, nextHop string) error {
	logger().Info("fib-p4: replace route (noop)", "prefix", prefix, "next-hop", nextHop)
	return nil
}

func (n *noopBackend) close() error { return nil }
