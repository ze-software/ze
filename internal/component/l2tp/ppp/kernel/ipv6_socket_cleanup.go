// Design: docs/research/l2tpv2-ze-integration.md -- exact Linux network socket release proof
package kernel

import (
	"context"
	"errors"
	"net"
	"runtime"
	"sync"
	"syscall"
)

// Native factory sockets are privately owned Go IPConn/UDPConn objects. Their
// poll.FD.Close seals the object, evicts and joins nonblocking I/O, waits for
// destroy, and sets Sysfd=-1 even if destroy reports its syscall Close error.
// Linux7.3 fs/open.c close calls file_close_fd before filp_flush/fput_close_sync; such a
// close error never authorizes retrying the numeric fd (it may be reused).
// Reconciliation probes ONLY the same concrete sealed Go object, never an
// interface lookup or arbitrary provider's net.ErrClosed result. Normal Close
// still returns its first actual error, which callers retain for diagnostics.
type nativeIPv6SocketCleanup struct {
	owner     net.PacketConn
	mu        sync.Mutex
	attempted bool
	firstErr  error
}

func (c *nativeIPv6SocketCleanup) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.attempted {
		return c.firstErr
	}
	c.attempted = true
	c.firstErr = c.owner.Close()
	return c.firstErr
}
func (c *nativeIPv6SocketCleanup) RetryIPv6SocketCleanup(ctx context.Context) error {
	if ctx == nil {
		return errors.New("IPv6 socket cleanup requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !c.mu.TryLock() {
		return errors.New("IPv6 socket cleanup is already running")
	}
	defer c.mu.Unlock()
	if !c.attempted {
		return errors.New("IPv6 socket cleanup requires its first Close")
	}
	if c.firstErr == nil {
		return nil
	}
	if runtime.GOOS != "linux" {
		return c.firstErr
	}
	var native syscall.Conn
	switch owner := c.owner.(type) {
	case *net.IPConn:
		native = owner
	case *net.UDPConn:
		native = owner
	default:
		return c.firstErr
	}
	raw, err := native.SyscallConn()
	if err != nil {
		return errors.Join(c.firstErr, err)
	}
	entered := false
	err = raw.Control(func(uintptr) { entered = true })
	if entered {
		return c.firstErr
	}
	if !errors.Is(err, net.ErrClosed) {
		return errors.Join(c.firstErr, err)
	}
	// On these exact concrete Linux owners, returned Close plus the sealed poll
	// object proves fdtable release. No second socket Close/syscall is attempted.
	return nil
}

type ownedIPv6DHCPSocket struct {
	net.PacketConn
	cleanup *nativeIPv6SocketCleanup
}

func (s *ownedIPv6DHCPSocket) Close() error { return s.cleanup.Close() }
func (s *ownedIPv6DHCPSocket) RetryIPv6SocketCleanup(ctx context.Context) error {
	return s.cleanup.RetryIPv6SocketCleanup(ctx)
}

// The constructor consumes dedicated DHCP sockets. Concrete Go sockets need
// their provider's exact-object proof; opaque provider objects remain opaque.
func ipv6DHCPCleanupOwner(socket dhcpSocket) dhcpSocket {
	switch native := socket.(type) {
	case *net.UDPConn:
		return &ownedIPv6DHCPSocket{PacketConn: native, cleanup: &nativeIPv6SocketCleanup{owner: native}}
	case *net.IPConn:
		return &ownedIPv6DHCPSocket{PacketConn: native, cleanup: &nativeIPv6SocketCleanup{owner: native}}
	default:
		return socket
	}
}
