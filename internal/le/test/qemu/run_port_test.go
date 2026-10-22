package testqemu

import (
	"net"
	"syscall"
	"testing"
)

// VALIDATES: the SSH port probe refuses a port another process holds on the
// address a QEMU hostfwd binds, loopback or wildcard, so two concurrent runs
// never hand QEMU the same port.
// METHOD: hold 127.0.0.1:<port> (the docker-lab forward), then 0.0.0.0:<port>
// (the run and installer forward), each with SO_REUSEADDR set as QEMU's slirp
// sets it, and ask the probe for exactly that port.
// PREVENTS: the 2026-10-21 docker-lab collision, where a wildcard probe bound
// beside another VM's 127.0.0.1 forward and the second run died with an sshd
// timeout.
func TestFreeSSHPortRefusesAHeldForward(t *testing.T) {
	for _, address := range []string{"127.0.0.1:0", "0.0.0.0:0"} {
		t.Run(address, func(t *testing.T) {
			var config net.ListenConfig
			held, err := config.Listen(t.Context(), "tcp4", address)
			if err != nil {
				t.Fatalf("hold %s: %v", address, err)
			}
			defer held.Close() //nolint:errcheck // the test's own listener
			tcp, ok := held.Addr().(*net.TCPAddr)
			if !ok {
				t.Fatalf("listener address %v is not TCP", held.Addr())
			}
			if port, err := freeSSHPortIn(tcp.Port, tcp.Port); err == nil {
				t.Fatalf("probe answered port %d while %s holds it", port, held.Addr())
			}
		})
	}
}

// VALIDATES: the probe refuses a port another process has bound on 127.0.0.1
// and not yet listened on, the window between another QEMU's bind and its
// listen.
// METHOD: bind a raw socket to 127.0.0.1:0 with SO_REUSEADDR, as slirp does,
// without listening, then ask the probe for that port. On Linux a probe that
// keeps SO_REUSEADDR set binds beside it; this is the red Linux can show, the
// listening case above being red on macOS.
// PREVENTS: a probe whose own SO_REUSEADDR lets it share a forward's port.
func TestFreeSSHPortRefusesABoundLoopbackForward(t *testing.T) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("open a socket: %v", err)
	}
	defer syscall.Close(fd) //nolint:errcheck // the test's own socket
	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		t.Fatalf("set SO_REUSEADDR: %v", err)
	}
	if err := syscall.Bind(fd, &syscall.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}); err != nil {
		t.Fatalf("bind 127.0.0.1:0: %v", err)
	}
	bound, err := syscall.Getsockname(fd)
	if err != nil {
		t.Fatalf("read the bound port: %v", err)
	}
	inet, ok := bound.(*syscall.SockaddrInet4)
	if !ok {
		t.Fatalf("bound address %v is not IPv4", bound)
	}
	if port, err := freeSSHPortIn(inet.Port, inet.Port); err == nil {
		t.Fatalf("probe answered port %d while 127.0.0.1:%d is bound", port, inet.Port)
	}
}

// VALIDATES: the probe still answers a port nobody holds.
// METHOD: take a free loopback port from the kernel, release it, then probe it.
// PREVENTS: a probe so strict it refuses every port and stops every run.
func TestFreeSSHPortAnswersAFreePort(t *testing.T) {
	var config net.ListenConfig
	listener, err := config.Listen(t.Context(), "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("take a port: %v", err)
	}
	tcp, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("listener address %v is not TCP", listener.Addr())
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("release the port: %v", err)
	}
	port, err := freeSSHPortIn(tcp.Port, tcp.Port)
	if err != nil {
		t.Fatalf("probe refused free port %d: %v", tcp.Port, err)
	}
	if port != tcp.Port {
		t.Fatalf("probe answered %d, want %d", port, tcp.Port)
	}
}
