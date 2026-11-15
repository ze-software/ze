// Design: docs/architecture/testing/qemu-integration.md -- native fixture TCP forwards.

package testqemu

import (
	"net"
	"testing"
)

// installTestListenerPort rejects an unexpected fixture transport before using
// the port of the real TCP listener that owns the forward.
func installTestListenerPort(t *testing.T, listener net.Listener) int {
	t.Helper()
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("fixture listener address is %T, want *net.TCPAddr", listener.Addr())
	}
	if address == nil {
		t.Fatal("fixture TCP listener has no address")
	}
	return address.Port
}
